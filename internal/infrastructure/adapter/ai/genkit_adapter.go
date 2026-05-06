// Package ai — GenkitAdapter is a pluggable implementation of port.AIProvider
// backed by Firebase Genkit Go. The provider-specific behavior (which Genkit
// plugin to load, model-name qualification, and the raw config struct passed
// to ai.WithConfig) is delegated to a genkitPluginAdapter selected at
// construction time.
//
// It exists alongside AnthropicAdapter (NOT a replacement). Selection is
// performed at DI-wiring time via the AGENT_AI_PROVIDER env var; the Genkit
// plugin within is selected via AGENT_GENKIT_PLUGIN.
//
// Design notes:
//   - The Genkit registry is process-global. Tools are registered ONCE at
//     construction; Clone() shares the same *genkit.Genkit reference so the
//     registry is not mutated again.
//   - Domain ToolParam.Name may not satisfy Genkit's tool-name regex
//     (`^[a-zA-Z0-9_-]{1,64}$`). We sanitize on registration and maintain a
//     bidirectional name map so the original (port-facing) name is returned in
//     ToolCallInfo.
//   - The domain layer drives the tool-calling loop; we therefore pass
//     ai.WithReturnToolRequests(true) so Genkit surfaces tool requests rather
//     than executing them.
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"

	genkitai "github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// ErrHFModelNotSupported is returned when a hf:* model string is passed to
// a Genkit plugin that requires native model identifiers (e.g. Anthropic).
// Users wishing to point at a custom endpoint should set the plugin's base
// URL env var (e.g. ANTHROPIC_BASE_URL) and use a native model identifier.
var ErrHFModelNotSupported = errors.New(
	`hf:* model strings are not supported by this Genkit plugin; ` +
		`set the plugin's base URL env var (e.g. ANTHROPIC_BASE_URL) to use a custom endpoint`,
)

var (
	// genkitToolNameRE is Genkit's enforced regex for tool names.
	genkitToolNameRE = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)
	// genkitInvalidCharRE matches any character that is NOT permitted in a
	// Genkit tool name; used to replace invalid runes with '_'.
	genkitInvalidCharRE = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
)

// GenkitAdapter implements the AIProvider port using Firebase Genkit Go.
//
// The struct holds a reference to a *genkit.Genkit registry (shared across
// Clone() calls — registries are process-global and intentionally so) and an
// independent model field per instance.
type GenkitAdapter struct {
	g               *genkit.Genkit
	plugin          genkitPluginAdapter
	model           string
	maxTokens       int64
	subagentManager port.SubagentManager
	logger          port.Logger

	// nameMapMu guards toolNameMap. Held as a pointer so Clone() shares the
	// SAME mutex (and the SAME map) across the original and all clones —
	// registration is lazy (first SendMessage), so concurrent SendMessage
	// calls across clones must serialize on a single mutex, not per-clone
	// zero-value mutexes.
	nameMapMu *sync.RWMutex
	// toolNameMap maps the sanitized (genkit) tool name back to the original
	// port-facing name so ToolCallInfo.ToolName preserves the contract.
	toolNameMap map[string]string
}

// NewGenkitAdapter constructs a Genkit-backed AIProvider using the default
// Anthropic plugin. Equivalent to NewGenkitAdapterWithPlugin with
// pluginName == GenkitPluginAnthropic; retained as the simpler call site for
// callers that don't need to vary the underlying provider.
func NewGenkitAdapter(
	model string,
	maxTokens int64,
	subagentManager port.SubagentManager,
	logger port.Logger,
) (port.AIProvider, error) {
	return NewGenkitAdapterWithPlugin(GenkitPluginAnthropic, model, maxTokens, subagentManager, logger)
}

// NewGenkitAdapterWithPlugin constructs a Genkit-backed AIProvider using the
// plugin registered under pluginName.
//
// Construction-time work:
//  1. Resolve the plugin from the registry.
//  2. Validate the model string against the plugin's rules (e.g. anthropic
//     rejects hf:* prefixes).
//  3. Initialize a Genkit instance with the resolved plugin loaded.
//  4. Stash references; tool registration happens lazily on first Send* call.
//
// Parameters:
//   - pluginName: registered plugin identifier (e.g. "anthropic", "googleai",
//     "vertexai", "ollama"). Empty defaults to "anthropic".
//   - model: provider-native model identifier. May be qualified with the
//     plugin's provider prefix; if not, the prefix is added at request time.
//   - maxTokens: max response tokens (passed via ai.WithConfig escape hatch).
//   - subagentManager: optional, for system-prompt augmentation.
//   - logger: structured logger (NopLogger if nil).
func NewGenkitAdapterWithPlugin(
	pluginName string,
	model string,
	maxTokens int64,
	subagentManager port.SubagentManager,
	logger port.Logger,
) (port.AIProvider, error) {
	logger = port.SafeLogger(logger)

	plugin, err := resolveGenkitPlugin(pluginName)
	if err != nil {
		return nil, err
	}

	if vErr := plugin.ValidateModel(model); vErr != nil {
		return nil, vErr
	}

	g, err := initGenkit(plugin, logger)
	if err != nil {
		return nil, err
	}

	return &GenkitAdapter{
		g:               g,
		plugin:          plugin,
		model:           model,
		maxTokens:       maxTokens,
		subagentManager: subagentManager,
		logger:          logger,
		nameMapMu:       &sync.RWMutex{},
		toolNameMap:     make(map[string]string),
	}, nil
}

// initGenkit calls genkit.Init with the resolved plugin. Panics from
// genkit.Init (invalid options, missing API keys, etc.) are caught and
// returned as errors. Dev-mode reflection servers are NOT started; we rely
// on Genkit's default of dev mode being off unless GENKIT_ENV=dev is set in
// the caller's environment.
func initGenkit(plugin genkitPluginAdapter, logger port.Logger) (g *genkit.Genkit, err error) { //nolint:nonamedreturns // err set in deferred recover
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("genkit.Init panicked: %v", r)
		}
	}()

	g = genkit.Init(context.Background(),
		genkit.WithPlugins(plugin.Plugin()),
	)
	logger.Debug("genkit initialized", "provider", plugin.Name())
	return g, nil
}

// SendMessage implements port.AIProvider.
func (a *GenkitAdapter) SendMessage(
	ctx context.Context,
	messages []port.MessageParam,
	tools []port.ToolParam,
	opts port.AIRequestOptions,
) (*entity.Message, []port.ToolCallInfo, error) {
	genOpts, err := a.buildGenerateOptions(ctx, messages, tools, opts)
	if err != nil {
		return nil, nil, err
	}

	resp, err := genkit.Generate(ctx, a.g, genOpts...)
	if err != nil {
		return nil, nil, fmt.Errorf("genkit Generate failed: %w", err)
	}

	return a.convertResponse(resp)
}

// SendMessageStreaming implements port.AIProvider. It iterates Genkit's
// streaming response, demuxing text and reasoning parts to the appropriate
// callback. Tool-request parts are deferred to the final response (which the
// iterator emits as Done).
//
//nolint:gocognit // streaming demux requires inline branching across part types
func (a *GenkitAdapter) SendMessageStreaming(
	ctx context.Context,
	messages []port.MessageParam,
	tools []port.ToolParam,
	opts port.AIRequestOptions,
	textCallback port.StreamCallback,
	thinkingCallback port.ThinkingCallback,
) (*entity.Message, []port.ToolCallInfo, error) {
	genOpts, err := a.buildGenerateOptions(ctx, messages, tools, opts)
	if err != nil {
		return nil, nil, err
	}

	thinkingEnabled := opts.Thinking != nil && opts.Thinking.Enabled

	var finalResp *genkitai.ModelResponse
	for value, iterErr := range genkit.GenerateStream(ctx, a.g, genOpts...) {
		if iterErr != nil {
			return nil, nil, fmt.Errorf("genkit stream error: %w", iterErr)
		}
		if value.Done {
			finalResp = value.Response
			continue
		}
		if value.Chunk == nil {
			continue
		}
		for _, p := range value.Chunk.Content {
			switch {
			case p.IsText():
				if textCallback != nil && p.Text != "" {
					if cbErr := textCallback(p.Text); cbErr != nil {
						return nil, nil, fmt.Errorf("text stream callback error: %w", cbErr)
					}
				}
			case p.IsReasoning():
				if thinkingCallback != nil && thinkingEnabled && p.Text != "" {
					if cbErr := thinkingCallback(p.Text); cbErr != nil {
						return nil, nil, fmt.Errorf("thinking stream callback error: %w", cbErr)
					}
				}
			}
		}
	}

	if finalResp == nil {
		return nil, nil, errors.New("genkit stream ended without a final response")
	}

	return a.convertResponse(finalResp)
}

// GenerateToolSchema implements port.AIProvider.
func (a *GenkitAdapter) GenerateToolSchema() port.ToolInputSchemaParam {
	return port.ToolInputSchemaParam{}
}

// HealthCheck implements port.AIProvider. Mirrors AnthropicAdapter: passes if
// a model is configured. The Anthropic Genkit plugin resolves models lazily
// (via ResolveAction), so deeper validation would require an API call.
func (a *GenkitAdapter) HealthCheck(_ context.Context) error {
	if a.model == "" {
		return fmt.Errorf("%w: model not configured", ErrClientHealthCheck)
	}
	return nil
}

// SetModel implements port.AIProvider.
func (a *GenkitAdapter) SetModel(model string) error {
	if model == "" {
		return errors.New("model cannot be empty")
	}
	if a.plugin != nil {
		if err := a.plugin.ValidateModel(model); err != nil {
			return err
		}
	}
	a.model = model
	return nil
}

// GetModel implements port.AIProvider.
func (a *GenkitAdapter) GetModel() string {
	return a.model
}

// Clone implements port.AIProvider. The clone shares the *genkit.Genkit
// registry (registries are process-global; re-registering tools across clones
// would be a bug) but holds independent model/maxTokens fields.
func (a *GenkitAdapter) Clone() port.AIProvider {
	return &GenkitAdapter{
		g:               a.g,
		plugin:          a.plugin,
		model:           a.model,
		maxTokens:       a.maxTokens,
		subagentManager: a.subagentManager,
		logger:          a.logger,
		// Tool name map AND its mutex are shared by reference — the map is
		// populated lazily by registerTools on every SendMessage, so the
		// original and clones must serialize on the SAME mutex to avoid a
		// data race on concurrent first registrations.
		nameMapMu:   a.nameMapMu,
		toolNameMap: a.toolNameMap,
	}
}

// buildGenerateOptions assembles the []ai.GenerateOption slice for a request.
// It validates inputs, registers any not-yet-registered tools, builds the
// system prompt, builds the raw-config escape hatch (max tokens + thinking),
// converts messages, and assembles the final option list.
func (a *GenkitAdapter) buildGenerateOptions(
	ctx context.Context,
	messages []port.MessageParam,
	tools []port.ToolParam,
	opts port.AIRequestOptions,
) ([]genkitai.GenerateOption, error) {
	if len(messages) == 0 {
		return nil, ErrEmptyMessages
	}
	if a.model == "" {
		return nil, ErrModelNotSet
	}

	systemPrompt, err := a.getSystemPrompt(ctx, opts)
	if err != nil {
		return nil, err
	}

	toolRefs, err := a.registerTools(tools)
	if err != nil {
		return nil, err
	}

	gMessages := a.convertMessagesToGenkit(messages)

	genOpts := []genkitai.GenerateOption{
		genkitai.WithModelName(a.qualifiedModelName(a.model)),
		genkitai.WithMessages(gMessages...),
		genkitai.WithSystem(systemPrompt),
		genkitai.WithReturnToolRequests(true),
	}

	// Raw-config escape hatch: each plugin returns its own config struct
	// (anthropic.MessageNewParams, *genai.GenerateContentConfig, …). A nil
	// return is valid and skips the option, which lets plugins that don't
	// need a config struct stay out of the way.
	if cfg := a.plugin.BuildRequestConfig(a.maxTokens, opts.Thinking); cfg != nil {
		genOpts = append(genOpts, genkitai.WithConfig(cfg))
	}

	if len(toolRefs) > 0 {
		genOpts = append(genOpts, genkitai.WithTools(toolRefs...))
	}

	return genOpts, nil
}

// qualifiedModelName ensures the model name carries the plugin's provider
// prefix expected by Genkit's resolver. Names that already contain a slash
// are assumed to be fully qualified and returned unchanged.
func (a *GenkitAdapter) qualifiedModelName(model string) string {
	if strings.Contains(model, "/") {
		return model
	}
	return a.plugin.Name() + "/" + model
}

// registerTools registers each domain ToolParam with the Genkit registry,
// using ai.WithStrict(false) (provided by the fork) so opt-out of strict
// schema enforcement works per-tool. Names that don't match Genkit's regex
// are sanitized; the original-name mapping is recorded for response decoding.
//
// Registration is idempotent: if a sanitized name has already been recorded
// in toolNameMap we reuse it (genkit's registry would otherwise panic on
// duplicate Define).
func (a *GenkitAdapter) registerTools(tools []port.ToolParam) ([]genkitai.ToolRef, error) {
	if len(tools) == 0 {
		return nil, nil
	}
	refs := make([]genkitai.ToolRef, 0, len(tools))

	for _, t := range tools {
		sanitized, didSanitize := sanitizeToolName(t.Name)
		if didSanitize {
			a.logger.Warn("tool name sanitized for genkit",
				"original", t.Name,
				"sanitized", sanitized,
			)
		}

		// Track the bidirectional mapping so the response decoder can map
		// genkit's reported tool name back to the original port name.
		// Steady state: every tool is already mapped — take RLock and skip
		// the write so per-request lock contention stays low.
		a.nameMapMu.RLock()
		existingOrig, alreadyRegistered := a.toolNameMap[sanitized]
		a.nameMapMu.RUnlock()
		if !alreadyRegistered || existingOrig != t.Name {
			a.nameMapMu.Lock()
			a.toolNameMap[sanitized] = t.Name
			a.nameMapMu.Unlock()
		}

		if alreadyRegistered {
			// Recover the existing registered tool by name (LookupTool returns
			// nil on miss; if so we re-register defensively below).
			if existing := genkit.LookupTool(a.g, sanitized); existing != nil {
				refs = append(refs, existing)
				continue
			}
		}

		schema := t.InputSchema
		if schema == nil {
			schema = map[string]interface{}{
				"type":       "object",
				"properties": map[string]interface{}{},
			}
		}

		// The tool function is a stub: we use WithReturnToolRequests(true) to
		// route execution back to the domain layer, so this body is never
		// invoked. We still need a non-nil function for genkit to register.
		stubFn := func(_ *genkitai.ToolContext, _ any) (any, error) {
			return nil, fmt.Errorf("genkit-registered tool %q must not be invoked directly; "+
				"the domain layer drives tool execution via WithReturnToolRequests", sanitized)
		}

		def, regErr := defineToolSafe(a.g, sanitized, t.Description, schema, stubFn, t.Strict)
		if regErr != nil {
			return nil, regErr
		}
		refs = append(refs, def)
	}

	return refs, nil
}

// defineToolSafe wraps genkit.DefineTool in a recover() so a panic (e.g.
// duplicate registration on the global registry) becomes an error.
func defineToolSafe(
	g *genkit.Genkit,
	name, description string,
	schema map[string]interface{},
	fn genkitai.ToolFunc[any, any],
	strict bool,
) (*genkitai.ToolDef[any, any], error) {
	var def *genkitai.ToolDef[any, any]
	var panicErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicErr = fmt.Errorf("DefineTool %q panicked: %v", name, r)
			}
		}()
		def = genkit.DefineTool(g, name, description, fn,
			genkitai.WithInputSchema(schema),
			genkitai.WithStrict(strict),
		)
	}()
	if panicErr != nil {
		return nil, panicErr
	}
	return def, nil
}

// sanitizeToolName returns a name acceptable to Genkit's regex
// (`^[a-zA-Z0-9_-]{1,64}$`). It returns the (possibly modified) name and a
// bool indicating whether modification occurred. Empty input becomes "_".
func sanitizeToolName(name string) (string, bool) {
	if genkitToolNameRE.MatchString(name) {
		return name, false
	}
	cleaned := genkitInvalidCharRE.ReplaceAllString(name, "_")
	if cleaned == "" {
		cleaned = "_"
	}
	if len(cleaned) > 64 {
		cleaned = cleaned[:64]
	}
	return cleaned, cleaned != name
}

// originalToolName resolves a sanitized (genkit-facing) tool name back to the
// original port-facing name. Falls back to the input when no mapping exists
// (e.g. for a name that didn't need sanitization).
func (a *GenkitAdapter) originalToolName(genkitName string) string {
	a.nameMapMu.RLock()
	defer a.nameMapMu.RUnlock()
	if orig, ok := a.toolNameMap[genkitName]; ok {
		return orig
	}
	return genkitName
}

// convertMessagesToGenkit translates the port message slice to Genkit's
// []*ai.Message representation. For assistant messages with tool calls and/or
// thinking blocks, parts are emitted in the order: reasoning → text → tool
// requests, mirroring what the Anthropic adapter sends back to the API.
func (a *GenkitAdapter) convertMessagesToGenkit(messages []port.MessageParam) []*genkitai.Message {
	out := make([]*genkitai.Message, 0, len(messages))
	for _, msg := range messages {
		out = append(out, a.convertMessageToGenkit(msg))
	}
	return out
}

func (a *GenkitAdapter) convertMessageToGenkit(msg port.MessageParam) *genkitai.Message {
	role := genkitai.RoleUser
	if msg.Role == entity.RoleAssistant {
		role = genkitai.RoleModel
	}

	if msg.Role == entity.RoleUser && len(msg.ToolResults) > 0 {
		parts := make([]*genkitai.Part, 0, len(msg.ToolResults))
		for _, tr := range msg.ToolResults {
			// Genkit pairs responses by Ref, not Name — carry ToolID through
			// as Ref so multi-call rounds match up across providers.
			parts = append(parts, genkitai.NewToolResponsePart(&genkitai.ToolResponse{
				Ref:    tr.ToolID,
				Output: toolResultOutput(tr.Result, tr.IsError),
			}))
		}
		return &genkitai.Message{Role: genkitai.RoleUser, Content: parts}
	}

	if msg.Role == entity.RoleAssistant && (len(msg.ToolCalls) > 0 || len(msg.ThinkingBlocks) > 0) {
		parts := make([]*genkitai.Part, 0, len(msg.ThinkingBlocks)+len(msg.ToolCalls)+1)

		// Reasoning first (preserves the assistant turn ordering).
		for _, tb := range msg.ThinkingBlocks {
			parts = append(parts, genkitai.NewReasoningPart(tb.Thinking, []byte(tb.Signature)))
		}
		if msg.Content != "" {
			parts = append(parts, genkitai.NewTextPart(msg.Content))
		}
		for _, tc := range msg.ToolCalls {
			sanitized, _ := sanitizeToolName(tc.ToolName)
			parts = append(parts, genkitai.NewToolRequestPart(&genkitai.ToolRequest{
				Ref:   tc.ToolID,
				Name:  sanitized,
				Input: tc.Input,
			}))
		}
		return &genkitai.Message{Role: role, Content: parts}
	}

	return &genkitai.Message{
		Role:    role,
		Content: []*genkitai.Part{genkitai.NewTextPart(msg.Content)},
	}
}

// toolResultOutput packages a tool-call string result into a JSON-friendly
// shape. Errors are reported as {"error": "..."} so they're distinguishable
// downstream.
func toolResultOutput(result string, isError bool) any {
	if isError {
		return map[string]any{"error": result}
	}
	return map[string]any{"result": result}
}

// convertResponse converts a Genkit *ModelResponse to (*entity.Message,
// []port.ToolCallInfo, error). It walks the message's parts, accumulating
// text, surfacing tool requests, and collecting reasoning blocks.
//
//nolint:gocognit // walks heterogeneous part types in one pass
func (a *GenkitAdapter) convertResponse(resp *genkitai.ModelResponse) (*entity.Message, []port.ToolCallInfo, error) {
	if resp == nil || resp.Message == nil {
		return nil, nil, errors.New("genkit response missing message")
	}

	var contentBuilder strings.Builder
	toolCalls := []port.ToolCallInfo{}
	entityToolCalls := []entity.ToolCall{}
	thinkingBlocks := []entity.ThinkingBlock{}

	for _, p := range resp.Message.Content {
		switch {
		case p.IsText():
			contentBuilder.WriteString(p.Text)
		case p.IsReasoning():
			sig := ""
			if p.Metadata != nil {
				if raw, ok := p.Metadata["signature"]; ok {
					switch v := raw.(type) {
					case []byte:
						sig = string(v)
					case string:
						sig = v
					}
				}
			}
			thinkingBlocks = append(thinkingBlocks, entity.ThinkingBlock{
				Thinking:  p.Text,
				Signature: sig,
			})
		case p.IsToolRequest() && p.ToolRequest != nil:
			tr := p.ToolRequest
			inputMap, inputJSON := toolRequestInput(tr.Input)
			origName := a.originalToolName(tr.Name)
			toolCalls = append(toolCalls, port.ToolCallInfo{
				ToolID:    tr.Ref,
				ToolName:  origName,
				Input:     inputMap,
				InputJSON: inputJSON,
			})
			entityToolCalls = append(entityToolCalls, entity.ToolCall{
				ToolID:   tr.Ref,
				ToolName: origName,
				Input:    inputMap,
			})
		}
	}

	content := contentBuilder.String()
	// Mirror AnthropicAdapter: tool-only turns (the common agentic loop) leave
	// content empty. Use FinishReason (e.g. "tool_use") so we don't inject a
	// fake "[No content received from AI]" string into conversation history
	// that the model would see on the next turn.
	if content == "" && resp.FinishReason != "" {
		content = string(resp.FinishReason)
	}
	if content == "" && len(thinkingBlocks) > 0 {
		content = "[AI internal reasoning completed]"
	}
	if content == "" {
		content = "[No content received from AI]"
	}

	msg, err := entity.NewMessageWithThinkingBlocks(entity.RoleAssistant, content, thinkingBlocks)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create message: %w", err)
	}
	if len(entityToolCalls) > 0 {
		msg.ToolCalls = entityToolCalls
	}

	if resp.Usage != nil {
		// Genkit uses int; the domain layer expects int64. Explicit cast.
		msg.InputTokens = int64(resp.Usage.InputTokens)
		msg.OutputTokens = int64(resp.Usage.OutputTokens)
		if resp.Usage.CachedContentTokens > 0 {
			a.logger.Debug("genkit cache tokens",
				"cached_content_tokens", resp.Usage.CachedContentTokens,
			)
		}
	}

	return msg, toolCalls, nil
}

// toolRequestInput normalizes a Genkit ToolRequest.Input (which is
// declared as `any`) into (map[string]interface{}, json-string). We accept
// either a map or a JSON-encodable value; a non-map is wrapped into
// {"value": ...} so the domain layer always sees a map.
func toolRequestInput(in any) (map[string]interface{}, string) {
	if in == nil {
		return map[string]interface{}{}, "{}"
	}
	if m, ok := in.(map[string]interface{}); ok {
		raw, err := json.Marshal(m)
		if err != nil {
			return m, "{}"
		}
		return m, string(raw)
	}
	// Fall back: round-trip via JSON to flatten.
	raw, err := json.Marshal(in)
	if err != nil {
		return map[string]interface{}{}, "{}"
	}
	var m map[string]interface{}
	if jsonErr := json.Unmarshal(raw, &m); jsonErr == nil && m != nil {
		return m, string(raw)
	}
	return map[string]interface{}{"value": in}, string(raw)
}

// getSystemPrompt delegates to the shared resolveSystemPrompt helper so prompt
// text is identical across adapters.
func (a *GenkitAdapter) getSystemPrompt(ctx context.Context, opts port.AIRequestOptions) (string, error) {
	return resolveSystemPrompt(ctx, opts, a.subagentManager)
}

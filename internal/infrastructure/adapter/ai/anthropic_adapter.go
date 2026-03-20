// Package ai provides an Anthropic AI adapter that implements the domain AIProvider port.
// It follows hexagonal architecture principles by providing infrastructure-level AI service
// operations with proper error handling and configuration management.
//
// The adapter uses the Anthropic SDK for API communication and supports multiple models
// including Claude models and custom models through the API.
//
// Example usage:
//
//	adapter := ai.NewAnthropicAdapter("hf:zai-org/GLM-4.7")
//	response, err := adapter.SendMessage(ctx, messages, tools)
//	if err != nil {
//		log.Fatal(err)
//	}
package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/shared/constant"
)

var (
	// ErrEmptyMessages is returned when SendMessage is called with no messages.
	ErrEmptyMessages = errors.New("messages cannot be empty")

	// ErrModelNotSet is returned when a request is made without setting a model.
	ErrModelNotSet = errors.New("model must be set before sending messages")

	// ErrClientHealthCheck is returned when the AI provider health check fails.
	ErrClientHealthCheck = errors.New("AI provider health check failed")
)

// AnthropicAdapter implements the AIProvider port using Anthropic's API.
// It provides a clean interface to interact with Anthropic's AI models while
// abstracting away the complexity of the API client implementation.
//
// The struct maintains an internal Anthropic client and model configuration,
// allowing for consistent model usage across all requests.
type AnthropicAdapter struct {
	client          anthropic.Client
	model           string
	maxTokens       int64
	subagentManager port.SubagentManager
}

// NewAnthropicAdapter creates a new AnthropicAdapter with the specified model.
// If the model is empty, a default error will be returned when SendMessage is called.
//
// Parameters:
//   - model: The AI model to use (e.g., "hf:zai-org/GLM-4.7", "claude-3-5-sonnet-20241022")
//   - maxTokens: Maximum tokens for AI response
//   - subagentManager: Optional subagent manager for providing subagent metadata to the system prompt
//
// Returns:
//   - port.AIProvider: An implementation of the AIProvider interface
func NewAnthropicAdapter(
	model string,
	maxTokens int64,
	subagentManager port.SubagentManager,
) port.AIProvider {
	return &AnthropicAdapter{
		client:          anthropic.NewClient(),
		model:           model,
		maxTokens:       maxTokens,
		subagentManager: subagentManager,
	}
}

// SendMessage sends a message to the Anthropic API with the provided messages and tools.
// It converts domain port types to Anthropic SDK types and handles the API response,
// converting it back to domain entity types.
//
// The method supports both regular text messages and tool use. If the AI responds with
// tool use, those will be included in the returned message's content.
//
// Parameters:
//   - ctx: Context for the request (supports cancellation and timeout)
//   - messages: Slice of MessageParam representing the conversation history
//   - tools: Slice of ToolParam representing available tools for the AI
//   - opts: AIRequestOptions carrying explicit configuration
//
// Returns:
//   - *entity.Message: The AI's response including any tool use blocks
//   - []port.ToolCallInfo: Information about tools requested by the AI
//   - error: An error if the request fails or validation fails
func (a *AnthropicAdapter) SendMessage(
	ctx context.Context,
	messages []port.MessageParam,
	tools []port.ToolParam,
	opts port.AIRequestOptions,
) (*entity.Message, []port.ToolCallInfo, error) {
	params, err := a.buildMessageParams(ctx, messages, tools, opts)
	if err != nil {
		return nil, nil, err
	}

	// Call Anthropic API
	response, err := a.client.Messages.New(ctx, params)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Convert response to domain Message and extract tool info
	return a.convertResponse(response)
}

// SendMessageStreaming sends a message to the Anthropic API with streaming support.
// It calls the provided callbacks for each text and thinking chunk as they arrive from the API.
//
// The method accumulates the full message while streaming and handles both text content
// and tool use blocks. The textCallback is called for text deltas, and thinkingCallback
// is called for thinking deltas (if provided and thinking mode is enabled).
//
// Parameters:
//   - ctx: Context for the request (supports cancellation and timeout)
//   - messages: Slice of MessageParam representing the conversation history
//   - tools: Slice of ToolParam representing available tools for the AI
//   - opts: AIRequestOptions carrying explicit configuration
//   - textCallback: Function called for each text chunk as it arrives
//   - thinkingCallback: Function called for each thinking chunk (can be nil to skip)
//
// Returns:
//   - *entity.Message: The complete AI response including any tool use blocks
//   - []port.ToolCallInfo: Information about tools requested by the AI
//   - error: An error if the request fails or validation fails
func (a *AnthropicAdapter) SendMessageStreaming(
	ctx context.Context,
	messages []port.MessageParam,
	tools []port.ToolParam,
	opts port.AIRequestOptions,
	textCallback port.StreamCallback,
	thinkingCallback port.ThinkingCallback,
) (*entity.Message, []port.ToolCallInfo, error) {
	params, err := a.buildMessageParams(ctx, messages, tools, opts)
	if err != nil {
		return nil, nil, err
	}

	// Create streaming request
	stream := a.client.Messages.NewStreaming(ctx, params)

	// Accumulate the message as events arrive
	message := anthropic.Message{}
	for stream.Next() {
		event := stream.Current()
		err := message.Accumulate(event)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to accumulate event: %w", err)
		}

		// Handle content block deltas (text and thinking)
		eventVariant, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent)
		if !ok {
			continue
		}

		// Handle text deltas for streaming display
		if textDelta, ok := eventVariant.Delta.AsAny().(anthropic.TextDelta); ok {
			if textCallback != nil {
				if err := textCallback(textDelta.Text); err != nil {
					return nil, nil, fmt.Errorf("text stream callback error: %w", err)
				}
			}
		}

		// Handle thinking deltas for streaming display
		if thinkingDelta, ok := eventVariant.Delta.AsAny().(anthropic.ThinkingDelta); ok {
			if thinkingCallback != nil {
				if err := thinkingCallback(thinkingDelta.Thinking); err != nil {
					return nil, nil, fmt.Errorf("thinking stream callback error: %w", err)
				}
			}
		}
	}

	// Check for streaming errors
	if stream.Err() != nil {
		return nil, nil, fmt.Errorf("streaming error: %w", stream.Err())
	}

	// Convert accumulated message to domain Message and extract tool info
	return a.convertResponse(&message)
}

// buildMessageParams encapsulates the setup logic for anthropic.MessageNewParams.
func (a *AnthropicAdapter) buildMessageParams(
	ctx context.Context,
	messages []port.MessageParam,
	tools []port.ToolParam,
	opts port.AIRequestOptions,
) (anthropic.MessageNewParams, error) {
	// Validate inputs
	if len(messages) == 0 {
		return anthropic.MessageNewParams{}, ErrEmptyMessages
	}
	if a.model == "" {
		return anthropic.MessageNewParams{}, ErrModelNotSet
	}

	// Convert port messages to Anthropic SDK messages
	anthropicMessages := a.convertMessages(messages)

	// Convert port tools to Anthropic SDK tools
	anthropicTools := a.convertTools(tools)

	// Get system prompt from explicit options
	systemPrompt, err := a.getSystemPrompt(ctx, opts)
	if err != nil {
		return anthropic.MessageNewParams{}, err
	}

	// Build thinking config from explicit options
	thinkingConfig := anthropic.ThinkingConfigParamUnion{OfDisabled: &anthropic.ThinkingConfigDisabledParam{}}
	if opts.Thinking != nil && opts.Thinking.Enabled {
		thinkingConfig = anthropic.ThinkingConfigParamOfEnabled(opts.Thinking.BudgetTokens)
	}

	return anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: a.maxTokens,
		Messages:  anthropicMessages,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Thinking:  thinkingConfig,
		Tools:     anthropicTools,
	}, nil
}

// getSystemPrompt returns the system prompt for the AI based on options priority.
//
// Priority order (highest to lowest):
//  1. Custom system prompt (from opts.SystemPrompt) - Takes precedence over all other prompts
//  2. Plan mode prompt (from opts.PlanMode) - Used when plan mode is active and no custom prompt exists
//  3. Base prompt with optional skill metadata - Default prompt when no special modes are active
func (a *AnthropicAdapter) getSystemPrompt(ctx context.Context, opts port.AIRequestOptions) (string, error) {
	// Priority 1: Check for custom system prompt (highest priority)
	if opts.SystemPrompt != "" {
		return opts.SystemPrompt, nil
	}

	// Priority 2: Check for plan mode prompt (second priority)
	if opts.PlanMode != nil && opts.PlanMode.Enabled {
		return a.buildPlanModePrompt(*opts.PlanMode), nil
	}

	// Priority 3: Return base prompt with optional skill metadata (default/fallback)
	return a.buildBasePromptWithSkills(ctx)
}

// buildPlanModePrompt constructs the specialized plan mode system prompt.
func (a *AnthropicAdapter) buildPlanModePrompt(planInfo port.PlanModeInfo) string {
	return fmt.Sprintf(
		`You are an AI assistant in PLAN MODE. Your job is to explore the codebase and write an implementation plan before making changes.

## Your Role in Plan Mode

You should:
1. Use read_file and list_files to understand the existing code
2. Use read-only bash commands (e.g., git status, ls, find) to explore
3. Write your implementation plan to: %s

## How to Write Your Plan

Use the edit_file tool to write your plan to %s. Structure your plan as:

### Summary
Brief overview of what you're implementing

### Files to Modify
- path/to/file1.go - what changes are needed
- path/to/file2.go - what changes are needed

### Implementation Steps
1. First step
2. Second step
...

### Considerations
- Any trade-offs or decisions to highlight

## Important Rules

- You CAN use edit_file to write to %s - this is your plan file
- Other mutating tools (edit_file for other paths, destructive bash commands) will be blocked
- If you try to use a blocked tool, you'll receive a reminder to write to your plan file instead
- Focus on thorough exploration and detailed planning before implementation

## When You're Done

When your plan is complete, tell the user to exit plan mode with :mode normal to begin implementation.
`,
		planInfo.PlanPath,
		planInfo.PlanPath,
		planInfo.PlanPath,
	)
}

// buildBasePromptWithSkills constructs the base system prompt.
func (a *AnthropicAdapter) buildBasePromptWithSkills(ctx context.Context) (string, error) {
	basePrompt := "You are an AI assistant that helps users with code editing and explanations. Use the available tools when necessary to provide accurate and helpful responses."

	if a.subagentManager == nil {
		return basePrompt, nil
	}

	agents, err := a.subagentManager.ListAgents(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list agents: %w", err)
	}

	if len(agents) == 0 {
		return basePrompt, nil
	}

	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\nYou have access to the following specialized agents:\n")
	for _, agent := range agents {
		fmt.Fprintf(&sb, "- %s: %s\n", agent.Name, agent.Description)
	}

	return sb.String(), nil
}

// GenerateToolSchema returns an empty tool input schema.
func (a *AnthropicAdapter) GenerateToolSchema() port.ToolInputSchemaParam {
	return port.ToolInputSchemaParam{}
}

// HealthCheck performs a basic health check on the Anthropic adapter.
func (a *AnthropicAdapter) HealthCheck(_ context.Context) error {
	if a.model == "" {
		return fmt.Errorf("%w: model not configured", ErrClientHealthCheck)
	}
	return nil
}

// SetModel sets the AI model to use for subsequent requests.
func (a *AnthropicAdapter) SetModel(model string) error {
	if model == "" {
		return errors.New("model cannot be empty")
	}
	a.model = model
	return nil
}

// GetModel returns the currently configured AI model.
func (a *AnthropicAdapter) GetModel() string {
	return a.model
}

// convertMessages converts port MessageParam slice to Anthropic SDK MessageParam slice.
func (a *AnthropicAdapter) convertMessages(messages []port.MessageParam) []anthropic.MessageParam {
	result := make([]anthropic.MessageParam, len(messages))
	for i, msg := range messages {
		result[i] = a.convertMessage(msg)
	}
	return result
}

// convertMessage converts a single port MessageParam to Anthropic SDK MessageParam.
func (a *AnthropicAdapter) convertMessage(msg port.MessageParam) anthropic.MessageParam {
	if msg.Role == entity.RoleUser && len(msg.ToolResults) > 0 {
		return a.convertUserToolResultMessage(msg)
	}
	if msg.Role == entity.RoleAssistant && (len(msg.ToolCalls) > 0 || len(msg.ThinkingBlocks) > 0) {
		return a.convertAssistantToolMessage(msg)
	}
	return a.convertSimpleMessage(msg)
}

// convertUserToolResultMessage converts a user message with tool results.
func (a *AnthropicAdapter) convertUserToolResultMessage(msg port.MessageParam) anthropic.MessageParam {
	resultBlocks := make([]anthropic.ContentBlockParamUnion, len(msg.ToolResults))
	for j, tr := range msg.ToolResults {
		resultBlocks[j] = anthropic.NewToolResultBlock(tr.ToolID, tr.Result, tr.IsError)
	}
	return anthropic.NewUserMessage(resultBlocks...)
}

// convertAssistantToolMessage converts an assistant message with thinking blocks, text, and tool calls.
func (a *AnthropicAdapter) convertAssistantToolMessage(msg port.MessageParam) anthropic.MessageParam {
	blocks := []anthropic.ContentBlockParamUnion{}

	// CRITICAL: Thinking blocks MUST come first
	for _, tb := range msg.ThinkingBlocks {
		blocks = append(blocks, anthropic.NewThinkingBlock(tb.Signature, tb.Thinking))
	}

	if msg.Content != "" {
		blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
	}

	for _, tc := range msg.ToolCalls {
		blocks = append(blocks, anthropic.NewToolUseBlock(tc.ToolID, tc.Input, tc.ToolName))
	}

	return anthropic.NewAssistantMessage(blocks...)
}

// convertSimpleMessage converts a simple text message.
func (a *AnthropicAdapter) convertSimpleMessage(msg port.MessageParam) anthropic.MessageParam {
	if msg.Role == entity.RoleAssistant {
		return anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content))
	}
	return anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content))
}

// convertTools converts port ToolParam slice to Anthropic SDK ToolUnionParam slice.
func (a *AnthropicAdapter) convertTools(tools []port.ToolParam) []anthropic.ToolUnionParam {
	result := make([]anthropic.ToolUnionParam, len(tools))
	for i, tool := range tools {
		result[i] = anthropic.ToolUnionParam{
			OfTool: a.buildToolParam(tool),
		}
	}
	return result
}

// buildToolParam constructs an anthropic.ToolParam from a port.ToolParam.
func (a *AnthropicAdapter) buildToolParam(tool port.ToolParam) *anthropic.ToolParam {
	param := &anthropic.ToolParam{
		Name:        tool.Name,
		Description: anthropic.String(tool.Description),
	}

	if tool.InputSchema != nil {
		param.InputSchema = a.convertInputSchema(tool.InputSchema)
	}

	return param
}

// convertInputSchema converts a port ToolInputSchemaParam to an anthropic ToolInputSchemaParam.
func (a *AnthropicAdapter) convertInputSchema(schema port.ToolInputSchemaParam) anthropic.ToolInputSchemaParam {
	return anthropic.ToolInputSchemaParam{
		Type:       constant.Object(extractStringField(schema, "type")),
		Properties: extractMapField(schema, "properties"),
		Required:   extractStringSliceField(schema, "required"),
	}
}

// extractStringField extracts a string value from a schema map.
func extractStringField(schema port.ToolInputSchemaParam, key string) string {
	if value, ok := schema[key].(string); ok {
		return value
	}
	return ""
}

// extractMapField extracts a map value from a schema map.
func extractMapField(schema port.ToolInputSchemaParam, key string) map[string]interface{} {
	if value, ok := schema[key].(map[string]interface{}); ok {
		return value
	}
	return nil
}

// extractStringSliceField extracts a string slice from a schema map.
func extractStringSliceField(schema port.ToolInputSchemaParam, key string) []string {
	if value, ok := schema[key].([]string); ok {
		return value
	}
	return nil
}

// convertResponse converts an Anthropic API response to a domain Message entity.
func (a *AnthropicAdapter) convertResponse(response *anthropic.Message) (*entity.Message, []port.ToolCallInfo, error) {
	var contentBuilder strings.Builder
	toolCalls := []port.ToolCallInfo{}
	entityToolCalls := []entity.ToolCall{}
	thinkingBlocks := []entity.ThinkingBlock{}

	for _, content := range response.Content {
		switch content.Type {
		case "text":
			contentBuilder.WriteString(content.Text)
		case "tool_use":
			toolID := content.ID
			toolName := content.Name
			inputMap := make(map[string]interface{})

			var thoughtSignature string
			if content.JSON.Signature.Valid() {
				sigRaw := content.JSON.Signature.Raw()
				if sigRaw != "" {
					thoughtSignature = sigRaw
				}
			}

			if content.JSON.Data.Valid() {
				dataRaw := content.JSON.Data.Raw()
				if dataRaw != "" && thoughtSignature == "" {
					thoughtSignature = dataRaw
				}
			}

			if len(content.Input) > 0 {
				if err := json.Unmarshal(content.Input, &inputMap); err == nil {
					inputJSON := string(content.Input)
					toolCalls = append(toolCalls, port.ToolCallInfo{
						ToolID:           toolID,
						ToolName:         toolName,
						Input:            inputMap,
						InputJSON:        inputJSON,
						ThoughtSignature: thoughtSignature,
					})
					entityToolCalls = append(entityToolCalls, entity.ToolCall{
						ToolID:           toolID,
						ToolName:         toolName,
						Input:            inputMap,
						ThoughtSignature: thoughtSignature,
					})
				}
			}
		case "thinking", "redacted_thinking":
			thinkingContent := content.Thinking
			if thinkingContent == "" && content.Type == "redacted_thinking" {
				thinkingContent = "[Thinking content is encrypted and cannot be displayed - Gemini extended thinking mode]"
			}

			thinkingBlocks = append(thinkingBlocks, entity.ThinkingBlock{
				Thinking:  thinkingContent,
				Signature: content.Signature,
			})
		}
	}

	content := contentBuilder.String()
	if content == "" {
		content = string(response.StopReason)
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

	msg.InputTokens = response.Usage.InputTokens
	msg.OutputTokens = response.Usage.OutputTokens

	return msg, toolCalls, nil
}

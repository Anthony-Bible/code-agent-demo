// Package ai provides an Anthropic AI adapter that implements the domain AIProvider port.
// It follows hexagonal architecture principles by providing infrastructure-level AI service
// operations with proper error handling and configuration management.
//
// The adapter uses the Anthropic SDK for API communication and supports multiple models
// including Claude models and custom models through the API.
//
// Example usage:
//
//	adapter := ai.NewAnthropicAdapter("hf:zai-org/GLM-4.6")
//	response, err := adapter.SendMessage(ctx, messages, tools)
//	if err != nil {
//		log.Fatal(err)
//	}
package ai

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	client             anthropic.Client
	model              string
	maxTokens          int64
	skillManager       port.SkillManager
	subagentManager    port.SubagentManager
	cachedSystemPrompt string // Cached system prompt to avoid repeated skill discovery
	skillsDiscovered   bool   // Whether skills have been discovered at least once
}

// NewAnthropicAdapter creates a new AnthropicAdapter with the specified model.
// If the model is empty, a default error will be returned when SendMessage is called.
//
// Parameters:
//   - model: The AI model to use (e.g., "hf:zai-org/GLM-4.6", "claude-3-5-sonnet-20241022")
//   - maxTokens: Maximum tokens for AI response
//   - skillManager: Optional skill manager for providing skill metadata to the system prompt
//   - subagentManager: Optional subagent manager for providing subagent metadata to the system prompt
//
// Returns:
//   - port.AIProvider: An implementation of the AIProvider interface
func NewAnthropicAdapter(
	model string,
	maxTokens int64,
	skillManager port.SkillManager,
	subagentManager port.SubagentManager,
) port.AIProvider {
	return &AnthropicAdapter{
		client:          anthropic.NewClient(),
		model:           model,
		maxTokens:       maxTokens,
		skillManager:    skillManager,
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
//
// Returns:
//   - *entity.Message: The AI's response including any tool use blocks
//   - []port.ToolCallInfo: Information about tools requested by the AI
//   - error: An error if the request fails or validation fails
func (a *AnthropicAdapter) SendMessage(
	ctx context.Context,
	messages []port.MessageParam,
	tools []port.ToolParam,
) (*entity.Message, []port.ToolCallInfo, error) {
	// Validate inputs
	if len(messages) == 0 {
		return nil, nil, ErrEmptyMessages
	}
	if a.model == "" {
		return nil, nil, ErrModelNotSet
	}

	// Convert port messages to Anthropic SDK messages
	anthropicMessages := a.convertMessages(messages)

	// Convert port tools to Anthropic SDK tools
	anthropicTools := a.convertTools(tools)

	// Get system prompt (may be modified if plan mode is active, includes skill metadata)
	systemPrompt := a.getSystemPrompt(ctx)

	// Build thinking config from context
	thinkingConfig := anthropic.ThinkingConfigParamUnion{OfDisabled: &anthropic.ThinkingConfigDisabledParam{}}
	if thinkingInfo, ok := port.ThinkingModeFromContext(ctx); ok && thinkingInfo.Enabled {
		thinkingConfig = anthropic.ThinkingConfigParamOfEnabled(thinkingInfo.BudgetTokens)
	}

	// Call Anthropic API
	response, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: a.maxTokens,
		Messages:  anthropicMessages,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Thinking:  thinkingConfig,
		Tools:     anthropicTools,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Convert response to domain Message and extract tool info
	return a.convertResponse(response)
}

// SendMessageStreaming sends a message to the Anthropic API with streaming support.
// It calls the provided callback for each text chunk as it arrives from the API.
//
// The method accumulates the full message while streaming and handles both text content
// and tool use blocks. The callback is only called for text deltas, not tool use blocks.
//
// Parameters:
//   - ctx: Context for the request (supports cancellation and timeout)
//   - messages: Slice of MessageParam representing the conversation history
//   - tools: Slice of ToolParam representing available tools for the AI
//   - callback: Function called for each text chunk as it arrives
//
// Returns:
//   - *entity.Message: The complete AI response including any tool use blocks
//   - []port.ToolCallInfo: Information about tools requested by the AI
//   - error: An error if the request fails or validation fails
func (a *AnthropicAdapter) SendMessageStreaming(
	ctx context.Context,
	messages []port.MessageParam,
	tools []port.ToolParam,
	callback port.StreamCallback,
) (*entity.Message, []port.ToolCallInfo, error) {
	// Validate inputs
	if len(messages) == 0 {
		return nil, nil, ErrEmptyMessages
	}
	if a.model == "" {
		return nil, nil, ErrModelNotSet
	}

	// Convert port messages to Anthropic SDK messages
	anthropicMessages := a.convertMessages(messages)

	// Convert port tools to Anthropic SDK tools
	anthropicTools := a.convertTools(tools)

	// Get system prompt (may be modified if plan mode is active, includes skill metadata)
	systemPrompt := a.getSystemPrompt(ctx)

	// Build thinking config from context
	thinkingConfig := anthropic.ThinkingConfigParamUnion{OfDisabled: &anthropic.ThinkingConfigDisabledParam{}}
	if thinkingInfo, ok := port.ThinkingModeFromContext(ctx); ok && thinkingInfo.Enabled {
		thinkingConfig = anthropic.ThinkingConfigParamOfEnabled(thinkingInfo.BudgetTokens)
	}

	// Create streaming request
	stream := a.client.Messages.NewStreaming(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: a.maxTokens,
		Messages:  anthropicMessages,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Thinking:  thinkingConfig,
		Tools:     anthropicTools,
	})

	// Accumulate the message as events arrive
	message := anthropic.Message{}
	for stream.Next() {
		event := stream.Current()
		err := message.Accumulate(event)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to accumulate event: %w", err)
		}

		// Handle text deltas for streaming display
		eventVariant, ok := event.AsAny().(anthropic.ContentBlockDeltaEvent)
		if !ok {
			continue
		}

		deltaVariant, ok := eventVariant.Delta.AsAny().(anthropic.TextDelta)
		if !ok {
			continue
		}

		if callback != nil {
			if err := callback(deltaVariant.Text); err != nil {
				return nil, nil, fmt.Errorf("stream callback error: %w", err)
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

// getSystemPrompt returns the system prompt for the AI based on context priority.
//
// Priority order (highest to lowest):
//  1. Custom system prompt (from CustomSystemPromptFromContext) - Takes precedence over all other prompts
//  2. Plan mode prompt (from PlanModeFromContext) - Used when plan mode is active and no custom prompt exists
//  3. Base prompt with optional skill metadata - Default prompt when no special modes are active
//
// The custom prompt feature allows callers to override the system prompt entirely
// for specialized tasks like code review, refactoring, or investigations.
func (a *AnthropicAdapter) getSystemPrompt(ctx context.Context) string {
	// Priority 1: Check for custom system prompt (highest priority)
	if customPromptInfo, ok := port.CustomSystemPromptFromContext(ctx); ok && customPromptInfo.Prompt != "" {
		return customPromptInfo.Prompt
	}

	// Priority 2: Check for plan mode prompt (second priority)
	planInfo, ok := port.PlanModeFromContext(ctx)
	if ok && planInfo.Enabled {
		return a.buildPlanModePrompt(planInfo)
	}

	// Priority 3: Return base prompt with optional skill metadata (default/fallback)
	return a.buildBasePromptWithSkills()
}

// buildPlanModePrompt constructs the specialized plan mode system prompt.
// This prompt instructs the agent to explore the codebase and write an implementation
// plan rather than making direct changes.
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

// buildBasePromptWithSkills constructs the base system prompt with optional skill metadata.
// If a skill manager is available, it includes available skills in the prompt
// following the agentskills.io specification format.
// The system prompt is cached after first discovery to avoid repeated filesystem scans.
func (a *AnthropicAdapter) buildBasePromptWithSkills() string {
	basePrompt := "You are an AI assistant that helps users with code editing and explanations. Use the available tools when necessary to provide accurate and helpful responses."

	// If no skill manager, return base prompt
	if a.skillManager == nil {
		return basePrompt
	}

	// Return cached prompt if skills have already been discovered
	if a.skillsDiscovered && a.cachedSystemPrompt != "" {
		return a.cachedSystemPrompt
	}

	// Try to discover skills (only done once per adapter instance)
	skills, err := a.skillManager.DiscoverSkills(context.Background())
	a.skillsDiscovered = true // Mark as discovered even on error to avoid retries

	if err != nil || len(skills.Skills) == 0 {
		a.cachedSystemPrompt = basePrompt
		return basePrompt
	}

	// Build skills section following agentskills.io XML specification
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n<available_skills>\n")

	for _, skill := range skills.Skills {
		sb.WriteString("  <skill>\n")
		sb.WriteString(fmt.Sprintf("    <name>%s</name>\n", skill.Name))
		sb.WriteString(fmt.Sprintf("    <description>%s</description>\n", skill.Description))
		if skill.DirectoryPath != "" {
			location := skill.DirectoryPath
			if absDir, err := filepath.Abs(skill.DirectoryPath); err == nil {
				location = absDir
			}
			sb.WriteString(fmt.Sprintf("    <location>%s</location>\n", filepath.Join(location, "SKILL.md")))
		}
		sb.WriteString("  </skill>\n")
	}

	sb.WriteString("</available_skills>\n\n")
	sb.WriteString(
		"Use the `activate_skill` tool to load the full content of a skill when its capabilities are needed for the task at hand.",
	)

	// Add subagents section if subagent manager is available
	if a.subagentManager != nil {
		agents, err := a.subagentManager.DiscoverAgents(context.Background())
		if err == nil && agents.TotalCount > 0 {
			sb.WriteString("\n\n<available_subagents>\n")
			sb.WriteString("Use the 'task' tool to delegate work to these specialized agents:\n")
			for _, agent := range agents.Subagents {
				sb.WriteString("  <agent>\n")
				sb.WriteString(fmt.Sprintf("    <name>%s</name>\n", agent.Name))
				sb.WriteString(fmt.Sprintf("    <description>%s</description>\n", agent.Description))
				sb.WriteString("  </agent>\n")
			}
			sb.WriteString("</available_subagents>\n")
		}
	}

	a.cachedSystemPrompt = sb.String()
	return a.cachedSystemPrompt
}

// GenerateToolSchema returns an empty tool input schema.
// In a more complex implementation, this could generate schemas based on
// registered tool definitions or configuration.
//
// For the Anthropic adapter, tool schemas are typically defined per-tool
// and passed directly in the SendMessage call.
//
// Returns:
//   - port.ToolInputSchemaParam: An empty schema map
func (a *AnthropicAdapter) GenerateToolSchema() port.ToolInputSchemaParam {
	return port.ToolInputSchemaParam{}
}

// HealthCheck performs a basic health check on the Anthropic adapter.
// It validates that the client is properly initialized and ready to accept requests.
//
// Parameters:
//   - ctx: Context for the health check (supports cancellation)
//
// Returns:
//   - error: nil if the health check passes, otherwise an error
func (a *AnthropicAdapter) HealthCheck(_ context.Context) error {
	// Basic health check - verify model is configured
	if a.model == "" {
		return fmt.Errorf("%w: model not configured", ErrClientHealthCheck)
	}
	return nil
}

// SetModel sets the AI model to use for subsequent requests.
//
// Parameters:
//   - model: The model identifier to use (e.g., "claude-3-5-sonnet-20241022")
//
// Returns:
//   - error: nil if the model was set successfully
func (a *AnthropicAdapter) SetModel(model string) error {
	if model == "" {
		return errors.New("model cannot be empty")
	}
	a.model = model
	return nil
}

// GetModel returns the currently configured AI model.
//
// Returns:
//   - string: The current model identifier
func (a *AnthropicAdapter) GetModel() string {
	return a.model
}

// convertMessages converts port MessageParam slice to Anthropic SDK MessageParam slice.
// It handles tool use blocks for assistant messages and tool result blocks for user messages.
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

		// Handle thought_signature from Gemini via Bifrost
		if tr.ThoughtSignature != "" {
			// SECURITY NOTE: The thought_signature is a cryptographic signature from Gemini AI
			// that validates the authenticity of tool execution results. It should be:
			// 1. Validated to ensure it hasn't been tampered with
			// 2. Preserved across tool calls to maintain chain of custody
			// 3. Injected at the HTTP level (not via SDK due to limitations)
			//
			// The signature format is currently opaque to this adapter and is passed through
			// as-is. Future implementation should include:
			// - Signature validation logic
			// - HTTP interceptor for proper injection
			// - Error handling for invalid signatures
			fmt.Fprintf(
				os.Stderr,
				"[AnthropicAdapter] Tool result has thought_signature (need HTTP-level injection): ToolID=%s, Sig=%s\n",
				tr.ToolID,
				tr.ThoughtSignature,
			)
			// TODO: The SDK doesn't support adding signature to tool_result blocks
			// We need to implement an HTTP interceptor to inject the signature field
			// into the JSON payload before sending to Bifrost and validate signature format
		}
	}
	return anthropic.NewUserMessage(resultBlocks...)
}

// convertAssistantToolMessage converts an assistant message with thinking blocks, text, and tool calls.
// CRITICAL: Thinking blocks MUST come first in the content array.
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
		// If thought_signature is present (Gemini via Bifrost), include it
		// The SDK doesn't expose Signature field, so we use the standard method
		// and rely on Bifrost to handle signature preservation at the HTTP level
		blocks = append(blocks, anthropic.NewToolUseBlock(tc.ToolID, tc.Input, tc.ToolName))

		// Log if we have a thought_signature (for debugging Bifrost integration)
		if tc.ThoughtSignature != "" {
			fmt.Fprintf(
				os.Stderr,
				"[AnthropicAdapter] Tool call has thought_signature (need HTTP-level injection): ID=%s, Sig=%s\n",
				tc.ToolID,
				tc.ThoughtSignature,
			)
			// TODO: Implement HTTP interceptor to inject signature field into JSON payload
			// The SDK doesn't support adding custom fields to tool_use blocks
			// For now, we'll need to intercept the HTTP request and inject the signature
		}
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

// extractStringField extracts a string value from a schema map, returning empty string if not found.
func extractStringField(schema port.ToolInputSchemaParam, key string) string {
	if value, ok := schema[key].(string); ok {
		return value
	}
	return ""
}

// extractMapField extracts a map value from a schema map, returning nil if not found.
func extractMapField(schema port.ToolInputSchemaParam, key string) map[string]interface{} {
	if value, ok := schema[key].(map[string]interface{}); ok {
		return value
	}
	return nil
}

// extractStringSliceField extracts a string slice from a schema map, returning nil if not found.
func extractStringSliceField(schema port.ToolInputSchemaParam, key string) []string {
	if value, ok := schema[key].([]string); ok {
		return value
	}
	return nil
}

// convertResponse converts an Anthropic API response to a domain Message entity.
// It extracts text content and tool use blocks from the response and constructs
// a simplified content string for the domain Message.
func (a *AnthropicAdapter) convertResponse(response *anthropic.Message) (*entity.Message, []port.ToolCallInfo, error) {
	// Build the content string from all content blocks
	var contentBuilder strings.Builder
	toolCalls := []port.ToolCallInfo{}
	entityToolCalls := []entity.ToolCall{}
	thinkingBlocks := []entity.ThinkingBlock{}

	for _, content := range response.Content {
		switch content.Type {
		case "text":
			contentBuilder.WriteString(content.Text)
		case "tool_use":
			// Extract tool ID, name, and input
			toolID := content.ID
			toolName := content.Name
			inputMap := make(map[string]interface{})

			// Extract thought_signature from Signature field (Gemini via Bifrost)
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

			// Convert Input JSON to map
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
			// Extract thinking blocks with signatures (preserve signature exactly)
			// Note: Gemini sends "redacted_thinking" with encrypted content in the "data" field
			// that cannot be decrypted client-side. We show a placeholder instead.
			thinkingContent := content.Thinking

			// If thinking is empty but this is a redacted_thinking block, use a placeholder
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

	// Create the message
	msg, err := entity.NewMessage(entity.RoleAssistant, content)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create message: %w", err)
	}

	// Store tool calls in the message entity so they persist in conversation history
	if len(entityToolCalls) > 0 {
		msg.ToolCalls = entityToolCalls
	}

	// Store thinking blocks in the message entity (CRITICAL: signatures preserved exactly)
	if len(thinkingBlocks) > 0 {
		msg.ThinkingBlocks = thinkingBlocks
	}

	return msg, toolCalls, nil
}

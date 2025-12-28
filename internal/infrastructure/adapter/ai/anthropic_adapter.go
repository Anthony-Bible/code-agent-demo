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
	skillManager       port.SkillManager
	cachedSystemPrompt string // Cached system prompt to avoid repeated skill discovery
	skillsDiscovered   bool   // Whether skills have been discovered at least once
}

// NewAnthropicAdapter creates a new AnthropicAdapter with the specified model.
// If the model is empty, a default error will be returned when SendMessage is called.
//
// Parameters:
//   - model: The AI model to use (e.g., "hf:zai-org/GLM-4.6", "claude-3-5-sonnet-20241022")
//   - skillManager: Optional skill manager for providing skill metadata to the system prompt
//
// Returns:
//   - port.AIProvider: An implementation of the AIProvider interface
func NewAnthropicAdapter(model string, skillManager port.SkillManager) port.AIProvider {
	return &AnthropicAdapter{
		client:       anthropic.NewClient(),
		model:        model,
		skillManager: skillManager,
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

	// Call Anthropic API
	response, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: int64(4096),
		Messages:  anthropicMessages,
		System:    []anthropic.TextBlockParam{{Text: systemPrompt}},
		Thinking:  anthropic.ThinkingConfigParamUnion{OfDisabled: &anthropic.ThinkingConfigDisabledParam{}},
		Tools:     anthropicTools,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Convert response to domain Message and extract tool info
	return a.convertResponse(response)
}

// getSystemPrompt returns the system prompt for the AI.
// If plan mode is active in the context, it returns a specialized prompt
// that instructs the agent to write a markdown implementation plan.
// Otherwise, it returns a base prompt with optional skill metadata.
func (a *AnthropicAdapter) getSystemPrompt(ctx context.Context) string {
	basePrompt := a.buildBasePromptWithSkills()

	planInfo, ok := port.PlanModeFromContext(ctx)
	if !ok || !planInfo.Enabled {
		return basePrompt
	}

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
		switch {
		case msg.Role == entity.RoleUser && len(msg.ToolResults) > 0:
			// Build tool result blocks for user messages
			resultBlocks := make([]anthropic.ContentBlockParamUnion, len(msg.ToolResults))
			for j, tr := range msg.ToolResults {
				resultBlocks[j] = anthropic.NewToolResultBlock(tr.ToolID, tr.Result, tr.IsError)
			}
			result[i] = anthropic.NewUserMessage(resultBlocks...)
		case msg.Role == entity.RoleAssistant && len(msg.ToolCalls) > 0:
			// Build assistant message with tool use blocks
			blocks := []anthropic.ContentBlockParamUnion{}
			if msg.Content != "" {
				blocks = append(blocks, anthropic.NewTextBlock(msg.Content))
			}
			for _, tc := range msg.ToolCalls {
				blocks = append(blocks, anthropic.NewToolUseBlock(tc.ToolID, tc.Input, tc.ToolName))
			}
			result[i] = anthropic.NewAssistantMessage(blocks...)
		default:
			// Simple text message (backward compatible)
			if msg.Role == entity.RoleAssistant {
				result[i] = anthropic.NewAssistantMessage(anthropic.NewTextBlock(msg.Content))
			} else {
				// Default to user message for roles like "user" and "system"
				result[i] = anthropic.NewUserMessage(anthropic.NewTextBlock(msg.Content))
			}
		}
	}
	return result
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

	for _, content := range response.Content {
		switch content.Type {
		case "text":
			contentBuilder.WriteString(content.Text)
		case "tool_use":
			// Extract tool ID, name, and input
			toolID := content.ID
			toolName := content.Name
			inputMap := make(map[string]interface{})

			// Convert Input JSON to map
			if len(content.Input) > 0 {
				if err := json.Unmarshal(content.Input, &inputMap); err == nil {
					inputJSON := string(content.Input)
					toolCalls = append(toolCalls, port.ToolCallInfo{
						ToolID:    toolID,
						ToolName:  toolName,
						Input:     inputMap,
						InputJSON: inputJSON,
					})
					// Populate entity tool calls for storage in Message
					entityToolCalls = append(entityToolCalls, entity.ToolCall{
						ToolID:   toolID,
						ToolName: toolName,
						Input:    inputMap,
					})
				}
			}
		case "thinking":
			// Thinking blocks are optional
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

	return msg, toolCalls, nil
}

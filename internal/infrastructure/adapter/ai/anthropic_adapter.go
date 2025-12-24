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
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
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
	client anthropic.Client
	model  string
}

// NewAnthropicAdapter creates a new AnthropicAdapter with the specified model.
// If the model is empty, a default error will be returned when SendMessage is called.
//
// Parameters:
//   - model: The AI model to use (e.g., "hf:zai-org/GLM-4.6", "claude-3-5-sonnet-20241022")
//
// Returns:
//   - port.AIProvider: An implementation of the AIProvider interface
func NewAnthropicAdapter(model string) port.AIProvider {
	return &AnthropicAdapter{
		client: anthropic.NewClient(),
		model:  model,
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

	// Call Anthropic API
	response, err := a.client.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(a.model),
		MaxTokens: int64(4096),
		Messages:  anthropicMessages,
		Thinking:  anthropic.ThinkingConfigParamUnion{OfDisabled: &anthropic.ThinkingConfigDisabledParam{}},
		Tools:     anthropicTools,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to send message: %w", err)
	}

	// Convert response to domain Message and extract tool info
	return a.convertResponse(response)
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
		// Build properties map for the input schema
		properties := make(map[string]interface{})

		if tool.InputSchema != nil {
			for key, val := range tool.InputSchema {
				properties[key] = val
			}
		}

		result[i] = anthropic.ToolUnionParam{
			OfTool: &anthropic.ToolParam{
				Name:        tool.Name,
				Description: anthropic.String(tool.Description),
				InputSchema: anthropic.ToolInputSchemaParam{
					Properties: properties,
				},
			},
		}
	}
	return result
}

// convertResponse converts an Anthropic API response to a domain Message entity.
// It extracts text content and tool use blocks from the response and constructs
// a simplified content string for the domain Message.
func (a *AnthropicAdapter) convertResponse(response *anthropic.Message) (*entity.Message, []port.ToolCallInfo, error) {
	// Build the content string from all content blocks
	var contentBuilder strings.Builder
	toolCalls := []port.ToolCallInfo{}

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

	return msg, toolCalls, nil
}

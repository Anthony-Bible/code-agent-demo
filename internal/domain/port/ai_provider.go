package port

import (
	"code-editing-agent/internal/domain/entity"
	"context"
)

// ThinkingBlockParam represents a thinking block parameter for AI providers.
// It contains the thinking process and an optional signature for verification.
// This type is used in the port layer to transfer thinking block data
// between the domain and infrastructure layers.
type ThinkingBlockParam struct {
	Thinking  string `json:"thinking"`  // The thinking process or reasoning content
	Signature string `json:"signature"` // Optional signature for verification
}

// MessageParam represents a parameter for sending messages to AI providers.
type MessageParam struct {
	Role           string               `json:"role"`
	Content        string               `json:"content"`
	ToolCalls      []ToolCallParam      `json:"tool_calls,omitempty"`
	ToolResults    []ToolResultParam    `json:"tool_results,omitempty"`
	ThinkingBlocks []ThinkingBlockParam `json:"thinking_blocks,omitempty"`
}

// ToolCallParam represents a tool use block in a message parameter.
type ToolCallParam struct {
	ToolID           string                 `json:"tool_id"`
	ToolName         string                 `json:"tool_name"`
	Input            map[string]interface{} `json:"input"`
	ThoughtSignature string                 `json:"thought_signature,omitempty"` // Gemini thought signature via Bifrost
}

// ToolResultParam represents a tool result block in a message parameter.
type ToolResultParam struct {
	ToolID           string `json:"tool_id"`
	Result           string `json:"result"`
	IsError          bool   `json:"is_error"`
	ThoughtSignature string `json:"thought_signature,omitempty"` // Gemini thought signature (via Bifrost)
}

// ToolParam represents a tool parameter for AI providers.
type ToolParam struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

// ToolInputSchemaParam represents a tool input schema parameter.
type ToolInputSchemaParam map[string]interface{}

// ToolCallInfo contains information about a tool that was requested by the AI.
type ToolCallInfo struct {
	ToolID           string                 `json:"tool_id"`                     // The tool identifier from the AI response
	ToolName         string                 `json:"tool_name"`                   // The name of the tool
	Input            map[string]interface{} `json:"input"`                       // The input parameters passed to the tool
	InputJSON        string                 `json:"input_json"`                  // JSON representation of the input
	ThoughtSignature string                 `json:"thought_signature,omitempty"` // Gemini thought signature via Bifrost
}

// StreamCallback is called when streaming text is received from the AI provider.
// It receives chunks of text as they arrive and returns an error if processing fails.
type StreamCallback func(text string) error

// ThinkingCallback is called when streaming thinking content is received from the AI provider.
// It receives chunks of thinking text as they arrive and returns an error if processing fails.
type ThinkingCallback func(thinking string) error

// AIProvider defines the interface for external AI service integration.
// This port represents the outbound dependency to AI services and follows
// hexagonal architecture principles by abstracting AI provider implementations.
type AIProvider interface {
	// SendMessage sends a message to the AI provider with optional tools and returns the response.
	SendMessage(
		ctx context.Context,
		messages []MessageParam,
		tools []ToolParam,
	) (*entity.Message, []ToolCallInfo, error)

	// SendMessageStreaming sends a message to the AI provider with streaming support.
	// The textCallback is called for each chunk of text as it arrives.
	// The thinkingCallback is called for each chunk of thinking content (can be nil to skip).
	// Returns the complete message, tool calls, and any error that occurred.
	SendMessageStreaming(
		ctx context.Context,
		messages []MessageParam,
		tools []ToolParam,
		textCallback StreamCallback,
		thinkingCallback ThinkingCallback,
	) (*entity.Message, []ToolCallInfo, error)

	// GenerateToolSchema generates a tool input schema.
	GenerateToolSchema() ToolInputSchemaParam

	// HealthCheck performs a health check on the AI provider.
	HealthCheck(ctx context.Context) error

	// SetModel sets the AI model to use for requests.
	SetModel(model string) error

	// GetModel returns the currently configured AI model.
	GetModel() string
}

// ConvertEntityThinkingBlockToParam converts an entity.ThinkingBlock to ThinkingBlockParam.
// This function is used when transferring thinking blocks from the domain layer
// to the infrastructure layer (e.g., sending to AI providers).
//
// Parameters:
//   - block: The entity.ThinkingBlock to convert
//
// Returns:
//   - ThinkingBlockParam: The converted parameter representation
func ConvertEntityThinkingBlockToParam(block entity.ThinkingBlock) ThinkingBlockParam {
	return ThinkingBlockParam{
		Thinking:  block.Thinking,
		Signature: block.Signature,
	}
}

// ConvertParamThinkingBlockToEntity converts a ThinkingBlockParam to entity.ThinkingBlock.
// This function is used when transferring thinking blocks from the infrastructure layer
// to the domain layer (e.g., receiving from AI providers).
//
// Parameters:
//   - param: The ThinkingBlockParam to convert
//
// Returns:
//   - entity.ThinkingBlock: The converted entity representation
func ConvertParamThinkingBlockToEntity(param ThinkingBlockParam) entity.ThinkingBlock {
	return entity.ThinkingBlock{
		Thinking:  param.Thinking,
		Signature: param.Signature,
	}
}

// ConvertEntityThinkingBlocksToParams converts a slice of entity.ThinkingBlock to []ThinkingBlockParam.
// This function performs batch conversion of thinking blocks from the domain layer
// to the infrastructure layer. It preserves nil slices (returns nil for nil input)
// to maintain semantic meaning in JSON serialization.
//
// Parameters:
//   - blocks: The slice of entity.ThinkingBlock to convert (can be nil)
//
// Returns:
//   - []ThinkingBlockParam: The converted slice, or nil if input was nil
func ConvertEntityThinkingBlocksToParams(blocks []entity.ThinkingBlock) []ThinkingBlockParam {
	if blocks == nil {
		return nil
	}

	params := make([]ThinkingBlockParam, len(blocks))
	for i, block := range blocks {
		params[i] = ConvertEntityThinkingBlockToParam(block)
	}
	return params
}

// ConvertParamThinkingBlocksToEntities converts a slice of ThinkingBlockParam to []entity.ThinkingBlock.
// This function performs batch conversion of thinking blocks from the infrastructure layer
// to the domain layer. It preserves nil slices (returns nil for nil input)
// to maintain semantic meaning when processing API responses.
//
// Parameters:
//   - params: The slice of ThinkingBlockParam to convert (can be nil)
//
// Returns:
//   - []entity.ThinkingBlock: The converted slice, or nil if input was nil
func ConvertParamThinkingBlocksToEntities(params []ThinkingBlockParam) []entity.ThinkingBlock {
	if params == nil {
		return nil
	}

	blocks := make([]entity.ThinkingBlock, len(params))
	for i, param := range params {
		blocks[i] = ConvertParamThinkingBlockToEntity(param)
	}
	return blocks
}

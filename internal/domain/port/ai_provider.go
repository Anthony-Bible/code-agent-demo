package port

import (
	"code-editing-agent/internal/domain/entity"
	"context"
)

// MessageParam represents a parameter for sending messages to AI providers.
type MessageParam struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToolParam represents a tool parameter for AI providers.
type ToolParam struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"input_schema,omitempty"`
}

// ToolInputSchemaParam represents a tool input schema parameter.
type ToolInputSchemaParam map[string]interface{}

// AIProvider defines the interface for external AI service integration.
// This port represents the outbound dependency to AI services and follows
// hexagonal architecture principles by abstracting AI provider implementations.
type AIProvider interface {
	// SendMessage sends a message to the AI provider with optional tools and returns the response.
	SendMessage(ctx context.Context, messages []MessageParam, tools []ToolParam) (*entity.Message, error)

	// GenerateToolSchema generates a tool input schema.
	GenerateToolSchema() ToolInputSchemaParam

	// HealthCheck performs a health check on the AI provider.
	HealthCheck(ctx context.Context) error

	// SetModel sets the AI model to use for requests.
	SetModel(model string) error

	// GetModel returns the currently configured AI model.
	GetModel() string
}

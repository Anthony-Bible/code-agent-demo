package port

import (
	"code-editing-agent/test/domain/entity"
	"context"
)

// ToolExecutor defines the interface for tool execution and management.
// This port represents the outbound dependency for tool operations and follows
// hexagonal architecture principles by abstracting tool execution implementations.
type ToolExecutor interface {
	// RegisterTool registers a new tool with the executor.
	RegisterTool(tool entity.Tool) error

	// UnregisterTool removes a tool from the executor by name.
	UnregisterTool(name string) error

	// ExecuteTool executes a tool with the given name and input.
	ExecuteTool(ctx context.Context, name string, input interface{}) (string, error)

	// ListTools returns a list of all registered tools.
	ListTools() ([]entity.Tool, error)

	// GetTool retrieves a specific tool by name.
	// Returns the tool and a boolean indicating if it was found.
	GetTool(name string) (entity.Tool, bool)

	// ValidateToolInput validates input for a specific tool.
	ValidateToolInput(name string, input interface{}) error
}

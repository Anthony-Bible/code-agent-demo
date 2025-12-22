package service

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"encoding/json"
	"errors"
)

// ToolService manages the business logic for tool operations.
// It handles tool registration, validation, execution, and state management.
type ToolService struct {
	toolExecutor port.ToolExecutor
	toolRegistry map[string]entity.Tool
}

// NewToolService creates a new instance of ToolService with the required dependencies.
func NewToolService(toolExecutor port.ToolExecutor) (*ToolService, error) {
	if toolExecutor == nil {
		return nil, errors.New("tool executor cannot be nil")
	}

	return &ToolService{
		toolExecutor: toolExecutor,
		toolRegistry: make(map[string]entity.Tool),
	}, nil
}

// RegisterTool registers a new tool with validation and metadata management.
func (ts *ToolService) RegisterTool(ctx context.Context, tool entity.Tool) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	default:
	}

	if err := tool.Validate(); err != nil {
		return err
	}

	if _, exists := ts.toolRegistry[tool.ID]; exists {
		return errors.New("tool already registered")
	}

	if err := ts.toolExecutor.RegisterTool(tool); err != nil {
		return err
	}

	ts.toolRegistry[tool.ID] = tool
	return nil
}

// UnregisterTool removes a tool from the registry by name.
func (ts *ToolService) UnregisterTool(ctx context.Context, name string) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	default:
	}

	if name == "" {
		return errors.New("tool name cannot be empty")
	}

	if _, exists := ts.toolRegistry[name]; !exists {
		return errors.New("tool not found")
	}

	if err := ts.toolExecutor.UnregisterTool(name); err != nil {
		return err
	}

	delete(ts.toolRegistry, name)
	return nil
}

// ExecuteTool executes a tool with comprehensive validation and error handling.
func (ts *ToolService) ExecuteTool(ctx context.Context, name string, input json.RawMessage) (string, error) {
	select {
	case <-ctx.Done():
		return "", context.Canceled
	default:
	}

	if _, exists := ts.toolRegistry[name]; !exists {
		return "", errors.New("tool not found")
	}

	tool := ts.toolRegistry[name]

	if err := tool.ValidateInput(input); err != nil {
		return "", err
	}

	// Parse input to interface for executor
	var inputData interface{}
	if len(input) > 0 {
		if err := json.Unmarshal(input, &inputData); err != nil {
			return "", errors.New("invalid JSON input")
		}
	}

	result, err := ts.toolExecutor.ExecuteTool(ctx, name, inputData)
	if err != nil {
		return "", err
	}

	return result, nil
}

// ListTools returns all registered tools with their metadata.
func (ts *ToolService) ListTools(ctx context.Context) ([]entity.Tool, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
	}

	tools := make([]entity.Tool, 0, len(ts.toolRegistry))
	for _, tool := range ts.toolRegistry {
		tools = append(tools, tool)
	}
	return tools, nil
}

// GetTool retrieves a specific tool by name with validation.
func (ts *ToolService) GetTool(ctx context.Context, name string) (*entity.Tool, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
	}

	if name == "" {
		return nil, errors.New("tool name cannot be empty")
	}

	tool, exists := ts.toolRegistry[name]
	if !exists {
		return nil, errors.New("tool not found")
	}
	return &tool, nil
}

// ValidateToolInput validates tool input against the tool's schema and business rules.
func (ts *ToolService) ValidateToolInput(ctx context.Context, name string, input json.RawMessage) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	default:
	}

	tool, exists := ts.toolRegistry[name]
	if !exists {
		return errors.New("tool not found")
	}

	return tool.ValidateInput(input)
}

// GetToolCount returns the total number of registered tools.
func (ts *ToolService) GetToolCount(ctx context.Context) (int, error) {
	select {
	case <-ctx.Done():
		return 0, context.Canceled
	default:
	}

	return len(ts.toolRegistry), nil
}

// IsToolRegistered checks if a tool is registered by name.
func (ts *ToolService) IsToolRegistered(ctx context.Context, name string) (bool, error) {
	select {
	case <-ctx.Done():
		return false, context.Canceled
	default:
	}

	if name == "" {
		return false, errors.New("tool name cannot be empty")
	}

	_, exists := ts.toolRegistry[name]
	return exists, nil
}

// UpdateTool updates an existing tool's metadata and schema.
func (ts *ToolService) UpdateTool(ctx context.Context, name string, tool entity.Tool) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	default:
	}

	if _, exists := ts.toolRegistry[name]; !exists {
		return errors.New("tool not found")
	}

	if err := tool.Validate(); err != nil {
		return err
	}

	// Unregister old tool and register new one
	if err := ts.toolExecutor.UnregisterTool(name); err != nil {
		return err
	}

	if err := ts.toolExecutor.RegisterTool(tool); err != nil {
		return err
	}

	// Update registry - using ID as key, but name for lookup
	delete(ts.toolRegistry, name)
	ts.toolRegistry[tool.ID] = tool
	return nil
}

// ClearTools removes all tools from the registry.
func (ts *ToolService) ClearTools(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	default:
	}

	// Clear executor tools first
	tools := make([]entity.Tool, 0, len(ts.toolRegistry))
	for _, tool := range ts.toolRegistry {
		tools = append(tools, tool)
	}

	for _, tool := range tools {
		if err := ts.toolExecutor.UnregisterTool(tool.ID); err != nil {
			return err
		}
	}

	// Clear registry
	ts.toolRegistry = make(map[string]entity.Tool)
	return nil
}

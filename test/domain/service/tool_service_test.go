package service

import (
	"code-editing-agent/test/domain/entity"
	"code-editing-agent/test/domain/port"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
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

	// Tool IDs should match for update
	if tool.ID != name {
		return errors.New("tool ID mismatch")
	}

	ts.toolRegistry[name] = tool
	return nil
}

// GetToolsByCategory returns tools that match a specific category pattern in their description.
func (ts *ToolService) GetToolsByCategory(ctx context.Context, category string) ([]entity.Tool, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
	}

	if category == "" {
		return nil, errors.New("category cannot be empty")
	}

	var matchingTools []entity.Tool
	for _, tool := range ts.toolRegistry {
		if strings.Contains(strings.ToUpper(tool.Description), strings.ToUpper(category)) {
			matchingTools = append(matchingTools, tool)
		}
	}

	return matchingTools, nil
}

// --- TESTS for ToolService ---

func TestNewToolService(t *testing.T) {
	t.Run("successful creation", func(t *testing.T) {
		toolExecutor := &mockToolExecutorWithErrors{}
		service, err := NewToolService(toolExecutor)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if service == nil {
			t.Errorf("expected service instance but got nil")
		}
		if service.toolExecutor != toolExecutor {
			t.Errorf("tool executor not set correctly")
		}
		if service.toolRegistry == nil {
			t.Errorf("tool registry not initialized")
		}
		if len(service.toolRegistry) != 0 {
			t.Errorf("expected empty registry but got %d tools", len(service.toolRegistry))
		}
	})

	t.Run("nil tool executor", func(t *testing.T) {
		_, err := NewToolService(nil)

		if err == nil {
			t.Errorf("expected error for nil tool executor")
		}
		if err.Error() != "tool executor cannot be nil" {
			t.Errorf("expected specific error message but got: %v", err)
		}
	})
}

func TestRegisterTool(t *testing.T) {
	toolExecutor := &mockToolExecutorWithErrors{}
	service, _ := NewToolService(toolExecutor)
	ctx := context.Background()

	t.Run("valid tool registration", func(t *testing.T) {
		tool, _ := entity.NewTool("read_file", "Read File", "Reads the contents of a file")
		schema := map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string"},
			},
			"required": []string{"path"},
		}
		tool.AddInputSchema(schema, []string{"path"})

		err := service.RegisterTool(ctx, *tool)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		count, _ := service.GetToolCount(ctx)
		if count != 1 {
			t.Errorf("expected 1 tool but got %d", count)
		}
		registered, _ := service.IsToolRegistered(ctx, "read_file")
		if !registered {
			t.Errorf("tool should be registered")
		}
	})

	t.Run("duplicate tool registration", func(t *testing.T) {
		tool, _ := entity.NewTool("read_file", "Read File", "Reads file contents")

		// Register first time
		_ = service.RegisterTool(ctx, *tool)

		// Register second time
		err := service.RegisterTool(ctx, *tool)

		if err == nil {
			t.Errorf("expected error for duplicate registration")
		}
	})

	t.Run("invalid tool", func(t *testing.T) {
		tool := entity.Tool{} // Invalid empty tool

		err := service.RegisterTool(ctx, tool)

		if err == nil {
			t.Errorf("expected error for invalid tool")
		}
	})

	t.Run("tool executor registration failure", func(t *testing.T) {
		toolExecutor := &mockToolExecutorWithErrors{
			registerError: errors.New("executor registration failed"),
		}
		service, _ := NewToolService(toolExecutor)

		tool, _ := entity.NewTool("test_tool", "Test", "Test tool")
		err := service.RegisterTool(ctx, *tool)

		if err == nil {
			t.Errorf("expected error from executor")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		tool, _ := entity.NewTool("test_tool", "Test", "Test tool")
		err := service.RegisterTool(ctx, *tool)

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context cancellation error but got: %v", err)
		}
	})
}

func TestUnregisterTool(t *testing.T) {
	toolExecutor := &mockToolExecutorWithErrors{}
	service, _ := NewToolService(toolExecutor)
	ctx := context.Background()

	// Register a tool first
	tool, _ := entity.NewTool("test_tool", "Test", "Test tool")
	_ = service.RegisterTool(ctx, *tool)

	t.Run("successful unregistration", func(t *testing.T) {
		err := service.UnregisterTool(ctx, "test_tool")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		count, _ := service.GetToolCount(ctx)
		if count != 0 {
			t.Errorf("expected 0 tools after unregistration but got %d", count)
		}
	})

	t.Run("unregister non-existent tool", func(t *testing.T) {
		err := service.UnregisterTool(ctx, "non_existent")

		if err == nil {
			t.Errorf("expected error for non-existent tool")
		}
	})

	t.Run("empty tool name", func(t *testing.T) {
		err := service.UnregisterTool(ctx, "")

		if err == nil {
			t.Errorf("expected error for empty tool name")
		}
	})
}

func TestExecuteTool(t *testing.T) {
	toolExecutor := &mockToolExecutorWithErrors{
		results: map[string]string{
			"read_file": "file content here",
		},
	}
	service, _ := NewToolService(toolExecutor)
	ctx := context.Background()

	// Register a tool
	tool, _ := entity.NewTool("read_file", "Read File", "Reads file contents")
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{"type": "string"},
		},
		"required": []string{"path"},
	}
	tool.AddInputSchema(schema, []string{"path"})
	_ = service.RegisterTool(ctx, *tool)

	t.Run("successful tool execution", func(t *testing.T) {
		input := json.RawMessage(`{"path": "test.txt"}`)
		result, err := service.ExecuteTool(ctx, "read_file", input)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if result != "file content here" {
			t.Errorf("expected 'file content here' but got: %s", result)
		}
	})

	t.Run("non-existent tool", func(t *testing.T) {
		input := json.RawMessage(`{"path": "test.txt"}`)
		_, err := service.ExecuteTool(ctx, "non_existent", input)

		if err == nil {
			t.Errorf("expected error for non-existent tool")
		}
	})

	t.Run("invalid JSON input", func(t *testing.T) {
		input := json.RawMessage(`invalid json`)
		_, err := service.ExecuteTool(ctx, "read_file", input)

		if err == nil {
			t.Errorf("expected error for invalid JSON")
		}
	})

	t.Run("missing required fields", func(t *testing.T) {
		input := json.RawMessage(`{"wrong_field": "value"}`)
		_, err := service.ExecuteTool(ctx, "read_file", input)

		if err == nil {
			t.Errorf("expected error for missing required fields")
		}
	})

	t.Run("tool executor error", func(t *testing.T) {
		toolExecutor := &mockToolExecutorWithErrors{
			err: errors.New("execution failed"),
		}
		service, _ := NewToolService(toolExecutor)
		tool, _ := entity.NewTool("test_tool", "Test", "Test")
		_ = service.RegisterTool(ctx, *tool)

		input := json.RawMessage(`{}`)
		_, err := service.ExecuteTool(ctx, "test_tool", input)

		if err == nil {
			t.Errorf("expected execution error")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		input := json.RawMessage(`{"path": "test.txt"}`)
		_, err := service.ExecuteTool(ctx, "read_file", input)

		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context cancellation error but got: %v", err)
		}
	})
}

func TestListTools(t *testing.T) {
	toolExecutor := &mockToolExecutorWithErrors{}
	service, _ := NewToolService(toolExecutor)
	ctx := context.Background()

	t.Run("empty tool list", func(t *testing.T) {
		tools, err := service.ListTools(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(tools) != 0 {
			t.Errorf("expected empty list but got %d tools", len(tools))
		}
	})

	t.Run("non-empty tool list", func(t *testing.T) {
		// Register multiple tools
		tools := []*entity.Tool{
			{ID: "1", Name: "tool1", Description: "Tool 1"},
			{ID: "2", Name: "tool2", Description: "Tool 2"},
		}

		for _, tool := range tools {
			tool, _ := entity.NewTool(tool.Name, tool.Name, tool.Description)
			_ = service.RegisterTool(ctx, *tool)
		}

		toolList, err := service.ListTools(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(toolList) != 2 {
			t.Errorf("expected 2 tools but got %d", len(toolList))
		}
	})
}

func TestGetTool(t *testing.T) {
	toolExecutor := &mockToolExecutorWithErrors{}
	service, _ := NewToolService(toolExecutor)
	ctx := context.Background()

	t.Run("get existing tool", func(t *testing.T) {
		tool, _ := entity.NewTool("test_tool", "Test Tool", "A test tool")
		_ = service.RegisterTool(ctx, *tool)

		retrieved, err := service.GetTool(ctx, "test_tool")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if retrieved == nil {
			t.Errorf("expected tool but got nil")
		}
		if retrieved.ID != "test_tool" {
			t.Errorf("expected tool ID 'test_tool' but got: %s", retrieved.ID)
		}
	})

	t.Run("get non-existent tool", func(t *testing.T) {
		_, err := service.GetTool(ctx, "non_existent")

		if err == nil {
			t.Errorf("expected error for non-existent tool")
		}
	})

	t.Run("empty tool name", func(t *testing.T) {
		_, err := service.GetTool(ctx, "")

		if err == nil {
			t.Errorf("expected error for empty tool name")
		}
	})
}

func TestValidateToolInput(t *testing.T) {
	toolExecutor := &mockToolExecutorWithErrors{}
	service, _ := NewToolService(toolExecutor)
	ctx := context.Background()

	// Register a tool with schema
	tool, _ := entity.NewTool("tool_with_schema", "Tool with Schema", "Tool with validation schema")
	schema := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"required_param": map[string]interface{}{"type": "string"},
			"optional_param": map[string]interface{}{"type": "number"},
		},
		"required": []string{"required_param"},
	}
	tool.AddInputSchema(schema, []string{"required_param"})
	_ = service.RegisterTool(ctx, *tool)

	t.Run("valid input", func(t *testing.T) {
		input := json.RawMessage(`{"required_param": "value", "optional_param": 123}`)
		err := service.ValidateToolInput(ctx, "tool_with_schema", input)
		if err != nil {
			t.Errorf("unexpected error for valid input: %v", err)
		}
	})

	t.Run("missing required field", func(t *testing.T) {
		input := json.RawMessage(`{"optional_param": 123}`)
		err := service.ValidateToolInput(ctx, "tool_with_schema", input)

		if err == nil {
			t.Errorf("expected error for missing required field")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		input := json.RawMessage(`invalid json`)
		err := service.ValidateToolInput(ctx, "tool_with_schema", input)

		if err == nil {
			t.Errorf("expected error for invalid JSON")
		}
	})

	t.Run("non-existent tool", func(t *testing.T) {
		input := json.RawMessage(`{"param": "value"}`)
		err := service.ValidateToolInput(ctx, "non_existent", input)

		if err == nil {
			t.Errorf("expected error for non-existent tool")
		}
	})
}

func TestIsToolRegistered(t *testing.T) {
	toolExecutor := &mockToolExecutorWithErrors{}
	service, _ := NewToolService(toolExecutor)
	ctx := context.Background()

	t.Run("registered tool", func(t *testing.T) {
		tool, _ := entity.NewTool("test_tool", "Test", "Test tool")
		_ = service.RegisterTool(ctx, *tool)

		registered, err := service.IsToolRegistered(ctx, "test_tool")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if !registered {
			t.Errorf("expected tool to be registered")
		}
	})

	t.Run("unregistered tool", func(t *testing.T) {
		registered, err := service.IsToolRegistered(ctx, "unregistered")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if registered {
			t.Errorf("expected tool to not be registered")
		}
	})

	t.Run("empty tool name", func(t *testing.T) {
		registered, err := service.IsToolRegistered(ctx, "")

		if err == nil {
			t.Errorf("expected error for empty tool name")
		}
		if registered {
			t.Errorf("expected false for empty tool name")
		}
	})
}

func TestUpdateTool(t *testing.T) {
	toolExecutor := &mockToolExecutorWithErrors{}
	service, _ := NewToolService(toolExecutor)
	ctx := context.Background()

	// Register initial tool
	tool, _ := entity.NewTool("test_tool", "Test Tool", "Original description")
	_ = service.RegisterTool(ctx, *tool)

	t.Run("successful update", func(t *testing.T) {
		updatedTool := *tool
		updatedTool.Description = "Updated description"

		err := service.UpdateTool(ctx, "test_tool", updatedTool)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		retrieved, _ := service.GetTool(ctx, "test_tool")
		if retrieved.Description != "Updated description" {
			t.Errorf("tool was not updated")
		}
	})

	t.Run("update non-existent tool", func(t *testing.T) {
		nonExistentTool := entity.Tool{Name: "new_tool", Description: "New tool"}
		err := service.UpdateTool(ctx, "non_existent", nonExistentTool)

		if err == nil {
			t.Errorf("expected error for non-existent tool")
		}
	})

	t.Run("update with invalid tool", func(t *testing.T) {
		invalidTool := entity.Tool{} // Empty/invalid tool
		err := service.UpdateTool(ctx, "test_tool", invalidTool)

		if err == nil {
			t.Errorf("expected error for invalid tool")
		}
	})
}

func TestGetToolsByCategory(t *testing.T) {
	toolExecutor := &mockToolExecutorWithErrors{}
	service, _ := NewToolService(toolExecutor)
	ctx := context.Background()

	// Register tools with different categories in description
	tools := []struct {
		name        string
		description string
		category    string
	}{
		{"file_reader", "Read file contents - FILE operation", "FILE"},
		{"file_writer", "Write to file - FILE operation", "FILE"},
		{"api_caller", "Call REST API - HTTP operation", "HTTP"},
		{"data_processor", "Process data - COMPUTATION operation", "COMPUTATION"},
	}

	for _, tinfo := range tools {
		tool, _ := entity.NewTool(tinfo.name, tinfo.name, tinfo.description)
		_ = service.RegisterTool(ctx, *tool)
	}

	t.Run("get FILE category tools", func(t *testing.T) {
		fileTools, err := service.GetToolsByCategory(ctx, "FILE")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(fileTools) != 2 {
			t.Errorf("expected 2 FILE tools but got %d", len(fileTools))
		}
	})

	t.Run("get HTTP category tools", func(t *testing.T) {
		httpTools, err := service.GetToolsByCategory(ctx, "HTTP")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(httpTools) != 1 {
			t.Errorf("expected 1 HTTP tool but got %d", len(httpTools))
		}
	})

	t.Run("get non-existent category", func(t *testing.T) {
		tools, err := service.GetToolsByCategory(ctx, "NONEXISTENT")
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(tools) != 0 {
			t.Errorf("expected 0 tools for non-existent category but got %d", len(tools))
		}
	})

	t.Run("empty category", func(t *testing.T) {
		_, err := service.GetToolsByCategory(ctx, "")

		if err == nil {
			t.Errorf("expected error for empty category")
		}
	})
}

func TestGetToolCount(t *testing.T) {
	toolExecutor := &mockToolExecutorWithErrors{}
	service, _ := NewToolService(toolExecutor)
	ctx := context.Background()

	t.Run("initial count", func(t *testing.T) {
		count, err := service.GetToolCount(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("expected initial count 0 but got %d", count)
		}
	})

	t.Run("count after registration", func(t *testing.T) {
		tool1, _ := entity.NewTool("tool1", "Tool 1", "First tool")
		tool2, _ := entity.NewTool("tool2", "Tool 2", "Second tool")

		_ = service.RegisterTool(ctx, *tool1)
		_ = service.RegisterTool(ctx, *tool2)

		count, err := service.GetToolCount(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if count != 2 {
			t.Errorf("expected count 2 but got %d", count)
		}
	})

	t.Run("count after unregistration", func(t *testing.T) {
		_ = service.UnregisterTool(ctx, "tool1")

		count, err := service.GetToolCount(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if count != 1 {
			t.Errorf("expected count 1 but got %d", count)
		}
	})
}

// Extended mock for tool executor with more comprehensive error handling.
type mockToolExecutorWithErrors struct {
	tools         map[string]entity.Tool
	results       map[string]string
	err           error
	registerError error
	executeError  map[string]error
}

func (m *mockToolExecutorWithErrors) RegisterTool(tool entity.Tool) error {
	if m.registerError != nil {
		return m.registerError
	}
	if m.tools == nil {
		m.tools = make(map[string]entity.Tool)
	}
	m.tools[tool.Name] = tool
	return nil
}

func (m *mockToolExecutorWithErrors) UnregisterTool(name string) error {
	if m.tools != nil {
		delete(m.tools, name)
	}
	return nil
}

func (m *mockToolExecutorWithErrors) ExecuteTool(ctx context.Context, name string, input interface{}) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.executeError != nil {
		if err, exists := m.executeError[name]; exists {
			return "", err
		}
	}
	if m.results != nil {
		if result, exists := m.results[name]; exists {
			return result, nil
		}
	}
	return "mock result", nil
}

func (m *mockToolExecutorWithErrors) ListTools() ([]entity.Tool, error) {
	if m.tools == nil {
		return []entity.Tool{}, nil
	}
	tools := make([]entity.Tool, 0, len(m.tools))
	for _, tool := range m.tools {
		tools = append(tools, tool)
	}
	return tools, nil
}

func (m *mockToolExecutorWithErrors) GetTool(name string) (entity.Tool, bool) {
	if m.tools == nil {
		return entity.Tool{}, false
	}
	tool, exists := m.tools[name]
	return tool, exists
}

func (m *mockToolExecutorWithErrors) ValidateToolInput(name string, input interface{}) error {
	return nil
}

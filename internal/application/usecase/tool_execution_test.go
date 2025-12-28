package usecase

import (
	"code-editing-agent/internal/application/dto"
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
	"testing"
)

// mockToolExecutor is a test mock for port.ToolExecutor.
type mockToolExecutor struct {
	tools         map[string]entity.Tool
	executeToolFn func(ctx context.Context, name string, input interface{}) (string, error)
}

func newMockToolExecutor() *mockToolExecutor {
	return &mockToolExecutor{
		tools: make(map[string]entity.Tool),
	}
}

func (m *mockToolExecutor) RegisterTool(tool entity.Tool) error {
	m.tools[tool.Name] = tool
	return nil
}

func (m *mockToolExecutor) UnregisterTool(name string) error {
	delete(m.tools, name)
	return nil
}

func (m *mockToolExecutor) GetTool(name string) (entity.Tool, bool) {
	tool, ok := m.tools[name]
	return tool, ok
}

func (m *mockToolExecutor) ListTools() ([]entity.Tool, error) {
	tools := make([]entity.Tool, 0, len(m.tools))
	for _, tool := range m.tools {
		tools = append(tools, tool)
	}
	return tools, nil
}

func (m *mockToolExecutor) ValidateToolInput(_ string, _ interface{}) error {
	return nil
}

func (m *mockToolExecutor) ExecuteTool(ctx context.Context, name string, input interface{}) (string, error) {
	if m.executeToolFn != nil {
		return m.executeToolFn(ctx, name, input)
	}
	return "ok", nil
}

func TestExecuteToolsInSession_PropagatesSessionIDInContext(t *testing.T) {
	// Arrange: Create a mock that captures the context via pointer
	var capturedCtx *context.Context
	mockExecutor := newMockToolExecutor()

	// Register a test tool
	testTool, _ := entity.NewTool("test-id", "test_tool", "A test tool")
	mockExecutor.RegisterTool(*testTool)

	// Set up the capture function - use pointer to capture context from closure
	mockExecutor.executeToolFn = func(ctx context.Context, _ string, _ interface{}) (string, error) {
		capturedCtx = &ctx
		return "success", nil
	}

	uc, err := NewToolExecutionUseCase(mockExecutor)
	if err != nil {
		t.Fatalf("Failed to create use case: %v", err)
	}

	// Act: Execute tool in session
	sessionID := "test-session-123"
	tools := []dto.ToolExecuteRequest{{ToolName: "test_tool", Input: map[string]interface{}{}}}

	_, err = uc.ExecuteToolsInSession(context.Background(), sessionID, tools)
	if err != nil {
		t.Fatalf("ExecuteToolsInSession failed: %v", err)
	}

	// Assert: Verify session ID was propagated in context
	if capturedCtx == nil {
		t.Fatal("Expected context to be captured, but it was nil")
	}
	extractedID, ok := port.SessionIDFromContext(*capturedCtx)
	if !ok {
		t.Error("Expected session ID to be in context, but it was not found")
	}
	if extractedID != sessionID {
		t.Errorf("Expected session ID %q, got %q", sessionID, extractedID)
	}
}

func TestExecuteToolsInSession_EmptySessionID(t *testing.T) {
	mockExecutor := newMockToolExecutor()
	uc, _ := NewToolExecutionUseCase(mockExecutor)

	tools := []dto.ToolExecuteRequest{{ToolName: "test_tool", Input: nil}}

	_, err := uc.ExecuteToolsInSession(context.Background(), "", tools)
	if err == nil {
		t.Error("Expected error for empty session ID, got nil")
	}
}

func TestExecuteToolsInSession_EmptyToolList(t *testing.T) {
	mockExecutor := newMockToolExecutor()
	uc, _ := NewToolExecutionUseCase(mockExecutor)

	_, err := uc.ExecuteToolsInSession(context.Background(), "session-id", nil)
	if err == nil {
		t.Error("Expected error for empty tool list, got nil")
	}
}

func TestNewToolExecutionUseCase_NilExecutor(t *testing.T) {
	_, err := NewToolExecutionUseCase(nil)
	if err == nil {
		t.Error("Expected error for nil executor, got nil")
	}
	if !errors.Is(err, ErrToolExecutorRequired) {
		t.Errorf("Expected ErrToolExecutorRequired, got %v", err)
	}
}

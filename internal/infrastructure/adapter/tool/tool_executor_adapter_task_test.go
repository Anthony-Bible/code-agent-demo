// Package tool contains tests for the task tool functionality.
//
// The task tool enables agents to spawn subagents for delegating work to specialized
// agents with specific capabilities. These tests verify:
//   - Tool registration and schema validation
//   - SetSubagentUseCase configuration
//   - executeTask execution flow
//   - Input validation and error handling
//   - Recursion prevention (subagents cannot spawn subagents)
//   - JSON result formatting
package tool

import (
	"code-editing-agent/internal/application/usecase"
	"code-editing-agent/internal/domain/port"
	"code-editing-agent/internal/infrastructure/adapter/file"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Mock SubagentUseCase
// =============================================================================

// MockSubagentUseCase is a mock implementation of SubagentUseCaseInterface for testing.
type MockSubagentUseCase struct {
	SpawnSubagentFunc        func(ctx context.Context, agentName string, prompt string) (*usecase.SubagentResult, error)
	SpawnDynamicSubagentFunc func(ctx context.Context, config usecase.DynamicSubagentConfig, taskPrompt string) (*usecase.SubagentResult, error)
}

func (m *MockSubagentUseCase) SpawnSubagent(
	ctx context.Context,
	agentName string,
	prompt string,
) (*usecase.SubagentResult, error) {
	if m.SpawnSubagentFunc != nil {
		return m.SpawnSubagentFunc(ctx, agentName, prompt)
	}
	return &usecase.SubagentResult{Status: "completed"}, nil
}

func (m *MockSubagentUseCase) SpawnDynamicSubagent(
	ctx context.Context,
	config usecase.DynamicSubagentConfig,
	taskPrompt string,
) (*usecase.SubagentResult, error) {
	if m.SpawnDynamicSubagentFunc != nil {
		return m.SpawnDynamicSubagentFunc(ctx, config, taskPrompt)
	}
	return &usecase.SubagentResult{Status: "completed"}, nil
}

// =============================================================================
// Tool Registration Tests
// =============================================================================

func TestTaskTool_RegisteredInDefaultTools(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Act
	tools, err := adapter.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	// Assert
	found := false
	for _, tool := range tools {
		if tool.Name == "task" {
			found = true
			break
		}
	}

	if !found {
		t.Error("task tool should be registered in default tools")
	}
}

func TestTaskTool_HasCorrectSchema(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Act
	tool, exists := adapter.GetTool("task")

	// Assert
	if !exists {
		t.Fatal("task tool should exist")
	}

	if tool.ID != "task" {
		t.Errorf("Expected tool ID 'task', got %q", tool.ID)
	}

	if tool.Name != "task" {
		t.Errorf("Expected tool name 'task', got %q", tool.Name)
	}

	if tool.Description == "" {
		t.Error("Task tool description should not be empty")
	}

	// Verify required fields
	expectedRequiredFields := []string{"agent_name", "prompt"}
	if len(tool.RequiredFields) != len(expectedRequiredFields) {
		t.Errorf("Expected %d required fields, got %d", len(expectedRequiredFields), len(tool.RequiredFields))
	}

	for _, field := range expectedRequiredFields {
		found := false
		for _, reqField := range tool.RequiredFields {
			if reqField == field {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Required field %q not found in task tool schema", field)
		}
	}
}

func TestTaskTool_AppearsInListTools(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Act
	tools, err := adapter.ListTools()
	// Assert
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	found := false
	for _, tool := range tools {
		if tool.Name == "task" {
			found = true
			// Verify tool properties
			if tool.ID != "task" {
				t.Errorf("Expected tool ID 'task', got %q", tool.ID)
			}
			if len(tool.RequiredFields) == 0 {
				t.Error("Task tool should have required fields")
			}
			break
		}
	}

	if !found {
		t.Error("task tool should appear in ListTools() output")
	}
}

// =============================================================================
// SetSubagentUseCase Tests
// =============================================================================

func TestExecutorAdapter_SetSubagentUseCase_StoresUseCase(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)
	mockUseCase := &MockSubagentUseCase{}

	// Act
	adapter.SetSubagentUseCase(mockUseCase)

	// Assert - verify by attempting to use the task tool
	// If use case is set, it should not error with "use case not available"
	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "test prompt",
	}
	inputJSON, _ := json.Marshal(input)

	// We expect it to fail because the mock returns nil, but not with "use case not available"
	_, err := adapter.ExecuteTool(context.Background(), "task", string(inputJSON))
	if err != nil && strings.Contains(err.Error(), "use case not available") {
		t.Error("SetSubagentUseCase should store the use case")
	}
}

func TestExecutorAdapter_SetSubagentUseCase_MultipleCallsUpdate(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	callCount := 0
	firstUseCase := &MockSubagentUseCase{
		SpawnSubagentFunc: func(_ context.Context, _ string, _ string) (*usecase.SubagentResult, error) {
			callCount++
			return &usecase.SubagentResult{Status: "first"}, nil
		},
	}

	secondUseCase := &MockSubagentUseCase{
		SpawnSubagentFunc: func(_ context.Context, _ string, _ string) (*usecase.SubagentResult, error) {
			callCount++
			return &usecase.SubagentResult{Status: "second"}, nil
		},
	}

	// Act - set first use case
	adapter.SetSubagentUseCase(firstUseCase)

	// Act - set second use case (should replace first)
	adapter.SetSubagentUseCase(secondUseCase)

	// Execute task tool
	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "test",
	}
	inputJSON, _ := json.Marshal(input)
	result, err := adapter.ExecuteTool(context.Background(), "task", string(inputJSON))
	// Assert - should use second use case
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	if !strings.Contains(result, "second") {
		t.Error("SetSubagentUseCase should update to the second use case")
	}

	if callCount != 1 {
		t.Errorf("Expected 1 call to second use case, got %d total calls", callCount)
	}
}

// =============================================================================
// ExecuteTool Task Tests
// =============================================================================

func TestExecutorAdapter_ExecuteTool_TaskSuccess(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCase{
		SpawnSubagentFunc: func(_ context.Context, agentName string, _ string) (*usecase.SubagentResult, error) {
			return &usecase.SubagentResult{
				SubagentID:   "test-id-123",
				AgentName:    agentName,
				Status:       "completed",
				Output:       "Task completed successfully",
				ActionsTaken: 5,
				Duration:     100 * time.Millisecond,
			}, nil
		},
	}
	adapter.SetSubagentUseCase(mockUseCase)

	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "Do something important",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	result, err := adapter.ExecuteTool(context.Background(), "task", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify result contains expected data
	if !strings.Contains(result, "completed") {
		t.Errorf("Expected result to contain 'completed', got: %s", result)
	}

	if !strings.Contains(result, "test-id-123") {
		t.Errorf("Expected result to contain subagent ID, got: %s", result)
	}

	// Verify it's valid JSON
	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resultMap); err != nil {
		t.Errorf("Result should be valid JSON: %v", err)
	}
}

func TestExecutorAdapter_ExecuteTool_TaskEmptyAgentName(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCase{
		SpawnSubagentFunc: func(_ context.Context, _ string, _ string) (*usecase.SubagentResult, error) {
			t.Error("SpawnSubagent should not be called with empty agent_name")
			return nil, errors.New("should not be called")
		},
	}
	adapter.SetSubagentUseCase(mockUseCase)

	input := map[string]interface{}{
		"agent_name": "",
		"prompt":     "test prompt",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(context.Background(), "task", string(inputJSON))

	// Assert
	if err == nil {
		t.Error("Expected error for empty agent_name, got nil")
	}

	if !strings.Contains(err.Error(), "agent_name") {
		t.Errorf("Error should mention agent_name, got: %v", err)
	}
}

func TestExecutorAdapter_ExecuteTool_TaskEmptyPrompt(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCase{
		SpawnSubagentFunc: func(_ context.Context, _ string, _ string) (*usecase.SubagentResult, error) {
			t.Error("SpawnSubagent should not be called with empty prompt")
			return nil, errors.New("should not be called")
		},
	}
	adapter.SetSubagentUseCase(mockUseCase)

	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(context.Background(), "task", string(inputJSON))

	// Assert
	if err == nil {
		t.Error("Expected error for empty prompt, got nil")
	}

	if !strings.Contains(err.Error(), "prompt") {
		t.Errorf("Error should mention prompt, got: %v", err)
	}
}

func TestExecutorAdapter_ExecuteTool_TaskSubagentUseCaseError(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	expectedError := errors.New("subagent execution failed")
	mockUseCase := &MockSubagentUseCase{
		SpawnSubagentFunc: func(_ context.Context, _ string, _ string) (*usecase.SubagentResult, error) {
			return nil, expectedError
		},
	}
	adapter.SetSubagentUseCase(mockUseCase)

	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "test prompt",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(context.Background(), "task", string(inputJSON))

	// Assert
	if err == nil {
		t.Error("Expected error to propagate from SubagentUseCase, got nil")
	}

	if !strings.Contains(err.Error(), "subagent execution failed") {
		t.Errorf("Error should propagate from use case, got: %v", err)
	}
}

func TestExecutorAdapter_ExecuteTool_TaskResultFormattedAsJSON(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCase{
		SpawnSubagentFunc: func(_ context.Context, _ string, _ string) (*usecase.SubagentResult, error) {
			return &usecase.SubagentResult{
				SubagentID:   "test-123",
				AgentName:    "test-agent",
				Status:       "completed",
				Output:       "Done",
				ActionsTaken: 3,
				Duration:     50 * time.Millisecond,
			}, nil
		},
	}
	adapter.SetSubagentUseCase(mockUseCase)

	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "test",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	result, err := adapter.ExecuteTool(context.Background(), "task", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Verify result is valid JSON
	var resultMap map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resultMap); err != nil {
		t.Errorf("Result should be valid JSON, got parse error: %v", err)
	}

	// Verify JSON contains expected fields
	expectedFields := []string{"subagent_id", "agent_name", "status", "output"}
	for _, field := range expectedFields {
		if _, exists := resultMap[field]; !exists {
			t.Errorf("Result JSON should contain field %q", field)
		}
	}
}

func TestExecutorAdapter_ExecuteTool_TaskRecursionBlockedInSubagentContext(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCase{
		SpawnSubagentFunc: func(_ context.Context, _ string, _ string) (*usecase.SubagentResult, error) {
			t.Error("SpawnSubagent should not be called in subagent context (recursion prevention)")
			return nil, errors.New("should not be called")
		},
	}
	adapter.SetSubagentUseCase(mockUseCase)

	// Create subagent context
	subagentCtx := port.WithSubagentContext(context.Background(), port.SubagentContextInfo{
		SubagentID: "parent-subagent",
		IsSubagent: true,
	})

	input := map[string]interface{}{
		"agent_name": "child-agent",
		"prompt":     "nested task",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(subagentCtx, "task", string(inputJSON))

	// Assert
	if err == nil {
		t.Error("Expected error for task tool in subagent context (recursion prevention), got nil")
	}

	if !strings.Contains(err.Error(), "recursion") && !strings.Contains(err.Error(), "blocked") {
		t.Errorf("Error should mention recursion or blocked, got: %v", err)
	}
}

// =============================================================================
// Integration Tests
// =============================================================================

func TestExecutorAdapter_TaskTool_EndToEnd(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Track calls to verify execution flow
	var capturedAgentName, capturedPrompt string
	mockUseCase := &MockSubagentUseCase{
		SpawnSubagentFunc: func(_ context.Context, agentName string, prompt string) (*usecase.SubagentResult, error) {
			capturedAgentName = agentName
			capturedPrompt = prompt
			return &usecase.SubagentResult{
				SubagentID:   "end-to-end-test",
				AgentName:    agentName,
				Status:       "completed",
				Output:       "Integration test passed",
				ActionsTaken: 10,
				Duration:     200 * time.Millisecond,
			}, nil
		},
	}
	adapter.SetSubagentUseCase(mockUseCase)

	input := map[string]interface{}{
		"agent_name": "integration-agent",
		"prompt":     "Run integration test",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	result, err := adapter.ExecuteTool(context.Background(), "task", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("End-to-end test failed: %v", err)
	}

	// Verify inputs were passed correctly
	if capturedAgentName != "integration-agent" {
		t.Errorf("Expected agent_name 'integration-agent', got %q", capturedAgentName)
	}

	if capturedPrompt != "Run integration test" {
		t.Errorf("Expected prompt 'Run integration test', got %q", capturedPrompt)
	}

	// Verify result structure
	if !strings.Contains(result, "completed") {
		t.Errorf("Result should contain 'completed', got: %s", result)
	}

	if !strings.Contains(result, "Integration test passed") {
		t.Errorf("Result should contain output, got: %s", result)
	}
}

func TestExecutorAdapter_TaskTool_MultipleSequentialExecutions(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	executionCount := 0
	mockUseCase := &MockSubagentUseCase{
		SpawnSubagentFunc: func(_ context.Context, agentName string, _ string) (*usecase.SubagentResult, error) {
			executionCount++
			return &usecase.SubagentResult{
				SubagentID:   "exec-" + string(rune(executionCount)),
				AgentName:    agentName,
				Status:       "completed",
				Output:       "Execution " + string(rune('0'+executionCount)),
				ActionsTaken: executionCount,
				Duration:     time.Duration(executionCount*10) * time.Millisecond,
			}, nil
		},
	}
	adapter.SetSubagentUseCase(mockUseCase)

	// Act - execute task tool 3 times sequentially
	for i := 1; i <= 3; i++ {
		input := map[string]interface{}{
			"agent_name": "sequential-agent",
			"prompt":     "Task " + string(rune('0'+i)),
		}
		inputJSON, _ := json.Marshal(input)

		result, err := adapter.ExecuteTool(context.Background(), "task", string(inputJSON))
		if err != nil {
			t.Fatalf("Execution %d failed: %v", i, err)
		}

		if !strings.Contains(result, "completed") {
			t.Errorf("Execution %d should return completed status", i)
		}
	}

	// Assert
	if executionCount != 3 {
		t.Errorf("Expected 3 sequential executions, got %d", executionCount)
	}
}

func TestExecutorAdapter_TaskTool_UnavailableIfUseCaseNotSet(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)
	// NOTE: Do NOT set SubagentUseCase

	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "test prompt",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(context.Background(), "task", string(inputJSON))

	// Assert
	if err == nil {
		t.Error("Expected error when SubagentUseCase is not set, got nil")
	}

	if !strings.Contains(err.Error(), "not available") && !strings.Contains(err.Error(), "not set") {
		t.Errorf("Error should indicate use case not available, got: %v", err)
	}
}

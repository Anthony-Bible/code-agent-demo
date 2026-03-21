package tool

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/application/usecase"
	"github.com/anthony-bible/code-agent-demo/internal/infrastructure/adapter/file"
)

// =============================================================================
// Tool Executor Subagent Propagation Tests
// =============================================================================

// MockSubagentUseCaseWithConfig is a mock that captures the parameters passed to spawning methods.
type MockSubagentUseCaseWithConfig struct {
	// Captured from SpawnSubagent calls
	CapturedContext     context.Context
	CapturedAgentName   string
	CapturedPrompt      string
	SpawnSubagentError  error
	SpawnSubagentResult *usecase.SubagentResult

	// Captured from SpawnDynamicSubagent calls
	CapturedDynamicContext context.Context
	CapturedDynamicConfig  usecase.DynamicSubagentConfig
	CapturedDynamicPrompt  string
	SpawnDynamicError      error
	SpawnDynamicResult     *usecase.SubagentResult
}

func (m *MockSubagentUseCaseWithConfig) SpawnSubagent(
	ctx context.Context,
	agentName string,
	prompt string,
) (*usecase.SubagentResult, error) {
	m.CapturedContext = ctx
	m.CapturedAgentName = agentName
	m.CapturedPrompt = prompt

	if m.SpawnSubagentError != nil {
		return nil, m.SpawnSubagentError
	}

	if m.SpawnSubagentResult != nil {
		return m.SpawnSubagentResult, nil
	}

	return &usecase.SubagentResult{
		SubagentID:   "test-subagent",
		AgentName:    agentName,
		Status:       "completed",
		Output:       "Test output",
		ActionsTaken: 1,
		Duration:     10 * time.Millisecond,
	}, nil
}

func (m *MockSubagentUseCaseWithConfig) SpawnDynamicSubagent(
	ctx context.Context,
	config usecase.DynamicSubagentConfig,
	taskPrompt string,
) (*usecase.SubagentResult, error) {
	m.CapturedDynamicContext = ctx
	m.CapturedDynamicConfig = config
	m.CapturedDynamicPrompt = taskPrompt

	if m.SpawnDynamicError != nil {
		return nil, m.SpawnDynamicError
	}

	if m.SpawnDynamicResult != nil {
		return m.SpawnDynamicResult, nil
	}

	return &usecase.SubagentResult{
		SubagentID:   "test-dynamic-subagent",
		AgentName:    config.Name,
		Status:       "completed",
		Output:       "Dynamic test output",
		ActionsTaken: 1,
		Duration:     10 * time.Millisecond,
	}, nil
}

func TestTaskTool_PropagatesParametersCorrectly(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCaseWithConfig{}
	adapter.SetSubagentUseCase(mockUseCase)

	ctx := context.Background()
	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "test task prompt",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(ctx, "task", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	if mockUseCase.CapturedAgentName != "test-agent" {
		t.Errorf("Expected agent_name 'test-agent', got '%s'", mockUseCase.CapturedAgentName)
	}

	if mockUseCase.CapturedPrompt != "test task prompt" {
		t.Errorf("Expected prompt 'test task prompt', got '%s'", mockUseCase.CapturedPrompt)
	}
}

func TestDelegateTool_PropagatesParametersCorrectly(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCaseWithConfig{}
	adapter.SetSubagentUseCase(mockUseCase)

	ctx := context.Background()
	input := map[string]interface{}{
		"name":          "dynamic-agent",
		"system_prompt": "You are a test agent",
		"task":          "Do something",
		"model":         "haiku",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(ctx, "delegate", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	if mockUseCase.CapturedDynamicConfig.Name != "dynamic-agent" {
		t.Errorf("Expected dynamic name 'dynamic-agent', got '%s'", mockUseCase.CapturedDynamicConfig.Name)
	}

	if mockUseCase.CapturedDynamicConfig.SystemPrompt != "You are a test agent" {
		t.Errorf("Expected system prompt 'You are a test agent', got '%s'", mockUseCase.CapturedDynamicConfig.SystemPrompt)
	}

	if mockUseCase.CapturedDynamicPrompt != "Do something" {
		t.Errorf("Expected task prompt 'Do something', got '%s'", mockUseCase.CapturedDynamicPrompt)
	}

	if mockUseCase.CapturedDynamicConfig.Model != "haiku" {
		t.Errorf("Expected model 'haiku', got '%s'", mockUseCase.CapturedDynamicConfig.Model)
	}
}

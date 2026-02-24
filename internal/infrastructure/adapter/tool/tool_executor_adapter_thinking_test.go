// Package tool contains tests for thinking mode propagation to subagents.
//
// RED PHASE: These tests verify the expected behavior of thinking mode propagation
// from parent to subagent. They test that executeTask() and executeDelegate() should
// extract thinking mode from the parent's context and pass it through to SubagentRunner.Run().
//
// Current behavior: The context is passed through to SubagentUseCase.SpawnSubagent/SpawnDynamicSubagent,
// but SubagentRunner.Run() currently reads thinking config from its static r.config field (set at
// initialization) instead of extracting it from the context parameter.
//
// Expected behavior: SubagentRunner.Run() should extract thinking mode from ctx using
// port.ThinkingModeFromContext(ctx) and use those values to configure the subagent's thinking mode,
// overriding the static config when present in context.
//
// Test coverage:
//   - Task tool: context with thinking enabled should propagate to subagent
//   - Delegate tool: context with thinking enabled should propagate to subagent
//   - Various thinking configurations (enabled/disabled, different budgets, show/hide)
//   - Default behavior when no thinking config in context
//   - Edge cases (zero budget, max budget)
//
// These tests currently PASS because the context is being passed through correctly.
// However, they demonstrate what SHOULD happen when SubagentRunner.Run() is updated
// to extract thinking mode from context. The real failing test would be to verify
// that SubagentRunner sets up thinking mode based on context values, which requires
// testing at the SubagentRunner level, not here.
package tool

import (
	"code-editing-agent/internal/application/usecase"
	"code-editing-agent/internal/domain/port"
	"code-editing-agent/internal/infrastructure/adapter/file"
	"context"
	"encoding/json"
	"testing"
	"time"
)

// =============================================================================
// Mock SubagentUseCase for Thinking Mode Tests
// =============================================================================

// MockSubagentUseCaseWithConfig is a mock that captures the config passed to spawning methods.
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
	// Capture the context and parameters for inspection
	m.CapturedContext = ctx
	m.CapturedAgentName = agentName
	m.CapturedPrompt = prompt

	if m.SpawnSubagentError != nil {
		return nil, m.SpawnSubagentError
	}

	if m.SpawnSubagentResult != nil {
		return m.SpawnSubagentResult, nil
	}

	// Default result
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
	// Capture the context, config, and parameters for inspection
	m.CapturedDynamicContext = ctx
	m.CapturedDynamicConfig = config
	m.CapturedDynamicPrompt = taskPrompt

	if m.SpawnDynamicError != nil {
		return nil, m.SpawnDynamicError
	}

	if m.SpawnDynamicResult != nil {
		return m.SpawnDynamicResult, nil
	}

	// Default result
	return &usecase.SubagentResult{
		SubagentID:   "test-dynamic-subagent",
		AgentName:    config.Name,
		Status:       "completed",
		Output:       "Dynamic test output",
		ActionsTaken: 1,
		Duration:     10 * time.Millisecond,
	}, nil
}

// =============================================================================
// Task Tool - Thinking Config Propagation Tests
// =============================================================================

func TestTaskTool_PropagatesThinkingEnabled_WhenParentHasThinkingEnabled(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCaseWithConfig{}
	adapter.SetSubagentUseCase(mockUseCase)

	// Create context with thinking enabled
	thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 5000,
		ShowThinking: true,
	})

	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "test task",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(thinkingCtx, "task", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Verify thinking mode was extracted from context and passed to subagent
	capturedThinking, hasThinking := port.ThinkingModeFromContext(mockUseCase.CapturedContext)
	if !hasThinking {
		t.Error("Expected thinking mode to be present in context passed to SpawnSubagent")
	}

	if !capturedThinking.Enabled {
		t.Error("Expected ThinkingEnabled to be true when parent has thinking enabled")
	}

	if capturedThinking.BudgetTokens != 5000 {
		t.Errorf("Expected ThinkingBudget to be 5000, got %d", capturedThinking.BudgetTokens)
	}

	if !capturedThinking.ShowThinking {
		t.Error("Expected ShowThinking to be true when parent has show thinking enabled")
	}
}

func TestTaskTool_PropagatesThinkingDisabled_WhenParentHasThinkingDisabled(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCaseWithConfig{}
	adapter.SetSubagentUseCase(mockUseCase)

	// Create context with thinking disabled
	thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      false,
		BudgetTokens: 0,
		ShowThinking: false,
	})

	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "test task",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(thinkingCtx, "task", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Verify thinking mode was extracted from context
	capturedThinking, hasThinking := port.ThinkingModeFromContext(mockUseCase.CapturedContext)
	if !hasThinking {
		t.Error("Expected thinking mode to be present in context passed to SpawnSubagent")
	}

	if capturedThinking.Enabled {
		t.Error("Expected ThinkingEnabled to be false when parent has thinking disabled")
	}

	if capturedThinking.BudgetTokens != 0 {
		t.Errorf("Expected ThinkingBudget to be 0, got %d", capturedThinking.BudgetTokens)
	}

	if capturedThinking.ShowThinking {
		t.Error("Expected ShowThinking to be false when parent has thinking disabled")
	}
}

func TestTaskTool_UsesDefaultThinking_WhenNoThinkingInContext(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCaseWithConfig{}
	adapter.SetSubagentUseCase(mockUseCase)

	// Use plain context without thinking mode
	plainCtx := context.Background()

	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "test task",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(plainCtx, "task", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Verify default thinking behavior (disabled)
	capturedThinking, hasThinking := port.ThinkingModeFromContext(mockUseCase.CapturedContext)

	// When there's no thinking in context, we expect the context to not have thinking mode
	// OR if it does have it (due to defaults), it should be disabled
	if hasThinking {
		if capturedThinking.Enabled {
			t.Error("Expected ThinkingEnabled to be false (default) when parent has no thinking in context")
		}
		if capturedThinking.BudgetTokens != 0 {
			t.Errorf("Expected ThinkingBudget to be 0 (default), got %d", capturedThinking.BudgetTokens)
		}
		if capturedThinking.ShowThinking {
			t.Error("Expected ShowThinking to be false (default) when parent has no thinking in context")
		}
	}
}

func TestTaskTool_PropagatesThinkingBudget_VariousValues(t *testing.T) {
	tests := []struct {
		name         string
		budgetTokens int64
	}{
		{
			name:         "Zero budget (unlimited)",
			budgetTokens: 0,
		},
		{
			name:         "Small budget",
			budgetTokens: 1000,
		},
		{
			name:         "Large budget",
			budgetTokens: 50000,
		},
		{
			name:         "Very large budget",
			budgetTokens: 1000000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			fileManager := file.NewLocalFileManager(".")
			adapter := NewExecutorAdapter(fileManager)

			mockUseCase := &MockSubagentUseCaseWithConfig{}
			adapter.SetSubagentUseCase(mockUseCase)

			// Create context with specific budget
			thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: tt.budgetTokens,
				ShowThinking: true,
			})

			input := map[string]interface{}{
				"agent_name": "test-agent",
				"prompt":     "test task",
			}
			inputJSON, _ := json.Marshal(input)

			// Act
			_, err := adapter.ExecuteTool(thinkingCtx, "task", string(inputJSON))
			// Assert
			if err != nil {
				t.Fatalf("ExecuteTool failed: %v", err)
			}

			capturedThinking, hasThinking := port.ThinkingModeFromContext(mockUseCase.CapturedContext)
			if !hasThinking {
				t.Fatal("Expected thinking mode to be present in context")
			}

			if capturedThinking.BudgetTokens != tt.budgetTokens {
				t.Errorf("Expected ThinkingBudget to be %d, got %d", tt.budgetTokens, capturedThinking.BudgetTokens)
			}
		})
	}
}

func TestTaskTool_PropagatesShowThinking_WhenEnabled(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCaseWithConfig{}
	adapter.SetSubagentUseCase(mockUseCase)

	// Create context with show thinking enabled
	thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 10000,
		ShowThinking: true,
	})

	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "test task",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(thinkingCtx, "task", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	capturedThinking, hasThinking := port.ThinkingModeFromContext(mockUseCase.CapturedContext)
	if !hasThinking {
		t.Fatal("Expected thinking mode to be present in context")
	}

	if !capturedThinking.ShowThinking {
		t.Error("Expected ShowThinking to be true when parent has show thinking enabled")
	}
}

func TestTaskTool_PropagatesShowThinking_WhenDisabled(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCaseWithConfig{}
	adapter.SetSubagentUseCase(mockUseCase)

	// Create context with show thinking disabled (but thinking enabled)
	thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 10000,
		ShowThinking: false,
	})

	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "test task",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(thinkingCtx, "task", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	capturedThinking, hasThinking := port.ThinkingModeFromContext(mockUseCase.CapturedContext)
	if !hasThinking {
		t.Fatal("Expected thinking mode to be present in context")
	}

	if capturedThinking.ShowThinking {
		t.Error("Expected ShowThinking to be false when parent has show thinking disabled")
	}
}

// =============================================================================
// Delegate Tool - Thinking Config Propagation Tests
// =============================================================================

func TestDelegateTool_PropagatesThinkingEnabled_WhenParentHasThinkingEnabled(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCaseWithConfig{}
	adapter.SetSubagentUseCase(mockUseCase)

	// Create context with thinking enabled
	thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 8000,
		ShowThinking: true,
	})

	input := map[string]interface{}{
		"name":          "dynamic-agent",
		"system_prompt": "You are a test agent",
		"task":          "Do something",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(thinkingCtx, "delegate", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Verify thinking mode was extracted from context and passed to dynamic subagent
	capturedThinking, hasThinking := port.ThinkingModeFromContext(mockUseCase.CapturedDynamicContext)
	if !hasThinking {
		t.Error("Expected thinking mode to be present in context passed to SpawnDynamicSubagent")
	}

	if !capturedThinking.Enabled {
		t.Error("Expected ThinkingEnabled to be true when parent has thinking enabled")
	}

	if capturedThinking.BudgetTokens != 8000 {
		t.Errorf("Expected ThinkingBudget to be 8000, got %d", capturedThinking.BudgetTokens)
	}

	if !capturedThinking.ShowThinking {
		t.Error("Expected ShowThinking to be true when parent has show thinking enabled")
	}
}

func TestDelegateTool_PropagatesThinkingDisabled_WhenParentHasThinkingDisabled(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCaseWithConfig{}
	adapter.SetSubagentUseCase(mockUseCase)

	// Create context with thinking disabled
	thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      false,
		BudgetTokens: 0,
		ShowThinking: false,
	})

	input := map[string]interface{}{
		"name":          "dynamic-agent",
		"system_prompt": "You are a test agent",
		"task":          "Do something",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(thinkingCtx, "delegate", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Verify thinking mode was extracted from context
	capturedThinking, hasThinking := port.ThinkingModeFromContext(mockUseCase.CapturedDynamicContext)
	if !hasThinking {
		t.Error("Expected thinking mode to be present in context passed to SpawnDynamicSubagent")
	}

	if capturedThinking.Enabled {
		t.Error("Expected ThinkingEnabled to be false when parent has thinking disabled")
	}

	if capturedThinking.BudgetTokens != 0 {
		t.Errorf("Expected ThinkingBudget to be 0, got %d", capturedThinking.BudgetTokens)
	}

	if capturedThinking.ShowThinking {
		t.Error("Expected ShowThinking to be false when parent has thinking disabled")
	}
}

func TestDelegateTool_UsesDefaultThinking_WhenNoThinkingInContext(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCaseWithConfig{}
	adapter.SetSubagentUseCase(mockUseCase)

	// Use plain context without thinking mode
	plainCtx := context.Background()

	input := map[string]interface{}{
		"name":          "dynamic-agent",
		"system_prompt": "You are a test agent",
		"task":          "Do something",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(plainCtx, "delegate", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Verify default thinking behavior (disabled)
	capturedThinking, hasThinking := port.ThinkingModeFromContext(mockUseCase.CapturedDynamicContext)

	// When there's no thinking in context, we expect the context to not have thinking mode
	// OR if it does have it (due to defaults), it should be disabled
	if hasThinking {
		if capturedThinking.Enabled {
			t.Error("Expected ThinkingEnabled to be false (default) when parent has no thinking in context")
		}
		if capturedThinking.BudgetTokens != 0 {
			t.Errorf("Expected ThinkingBudget to be 0 (default), got %d", capturedThinking.BudgetTokens)
		}
		if capturedThinking.ShowThinking {
			t.Error("Expected ShowThinking to be false (default) when parent has no thinking in context")
		}
	}
}

func TestDelegateTool_PropagatesThinkingBudget_VariousValues(t *testing.T) {
	tests := []struct {
		name         string
		budgetTokens int64
	}{
		{
			name:         "Zero budget (unlimited)",
			budgetTokens: 0,
		},
		{
			name:         "Small budget",
			budgetTokens: 2000,
		},
		{
			name:         "Large budget",
			budgetTokens: 100000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			fileManager := file.NewLocalFileManager(".")
			adapter := NewExecutorAdapter(fileManager)

			mockUseCase := &MockSubagentUseCaseWithConfig{}
			adapter.SetSubagentUseCase(mockUseCase)

			// Create context with specific budget
			thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: tt.budgetTokens,
				ShowThinking: false,
			})

			input := map[string]interface{}{
				"name":          "dynamic-agent",
				"system_prompt": "You are a test agent",
				"task":          "Do something",
			}
			inputJSON, _ := json.Marshal(input)

			// Act
			_, err := adapter.ExecuteTool(thinkingCtx, "delegate", string(inputJSON))
			// Assert
			if err != nil {
				t.Fatalf("ExecuteTool failed: %v", err)
			}

			capturedThinking, hasThinking := port.ThinkingModeFromContext(mockUseCase.CapturedDynamicContext)
			if !hasThinking {
				t.Fatal("Expected thinking mode to be present in context")
			}

			if capturedThinking.BudgetTokens != tt.budgetTokens {
				t.Errorf("Expected ThinkingBudget to be %d, got %d", tt.budgetTokens, capturedThinking.BudgetTokens)
			}
		})
	}
}

// =============================================================================
// Edge Case Tests
// =============================================================================

func TestTaskTool_ThinkingPropagation_WithAllFlagsEnabled(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCaseWithConfig{}
	adapter.SetSubagentUseCase(mockUseCase)

	// Create context with all thinking features enabled
	thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 25000,
		ShowThinking: true,
	})

	input := map[string]interface{}{
		"agent_name": "test-agent",
		"prompt":     "test task",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(thinkingCtx, "task", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	capturedThinking, hasThinking := port.ThinkingModeFromContext(mockUseCase.CapturedContext)
	if !hasThinking {
		t.Fatal("Expected thinking mode to be present in context")
	}

	// Verify ALL thinking fields are correctly propagated
	if !capturedThinking.Enabled {
		t.Error("Expected ThinkingEnabled to be true")
	}
	if capturedThinking.BudgetTokens != 25000 {
		t.Errorf("Expected ThinkingBudget to be 25000, got %d", capturedThinking.BudgetTokens)
	}
	if !capturedThinking.ShowThinking {
		t.Error("Expected ShowThinking to be true")
	}
}

func TestDelegateTool_ThinkingPropagation_WithAllFlagsEnabled(t *testing.T) {
	// Arrange
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	mockUseCase := &MockSubagentUseCaseWithConfig{}
	adapter.SetSubagentUseCase(mockUseCase)

	// Create context with all thinking features enabled
	thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 30000,
		ShowThinking: true,
	})

	input := map[string]interface{}{
		"name":          "dynamic-agent",
		"system_prompt": "You are a comprehensive test agent",
		"task":          "Do complex work",
	}
	inputJSON, _ := json.Marshal(input)

	// Act
	_, err := adapter.ExecuteTool(thinkingCtx, "delegate", string(inputJSON))
	// Assert
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	capturedThinking, hasThinking := port.ThinkingModeFromContext(mockUseCase.CapturedDynamicContext)
	if !hasThinking {
		t.Fatal("Expected thinking mode to be present in context")
	}

	// Verify ALL thinking fields are correctly propagated
	if !capturedThinking.Enabled {
		t.Error("Expected ThinkingEnabled to be true")
	}
	if capturedThinking.BudgetTokens != 30000 {
		t.Errorf("Expected ThinkingBudget to be 30000, got %d", capturedThinking.BudgetTokens)
	}
	if !capturedThinking.ShowThinking {
		t.Error("Expected ShowThinking to be true")
	}
}

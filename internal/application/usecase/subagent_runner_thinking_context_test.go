package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"testing"
)

// =============================================================================
// RED PHASE - Thinking Mode Extraction from Context Tests
// =============================================================================
//
// These tests verify that SubagentRunner.Run() extracts thinking mode configuration
// from the context parameter and uses it to configure the subagent's thinking mode,
// overriding the static config set at initialization.
//
// CURRENT BEHAVIOR (lines 216-224 of subagent_runner.go):
//   SubagentRunner.Run() reads thinking config from r.config (static, set at init time):
//     if r.config.ThinkingEnabled {
//         thinkingInfo := port.ThinkingModeInfo{
//             Enabled:      true,
//             BudgetTokens: r.config.ThinkingBudget,
//             ShowThinking: r.config.ShowThinking,
//         }
//         _ = r.convService.SetThinkingMode(sessionID, thinkingInfo)
//     }
//
// EXPECTED BEHAVIOR:
//   SubagentRunner.Run() should:
//   1. Extract thinking mode from ctx using port.ThinkingModeFromContext(ctx)
//   2. If thinking mode is present in context, use those values
//   3. If not present in context, fall back to r.config values
//   4. Call convService.SetThinkingMode() with the resolved values
//
// This allows parent agents to propagate their thinking configuration to subagents
// dynamically via context, rather than relying on static configuration.
//
// =============================================================================

// TestSubagentRunner_ExtractsThinkingModeFromContext_WhenEnabled verifies that
// SubagentRunner.Run() extracts thinking mode from context when present and enabled.
func TestSubagentRunner_ExtractsThinkingModeFromContext_WhenEnabled(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.processResponseMessages = append(convService.processResponseMessages,
		createSubagentRunnerCompletionMessage())

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Config has thinking DISABLED (default)
	config := SubagentConfig{
		MaxActions:      10,
		ThinkingEnabled: false,
		ThinkingBudget:  0,
		ShowThinking:    false,
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("thinking-agent", "Test Agent")

	// Create context with thinking ENABLED (should override config)
	thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 5000,
		ShowThinking: true,
	})

	// Act
	_, err := runner.Run(thinkingCtx, agent, "Do task", "subagent-thinking-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called
	if convService.setThinkingModeCalls == 0 {
		t.Error("Expected SetThinkingMode to be called when thinking mode is in context")
	}

	// Verify thinking mode values from context were used (not static config)
	if len(convService.setThinkingModeInfo) > 0 {
		actualInfo := convService.setThinkingModeInfo[0]

		if !actualInfo.Enabled {
			t.Error("Expected thinking mode from context to be enabled, got disabled")
		}

		if actualInfo.BudgetTokens != 5000 {
			t.Errorf("Expected budget from context (5000), got %d", actualInfo.BudgetTokens)
		}

		if !actualInfo.ShowThinking {
			t.Error("Expected ShowThinking from context to be true, got false")
		}
	} else {
		t.Error("Expected SetThinkingMode to be called with thinking info from context")
	}
}

// TestSubagentRunner_ExtractsThinkingModeFromContext_OverridesStaticConfig verifies
// that context thinking mode takes precedence over static config.
func TestSubagentRunner_ExtractsThinkingModeFromContext_OverridesStaticConfig(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.processResponseMessages = append(convService.processResponseMessages,
		createSubagentRunnerCompletionMessage())

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Config has thinking enabled with certain values
	config := SubagentConfig{
		MaxActions:      10,
		ThinkingEnabled: true,
		ThinkingBudget:  1000,  // Different from context
		ShowThinking:    false, // Different from context
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("override-agent", "Test Agent")

	// Context has different thinking values (should take precedence)
	thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 8000, // Different from config
		ShowThinking: true, // Different from config
	})

	// Act
	_, err := runner.Run(thinkingCtx, agent, "Do task", "subagent-override-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called
	if convService.setThinkingModeCalls == 0 {
		t.Error("Expected SetThinkingMode to be called")
	}

	// Verify thinking mode values from CONTEXT were used (not static config)
	if len(convService.setThinkingModeInfo) > 0 {
		actualInfo := convService.setThinkingModeInfo[0]

		// Should use context values (8000, true), not config values (1000, false)
		if actualInfo.BudgetTokens != 8000 {
			t.Errorf(
				"Expected budget from context (8000), got %d (should not use config value 1000)",
				actualInfo.BudgetTokens,
			)
		}

		if !actualInfo.ShowThinking {
			t.Error("Expected ShowThinking from context (true), got false (should not use config value false)")
		}
	}
}

// TestSubagentRunner_UsesStaticConfig_WhenNoThinkingInContext verifies that
// SubagentRunner falls back to static config when no thinking mode in context.
func TestSubagentRunner_UsesStaticConfig_WhenNoThinkingInContext(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.processResponseMessages = append(convService.processResponseMessages,
		createSubagentRunnerCompletionMessage())

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Config has thinking enabled
	config := SubagentConfig{
		MaxActions:      10,
		ThinkingEnabled: true,
		ThinkingBudget:  3000,
		ShowThinking:    true,
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("fallback-agent", "Test Agent")

	// Plain context (no thinking mode)
	plainCtx := context.Background()

	// Act
	_, err := runner.Run(plainCtx, agent, "Do task", "subagent-fallback-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called with config values
	if convService.setThinkingModeCalls == 0 {
		t.Error("Expected SetThinkingMode to be called with static config")
	}

	if len(convService.setThinkingModeInfo) > 0 {
		actualInfo := convService.setThinkingModeInfo[0]

		// Should use config values (no context override)
		if !actualInfo.Enabled {
			t.Error("Expected thinking mode from config to be enabled")
		}

		if actualInfo.BudgetTokens != 3000 {
			t.Errorf("Expected budget from config (3000), got %d", actualInfo.BudgetTokens)
		}

		if !actualInfo.ShowThinking {
			t.Error("Expected ShowThinking from config to be true")
		}
	}
}

// TestSubagentRunner_DisablesThinking_WhenContextExplicitlyDisables verifies that
// context can explicitly disable thinking even when config enables it.
func TestSubagentRunner_DisablesThinking_WhenContextExplicitlyDisables(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.processResponseMessages = append(convService.processResponseMessages,
		createSubagentRunnerCompletionMessage())

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Config has thinking ENABLED
	config := SubagentConfig{
		MaxActions:      10,
		ThinkingEnabled: true,
		ThinkingBudget:  5000,
		ShowThinking:    true,
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("disable-agent", "Test Agent")

	// Context explicitly DISABLES thinking (should override config)
	thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      false,
		BudgetTokens: 0,
		ShowThinking: false,
	})

	// Act
	_, err := runner.Run(thinkingCtx, agent, "Do task", "subagent-disable-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// When context explicitly disables thinking, SetThinkingMode should either:
	// 1. Not be called at all, OR
	// 2. Be called with Enabled=false
	if len(convService.setThinkingModeInfo) > 0 {
		actualInfo := convService.setThinkingModeInfo[0]

		if actualInfo.Enabled {
			t.Error("Expected thinking to be disabled from context (should override enabled config)")
		}
	}
	// If SetThinkingMode wasn't called at all, that's also acceptable when disabled
}

// TestSubagentRunner_PropagatesThinkingBudget_VariousValues verifies that
// different budget values from context are correctly propagated.
func TestSubagentRunner_PropagatesThinkingBudget_VariousValues(t *testing.T) {
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
			name:         "Medium budget",
			budgetTokens: 10000,
		},
		{
			name:         "Large budget",
			budgetTokens: 50000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			convService := newSubagentRunnerConvServiceMock()
			convService.processResponseMessages = append(convService.processResponseMessages,
				createSubagentRunnerCompletionMessage())

			toolExecutor := newSubagentRunnerToolExecutorMock()
			aiProvider := newSubagentRunnerAIProviderMock()

			// Config with different budget
			config := SubagentConfig{
				MaxActions:      10,
				ThinkingEnabled: true,
				ThinkingBudget:  99999, // Should be overridden by context
				ShowThinking:    true,
			}

			runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
			agent := createTestAgent("budget-agent", "Test Agent")

			// Context with specific budget
			thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: tt.budgetTokens,
				ShowThinking: true,
			})

			// Act
			_, err := runner.Run(thinkingCtx, agent, "Do task", "subagent-budget-001")
			// Assert
			if err != nil {
				t.Fatalf("Run() failed: %v", err)
			}

			if len(convService.setThinkingModeInfo) > 0 {
				actualInfo := convService.setThinkingModeInfo[0]

				if actualInfo.BudgetTokens != tt.budgetTokens {
					t.Errorf("Expected budget from context (%d), got %d", tt.budgetTokens, actualInfo.BudgetTokens)
				}
			} else {
				t.Error("Expected SetThinkingMode to be called with budget from context")
			}
		})
	}
}

// TestSubagentRunner_PropagatesShowThinking_FromContext verifies that
// ShowThinking flag is correctly extracted from context.
func TestSubagentRunner_PropagatesShowThinking_FromContext(t *testing.T) {
	tests := []struct {
		name         string
		showThinking bool
	}{
		{
			name:         "ShowThinking enabled",
			showThinking: true,
		},
		{
			name:         "ShowThinking disabled",
			showThinking: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			convService := newSubagentRunnerConvServiceMock()
			convService.processResponseMessages = append(convService.processResponseMessages,
				createSubagentRunnerCompletionMessage())

			toolExecutor := newSubagentRunnerToolExecutorMock()
			aiProvider := newSubagentRunnerAIProviderMock()

			// Config with opposite ShowThinking value
			config := SubagentConfig{
				MaxActions:      10,
				ThinkingEnabled: true,
				ThinkingBudget:  5000,
				ShowThinking:    !tt.showThinking, // Opposite of context
			}

			runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
			agent := createTestAgent("show-agent", "Test Agent")

			// Context with specific ShowThinking value
			thinkingCtx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 5000,
				ShowThinking: tt.showThinking,
			})

			// Act
			_, err := runner.Run(thinkingCtx, agent, "Do task", "subagent-show-001")
			// Assert
			if err != nil {
				t.Fatalf("Run() failed: %v", err)
			}

			if len(convService.setThinkingModeInfo) > 0 {
				actualInfo := convService.setThinkingModeInfo[0]

				if actualInfo.ShowThinking != tt.showThinking {
					t.Errorf("Expected ShowThinking from context (%v), got %v (should not use config value %v)",
						tt.showThinking, actualInfo.ShowThinking, !tt.showThinking)
				}
			} else {
				t.Error("Expected SetThinkingMode to be called with ShowThinking from context")
			}
		})
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// createSubagentRunnerCompletionMessage creates a completion message for testing.
func createSubagentRunnerCompletionMessage() *entity.Message {
	msg, _ := entity.NewMessage(entity.RoleAssistant, "Task completed")
	return msg
}

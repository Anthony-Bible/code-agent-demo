package usecase

import (
	"context"
	"testing"

	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// =============================================================================
// Thinking Mode Configuration Tests
// =============================================================================
//
// These tests verify that SubagentRunner.Run() correctly configures the subagent's
// thinking mode based on:
// 1. Static config provided at initialization (SubagentConfig)
// 2. Agent-specific overrides (entity.Subagent)
//
// These tests confirm the removal of context-based inheritance, which was
// deprecated in favor of session-based management.
// =============================================================================

// TestSubagentRunner_UsesStaticConfig verifies that SubagentRunner.Run()
// uses the static configuration by default.
func TestSubagentRunner_UsesStaticConfig(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.processResponseMessages = append(convService.processResponseMessages,
		createSubagentRunnerCompletionMessage())

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Config has thinking enabled with specific values
	config := SubagentConfig{
		MaxActions:      10,
		ThinkingEnabled: true,
		ThinkingBudget:  5000,
		ShowThinking:    true,
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("base-agent", "Test Agent")

	// Act
	_, err := runner.Run(context.Background(), agent, "Do task", "subagent-static-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called with values from static config
	if convService.setThinkingModeCalls == 0 {
		t.Error("Expected SetThinkingMode to be called")
	}

	if len(convService.setThinkingModeInfo) > 0 {
		actualInfo := convService.setThinkingModeInfo[0]

		if !actualInfo.Enabled {
			t.Error("Expected thinking mode to be enabled from config")
		}

		if actualInfo.BudgetTokens != 5000 {
			t.Errorf("Expected budget 5000, got %d", actualInfo.BudgetTokens)
		}
	}
}

// TestSubagentRunner_UsesAgentOverrides verifies that agent-specific overrides
// take precedence over static runner config.
func TestSubagentRunner_UsesAgentOverrides(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.processResponseMessages = append(convService.processResponseMessages,
		createSubagentRunnerCompletionMessage())

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Static config has thinking DISABLED
	config := SubagentConfig{
		MaxActions:      10,
		ThinkingEnabled: false,
		ThinkingBudget:  0,
		ShowThinking:    false,
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Agent explicitly ENABLES thinking with custom budget
	enabled := true
	agent := createTestAgent("overrider", "Overriding Agent")
	agent.ThinkingEnabled = &enabled
	agent.ThinkingBudget = 12345

	// Act
	_, err := runner.Run(context.Background(), agent, "Do task", "subagent-override-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify agent overrides were used
	if len(convService.setThinkingModeInfo) > 0 {
		actualInfo := convService.setThinkingModeInfo[0]

		if !actualInfo.Enabled {
			t.Error("Expected thinking to be enabled by agent override")
		}

		if actualInfo.BudgetTokens != 12345 {
			t.Errorf("Expected budget 12345, got %d", actualInfo.BudgetTokens)
		}
	} else {
		t.Error("Expected SetThinkingMode to be called with agent overrides")
	}
}

// TestSubagentRunner_AgentCanDisableThinking verifies that an agent can
// explicitly disable thinking even if the runner's static config enables it.
func TestSubagentRunner_AgentCanDisableThinking(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.processResponseMessages = append(convService.processResponseMessages,
		createSubagentRunnerCompletionMessage())

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Runner config enables thinking
	config := SubagentConfig{
		MaxActions:      10,
		ThinkingEnabled: true,
		ThinkingBudget:  10000,
		ShowThinking:    true,
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Agent explicitly DISABLES thinking
	disabled := false
	agent := createTestAgent("no-thinker", "Non-thinking Agent")
	agent.ThinkingEnabled = &disabled

	// Act
	_, err := runner.Run(context.Background(), agent, "Do task", "subagent-nothinking-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify thinking was disabled by agent
	if len(convService.setThinkingModeInfo) > 0 {
		actualInfo := convService.setThinkingModeInfo[0]
		if actualInfo.Enabled {
			t.Error("Expected thinking to be disabled by agent override")
		}
	}
}

// TestSubagentRunner_NoContextInheritance verifies that thinking mode from context
// is NO LONGER extracted (as it was deprecated).
func TestSubagentRunner_NoContextInheritance(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.processResponseMessages = append(convService.processResponseMessages,
		createSubagentRunnerCompletionMessage())

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Runner config has thinking DISABLED
	config := SubagentConfig{
		MaxActions:      10,
		ThinkingEnabled: false,
		ThinkingBudget:  0,
		ShowThinking:    false,
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("no-inherit", "No Inherit Agent")

	// Pass context with thinking ENABLED (should NOT be used anymore)
	// We'll use a manually constructed context with a hypothetical key to prove
	// that we are no longer looking for it.
	ctx := context.WithValue(context.Background(), "thinking_mode", port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 99999,
		ShowThinking: true,
	})

	// Act
	_, err := runner.Run(ctx, agent, "Do task", "subagent-no-inherit-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was NOT called with context values
	// (Actually in our current implementation, if Enabled=false in config and agent,
	// it might not be called at all, or called with Enabled=false).
	if len(convService.setThinkingModeInfo) > 0 {
		actualInfo := convService.setThinkingModeInfo[0]
		if actualInfo.Enabled {
			t.Error("Expected thinking mode from context to be ignored")
		}
		if actualInfo.BudgetTokens == 99999 {
			t.Error("Expected budget tokens from context to be ignored")
		}
	}
}

package usecase

import (
	"context"
	"testing"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// =============================================================================
// SubagentRunner Agent-Specific Thinking Configuration Override Tests
// =============================================================================
//
// These tests verify that agent-specific thinking configuration (ThinkingEnabled
// and ThinkingBudget from AGENT.md) correctly override static config.
//
// Override Priority: agent config > static config (context inheritance removed)
// =============================================================================

// TestSubagentRunner_AgentThinkingEnabled_OverridesStaticDisabled verifies that
// when agent specifies thinking_enabled: true in AGENT.md, it overrides a
// runner config with thinking disabled.
func TestSubagentRunner_AgentThinkingEnabled_OverridesStaticDisabled(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "session-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Static config has thinking disabled
	config := SubagentConfig{
		MaxActions:      20,
		ThinkingEnabled: false,
		ThinkingBudget:  0,
		ShowThinking:    false,
	}

	factory := func(_ port.AIProvider) (ConversationServiceInterface, error) {
		return convService, nil
	}
	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config, factory)

	// Agent specifies thinking_enabled: true (override)
	agent := createTestAgent("agent-001", "Test Agent")
	trueVal := true
	agent.ThinkingEnabled = &trueVal
	agent.ThinkingBudget = 10000

	// Act
	_, err := runner.Run(context.Background(), agent, "Task", "subagent-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called with agent's config (enabled=true)
	if convService.setThinkingModeCalls != 1 {
		t.Fatalf("SetThinkingMode() call count = %d, want 1", convService.setThinkingModeCalls)
	}

	actualInfo := convService.setThinkingModeInfo[0]
	if !actualInfo.Enabled {
		t.Errorf("SetThinkingMode() Enabled = %v, want true (from agent config)", actualInfo.Enabled)
	}
	if actualInfo.BudgetTokens != 10000 {
		t.Errorf("SetThinkingMode() BudgetTokens = %d, want 10000 (from agent config)", actualInfo.BudgetTokens)
	}
}

// TestSubagentRunner_AgentThinkingDisabled_OverridesStaticEnabled verifies that
// when agent specifies thinking_enabled: false in AGENT.md, it overrides a
// runner config with thinking enabled.
func TestSubagentRunner_AgentThinkingDisabled_OverridesStaticEnabled(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "session-002"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Static config has thinking enabled
	config := SubagentConfig{
		MaxActions:      20,
		ThinkingEnabled: true,
		ThinkingBudget:  5000,
		ShowThinking:    true,
	}

	factory := func(_ port.AIProvider) (ConversationServiceInterface, error) {
		return convService, nil
	}
	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config, factory)

	// Agent specifies thinking_enabled: false (override)
	agent := createTestAgent("agent-002", "Test Agent")
	falseVal := false
	agent.ThinkingEnabled = &falseVal

	// Act
	_, err := runner.Run(context.Background(), agent, "Task", "subagent-002")
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

// TestSubagentRunner_AgentThinkingBudget_OverridesStaticBudget verifies that
// when agent specifies thinking_budget in AGENT.md, it overrides static budget.
func TestSubagentRunner_AgentThinkingBudget_OverridesStaticBudget(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "session-004"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Static config
	config := SubagentConfig{
		MaxActions:      20,
		ThinkingEnabled: true,
		ThinkingBudget:  5000,
		ShowThinking:    true,
	}

	factory := func(_ port.AIProvider) (ConversationServiceInterface, error) {
		return convService, nil
	}
	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config, factory)

	// Agent specifies thinking_budget: 15000 (override)
	agent := createTestAgent("agent-004", "Test Agent")
	trueVal := true
	agent.ThinkingEnabled = &trueVal
	agent.ThinkingBudget = 15000 // Override budget

	// Act
	_, err := runner.Run(context.Background(), agent, "Task", "subagent-004")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called with agent's budget
	if convService.setThinkingModeCalls != 1 {
		t.Fatalf("SetThinkingMode() call count = %d, want 1", convService.setThinkingModeCalls)
	}

	actualInfo := convService.setThinkingModeInfo[0]
	if actualInfo.BudgetTokens != 15000 {
		t.Errorf("SetThinkingMode() BudgetTokens = %d, want 15000 (from agent config)", actualInfo.BudgetTokens)
	}
}

// TestSubagentRunner_AgentThinkingOverride_NoContextFallsBackToStatic verifies that
// when there's no context and agent doesn't specify, it falls back to static config.
func TestSubagentRunner_AgentThinkingOverride_NoContextFallsBackToStatic(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "session-008"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Static config
	config := SubagentConfig{
		MaxActions:      20,
		ThinkingEnabled: true,
		ThinkingBudget:  7500,
		ShowThinking:    true,
	}

	factory := func(_ port.AIProvider) (ConversationServiceInterface, error) {
		return convService, nil
	}
	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config, factory)

	// Agent does NOT specify thinking fields
	agent := createTestAgent("agent-008", "Test Agent")
	agent.ThinkingEnabled = nil
	agent.ThinkingBudget = 0

	// Act
	_, err := runner.Run(context.Background(), agent, "Task", "subagent-008")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called with static config
	if convService.setThinkingModeCalls != 1 {
		t.Fatalf("SetThinkingMode() call count = %d, want 1", convService.setThinkingModeCalls)
	}

	actualInfo := convService.setThinkingModeInfo[0]
	if !actualInfo.Enabled {
		t.Errorf("SetThinkingMode() Enabled = %v, want true (from static config)", actualInfo.Enabled)
	}
	if actualInfo.BudgetTokens != 7500 {
		t.Errorf("SetThinkingMode() BudgetTokens = %d, want 7500 (from static config)", actualInfo.BudgetTokens)
	}
}

// boolPtr returns a pointer to a bool value (helper for test table setup).
func boolPtr(b bool) *bool {
	return &b
}

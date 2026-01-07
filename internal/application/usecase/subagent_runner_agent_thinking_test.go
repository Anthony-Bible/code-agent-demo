package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"testing"
)

// =============================================================================
// SubagentRunner Agent-Specific Thinking Configuration Override Tests (RED PHASE)
// =============================================================================
//
// These tests verify that agent-specific thinking configuration (ThinkingEnabled
// and ThinkingBudget from AGENT.md) correctly override context and static config.
//
// Override Priority: agent config > context > static config
//
// EXPECTED TO FAIL: Current implementation does not read/apply agent.ThinkingEnabled
// or agent.ThinkingBudget fields. The integration code is missing.
//
// =============================================================================

// TestSubagentRunner_AgentThinkingEnabled_OverridesContextDisabled verifies that
// when agent specifies thinking_enabled: true in AGENT.md, it overrides a context
// with thinking disabled.
//
// EXPECTED TO FAIL: Agent's ThinkingEnabled field is not consulted.
func TestSubagentRunner_AgentThinkingEnabled_OverridesContextDisabled(t *testing.T) {
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

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Agent specifies thinking_enabled: true (override)
	agent := createTestAgent("agent-001", "Test Agent")
	trueVal := true
	agent.ThinkingEnabled = &trueVal
	agent.ThinkingBudget = 10000

	// Context also has thinking disabled
	ctx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      false,
		BudgetTokens: 0,
		ShowThinking: false,
	})

	// Act
	_, err := runner.Run(ctx, agent, "Task", "subagent-001")
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

// TestSubagentRunner_AgentThinkingDisabled_OverridesContextEnabled verifies that
// when agent specifies thinking_enabled: false in AGENT.md, it overrides a context
// with thinking enabled.
//
// EXPECTED TO FAIL: Agent's ThinkingEnabled field is not consulted.
func TestSubagentRunner_AgentThinkingDisabled_OverridesContextEnabled(t *testing.T) {
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

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Agent specifies thinking_enabled: false (override)
	agent := createTestAgent("agent-002", "Test Agent")
	falseVal := false
	agent.ThinkingEnabled = &falseVal

	// Context has thinking enabled
	ctx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 8000,
		ShowThinking: true,
	})

	// Act
	_, err := runner.Run(ctx, agent, "Task", "subagent-002")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was NOT called (agent disabled thinking)
	if convService.setThinkingModeCalls != 0 {
		t.Errorf(
			"SetThinkingMode() call count = %d, want 0 (agent disabled thinking)",
			convService.setThinkingModeCalls,
		)
	}
}

// TestSubagentRunner_AgentThinkingEnabledNil_InheritsFromContext verifies that
// when agent does not specify thinking_enabled (nil), it inherits from context.
//
// EXPECTED TO PASS: Current implementation uses context when available.
func TestSubagentRunner_AgentThinkingEnabledNil_InheritsFromContext(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "session-003"
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

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Agent does NOT specify thinking_enabled (nil = inherit)
	agent := createTestAgent("agent-003", "Test Agent")
	agent.ThinkingEnabled = nil // Explicitly nil (omitted in AGENT.md)

	// Context has thinking enabled
	ctx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 12000,
		ShowThinking: true,
	})

	// Act
	_, err := runner.Run(ctx, agent, "Task", "subagent-003")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called with context's config
	if convService.setThinkingModeCalls != 1 {
		t.Fatalf("SetThinkingMode() call count = %d, want 1", convService.setThinkingModeCalls)
	}

	actualInfo := convService.setThinkingModeInfo[0]
	if !actualInfo.Enabled {
		t.Errorf("SetThinkingMode() Enabled = %v, want true (from context)", actualInfo.Enabled)
	}
	if actualInfo.BudgetTokens != 12000 {
		t.Errorf("SetThinkingMode() BudgetTokens = %d, want 12000 (from context)", actualInfo.BudgetTokens)
	}
}

// TestSubagentRunner_AgentThinkingBudget_OverridesContextBudget verifies that
// when agent specifies thinking_budget in AGENT.md, it overrides context budget.
//
// EXPECTED TO FAIL: Agent's ThinkingBudget field is not consulted.
func TestSubagentRunner_AgentThinkingBudget_OverridesContextBudget(t *testing.T) {
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

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Agent specifies thinking_budget: 15000 (override)
	agent := createTestAgent("agent-004", "Test Agent")
	trueVal := true
	agent.ThinkingEnabled = &trueVal
	agent.ThinkingBudget = 15000 // Override budget

	// Context has different budget
	ctx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 8000, // Should be overridden by agent's 15000
		ShowThinking: true,
	})

	// Act
	_, err := runner.Run(ctx, agent, "Task", "subagent-004")
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

// TestSubagentRunner_AgentThinkingBudgetZero_InheritsFromContext verifies that
// when agent does not specify thinking_budget (0), it inherits from context.
//
// EXPECTED TO PASS: Current implementation uses context when available.
func TestSubagentRunner_AgentThinkingBudgetZero_InheritsFromContext(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "session-005"
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

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Agent does NOT specify thinking_budget (0 = inherit)
	agent := createTestAgent("agent-005", "Test Agent")
	trueVal := true
	agent.ThinkingEnabled = &trueVal
	agent.ThinkingBudget = 0 // Inherit from context/config

	// Context has specific budget
	ctx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 20000, // Should be used
		ShowThinking: true,
	})

	// Act
	_, err := runner.Run(ctx, agent, "Task", "subagent-005")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called with context's budget
	if convService.setThinkingModeCalls != 1 {
		t.Fatalf("SetThinkingMode() call count = %d, want 1", convService.setThinkingModeCalls)
	}

	actualInfo := convService.setThinkingModeInfo[0]
	if actualInfo.BudgetTokens != 20000 {
		t.Errorf("SetThinkingMode() BudgetTokens = %d, want 20000 (from context)", actualInfo.BudgetTokens)
	}
}

// TestSubagentRunner_AgentThinkingBothFieldsSpecified_OverridesBoth verifies that
// when agent specifies both thinking_enabled and thinking_budget, both override.
//
// EXPECTED TO FAIL: Agent's ThinkingEnabled and ThinkingBudget fields are not consulted.
func TestSubagentRunner_AgentThinkingBothFieldsSpecified_OverridesBoth(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "session-006"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Static config
	config := SubagentConfig{
		MaxActions:      20,
		ThinkingEnabled: false,
		ThinkingBudget:  1000,
		ShowThinking:    false,
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Agent specifies BOTH thinking fields (override both)
	agent := createTestAgent("agent-006", "Test Agent")
	trueVal := true
	agent.ThinkingEnabled = &trueVal // Override: enable thinking
	agent.ThinkingBudget = 25000     // Override: set budget to 25000

	// Context has different values
	ctx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      false,
		BudgetTokens: 5000,
		ShowThinking: false,
	})

	// Act
	_, err := runner.Run(ctx, agent, "Task", "subagent-006")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called with agent's config for both fields
	if convService.setThinkingModeCalls != 1 {
		t.Fatalf("SetThinkingMode() call count = %d, want 1", convService.setThinkingModeCalls)
	}

	actualInfo := convService.setThinkingModeInfo[0]
	if !actualInfo.Enabled {
		t.Errorf("SetThinkingMode() Enabled = %v, want true (from agent config)", actualInfo.Enabled)
	}
	if actualInfo.BudgetTokens != 25000 {
		t.Errorf("SetThinkingMode() BudgetTokens = %d, want 25000 (from agent config)", actualInfo.BudgetTokens)
	}
}

// TestSubagentRunner_AgentThinkingOverride_ShowThinkingPreserved verifies that
// agent overrides do not affect ShowThinking (ShowThinking is not agent-configurable).
//
// EXPECTED TO FAIL: Agent's ThinkingEnabled and ThinkingBudget fields are not consulted.
func TestSubagentRunner_AgentThinkingOverride_ShowThinkingPreserved(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "session-007"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Static config has ShowThinking: true
	config := SubagentConfig{
		MaxActions:      20,
		ThinkingEnabled: false,
		ThinkingBudget:  1000,
		ShowThinking:    true, // This should be preserved
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Agent specifies thinking fields (but NOT ShowThinking)
	agent := createTestAgent("agent-007", "Test Agent")
	trueVal := true
	agent.ThinkingEnabled = &trueVal
	agent.ThinkingBudget = 10000

	// Context has ShowThinking: false
	ctx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      false,
		BudgetTokens: 5000,
		ShowThinking: false, // This should be used (from context/config)
	})

	// Act
	_, err := runner.Run(ctx, agent, "Task", "subagent-007")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called with agent's Enabled/Budget, but context's ShowThinking
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
	// ShowThinking should be inherited from context (not from agent, as agent can't specify it)
	if actualInfo.ShowThinking != false {
		t.Errorf("SetThinkingMode() ShowThinking = %v, want false (from context)", actualInfo.ShowThinking)
	}
}

// TestSubagentRunner_AgentThinkingOverride_PriorityOrder verifies the complete
// override priority order: agent config > context > static config.
//
// EXPECTED TO FAIL: Agent's ThinkingEnabled and ThinkingBudget fields are not consulted.
func TestSubagentRunner_AgentThinkingOverride_PriorityOrder(t *testing.T) {
	tests := []struct {
		name               string
		staticEnabled      bool
		staticBudget       int64
		contextEnabled     bool
		contextBudget      int64
		agentEnabled       *bool
		agentBudget        int64
		expectEnabled      bool
		expectBudget       int64
		expectSetThinkCall bool
	}{
		{
			name:               "agent overrides all (enabled=true)",
			staticEnabled:      false,
			staticBudget:       1000,
			contextEnabled:     false,
			contextBudget:      2000,
			agentEnabled:       boolPtr(true),
			agentBudget:        15000,
			expectEnabled:      true,
			expectBudget:       15000,
			expectSetThinkCall: true,
		},
		{
			name:               "agent overrides all (enabled=false)",
			staticEnabled:      true,
			staticBudget:       5000,
			contextEnabled:     true,
			contextBudget:      10000,
			agentEnabled:       boolPtr(false),
			agentBudget:        0,
			expectEnabled:      false,
			expectBudget:       0,
			expectSetThinkCall: false, // Disabled, so no call
		},
		{
			name:               "agent nil, context overrides static",
			staticEnabled:      false,
			staticBudget:       1000,
			contextEnabled:     true,
			contextBudget:      8000,
			agentEnabled:       nil,
			agentBudget:        0,
			expectEnabled:      true,
			expectBudget:       8000,
			expectSetThinkCall: true,
		},
		{
			name:               "agent only overrides budget, enabled from context",
			staticEnabled:      false,
			staticBudget:       1000,
			contextEnabled:     true,
			contextBudget:      5000,
			agentEnabled:       nil,   // Inherit from context
			agentBudget:        20000, // Override budget
			expectEnabled:      true,  // From context
			expectBudget:       20000, // From agent
			expectSetThinkCall: true,
		},
		{
			name:               "agent only overrides enabled, budget from context",
			staticEnabled:      false,
			staticBudget:       1000,
			contextEnabled:     false,
			contextBudget:      5000,
			agentEnabled:       boolPtr(true), // Override enabled
			agentBudget:        0,             // Inherit budget
			expectEnabled:      true,          // From agent
			expectBudget:       5000,          // From context
			expectSetThinkCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			convService, runner, agent, ctx := setupPriorityOrderTest(t,
				tt.staticEnabled, tt.staticBudget,
				tt.contextEnabled, tt.contextBudget,
				tt.agentEnabled, tt.agentBudget)

			// Act
			_, err := runner.Run(ctx, agent, "Task", "subagent-priority")
			// Assert
			if err != nil {
				t.Fatalf("Run() failed: %v", err)
			}

			verifyThinkingModeCall(t, convService, tt.expectSetThinkCall, tt.expectEnabled, tt.expectBudget)
		})
	}
}

// TestSubagentRunner_AgentThinkingOverride_NoContextFallsBackToStatic verifies that
// when there's no context and agent doesn't specify, it falls back to static config.
//
// EXPECTED TO PASS: Current implementation already handles this case.
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

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Agent does NOT specify thinking fields
	agent := createTestAgent("agent-008", "Test Agent")
	agent.ThinkingEnabled = nil
	agent.ThinkingBudget = 0

	// NO context (use regular context.Background())
	ctx := context.Background()

	// Act
	_, err := runner.Run(ctx, agent, "Task", "subagent-008")
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

// TestSubagentRunner_AgentThinkingOverride_NoContextAgentOverrides verifies that
// when there's no context, agent config overrides static config.
//
// EXPECTED TO FAIL: Agent's ThinkingEnabled and ThinkingBudget fields are not consulted.
func TestSubagentRunner_AgentThinkingOverride_NoContextAgentOverrides(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "session-009"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Static config
	config := SubagentConfig{
		MaxActions:      20,
		ThinkingEnabled: false,
		ThinkingBudget:  1000,
		ShowThinking:    false,
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Agent specifies thinking fields (override static)
	agent := createTestAgent("agent-009", "Test Agent")
	trueVal := true
	agent.ThinkingEnabled = &trueVal
	agent.ThinkingBudget = 30000

	// NO context (use regular context.Background())
	ctx := context.Background()

	// Act
	_, err := runner.Run(ctx, agent, "Task", "subagent-009")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called with agent's config
	if convService.setThinkingModeCalls != 1 {
		t.Fatalf("SetThinkingMode() call count = %d, want 1", convService.setThinkingModeCalls)
	}

	actualInfo := convService.setThinkingModeInfo[0]
	if !actualInfo.Enabled {
		t.Errorf("SetThinkingMode() Enabled = %v, want true (from agent config)", actualInfo.Enabled)
	}
	if actualInfo.BudgetTokens != 30000 {
		t.Errorf("SetThinkingMode() BudgetTokens = %d, want 30000 (from agent config)", actualInfo.BudgetTokens)
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

// boolPtr returns a pointer to a bool value (helper for test table setup).
func boolPtr(b bool) *bool {
	return &b
}

// setupPriorityOrderTest sets up the test environment for priority order tests.
func setupPriorityOrderTest(
	t *testing.T,
	staticEnabled bool,
	staticBudget int64,
	contextEnabled bool,
	contextBudget int64,
	agentEnabled *bool,
	agentBudget int64,
) (*subagentRunnerConvServiceMock, *SubagentRunner, *entity.Subagent, context.Context) {
	t.Helper()

	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "session-priority"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	// Static config
	config := SubagentConfig{
		MaxActions:      20,
		ThinkingEnabled: staticEnabled,
		ThinkingBudget:  staticBudget,
		ShowThinking:    false,
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Agent config
	agent := createTestAgent("agent-priority", "Priority Test Agent")
	agent.ThinkingEnabled = agentEnabled
	agent.ThinkingBudget = agentBudget

	// Context config
	ctx := port.WithThinkingMode(context.Background(), port.ThinkingModeInfo{
		Enabled:      contextEnabled,
		BudgetTokens: contextBudget,
		ShowThinking: false,
	})

	return convService, runner, agent, ctx
}

// verifyThinkingModeCall verifies the SetThinkingMode call expectations.
func verifyThinkingModeCall(
	t *testing.T,
	convService *subagentRunnerConvServiceMock,
	expectSetThinkCall bool,
	expectEnabled bool,
	expectBudget int64,
) {
	t.Helper()

	if !expectSetThinkCall {
		if convService.setThinkingModeCalls != 0 {
			t.Errorf(
				"SetThinkingMode() call count = %d, want 0 (thinking disabled)",
				convService.setThinkingModeCalls,
			)
		}
		return
	}

	if convService.setThinkingModeCalls != 1 {
		t.Errorf("SetThinkingMode() call count = %d, want 1", convService.setThinkingModeCalls)
		return
	}

	actualInfo := convService.setThinkingModeInfo[0]
	if actualInfo.Enabled != expectEnabled {
		t.Errorf("SetThinkingMode() Enabled = %v, want %v", actualInfo.Enabled, expectEnabled)
	}
	if actualInfo.BudgetTokens != expectBudget {
		t.Errorf("SetThinkingMode() BudgetTokens = %d, want %d", actualInfo.BudgetTokens, expectBudget)
	}
}

package usecase

import (
	"context"
	"sync"
	"testing"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// =============================================================================
// SubagentRunner Execution Loop Tests
// =============================================================================
// These tests verify that SubagentRunner properly manages the execution loop,
// including triggering thinking status indicators when thinking mode is enabled.
// =============================================================================

// statusTrackingRunnerMock tracks calls to displayStatus.
type statusTrackingRunnerMock struct {
	mu                 sync.Mutex
	displayStatusCalls int
	lastAgentName      string
	lastStatus         string
}

// Note: We need a way to mock displayStatus in SubagentRunner or
// use a mock UI. Since SubagentRunner uses a private displayStatus method
// that calls its own internal field (which might be a UI adapter),
// we'll check the mock conversation service for state instead.
//
// Actually, SubagentRunner has:
// func (r *SubagentRunner) displayStatus(agentName string, status string, details string)
//
// Let's use the existing mock structure but focus on loop behavior.

// TestSubagentRunner_Loop_TriggersThinkingStatus tests that when thinking mode
// is enabled, the runner correctly fetches thinking mode in each iteration.
func TestSubagentRunner_Loop_TriggersThinkingStatus(t *testing.T) {
	// Setup: Create mocks
	convServiceMock := newContextTrackingConvServiceMock()
	toolExecutorMock := newSubagentRunnerToolExecutorMock()
	aiProviderMock := newSubagentRunnerAIProviderMock()

	// Configure thinking mode to be enabled
	thinkingInfo := port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 10000,
		ShowThinking: true,
	}
	_ = convServiceMock.SetThinkingMode("session-123", thinkingInfo)

	// Configure mock to simulate 2 iterations
	msg1, _ := entity.NewMessage(entity.RoleAssistant, "First response")
	msg2, _ := entity.NewMessage(entity.RoleAssistant, "Final response")

	toolCall1 := []port.ToolCallInfo{
		{ToolID: "call_1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}},
	}
	toolCall2 := []port.ToolCallInfo{} // Completion

	convServiceMock.processResponseMessages = []*entity.Message{msg1, msg2}
	convServiceMock.processResponseToolCalls = [][]port.ToolCallInfo{toolCall1, toolCall2}

	// Create runner with thinking enabled in config
	config := SubagentConfig{
		MaxActions:      10,
		ThinkingEnabled: true,
		ThinkingBudget:  10000,
		ShowThinking:    true,
	}
	runner := NewSubagentRunner(convServiceMock, toolExecutorMock, aiProviderMock, nil, config)

	// Execute: Run a subagent task
	agent := &entity.Subagent{
		Name:       "test-agent",
		RawContent: "Test system prompt",
		Model:      "sonnet",
	}

	ctx := context.Background()
	_, err := runner.Run(ctx, agent, "Test task", "subagent-1")
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify: GetThinkingMode was called before each ProcessAssistantResponse call
	// This confirms the runner is checking thinking state to decide whether to show status.
	if convServiceMock.getThinkingModeCalls < 2 {
		t.Errorf("Expected GetThinkingMode to be called at least twice, got %d", convServiceMock.getThinkingModeCalls)
	}

	// Verify: ProcessAssistantResponse was called twice
	if convServiceMock.processResponseCalls != 2 {
		t.Errorf("Expected 2 ProcessAssistantResponse calls, got %d", convServiceMock.processResponseCalls)
	}
}

// TestSubagentRunner_Loop_RespectsMaxActions verifies that the loop stops
// after reaching the maximum number of actions.
func TestSubagentRunner_Loop_RespectsMaxActions(t *testing.T) {
	// Setup
	convServiceMock := newContextTrackingConvServiceMock()
	toolExecutorMock := newSubagentRunnerToolExecutorMock()
	aiProviderMock := newSubagentRunnerAIProviderMock()

	// Configure mock to always return a tool call (infinite loop if not for MaxActions)
	msg, _ := entity.NewMessage(entity.RoleAssistant, "Keep going")
	toolCall := []port.ToolCallInfo{
		{ToolID: "call", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}},
	}

	// Provide enough messages for 5 iterations
	for range 5 {
		convServiceMock.processResponseMessages = append(convServiceMock.processResponseMessages, msg)
		convServiceMock.processResponseToolCalls = append(convServiceMock.processResponseToolCalls, toolCall)
	}

	// Create runner with MaxActions = 3
	config := SubagentConfig{
		MaxActions: 3,
	}
	runner := NewSubagentRunner(convServiceMock, toolExecutorMock, aiProviderMock, nil, config)

	// Execute
	agent := &entity.Subagent{Name: "test-agent"}
	_, err := runner.Run(context.Background(), agent, "Test task", "subagent-max")
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify: ProcessAssistantResponse was called exactly 3 times
	if convServiceMock.processResponseCalls != 3 {
		t.Errorf("Expected exactly 3 calls (MaxActions), got %d", convServiceMock.processResponseCalls)
	}
}

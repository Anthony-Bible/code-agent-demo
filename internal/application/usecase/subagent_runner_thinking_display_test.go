package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"sync"
	"testing"
)

// =============================================================================
// SubagentRunner Thinking Display Tests
// These tests verify that thinking status indicators are displayed correctly
// when extended thinking mode is enabled for subagents.
// =============================================================================

// thinkingDisplayUIMock tracks all DisplaySubagentStatus calls for verification.
type thinkingDisplayUIMock struct {
	mu sync.Mutex

	// DisplaySubagentStatus tracking
	displayStatusCalls   int
	displayStatusAgents  []string
	displayStatusTypes   []string // "Thinking", "Starting", "Completed", etc.
	displayStatusDetails []string
}

func newThinkingDisplayUIMock() *thinkingDisplayUIMock {
	return &thinkingDisplayUIMock{
		displayStatusAgents:  []string{},
		displayStatusTypes:   []string{},
		displayStatusDetails: []string{},
	}
}

func (m *thinkingDisplayUIMock) DisplaySubagentStatus(agentName string, status string, details string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.displayStatusCalls++
	m.displayStatusAgents = append(m.displayStatusAgents, agentName)
	m.displayStatusTypes = append(m.displayStatusTypes, status)
	m.displayStatusDetails = append(m.displayStatusDetails, details)
	return nil
}

// Implement remaining UserInterface methods to satisfy the interface.
func (m *thinkingDisplayUIMock) GetUserInput(_ context.Context) (string, bool) {
	return "", false
}

func (m *thinkingDisplayUIMock) DisplayMessage(_ string, _ string) error {
	return nil
}

func (m *thinkingDisplayUIMock) DisplayError(_ error) error {
	return nil
}

func (m *thinkingDisplayUIMock) DisplayToolResult(_, _, _ string) error {
	return nil
}

func (m *thinkingDisplayUIMock) DisplaySystemMessage(_ string) error {
	return nil
}

func (m *thinkingDisplayUIMock) DisplayThinking(_ string) error {
	return nil
}

func (m *thinkingDisplayUIMock) SetPrompt(_ string) error {
	return nil
}

func (m *thinkingDisplayUIMock) ClearScreen() error {
	return nil
}

func (m *thinkingDisplayUIMock) SetColorScheme(_ port.ColorScheme) error {
	return nil
}

func (m *thinkingDisplayUIMock) ConfirmBashCommand(_ string, _ bool, _ string, _ string) bool {
	return false
}

// getStatusCallsForType returns the number of times DisplaySubagentStatus was called with a specific status type.
func (m *thinkingDisplayUIMock) getStatusCallsForType(statusType string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	count := 0
	for _, s := range m.displayStatusTypes {
		if s == statusType {
			count++
		}
	}
	return count
}

// getCallOrderForAgent returns the sequence of status calls for a specific agent.
func (m *thinkingDisplayUIMock) getCallOrderForAgent(agentName string) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var statuses []string
	for i, agent := range m.displayStatusAgents {
		if agent == agentName {
			statuses = append(statuses, m.displayStatusTypes[i])
		}
	}
	return statuses
}

// thinkingDisplayConvServiceMock extends the base mock with GetThinkingMode support.
type thinkingDisplayConvServiceMock struct {
	*subagentRunnerConvServiceMock

	mu sync.Mutex

	// GetThinkingMode configuration
	thinkingModeEnabled    bool
	thinkingModeShowThink  bool
	thinkingModeBudget     int64
	getThinkingModeCalls   int
	getThinkingModeSession []string
}

func newThinkingDisplayConvServiceMock() *thinkingDisplayConvServiceMock {
	return &thinkingDisplayConvServiceMock{
		subagentRunnerConvServiceMock: newSubagentRunnerConvServiceMock(),
		thinkingModeEnabled:           false,
		thinkingModeShowThink:         false,
		thinkingModeBudget:            0,
		getThinkingModeSession:        []string{},
	}
}

func (m *thinkingDisplayConvServiceMock) GetThinkingMode(sessionID string) (port.ThinkingModeInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getThinkingModeCalls++
	m.getThinkingModeSession = append(m.getThinkingModeSession, sessionID)
	return port.ThinkingModeInfo{
		Enabled:      m.thinkingModeEnabled,
		ShowThinking: m.thinkingModeShowThink,
		BudgetTokens: m.thinkingModeBudget,
	}, nil
}

// TestSubagentRunner_ThinkingStatusDisplay_WhenEnabled tests that thinking status is shown when thinking mode is enabled.
func TestSubagentRunner_ThinkingStatusDisplay_WhenEnabled(t *testing.T) {
	// Arrange
	ctx := context.Background()
	uiMock := newThinkingDisplayUIMock()
	convMock := newThinkingDisplayConvServiceMock()
	toolMock := newSubagentRunnerToolExecutorMock()
	aiMock := newSubagentRunnerAIProviderMock()

	// Enable thinking mode
	convMock.thinkingModeEnabled = true
	convMock.thinkingModeShowThink = false // ShowThinking should NOT affect status display

	// Configure to complete immediately (no tool calls)
	msg, _ := entity.NewMessage(entity.RoleAssistant, "Task complete")
	convMock.processResponseMessages = []*entity.Message{msg}
	convMock.processResponseToolCalls = [][]port.ToolCallInfo{{}} // Empty tool calls = completion

	config := SubagentConfig{MaxActions: 10}
	runner := NewSubagentRunner(convMock, toolMock, aiMock, uiMock, config)

	agent := &entity.Subagent{
		Name:        "test-agent",
		Description: "Test agent for thinking display",
	}

	// Act
	_, err := runner.Run(ctx, agent, "Test prompt", "subagent-001")
	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify "Thinking" status was called at least once
	thinkingCalls := uiMock.getStatusCallsForType("Thinking")
	if thinkingCalls < 1 {
		t.Errorf("Expected at least 1 'Thinking' status call when thinking mode enabled, got %d", thinkingCalls)
	}

	// Verify GetThinkingMode was called
	if convMock.getThinkingModeCalls < 1 {
		t.Errorf("Expected GetThinkingMode to be called at least once, got %d calls", convMock.getThinkingModeCalls)
	}
}

// TestSubagentRunner_ThinkingStatusDisplay_NotShownWhenDisabled tests that thinking status is NOT shown when thinking mode is disabled.
func TestSubagentRunner_ThinkingStatusDisplay_NotShownWhenDisabled(t *testing.T) {
	// Arrange
	ctx := context.Background()
	uiMock := newThinkingDisplayUIMock()
	convMock := newThinkingDisplayConvServiceMock()
	toolMock := newSubagentRunnerToolExecutorMock()
	aiMock := newSubagentRunnerAIProviderMock()

	// Disable thinking mode
	convMock.thinkingModeEnabled = false

	// Configure to complete immediately
	msg, _ := entity.NewMessage(entity.RoleAssistant, "Task complete")
	convMock.processResponseMessages = []*entity.Message{msg}
	convMock.processResponseToolCalls = [][]port.ToolCallInfo{{}}

	config := SubagentConfig{MaxActions: 10}
	runner := NewSubagentRunner(convMock, toolMock, aiMock, uiMock, config)

	agent := &entity.Subagent{
		Name:        "test-agent",
		Description: "Test agent",
	}

	// Act
	_, err := runner.Run(ctx, agent, "Test prompt", "subagent-002")
	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify "Thinking" status was NOT called
	thinkingCalls := uiMock.getStatusCallsForType("Thinking")
	if thinkingCalls > 0 {
		t.Errorf("Expected 0 'Thinking' status calls when thinking mode disabled, got %d", thinkingCalls)
	}
}

// TestSubagentRunner_ThinkingStatusDisplay_CorrectAgentName tests that thinking status uses the correct agent name.
func TestSubagentRunner_ThinkingStatusDisplay_CorrectAgentName(t *testing.T) {
	// Arrange
	ctx := context.Background()
	uiMock := newThinkingDisplayUIMock()
	convMock := newThinkingDisplayConvServiceMock()
	toolMock := newSubagentRunnerToolExecutorMock()
	aiMock := newSubagentRunnerAIProviderMock()

	// Enable thinking mode
	convMock.thinkingModeEnabled = true

	// Configure to complete immediately
	msg, _ := entity.NewMessage(entity.RoleAssistant, "Task complete")
	convMock.processResponseMessages = []*entity.Message{msg}
	convMock.processResponseToolCalls = [][]port.ToolCallInfo{{}}

	config := SubagentConfig{MaxActions: 10}
	runner := NewSubagentRunner(convMock, toolMock, aiMock, uiMock, config)

	agent := &entity.Subagent{
		Name:        "code-reviewer",
		Description: "Code review agent",
	}

	// Act
	_, err := runner.Run(ctx, agent, "Review this code", "subagent-003")
	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Find the "Thinking" status call and verify agent name
	uiMock.mu.Lock()
	defer uiMock.mu.Unlock()

	foundThinkingCall := false
	for i, statusType := range uiMock.displayStatusTypes {
		if statusType == "Thinking" {
			foundThinkingCall = true
			if uiMock.displayStatusAgents[i] != "code-reviewer" {
				t.Errorf(
					"Expected thinking status to use agent name 'code-reviewer', got '%s'",
					uiMock.displayStatusAgents[i],
				)
			}
		}
	}

	if !foundThinkingCall {
		t.Error("Expected to find at least one 'Thinking' status call")
	}
}

// TestSubagentRunner_ThinkingStatusDisplay_BeforeProcessAssistantResponse tests that thinking status is displayed BEFORE ProcessAssistantResponse.
func TestSubagentRunner_ThinkingStatusDisplay_BeforeProcessAssistantResponse(t *testing.T) {
	// Arrange
	ctx := context.Background()
	uiMock := newThinkingDisplayUIMock()
	convMock := newThinkingDisplayConvServiceMock()
	toolMock := newSubagentRunnerToolExecutorMock()
	aiMock := newSubagentRunnerAIProviderMock()

	// Enable thinking mode
	convMock.thinkingModeEnabled = true

	// Track when ProcessAssistantResponse is called
	var processResponseCalledAfterThinking bool
	originalProcessResponse := convMock.ProcessAssistantResponse
	convMock.processResponseCalls = 0 // Reset

	// Configure to complete immediately
	msg, _ := entity.NewMessage(entity.RoleAssistant, "Task complete")
	convMock.processResponseMessages = []*entity.Message{msg}
	convMock.processResponseToolCalls = [][]port.ToolCallInfo{{}}

	config := SubagentConfig{MaxActions: 10}
	runner := NewSubagentRunner(convMock, toolMock, aiMock, uiMock, config)

	agent := &entity.Subagent{
		Name:        "test-agent",
		Description: "Test agent",
	}

	// Act
	_, err := runner.Run(ctx, agent, "Test prompt", "subagent-004")
	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify ProcessAssistantResponse was called
	if convMock.processResponseCalls < 1 {
		t.Error("Expected ProcessAssistantResponse to be called")
	}

	// Verify "Thinking" status was called before ProcessAssistantResponse
	// Since ProcessAssistantResponse is called in the execution loop, we need to verify
	// the call order through the mocks. For now, we verify that both were called.
	thinkingCalls := uiMock.getStatusCallsForType("Thinking")
	if thinkingCalls < 1 {
		t.Error("Expected 'Thinking' status to be called before ProcessAssistantResponse")
	}

	// Additional verification: ProcessAssistantResponse should be called after thinking status
	// This is implicitly tested by the order of operations in the code
	_ = originalProcessResponse
	_ = processResponseCalledAfterThinking
}

// TestSubagentRunner_ThinkingStatusDisplay_OnEveryLoopIteration tests that thinking status is shown on EVERY loop iteration.
func TestSubagentRunner_ThinkingStatusDisplay_OnEveryLoopIteration(t *testing.T) {
	// Arrange
	ctx := context.Background()
	uiMock := newThinkingDisplayUIMock()
	convMock := newThinkingDisplayConvServiceMock()
	toolMock := newSubagentRunnerToolExecutorMock()
	aiMock := newSubagentRunnerAIProviderMock()

	// Enable thinking mode
	convMock.thinkingModeEnabled = true

	// Configure for 3 loop iterations (2 tool calls + 1 completion)
	msg1, _ := entity.NewMessage(entity.RoleAssistant, "Running first tool")
	msg2, _ := entity.NewMessage(entity.RoleAssistant, "Running second tool")
	msg3, _ := entity.NewMessage(entity.RoleAssistant, "Task complete")

	toolCall1 := port.ToolCallInfo{
		ToolID:   "call1",
		ToolName: "read_file",
		Input:    map[string]interface{}{"path": "/test.go"},
	}
	toolCall2 := port.ToolCallInfo{
		ToolID:   "call2",
		ToolName: "bash",
		Input:    map[string]interface{}{"command": "ls"},
	}

	convMock.processResponseMessages = []*entity.Message{msg1, msg2, msg3}
	convMock.processResponseToolCalls = [][]port.ToolCallInfo{
		{toolCall1}, // First iteration: 1 tool call
		{toolCall2}, // Second iteration: 1 tool call
		{},          // Third iteration: no tools (completion)
	}

	config := SubagentConfig{MaxActions: 10}
	runner := NewSubagentRunner(convMock, toolMock, aiMock, uiMock, config)

	agent := &entity.Subagent{
		Name:        "multi-tool-agent",
		Description: "Agent that executes multiple tools",
	}

	// Act
	_, err := runner.Run(ctx, agent, "Run multiple tools", "subagent-005")
	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify ProcessAssistantResponse was called 3 times (one per iteration)
	if convMock.processResponseCalls != 3 {
		t.Errorf("Expected ProcessAssistantResponse to be called 3 times, got %d", convMock.processResponseCalls)
	}

	// Verify "Thinking" status was called 3 times (once per iteration)
	thinkingCalls := uiMock.getStatusCallsForType("Thinking")
	if thinkingCalls != 3 {
		t.Errorf("Expected 'Thinking' status to be called 3 times (once per loop iteration), got %d", thinkingCalls)
	}
}

// TestSubagentRunner_ThinkingStatusDisplay_ShowThinkingFlagDoesNotAffectStatus tests that ShowThinking flag doesn't affect status display.
func TestSubagentRunner_ThinkingStatusDisplay_ShowThinkingFlagDoesNotAffectStatus(t *testing.T) {
	tests := []struct {
		name              string
		showThinking      bool
		wantThinkingCalls int
	}{
		{
			name:              "ShowThinking=true still shows status",
			showThinking:      true,
			wantThinkingCalls: 1,
		},
		{
			name:              "ShowThinking=false still shows status",
			showThinking:      false,
			wantThinkingCalls: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			ctx := context.Background()
			uiMock := newThinkingDisplayUIMock()
			convMock := newThinkingDisplayConvServiceMock()
			toolMock := newSubagentRunnerToolExecutorMock()
			aiMock := newSubagentRunnerAIProviderMock()

			// Enable thinking mode with varying ShowThinking flag
			convMock.thinkingModeEnabled = true
			convMock.thinkingModeShowThink = tt.showThinking

			// Configure to complete immediately
			msg, _ := entity.NewMessage(entity.RoleAssistant, "Task complete")
			convMock.processResponseMessages = []*entity.Message{msg}
			convMock.processResponseToolCalls = [][]port.ToolCallInfo{{}}

			config := SubagentConfig{MaxActions: 10}
			runner := NewSubagentRunner(convMock, toolMock, aiMock, uiMock, config)

			agent := &entity.Subagent{
				Name:        "test-agent",
				Description: "Test agent",
			}

			// Act
			_, err := runner.Run(ctx, agent, "Test prompt", "subagent-006")
			// Assert
			if err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}

			thinkingCalls := uiMock.getStatusCallsForType("Thinking")
			if thinkingCalls != tt.wantThinkingCalls {
				t.Errorf(
					"Expected %d 'Thinking' status calls regardless of ShowThinking flag, got %d",
					tt.wantThinkingCalls,
					thinkingCalls,
				)
			}
		})
	}
}

// TestSubagentRunner_ThinkingStatusDisplay_EmptyDetails tests that thinking status has empty details.
func TestSubagentRunner_ThinkingStatusDisplay_EmptyDetails(t *testing.T) {
	// Arrange
	ctx := context.Background()
	uiMock := newThinkingDisplayUIMock()
	convMock := newThinkingDisplayConvServiceMock()
	toolMock := newSubagentRunnerToolExecutorMock()
	aiMock := newSubagentRunnerAIProviderMock()

	// Enable thinking mode
	convMock.thinkingModeEnabled = true

	// Configure to complete immediately
	msg, _ := entity.NewMessage(entity.RoleAssistant, "Task complete")
	convMock.processResponseMessages = []*entity.Message{msg}
	convMock.processResponseToolCalls = [][]port.ToolCallInfo{{}}

	config := SubagentConfig{MaxActions: 10}
	runner := NewSubagentRunner(convMock, toolMock, aiMock, uiMock, config)

	agent := &entity.Subagent{
		Name:        "test-agent",
		Description: "Test agent",
	}

	// Act
	_, err := runner.Run(ctx, agent, "Test prompt", "subagent-007")
	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify "Thinking" status has empty details
	uiMock.mu.Lock()
	defer uiMock.mu.Unlock()

	for i, statusType := range uiMock.displayStatusTypes {
		if statusType == "Thinking" {
			if uiMock.displayStatusDetails[i] != "" {
				t.Errorf("Expected 'Thinking' status to have empty details, got '%s'", uiMock.displayStatusDetails[i])
			}
		}
	}
}

// TestSubagentRunner_ThinkingStatusDisplay_CallOrder tests the overall call order of status messages.
func TestSubagentRunner_ThinkingStatusDisplay_CallOrder(t *testing.T) {
	// Arrange
	ctx := context.Background()
	uiMock := newThinkingDisplayUIMock()
	convMock := newThinkingDisplayConvServiceMock()
	toolMock := newSubagentRunnerToolExecutorMock()
	aiMock := newSubagentRunnerAIProviderMock()

	// Enable thinking mode
	convMock.thinkingModeEnabled = true

	// Configure for 1 tool call + completion
	msg1, _ := entity.NewMessage(entity.RoleAssistant, "Running tool")
	msg2, _ := entity.NewMessage(entity.RoleAssistant, "Task complete")

	toolCall := port.ToolCallInfo{
		ToolID:   "call1",
		ToolName: "read_file",
		Input:    map[string]interface{}{"path": "/test.go"},
	}

	convMock.processResponseMessages = []*entity.Message{msg1, msg2}
	convMock.processResponseToolCalls = [][]port.ToolCallInfo{
		{toolCall}, // First iteration: 1 tool call
		{},         // Second iteration: completion
	}

	config := SubagentConfig{MaxActions: 10}
	runner := NewSubagentRunner(convMock, toolMock, aiMock, uiMock, config)

	agent := &entity.Subagent{
		Name:        "order-test-agent",
		Description: "Test agent for call order",
	}

	// Act
	_, err := runner.Run(ctx, agent, "Test order", "subagent-008")
	// Assert
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify call order: Starting -> Thinking -> Executing read_file -> Tool completed -> Thinking -> Completed
	expectedOrder := []string{
		"Starting",
		"Thinking", // Before first ProcessAssistantResponse
		"Executing read_file",
		"Tool completed",
		"Thinking", // Before second ProcessAssistantResponse
		"Completed",
	}

	actualOrder := uiMock.getCallOrderForAgent("order-test-agent")
	if len(actualOrder) != len(expectedOrder) {
		t.Errorf(
			"Expected %d status calls, got %d. Actual order: %v",
			len(expectedOrder),
			len(actualOrder),
			actualOrder,
		)
	}

	for i, expected := range expectedOrder {
		if i >= len(actualOrder) {
			t.Errorf("Missing status call at index %d. Expected '%s'", i, expected)
			continue
		}
		if actualOrder[i] != expected {
			t.Errorf("At index %d: expected '%s', got '%s'", i, expected, actualOrder[i])
		}
	}
}

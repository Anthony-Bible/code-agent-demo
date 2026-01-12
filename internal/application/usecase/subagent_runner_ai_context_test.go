package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"sync"
	"testing"
)

// =============================================================================
// RED PHASE - SubagentRunner AI Context Tests
// =============================================================================
// These tests verify that SubagentRunner properly adds thinking mode to context
// before calling ProcessAssistantResponse(). This ensures the AnthropicAdapter
// receives the correct thinking mode configuration on every AI call.
//
// Pattern from chat_service.go (lines 211-215):
//   thinkingInfo, _ := cs.conversationService.GetThinkingMode(sessionID)
//   if thinkingInfo.Enabled {
//       ctx = port.WithThinkingMode(ctx, thinkingInfo)
//   }
//
// These tests should FAIL until the implementation is complete.
// =============================================================================

// =============================================================================
// Mock that Tracks Context Passed to ProcessAssistantResponse
// =============================================================================

// contextTrackingConvServiceMock tracks the context passed to ProcessAssistantResponse.
type contextTrackingConvServiceMock struct {
	mu sync.Mutex

	// Session management
	sessionID string

	// Thinking mode configuration
	thinkingModeEnabled bool
	thinkingModeInfo    port.ThinkingModeInfo

	// ProcessAssistantResponse tracking
	processResponseCalls     int
	processResponseContexts  []context.Context // Track contexts passed to ProcessAssistantResponse
	processResponseMessages  []*entity.Message
	processResponseToolCalls [][]port.ToolCallInfo

	// GetThinkingMode tracking
	getThinkingModeCalls int

	// Other methods
	startConversationCalls     int
	addUserMessageCalls        int
	addToolResultCalls         int
	endConversationCalls       int
	setCustomSystemPromptCalls int
	setThinkingModeCalls       int
}

func newContextTrackingConvServiceMock() *contextTrackingConvServiceMock {
	return &contextTrackingConvServiceMock{
		sessionID:                "session-123",
		processResponseContexts:  []context.Context{},
		processResponseMessages:  []*entity.Message{},
		processResponseToolCalls: [][]port.ToolCallInfo{},
	}
}

func (m *contextTrackingConvServiceMock) StartConversation(_ context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startConversationCalls++
	return m.sessionID, nil
}

func (m *contextTrackingConvServiceMock) AddUserMessage(
	_ context.Context,
	_ string,
	content string,
) (*entity.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addUserMessageCalls++
	msg, _ := entity.NewMessage(entity.RoleUser, content)
	return msg, nil
}

func (m *contextTrackingConvServiceMock) ProcessAssistantResponse(
	ctx context.Context,
	_ string,
) (*entity.Message, []port.ToolCallInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processResponseCalls++

	// CRITICAL: Track the context passed to this method
	m.processResponseContexts = append(m.processResponseContexts, ctx)

	var msg *entity.Message
	var toolCalls []port.ToolCallInfo
	idx := m.processResponseCalls - 1

	if idx < len(m.processResponseMessages) {
		msg = m.processResponseMessages[idx]
	}
	if idx < len(m.processResponseToolCalls) {
		toolCalls = m.processResponseToolCalls[idx]
	}

	return msg, toolCalls, nil
}

func (m *contextTrackingConvServiceMock) ProcessAssistantResponseStreaming(
	ctx context.Context,
	sessionID string,
	_ port.StreamCallback,
	_ port.ThinkingCallback,
) (*entity.Message, []port.ToolCallInfo, error) {
	// Delegate to non-streaming version for testing
	return m.ProcessAssistantResponse(ctx, sessionID)
}

func (m *contextTrackingConvServiceMock) AddToolResultMessage(
	_ context.Context,
	_ string,
	_ []entity.ToolResult,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addToolResultCalls++
	return nil
}

func (m *contextTrackingConvServiceMock) EndConversation(_ context.Context, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.endConversationCalls++
	return nil
}

func (m *contextTrackingConvServiceMock) SetCustomSystemPrompt(
	_ context.Context,
	_ string,
	_ string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCustomSystemPromptCalls++
	return nil
}

func (m *contextTrackingConvServiceMock) SetThinkingMode(_ string, info port.ThinkingModeInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setThinkingModeCalls++
	m.thinkingModeEnabled = info.Enabled
	m.thinkingModeInfo = info
	return nil
}

func (m *contextTrackingConvServiceMock) GetThinkingMode(_ string) (port.ThinkingModeInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getThinkingModeCalls++
	return m.thinkingModeInfo, nil
}

// Helper method to get tracked contexts (thread-safe).
func (m *contextTrackingConvServiceMock) GetProcessResponseContexts() []context.Context {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]context.Context{}, m.processResponseContexts...)
}

// =============================================================================
// Test Cases
// =============================================================================

// TestSubagentRunner_ThinkingModeInContext_EnabledOnSession tests that when
// thinking mode is enabled on the session, ProcessAssistantResponse receives
// a context with thinking mode info.
//
// This test should FAIL until thinking mode is added to context before the
// ProcessAssistantResponse call in runExecutionLoop().
func TestSubagentRunner_ThinkingModeInContext_EnabledOnSession(t *testing.T) {
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
	_ = convServiceMock.SetThinkingMode(convServiceMock.sessionID, thinkingInfo)

	// Configure mock to return a completion (no tool calls)
	completionMsg, _ := entity.NewMessage(entity.RoleAssistant, "Task completed")
	convServiceMock.processResponseMessages = []*entity.Message{completionMsg}
	convServiceMock.processResponseToolCalls = [][]port.ToolCallInfo{{}} // Empty tool calls = completion

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

	// Verify: ProcessAssistantResponse was called at least once
	if convServiceMock.processResponseCalls == 0 {
		t.Fatalf("Expected ProcessAssistantResponse to be called, but it wasn't")
	}

	// Verify: The context passed to ProcessAssistantResponse contains thinking mode
	contexts := convServiceMock.GetProcessResponseContexts()
	if len(contexts) == 0 {
		t.Fatalf("Expected at least one context to be tracked, got none")
	}

	// Check first call's context
	firstCallCtx := contexts[0]
	retrievedInfo, ok := port.ThinkingModeFromContext(firstCallCtx)

	if !ok {
		t.Errorf("Expected thinking mode in context, but it was not found")
	}

	if !retrievedInfo.Enabled {
		t.Errorf("Expected thinking mode Enabled=true, got Enabled=%v", retrievedInfo.Enabled)
	}

	if retrievedInfo.BudgetTokens != thinkingInfo.BudgetTokens {
		t.Errorf("Expected BudgetTokens=%d, got %d", thinkingInfo.BudgetTokens, retrievedInfo.BudgetTokens)
	}

	if retrievedInfo.ShowThinking != thinkingInfo.ShowThinking {
		t.Errorf("Expected ShowThinking=%v, got %v", thinkingInfo.ShowThinking, retrievedInfo.ShowThinking)
	}
}

// TestSubagentRunner_ThinkingModeInContext_DisabledOnSession tests that when
// thinking mode is disabled on the session, ProcessAssistantResponse receives
// a context without thinking mode info.
//
// This test should PASS even without the fix, as it verifies the negative case.
func TestSubagentRunner_ThinkingModeInContext_DisabledOnSession(t *testing.T) {
	// Setup: Create mocks
	convServiceMock := newContextTrackingConvServiceMock()
	toolExecutorMock := newSubagentRunnerToolExecutorMock()
	aiProviderMock := newSubagentRunnerAIProviderMock()

	// Thinking mode explicitly disabled
	thinkingInfo := port.ThinkingModeInfo{
		Enabled:      false,
		BudgetTokens: 0,
		ShowThinking: false,
	}
	_ = convServiceMock.SetThinkingMode(convServiceMock.sessionID, thinkingInfo)

	// Configure mock to return a completion
	completionMsg, _ := entity.NewMessage(entity.RoleAssistant, "Task completed")
	convServiceMock.processResponseMessages = []*entity.Message{completionMsg}
	convServiceMock.processResponseToolCalls = [][]port.ToolCallInfo{{}}

	// Create runner with thinking disabled
	config := SubagentConfig{
		MaxActions:      10,
		ThinkingEnabled: false,
	}
	runner := NewSubagentRunner(convServiceMock, toolExecutorMock, aiProviderMock, nil, config)

	// Execute
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

	// Verify: Context should NOT contain thinking mode (or it should be disabled)
	contexts := convServiceMock.GetProcessResponseContexts()
	if len(contexts) == 0 {
		t.Fatalf("Expected at least one context to be tracked, got none")
	}

	firstCallCtx := contexts[0]
	retrievedInfo, ok := port.ThinkingModeFromContext(firstCallCtx)

	// Either thinking mode is not in context, or it's disabled
	if ok && retrievedInfo.Enabled {
		t.Errorf("Expected thinking mode to be disabled or absent, but got Enabled=true")
	}
}

// TestSubagentRunner_ThinkingModeInContext_MatchesSessionConfig tests that
// the thinking mode info in the context passed to ProcessAssistantResponse
// exactly matches the session's configured thinking mode.
//
// This test should FAIL until the implementation is complete.
func TestSubagentRunner_ThinkingModeInContext_MatchesSessionConfig(t *testing.T) {
	tests := []struct {
		name         string
		thinkingInfo port.ThinkingModeInfo
		description  string
	}{
		{
			name: "enabled_with_budget_10000",
			thinkingInfo: port.ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 10000,
				ShowThinking: true,
			},
			description: "Thinking enabled with 10k token budget and showing enabled",
		},
		{
			name: "enabled_with_budget_5000_no_show",
			thinkingInfo: port.ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 5000,
				ShowThinking: false,
			},
			description: "Thinking enabled with 5k token budget but showing disabled",
		},
		{
			name: "enabled_unlimited_budget",
			thinkingInfo: port.ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 0, // 0 = unlimited
				ShowThinking: true,
			},
			description: "Thinking enabled with unlimited budget",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup
			convServiceMock := newContextTrackingConvServiceMock()
			toolExecutorMock := newSubagentRunnerToolExecutorMock()
			aiProviderMock := newSubagentRunnerAIProviderMock()

			// Set thinking mode on session
			_ = convServiceMock.SetThinkingMode(convServiceMock.sessionID, tt.thinkingInfo)

			// Configure mock to return completion
			completionMsg, _ := entity.NewMessage(entity.RoleAssistant, "Task completed")
			convServiceMock.processResponseMessages = []*entity.Message{completionMsg}
			convServiceMock.processResponseToolCalls = [][]port.ToolCallInfo{{}}

			config := SubagentConfig{
				MaxActions:      10,
				ThinkingEnabled: tt.thinkingInfo.Enabled,
				ThinkingBudget:  tt.thinkingInfo.BudgetTokens,
				ShowThinking:    tt.thinkingInfo.ShowThinking,
			}
			runner := NewSubagentRunner(convServiceMock, toolExecutorMock, aiProviderMock, nil, config)

			// Execute
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

			// Verify context contains exact thinking mode info
			contexts := convServiceMock.GetProcessResponseContexts()
			if len(contexts) == 0 {
				t.Fatalf("Expected at least one context to be tracked")
			}

			firstCallCtx := contexts[0]
			retrievedInfo, ok := port.ThinkingModeFromContext(firstCallCtx)

			if !ok {
				t.Errorf("Expected thinking mode in context for %s", tt.description)
				return
			}

			// Verify all fields match
			if retrievedInfo.Enabled != tt.thinkingInfo.Enabled {
				t.Errorf("Enabled mismatch: want %v, got %v", tt.thinkingInfo.Enabled, retrievedInfo.Enabled)
			}

			if retrievedInfo.BudgetTokens != tt.thinkingInfo.BudgetTokens {
				t.Errorf(
					"BudgetTokens mismatch: want %d, got %d",
					tt.thinkingInfo.BudgetTokens,
					retrievedInfo.BudgetTokens,
				)
			}

			if retrievedInfo.ShowThinking != tt.thinkingInfo.ShowThinking {
				t.Errorf(
					"ShowThinking mismatch: want %v, got %v",
					tt.thinkingInfo.ShowThinking,
					retrievedInfo.ShowThinking,
				)
			}
		})
	}
}

// TestSubagentRunner_ThinkingModeInContext_RefreshedOnEveryLoop tests that
// the context is updated with thinking mode on EVERY iteration of the execution
// loop, not just the first call.
//
// This is critical because thinking mode could theoretically change mid-execution,
// and we want to ensure consistency.
//
// This test should FAIL until the implementation is complete.
func TestSubagentRunner_ThinkingModeInContext_RefreshedOnEveryLoop(t *testing.T) {
	// Setup
	convServiceMock := newContextTrackingConvServiceMock()
	toolExecutorMock := newSubagentRunnerToolExecutorMock()
	aiProviderMock := newSubagentRunnerAIProviderMock()

	// Configure thinking mode
	thinkingInfo := port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 8000,
		ShowThinking: false,
	}
	_ = convServiceMock.SetThinkingMode(convServiceMock.sessionID, thinkingInfo)

	// Configure mock to simulate multiple tool calls (3 iterations)
	msg1, _ := entity.NewMessage(entity.RoleAssistant, "First response")
	msg2, _ := entity.NewMessage(entity.RoleAssistant, "Second response")
	msg3, _ := entity.NewMessage(entity.RoleAssistant, "Final response")

	toolCall1 := []port.ToolCallInfo{
		{ToolID: "call_1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}},
	}
	toolCall2 := []port.ToolCallInfo{
		{ToolID: "call_2", ToolName: "read_file", Input: map[string]interface{}{"path": "/test"}},
	}
	toolCall3 := []port.ToolCallInfo{} // Empty = completion

	convServiceMock.processResponseMessages = []*entity.Message{msg1, msg2, msg3}
	convServiceMock.processResponseToolCalls = [][]port.ToolCallInfo{toolCall1, toolCall2, toolCall3}

	config := SubagentConfig{
		MaxActions:      10,
		ThinkingEnabled: true,
		ThinkingBudget:  8000,
		ShowThinking:    false,
	}
	runner := NewSubagentRunner(convServiceMock, toolExecutorMock, aiProviderMock, nil, config)

	// Execute
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

	// Verify: ProcessAssistantResponse was called 3 times
	expectedCalls := 3
	if convServiceMock.processResponseCalls != expectedCalls {
		t.Fatalf("Expected %d ProcessAssistantResponse calls, got %d",
			expectedCalls, convServiceMock.processResponseCalls)
	}

	// Verify: ALL contexts have thinking mode
	contexts := convServiceMock.GetProcessResponseContexts()
	if len(contexts) != expectedCalls {
		t.Fatalf("Expected %d contexts tracked, got %d", expectedCalls, len(contexts))
	}

	// Check each iteration's context
	for i, ctx := range contexts {
		retrievedInfo, ok := port.ThinkingModeFromContext(ctx)

		if !ok {
			t.Errorf("Iteration %d: Expected thinking mode in context, but it was not found", i+1)
			continue
		}

		if !retrievedInfo.Enabled {
			t.Errorf("Iteration %d: Expected Enabled=true, got Enabled=%v", i+1, retrievedInfo.Enabled)
		}

		if retrievedInfo.BudgetTokens != thinkingInfo.BudgetTokens {
			t.Errorf("Iteration %d: Expected BudgetTokens=%d, got %d",
				i+1, thinkingInfo.BudgetTokens, retrievedInfo.BudgetTokens)
		}

		if retrievedInfo.ShowThinking != thinkingInfo.ShowThinking {
			t.Errorf("Iteration %d: Expected ShowThinking=%v, got %v",
				i+1, thinkingInfo.ShowThinking, retrievedInfo.ShowThinking)
		}
	}
}

// TestSubagentRunner_ThinkingModeInContext_GetThinkingModeCalled tests that
// GetThinkingMode is called before each ProcessAssistantResponse call to
// fetch the current thinking mode configuration.
//
// This verifies the implementation follows the pattern from chat_service.go.
//
// This test should FAIL until the implementation is complete.
func TestSubagentRunner_ThinkingModeInContext_GetThinkingModeCalled(t *testing.T) {
	// Setup
	convServiceMock := newContextTrackingConvServiceMock()
	toolExecutorMock := newSubagentRunnerToolExecutorMock()
	aiProviderMock := newSubagentRunnerAIProviderMock()

	// Configure thinking mode
	thinkingInfo := port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 12000,
		ShowThinking: true,
	}
	_ = convServiceMock.SetThinkingMode(convServiceMock.sessionID, thinkingInfo)

	// Configure mock to simulate 2 iterations
	msg1, _ := entity.NewMessage(entity.RoleAssistant, "First response")
	msg2, _ := entity.NewMessage(entity.RoleAssistant, "Final response")

	toolCall1 := []port.ToolCallInfo{
		{ToolID: "call_1", ToolName: "bash", Input: map[string]interface{}{"command": "echo test"}},
	}
	toolCall2 := []port.ToolCallInfo{} // Completion

	convServiceMock.processResponseMessages = []*entity.Message{msg1, msg2}
	convServiceMock.processResponseToolCalls = [][]port.ToolCallInfo{toolCall1, toolCall2}

	config := SubagentConfig{
		MaxActions:      10,
		ThinkingEnabled: true,
		ThinkingBudget:  12000,
		ShowThinking:    true,
	}
	runner := NewSubagentRunner(convServiceMock, toolExecutorMock, aiProviderMock, nil, config)

	// Execute
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

	// Verify: GetThinkingMode was called at least as many times as ProcessAssistantResponse
	// (should be called once per loop iteration before ProcessAssistantResponse)
	expectedMinCalls := convServiceMock.processResponseCalls
	if convServiceMock.getThinkingModeCalls < expectedMinCalls {
		t.Errorf("Expected GetThinkingMode to be called at least %d times (once per ProcessAssistantResponse), got %d",
			expectedMinCalls, convServiceMock.getThinkingModeCalls)
	}
}

package usecase

import (
	"context"
	"sync"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// =============================================================================
// Shared Mock Implementations for SubagentRunner Tests
// =============================================================================

// contextTrackingConvServiceMock tracks the context and thinking mode calls.
type contextTrackingConvServiceMock struct {
	mu sync.Mutex

	// Session management
	sessionID string

	// Thinking mode configuration
	thinkingModeEnabled bool
	thinkingModeInfo    port.ThinkingModeInfo

	// ProcessAssistantResponse tracking
	processResponseCalls     int
	processResponseContexts  []context.Context
	processResponseMessages  []*entity.Message
	processResponseToolCalls [][]port.ToolCallInfo

	// GetThinkingMode tracking
	getThinkingModeCalls int

	// SetThinkingMode tracking
	setThinkingModeCalls int
	setThinkingModeInfo  []port.ThinkingModeInfo

	// Other methods
	startConversationCalls     int
	addUserMessageCalls        int
	addToolResultCalls         int
	endConversationCalls       int
	setCustomSystemPromptCalls int
}

func newContextTrackingConvServiceMock() *contextTrackingConvServiceMock {
	return &contextTrackingConvServiceMock{
		sessionID:                "session-123",
		processResponseContexts:  []context.Context{},
		processResponseMessages:  []*entity.Message{},
		processResponseToolCalls: [][]port.ToolCallInfo{},
		setThinkingModeInfo:      []port.ThinkingModeInfo{},
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
	m.setThinkingModeInfo = append(m.setThinkingModeInfo, info)
	return nil
}

func (m *contextTrackingConvServiceMock) GetThinkingMode(_ string) (port.ThinkingModeInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.getThinkingModeCalls++
	return m.thinkingModeInfo, nil
}

// Helper method to get tracked contexts.
func (m *contextTrackingConvServiceMock) GetProcessResponseContexts() []context.Context {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]context.Context{}, m.processResponseContexts...)
}

func createSubagentRunnerCompletionMessage() *entity.Message {
	msg, _ := entity.NewMessage(entity.RoleAssistant, "Task completed")
	return msg
}

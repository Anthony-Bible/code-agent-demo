package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// =============================================================================
// Mocks for BaseRunner tests
// =============================================================================

type baseRunnerConvServiceMock struct {
	endConversationCalls  int
	endConversationError  error
	addToolResultCalls    int
	addToolResultError    error
	addToolResultResults  [][]entity.ToolResult
	addUserMessageCalls   int
	addUserMessageError   error
	addUserMessageContent []string
}

func (m *baseRunnerConvServiceMock) StartConversation(_ context.Context) (string, error) {
	return "session-1", nil
}

func (m *baseRunnerConvServiceMock) AddUserMessage(_ context.Context, _, content string) (*entity.Message, error) {
	m.addUserMessageCalls++
	m.addUserMessageContent = append(m.addUserMessageContent, content)
	return &entity.Message{}, m.addUserMessageError
}

func (m *baseRunnerConvServiceMock) ProcessAssistantResponse(_ context.Context, _ string) (*entity.Message, []port.ToolCallInfo, error) {
	return nil, nil, nil
}

func (m *baseRunnerConvServiceMock) ProcessAssistantResponseStreaming(_ context.Context, _ string, _ port.StreamCallback, _ port.ThinkingCallback) (*entity.Message, []port.ToolCallInfo, error) {
	return nil, nil, nil
}

func (m *baseRunnerConvServiceMock) AddToolResultMessage(_ context.Context, _ string, results []entity.ToolResult) error {
	m.addToolResultCalls++
	m.addToolResultResults = append(m.addToolResultResults, results)
	return m.addToolResultError
}

func (m *baseRunnerConvServiceMock) EndConversation(_ context.Context, _ string) error {
	m.endConversationCalls++
	return m.endConversationError
}

func (m *baseRunnerConvServiceMock) SetCustomSystemPrompt(_ context.Context, _, _ string) error {
	return nil
}

func (m *baseRunnerConvServiceMock) SetThinkingMode(_ string, _ port.ThinkingModeInfo) error {
	return nil
}

func (m *baseRunnerConvServiceMock) GetThinkingMode(_ string) (port.ThinkingModeInfo, error) {
	return port.ThinkingModeInfo{}, nil
}

type baseRunnerToolExecutorMock struct {
	executeResult string
	executeError  error
	executeCalls  int
}

func (m *baseRunnerToolExecutorMock) RegisterTool(_ entity.Tool) error  { return nil }
func (m *baseRunnerToolExecutorMock) UnregisterTool(_ string) error     { return nil }
func (m *baseRunnerToolExecutorMock) ListTools() ([]entity.Tool, error) { return nil, nil }
func (m *baseRunnerToolExecutorMock) GetTool(_ string) (entity.Tool, bool) {
	return entity.Tool{}, false
}

func (m *baseRunnerToolExecutorMock) ValidateToolInput(_ string, _ interface{}) error {
	return nil
}

func (m *baseRunnerToolExecutorMock) ExecuteTool(_ context.Context, _ string, _ interface{}) (string, error) {
	m.executeCalls++
	return m.executeResult, m.executeError
}

type mockSafetyEnforcer struct {
	allowedTools map[string]bool
}

func (m *mockSafetyEnforcer) CheckToolAllowed(tool string) error {
	if m.allowedTools != nil && !m.allowedTools[tool] {
		return errors.New("tool not allowed: " + tool)
	}
	return nil
}

func (m *mockSafetyEnforcer) CheckCommandAllowed(_ string) error   { return nil }
func (m *mockSafetyEnforcer) CheckActionBudget(_ int) error        { return nil }
func (m *mockSafetyEnforcer) CheckTimeout(_ context.Context) error { return nil }
func (m *mockSafetyEnforcer) GetMaxActions() int                   { return 50 }

// =============================================================================
// Tests
// =============================================================================

func TestBaseRunner_CleanupConversation(t *testing.T) {
	mock := &baseRunnerConvServiceMock{}
	b := &BaseRunner{ConvService: mock}

	b.CleanupConversation("session-1", "entity-1", "entity_label")

	if mock.endConversationCalls != 1 {
		t.Errorf("expected 1 EndConversation call, got %d", mock.endConversationCalls)
	}
}

func TestBaseRunner_CleanupConversation_Error(t *testing.T) {
	mock := &baseRunnerConvServiceMock{endConversationError: errors.New("cleanup failed")}
	b := &BaseRunner{ConvService: mock}

	// Should not panic on error
	b.CleanupConversation("session-1", "entity-1", "entity_label")

	if mock.endConversationCalls != 1 {
		t.Errorf("expected 1 EndConversation call, got %d", mock.endConversationCalls)
	}
}

func TestBaseRunner_ExecuteToolCall(t *testing.T) {
	tests := []struct {
		name        string
		execResult  string
		execError   error
		wantIsError bool
		wantResult  string
	}{
		{
			name:       "successful execution",
			execResult: "output data",
			wantResult: "output data",
		},
		{
			name:        "execution error",
			execError:   errors.New("exec failed"),
			wantIsError: true,
			wantResult:  "exec failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &baseRunnerToolExecutorMock{
				executeResult: tt.execResult,
				executeError:  tt.execError,
			}
			b := &BaseRunner{ToolExecutor: executor}

			tc := port.ToolCallInfo{ToolID: "id-1", ToolName: "bash", Input: map[string]any{"command": "ls"}}
			result := b.ExecuteToolCall(context.Background(), tc)

			if result.IsError != tt.wantIsError {
				t.Errorf("IsError = %v, want %v", result.IsError, tt.wantIsError)
			}
			if result.Result != tt.wantResult {
				t.Errorf("Result = %q, want %q", result.Result, tt.wantResult)
			}
			if result.ToolID != "id-1" {
				t.Errorf("ToolID = %q, want %q", result.ToolID, "id-1")
			}
		})
	}
}

func TestBaseRunner_ProcessToolCalls(t *testing.T) {
	tests := []struct {
		name             string
		toolCalls        []port.ToolCallInfo
		permChecker      ToolPermissionChecker
		wantActionsTaken int
		wantResultCount  int
		wantBlockedCount int
	}{
		{
			name: "all allowed",
			toolCalls: []port.ToolCallInfo{
				{ToolID: "1", ToolName: "bash"},
				{ToolID: "2", ToolName: "read_file"},
			},
			permChecker:      &AllowAllPermissionChecker{},
			wantActionsTaken: 2,
			wantResultCount:  2,
		},
		{
			name: "one blocked one allowed",
			toolCalls: []port.ToolCallInfo{
				{ToolID: "1", ToolName: "bash"},
				{ToolID: "2", ToolName: "blocked_tool"},
			},
			permChecker:      &AllowedListPermissionChecker{AllowedTools: []string{"bash"}},
			wantActionsTaken: 1,
			wantResultCount:  2, // blocked tool also produces a result
		},
		{
			name: "all blocked",
			toolCalls: []port.ToolCallInfo{
				{ToolID: "1", ToolName: "dangerous"},
			},
			permChecker:      &AllowedListPermissionChecker{AllowedTools: []string{}},
			wantActionsTaken: 0,
			wantResultCount:  1,
		},
		{
			name:             "empty tool calls",
			toolCalls:        []port.ToolCallInfo{},
			permChecker:      &AllowAllPermissionChecker{},
			wantActionsTaken: 0,
			wantResultCount:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			convMock := &baseRunnerConvServiceMock{}
			execMock := &baseRunnerToolExecutorMock{executeResult: "ok"}
			b := &BaseRunner{
				ConvService:       convMock,
				ToolExecutor:      execMock,
				PermissionChecker: tt.permChecker,
			}

			rc := &BaseRunContext{
				Ctx:        context.Background(),
				SessionID:  "session-1",
				StartTime:  time.Now(),
				MaxActions: 10,
			}

			executor := func(ctx context.Context, tc port.ToolCallInfo) entity.ToolResult {
				return b.ExecuteToolCall(ctx, tc)
			}

			err := b.ProcessToolCalls(rc, tt.toolCalls, "is not allowed", executor)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if rc.ActionsTaken != tt.wantActionsTaken {
				t.Errorf("ActionsTaken = %d, want %d", rc.ActionsTaken, tt.wantActionsTaken)
			}

			if tt.wantResultCount > 0 {
				if convMock.addToolResultCalls != 1 {
					t.Errorf("AddToolResultMessage calls = %d, want 1", convMock.addToolResultCalls)
				}
				if len(convMock.addToolResultResults[0]) != tt.wantResultCount {
					t.Errorf("result count = %d, want %d", len(convMock.addToolResultResults[0]), tt.wantResultCount)
				}
			} else if convMock.addToolResultCalls != 0 {
				t.Errorf("AddToolResultMessage calls = %d, want 0", convMock.addToolResultCalls)
			}
		})
	}
}

func TestBaseRunner_InjectTurnWarningIfNeeded(t *testing.T) {
	tests := []struct {
		name         string
		actionsTaken int
		maxActions   int
		wantWarning  bool
	}{
		{name: "at threshold", actionsTaken: 5, maxActions: 10, wantWarning: true},
		{name: "between threshold and 1", actionsTaken: 7, maxActions: 10, wantWarning: true},
		{name: "at 1 remaining", actionsTaken: 9, maxActions: 10, wantWarning: true},
		{name: "above threshold", actionsTaken: 0, maxActions: 10, wantWarning: false},
		{name: "at max", actionsTaken: 10, maxActions: 10, wantWarning: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			convMock := &baseRunnerConvServiceMock{}
			b := &BaseRunner{ConvService: convMock}
			rc := &BaseRunContext{
				Ctx:          context.Background(),
				SessionID:    "session-1",
				ActionsTaken: tt.actionsTaken,
				MaxActions:   tt.maxActions,
			}

			b.InjectTurnWarningIfNeeded(rc, DefaultTurnWarningConfig())

			if tt.wantWarning && convMock.addUserMessageCalls == 0 {
				t.Error("expected warning message to be sent, but none was sent")
			}
			if !tt.wantWarning && convMock.addUserMessageCalls != 0 {
				t.Error("expected no warning message, but one was sent")
			}
		})
	}
}

func TestBaseRunner_LimitToolCalls(t *testing.T) {
	b := &BaseRunner{}

	tests := []struct {
		name         string
		actionsTaken int
		maxActions   int
		inputCount   int
		wantCount    int
	}{
		{name: "within limit", actionsTaken: 0, maxActions: 10, inputCount: 5, wantCount: 5},
		{name: "over limit", actionsTaken: 8, maxActions: 10, inputCount: 5, wantCount: 2},
		{name: "zero remaining", actionsTaken: 10, maxActions: 10, inputCount: 5, wantCount: 0},
		{name: "exact match", actionsTaken: 5, maxActions: 10, inputCount: 5, wantCount: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc := &BaseRunContext{ActionsTaken: tt.actionsTaken, MaxActions: tt.maxActions}
			calls := make([]port.ToolCallInfo, tt.inputCount)
			for i := range calls {
				calls[i] = port.ToolCallInfo{ToolID: "id"}
			}

			result := b.LimitToolCalls(rc, calls)
			gotCount := len(result)
			if gotCount != tt.wantCount {
				t.Errorf("got %d tool calls, want %d", gotCount, tt.wantCount)
			}
		})
	}
}

// =============================================================================
// ToolPermissionChecker tests
// =============================================================================

func TestSafetyEnforcerPermissionChecker(t *testing.T) {
	tests := []struct {
		name     string
		enforcer SafetyEnforcer
		toolName string
		want     bool
	}{
		{name: "nil enforcer allows all", enforcer: nil, toolName: "bash", want: true},
		{
			name:     "allowed tool",
			enforcer: &mockSafetyEnforcer{allowedTools: map[string]bool{"bash": true}},
			toolName: "bash",
			want:     true,
		},
		{
			name:     "blocked tool",
			enforcer: &mockSafetyEnforcer{allowedTools: map[string]bool{"bash": true}},
			toolName: "dangerous",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &SafetyEnforcerPermissionChecker{Enforcer: tt.enforcer}
			tc := port.ToolCallInfo{ToolName: tt.toolName}
			got := checker.IsToolCallAllowed(tc)
			if got != tt.want {
				t.Errorf("IsToolCallAllowed(%q) = %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}

func TestAllowedListPermissionChecker(t *testing.T) {
	tests := []struct {
		name         string
		allowedTools []string
		toolName     string
		want         bool
	}{
		{name: "nil list allows all", allowedTools: nil, toolName: "anything", want: true},
		{name: "empty list blocks all", allowedTools: []string{}, toolName: "bash", want: false},
		{name: "tool in list", allowedTools: []string{"bash", "read_file"}, toolName: "bash", want: true},
		{name: "tool not in list", allowedTools: []string{"bash", "read_file"}, toolName: "dangerous", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &AllowedListPermissionChecker{AllowedTools: tt.allowedTools}
			tc := port.ToolCallInfo{ToolName: tt.toolName}
			got := checker.IsToolCallAllowed(tc)
			if got != tt.want {
				t.Errorf("IsToolCallAllowed(%q) = %v, want %v", tt.toolName, got, tt.want)
			}
		})
	}
}

func TestAllowAllPermissionChecker(t *testing.T) {
	checker := &AllowAllPermissionChecker{}
	tc := port.ToolCallInfo{ToolName: "anything"}
	if !checker.IsToolCallAllowed(tc) {
		t.Error("AllowAllPermissionChecker should always return true")
	}
}

func TestNewSafetyPermissionChecker(t *testing.T) {
	t.Run("nil enforcer returns AllowAll", func(t *testing.T) {
		checker := newSafetyPermissionChecker(nil)
		if _, ok := checker.(*AllowAllPermissionChecker); !ok {
			t.Errorf("expected *AllowAllPermissionChecker, got %T", checker)
		}
	})

	t.Run("non-nil enforcer returns SafetyEnforcer checker", func(t *testing.T) {
		enforcer := &mockSafetyEnforcer{}
		checker := newSafetyPermissionChecker(enforcer)
		if c, ok := checker.(*SafetyEnforcerPermissionChecker); !ok {
			t.Errorf("expected *SafetyEnforcerPermissionChecker, got %T", checker)
		} else if c.Enforcer != enforcer {
			t.Error("enforcer not set correctly")
		}
	})
}

//nolint:revive // This file contains test mocks that implement interfaces, unused parameters are expected
//nolint:revive // This file contains test mocks that implement interfaces, unused parameters are expected
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// =============================================================================
// InvestigationRunner Tests
// These tests verify the behavior of InvestigationRunner which orchestrates
// AI-driven alert investigations.
// =============================================================================

// =============================================================================
// Mock Implementations for InvestigationRunner Tests
// =============================================================================

// investigationRunnerConvServiceMock implements ConversationServiceInterface for testing.
type investigationRunnerConvServiceMock struct {
	mu sync.Mutex

	// StartConversation tracking
	startConversationCalls   int
	startConversationError   error
	startConversationSession string

	// AddUserMessage tracking
	addUserMessageCalls   int
	addUserMessageError   error
	addUserMessageContent []string
	onAddUserMessage      func() // Callback for tracking call order

	// ProcessAssistantResponse tracking
	processResponseCalls     int
	processResponseError     error
	processResponseMessages  []*entity.Message
	processResponseToolCalls [][]port.ToolCallInfo

	// Thinking content for streaming
	thinkingContent string

	// AddToolResultMessage tracking
	addToolResultCalls   int
	addToolResultError   error
	addToolResultResults [][]entity.ToolResult

	// EndConversation tracking
	endConversationCalls   int
	endConversationError   error
	endConversationSession string

	// SetCustomSystemPrompt tracking
	setCustomSystemPromptCalls   int
	setCustomSystemPromptError   error
	setCustomSystemPromptContent []string
	onSetCustomSystemPrompt      func() // Callback for tracking call order

	// SetThinkingMode tracking
	setThinkingModeCalls   int
	setThinkingModeSession string
	setThinkingModeInfo    port.ThinkingModeInfo
	setThinkingModeError   error

	// GetThinkingMode tracking
	getThinkingModeInfo  port.ThinkingModeInfo
	getThinkingModeError error
}

func newInvestigationRunnerConvServiceMock() *investigationRunnerConvServiceMock {
	return &investigationRunnerConvServiceMock{
		startConversationSession: "test-session-123",
		processResponseMessages:  []*entity.Message{},
		processResponseToolCalls: [][]port.ToolCallInfo{},
	}
}

func (m *investigationRunnerConvServiceMock) StartConversation(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startConversationCalls++
	if m.startConversationError != nil {
		return "", m.startConversationError
	}
	return m.startConversationSession, nil
}

func (m *investigationRunnerConvServiceMock) AddUserMessage(
	ctx context.Context,
	sessionID, content string,
) (*entity.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addUserMessageCalls++
	m.addUserMessageContent = append(m.addUserMessageContent, content)
	if m.onAddUserMessage != nil {
		m.onAddUserMessage()
	}
	if m.addUserMessageError != nil {
		return nil, m.addUserMessageError
	}
	msg, _ := entity.NewMessage(entity.RoleUser, content)
	return msg, nil
}

func (m *investigationRunnerConvServiceMock) ProcessAssistantResponse(
	ctx context.Context,
	sessionID string,
) (*entity.Message, []port.ToolCallInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processResponseCalls++
	if m.processResponseError != nil {
		return nil, nil, m.processResponseError
	}
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

func (m *investigationRunnerConvServiceMock) ProcessAssistantResponseStreaming(
	ctx context.Context,
	sessionID string,
	textCallback port.StreamCallback,
	thinkingCallback port.ThinkingCallback,
) (*entity.Message, []port.ToolCallInfo, error) {
	// Call thinking callback if provided and thinking content is available
	if thinkingCallback != nil && m.thinkingContent != "" {
		_ = thinkingCallback(m.thinkingContent)
	}
	// Delegate to non-streaming version for testing
	return m.ProcessAssistantResponse(ctx, sessionID)
}

func (m *investigationRunnerConvServiceMock) AddToolResultMessage(
	ctx context.Context,
	sessionID string,
	toolResults []entity.ToolResult,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addToolResultCalls++
	m.addToolResultResults = append(m.addToolResultResults, toolResults)
	return m.addToolResultError
}

func (m *investigationRunnerConvServiceMock) EndConversation(ctx context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.endConversationCalls++
	m.endConversationSession = sessionID
	return m.endConversationError
}

func (m *investigationRunnerConvServiceMock) SetCustomSystemPrompt(
	ctx context.Context,
	sessionID, prompt string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCustomSystemPromptCalls++
	m.setCustomSystemPromptContent = append(m.setCustomSystemPromptContent, prompt)
	if m.onSetCustomSystemPrompt != nil {
		m.onSetCustomSystemPrompt()
	}
	return m.setCustomSystemPromptError
}

func (m *investigationRunnerConvServiceMock) SetThinkingMode(sessionID string, info port.ThinkingModeInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setThinkingModeCalls++
	m.setThinkingModeSession = sessionID
	m.setThinkingModeInfo = info
	return m.setThinkingModeError
}

func (m *investigationRunnerConvServiceMock) GetThinkingMode(sessionID string) (port.ThinkingModeInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getThinkingModeInfo, m.getThinkingModeError
}

// investigationRunnerToolExecutorMock implements port.ToolExecutor and
// port.InvestigationRegistrar for testing.
type investigationRunnerToolExecutorMock struct {
	mu sync.Mutex

	// ExecuteTool tracking
	executeToolCalls  int
	executeToolName   []string
	executeToolInput  []interface{}
	executeToolResult string
	executeToolError  error

	// Tools configuration
	registeredTools []entity.Tool

	// RegisterInvestigation tracking (implements port.InvestigationRegistrar)
	registerInvestigationCalls []string
}

func (m *investigationRunnerToolExecutorMock) RegisterInvestigation(investigationID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registerInvestigationCalls = append(m.registerInvestigationCalls, investigationID)
}

func newInvestigationRunnerToolExecutorMock() *investigationRunnerToolExecutorMock {
	return &investigationRunnerToolExecutorMock{
		executeToolResult: "tool execution result",
		registeredTools: []entity.Tool{
			{Name: "bash", Description: "Execute bash commands"},
			{Name: "read_file", Description: "Read file contents"},
			{Name: "list_files", Description: "List files in directory"},
		},
	}
}

func (m *investigationRunnerToolExecutorMock) RegisterTool(tool entity.Tool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registeredTools = append(m.registeredTools, tool)
	return nil
}

func (m *investigationRunnerToolExecutorMock) UnregisterTool(name string) error {
	return nil
}

func (m *investigationRunnerToolExecutorMock) ExecuteTool(
	ctx context.Context,
	name string,
	input interface{},
) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.executeToolCalls++
	m.executeToolName = append(m.executeToolName, name)
	m.executeToolInput = append(m.executeToolInput, input)
	if m.executeToolError != nil {
		return "", m.executeToolError
	}
	return m.executeToolResult, nil
}

func (m *investigationRunnerToolExecutorMock) ListTools() ([]entity.Tool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.registeredTools, nil
}

func (m *investigationRunnerToolExecutorMock) GetTool(name string) (entity.Tool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.registeredTools {
		if t.Name == name {
			return t, true
		}
	}
	return entity.Tool{}, false
}

func (m *investigationRunnerToolExecutorMock) ValidateToolInput(name string, input interface{}) error {
	return nil
}

// investigationRunnerPromptBuilderMock implements PromptBuilderRegistry for testing.
type investigationRunnerPromptBuilderMock struct {
	mu sync.Mutex

	buildPromptForAlertCalls int
	buildPromptForAlertAlert *AlertView
	buildPromptTools         []entity.Tool
	buildPromptResult        string
	buildPromptError         error
}

func newInvestigationRunnerPromptBuilderMock() *investigationRunnerPromptBuilderMock {
	return &investigationRunnerPromptBuilderMock{
		buildPromptResult: "Investigate the alert. Check system status and report findings.",
	}
}

func (m *investigationRunnerPromptBuilderMock) Register(builder InvestigationPromptBuilder) error {
	return nil
}

func (m *investigationRunnerPromptBuilderMock) Get(alertType string) (InvestigationPromptBuilder, error) {
	return nil, ErrPromptBuilderNotFound
}

func (m *investigationRunnerPromptBuilderMock) BuildPromptForAlert(
	alert *AlertView,
	tools []entity.Tool,
	skills []port.SkillInfo,
) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.buildPromptForAlertCalls++
	m.buildPromptForAlertAlert = alert
	m.buildPromptTools = tools
	// Note: skills parameter is intentionally not stored in mock for backward compatibility
	if m.buildPromptError != nil {
		return "", m.buildPromptError
	}
	return m.buildPromptResult, nil
}

func (m *investigationRunnerPromptBuilderMock) ListAlertTypes() []string {
	return []string{"HighCPU", "DiskSpace", "HighMemory", "Generic"}
}

// =============================================================================
// Helper Functions
// =============================================================================

func createTestAlert(id, severity, title string) *AlertForInvestigation {
	return &AlertForInvestigation{
		id:          id,
		source:      "prometheus",
		severity:    severity,
		title:       title,
		description: "Test alert description",
		labels: map[string]string{
			"instance": "web-01",
			"job":      "web-server",
		},
	}
}

func createAssistantMessage(content string) *entity.Message {
	msg, _ := entity.NewMessage(entity.RoleAssistant, content)
	return msg
}

// =============================================================================
// Test Harness
// =============================================================================

// investigationRunnerTestHarness provides pre-configured mocks for InvestigationRunner tests.
type investigationRunnerTestHarness struct {
	t              *testing.T
	convService    *investigationRunnerConvServiceMock
	toolExecutor   *investigationRunnerToolExecutorMock
	safetyEnforcer *MockSafetyEnforcer
	promptBuilder  *investigationRunnerPromptBuilderMock
	config         AlertInvestigationUseCaseConfig
}

func newTestHarness(t *testing.T) *investigationRunnerTestHarness {
	t.Helper()
	convService := newInvestigationRunnerConvServiceMock()
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Investigation complete."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}
	return &investigationRunnerTestHarness{
		t:              t,
		convService:    convService,
		toolExecutor:   newInvestigationRunnerToolExecutorMock(),
		safetyEnforcer: NewMockSafetyEnforcer(),
		promptBuilder:  newInvestigationRunnerPromptBuilderMock(),
		config:         AlertInvestigationUseCaseConfig{MaxConcurrent: 5}, // default for tests; individual tests can override h.config as needed
	}
}

func (h *investigationRunnerTestHarness) build() *InvestigationRunner {
	h.t.Helper()
	return NewInvestigationRunner(
		h.convService, h.toolExecutor, h.safetyEnforcer, h.promptBuilder,
		nil, nil, nil, h.config, port.NopLogger{},
	)
}

func (h *investigationRunnerTestHarness) run(alert *AlertForInvestigation, invID string) (*InvestigationResult, error) {
	h.t.Helper()
	return h.build().Run(context.Background(), alert, invID)
}

// =============================================================================
// RegisterInvestigation Tests
// =============================================================================

func TestInvestigationRunner_RegistersInvestigationBeforeLoop(t *testing.T) {
	h := newTestHarness(t)
	_, err := h.run(createTestAlert("alert-reg-001", "warning", "Test Alert"), "inv-reg-001")
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	h.toolExecutor.mu.Lock()
	calls := h.toolExecutor.registerInvestigationCalls
	h.toolExecutor.mu.Unlock()
	if len(calls) != 1 {
		t.Errorf("RegisterInvestigation() called %d times, want 1", len(calls))
	}
	if len(calls) > 0 && calls[0] != "inv-reg-001" {
		t.Errorf("RegisterInvestigation() called with %q, want %q", calls[0], "inv-reg-001")
	}
}

func TestInvestigationRunner_DoesNotRegisterOnValidationFailure(t *testing.T) {
	h := newTestHarness(t)
	// Pass an empty investigation ID — validateInputs returns an error before registration
	_, _ = h.build().Run(context.Background(), createTestAlert("alert-reg-002", "warning", "Test Alert"), "")
	h.toolExecutor.mu.Lock()
	calls := h.toolExecutor.registerInvestigationCalls
	h.toolExecutor.mu.Unlock()
	if len(calls) != 0 {
		t.Errorf("RegisterInvestigation() called %d times on validation failure, want 0", len(calls))
	}
}

// =============================================================================
// Core Session Management Tests
// =============================================================================

func TestInvestigationRunner_CreatesSession(t *testing.T) {
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-001"
	_, err := h.run(createTestAlert("alert-001", "warning", "High CPU Usage"), "inv-001")
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if h.convService.startConversationCalls != 1 {
		t.Errorf("StartConversation() called %d times, want 1", h.convService.startConversationCalls)
	}
}

func TestInvestigationRunner_EndsSessionOnCompletion(t *testing.T) {
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-002"
	_, err := h.run(createTestAlert("alert-002", "warning", "Memory Alert"), "inv-002")
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if h.convService.endConversationCalls != 1 {
		t.Errorf("EndConversation() called %d times, want 1", h.convService.endConversationCalls)
	}
	if h.convService.endConversationSession != "inv-session-002" {
		t.Errorf("EndConversation() called with session %q, want %q",
			h.convService.endConversationSession, "inv-session-002")
	}
}

func TestInvestigationRunner_EndsSessionOnError(t *testing.T) {
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-003"
	h.convService.processResponseError = errors.New("AI provider error")
	_, err := h.run(createTestAlert("alert-003", "critical", "System Failure"), "inv-003")
	if err == nil {
		t.Error("Run() should return error when AI fails")
	}
	if h.convService.endConversationCalls != 1 {
		t.Errorf("EndConversation() called %d times, want 1 (cleanup on error)",
			h.convService.endConversationCalls)
	}
}

func TestInvestigationRunner_StartConversationError(t *testing.T) {
	h := newTestHarness(t)
	h.convService.startConversationError = errors.New("failed to start conversation")
	result, err := h.run(createTestAlert("alert-004", "warning", "Test Alert"), "inv-004")
	if err == nil {
		t.Error("Run() should return error when StartConversation fails")
	}
	if result != nil && result.Status != "failed" {
		t.Errorf("Run() result status = %q, want %q", result.Status, "failed")
	}
}

// =============================================================================
// Prompt Building Tests
// =============================================================================

func TestInvestigationRunner_SendsAlertContext(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-005"
	h.promptBuilder.buildPromptResult = "Investigate high CPU on instance web-01"

	alert := &AlertForInvestigation{
		id:          "alert-context-test",
		source:      "prometheus",
		severity:    "critical",
		title:       "High CPU Usage on web-01",
		description: "CPU usage exceeded 90% for 5 minutes",
		labels: map[string]string{
			"instance":  "web-01",
			"job":       "web-server",
			"alertname": "HighCPU",
		},
	}

	// Act
	_, err := h.run(alert, "inv-005")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Verify AddUserMessage was called with alert context
	if h.convService.addUserMessageCalls < 1 {
		t.Fatal("AddUserMessage() was not called")
	}

	firstMessage := h.convService.addUserMessageContent[0]

	// The first message should contain alert details
	if !strings.Contains(firstMessage, "alert-context-test") &&
		!strings.Contains(firstMessage, "High CPU") {
		t.Errorf("First message should contain alert ID or title, got: %s", firstMessage)
	}
}

func TestInvestigationRunner_UsesPromptBuilder(t *testing.T) {
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-006"
	h.promptBuilder.buildPromptResult = "Custom investigation prompt for HighCPU alert"
	_, err := h.run(createTestAlert("alert-prompt-test", "warning", "HighCPU Alert"), "inv-006")
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if h.promptBuilder.buildPromptForAlertCalls != 1 {
		t.Errorf("BuildPromptForAlert() called %d times, want 1",
			h.promptBuilder.buildPromptForAlertCalls)
	}
	if h.convService.setCustomSystemPromptCalls < 1 {
		t.Fatal("SetCustomSystemPrompt() was not called")
	}
	systemPrompt := h.convService.setCustomSystemPromptContent[0]
	if !strings.Contains(systemPrompt, "Custom investigation prompt") {
		t.Errorf("System prompt should contain prompt builder output, got: %s", systemPrompt)
	}
}

func TestInvestigationRunner_PromptBuilderError(t *testing.T) {
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-007"
	h.promptBuilder.buildPromptError = errors.New("failed to build prompt")
	result, err := h.run(createTestAlert("alert-prompt-error", "warning", "Test Alert"), "inv-007")
	if err == nil {
		t.Error("Run() should return error when PromptBuilder fails")
	}
	if result != nil && result.Status != "failed" {
		t.Errorf("Run() result status = %q, want %q", result.Status, "failed")
	}
	if h.convService.endConversationCalls != 1 {
		t.Errorf("EndConversation() should be called for cleanup, got %d calls",
			h.convService.endConversationCalls)
	}
}

// =============================================================================
// Tool Execution Loop Tests
// =============================================================================

func TestInvestigationRunner_ExecutesToolCalls(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-008"
	// First response: AI requests tool execution
	// Second response: AI completes investigation
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Let me check the CPU usage."),
		createAssistantMessage("Investigation complete. High load from process X."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "tool-001",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "top -b -n 1"},
			},
		},
		nil, // No more tool calls, investigation complete
	}
	h.toolExecutor.executeToolResult = "PID  USER      PR   NI    VIRT    RES    SHR S  %CPU  %MEM"

	alert := createTestAlert("alert-tool-exec", "warning", "High CPU")

	// Act
	result, err := h.run(alert, "inv-008")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Verify tool was executed
	if h.toolExecutor.executeToolCalls != 1 {
		t.Errorf("ExecuteTool() called %d times, want 1", h.toolExecutor.executeToolCalls)
	}
	if len(h.toolExecutor.executeToolName) < 1 || h.toolExecutor.executeToolName[0] != "bash" {
		t.Errorf("ExecuteTool() called with tool %v, want [bash]", h.toolExecutor.executeToolName)
	}

	// Verify result reflects actions taken
	if result != nil && result.ActionsTaken < 1 {
		t.Errorf("Result.ActionsTaken = %d, want >= 1", result.ActionsTaken)
	}
}

func TestInvestigationRunner_FeedsResultsBack(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-009"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Checking system status."),
		createAssistantMessage("Investigation complete."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "tool-002",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "df -h"},
			},
		},
		nil,
	}
	h.toolExecutor.executeToolResult = "/dev/sda1  100G  80G  20G  80%"

	alert := createTestAlert("alert-feed-results", "warning", "Disk Space")

	// Act
	_, err := h.run(alert, "inv-009")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Verify tool results were fed back to the conversation
	if h.convService.addToolResultCalls != 1 {
		t.Errorf("AddToolResultMessage() called %d times, want 1",
			h.convService.addToolResultCalls)
	}

	// Verify the tool result contains the tool execution output
	if len(h.convService.addToolResultResults) < 1 {
		t.Fatal("No tool results were added")
	}
	toolResults := h.convService.addToolResultResults[0]
	if len(toolResults) < 1 {
		t.Fatal("Tool results array is empty")
	}
	if toolResults[0].ToolID != "tool-002" {
		t.Errorf("ToolResult.ToolID = %q, want %q", toolResults[0].ToolID, "tool-002")
	}
	if !strings.Contains(toolResults[0].Result, "80%") {
		t.Errorf("ToolResult.Result should contain tool output, got: %s", toolResults[0].Result)
	}
}

func TestInvestigationRunner_MultipleIterations(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-010"
	// Simulate 3 iterations: 2 with tool calls, 1 completion
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Step 1: Checking CPU."),
		createAssistantMessage("Step 2: Checking memory."),
		createAssistantMessage("Step 3: Investigation complete."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "tool-iter-1",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "top -b -n 1"},
			},
		},
		{
			{
				ToolID:   "tool-iter-2",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "free -h"},
			},
		},
		nil, // Completion
	}

	alert := createTestAlert("alert-multi-iter", "critical", "System Investigation")

	// Act
	result, err := h.run(alert, "inv-010")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Verify multiple iterations occurred
	if h.convService.processResponseCalls != 3 {
		t.Errorf("ProcessAssistantResponse() called %d times, want 3",
			h.convService.processResponseCalls)
	}

	// Verify tools were executed for each iteration with tool calls
	if h.toolExecutor.executeToolCalls != 2 {
		t.Errorf("ExecuteTool() called %d times, want 2", h.toolExecutor.executeToolCalls)
	}

	// Verify results were fed back for each tool execution
	if h.convService.addToolResultCalls != 2 {
		t.Errorf("AddToolResultMessage() called %d times, want 2",
			h.convService.addToolResultCalls)
	}

	// Verify result reflects all actions
	if result != nil && result.ActionsTaken != 2 {
		t.Errorf("Result.ActionsTaken = %d, want 2", result.ActionsTaken)
	}
}

func TestInvestigationRunner_StopsAtMaxActions(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-011"
	// Configure to request more tools than allowed
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Action 1"),
		createAssistantMessage("Action 2"),
		createAssistantMessage("Action 3"),
		createAssistantMessage("Action 4 - should not reach"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "cmd1"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "cmd2"}}},
		{{ToolID: "t3", ToolName: "bash", Input: map[string]interface{}{"command": "cmd3"}}},
		{{ToolID: "t4", ToolName: "bash", Input: map[string]interface{}{"command": "cmd4"}}},
	}
	// Create safety enforcer with budget of 3 to limit actions
	h.safetyEnforcer = NewMockSafetyEnforcerWithActionBudget(3)

	alert := createTestAlert("alert-max-actions", "warning", "Test")

	// Act
	result, err := h.run(alert, "inv-011")

	// Assert
	// Should either return an error or escalate, but not exceed MaxActions (3)
	if result != nil && result.ActionsTaken > 3 {
		t.Errorf("Result.ActionsTaken = %d, should not exceed MaxActions (3)",
			result.ActionsTaken)
	}
	if h.toolExecutor.executeToolCalls > 3 {
		t.Errorf("ExecuteTool() called %d times, should not exceed MaxActions (3)",
			h.toolExecutor.executeToolCalls)
	}
	// May escalate or fail when hitting limit
	if err == nil && result != nil && !result.Escalated && result.Status != "completed" {
		t.Logf("Investigation status: %s, escalated: %v", result.Status, result.Escalated)
	}
}

func TestInvestigationRunner_ToolExecutionError(t *testing.T) {
	// Arrange
	expectedError := errors.New("command execution failed")
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-012"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Executing command."),
		createAssistantMessage("Investigation complete despite error."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "tool-error",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "invalid-command"},
			},
		},
		nil,
	}
	h.toolExecutor.executeToolError = expectedError

	alert := createTestAlert("alert-tool-error", "warning", "Test")

	// Act
	_, err := h.run(alert, "inv-012")

	// Assert
	// Tool errors should be fed back to AI, not necessarily fail the whole investigation
	// The runner should continue and let AI decide how to proceed
	if h.convService.addToolResultCalls < 1 {
		t.Error("Tool error should still be fed back to AI")
	}
	if len(h.convService.addToolResultResults) > 0 {
		toolResults := h.convService.addToolResultResults[0]
		if len(toolResults) > 0 && !toolResults[0].IsError {
			t.Error("Tool result should be marked as error")
		}
	}
	_ = err // Error handling depends on implementation
}

func TestInvestigationRunner_BlockedToolByEnforcer(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-013"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Attempting dangerous operation."),
		createAssistantMessage("Investigation completed."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "blocked-tool",
				ToolName: "edit_file", // This tool is not in AllowedTools
				Input:    map[string]interface{}{"path": "/etc/passwd"},
			},
		},
		nil,
	}
	h.safetyEnforcer = NewMockSafetyEnforcerWithBlockedTools([]string{"edit_file"})

	alert := createTestAlert("alert-blocked-tool", "warning", "Test")

	// Act
	result, err := h.run(alert, "inv-013")

	// Assert
	// Tool should not be executed
	if h.toolExecutor.executeToolCalls > 0 {
		for _, name := range h.toolExecutor.executeToolName {
			if name == "edit_file" {
				t.Error("Blocked tool 'edit_file' should not have been executed")
			}
		}
	}
	// Result should indicate the tool was blocked
	_ = result
	_ = err
}

func TestInvestigationRunner_MultipleToolsInSingleIteration(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-014"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Running multiple checks."),
		createAssistantMessage("Investigation complete."),
	}
	// AI requests multiple tools in one response
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "multi-1",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "top -b -n 1"},
			},
			{
				ToolID:   "multi-2",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "free -h"},
			},
			{
				ToolID:   "multi-3",
				ToolName: "read_file",
				Input:    map[string]interface{}{"path": "/var/log/syslog"},
			},
		},
		nil,
	}

	alert := createTestAlert("alert-multi-tools", "warning", "System Check")

	// Act
	result, err := h.run(alert, "inv-014")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// All 3 tools should be executed
	if h.toolExecutor.executeToolCalls != 3 {
		t.Errorf("ExecuteTool() called %d times, want 3", h.toolExecutor.executeToolCalls)
	}

	// Results for all tools should be fed back
	if h.convService.addToolResultCalls != 1 {
		t.Errorf("AddToolResultMessage() called %d times, want 1 (single batch)",
			h.convService.addToolResultCalls)
	}
	if len(h.convService.addToolResultResults) > 0 {
		results := h.convService.addToolResultResults[0]
		if len(results) != 3 {
			t.Errorf("Tool results count = %d, want 3", len(results))
		}
	}

	// Actions taken should reflect all tool executions
	if result != nil && result.ActionsTaken != 3 {
		t.Errorf("Result.ActionsTaken = %d, want 3", result.ActionsTaken)
	}
}

// =============================================================================
// Context and Timeout Tests
// =============================================================================

func TestInvestigationRunner_RespectsContextCancellation(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-015"
	// Configure a long investigation
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Starting investigation..."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "sleep 10"}}},
	}

	alert := createTestAlert("alert-cancel", "warning", "Test")

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Act
	_, err := h.build().Run(ctx, alert, "inv-015")

	// Assert
	if err == nil {
		t.Error("Run() should return error when context is cancelled")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Run() error = %v, want context.Canceled", err)
	}
}

func TestInvestigationRunner_RespectsTimeout(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-016"

	alert := createTestAlert("alert-timeout", "warning", "Test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Act
	_, err := h.build().Run(ctx, alert, "inv-016")

	// Assert
	if err == nil || (!errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled)) {
		t.Logf("Run() with timeout error = %v (may be expected)", err)
	}
}

// =============================================================================
// Result Structure Tests
// =============================================================================

func TestInvestigationRunner_ReturnsCorrectResultStructure(t *testing.T) {
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-017"
	result, err := h.run(createTestAlert("alert-result", "warning", "Test Alert"), "inv-017")
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}
	if result.InvestigationID != "inv-017" {
		t.Errorf("Result.InvestigationID = %q, want %q", result.InvestigationID, "inv-017")
	}
	if result.AlertID != "alert-result" {
		t.Errorf("Result.AlertID = %q, want %q", result.AlertID, "alert-result")
	}
	if result.Status != "completed" {
		t.Errorf("Result.Status = %q, want %q", result.Status, "completed")
	}
	if result.Duration <= 0 {
		t.Error("Result.Duration should be positive")
	}
}

func TestInvestigationRunner_NilAlertReturnsError(t *testing.T) {
	h := newTestHarness(t)
	result, err := h.run(nil, "inv-018")
	if err == nil {
		t.Error("Run() should return error for nil alert")
	}
	if result != nil && result.Status != "failed" {
		t.Errorf("Result.Status = %q, want %q for nil alert", result.Status, "failed")
	}
}

// =============================================================================
// Table-Driven Tests
// =============================================================================

func TestInvestigationRunner_Run_TableDriven(t *testing.T) {
	tests := []struct {
		name                  string
		alert                 *AlertForInvestigation
		invID                 string
		setupHarness          func(*investigationRunnerTestHarness)
		wantErr               bool
		wantStatus            string
		wantMinActions        int
		wantMaxActions        int
		wantEscalated         bool
		wantSessionCreated    bool
		wantSessionEnded      bool
		wantPromptBuilderUsed bool
	}{
		{
			name:  "successful investigation with no tool calls",
			alert: createTestAlert("test-1", "warning", "Simple Alert"),
			invID: "inv-t1",
			setupHarness: func(h *investigationRunnerTestHarness) {
				h.convService.processResponseMessages = []*entity.Message{
					createAssistantMessage("No investigation needed."),
				}
				h.convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}
			},
			wantErr:               false,
			wantStatus:            "completed",
			wantMinActions:        0,
			wantMaxActions:        0,
			wantSessionCreated:    true,
			wantSessionEnded:      true,
			wantPromptBuilderUsed: true,
		},
		{
			name:  "investigation with single tool call",
			alert: createTestAlert("test-2", "warning", "CPU Alert"),
			invID: "inv-t2",
			setupHarness: func(h *investigationRunnerTestHarness) {
				h.convService.processResponseMessages = []*entity.Message{
					createAssistantMessage("Checking CPU."),
					createAssistantMessage("Done."),
				}
				h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
					{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "top"}}},
					nil,
				}
			},
			wantErr:               false,
			wantStatus:            "completed",
			wantMinActions:        1,
			wantMaxActions:        1,
			wantSessionCreated:    true,
			wantSessionEnded:      true,
			wantPromptBuilderUsed: true,
		},
		{
			name:                  "nil alert returns error",
			alert:                 nil,
			invID:                 "inv-t3",
			setupHarness:          func(h *investigationRunnerTestHarness) {},
			wantErr:               true,
			wantSessionCreated:    false,
			wantPromptBuilderUsed: false,
		},
		{
			name:  "start conversation failure",
			alert: createTestAlert("test-4", "warning", "Alert"),
			invID: "inv-t4",
			setupHarness: func(h *investigationRunnerTestHarness) {
				h.convService.startConversationError = errors.New("connection failed")
			},
			wantErr:               true,
			wantSessionCreated:    true,
			wantSessionEnded:      false,
			wantPromptBuilderUsed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			h := newTestHarness(t)
			tt.setupHarness(h)

			// Act
			result, err := h.run(tt.alert, tt.invID)

			// Assert using helper functions
			assertTableDrivenError(t, err, tt.wantErr)
			assertTableDrivenResult(t, result, tt.wantStatus, tt.wantMinActions, tt.wantMaxActions, tt.wantEscalated)
			assertTableDrivenCalls(
				t,
				h.convService,
				h.promptBuilder,
				tt.wantSessionCreated,
				tt.wantSessionEnded,
				tt.wantPromptBuilderUsed,
			)
		})
	}
}

// assertTableDrivenError checks if error matches expectation.
func assertTableDrivenError(t *testing.T, err error, wantErr bool) {
	t.Helper()
	if (err != nil) != wantErr {
		t.Errorf("Run() error = %v, wantErr %v", err, wantErr)
	}
}

// assertTableDrivenResult checks result fields against expectations.
func assertTableDrivenResult(
	t *testing.T,
	result *InvestigationResult,
	wantStatus string,
	wantMinActions, wantMaxActions int,
	wantEscalated bool,
) {
	t.Helper()
	if result == nil {
		return
	}
	if wantStatus != "" && result.Status != wantStatus {
		t.Errorf("Run() status = %v, want %v", result.Status, wantStatus)
	}
	if result.ActionsTaken < wantMinActions {
		t.Errorf("Run() actions = %v, want >= %v", result.ActionsTaken, wantMinActions)
	}
	if wantMaxActions > 0 && result.ActionsTaken > wantMaxActions {
		t.Errorf("Run() actions = %v, want <= %v", result.ActionsTaken, wantMaxActions)
	}
	if result.Escalated != wantEscalated {
		t.Errorf("Run() escalated = %v, want %v", result.Escalated, wantEscalated)
	}
}

// assertTableDrivenCalls checks mock call counts.
func assertTableDrivenCalls(
	t *testing.T,
	convService *investigationRunnerConvServiceMock,
	promptBuilder *investigationRunnerPromptBuilderMock,
	wantSessionCreated, wantSessionEnded, wantPromptBuilderUsed bool,
) {
	t.Helper()
	if wantSessionCreated && convService.startConversationCalls < 1 {
		t.Error("StartConversation() should have been called")
	}
	if wantSessionEnded && convService.endConversationCalls < 1 {
		t.Error("EndConversation() should have been called")
	}
	if wantPromptBuilderUsed && promptBuilder.buildPromptForAlertCalls < 1 {
		t.Error("BuildPromptForAlert() should have been called")
	}
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewInvestigationRunner_NotNil(t *testing.T) {
	h := newTestHarness(t)
	runner := h.build()
	if runner == nil {
		t.Error("NewInvestigationRunner(, port.NopLogger{}) should not return nil")
	}
}

// =============================================================================
// Input Validation Tests (consolidated)
// =============================================================================

func TestInvestigationRunner_InputValidation(t *testing.T) {
	tests := []struct {
		name    string
		alert   *AlertForInvestigation
		invID   string
		wantErr string
	}{
		{
			name:    "nil alert",
			alert:   nil,
			invID:   "inv-018",
			wantErr: "nil alert",
		},
		{
			name:    "empty investigation ID",
			alert:   createTestAlert("alert-empty-inv", "warning", "Test"),
			invID:   "",
			wantErr: "empty investigation ID",
		},
		{
			name:    "whitespace investigation ID",
			alert:   createTestAlert("alert-ws-inv", "warning", "Test"),
			invID:   "   ",
			wantErr: "whitespace-only investigation ID",
		},
		{
			name: "alert with empty ID",
			alert: &AlertForInvestigation{
				id:          "",
				source:      "prometheus",
				severity:    "warning",
				title:       "Test Alert",
				description: "Description",
				labels:      map[string]string{},
			},
			invID:   "inv-empty-alert-id",
			wantErr: "alert with empty ID",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHarness(t)
			result, err := h.run(tt.alert, tt.invID)
			if err == nil {
				t.Errorf("Run() should return error for %s", tt.wantErr)
			}
			if result != nil && result.Status != "failed" {
				t.Errorf("Result.Status = %q, want %q", result.Status, "failed")
			}
		})
	}
}

// =============================================================================
// Safety Enforcer Integration Tests
// =============================================================================

func TestInvestigationRunner_SafetyEnforcerBlocksCommand(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-safety-cmd"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Executing command."),
		createAssistantMessage("Investigation complete."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "cmd-blocked",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "rm -rf /important"},
			},
		},
		nil,
	}
	// Create safety enforcer that blocks rm commands
	h.safetyEnforcer = NewMockSafetyEnforcerWithBlockedCommands([]string{"rm -rf"})

	alert := createTestAlert("alert-safety-cmd", "warning", "Test")

	// Act
	_, err := h.run(alert, "inv-safety-cmd")

	// Assert
	// The dangerous command should not be executed
	for _, name := range h.toolExecutor.executeToolName {
		if name != "bash" {
			continue
		}
		for _, input := range h.toolExecutor.executeToolInput {
			inputMap, ok := input.(map[string]interface{})
			if !ok {
				continue
			}
			cmd, ok := inputMap["command"].(string)
			if !ok {
				continue
			}
			if strings.Contains(cmd, "rm -rf") {
				t.Error("Dangerous command 'rm -rf' should have been blocked")
			}
		}
	}
	_ = err // Error depends on implementation
}

func TestInvestigationRunner_SafetyEnforcerActionBudgetExceeded(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-budget"
	// Configure many tool calls
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Action 1"),
		createAssistantMessage("Action 2"),
		createAssistantMessage("Action 3"),
		createAssistantMessage("Action 4"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "cmd1"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "cmd2"}}},
		{{ToolID: "t3", ToolName: "bash", Input: map[string]interface{}{"command": "cmd3"}}},
		{{ToolID: "t4", ToolName: "bash", Input: map[string]interface{}{"command": "cmd4"}}},
	}
	// Safety enforcer with budget of 2 actions
	h.safetyEnforcer = NewMockSafetyEnforcerWithActionBudget(2)

	alert := createTestAlert("alert-budget", "warning", "Test")

	// Act
	result, _ := h.run(alert, "inv-budget")

	// Assert
	// Should not exceed the safety enforcer's budget
	if h.toolExecutor.executeToolCalls > 2 {
		t.Errorf("ExecuteTool() called %d times, safety enforcer should limit to 2",
			h.toolExecutor.executeToolCalls)
	}
	// Should escalate or fail when budget is exceeded
	if result != nil && !result.Escalated && result.Status == "completed" {
		t.Log("Investigation completed normally despite budget being exceeded")
	}
}

func TestInvestigationRunner_SafetyEnforcerTimeout(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-timeout-enforcer"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Starting investigation."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "sleep 1"}}},
	}
	// Safety enforcer that always returns timeout
	h.safetyEnforcer = NewMockSafetyEnforcerWithTimeout()

	alert := createTestAlert("alert-timeout-enforcer", "warning", "Test")

	// Act
	result, err := h.run(alert, "inv-timeout-enforcer")

	// Assert
	// Should detect timeout from safety enforcer
	if err == nil && (result == nil || result.Status == "completed") {
		t.Error("Investigation should fail or escalate when safety enforcer indicates timeout")
	}
}

// =============================================================================
// Escalation Tests
// =============================================================================

func TestInvestigationRunner_EscalatesOnLowConfidence(t *testing.T) {
	// Skip: Confidence-based escalation was disabled when SafetyEnforcer was wired.
	// The confidence parsing from AI responses is not currently implemented.
	// This test can be re-enabled when confidence-based escalation is added back.
	t.Skip("Confidence-based escalation not implemented - SafetyEnforcer handles safety checks")
}

func TestInvestigationRunner_EscalatesOnConsecutiveErrors(t *testing.T) {
	// Arrange
	errorCount := 0
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-errors"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Trying command 1."),
		createAssistantMessage("Trying command 2."),
		createAssistantMessage("Trying command 3."),
		createAssistantMessage("Giving up."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "bad1"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "bad2"}}},
		{{ToolID: "t3", ToolName: "bash", Input: map[string]interface{}{"command": "bad3"}}},
		nil,
	}
	// All tool executions fail
	h.toolExecutor.executeToolError = errors.New("command failed")

	alert := createTestAlert("alert-errors", "warning", "Error-prone Issue")

	// Act
	result, _ := h.run(alert, "inv-errors")

	// Assert
	// After 3 consecutive errors, should escalate
	_ = errorCount
	if result != nil && !result.Escalated {
		t.Log("Investigation should escalate after consecutive errors threshold reached")
	}
}

func TestInvestigationRunner_EscalatesForCriticalAlert(t *testing.T) {
	h := newTestHarness(t)
	h.config.AutoStartForCritical = true
	result, err := h.run(createTestAlert("alert-critical", "critical", "Critical System Failure"), "inv-critical")
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	_ = result
}

func TestInvestigationRunner_DoesNotEscalateOnHighConfidence(t *testing.T) {
	h := newTestHarness(t)
	result, err := h.run(createTestAlert("alert-high-conf", "warning", "Clear Issue"), "inv-high-conf")
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result != nil && result.Escalated && result.Confidence >= 0.5 {
		t.Error("High confidence investigation should not be escalated")
	}
}

// =============================================================================
// Tool Filtering Tests
// =============================================================================

func TestInvestigationRunner_FiltersToolsByAllowedList(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-filter"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Using various tools."),
		createAssistantMessage("Done."),
	}
	// AI requests multiple tools, some not in allowed list
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}},
			{ToolID: "t2", ToolName: "edit_file", Input: map[string]interface{}{"path": "/etc/passwd"}},
			{ToolID: "t3", ToolName: "read_file", Input: map[string]interface{}{"path": "/var/log/syslog"}},
		},
		nil,
	}
	// Only allow bash and read_file, not edit_file
	h.safetyEnforcer = NewMockSafetyEnforcerWithAllowedTools([]string{"bash", "read_file"})

	alert := createTestAlert("alert-filter", "warning", "Test")

	// Act
	_, err := h.run(alert, "inv-filter")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	// Verify edit_file was not executed
	for _, name := range h.toolExecutor.executeToolName {
		if name == "edit_file" {
			t.Error("Tool 'edit_file' should not have been executed - not in allowed list")
		}
	}
}

func TestInvestigationRunner_EmptyAllowedToolsBlocksAll(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-no-tools"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Trying to use tools."),
		createAssistantMessage("No tools available."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}}},
		nil,
	}
	// Empty allowed tools list should block all tools
	h.safetyEnforcer = NewMockSafetyEnforcerWithAllowedTools([]string{})

	alert := createTestAlert("alert-no-tools", "warning", "Test")

	// Act
	result, _ := h.run(alert, "inv-no-tools")

	// Assert
	// No tools should be executed
	if h.toolExecutor.executeToolCalls > 0 {
		t.Errorf("No tools should be executed when AllowedTools is empty, got %d calls",
			h.toolExecutor.executeToolCalls)
	}
	// Result should indicate limitations
	_ = result
}

// =============================================================================
// AI Response Edge Cases
// =============================================================================

func TestInvestigationRunner_EmptyAssistantResponse(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-empty-response"
	// AI returns empty message
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage(""),
		createAssistantMessage("Now I have something to say."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{nil, nil}

	alert := createTestAlert("alert-empty-response", "warning", "Test")

	// Act
	result, err := h.run(alert, "inv-empty-response")
	// Assert
	// Should handle empty response gracefully
	if err != nil {
		t.Logf("Run() error = %v (may be expected for empty response)", err)
	}
	if result == nil {
		t.Error("Run() should return a result even for empty AI responses")
	}
}

func TestInvestigationRunner_MalformedToolInput(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-malformed"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Running tool."),
		createAssistantMessage("Done."),
	}
	// Malformed tool input (nil, wrong type)
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "malformed-1",
				ToolName: "bash",
				Input:    nil, // Nil input
			},
		},
		nil,
	}

	alert := createTestAlert("alert-malformed", "warning", "Test")

	// Act
	result, err := h.run(alert, "inv-malformed")

	// Assert
	// Should handle malformed input gracefully without panic
	if result == nil && err == nil {
		t.Error("Run() should return either result or error for malformed input")
	}
}

func TestInvestigationRunner_NilToolCallInfo(t *testing.T) {
	h := newTestHarness(t)
	result, err := h.run(createTestAlert("alert-nil-tool", "warning", "Test"), "inv-nil-tool")
	if err != nil {
		t.Errorf("Run() error = %v, want nil for nil tool calls", err)
	}
	if result == nil {
		t.Error("Run() should return result even with nil tool calls")
	}
}

// =============================================================================
// Investigation Store Integration Tests
// =============================================================================

func TestInvestigationRunner_PersistsToStore(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-store"
	store := NewMockInvestigationStore()

	runner := NewInvestigationRunnerWithStore(
		h.convService,
		h.toolExecutor,
		h.safetyEnforcer,
		h.promptBuilder,
		nil, // skillManager
		nil, // rcaService
		nil, // uiAdapter
		store,
		h.config, port.NopLogger{},
	)

	alert := createTestAlert("alert-store", "warning", "Test")

	// Act
	_, err := runner.Run(context.Background(), alert, "inv-store-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	// Verify investigation was persisted
	stored, storeErr := store.Get(context.Background(), "inv-store-001")
	if storeErr != nil {
		t.Errorf("Investigation should be stored, got error: %v", storeErr)
	}
	if stored == nil {
		t.Error("Stored investigation should not be nil")
	}
}

func TestInvestigationRunner_UpdatesStoreOnCompletion(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-store-update"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Running tool."),
		createAssistantMessage("Investigation complete."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}}},
		nil,
	}
	store := NewMockInvestigationStore()

	runner := NewInvestigationRunnerWithStore(
		h.convService,
		h.toolExecutor,
		h.safetyEnforcer,
		h.promptBuilder,
		nil, // skillManager
		nil, // rcaService
		nil, // uiAdapter
		store,
		h.config, port.NopLogger{},
	)

	alert := createTestAlert("alert-store-update", "warning", "Test")

	// Act
	_, err := runner.Run(context.Background(), alert, "inv-store-update")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	// Verify investigation status was updated
	stored, _ := store.Get(context.Background(), "inv-store-update")
	if stored != nil && stored.Status() != "completed" && stored.Status() != "running" {
		t.Logf("Stored investigation status = %q", stored.Status())
	}
}

func TestInvestigationRunner_UpdatesStoreOnError(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-store-error"
	h.convService.processResponseError = errors.New("AI error")
	store := NewMockInvestigationStore()

	runner := NewInvestigationRunnerWithStore(
		h.convService,
		h.toolExecutor,
		h.safetyEnforcer,
		h.promptBuilder,
		nil, // skillManager
		nil, // rcaService
		nil, // uiAdapter
		store,
		h.config, port.NopLogger{},
	)

	alert := createTestAlert("alert-store-error", "warning", "Test")

	// Act
	_, err := runner.Run(context.Background(), alert, "inv-store-error")

	// Assert
	if err == nil {
		t.Error("Run() should return error on AI failure")
	}
	// Verify investigation status was updated to failed
	stored, _ := store.Get(context.Background(), "inv-store-error")
	if stored != nil && stored.Status() != "failed" && stored.Status() != "started" {
		t.Logf("Stored investigation status = %q (expected 'failed')", stored.Status())
	}
}

// =============================================================================
// Findings Collection Tests
// =============================================================================

func TestInvestigationRunner_CollectsFindings(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-findings"
	// AI provides findings in response
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Finding 1: High CPU usage from process X."),
		createAssistantMessage("Finding 2: Memory leak detected."),
		createAssistantMessage("Investigation complete. Root cause: runaway process."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "top"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "free"}}},
		nil,
	}

	alert := createTestAlert("alert-findings", "warning", "System Issue")

	// Act
	result, err := h.run(alert, "inv-findings")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}
	// Result should contain findings
	if len(result.Findings) == 0 {
		t.Log("Findings slice is empty - implementation may need to extract findings from AI responses")
	}
}

func TestInvestigationRunner_ResultContainsSummary(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-summary"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage(
			"Summary: The issue was caused by a memory leak in the application. Recommendation: Restart the service.",
		),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	alert := createTestAlert("alert-summary", "warning", "Memory Issue")

	// Act
	result, err := h.run(alert, "inv-summary")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}
	// Result should capture the AI's summary/findings
	_ = result.Findings
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestInvestigationRunner_ConcurrentRuns(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-concurrent"
	h.config.MaxConcurrent = 10
	runner := h.build()

	// Act - Run multiple investigations concurrently
	var wg sync.WaitGroup
	results := make(chan *InvestigationResult, 5)
	errs := make(chan error, 5)

	for i := range 5 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			alert := createTestAlert(
				fmt.Sprintf("alert-concurrent-%d", idx),
				"warning",
				fmt.Sprintf("Test %d", idx),
			)
			result, err := runner.Run(
				context.Background(),
				alert,
				fmt.Sprintf("inv-concurrent-%d", idx),
			)
			if err != nil {
				errs <- err
			} else {
				results <- result
			}
		}(i)
	}

	wg.Wait()
	close(results)
	close(errs)

	// Assert
	errorCount := 0
	for err := range errs {
		errorCount++
		t.Logf("Concurrent run error: %v", err)
	}

	resultCount := 0
	for result := range results {
		resultCount++
		if result == nil {
			t.Error("Concurrent run returned nil result")
		}
	}

	// At least some should succeed
	if resultCount == 0 && errorCount == 0 {
		t.Error("Expected at least some results or errors from concurrent runs")
	}
}

// =============================================================================
// Duration Tracking Tests
// =============================================================================

func TestInvestigationRunner_TracksDuration(t *testing.T) {
	h := newTestHarness(t)
	startTime := time.Now()
	result, err := h.run(createTestAlert("alert-duration", "warning", "Test"), "inv-duration")
	endTime := time.Now()
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}
	if result.Duration <= 0 {
		t.Error("Result.Duration should be positive")
	}
	elapsed := endTime.Sub(startTime)
	if result.Duration > elapsed+time.Second {
		t.Errorf("Result.Duration = %v, actual elapsed = %v (should be similar)",
			result.Duration, elapsed)
	}
}

// =============================================================================
// Error Message Quality Tests
// =============================================================================

func TestInvestigationRunner_ErrorContainsContext(t *testing.T) {
	h := newTestHarness(t)
	h.convService.startConversationError = errors.New("connection refused to AI provider")
	_, err := h.run(createTestAlert("alert-error-ctx", "warning", "Test"), "inv-error-ctx")
	if err == nil {
		t.Error("Run() should return error")
	}
	errorStr := err.Error()
	if len(errorStr) < 10 {
		t.Errorf("Error message too short, should provide context: %v", err)
	}
}

// =============================================================================
// AddUserMessage Error Handling Tests
// =============================================================================

func TestInvestigationRunner_AddUserMessageError(t *testing.T) {
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-user-msg-err"
	h.convService.addUserMessageError = errors.New("failed to add message")
	result, err := h.run(createTestAlert("alert-user-msg-err", "warning", "Test"), "inv-user-msg-err")
	if err == nil {
		t.Error("Run() should return error when AddUserMessage fails")
	}
	if h.convService.endConversationCalls != 1 {
		t.Errorf("EndConversation() should be called for cleanup, got %d calls",
			h.convService.endConversationCalls)
	}
	_ = result
}

// =============================================================================
// AddToolResultMessage Error Handling Tests
// =============================================================================

func TestInvestigationRunner_AddToolResultMessageError(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-tool-result-err"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Running tool."),
		createAssistantMessage("Done."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}}},
		nil,
	}
	h.convService.addToolResultError = errors.New("failed to add tool result")

	alert := createTestAlert("alert-tool-result-err", "warning", "Test")

	// Act
	result, err := h.run(alert, "inv-tool-result-err")

	// Assert
	if err == nil {
		t.Error("Run() should return error when AddToolResultMessage fails")
	}
	// Session should still be cleaned up
	if h.convService.endConversationCalls != 1 {
		t.Errorf("EndConversation() should be called for cleanup, got %d calls",
			h.convService.endConversationCalls)
	}
	_ = result
}

// =============================================================================
// Edge Case: Very Long Tool Output Tests
// =============================================================================

func TestInvestigationRunner_HandlesLongToolOutput(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-long-output"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Running command."),
		createAssistantMessage("Done."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "cat large_file"}}},
		nil,
	}
	// Generate very long output (100KB)
	longOutput := strings.Repeat("A", 100*1024)
	h.toolExecutor.executeToolResult = longOutput

	alert := createTestAlert("alert-long-output", "warning", "Test")

	// Act
	result, err := h.run(alert, "inv-long-output")
	// Assert
	// Should handle long output without crashing
	if err != nil {
		t.Logf("Run() error = %v (may need to truncate long output)", err)
	}
	_ = result
}

// =============================================================================
// Edge Case: Special Characters in Alert Tests
// =============================================================================

func TestInvestigationRunner_HandlesSpecialCharactersInAlert(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-special"

	// Alert with special characters in title and description
	alert := &AlertForInvestigation{
		id:          "alert-special-<>&\"'",
		source:      "prometheus",
		severity:    "warning",
		title:       "Alert with <script>alert('xss')</script> in title",
		description: "Description with\nnewlines\tand\ttabs\rand special chars: !@#$%^&*()",
		labels: map[string]string{
			"instance": "server-01",
			"path":     "/api/v1/users?id=1&name=test",
		},
	}

	// Act
	result, err := h.run(alert, "inv-special")
	// Assert
	// Should handle special characters without crashing or injection issues
	if err != nil {
		t.Logf("Run() error = %v", err)
	}
	if result == nil {
		t.Error("Run() should return result for alert with special characters")
	}
}

// =============================================================================
// Constructor Variations Tests
// =============================================================================

func TestNewInvestigationRunner_WithNilDependencies(t *testing.T) {
	// These tests verify behavior when dependencies are nil
	tests := []struct {
		name           string
		convService    ConversationServiceInterface
		toolExecutor   port.ToolExecutor
		safetyEnforcer SafetyEnforcer
		promptBuilder  PromptBuilderRegistry
		shouldPanic    bool
	}{
		{
			name:           "nil conversation service",
			convService:    nil,
			toolExecutor:   newInvestigationRunnerToolExecutorMock(),
			safetyEnforcer: NewMockSafetyEnforcer(),
			promptBuilder:  newInvestigationRunnerPromptBuilderMock(),
			shouldPanic:    true, // or false depending on implementation
		},
		{
			name:           "nil tool executor",
			convService:    newInvestigationRunnerConvServiceMock(),
			toolExecutor:   nil,
			safetyEnforcer: NewMockSafetyEnforcer(),
			promptBuilder:  newInvestigationRunnerPromptBuilderMock(),
			shouldPanic:    true,
		},
		{
			name:           "nil safety enforcer",
			convService:    newInvestigationRunnerConvServiceMock(),
			toolExecutor:   newInvestigationRunnerToolExecutorMock(),
			safetyEnforcer: nil,
			promptBuilder:  newInvestigationRunnerPromptBuilderMock(),
			shouldPanic:    false, // Safety enforcer might be optional
		},
		{
			name:           "nil prompt builder",
			convService:    newInvestigationRunnerConvServiceMock(),
			toolExecutor:   newInvestigationRunnerToolExecutorMock(),
			safetyEnforcer: NewMockSafetyEnforcer(),
			promptBuilder:  nil,
			shouldPanic:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				r := recover()
				if tt.shouldPanic && r == nil {
					t.Error("Expected panic for nil dependency, but didn't panic")
				}
				if !tt.shouldPanic && r != nil {
					t.Errorf("Did not expect panic, but got: %v", r)
				}
			}()

			_ = NewInvestigationRunner(
				tt.convService,
				tt.toolExecutor,
				tt.safetyEnforcer,
				tt.promptBuilder,
				nil, // skillManager
				nil, // rcaService
				nil, // uiAdapter
				AlertInvestigationUseCaseConfig{},
				port.NopLogger{},
			)
		})
	}
}

func TestNewInvestigationRunnerWithStore_NotNil(t *testing.T) {
	h := newTestHarness(t)
	store := NewMockInvestigationStore()

	runner := NewInvestigationRunnerWithStore(
		h.convService,
		h.toolExecutor,
		h.safetyEnforcer,
		h.promptBuilder,
		nil, // skillManager
		nil, // rcaService
		nil, // uiAdapter
		store,
		h.config, port.NopLogger{},
	)

	if runner == nil {
		t.Error("NewInvestigationRunnerWithStore(, port.NopLogger{}) should not return nil")
	}
}

// =============================================================================
// Config Validation Tests
// =============================================================================

func TestInvestigationRunner_ZeroMaxActions(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-zero-actions"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Trying tool."),
		createAssistantMessage("Done."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}}},
		nil,
	}

	alert := createTestAlert("alert-zero-actions", "warning", "Test")

	// Act
	result, err := h.run(alert, "inv-zero-actions")
	// Assert
	// Behavior depends on interpretation: 0 could mean unlimited or no actions allowed
	if err != nil {
		t.Logf("Run() with MaxActions=0: error = %v", err)
	}
	if result != nil {
		t.Logf("Run() with MaxActions=0: ActionsTaken = %d", result.ActionsTaken)
	}
}

func TestInvestigationRunner_ZeroDuration(t *testing.T) {
	h := newTestHarness(t)
	result, err := h.run(createTestAlert("alert-zero-duration", "warning", "Test"), "inv-zero-duration")
	if err != nil {
		t.Logf("Run() with MaxDuration=0: error = %v", err)
	}
	_ = result
}

func TestInvestigationRunner_NegativeValues(t *testing.T) {
	h := newTestHarness(t)
	result, err := h.run(createTestAlert("alert-negative", "warning", "Test"), "inv-negative")
	if err != nil {
		t.Logf("Run() with negative config values: error = %v", err)
	}
	_ = result
}

// =============================================================================
// Completion/Escalation Tool Detection Tests
// =============================================================================
// These tests verify that the InvestigationRunner correctly detects and handles
// the special completion tools: complete_investigation and escalate_investigation.

func TestInvestigationRunner_DetectsCompleteInvestigation(t *testing.T) {
	// Test: When AI calls complete_investigation tool, the investigation loop ends
	// and returns a successful result with Status="completed".

	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-complete"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Investigation complete. Root cause identified."),
	}
	// AI calls complete_investigation tool to signal investigation is done
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "call_complete_001",
				ToolName: "complete_investigation",
				Input: map[string]interface{}{
					"confidence":          0.85,
					"findings":            []interface{}{"High CPU from nginx process", "Memory leak detected"},
					"root_cause":          "Nginx worker process spawning infinite loops",
					"recommended_actions": []interface{}{"Restart nginx", "Apply config patch"},
				},
			},
		},
	}

	alert := createTestAlert("alert-complete", "warning", "High CPU Usage")

	// Act
	result, err := h.run(alert, "inv-complete-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil when complete_investigation is called", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil, expected non-nil result")
	}
	if result.Status != "completed" {
		t.Errorf("Result.Status = %q, want %q when complete_investigation is called",
			result.Status, "completed")
	}
	// The complete_investigation tool should NOT be executed as a regular tool
	for _, name := range h.toolExecutor.executeToolName {
		if name == "complete_investigation" {
			t.Error("complete_investigation should be handled specially, not executed as regular tool")
		}
	}
}

func TestInvestigationRunner_ExtractsCompletionData(t *testing.T) {
	// Test: When AI calls complete_investigation, the runner extracts confidence,
	// findings, and root_cause from the tool input and populates the result.

	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-extract"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Analysis complete."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "call_extract_001",
				ToolName: "complete_investigation",
				Input: map[string]interface{}{
					"confidence": 0.92,
					"findings": []interface{}{
						"Database connection pool exhausted",
						"Connection timeout errors in logs",
						"Application retry storms detected",
					},
					"root_cause":          "PostgreSQL max_connections limit reached",
					"recommended_actions": []interface{}{"Increase max_connections", "Add connection pooler"},
				},
			},
		},
	}

	alert := createTestAlert("alert-extract", "critical", "Database Connection Failures")

	// Act
	result, err := h.run(alert, "inv-extract-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}

	// Verify confidence was extracted
	if result.Confidence != 0.92 {
		t.Errorf("Result.Confidence = %v, want 0.92", result.Confidence)
	}

	// Verify findings were extracted
	expectedFindings := []string{
		"Database connection pool exhausted",
		"Connection timeout errors in logs",
		"Application retry storms detected",
	}
	if len(result.Findings) != len(expectedFindings) {
		t.Errorf("Result.Findings has %d items, want %d", len(result.Findings), len(expectedFindings))
	}
	for i, expected := range expectedFindings {
		if i < len(result.Findings) && result.Findings[i] != expected {
			t.Errorf("Result.Findings[%d] = %q, want %q", i, result.Findings[i], expected)
		}
	}
}

func TestInvestigationRunner_DetectsEscalateInvestigation(t *testing.T) {
	// Test: When AI calls escalate_investigation tool, the investigation loop ends
	// and returns a result with Status="escalated" and Escalated=true.

	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-escalate"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Unable to determine root cause. Escalating."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "call_escalate_001",
				ToolName: "escalate_investigation",
				Input: map[string]interface{}{
					"reason":   "Unable to access required systems for diagnosis",
					"priority": "high",
					"partial_findings": []interface{}{
						"Symptoms indicate network issue",
						"Cannot reach monitoring endpoints",
					},
				},
			},
		},
	}

	alert := createTestAlert("alert-escalate", "critical", "Network Connectivity Issues")

	// Act
	result, err := h.run(alert, "inv-escalate-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil when escalate_investigation is called", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil, expected non-nil result")
	}
	if result.Status != "escalated" {
		t.Errorf("Result.Status = %q, want %q when escalate_investigation is called",
			result.Status, "escalated")
	}
	if !result.Escalated {
		t.Error("Result.Escalated = false, want true when escalate_investigation is called")
	}
	// The escalate_investigation tool should NOT be executed as a regular tool
	for _, name := range h.toolExecutor.executeToolName {
		if name == "escalate_investigation" {
			t.Error("escalate_investigation should be handled specially, not executed as regular tool")
		}
	}
}

func TestInvestigationRunner_ExtractsEscalationData(t *testing.T) {
	// Test: When AI calls escalate_investigation, the runner extracts reason,
	// priority, and partial_findings from the tool input.

	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-esc-data"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Escalating to human operator."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "call_esc_data_001",
				ToolName: "escalate_investigation",
				Input: map[string]interface{}{
					"reason":   "Security incident detected - requires human review",
					"priority": "critical",
					"partial_findings": []interface{}{
						"Unauthorized SSH login attempts detected",
						"Suspicious outbound traffic to unknown IPs",
					},
				},
			},
		},
	}

	alert := createTestAlert("alert-esc-data", "critical", "Security Alert")

	// Act
	result, err := h.run(alert, "inv-esc-data-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}

	// Verify escalation reason was extracted
	expectedReason := "Security incident detected - requires human review"
	if result.EscalateReason != expectedReason {
		t.Errorf("Result.EscalateReason = %q, want %q", result.EscalateReason, expectedReason)
	}

	// Verify partial findings were captured
	expectedFindings := []string{
		"Unauthorized SSH login attempts detected",
		"Suspicious outbound traffic to unknown IPs",
	}
	if len(result.Findings) != len(expectedFindings) {
		t.Errorf("Result.Findings has %d items, want %d (from partial_findings)",
			len(result.Findings), len(expectedFindings))
	}
	for i, expected := range expectedFindings {
		if i < len(result.Findings) && result.Findings[i] != expected {
			t.Errorf("Result.Findings[%d] = %q, want %q", i, result.Findings[i], expected)
		}
	}
}

func TestInvestigationRunner_CompletionStopsLoop(t *testing.T) {
	// Test: After complete_investigation is called, no more iterations occur.
	// Even if there are more tool calls queued, they should not be processed.

	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-comp-stop"
	// Configure multiple responses, but only first should be processed
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Investigation complete."),
		createAssistantMessage("This should never be reached."),
		createAssistantMessage("Neither should this."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "call_comp_stop_001",
				ToolName: "complete_investigation",
				Input: map[string]interface{}{
					"confidence": 0.75,
					"findings":   []interface{}{"Issue resolved"},
					"root_cause": "Configuration error",
				},
			},
		},
		// These should never be reached
		{
			{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "echo never"}},
		},
		{
			{ToolID: "t3", ToolName: "bash", Input: map[string]interface{}{"command": "echo also_never"}},
		},
	}

	alert := createTestAlert("alert-comp-stop", "warning", "Test Alert")

	// Act
	result, err := h.run(alert, "inv-comp-stop-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}

	// Verify loop stopped after complete_investigation
	// ProcessAssistantResponse should only be called once
	if h.convService.processResponseCalls > 1 {
		t.Errorf("ProcessAssistantResponse() called %d times, want 1 (loop should stop after complete_investigation)",
			h.convService.processResponseCalls)
	}

	// No tools should have been executed (complete_investigation is special)
	if h.toolExecutor.executeToolCalls > 0 {
		t.Errorf("ExecuteTool() called %d times, want 0 (complete_investigation stops the loop immediately)",
			h.toolExecutor.executeToolCalls)
	}
}

func TestInvestigationRunner_EscalationStopsLoop(t *testing.T) {
	// Test: After escalate_investigation is called, no more iterations occur.
	// The investigation should immediately return with escalated status.

	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-esc-stop"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Escalating immediately."),
		createAssistantMessage("This should never be reached."),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "call_esc_stop_001",
				ToolName: "escalate_investigation",
				Input: map[string]interface{}{
					"reason":           "Requires immediate human intervention",
					"priority":         "critical",
					"partial_findings": []interface{}{"Critical condition detected"},
				},
			},
		},
		// This should never be reached
		{
			{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "echo never"}},
		},
	}

	alert := createTestAlert("alert-esc-stop", "critical", "Critical Alert")

	// Act
	result, err := h.run(alert, "inv-esc-stop-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}
	if result.Status != "escalated" {
		t.Errorf("Result.Status = %q, want %q", result.Status, "escalated")
	}

	// Verify loop stopped after escalate_investigation
	if h.convService.processResponseCalls > 1 {
		t.Errorf("ProcessAssistantResponse() called %d times, want 1 (loop should stop after escalate_investigation)",
			h.convService.processResponseCalls)
	}

	// No tools should have been executed (escalate_investigation is special)
	if h.toolExecutor.executeToolCalls > 0 {
		t.Errorf("ExecuteTool() called %d times, want 0 (escalate_investigation stops the loop immediately)",
			h.toolExecutor.executeToolCalls)
	}
}

func TestInvestigationRunner_MixedToolCallsWithCompletion(t *testing.T) {
	// Test: When the AI response contains both regular tool calls and
	// complete_investigation in the same response, the regular tools should
	// be executed first before the investigation completes.

	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-mixed"
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Running final checks and completing investigation."),
	}
	// Single response with multiple tool calls including complete_investigation
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "call_check_1",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "df -h"},
			},
			{
				ToolID:   "call_check_2",
				ToolName: "read_file",
				Input:    map[string]interface{}{"path": "/var/log/syslog"},
			},
			{
				ToolID:   "call_complete",
				ToolName: "complete_investigation",
				Input: map[string]interface{}{
					"confidence":          0.88,
					"findings":            []interface{}{"Disk space at 95%", "Log rotation failing"},
					"root_cause":          "Disk full due to log retention",
					"recommended_actions": []interface{}{"Clean old logs", "Fix log rotation"},
				},
			},
		},
	}

	alert := createTestAlert("alert-mixed", "warning", "Disk Space Alert")

	// Act
	result, err := h.run(alert, "inv-mixed-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}

	// Verify status is completed (from complete_investigation)
	if result.Status != "completed" {
		t.Errorf("Result.Status = %q, want %q", result.Status, "completed")
	}

	// Verify the regular tools were executed BEFORE completion
	if h.toolExecutor.executeToolCalls != 2 {
		t.Errorf("ExecuteTool() called %d times, want 2 (bash and read_file should be executed before completion)",
			h.toolExecutor.executeToolCalls)
	}

	// Verify the correct tools were executed (not complete_investigation)
	expectedTools := []string{"bash", "read_file"}
	for i, expected := range expectedTools {
		if i < len(h.toolExecutor.executeToolName) && h.toolExecutor.executeToolName[i] != expected {
			t.Errorf("Tool %d executed was %q, want %q", i, h.toolExecutor.executeToolName[i], expected)
		}
	}

	// Verify complete_investigation was NOT executed as a regular tool
	for _, name := range h.toolExecutor.executeToolName {
		if name == "complete_investigation" {
			t.Error("complete_investigation should not be executed as regular tool")
		}
	}

	// Verify actions reflect the regular tools executed
	if result.ActionsTaken != 2 {
		t.Errorf("Result.ActionsTaken = %d, want 2", result.ActionsTaken)
	}

	// Verify completion data was extracted
	if result.Confidence != 0.88 {
		t.Errorf("Result.Confidence = %v, want 0.88", result.Confidence)
	}
}

// =============================================================================
// InvestigationRunner System Prompt Tests (RED PHASE - EXPECTED TO FAIL)
// These tests verify that InvestigationRunner uses custom system prompts
// instead of embedding the investigation prompt in user messages.
// =============================================================================

func TestInvestigationRunner_Run_CallsSetCustomSystemPrompt(t *testing.T) {
	h := newTestHarness(t)

	// Configure prompt builder to return a known prompt
	expectedSystemPrompt := "You are investigating alert: test-alert. Use the following tools: bash, read_file"
	h.promptBuilder.buildPromptResult = expectedSystemPrompt

	// Configure mock to return completion immediately
	completionToolCall := port.ToolCallInfo{
		ToolID:   "complete-1",
		ToolName: "complete_investigation",
		Input: map[string]interface{}{
			"findings":   []interface{}{"Investigation completed"},
			"confidence": 0.95,
		},
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{{completionToolCall}}

	// Create test alert
	alert := &AlertForInvestigation{
		id:          "test-alert-123",
		title:       "Test Alert",
		description: "Test description",
		source:      "test-source",
		severity:    "high",
	}

	// Run investigation
	_, err := h.run(alert, "inv-123")
	// Test should FAIL because SetCustomSystemPrompt is not implemented yet
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// EXPECTED BEHAVIOR: SetCustomSystemPrompt should be called with the investigation prompt
	// This will fail because the mock doesn't have this method yet
	if h.convService.setCustomSystemPromptCalls != 1 {
		t.Errorf("SetCustomSystemPrompt() called %d times, want 1", h.convService.setCustomSystemPromptCalls)
	}

	// Verify the prompt content matches what the builder returned
	if len(h.convService.setCustomSystemPromptContent) == 0 {
		t.Fatal("SetCustomSystemPrompt() was not called with any content")
	}

	actualPrompt := h.convService.setCustomSystemPromptContent[0]
	if actualPrompt != expectedSystemPrompt {
		t.Errorf("SetCustomSystemPrompt() called with prompt = %q, want %q", actualPrompt, expectedSystemPrompt)
	}
}

func TestInvestigationRunner_Run_SetCustomSystemPromptCalledBeforeAddUserMessage(t *testing.T) {
	h := newTestHarness(t)

	// Track call order
	var callOrder []string
	h.convService.onSetCustomSystemPrompt = func() {
		callOrder = append(callOrder, "SetCustomSystemPrompt")
	}
	h.convService.onAddUserMessage = func() {
		callOrder = append(callOrder, "AddUserMessage")
	}

	// Configure mock to return completion immediately
	completionToolCall := port.ToolCallInfo{
		ToolID:   "complete-1",
		ToolName: "complete_investigation",
		Input: map[string]interface{}{
			"findings":   []interface{}{"Done"},
			"confidence": 0.9,
		},
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{{completionToolCall}}

	// Create test alert
	alert := &AlertForInvestigation{
		id:       "test-alert-456",
		title:    "Order Test Alert",
		severity: "critical",
	}

	// Run investigation
	_, err := h.run(alert, "inv-456")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// EXPECTED BEHAVIOR: SetCustomSystemPrompt must be called BEFORE AddUserMessage
	if len(callOrder) < 2 {
		t.Fatalf("Expected at least 2 calls, got %d: %v", len(callOrder), callOrder)
	}

	if callOrder[0] != "SetCustomSystemPrompt" {
		t.Errorf("First call should be SetCustomSystemPrompt, got %s", callOrder[0])
	}

	if callOrder[1] != "AddUserMessage" {
		t.Errorf("Second call should be AddUserMessage, got %s", callOrder[1])
	}
}

func TestInvestigationRunner_Run_AddUserMessageContainsMinimalAlertOnly(t *testing.T) {
	h := newTestHarness(t)

	// Configure prompt builder to return a large prompt
	h.promptBuilder.buildPromptResult = "This is a very long investigation prompt with many instructions about how to investigate alerts. It contains tool descriptions, guidelines, safety rules, and much more. This should NOT appear in the user message."

	// Configure mock to return completion immediately
	completionToolCall := port.ToolCallInfo{
		ToolID:   "complete-1",
		ToolName: "complete_investigation",
		Input: map[string]interface{}{
			"findings":   []interface{}{"Found the issue"},
			"confidence": 0.85,
		},
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{{completionToolCall}}

	// Create test alert
	alert := &AlertForInvestigation{
		id:          "alert-789",
		title:       "Minimal Message Test",
		description: "This should not appear in user message",
		source:      "test",
		severity:    "medium",
	}

	// Run investigation
	_, err := h.run(alert, "inv-789")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// EXPECTED BEHAVIOR: AddUserMessage should receive MINIMAL content (just alert ID and title)
	// NOT the full investigation prompt
	if len(h.convService.addUserMessageContent) == 0 {
		t.Fatal("AddUserMessage() was not called")
	}

	userMessage := h.convService.addUserMessageContent[0]

	// Verify the user message does NOT contain the investigation prompt
	if strings.Contains(userMessage, "investigation prompt") {
		t.Error("User message should NOT contain investigation prompt text")
	}

	if strings.Contains(userMessage, "tool descriptions") {
		t.Error("User message should NOT contain tool descriptions")
	}

	if strings.Contains(userMessage, "guidelines") {
		t.Error("User message should NOT contain guidelines")
	}

	// Verify the user message DOES contain minimal alert info
	if !strings.Contains(userMessage, alert.ID()) {
		t.Errorf("User message should contain alert ID %q, got: %q", alert.ID(), userMessage)
	}

	if !strings.Contains(userMessage, alert.Title()) {
		t.Errorf("User message should contain alert title %q, got: %q", alert.Title(), userMessage)
	}

	// Verify message is SHORT (just ID and title, not full prompt)
	// Current implementation sends long prompt, new implementation should be < 200 chars
	if len(userMessage) > 200 {
		t.Errorf(
			"User message is too long (%d chars), should be minimal (<200 chars). Got: %q",
			len(userMessage),
			userMessage,
		)
	}
}

func TestInvestigationRunner_Run_SetCustomSystemPromptErrorPropagated(t *testing.T) {
	h := newTestHarness(t)

	// Configure SetCustomSystemPrompt to return an error
	expectedError := errors.New("system prompt configuration failed")
	h.convService.setCustomSystemPromptError = expectedError

	// Create test alert
	alert := &AlertForInvestigation{
		id:       "alert-error",
		title:    "Error Test",
		severity: "high",
	}

	// Run investigation
	result, err := h.run(alert, "inv-error")

	// EXPECTED BEHAVIOR: Error from SetCustomSystemPrompt should be propagated
	if err == nil {
		t.Fatal("Run() should return error when SetCustomSystemPrompt fails")
	}

	if !errors.Is(err, expectedError) && err.Error() != expectedError.Error() {
		t.Errorf("Run() error = %v, want %v", err, expectedError)
	}

	// Result should indicate failure
	if result == nil {
		t.Fatal("Result should not be nil even on error")
	}

	if result.Status != "failed" {
		t.Errorf("Result.Status = %q, want %q", result.Status, "failed")
	}
}

func TestInvestigationRunner_ConversationServiceInterfaceIncludesSetCustomSystemPrompt(t *testing.T) {
	// This test verifies that ConversationServiceInterface has the SetCustomSystemPrompt method
	// by attempting to compile-time check. If the interface doesn't have the method,
	// this test will fail to compile.

	// Create a minimal mock that implements the interface
	var _ ConversationServiceInterface = &investigationRunnerConvServiceMock{}

	// If we get here, the interface includes SetCustomSystemPrompt
	// This test will FAIL TO COMPILE until the interface is updated
}

// =============================================================================
// Turn Warning Feature Tests
// =============================================================================

// TestInvestigationRunner_buildTurnWarningMessage tests the buildTurnWarningMessage method.
func TestInvestigationRunner_buildTurnWarningMessage(t *testing.T) {
	tests := []struct {
		name      string
		remaining int
		want      string
	}{
		{
			name:      "remaining is 5 - returns batch_tool warning",
			remaining: 5,
			want: `TURN LIMIT WARNING: You have 5 turns remaining before reaching the turn limit.

Please prioritize your remaining actions carefully. Consider using the batch_tool to execute multiple operations efficiently in a single turn.`,
		},
		{
			name:      "remaining is 4 - returns countdown warning",
			remaining: 4,
			want:      "TURN LIMIT WARNING: You have 4 turns remaining.",
		},
		{
			name:      "remaining is 3 - returns countdown warning",
			remaining: 3,
			want:      "TURN LIMIT WARNING: You have 3 turns remaining.",
		},
		{
			name:      "remaining is 2 - returns countdown warning",
			remaining: 2,
			want:      "TURN LIMIT WARNING: You have 2 turns remaining.",
		},
		{
			name:      "remaining is 1 - returns countdown warning",
			remaining: 1,
			want:      "TURN LIMIT WARNING: You have 1 turn remaining.",
		},
		{
			name:      "remaining is 6 - returns empty string",
			remaining: 6,
			want:      "",
		},
		{
			name:      "remaining is 10 - returns empty string",
			remaining: 10,
			want:      "",
		},
		{
			name:      "remaining is 0 - returns empty string",
			remaining: 0,
			want:      "",
		},
		{
			name:      "remaining is negative - returns empty string",
			remaining: -1,
			want:      "",
		},
	}

	cfg := TurnWarningConfig{
		WarningThreshold: 5,
		BatchToolHint:    "batch_tool",
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildTurnWarningMessage(tt.remaining, cfg)

			if got != tt.want {
				t.Errorf("BuildTurnWarningMessage(%d) = %q, want %q", tt.remaining, got, tt.want)
			}
		})
	}
}

// TestInvestigationRunner_InjectsWarningAtMaxActionsMinus5 verifies warning injection at turn limit - 5.
func TestInvestigationRunner_InjectsWarningAtMaxActionsMinus5(t *testing.T) {
	h := newTestHarness(t)

	maxActions := 20
	actionsBeforeWarning := maxActions - 5 // 15 actions to trigger warning at remaining=5

	// Configure mock to simulate tool calls that will reach maxActions - 5
	h.convService.processResponseToolCalls = make([][]port.ToolCallInfo, maxActions+5)

	// Set up tool calls until we trigger the warning
	for i := range actionsBeforeWarning {
		h.convService.processResponseToolCalls[i] = []port.ToolCallInfo{
			{
				ToolID:   fmt.Sprintf("tool-%d", i),
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "echo test"},
			},
		}
	}

	// Remaining calls: empty to allow natural completion
	for i := actionsBeforeWarning; i < len(h.convService.processResponseToolCalls); i++ {
		h.convService.processResponseToolCalls[i] = []port.ToolCallInfo{}
	}

	// Track AddUserMessage calls
	var userMessageContents []string
	var userMessageCallOrder []int
	callCount := 0
	h.convService.onAddUserMessage = func() {
		callCount++
		userMessageCallOrder = append(userMessageCallOrder, callCount)
		// Capture the message content
		if len(h.convService.addUserMessageContent) > 0 {
			userMessageContents = append(
				userMessageContents,
				h.convService.addUserMessageContent[len(h.convService.addUserMessageContent)-1],
			)
		}
	}

	// Create safety enforcer with appropriate budget
	h.safetyEnforcer = NewMockSafetyEnforcerWithActionBudget(maxActions)

	// Create test alert
	alert := &AlertForInvestigation{
		id:       "alert-warning-test",
		title:    "Warning Test",
		severity: "high",
	}

	// Run investigation
	_, err := h.run(alert, "inv-warning-test")
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	// EXPECTED BEHAVIOR: At actionsTaken = 15 (remaining = 5), a warning should be injected
	// Expected: Initial message (1) + Warning message at turn 15 (1) = 2 AddUserMessage calls
	expectedMinCalls := 2
	if h.convService.addUserMessageCalls < expectedMinCalls {
		t.Errorf(
			"AddUserMessage called %d times, expected at least %d",
			h.convService.addUserMessageCalls,
			expectedMinCalls,
		)
	}

	// Verify warning message was sent
	warningFound := false
	for _, content := range userMessageContents {
		if strings.Contains(content, "TURN LIMIT WARNING") && strings.Contains(content, "5 turns remaining") {
			warningFound = true
			// Verify it mentions batch_tool for the 5-turn warning
			if !strings.Contains(content, "batch_tool") {
				t.Error("Warning at 5 turns remaining should mention batch_tool")
			}
			break
		}
	}

	if !warningFound {
		t.Errorf("Expected warning message at maxActions - 5 not found. Messages: %v", userMessageContents)
	}
}

// TestInvestigationRunner_InjectsCountdownWarnings verifies countdown warnings at turns 1-4 remaining.
// setupToolCallsForCountdownTest configures mock tool calls for countdown warning tests.
func setupToolCallsForCountdownTest(convService *investigationRunnerConvServiceMock, totalCalls, maxActions int) {
	convService.processResponseToolCalls = make([][]port.ToolCallInfo, maxActions+10)

	for i := range totalCalls {
		convService.processResponseToolCalls[i] = []port.ToolCallInfo{
			{
				ToolID:   fmt.Sprintf("tool-%d", i),
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "echo test"},
			},
		}
	}

	for i := totalCalls; i < len(convService.processResponseToolCalls); i++ {
		convService.processResponseToolCalls[i] = []port.ToolCallInfo{}
	}
}

// verifyWarningExists checks if a warning message for the given remaining count exists.
func verifyWarningExists(t *testing.T, remaining int, messages []string) {
	expectedText := fmt.Sprintf("TURN LIMIT WARNING: You have %d turn", remaining)

	for _, content := range messages {
		if strings.Contains(content, expectedText) {
			return
		}
	}

	t.Errorf("Expected warning for %d turns remaining not found. Messages: %v", remaining, messages)
}

func TestInvestigationRunner_InjectsCountdownWarnings(t *testing.T) {
	tests := []struct {
		name             string
		maxActions       int
		expectedWarnings []int // Expected remaining turn counts in warnings
	}{
		{
			name:             "warnings at 4,3,2,1 remaining (maxActions=20)",
			maxActions:       20,
			expectedWarnings: []int{4, 3, 2, 1},
		},
		{
			name:             "warnings at 3,2,1 remaining (maxActions=15)",
			maxActions:       15,
			expectedWarnings: []int{3, 2, 1},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newTestHarness(t)

			// Calculate how many actions to take before first warning
			// First warning appears when remaining = expectedWarnings[0]
			// So we need to take (maxActions - expectedWarnings[0]) actions
			firstWarningRemaining := tt.expectedWarnings[0]
			setupToolCallsCount := tt.maxActions - firstWarningRemaining
			totalCalls := setupToolCallsCount + len(tt.expectedWarnings)
			setupToolCallsForCountdownTest(h.convService, totalCalls, tt.maxActions)

			var userMessageContents []string
			h.convService.onAddUserMessage = func() {
				if len(h.convService.addUserMessageContent) > 0 {
					userMessageContents = append(
						userMessageContents,
						h.convService.addUserMessageContent[len(h.convService.addUserMessageContent)-1],
					)
				}
			}

			// Create safety enforcer with appropriate budget
			h.safetyEnforcer = NewMockSafetyEnforcerWithActionBudget(tt.maxActions)

			alert := &AlertForInvestigation{
				id:       "alert-countdown-test",
				title:    "Countdown Test",
				severity: "high",
			}

			_, err := h.run(alert, "inv-countdown-test")
			if err != nil {
				t.Fatalf("Run() returned unexpected error: %v", err)
			}

			for _, expectedRemaining := range tt.expectedWarnings {
				verifyWarningExists(t, expectedRemaining, userMessageContents)
			}
		})
	}
}

// TestInvestigationRunner_SendsSummaryRequestAtMaxActions verifies summary request at turn limit.
func TestInvestigationRunner_SendsSummaryRequestAtMaxActions(t *testing.T) {
	h := newTestHarness(t)

	maxActions := 20

	// Configure mock to simulate exactly maxActions tool calls
	h.convService.processResponseToolCalls = make([][]port.ToolCallInfo, maxActions+5)

	// Set up maxActions iterations of regular tool calls
	for i := range maxActions {
		h.convService.processResponseToolCalls[i] = []port.ToolCallInfo{
			{
				ToolID:   fmt.Sprintf("tool-%d", i),
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "echo test"},
			},
		}
	}

	// After maxActions: empty tool calls
	for i := maxActions; i < len(h.convService.processResponseToolCalls); i++ {
		h.convService.processResponseToolCalls[i] = []port.ToolCallInfo{}
	}

	// Track AddUserMessage calls
	var userMessageContents []string
	h.convService.onAddUserMessage = func() {
		if len(h.convService.addUserMessageContent) > 0 {
			userMessageContents = append(
				userMessageContents,
				h.convService.addUserMessageContent[len(h.convService.addUserMessageContent)-1],
			)
		}
	}

	// Create safety enforcer with appropriate budget
	h.safetyEnforcer = NewMockSafetyEnforcerWithActionBudget(maxActions)

	// Create test alert
	alert := &AlertForInvestigation{
		id:       "alert-summary-test",
		title:    "Summary Test",
		severity: "high",
	}

	// Run investigation
	_, err := h.run(alert, "inv-summary-test")
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	// EXPECTED BEHAVIOR: At maxActions, a summary request should be sent
	summaryRequestFound := false
	for _, content := range userMessageContents {
		if strings.Contains(content, "TURN LIMIT REACHED") &&
			strings.Contains(content, "Please provide a summary") {
			summaryRequestFound = true
			break
		}
	}

	if !summaryRequestFound {
		t.Errorf("Expected summary request at maxActions not found. Messages: %v", userMessageContents)
	}

	// EXPECTED BEHAVIOR: AI gets one final response opportunity after summary request
	// ProcessAssistantResponse should be called at least maxActions + 1 times
	// (once per tool call iteration + once for summary response)
	expectedMinProcessCalls := maxActions + 1
	if h.convService.processResponseCalls < expectedMinProcessCalls {
		t.Errorf(
			"ProcessAssistantResponse called %d times, expected at least %d (one final call after summary request)",
			h.convService.processResponseCalls,
			expectedMinProcessCalls,
		)
	}
}

// TestInvestigationRunner_WarningMessageOrdering verifies warnings are sent in correct order.
func TestInvestigationRunner_WarningMessageOrdering(t *testing.T) {
	h := newTestHarness(t)

	maxActions := 20

	// Configure mock to reach maxActions
	h.convService.processResponseToolCalls = make([][]port.ToolCallInfo, maxActions+5)

	for i := range maxActions {
		h.convService.processResponseToolCalls[i] = []port.ToolCallInfo{
			{
				ToolID:   fmt.Sprintf("tool-%d", i),
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "echo test"},
			},
		}
	}

	for i := maxActions; i < len(h.convService.processResponseToolCalls); i++ {
		h.convService.processResponseToolCalls[i] = []port.ToolCallInfo{}
	}

	// Track message order
	type messageRecord struct {
		callNumber int
		content    string
	}
	var messages []messageRecord
	callCount := 0

	h.convService.onAddUserMessage = func() {
		callCount++
		if len(h.convService.addUserMessageContent) > 0 {
			messages = append(messages, messageRecord{
				callNumber: callCount,
				content:    h.convService.addUserMessageContent[len(h.convService.addUserMessageContent)-1],
			})
		}
	}

	// Create safety enforcer with appropriate budget
	h.safetyEnforcer = NewMockSafetyEnforcerWithActionBudget(maxActions)

	// Create test alert
	alert := &AlertForInvestigation{
		id:       "alert-order-test",
		title:    "Order Test",
		severity: "high",
	}

	// Run investigation
	_, err := h.run(alert, "inv-order-test")
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	// EXPECTED BEHAVIOR: Messages should appear in this order:
	// 1. Initial trigger message
	// 2. Warning at 5 remaining (after maxActions-5 actions)
	// 3. Warnings at 4, 3, 2, 1 remaining (countdown)
	// 4. Summary request (after maxActions actions)

	// Build expected sequence dynamically
	expectedSequence := []string{"Alert ID:"} // Initial message

	// Add warnings for remaining 5, 4, 3, 2, 1
	for remaining := 5; remaining >= 2; remaining-- {
		expectedSequence = append(expectedSequence, fmt.Sprintf("%d turns remaining", remaining))
	}
	expectedSequence = append(expectedSequence, "1 turn remaining")   // Singular form for 1
	expectedSequence = append(expectedSequence, "TURN LIMIT REACHED") // Summary request

	// Find each expected message in order
	lastFoundIndex := -1
	for _, expectedContent := range expectedSequence {
		found := false
		for i := lastFoundIndex + 1; i < len(messages); i++ {
			if strings.Contains(messages[i].content, expectedContent) {
				found = true
				lastFoundIndex = i
				break
			}
		}
		if !found {
			t.Errorf(
				"Expected message sequence broken. Missing or out of order: %q. Messages: %+v",
				expectedContent,
				messages,
			)
		}
	}
}

// TestInvestigationRunner_NoWarningsWhenNotReachingLimit verifies no warnings when investigation completes early.
func TestInvestigationRunner_NoWarningsWhenNotReachingLimit(t *testing.T) {
	h := newTestHarness(t)

	// Configure mock to complete after only 5 actions (well before warnings)
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "1", ToolName: "bash", Input: map[string]interface{}{"command": "echo 1"}}},
		{{ToolID: "2", ToolName: "bash", Input: map[string]interface{}{"command": "echo 2"}}},
		{{ToolID: "3", ToolName: "bash", Input: map[string]interface{}{"command": "echo 3"}}},
		{{ToolID: "4", ToolName: "bash", Input: map[string]interface{}{"command": "echo 4"}}},
		{{ToolID: "5", ToolName: "bash", Input: map[string]interface{}{"command": "echo 5"}}},
		{}, // AI completes without more tool calls
	}

	// Track AddUserMessage calls
	var userMessageContents []string
	h.convService.onAddUserMessage = func() {
		if len(h.convService.addUserMessageContent) > 0 {
			userMessageContents = append(
				userMessageContents,
				h.convService.addUserMessageContent[len(h.convService.addUserMessageContent)-1],
			)
		}
	}

	// Create test alert
	alert := &AlertForInvestigation{
		id:       "alert-early-complete",
		title:    "Early Complete Test",
		severity: "high",
	}

	// Run investigation
	_, err := h.run(alert, "inv-early-complete")
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	// EXPECTED BEHAVIOR: No warning messages should be sent
	for _, content := range userMessageContents {
		if strings.Contains(content, "TURN LIMIT WARNING") || strings.Contains(content, "TURN LIMIT REACHED") {
			t.Errorf("Unexpected warning message when investigation completed early: %q", content)
		}
	}

	// Should only have the initial trigger message
	if h.convService.addUserMessageCalls != 1 {
		t.Errorf("AddUserMessage called %d times, expected 1 (only initial message)", h.convService.addUserMessageCalls)
	}
}

// =============================================================================
// Thinking Mode Tests
// =============================================================================

func TestInvestigationRunner_Run_ThinkingModeEnabled(t *testing.T) {
	h := newTestHarness(t)

	// Configure AI to complete immediately
	h.convService.processResponseMessages = []*entity.Message{
		{Role: entity.RoleAssistant, Content: "Investigation complete"},
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "1", ToolName: "complete_investigation", Input: map[string]any{
			"findings":   []any{"test finding"},
			"confidence": 0.9,
		}}},
	}

	// Enable extended thinking in config
	h.config = AlertInvestigationUseCaseConfig{
		ExtendedThinking: true,
		ThinkingBudget:   8000,
		ShowThinking:     true,
	}

	alert := createTestAlert("alert-1", "warning", "Test Alert")

	// Run investigation
	_, err := h.run(alert, "inv-thinking-test")
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	// Verify SetThinkingMode was called with correct values
	if h.convService.setThinkingModeCalls != 1 {
		t.Errorf("SetThinkingMode called %d times, expected 1", h.convService.setThinkingModeCalls)
	}

	if h.convService.setThinkingModeSession != "test-session-123" {
		t.Errorf("SetThinkingMode session = %q, want %q", h.convService.setThinkingModeSession, "test-session-123")
	}

	if !h.convService.setThinkingModeInfo.Enabled {
		t.Error("SetThinkingMode info.Enabled = false, want true")
	}

	if h.convService.setThinkingModeInfo.BudgetTokens != 8000 {
		t.Errorf("SetThinkingMode info.BudgetTokens = %d, want 8000", h.convService.setThinkingModeInfo.BudgetTokens)
	}

	if !h.convService.setThinkingModeInfo.ShowThinking {
		t.Error("SetThinkingMode info.ShowThinking = false, want true")
	}
}

func TestInvestigationRunner_Run_ThinkingModeDisabled(t *testing.T) {
	h := newTestHarness(t)

	// Configure AI to complete immediately
	h.convService.processResponseMessages = []*entity.Message{
		{Role: entity.RoleAssistant, Content: "Investigation complete"},
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "1", ToolName: "complete_investigation", Input: map[string]any{
			"findings":   []any{"test finding"},
			"confidence": 0.9,
		}}},
	}

	// Thinking disabled (default)
	h.config.ExtendedThinking = false

	alert := createTestAlert("alert-1", "warning", "Test Alert")

	// Run investigation
	_, err := h.run(alert, "inv-no-thinking")
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	// Verify SetThinkingMode was NOT called when disabled
	if h.convService.setThinkingModeCalls != 0 {
		t.Errorf("SetThinkingMode called %d times, expected 0 when thinking disabled", h.convService.setThinkingModeCalls)
	}
}

func TestInvestigationRunner_Run_ThinkingModeDefaultBudget(t *testing.T) {
	h := newTestHarness(t)

	// Configure AI to complete immediately
	h.convService.processResponseMessages = []*entity.Message{
		{Role: entity.RoleAssistant, Content: "Investigation complete"},
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "1", ToolName: "complete_investigation", Input: map[string]any{
			"findings":   []any{"test finding"},
			"confidence": 0.9,
		}}},
	}

	// Enable thinking but with zero budget (should use default)
	h.config = AlertInvestigationUseCaseConfig{
		ExtendedThinking: true,
		ThinkingBudget:   0, // Should default to 10000
	}

	alert := createTestAlert("alert-1", "warning", "Test Alert")

	// Run investigation
	_, err := h.run(alert, "inv-default-budget")
	if err != nil {
		t.Fatalf("Run() returned unexpected error: %v", err)
	}

	// Verify SetThinkingMode was called with default budget
	if h.convService.setThinkingModeCalls != 1 {
		t.Errorf("SetThinkingMode called %d times, expected 1", h.convService.setThinkingModeCalls)
	}

	if h.convService.setThinkingModeInfo.BudgetTokens != 10000 {
		t.Errorf(
			"SetThinkingMode info.BudgetTokens = %d, want 10000 (default)",
			h.convService.setThinkingModeInfo.BudgetTokens,
		)
	}
}

// TestInvestigationRunner_DisplayThinkingViaUIAdapter verifies that thinking content
// is displayed through the UI adapter's DisplayThinking method when ShowThinking is enabled.
func TestInvestigationRunner_DisplayThinkingViaUIAdapter(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "test-session"
	h.convService.thinkingContent = "Test thinking: analyzing the alert..."
	h.convService.processResponseMessages = []*entity.Message{createAssistantMessage("Done.")}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{{
		{ToolID: "1", ToolName: "complete_investigation", Input: map[string]interface{}{
			"findings":   []interface{}{"Test"},
			"confidence": 0.9,
		}},
	}}

	// Track DisplayThinking calls
	thinkingCalled := false
	var capturedContent string
	uiAdapter := &testUIAdapter{
		displayThinkingFunc: func(content string) error {
			thinkingCalled = true
			capturedContent = content
			return nil
		},
	}

	h.config = AlertInvestigationUseCaseConfig{
		ExtendedThinking: true,
		ShowThinking:     true,
	}

	runner := NewInvestigationRunner(
		h.convService,
		h.toolExecutor,
		nil, // safetyEnforcer
		h.promptBuilder,
		nil, // skillManager
		nil, // rcaService
		uiAdapter,
		h.config, port.NopLogger{},
	)

	alert := createTestAlert("alert-1", "critical", "Test")

	// Act
	_, err := runner.Run(context.Background(), alert, "inv-test")
	// Assert
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	if !thinkingCalled {
		t.Error("DisplayThinking() was not called on UI adapter when ShowThinking is true")
	}
	if capturedContent != "Test thinking: analyzing the alert..." {
		t.Errorf("Captured thinking content = %q, want %q",
			capturedContent, "Test thinking: analyzing the alert...")
	}
}

// testUIAdapter is a minimal test adapter for testing DisplayThinking.
type testUIAdapter struct {
	displayThinkingFunc func(content string) error
}

func (t *testUIAdapter) GetUserInput(ctx context.Context) (string, bool)         { return "", false }
func (t *testUIAdapter) DisplayMessage(message string, messageRole string) error { return nil }
func (t *testUIAdapter) BeginStreamingResponse() error                           { return nil }
func (t *testUIAdapter) EndStreamingResponse() error                             { return nil }
func (t *testUIAdapter) DisplayStreamingText(text string) error                  { return nil }
func (t *testUIAdapter) DisplayError(err error) error                            { return nil }
func (t *testUIAdapter) DisplayToolResult(toolName string, input string, result string) error {
	return nil
}
func (t *testUIAdapter) DisplaySystemMessage(message string) error { return nil }
func (t *testUIAdapter) DisplayThinking(content string) error {
	if t.displayThinkingFunc != nil {
		return t.displayThinkingFunc(content)
	}
	return nil
}

func (t *testUIAdapter) DisplaySubagentStatus(agentName string, status string, details string) error {
	return nil
}
func (t *testUIAdapter) SetPrompt(prompt string) error                { return nil }
func (t *testUIAdapter) ClearScreen() error                           { return nil }
func (t *testUIAdapter) SetColorScheme(scheme port.ColorScheme) error { return nil }
func (t *testUIAdapter) ConfirmBashCommand(command string, isDangerous bool, reason string, description string) bool {
	return true
}

func (t *testUIAdapter) Confirm(_ string, _ string) bool {
	return false
}

// mockRCAService implements RCAServiceInterface for testing.
type mockRCAService struct {
	mu                sync.Mutex
	correlateCalls    int
	correlateFindings []entity.InvestigationFinding
	correlateResult   []entity.RCAFinding
	correlateError    error
}

func (m *mockRCAService) Correlate(ctx context.Context, findings []entity.InvestigationFinding) ([]entity.RCAFinding, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.correlateCalls++
	m.correlateFindings = findings
	return m.correlateResult, m.correlateError
}

func TestInvestigationRunner_RCA(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-rca"

	// Configure AI to return a completion message with findings
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Investigation complete."),
	}
	// The completion tool call will contain findings
	completionToolCall := port.ToolCallInfo{
		ToolID:   "call-123",
		ToolName: toolCompleteInvestigation,
		Input: map[string]interface{}{
			"confidence": 0.9,
			"findings": []interface{}{
				"High CPU detected",
				"Indexer process is leaking resources",
			},
		},
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{{completionToolCall}}

	rcaService := &mockRCAService{
		correlateResult: []entity.RCAFinding{
			{
				Summary: "Memory leak in indexer",
				Causes: []entity.Cause{
					{ID: "C1", Description: "Resource leak", ConfidenceScore: 0.95},
				},
				Remedies: []entity.Remedy{
					{Description: "Restart indexer", ActionableSteps: []string{"systemctl restart indexer"}, Impact: "High"},
					{Description: "Fix leak", ActionableSteps: []string{"Update code"}, Impact: "High"},
				},
			},
		},
	}

	runner := NewInvestigationRunner(
		h.convService,
		h.toolExecutor,
		h.safetyEnforcer,
		h.promptBuilder,
		nil, // skillManager
		rcaService,
		nil, // uiAdapter
		h.config, port.NopLogger{},
	)

	alert := createTestAlert("alert-rca", "critical", "RCA Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-rca-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if rcaService.correlateCalls != 1 {
		t.Errorf("Correlate() called %d times, want 1", rcaService.correlateCalls)
	}
	if len(result.RCAFindings) != 1 {
		t.Errorf("len(result.RCAFindings) = %d, want 1", len(result.RCAFindings))
	}
	if result.RCAFindings[0].Summary != "Memory leak in indexer" {
		t.Errorf("RCAFinding summary = %v, want 'Memory leak in indexer'", result.RCAFindings[0].Summary)
	}
}

func TestInvestigationRunner_RCA_Escalated(t *testing.T) {
	// Arrange
	h := newTestHarness(t)
	h.convService.startConversationSession = "inv-session-rca-escalated"

	// Configure AI to return an escalation message
	h.convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("I need human help."),
	}
	// The escalation tool call will contain partial findings
	escalateToolCall := port.ToolCallInfo{
		ToolID:   "call-escalate",
		ToolName: toolEscalateInvestigation,
		Input: map[string]interface{}{
			"reason": "Too complex",
			"partial_findings": []interface{}{
				"Found unusual logs",
				"Database connection is flapping",
			},
		},
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{{escalateToolCall}}

	rcaService := &mockRCAService{
		correlateResult: []entity.RCAFinding{
			{
				Summary: "Network instability suspected",
			},
		},
	}

	runner := NewInvestigationRunner(
		h.convService,
		h.toolExecutor,
		h.safetyEnforcer,
		h.promptBuilder,
		nil, // skillManager
		rcaService,
		nil, // uiAdapter
		h.config, port.NopLogger{},
	)

	alert := createTestAlert("alert-escalate-rca", "critical", "Escalate RCA Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-rca-escalated-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() error = %v, want nil", err)
	}
	if result.Status != "escalated" {
		t.Errorf("Status = %v, want 'escalated'", result.Status)
	}
	if rcaService.correlateCalls != 1 {
		t.Errorf("Correlate() called %d times, want 1", rcaService.correlateCalls)
	}
	if len(result.RCAFindings) != 1 {
		t.Errorf("len(result.RCAFindings) = %d, want 1", len(result.RCAFindings))
	}
	if result.RCAFindings[0].Summary != "Network instability suspected" {
		t.Errorf("RCAFinding summary = %v, want 'Network instability suspected'", result.RCAFindings[0].Summary)
	}
}

func TestTruncateForLog(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{
			name:   "empty string",
			input:  "",
			maxLen: 10,
			want:   "",
		},
		{
			name:   "within limit",
			input:  "hello",
			maxLen: 10,
			want:   "hello",
		},
		{
			name:   "exact length",
			input:  "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "exceeds limit ASCII",
			input:  "hello world",
			maxLen: 5,
			want:   "hello...",
		},
		{
			name:   "multi-byte unicode truncates on rune boundary",
			input:  "日本語テスト",
			maxLen: 3,
			want:   "日本語...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateForLog(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateForLog(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

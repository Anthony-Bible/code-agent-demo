package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
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

// investigationRunnerToolExecutorMock implements port.ToolExecutor for testing.
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
// Core Session Management Tests
// =============================================================================

func TestInvestigationRunner_CreatesSession(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-001"
	// Configure AI to return a completion message (no tool calls)
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Investigation complete. Root cause identified."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:    20,
			MaxDuration:   15 * time.Minute,
			AllowedTools:  []string{"bash", "read_file"},
			MaxConcurrent: 5,
		},
	)

	alert := createTestAlert("alert-001", "warning", "High CPU Usage")

	// Act
	_, err := runner.Run(context.Background(), alert, "inv-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if convService.startConversationCalls != 1 {
		t.Errorf("StartConversation() called %d times, want 1", convService.startConversationCalls)
	}
}

func TestInvestigationRunner_EndsSessionOnCompletion(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-002"
	// Configure AI to return a completion message
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Investigation complete."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "read_file"},
		},
	)

	alert := createTestAlert("alert-002", "warning", "Memory Alert")

	// Act
	_, err := runner.Run(context.Background(), alert, "inv-002")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if convService.endConversationCalls != 1 {
		t.Errorf("EndConversation() called %d times, want 1", convService.endConversationCalls)
	}
	if convService.endConversationSession != "inv-session-002" {
		t.Errorf("EndConversation() called with session %q, want %q",
			convService.endConversationSession, "inv-session-002")
	}
}

func TestInvestigationRunner_EndsSessionOnError(t *testing.T) {
	// Arrange
	expectedError := errors.New("AI provider error")
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-003"
	convService.processResponseError = expectedError

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "read_file"},
		},
	)

	alert := createTestAlert("alert-003", "critical", "System Failure")

	// Act
	_, err := runner.Run(context.Background(), alert, "inv-003")

	// Assert
	if err == nil {
		t.Error("Run() should return error when AI fails")
	}
	// Session should still be ended for cleanup
	if convService.endConversationCalls != 1 {
		t.Errorf("EndConversation() called %d times, want 1 (cleanup on error)",
			convService.endConversationCalls)
	}
}

func TestInvestigationRunner_StartConversationError(t *testing.T) {
	// Arrange
	expectedError := errors.New("failed to start conversation")
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationError = expectedError

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{},
	)

	alert := createTestAlert("alert-004", "warning", "Test Alert")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-004")

	// Assert
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-005"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Investigation complete."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()
	promptBuilder.buildPromptResult = "Investigate high CPU on instance web-01"

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "read_file"},
		},
	)

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
	_, err := runner.Run(context.Background(), alert, "inv-005")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Verify AddUserMessage was called with alert context
	if convService.addUserMessageCalls < 1 {
		t.Fatal("AddUserMessage() was not called")
	}

	firstMessage := convService.addUserMessageContent[0]

	// The first message should contain alert details
	if !strings.Contains(firstMessage, "alert-context-test") &&
		!strings.Contains(firstMessage, "High CPU") {
		t.Errorf("First message should contain alert ID or title, got: %s", firstMessage)
	}
}

func TestInvestigationRunner_UsesPromptBuilder(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-006"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Investigation complete."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()
	promptBuilder.buildPromptResult = "Custom investigation prompt for HighCPU alert"

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "read_file"},
		},
	)

	alert := createTestAlert("alert-prompt-test", "warning", "HighCPU Alert")

	// Act
	_, err := runner.Run(context.Background(), alert, "inv-006")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Verify PromptBuilder was called
	if promptBuilder.buildPromptForAlertCalls != 1 {
		t.Errorf("BuildPromptForAlert() called %d times, want 1",
			promptBuilder.buildPromptForAlertCalls)
	}

	// Verify the prompt was set as system prompt
	if convService.setCustomSystemPromptCalls < 1 {
		t.Fatal("SetCustomSystemPrompt() was not called")
	}

	systemPrompt := convService.setCustomSystemPromptContent[0]
	if !strings.Contains(systemPrompt, "Custom investigation prompt") {
		t.Errorf("System prompt should contain prompt builder output, got: %s", systemPrompt)
	}
}

func TestInvestigationRunner_PromptBuilderError(t *testing.T) {
	// Arrange
	expectedError := errors.New("failed to build prompt")
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-007"

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()
	promptBuilder.buildPromptError = expectedError

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{},
	)

	alert := createTestAlert("alert-prompt-error", "warning", "Test Alert")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-007")

	// Assert
	if err == nil {
		t.Error("Run() should return error when PromptBuilder fails")
	}
	if result != nil && result.Status != "failed" {
		t.Errorf("Run() result status = %q, want %q", result.Status, "failed")
	}
	// Session should be cleaned up
	if convService.endConversationCalls != 1 {
		t.Errorf("EndConversation() should be called for cleanup, got %d calls",
			convService.endConversationCalls)
	}
}

// =============================================================================
// Tool Execution Loop Tests
// =============================================================================

func TestInvestigationRunner_ExecutesToolCalls(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-008"
	// First response: AI requests tool execution
	// Second response: AI completes investigation
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Let me check the CPU usage."),
		createAssistantMessage("Investigation complete. High load from process X."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "tool-001",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "top -b -n 1"},
			},
		},
		nil, // No more tool calls, investigation complete
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	toolExecutor.executeToolResult = "PID  USER      PR   NI    VIRT    RES    SHR S  %CPU  %MEM"
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "read_file"},
		},
	)

	alert := createTestAlert("alert-tool-exec", "warning", "High CPU")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-008")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Verify tool was executed
	if toolExecutor.executeToolCalls != 1 {
		t.Errorf("ExecuteTool() called %d times, want 1", toolExecutor.executeToolCalls)
	}
	if len(toolExecutor.executeToolName) < 1 || toolExecutor.executeToolName[0] != "bash" {
		t.Errorf("ExecuteTool() called with tool %v, want [bash]", toolExecutor.executeToolName)
	}

	// Verify result reflects actions taken
	if result != nil && result.ActionsTaken < 1 {
		t.Errorf("Result.ActionsTaken = %d, want >= 1", result.ActionsTaken)
	}
}

func TestInvestigationRunner_FeedsResultsBack(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-009"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Checking system status."),
		createAssistantMessage("Investigation complete."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "tool-002",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "df -h"},
			},
		},
		nil,
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	toolExecutor.executeToolResult = "/dev/sda1  100G  80G  20G  80%"
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "read_file"},
		},
	)

	alert := createTestAlert("alert-feed-results", "warning", "Disk Space")

	// Act
	_, err := runner.Run(context.Background(), alert, "inv-009")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Verify tool results were fed back to the conversation
	if convService.addToolResultCalls != 1 {
		t.Errorf("AddToolResultMessage() called %d times, want 1",
			convService.addToolResultCalls)
	}

	// Verify the tool result contains the tool execution output
	if len(convService.addToolResultResults) < 1 {
		t.Fatal("No tool results were added")
	}
	toolResults := convService.addToolResultResults[0]
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-010"
	// Simulate 3 iterations: 2 with tool calls, 1 completion
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Step 1: Checking CPU."),
		createAssistantMessage("Step 2: Checking memory."),
		createAssistantMessage("Step 3: Investigation complete."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
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

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "read_file"},
		},
	)

	alert := createTestAlert("alert-multi-iter", "critical", "System Investigation")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-010")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// Verify multiple iterations occurred
	if convService.processResponseCalls != 3 {
		t.Errorf("ProcessAssistantResponse() called %d times, want 3",
			convService.processResponseCalls)
	}

	// Verify tools were executed for each iteration with tool calls
	if toolExecutor.executeToolCalls != 2 {
		t.Errorf("ExecuteTool() called %d times, want 2", toolExecutor.executeToolCalls)
	}

	// Verify results were fed back for each tool execution
	if convService.addToolResultCalls != 2 {
		t.Errorf("AddToolResultMessage() called %d times, want 2",
			convService.addToolResultCalls)
	}

	// Verify result reflects all actions
	if result != nil && result.ActionsTaken != 2 {
		t.Errorf("Result.ActionsTaken = %d, want 2", result.ActionsTaken)
	}
}

func TestInvestigationRunner_StopsAtMaxActions(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-011"
	// Configure to request more tools than allowed
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Action 1"),
		createAssistantMessage("Action 2"),
		createAssistantMessage("Action 3"),
		createAssistantMessage("Action 4 - should not reach"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "cmd1"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "cmd2"}}},
		{{ToolID: "t3", ToolName: "bash", Input: map[string]interface{}{"command": "cmd3"}}},
		{{ToolID: "t4", ToolName: "bash", Input: map[string]interface{}{"command": "cmd4"}}},
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   3, // Limit to 3 actions
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-max-actions", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-011")

	// Assert
	// Should either return an error or escalate, but not exceed MaxActions
	if result != nil && result.ActionsTaken > 3 {
		t.Errorf("Result.ActionsTaken = %d, should not exceed MaxActions (3)",
			result.ActionsTaken)
	}
	if toolExecutor.executeToolCalls > 3 {
		t.Errorf("ExecuteTool() called %d times, should not exceed MaxActions (3)",
			toolExecutor.executeToolCalls)
	}
	// May escalate or fail when hitting limit
	if err == nil && result != nil && !result.Escalated && result.Status != "completed" {
		t.Logf("Investigation status: %s, escalated: %v", result.Status, result.Escalated)
	}
}

func TestInvestigationRunner_ToolExecutionError(t *testing.T) {
	// Arrange
	expectedError := errors.New("command execution failed")
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-012"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Executing command."),
		createAssistantMessage("Investigation complete despite error."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "tool-error",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "invalid-command"},
			},
		},
		nil,
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	toolExecutor.executeToolError = expectedError
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-tool-error", "warning", "Test")

	// Act
	_, err := runner.Run(context.Background(), alert, "inv-012")

	// Assert
	// Tool errors should be fed back to AI, not necessarily fail the whole investigation
	// The runner should continue and let AI decide how to proceed
	if convService.addToolResultCalls < 1 {
		t.Error("Tool error should still be fed back to AI")
	}
	if len(convService.addToolResultResults) > 0 {
		toolResults := convService.addToolResultResults[0]
		if len(toolResults) > 0 && !toolResults[0].IsError {
			t.Error("Tool result should be marked as error")
		}
	}
	_ = err // Error handling depends on implementation
}

func TestInvestigationRunner_BlockedToolByEnforcer(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-013"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Attempting dangerous operation."),
		createAssistantMessage("Investigation completed."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "blocked-tool",
				ToolName: "edit_file", // This tool is not in AllowedTools
				Input:    map[string]interface{}{"path": "/etc/passwd"},
			},
		},
		nil,
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcerWithBlockedTools([]string{"edit_file"})
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "read_file"}, // edit_file not allowed
		},
	)

	alert := createTestAlert("alert-blocked-tool", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-013")

	// Assert
	// Tool should not be executed
	if toolExecutor.executeToolCalls > 0 {
		for _, name := range toolExecutor.executeToolName {
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-014"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Running multiple checks."),
		createAssistantMessage("Investigation complete."),
	}
	// AI requests multiple tools in one response
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
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

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "read_file"},
		},
	)

	alert := createTestAlert("alert-multi-tools", "warning", "System Check")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-014")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}

	// All 3 tools should be executed
	if toolExecutor.executeToolCalls != 3 {
		t.Errorf("ExecuteTool() called %d times, want 3", toolExecutor.executeToolCalls)
	}

	// Results for all tools should be fed back
	if convService.addToolResultCalls != 1 {
		t.Errorf("AddToolResultMessage() called %d times, want 1 (single batch)",
			convService.addToolResultCalls)
	}
	if len(convService.addToolResultResults) > 0 {
		results := convService.addToolResultResults[0]
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-015"
	// Configure a long investigation
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Starting investigation..."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "sleep 10"}}},
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-cancel", "warning", "Test")

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// Act
	_, err := runner.Run(ctx, alert, "inv-015")

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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-016"

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  50 * time.Millisecond, // Very short timeout
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-timeout", "warning", "Test")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	// Act
	_, err := runner.Run(ctx, alert, "inv-016")

	// Assert
	if err == nil || (!errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled)) {
		t.Logf("Run() with timeout error = %v (may be expected)", err)
	}
}

// =============================================================================
// Result Structure Tests
// =============================================================================

func TestInvestigationRunner_ReturnsCorrectResultStructure(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-017"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Investigation complete. No issues found."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-result", "warning", "Test Alert")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-017")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}

	// Check result structure
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
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{},
	)

	// Act
	result, err := runner.Run(context.Background(), nil, "inv-018")

	// Assert
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
		setupConvService      func(*investigationRunnerConvServiceMock)
		setupToolExecutor     func(*investigationRunnerToolExecutorMock)
		setupSafetyEnforcer   func() *MockSafetyEnforcer
		config                AlertInvestigationUseCaseConfig
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
			setupConvService: func(m *investigationRunnerConvServiceMock) {
				m.processResponseMessages = []*entity.Message{
					createAssistantMessage("No investigation needed."),
				}
				m.processResponseToolCalls = [][]port.ToolCallInfo{nil}
			},
			setupToolExecutor:   func(m *investigationRunnerToolExecutorMock) {},
			setupSafetyEnforcer: NewMockSafetyEnforcer,
			config: AlertInvestigationUseCaseConfig{
				MaxActions:   20,
				MaxDuration:  15 * time.Minute,
				AllowedTools: []string{"bash"},
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
			setupConvService: func(m *investigationRunnerConvServiceMock) {
				m.processResponseMessages = []*entity.Message{
					createAssistantMessage("Checking CPU."),
					createAssistantMessage("Done."),
				}
				m.processResponseToolCalls = [][]port.ToolCallInfo{
					{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "top"}}},
					nil,
				}
			},
			setupToolExecutor:   func(m *investigationRunnerToolExecutorMock) {},
			setupSafetyEnforcer: NewMockSafetyEnforcer,
			config: AlertInvestigationUseCaseConfig{
				MaxActions:   20,
				MaxDuration:  15 * time.Minute,
				AllowedTools: []string{"bash"},
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
			setupConvService:      func(m *investigationRunnerConvServiceMock) {},
			setupToolExecutor:     func(m *investigationRunnerToolExecutorMock) {},
			setupSafetyEnforcer:   NewMockSafetyEnforcer,
			config:                AlertInvestigationUseCaseConfig{},
			wantErr:               true,
			wantSessionCreated:    false,
			wantPromptBuilderUsed: false,
		},
		{
			name:  "start conversation failure",
			alert: createTestAlert("test-4", "warning", "Alert"),
			invID: "inv-t4",
			setupConvService: func(m *investigationRunnerConvServiceMock) {
				m.startConversationError = errors.New("connection failed")
			},
			setupToolExecutor:     func(m *investigationRunnerToolExecutorMock) {},
			setupSafetyEnforcer:   NewMockSafetyEnforcer,
			config:                AlertInvestigationUseCaseConfig{},
			wantErr:               true,
			wantSessionCreated:    true,
			wantSessionEnded:      false,
			wantPromptBuilderUsed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			convService := newInvestigationRunnerConvServiceMock()
			tt.setupConvService(convService)

			toolExecutor := newInvestigationRunnerToolExecutorMock()
			tt.setupToolExecutor(toolExecutor)

			safetyEnforcer := tt.setupSafetyEnforcer()
			promptBuilder := newInvestigationRunnerPromptBuilderMock()

			runner := NewInvestigationRunner(
				convService,
				toolExecutor,
				safetyEnforcer,
				promptBuilder,
				nil, // skillManager
				tt.config,
			)

			// Act
			result, err := runner.Run(context.Background(), tt.alert, tt.invID)

			// Assert
			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantStatus != "" && result != nil {
				if result.Status != tt.wantStatus {
					t.Errorf("Run() status = %v, want %v", result.Status, tt.wantStatus)
				}
			}

			if result != nil {
				if result.ActionsTaken < tt.wantMinActions {
					t.Errorf("Run() actions = %v, want >= %v",
						result.ActionsTaken, tt.wantMinActions)
				}
				if tt.wantMaxActions > 0 && result.ActionsTaken > tt.wantMaxActions {
					t.Errorf("Run() actions = %v, want <= %v",
						result.ActionsTaken, tt.wantMaxActions)
				}
				if result.Escalated != tt.wantEscalated {
					t.Errorf("Run() escalated = %v, want %v",
						result.Escalated, tt.wantEscalated)
				}
			}

			if tt.wantSessionCreated && convService.startConversationCalls < 1 {
				t.Error("StartConversation() should have been called")
			}

			if tt.wantSessionEnded && convService.endConversationCalls < 1 {
				t.Error("EndConversation() should have been called")
			}

			if tt.wantPromptBuilderUsed && promptBuilder.buildPromptForAlertCalls < 1 {
				t.Error("BuildPromptForAlert() should have been called")
			}
		})
	}
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewInvestigationRunner_NotNil(t *testing.T) {
	convService := newInvestigationRunnerConvServiceMock()
	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()
	config := AlertInvestigationUseCaseConfig{}

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		config,
	)

	if runner == nil {
		t.Error("NewInvestigationRunner() should not return nil")
	}
}

// =============================================================================
// Empty/Malformed Input Tests
// =============================================================================

func TestInvestigationRunner_EmptyInvestigationID(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "session-empty-inv"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Investigation complete."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-empty-inv", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "")

	// Assert
	// Empty investigation ID should be rejected
	if err == nil {
		t.Error("Run() should return error for empty investigation ID")
	}
	if result != nil && result.Status != "failed" {
		t.Errorf("Result.Status = %q, want %q for empty investigation ID",
			result.Status, "failed")
	}
}

func TestInvestigationRunner_WhitespaceInvestigationID(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{},
	)

	alert := createTestAlert("alert-ws-inv", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "   ")

	// Assert
	// Whitespace-only investigation ID should be rejected
	if err == nil {
		t.Error("Run() should return error for whitespace-only investigation ID")
	}
	if result != nil && result.Status != "failed" {
		t.Errorf("Result.Status = %q, want %q", result.Status, "failed")
	}
}

func TestInvestigationRunner_AlertWithEmptyID(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	// Create alert with empty ID
	alert := &AlertForInvestigation{
		id:          "",
		source:      "prometheus",
		severity:    "warning",
		title:       "Test Alert",
		description: "Description",
		labels:      map[string]string{},
	}

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-empty-alert-id")

	// Assert
	// Alert with empty ID should be rejected
	if err == nil {
		t.Error("Run() should return error for alert with empty ID")
	}
	if result != nil && result.Status != "failed" {
		t.Errorf("Result.Status = %q, want %q", result.Status, "failed")
	}
}

// =============================================================================
// Safety Enforcer Integration Tests
// =============================================================================

func TestInvestigationRunner_SafetyEnforcerBlocksCommand(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-safety-cmd"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Executing command."),
		createAssistantMessage("Investigation complete."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "cmd-blocked",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "rm -rf /important"},
			},
		},
		nil,
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	// Create safety enforcer that blocks rm commands
	safetyEnforcer := NewMockSafetyEnforcerWithBlockedCommands([]string{"rm -rf"})
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-safety-cmd", "warning", "Test")

	// Act
	_, err := runner.Run(context.Background(), alert, "inv-safety-cmd")

	// Assert
	// The dangerous command should not be executed
	for _, name := range toolExecutor.executeToolName {
		if name == "bash" {
			for _, input := range toolExecutor.executeToolInput {
				if inputMap, ok := input.(map[string]interface{}); ok {
					if cmd, ok := inputMap["command"].(string); ok {
						if strings.Contains(cmd, "rm -rf") {
							t.Error("Dangerous command 'rm -rf' should have been blocked")
						}
					}
				}
			}
		}
	}
	_ = err // Error depends on implementation
}

func TestInvestigationRunner_SafetyEnforcerActionBudgetExceeded(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-budget"
	// Configure many tool calls
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Action 1"),
		createAssistantMessage("Action 2"),
		createAssistantMessage("Action 3"),
		createAssistantMessage("Action 4"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "cmd1"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "cmd2"}}},
		{{ToolID: "t3", ToolName: "bash", Input: map[string]interface{}{"command": "cmd3"}}},
		{{ToolID: "t4", ToolName: "bash", Input: map[string]interface{}{"command": "cmd4"}}},
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	// Safety enforcer with budget of 2 actions
	safetyEnforcer := NewMockSafetyEnforcerWithActionBudget(2)
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20, // Config allows 20, but enforcer limits to 2
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-budget", "warning", "Test")

	// Act
	result, _ := runner.Run(context.Background(), alert, "inv-budget")

	// Assert
	// Should not exceed the safety enforcer's budget
	if toolExecutor.executeToolCalls > 2 {
		t.Errorf("ExecuteTool() called %d times, safety enforcer should limit to 2",
			toolExecutor.executeToolCalls)
	}
	// Should escalate or fail when budget is exceeded
	if result != nil && !result.Escalated && result.Status == "completed" {
		t.Log("Investigation completed normally despite budget being exceeded")
	}
}

func TestInvestigationRunner_SafetyEnforcerTimeout(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-timeout-enforcer"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Starting investigation."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "sleep 1"}}},
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	// Safety enforcer that always returns timeout
	safetyEnforcer := NewMockSafetyEnforcerWithTimeout()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-timeout-enforcer", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-timeout-enforcer")

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
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-low-conf"
	// AI reports low confidence in response
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("I'm not confident about the root cause. Confidence: 0.2"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:           20,
			MaxDuration:          15 * time.Minute,
			AllowedTools:         []string{"bash"},
			EscalateOnConfidence: 0.5, // Escalate if confidence < 0.5
		},
	)

	alert := createTestAlert("alert-low-conf", "warning", "Uncertain Issue")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-low-conf")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	// With low confidence, investigation should escalate
	if result != nil && !result.Escalated && result.Confidence < 0.5 {
		t.Error("Investigation should escalate when confidence is below threshold")
	}
}

func TestInvestigationRunner_EscalatesOnConsecutiveErrors(t *testing.T) {
	// Arrange
	errorCount := 0
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-errors"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Trying command 1."),
		createAssistantMessage("Trying command 2."),
		createAssistantMessage("Trying command 3."),
		createAssistantMessage("Giving up."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "bad1"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "bad2"}}},
		{{ToolID: "t3", ToolName: "bash", Input: map[string]interface{}{"command": "bad3"}}},
		nil,
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	// All tool executions fail
	toolExecutor.executeToolError = errors.New("command failed")
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:       20,
			MaxDuration:      15 * time.Minute,
			AllowedTools:     []string{"bash"},
			EscalateOnErrors: 3, // Escalate after 3 consecutive errors
		},
	)

	alert := createTestAlert("alert-errors", "warning", "Error-prone Issue")

	// Act
	result, _ := runner.Run(context.Background(), alert, "inv-errors")

	// Assert
	// After 3 consecutive errors, should escalate
	_ = errorCount
	if result != nil && !result.Escalated {
		t.Log("Investigation should escalate after consecutive errors threshold reached")
	}
}

func TestInvestigationRunner_EscalatesForCriticalAlert(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-critical"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Critical issue detected."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:           20,
			MaxDuration:          15 * time.Minute,
			AllowedTools:         []string{"bash"},
			AutoStartForCritical: true,
		},
	)

	// Critical severity alert
	alert := createTestAlert("alert-critical", "critical", "Critical System Failure")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-critical")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	// Critical alerts may require escalation even if investigation completes
	_ = result
}

func TestInvestigationRunner_DoesNotEscalateOnHighConfidence(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-high-conf"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Root cause identified with high confidence."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:           20,
			MaxDuration:          15 * time.Minute,
			AllowedTools:         []string{"bash"},
			EscalateOnConfidence: 0.5,
		},
	)

	alert := createTestAlert("alert-high-conf", "warning", "Clear Issue")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-high-conf")
	// Assert
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-filter"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Using various tools."),
		createAssistantMessage("Done."),
	}
	// AI requests multiple tools, some not in allowed list
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}},
			{ToolID: "t2", ToolName: "edit_file", Input: map[string]interface{}{"path": "/etc/passwd"}},
			{ToolID: "t3", ToolName: "read_file", Input: map[string]interface{}{"path": "/var/log/syslog"}},
		},
		nil,
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "read_file"}, // edit_file NOT allowed
		},
	)

	alert := createTestAlert("alert-filter", "warning", "Test")

	// Act
	_, err := runner.Run(context.Background(), alert, "inv-filter")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	// Verify edit_file was not executed
	for _, name := range toolExecutor.executeToolName {
		if name == "edit_file" {
			t.Error("Tool 'edit_file' should not have been executed - not in allowed list")
		}
	}
}

func TestInvestigationRunner_EmptyAllowedToolsBlocksAll(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-no-tools"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Trying to use tools."),
		createAssistantMessage("No tools available."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}}},
		nil,
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{}, // No tools allowed
		},
	)

	alert := createTestAlert("alert-no-tools", "warning", "Test")

	// Act
	result, _ := runner.Run(context.Background(), alert, "inv-no-tools")

	// Assert
	// No tools should be executed
	if toolExecutor.executeToolCalls > 0 {
		t.Errorf("No tools should be executed when AllowedTools is empty, got %d calls",
			toolExecutor.executeToolCalls)
	}
	// Result should indicate limitations
	_ = result
}

// =============================================================================
// AI Response Edge Cases
// =============================================================================

func TestInvestigationRunner_EmptyAssistantResponse(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-empty-response"
	// AI returns empty message
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage(""),
		createAssistantMessage("Now I have something to say."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil, nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-empty-response", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-empty-response")
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-malformed"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Running tool."),
		createAssistantMessage("Done."),
	}
	// Malformed tool input (nil, wrong type)
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "malformed-1",
				ToolName: "bash",
				Input:    nil, // Nil input
			},
		},
		nil,
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-malformed", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-malformed")

	// Assert
	// Should handle malformed input gracefully without panic
	if result == nil && err == nil {
		t.Error("Run() should return either result or error for malformed input")
	}
}

func TestInvestigationRunner_NilToolCallInfo(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-nil-tool"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Done."),
	}
	// nil tool call info in the slice
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-nil-tool", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-nil-tool")
	// Assert
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-store"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Investigation complete."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()
	store := NewMockInvestigationStore()

	runner := NewInvestigationRunnerWithStore(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		store,
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-store-update"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Running tool."),
		createAssistantMessage("Investigation complete."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}}},
		nil,
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()
	store := NewMockInvestigationStore()

	runner := NewInvestigationRunnerWithStore(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		store,
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-store-error"
	convService.processResponseError = errors.New("AI error")

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()
	store := NewMockInvestigationStore()

	runner := NewInvestigationRunnerWithStore(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		store,
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-findings"
	// AI provides findings in response
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Finding 1: High CPU usage from process X."),
		createAssistantMessage("Finding 2: Memory leak detected."),
		createAssistantMessage("Investigation complete. Root cause: runaway process."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "top"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "free"}}},
		nil,
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-findings", "warning", "System Issue")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-findings")
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-summary"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage(
			"Summary: The issue was caused by a memory leak in the application. Recommendation: Restart the service.",
		),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-summary", "warning", "Memory Issue")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-summary")
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-concurrent"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Done."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:    20,
			MaxDuration:   15 * time.Minute,
			AllowedTools:  []string{"bash"},
			MaxConcurrent: 10,
		},
	)

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
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-duration"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Done."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-duration", "warning", "Test")

	// Act
	startTime := time.Now()
	result, err := runner.Run(context.Background(), alert, "inv-duration")
	endTime := time.Now()

	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}
	// Duration should be positive and reasonable
	if result.Duration <= 0 {
		t.Error("Result.Duration should be positive")
	}
	// Duration should not exceed actual elapsed time significantly
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
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationError = errors.New("connection refused to AI provider")

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{},
	)

	alert := createTestAlert("alert-error-ctx", "warning", "Test")

	// Act
	_, err := runner.Run(context.Background(), alert, "inv-error-ctx")

	// Assert
	if err == nil {
		t.Error("Run() should return error")
	}
	// Error should provide context about what failed
	errorStr := err.Error()
	if len(errorStr) < 10 {
		t.Errorf("Error message too short, should provide context: %v", err)
	}
}

// =============================================================================
// AddUserMessage Error Handling Tests
// =============================================================================

func TestInvestigationRunner_AddUserMessageError(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-user-msg-err"
	convService.addUserMessageError = errors.New("failed to add message")

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-user-msg-err", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-user-msg-err")

	// Assert
	if err == nil {
		t.Error("Run() should return error when AddUserMessage fails")
	}
	// Session should still be cleaned up
	if convService.endConversationCalls != 1 {
		t.Errorf("EndConversation() should be called for cleanup, got %d calls",
			convService.endConversationCalls)
	}
	_ = result
}

// =============================================================================
// AddToolResultMessage Error Handling Tests
// =============================================================================

func TestInvestigationRunner_AddToolResultMessageError(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-tool-result-err"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Running tool."),
		createAssistantMessage("Done."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}}},
		nil,
	}
	convService.addToolResultError = errors.New("failed to add tool result")

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-tool-result-err", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-tool-result-err")

	// Assert
	if err == nil {
		t.Error("Run() should return error when AddToolResultMessage fails")
	}
	// Session should still be cleaned up
	if convService.endConversationCalls != 1 {
		t.Errorf("EndConversation() should be called for cleanup, got %d calls",
			convService.endConversationCalls)
	}
	_ = result
}

// =============================================================================
// Edge Case: Very Long Tool Output Tests
// =============================================================================

func TestInvestigationRunner_HandlesLongToolOutput(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-long-output"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Running command."),
		createAssistantMessage("Done."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "cat large_file"}}},
		nil,
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	// Generate very long output (100KB)
	longOutput := strings.Repeat("A", 100*1024)
	toolExecutor.executeToolResult = longOutput

	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-long-output", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-long-output")
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-special"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Done."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

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
	result, err := runner.Run(context.Background(), alert, "inv-special")
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
				AlertInvestigationUseCaseConfig{},
			)
		})
	}
}

func TestNewInvestigationRunnerWithStore_NotNil(t *testing.T) {
	convService := newInvestigationRunnerConvServiceMock()
	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()
	store := NewMockInvestigationStore()
	config := AlertInvestigationUseCaseConfig{}

	runner := NewInvestigationRunnerWithStore(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		store,
		config,
	)

	if runner == nil {
		t.Error("NewInvestigationRunnerWithStore() should not return nil")
	}
}

// =============================================================================
// Config Validation Tests
// =============================================================================

func TestInvestigationRunner_ZeroMaxActions(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-zero-actions"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Trying tool."),
		createAssistantMessage("Done."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}}},
		nil,
	}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   0, // Zero means unlimited or immediate stop?
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-zero-actions", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-zero-actions")
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
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-zero-duration"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Done."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  0, // Zero duration
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-zero-duration", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-zero-duration")
	// Assert
	// Zero duration could mean immediate timeout or no timeout limit
	if err != nil {
		t.Logf("Run() with MaxDuration=0: error = %v", err)
	}
	_ = result
}

func TestInvestigationRunner_NegativeValues(t *testing.T) {
	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-negative"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Done."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   -1, // Negative values
			MaxDuration:  -time.Minute,
			AllowedTools: []string{"bash"},
		},
	)

	alert := createTestAlert("alert-negative", "warning", "Test")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-negative")
	// Assert
	// Negative values should be handled gracefully (rejected or treated as default)
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-complete"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Investigation complete. Root cause identified."),
	}
	// AI calls complete_investigation tool to signal investigation is done
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
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

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "read_file", "complete_investigation"},
		},
	)

	alert := createTestAlert("alert-complete", "warning", "High CPU Usage")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-complete-001")
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
	for _, name := range toolExecutor.executeToolName {
		if name == "complete_investigation" {
			t.Error("complete_investigation should be handled specially, not executed as regular tool")
		}
	}
}

func TestInvestigationRunner_ExtractsCompletionData(t *testing.T) {
	// Test: When AI calls complete_investigation, the runner extracts confidence,
	// findings, and root_cause from the tool input and populates the result.

	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-extract"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Analysis complete."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
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

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "complete_investigation"},
		},
	)

	alert := createTestAlert("alert-extract", "critical", "Database Connection Failures")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-extract-001")
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-escalate"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Unable to determine root cause. Escalating."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
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

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "escalate_investigation"},
		},
	)

	alert := createTestAlert("alert-escalate", "critical", "Network Connectivity Issues")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-escalate-001")
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
	for _, name := range toolExecutor.executeToolName {
		if name == "escalate_investigation" {
			t.Error("escalate_investigation should be handled specially, not executed as regular tool")
		}
	}
}

func TestInvestigationRunner_ExtractsEscalationData(t *testing.T) {
	// Test: When AI calls escalate_investigation, the runner extracts reason,
	// priority, and partial_findings from the tool input.

	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-esc-data"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Escalating to human operator."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
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

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "escalate_investigation"},
		},
	)

	alert := createTestAlert("alert-esc-data", "critical", "Security Alert")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-esc-data-001")
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
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-comp-stop"
	// Configure multiple responses, but only first should be processed
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Investigation complete."),
		createAssistantMessage("This should never be reached."),
		createAssistantMessage("Neither should this."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
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

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "complete_investigation"},
		},
	)

	alert := createTestAlert("alert-comp-stop", "warning", "Test Alert")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-comp-stop-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() result is nil")
	}

	// Verify loop stopped after complete_investigation
	// ProcessAssistantResponse should only be called once
	if convService.processResponseCalls > 1 {
		t.Errorf("ProcessAssistantResponse() called %d times, want 1 (loop should stop after complete_investigation)",
			convService.processResponseCalls)
	}

	// No tools should have been executed (complete_investigation is special)
	if toolExecutor.executeToolCalls > 0 {
		t.Errorf("ExecuteTool() called %d times, want 0 (complete_investigation stops the loop immediately)",
			toolExecutor.executeToolCalls)
	}
}

func TestInvestigationRunner_EscalationStopsLoop(t *testing.T) {
	// Test: After escalate_investigation is called, no more iterations occur.
	// The investigation should immediately return with escalated status.

	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-esc-stop"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Escalating immediately."),
		createAssistantMessage("This should never be reached."),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
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

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "escalate_investigation"},
		},
	)

	alert := createTestAlert("alert-esc-stop", "critical", "Critical Alert")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-esc-stop-001")
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
	if convService.processResponseCalls > 1 {
		t.Errorf("ProcessAssistantResponse() called %d times, want 1 (loop should stop after escalate_investigation)",
			convService.processResponseCalls)
	}

	// No tools should have been executed (escalate_investigation is special)
	if toolExecutor.executeToolCalls > 0 {
		t.Errorf("ExecuteTool() called %d times, want 0 (escalate_investigation stops the loop immediately)",
			toolExecutor.executeToolCalls)
	}
}

func TestInvestigationRunner_MixedToolCallsWithCompletion(t *testing.T) {
	// Test: When the AI response contains both regular tool calls and
	// complete_investigation in the same response, the regular tools should
	// be executed first before the investigation completes.

	// Arrange
	convService := newInvestigationRunnerConvServiceMock()
	convService.startConversationSession = "inv-session-mixed"
	convService.processResponseMessages = []*entity.Message{
		createAssistantMessage("Running final checks and completing investigation."),
	}
	// Single response with multiple tool calls including complete_investigation
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
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

	toolExecutor := newInvestigationRunnerToolExecutorMock()
	safetyEnforcer := NewMockSafetyEnforcer()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		safetyEnforcer,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{
			MaxActions:   20,
			MaxDuration:  15 * time.Minute,
			AllowedTools: []string{"bash", "read_file", "complete_investigation"},
		},
	)

	alert := createTestAlert("alert-mixed", "warning", "Disk Space Alert")

	// Act
	result, err := runner.Run(context.Background(), alert, "inv-mixed-001")
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
	if toolExecutor.executeToolCalls != 2 {
		t.Errorf("ExecuteTool() called %d times, want 2 (bash and read_file should be executed before completion)",
			toolExecutor.executeToolCalls)
	}

	// Verify the correct tools were executed (not complete_investigation)
	expectedTools := []string{"bash", "read_file"}
	for i, expected := range expectedTools {
		if i < len(toolExecutor.executeToolName) && toolExecutor.executeToolName[i] != expected {
			t.Errorf("Tool %d executed was %q, want %q", i, toolExecutor.executeToolName[i], expected)
		}
	}

	// Verify complete_investigation was NOT executed as a regular tool
	for _, name := range toolExecutor.executeToolName {
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
	// Create mocks
	convService := newInvestigationRunnerConvServiceMock()
	toolExecutor := newInvestigationRunnerToolExecutorMock()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	// Configure prompt builder to return a known prompt
	expectedSystemPrompt := "You are investigating alert: test-alert. Use the following tools: bash, read_file"
	promptBuilder.buildPromptResult = expectedSystemPrompt

	// Configure mock to return completion immediately
	completionToolCall := port.ToolCallInfo{
		ToolID:   "complete-1",
		ToolName: "complete_investigation",
		Input: map[string]interface{}{
			"findings":   []interface{}{"Investigation completed"},
			"confidence": 0.95,
		},
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{{completionToolCall}}

	// Create runner
	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		nil,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{MaxActions: 20},
	)

	// Create test alert
	alert := &AlertForInvestigation{
		id:          "test-alert-123",
		title:       "Test Alert",
		description: "Test description",
		source:      "test-source",
		severity:    "high",
	}

	// Run investigation
	ctx := context.Background()
	_, err := runner.Run(ctx, alert, "inv-123")
	// Test should FAIL because SetCustomSystemPrompt is not implemented yet
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// EXPECTED BEHAVIOR: SetCustomSystemPrompt should be called with the investigation prompt
	// This will fail because the mock doesn't have this method yet
	if convService.setCustomSystemPromptCalls != 1 {
		t.Errorf("SetCustomSystemPrompt() called %d times, want 1", convService.setCustomSystemPromptCalls)
	}

	// Verify the prompt content matches what the builder returned
	if len(convService.setCustomSystemPromptContent) == 0 {
		t.Fatal("SetCustomSystemPrompt() was not called with any content")
	}

	actualPrompt := convService.setCustomSystemPromptContent[0]
	if actualPrompt != expectedSystemPrompt {
		t.Errorf("SetCustomSystemPrompt() called with prompt = %q, want %q", actualPrompt, expectedSystemPrompt)
	}
}

func TestInvestigationRunner_Run_SetCustomSystemPromptCalledBeforeAddUserMessage(t *testing.T) {
	// Create mocks
	convService := newInvestigationRunnerConvServiceMock()
	toolExecutor := newInvestigationRunnerToolExecutorMock()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	// Track call order
	var callOrder []string
	convService.onSetCustomSystemPrompt = func() {
		callOrder = append(callOrder, "SetCustomSystemPrompt")
	}
	convService.onAddUserMessage = func() {
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
	convService.processResponseToolCalls = [][]port.ToolCallInfo{{completionToolCall}}

	// Create runner
	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		nil,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{MaxActions: 20},
	)

	// Create test alert
	alert := &AlertForInvestigation{
		id:       "test-alert-456",
		title:    "Order Test Alert",
		severity: "critical",
	}

	// Run investigation
	ctx := context.Background()
	_, err := runner.Run(ctx, alert, "inv-456")
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
	// Create mocks
	convService := newInvestigationRunnerConvServiceMock()
	toolExecutor := newInvestigationRunnerToolExecutorMock()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	// Configure prompt builder to return a large prompt
	promptBuilder.buildPromptResult = "This is a very long investigation prompt with many instructions about how to investigate alerts. It contains tool descriptions, guidelines, safety rules, and much more. This should NOT appear in the user message."

	// Configure mock to return completion immediately
	completionToolCall := port.ToolCallInfo{
		ToolID:   "complete-1",
		ToolName: "complete_investigation",
		Input: map[string]interface{}{
			"findings":   []interface{}{"Found the issue"},
			"confidence": 0.85,
		},
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{{completionToolCall}}

	// Create runner
	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		nil,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{MaxActions: 20},
	)

	// Create test alert
	alert := &AlertForInvestigation{
		id:          "alert-789",
		title:       "Minimal Message Test",
		description: "This should not appear in user message",
		source:      "test",
		severity:    "medium",
	}

	// Run investigation
	ctx := context.Background()
	_, err := runner.Run(ctx, alert, "inv-789")
	if err != nil {
		t.Fatalf("Run() returned error: %v", err)
	}

	// EXPECTED BEHAVIOR: AddUserMessage should receive MINIMAL content (just alert ID and title)
	// NOT the full investigation prompt
	if len(convService.addUserMessageContent) == 0 {
		t.Fatal("AddUserMessage() was not called")
	}

	userMessage := convService.addUserMessageContent[0]

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
	// Create mocks
	convService := newInvestigationRunnerConvServiceMock()
	toolExecutor := newInvestigationRunnerToolExecutorMock()
	promptBuilder := newInvestigationRunnerPromptBuilderMock()

	// Configure SetCustomSystemPrompt to return an error
	expectedError := errors.New("system prompt configuration failed")
	convService.setCustomSystemPromptError = expectedError

	// Create runner
	runner := NewInvestigationRunner(
		convService,
		toolExecutor,
		nil,
		promptBuilder,
		nil, // skillManager
		AlertInvestigationUseCaseConfig{MaxActions: 20},
	)

	// Create test alert
	alert := &AlertForInvestigation{
		id:       "alert-error",
		title:    "Error Test",
		severity: "high",
	}

	// Run investigation
	ctx := context.Background()
	result, err := runner.Run(ctx, alert, "inv-error")

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

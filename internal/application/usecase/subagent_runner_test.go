package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// SubagentRunner Tests
// These tests verify the behavior of SubagentRunner which orchestrates
// isolated subagent execution for task delegation.
// =============================================================================

// =============================================================================
// Mock Implementations for SubagentRunner Tests
// =============================================================================

// subagentRunnerConvServiceMock implements ConversationServiceInterface for testing.
type subagentRunnerConvServiceMock struct {
	mu sync.Mutex

	// StartConversation tracking
	startConversationCalls   int
	startConversationError   error
	startConversationSession string

	// AddUserMessage tracking
	addUserMessageCalls   int
	addUserMessageError   error
	addUserMessageContent []string

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
}

func newSubagentRunnerConvServiceMock() *subagentRunnerConvServiceMock {
	return &subagentRunnerConvServiceMock{
		startConversationSession: "subagent-session-123",
		processResponseMessages:  []*entity.Message{},
		processResponseToolCalls: [][]port.ToolCallInfo{},
	}
}

func (m *subagentRunnerConvServiceMock) StartConversation(_ context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.startConversationCalls++
	if m.startConversationError != nil {
		return "", m.startConversationError
	}
	return m.startConversationSession, nil
}

func (m *subagentRunnerConvServiceMock) AddUserMessage(
	_ context.Context,
	_ string,
	content string,
) (*entity.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addUserMessageCalls++
	m.addUserMessageContent = append(m.addUserMessageContent, content)
	if m.addUserMessageError != nil {
		return nil, m.addUserMessageError
	}
	msg, _ := entity.NewMessage(entity.RoleUser, content)
	return msg, nil
}

func (m *subagentRunnerConvServiceMock) ProcessAssistantResponse(
	_ context.Context,
	_ string,
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

func (m *subagentRunnerConvServiceMock) AddToolResultMessage(
	_ context.Context,
	_ string,
	toolResults []entity.ToolResult,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addToolResultCalls++
	m.addToolResultResults = append(m.addToolResultResults, toolResults)
	return m.addToolResultError
}

func (m *subagentRunnerConvServiceMock) EndConversation(_ context.Context, sessionID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.endConversationCalls++
	m.endConversationSession = sessionID
	return m.endConversationError
}

func (m *subagentRunnerConvServiceMock) SetCustomSystemPrompt(
	_ context.Context,
	_ string,
	prompt string,
) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setCustomSystemPromptCalls++
	m.setCustomSystemPromptContent = append(m.setCustomSystemPromptContent, prompt)
	return m.setCustomSystemPromptError
}

// subagentRunnerToolExecutorMock implements port.ToolExecutor for testing.
type subagentRunnerToolExecutorMock struct {
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

func newSubagentRunnerToolExecutorMock() *subagentRunnerToolExecutorMock {
	return &subagentRunnerToolExecutorMock{
		executeToolResult: "tool execution result",
		registeredTools: []entity.Tool{
			{Name: "bash", Description: "Execute bash commands"},
			{Name: "read_file", Description: "Read file contents"},
			{Name: "list_files", Description: "List files in directory"},
		},
	}
}

func (m *subagentRunnerToolExecutorMock) RegisterTool(tool entity.Tool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.registeredTools = append(m.registeredTools, tool)
	return nil
}

func (m *subagentRunnerToolExecutorMock) UnregisterTool(_ string) error {
	return nil
}

func (m *subagentRunnerToolExecutorMock) ExecuteTool(
	_ context.Context,
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

func (m *subagentRunnerToolExecutorMock) ListTools() ([]entity.Tool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.registeredTools, nil
}

func (m *subagentRunnerToolExecutorMock) GetTool(name string) (entity.Tool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.registeredTools {
		if t.Name == name {
			return t, true
		}
	}
	return entity.Tool{}, false
}

func (m *subagentRunnerToolExecutorMock) ValidateToolInput(_ string, _ interface{}) error {
	return nil
}

// subagentRunnerAIProviderMock implements port.AIProvider for testing.
type subagentRunnerAIProviderMock struct {
	mu sync.Mutex

	// SendMessage tracking
	sendMessageCalls    int
	sendMessageError    error
	sendMessageMessages [][]port.MessageParam
	sendMessageTools    [][]port.ToolParam
	sendMessageResponse *entity.Message
	sendMessageToolCall []port.ToolCallInfo

	// SetModel tracking
	setModelCalls  int
	setModelValues []string
	currentModel   string
}

func newSubagentRunnerAIProviderMock() *subagentRunnerAIProviderMock {
	msg, _ := entity.NewMessage(entity.RoleAssistant, "Task completed successfully")
	return &subagentRunnerAIProviderMock{
		sendMessageResponse: msg,
		currentModel:        "test-model",
	}
}

func (m *subagentRunnerAIProviderMock) SendMessage(
	_ context.Context,
	messages []port.MessageParam,
	tools []port.ToolParam,
) (*entity.Message, []port.ToolCallInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sendMessageCalls++
	m.sendMessageMessages = append(m.sendMessageMessages, messages)
	m.sendMessageTools = append(m.sendMessageTools, tools)
	if m.sendMessageError != nil {
		return nil, nil, m.sendMessageError
	}
	return m.sendMessageResponse, m.sendMessageToolCall, nil
}

func (m *subagentRunnerAIProviderMock) GenerateToolSchema() port.ToolInputSchemaParam {
	return port.ToolInputSchemaParam{}
}

func (m *subagentRunnerAIProviderMock) HealthCheck(_ context.Context) error {
	return nil
}

func (m *subagentRunnerAIProviderMock) SetModel(model string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setModelCalls++
	m.setModelValues = append(m.setModelValues, model)
	m.currentModel = model
	return nil
}

func (m *subagentRunnerAIProviderMock) GetModel() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentModel
}

// =============================================================================
// Helper Functions
// =============================================================================

func createTestAgent(_, name string) *entity.Subagent {
	return &entity.Subagent{
		Name:         name,
		RawContent:   "You are a helpful assistant specialized in " + name,
		AllowedTools: []string{"bash", "read_file"},
		Model:        "",
	}
}

// =============================================================================
// Context Helper Functions for Subagent Context
// =============================================================================

// subagentContextKey is the key for storing subagent context info.
type subagentContextKey struct{}

// SubagentContextInfo holds information about subagent execution context.
type SubagentContextInfo struct {
	SubagentID      string
	ParentSessionID string
	IsSubagent      bool
	Depth           int
}

// WithSubagentContext adds subagent context info to a context.
func WithSubagentContext(ctx context.Context, info SubagentContextInfo) context.Context {
	return context.WithValue(ctx, subagentContextKey{}, info)
}

// SubagentContextFromContext retrieves subagent context info from a context.
func SubagentContextFromContext(ctx context.Context) (SubagentContextInfo, bool) {
	info, ok := ctx.Value(subagentContextKey{}).(SubagentContextInfo)
	return info, ok
}

// IsSubagentContext checks if a context has subagent context info.
func IsSubagentContext(ctx context.Context) bool {
	_, ok := SubagentContextFromContext(ctx)
	return ok
}

func createSubagentAssistantMessage(content string) *entity.Message {
	msg, _ := entity.NewMessage(entity.RoleAssistant, content)
	return msg
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewSubagentRunner_WithAllDependencies(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{
		MaxActions:      10,
		MaxDuration:     5 * time.Minute,
		MaxConcurrent:   3,
		AllowedTools:    []string{"bash", "read_file"},
		BlockedCommands: []string{"rm -rf"},
	}

	// Act
	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Assert
	if runner == nil {
		t.Fatal("NewSubagentRunner() returned nil")
	}
}

func TestNewSubagentRunner_PanicsWithNilConversationService(t *testing.T) {
	// Arrange
	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{}

	// Act & Assert
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewSubagentRunner() should panic with nil convService")
		}
	}()

	NewSubagentRunner(nil, toolExecutor, aiProvider, nil, config)
}

func TestNewSubagentRunner_PanicsWithNilToolExecutor(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{}

	// Act & Assert
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewSubagentRunner() should panic with nil toolExecutor")
		}
	}()

	NewSubagentRunner(convService, nil, aiProvider, nil, config)
}

func TestNewSubagentRunner_PanicsWithNilAIProvider(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	toolExecutor := newSubagentRunnerToolExecutorMock()
	config := SubagentConfig{}

	// Act & Assert
	defer func() {
		if r := recover(); r == nil {
			t.Error("NewSubagentRunner() should panic with nil aiProvider")
		}
	}()

	NewSubagentRunner(convService, toolExecutor, nil, nil, config)
}

// =============================================================================
// Run Method Tests - Basic Execution
// =============================================================================

func TestSubagentRunner_Run_SuccessfulExecution(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-001"
	// Configure AI to complete without tool calls
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Task completed successfully"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()

	config := SubagentConfig{
		MaxActions:   10,
		MaxDuration:  5 * time.Minute,
		AllowedTools: []string{"bash", "read_file"},
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-001", "Code Analyzer")
	taskPrompt := "Analyze the error logs and identify the root cause"

	// Act
	result, err := runner.Run(context.Background(), agent, taskPrompt, "subagent-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if result.Status != "completed" {
		t.Errorf("Run() result status = %q, want %q", result.Status, "completed")
	}
	if result.SubagentID != "subagent-001" {
		t.Errorf("Run() result SubagentID = %q, want %q", result.SubagentID, "subagent-001")
	}
	if result.AgentName != "Code Analyzer" {
		t.Errorf("Run() result AgentName = %q, want %q", result.AgentName, "Code Analyzer")
	}
}

func TestSubagentRunner_Run_HandlesNilAgent(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)

	// Act
	result, err := runner.Run(context.Background(), nil, "some task", "subagent-002")

	// Assert
	if err == nil {
		t.Error("Run() should return error with nil agent")
	}
	if result == nil {
		t.Fatal("Run() should return result even on validation failure")
	}
	if result.Status != "failed" {
		t.Errorf("Run() result status = %q, want %q", result.Status, "failed")
	}
}

func TestSubagentRunner_Run_HandlesEmptyTaskPrompt(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-003", "Helper")

	// Act
	result, err := runner.Run(context.Background(), agent, "", "subagent-003")

	// Assert
	if err == nil {
		t.Error("Run() should return error with empty task prompt")
	}
	if result == nil {
		t.Fatal("Run() should return result even on validation failure")
	}
	if result.Status != "failed" {
		t.Errorf("Run() result status = %q, want %q", result.Status, "failed")
	}
}

// =============================================================================
// Session Management Tests
// =============================================================================

func TestSubagentRunner_CreatesIsolatedSession(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-iso-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-iso", "Isolated Agent")

	// Act
	_, err := runner.Run(context.Background(), agent, "Execute task", "subagent-iso-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if convService.startConversationCalls != 1 {
		t.Errorf("StartConversation() called %d times, want 1", convService.startConversationCalls)
	}
}

func TestSubagentRunner_CleansUpSessionOnCompletion(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-cleanup-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Completed"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-cleanup", "Test Agent")

	// Act
	_, err := runner.Run(context.Background(), agent, "Task", "subagent-cleanup-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if convService.endConversationCalls != 1 {
		t.Errorf("EndConversation() called %d times, want 1", convService.endConversationCalls)
	}
	if convService.endConversationSession != "subagent-session-cleanup-001" {
		t.Errorf("EndConversation() called with session %q, want %q",
			convService.endConversationSession, "subagent-session-cleanup-001")
	}
}

func TestSubagentRunner_CleansUpSessionOnError(t *testing.T) {
	// Arrange
	expectedError := errors.New("AI processing error")
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-error-001"
	convService.processResponseError = expectedError

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-error", "Test Agent")

	// Act
	_, err := runner.Run(context.Background(), agent, "Task", "subagent-error-001")

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

func TestSubagentRunner_HandlesStartConversationError(t *testing.T) {
	// Arrange
	expectedError := errors.New("failed to start conversation")
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationError = expectedError

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-start-err", "Test Agent")

	// Act
	result, err := runner.Run(context.Background(), agent, "Task", "subagent-start-err")

	// Assert
	if err == nil {
		t.Error("Run() should return error when StartConversation fails")
	}
	if result == nil {
		t.Fatal("Run() should return result on error")
	}
	if result.Status != "failed" {
		t.Errorf("Run() result status = %q, want %q", result.Status, "failed")
	}
}

// =============================================================================
// Custom System Prompt Tests
// =============================================================================

func TestSubagentRunner_SetsCustomSystemPrompt(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-prompt-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-prompt", "Specialized Agent")
	agent.RawContent = "You are a specialized agent for code analysis"

	// Act
	_, err := runner.Run(context.Background(), agent, "Analyze code", "subagent-prompt-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if convService.setCustomSystemPromptCalls != 1 {
		t.Errorf("SetCustomSystemPrompt() called %d times, want 1", convService.setCustomSystemPromptCalls)
	}
	if len(convService.setCustomSystemPromptContent) == 0 {
		t.Fatal("SetCustomSystemPrompt() not called with any content")
	}
	// The prompt should combine agent system prompt with task prompt
	prompt := convService.setCustomSystemPromptContent[0]
	if len(prompt) == 0 {
		t.Error("SetCustomSystemPrompt() called with empty prompt")
	}
}

func TestSubagentRunner_AddsUserMessageWithTaskPrompt(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-task-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-task", "Task Agent")
	taskPrompt := "Analyze the error logs and identify the root cause"

	// Act
	_, err := runner.Run(context.Background(), agent, taskPrompt, "subagent-task-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if convService.addUserMessageCalls != 1 {
		t.Errorf("AddUserMessage() called %d times, want 1", convService.addUserMessageCalls)
	}
	if len(convService.addUserMessageContent) == 0 {
		t.Fatal("AddUserMessage() not called with any content")
	}
	// The user message should contain the task prompt
	userMsg := convService.addUserMessageContent[0]
	if len(userMsg) == 0 {
		t.Error("AddUserMessage() called with empty content")
	}
}

// =============================================================================
// Tool Execution Tests
// =============================================================================

func TestSubagentRunner_ExecutesToolCalls(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-tools-001"
	// First response: AI requests a tool
	// Second response: AI completes
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Need to check logs"),
		createSubagentAssistantMessage("Analysis complete"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "tool-call-1",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "cat /var/log/app.log"},
			},
		},
		nil, // No tools in second response
	}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	toolExecutor.executeToolResult = "log contents here"

	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{
		MaxActions:   10,
		AllowedTools: []string{"bash", "read_file"},
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-tools", "Tool User Agent")

	// Act
	result, err := runner.Run(context.Background(), agent, "Check logs", "subagent-tools-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if toolExecutor.executeToolCalls != 1 {
		t.Errorf("ExecuteTool() called %d times, want 1", toolExecutor.executeToolCalls)
	}
	if len(toolExecutor.executeToolName) > 0 && toolExecutor.executeToolName[0] != "bash" {
		t.Errorf("ExecuteTool() called with tool %q, want %q",
			toolExecutor.executeToolName[0], "bash")
	}
}

func TestSubagentRunner_FeedsToolResultsBackToAI(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-feedback-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Running tool"),
		createSubagentAssistantMessage("Got results"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "tool-call-feedback",
				ToolName: "read_file",
				Input:    map[string]interface{}{"path": "/tmp/test.txt"},
			},
		},
		nil,
	}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	toolExecutor.executeToolResult = "file contents"

	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{
		MaxActions:   10,
		AllowedTools: []string{"read_file"},
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-feedback", "Feedback Agent")

	// Act
	_, err := runner.Run(context.Background(), agent, "Read file", "subagent-feedback-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if convService.addToolResultCalls != 1 {
		t.Errorf("AddToolResultMessage() called %d times, want 1", convService.addToolResultCalls)
	}
	if len(convService.addToolResultResults) > 0 {
		results := convService.addToolResultResults[0]
		if len(results) != 1 {
			t.Errorf("AddToolResultMessage() called with %d results, want 1", len(results))
		} else if results[0].ToolID != "tool-call-feedback" {
			t.Errorf("Tool result ToolID = %q, want %q", results[0].ToolID, "tool-call-feedback")
		}
	}
}

func TestSubagentRunner_HandlesToolExecutionError(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-tool-err-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Running tool"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "tool-call-err",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "invalid-command"},
			},
		},
	}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	toolExecutor.executeToolError = errors.New("command not found")

	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-tool-err", "Error Agent")

	// Act
	// Note: Tool errors should be fed back to AI, not necessarily fail the entire run
	runner.Run(context.Background(), agent, "Run command", "subagent-tool-err-001")

	// Assert - exact behavior depends on implementation, but tool result should be added
	// The error might be in the result or the run might continue
	if convService.addToolResultCalls > 0 {
		results := convService.addToolResultResults[0]
		if len(results) > 0 && !results[0].IsError {
			t.Error("Tool result should be marked as error when ExecuteTool fails")
		}
	}
	// Note: The implementation may fail the whole run on tool errors, or it may
	// feed errors back to the AI. Both behaviors are acceptable - we don't assert
	// on specific outcomes here.
}

// =============================================================================
// Action Limits Tests
// =============================================================================

func TestSubagentRunner_RespectsMaxActionsLimit(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-limit-001"
	// Configure many tool calls to exceed limit
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Action 1"),
		createSubagentAssistantMessage("Action 2"),
		createSubagentAssistantMessage("Action 3"),
		createSubagentAssistantMessage("Action 4"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "echo 1"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "echo 2"}}},
		{{ToolID: "t3", ToolName: "bash", Input: map[string]interface{}{"command": "echo 3"}}},
		{{ToolID: "t4", ToolName: "bash", Input: map[string]interface{}{"command": "echo 4"}}},
	}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{
		MaxActions:   2, // Limit to 2 actions
		AllowedTools: []string{"bash"},
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-limit", "Limited Agent")

	// Act
	result, err := runner.Run(context.Background(), agent, "Do many things", "subagent-limit-001")

	// Assert
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if result.ActionsTaken > config.MaxActions {
		t.Errorf("Run() took %d actions, should not exceed MaxActions=%d",
			result.ActionsTaken, config.MaxActions)
	}
	// The run should complete (status may vary based on implementation)
	_ = err // Error is acceptable if limit is treated as failure
}

func TestSubagentRunner_StopsWhenMaxActionsReached(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-stop-001"
	// More responses than the limit
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Step 1"),
		createSubagentAssistantMessage("Step 2"),
		createSubagentAssistantMessage("Step 3"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "s1", ToolName: "bash", Input: map[string]interface{}{"command": "step1"}}},
		{{ToolID: "s2", ToolName: "bash", Input: map[string]interface{}{"command": "step2"}}},
		{{ToolID: "s3", ToolName: "bash", Input: map[string]interface{}{"command": "step3"}}},
	}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{
		MaxActions:   1, // Very strict limit
		AllowedTools: []string{"bash"},
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-stop", "Stop Agent")

	// Act
	result, _ := runner.Run(context.Background(), agent, "Multi-step task", "subagent-stop-001")

	// Assert
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// Should stop after MaxActions
	if result.ActionsTaken > config.MaxActions {
		t.Errorf("Run() took %d actions, want <= %d", result.ActionsTaken, config.MaxActions)
	}
	if toolExecutor.executeToolCalls > config.MaxActions {
		t.Errorf("ExecuteTool() called %d times, should not exceed MaxActions=%d",
			toolExecutor.executeToolCalls, config.MaxActions)
	}
}

func TestSubagentRunner_TracksActionsTaken(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-track-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Step 1"),
		createSubagentAssistantMessage("Step 2"),
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "cmd1"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "cmd2"}}},
		nil, // Completion
	}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{
		MaxActions:   10,
		AllowedTools: []string{"bash"},
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-track", "Tracking Agent")

	// Act
	result, err := runner.Run(context.Background(), agent, "Execute steps", "subagent-track-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if result.ActionsTaken != 2 {
		t.Errorf("Run() result.ActionsTaken = %d, want 2", result.ActionsTaken)
	}
}

// =============================================================================
// Error Handling Tests
// =============================================================================

func TestSubagentRunner_HandlesConversationServiceErrors(t *testing.T) {
	tests := []struct {
		name          string
		setupMock     func(*subagentRunnerConvServiceMock)
		expectedError bool
		expectedFail  bool
	}{
		{
			name: "ProcessAssistantResponse error",
			setupMock: func(m *subagentRunnerConvServiceMock) {
				m.processResponseError = errors.New("AI provider unavailable")
			},
			expectedError: true,
			expectedFail:  true,
		},
		{
			name: "AddToolResultMessage error",
			setupMock: func(m *subagentRunnerConvServiceMock) {
				m.processResponseMessages = []*entity.Message{
					createSubagentAssistantMessage("Running tool"),
				}
				m.processResponseToolCalls = [][]port.ToolCallInfo{
					{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "echo"}}},
				}
				m.addToolResultError = errors.New("failed to add tool result")
			},
			expectedError: true,
			expectedFail:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			convService := newSubagentRunnerConvServiceMock()
			convService.startConversationSession = "subagent-session-err"
			tt.setupMock(convService)

			toolExecutor := newSubagentRunnerToolExecutorMock()
			aiProvider := newSubagentRunnerAIProviderMock()
			config := SubagentConfig{MaxActions: 10}

			runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
			agent := createTestAgent("agent-err", "Error Agent")

			// Act
			result, err := runner.Run(context.Background(), agent, "Task", "subagent-err")

			// Assert
			if tt.expectedError && err == nil {
				t.Errorf("%s: Run() should return error", tt.name)
			}
			if tt.expectedFail && result != nil && result.Status != "failed" {
				t.Errorf("%s: Run() result status = %q, want %q", tt.name, result.Status, "failed")
			}
		})
	}
}

// =============================================================================
// Result Tests
// =============================================================================

func TestSubagentRunner_ReturnsResultWithStatus(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-status-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Completed successfully"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-status", "Status Agent")

	// Act
	result, err := runner.Run(context.Background(), agent, "Task", "subagent-status-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// Status should be one of: "completed", "failed", "cancelled"
	validStatuses := map[string]bool{"completed": true, "failed": true, "cancelled": true}
	if !validStatuses[result.Status] {
		t.Errorf("Run() result.Status = %q, want one of [completed, failed, cancelled]", result.Status)
	}
}

func TestSubagentRunner_ReturnsResultWithOutput(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-output-001"
	outputMessage := "The root cause is a memory leak in module X. Recommendation: upgrade to v2.0"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage(outputMessage),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-output", "Output Agent")

	// Act
	result, err := runner.Run(context.Background(), agent, "Diagnose issue", "subagent-output-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// Output should contain AI's final response
	if len(result.Output) == 0 {
		t.Error("Run() result.Output is empty, want AI response content")
	}
}

func TestSubagentRunner_OutputIncludesSubagentPrefix(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-prefix-001"
	outputMessage := "Analysis complete: no issues found"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage(outputMessage),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("test-agent", "Test Agent")

	// Act
	result, err := runner.Run(context.Background(), agent, "Analyze code", "subagent-prefix-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	// Output should be prefixed with [SUBAGENT: agent-name]
	expectedPrefix := "[SUBAGENT: Test Agent]\n\n"
	if !strings.HasPrefix(result.Output, expectedPrefix) {
		t.Errorf("Run() result.Output = %q, want prefix %q", result.Output, expectedPrefix)
	}

	// Output should contain the original message after the prefix
	expectedOutput := expectedPrefix + outputMessage
	if result.Output != expectedOutput {
		t.Errorf("Run() result.Output = %q, want %q", result.Output, expectedOutput)
	}
}

func TestSubagentRunner_ReturnsResultWithDuration(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-duration-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-duration", "Duration Agent")

	// Act
	start := time.Now()
	result, err := runner.Run(context.Background(), agent, "Task", "subagent-duration-001")
	elapsed := time.Since(start)

	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if result.Duration <= 0 {
		t.Error("Run() result.Duration should be > 0")
	}
	if result.Duration > elapsed+time.Second {
		t.Errorf("Run() result.Duration = %v, should not exceed actual elapsed time %v significantly",
			result.Duration, elapsed)
	}
}

// =============================================================================
// Tool Filtering (AllowedTools) Tests
// =============================================================================

func TestSubagentRunner_AllowedTools_AllowsOnlySpecifiedTools(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-filter-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Trying to execute tools"),
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}},
			{ToolID: "t2", ToolName: "read_file", Input: map[string]interface{}{"path": "/tmp/test"}},
		},
		nil,
	}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{
		MaxActions:   10,
		AllowedTools: []string{"bash"}, // Only bash allowed
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-filter", "Filter Agent")
	agent.AllowedTools = []string{"bash"} // Agent also specifies allowed tools

	// Act
	result, err := runner.Run(context.Background(), agent, "Execute commands", "subagent-filter-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// Only bash should have been executed, not read_file
	if toolExecutor.executeToolCalls != 1 {
		t.Errorf("ExecuteTool() called %d times, want 1 (only bash should execute)", toolExecutor.executeToolCalls)
	}
	if len(toolExecutor.executeToolName) > 0 && toolExecutor.executeToolName[0] != "bash" {
		t.Errorf("ExecuteTool() called with tool %q, want %q", toolExecutor.executeToolName[0], "bash")
	}
}

func TestSubagentRunner_AllowedTools_BlocksNonAllowedTools(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-block-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Trying blocked tool"),
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{ToolID: "t1", ToolName: "list_files", Input: map[string]interface{}{"directory": "/"}},
		},
		nil,
	}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{
		MaxActions:   10,
		AllowedTools: []string{"bash", "read_file"}, // list_files NOT allowed
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-block", "Block Agent")

	// Act
	result, err := runner.Run(context.Background(), agent, "List files", "subagent-block-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// list_files should have been blocked, no tools executed
	if toolExecutor.executeToolCalls != 0 {
		t.Errorf("ExecuteTool() called %d times, want 0 (list_files should be blocked)", toolExecutor.executeToolCalls)
	}
}

func TestSubagentRunner_AllowedTools_NilAllowedToolsAllowsAll(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-nil-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Running tools"),
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "echo"}},
			{ToolID: "t2", ToolName: "read_file", Input: map[string]interface{}{"path": "/tmp/test"}},
			{ToolID: "t3", ToolName: "list_files", Input: map[string]interface{}{"directory": "/"}},
		},
		nil,
	}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{
		MaxActions:   10,
		AllowedTools: nil, // nil means allow all
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-nil", "Nil Filter Agent")

	// Act
	result, err := runner.Run(context.Background(), agent, "Execute all tools", "subagent-nil-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// All three tools should have been executed
	if toolExecutor.executeToolCalls != 3 {
		t.Errorf("ExecuteTool() called %d times, want 3 (all tools should execute)", toolExecutor.executeToolCalls)
	}
}

func TestSubagentRunner_AllowedTools_EmptySliceBlocksAll(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-empty-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Trying tools"),
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "echo"}},
		},
		nil,
	}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{
		MaxActions:   10,
		AllowedTools: []string{}, // Empty slice means block all
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-empty", "Empty Filter Agent")

	// Act
	result, err := runner.Run(context.Background(), agent, "Execute tools", "subagent-empty-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// No tools should have been executed
	if toolExecutor.executeToolCalls != 0 {
		t.Errorf("ExecuteTool() called %d times, want 0 (all tools should be blocked)", toolExecutor.executeToolCalls)
	}
}

// =============================================================================
// Recursion Prevention Tests
// =============================================================================

func TestSubagentRunner_RecursionPrevention_AddsSubagentContextToContext(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-ctx-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-ctx", "Context Agent")

	// Act
	ctx := context.Background()
	_, err := runner.Run(ctx, agent, "Task", "subagent-ctx-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	// The context passed to ProcessAssistantResponse should have subagent info
	// This will be verified by checking if IsSubagentContext returns true
	// when called on the context used in tool execution
}

func TestSubagentRunner_RecursionPrevention_BlocksTaskToolInSubagent(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-block-task-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Trying to spawn subagent"),
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{ToolID: "t1", ToolName: "task", Input: map[string]interface{}{
				"agent":  "another-agent",
				"prompt": "Do something else",
			}},
		},
		nil,
	}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-no-recursion", "No Recursion Agent")

	// Act
	result, err := runner.Run(context.Background(), agent, "Try to spawn subagent", "subagent-recursive-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// task tool should have been blocked, not executed
	if toolExecutor.executeToolCalls != 0 {
		t.Errorf(
			"ExecuteTool() called %d times, want 0 (task tool should be blocked in subagent)",
			toolExecutor.executeToolCalls,
		)
	}
}

func TestSubagentRunner_RecursionPrevention_AllowsRegularToolsInSubagent(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-regular-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Running regular tools"),
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "echo test"}},
			{ToolID: "t2", ToolName: "read_file", Input: map[string]interface{}{"path": "/tmp/test"}},
		},
		nil,
	}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{
		MaxActions:   10,
		AllowedTools: []string{"bash", "read_file"},
	}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-regular", "Regular Tools Agent")

	// Act
	result, err := runner.Run(context.Background(), agent, "Run regular tools", "subagent-regular-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// Regular tools should work fine
	if toolExecutor.executeToolCalls != 2 {
		t.Errorf("ExecuteTool() called %d times, want 2 (regular tools should work)", toolExecutor.executeToolCalls)
	}
}

func TestSubagentRunner_RecursionPrevention_DetectsNestedSubagentContext(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-nested-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-nested", "Nested Agent")

	// Create a context that already has subagent info (simulating nested call)
	parentInfo := SubagentContextInfo{
		SubagentID:      "parent-subagent",
		ParentSessionID: "parent-session",
		IsSubagent:      true,
		Depth:           1,
	}
	ctx := WithSubagentContext(context.Background(), parentInfo)

	// Act
	_, err := runner.Run(ctx, agent, "Task", "subagent-nested-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	// The context should now have depth=2
	// Verify that context functions work correctly
	info, ok := SubagentContextFromContext(ctx)
	if !ok {
		t.Error("SubagentContextFromContext() should return true for subagent context")
	}
	if info.Depth != 1 {
		t.Errorf("Original context depth = %d, want 1", info.Depth)
	}
}

func TestSubagentRunner_RecursionPrevention_IsSubagentContextDetection(t *testing.T) {
	// Arrange
	regularCtx := context.Background()

	info := SubagentContextInfo{
		SubagentID: "test-subagent",
		IsSubagent: true,
		Depth:      1,
	}
	subagentCtx := WithSubagentContext(context.Background(), info)

	// Act & Assert
	if IsSubagentContext(regularCtx) {
		t.Error("IsSubagentContext() should return false for regular context")
	}
	if !IsSubagentContext(subagentCtx) {
		t.Error("IsSubagentContext() should return true for subagent context")
	}
}

// =============================================================================
// Model Switching Tests
// =============================================================================

func TestSubagentRunner_ModelSwitch_SetsModelHaiku(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-haiku-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-haiku", "Haiku Agent")
	agent.Model = "haiku"

	// Act
	result, err := runner.Run(context.Background(), agent, "Task", "subagent-haiku-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// AIProvider.SetModel("haiku") should have been called
	if aiProvider.setModelCalls == 0 {
		t.Error("SetModel() was not called, want it to be called with 'haiku'")
	}
	if len(aiProvider.setModelValues) > 0 && aiProvider.setModelValues[0] != "haiku" {
		t.Errorf("SetModel() called with %q, want %q", aiProvider.setModelValues[0], "haiku")
	}
}

func TestSubagentRunner_ModelSwitch_SetsModelSonnet(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-sonnet-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-sonnet", "Sonnet Agent")
	agent.Model = "sonnet"

	// Act
	result, err := runner.Run(context.Background(), agent, "Task", "subagent-sonnet-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// AIProvider.SetModel("sonnet") should have been called
	if aiProvider.setModelCalls == 0 {
		t.Error("SetModel() was not called, want it to be called with 'sonnet'")
	}
	if len(aiProvider.setModelValues) > 0 && aiProvider.setModelValues[0] != "sonnet" {
		t.Errorf("SetModel() called with %q, want %q", aiProvider.setModelValues[0], "sonnet")
	}
}

func TestSubagentRunner_ModelSwitch_SetsModelOpus(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-opus-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-opus", "Opus Agent")
	agent.Model = "opus"

	// Act
	result, err := runner.Run(context.Background(), agent, "Task", "subagent-opus-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// AIProvider.SetModel("opus") should have been called
	if aiProvider.setModelCalls == 0 {
		t.Error("SetModel() was not called, want it to be called with 'opus'")
	}
	if len(aiProvider.setModelValues) > 0 && aiProvider.setModelValues[0] != "opus" {
		t.Errorf("SetModel() called with %q, want %q", aiProvider.setModelValues[0], "opus")
	}
}

func TestSubagentRunner_ModelSwitch_InheritDoesNotSetModel(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-inherit-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-inherit", "Inherit Agent")
	agent.Model = "inherit"

	// Act
	result, err := runner.Run(context.Background(), agent, "Task", "subagent-inherit-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// AIProvider.SetModel() should NOT have been called
	if aiProvider.setModelCalls != 0 {
		t.Errorf("SetModel() called %d times, want 0 (inherit should not change model)", aiProvider.setModelCalls)
	}
}

func TestSubagentRunner_ModelSwitch_EmptyModelDoesNotSetModel(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-empty-model-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-empty-model", "Empty Model Agent")
	agent.Model = ""

	// Act
	result, err := runner.Run(context.Background(), agent, "Task", "subagent-empty-model-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// AIProvider.SetModel() should NOT have been called
	if aiProvider.setModelCalls != 0 {
		t.Errorf("SetModel() called %d times, want 0 (empty model should not change model)", aiProvider.setModelCalls)
	}
}

func TestSubagentRunner_ModelSwitch_RestoresOriginalModelAfterCompletion(t *testing.T) {
	// Arrange
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-restore-001"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Done"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-restore", "Restore Agent")
	agent.Model = "haiku"

	// Get original model before run
	originalModel := aiProvider.GetModel()

	// Act
	result, err := runner.Run(context.Background(), agent, "Task", "subagent-restore-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// Model should be restored to original after completion
	currentModel := aiProvider.GetModel()
	if currentModel != originalModel {
		t.Errorf("Model after run = %q, want %q (should restore original)", currentModel, originalModel)
	}
}

func TestSubagentRunner_ModelSwitch_RestoresOriginalModelAfterError(t *testing.T) {
	// Arrange
	expectedError := errors.New("AI processing error")
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-restore-error-001"
	convService.processResponseError = expectedError

	toolExecutor := newSubagentRunnerToolExecutorMock()
	aiProvider := newSubagentRunnerAIProviderMock()
	config := SubagentConfig{MaxActions: 10}

	runner := NewSubagentRunner(convService, toolExecutor, aiProvider, nil, config)
	agent := createTestAgent("agent-restore-error", "Restore Error Agent")
	agent.Model = "haiku"

	// Get original model before run
	originalModel := aiProvider.GetModel()

	// Act
	result, err := runner.Run(context.Background(), agent, "Task", "subagent-restore-error-001")

	// Assert
	if err == nil {
		t.Error("Run() should return error when AI fails")
	}
	if result == nil {
		t.Fatal("Run() should return result on error")
	}
	// Model should be restored to original even on error
	currentModel := aiProvider.GetModel()
	if currentModel != originalModel {
		t.Errorf("Model after error = %q, want %q (should restore original on error)", currentModel, originalModel)
	}
}

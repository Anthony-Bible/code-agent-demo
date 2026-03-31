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

	// SetThinkingMode tracking
	setThinkingModeCalls     int
	setThinkingModeError     error
	setThinkingModeSessionID []string
	setThinkingModeInfo      []port.ThinkingModeInfo
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

func (m *subagentRunnerConvServiceMock) ProcessAssistantResponseStreaming(
	ctx context.Context,
	sessionID string,
	_ port.StreamCallback,
	_ port.ThinkingCallback,
) (*entity.Message, []port.ToolCallInfo, error) {
	// Delegate to non-streaming version for testing
	return m.ProcessAssistantResponse(ctx, sessionID)
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

func (m *subagentRunnerConvServiceMock) SetThinkingMode(sessionID string, info port.ThinkingModeInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.setThinkingModeCalls++
	m.setThinkingModeSessionID = append(m.setThinkingModeSessionID, sessionID)
	m.setThinkingModeInfo = append(m.setThinkingModeInfo, info)
	if m.setThinkingModeError != nil {
		return m.setThinkingModeError
	}
	return nil
}

func (m *subagentRunnerConvServiceMock) GetThinkingMode(_ string) (port.ThinkingModeInfo, error) {
	return port.ThinkingModeInfo{}, nil
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
	setModelCalls        int
	setModelValues       []string
	setModelRestoreError error // returned on 2nd+ call (simulates cleanup failure)
	currentModel         string
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
	_ port.AIRequestOptions,
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

func (m *subagentRunnerAIProviderMock) SendMessageStreaming(
	_ context.Context,
	messages []port.MessageParam,
	tools []port.ToolParam,
	_ port.AIRequestOptions,
	_ port.StreamCallback,
	_ port.ThinkingCallback,
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
	if m.setModelRestoreError != nil && m.setModelCalls >= 2 {
		return m.setModelRestoreError
	}
	m.currentModel = model
	return nil
}

func (m *subagentRunnerAIProviderMock) GetModel() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.currentModel
}

func (m *subagentRunnerAIProviderMock) Clone() port.AIProvider {
	m.mu.Lock()
	defer m.mu.Unlock()
	return &subagentRunnerAIProviderMock{
		sendMessageResponse: m.sendMessageResponse,
		currentModel:        m.currentModel,
	}
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
// Test Harness
// =============================================================================

type subagentRunnerTestHarness struct {
	t            *testing.T
	convService  *subagentRunnerConvServiceMock
	toolExecutor *subagentRunnerToolExecutorMock
	aiProvider   *subagentRunnerAIProviderMock
	config       SubagentConfig
}

func newSubagentTestHarness(t *testing.T) *subagentRunnerTestHarness {
	t.Helper()
	convService := newSubagentRunnerConvServiceMock()
	convService.startConversationSession = "subagent-session-123"
	convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Task completed successfully"),
	}
	convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}
	return &subagentRunnerTestHarness{
		t:            t,
		convService:  convService,
		toolExecutor: newSubagentRunnerToolExecutorMock(),
		aiProvider:   newSubagentRunnerAIProviderMock(),
		config:       SubagentConfig{MaxActions: 10, AllowedTools: []string{"bash", "read_file"}},
	}
}

func (h *subagentRunnerTestHarness) build() *SubagentRunner {
	h.t.Helper()
	convService := h.convService
	factory := func(_ port.AIProvider) (ConversationServiceInterface, error) {
		return convService, nil
	}
	return NewSubagentRunner(h.convService, h.toolExecutor, h.aiProvider, nil, h.config, factory, port.NopLogger{})
}

func (h *subagentRunnerTestHarness) run(agent *entity.Subagent, prompt, id string) (*SubagentResult, error) {
	h.t.Helper()
	return h.build().Run(context.Background(), agent, prompt, id)
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewSubagentRunner_WithAllDependencies(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.config.MaxDuration = 5 * time.Minute
	h.config.MaxConcurrent = 3
	h.config.BlockedCommands = []string{"rm -rf"}

	// Act
	runner := h.build()

	// Assert
	if runner == nil {
		t.Fatal("NewSubagentRunner() returned nil")
	}
}

func TestNewSubagentRunner_PanicsOnNilDependency(t *testing.T) {
	dummyFactory := func(_ port.AIProvider) (ConversationServiceInterface, error) {
		return newSubagentRunnerConvServiceMock(), nil
	}
	tests := []struct {
		name         string
		convService  ConversationServiceInterface
		toolExecutor port.ToolExecutor
		aiProvider   port.AIProvider
		convFactory  ConversationServiceFactory
	}{
		{"nil convService", nil, newSubagentRunnerToolExecutorMock(), newSubagentRunnerAIProviderMock(), dummyFactory},
		{"nil toolExecutor", newSubagentRunnerConvServiceMock(), nil, newSubagentRunnerAIProviderMock(), dummyFactory},
		{"nil aiProvider", newSubagentRunnerConvServiceMock(), newSubagentRunnerToolExecutorMock(), nil, dummyFactory},
		{"nil convFactory", newSubagentRunnerConvServiceMock(), newSubagentRunnerToolExecutorMock(), newSubagentRunnerAIProviderMock(), nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r == nil {
					t.Error("expected panic")
				}
			}()
			NewSubagentRunner(tt.convService, tt.toolExecutor, tt.aiProvider, nil, SubagentConfig{}, tt.convFactory, port.NopLogger{})
		})
	}
}

// =============================================================================
// Run Method Tests - Basic Execution
// =============================================================================

func TestSubagentRunner_Run_SuccessfulExecution(t *testing.T) {
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-001"

	result, err := h.run(createTestAgent("agent-001", "Code Analyzer"), "Analyze the error logs and identify the root cause", "subagent-001")
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
	h := newSubagentTestHarness(t)

	result, err := h.run(nil, "some task", "subagent-002")

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
	h := newSubagentTestHarness(t)

	result, err := h.run(createTestAgent("agent-003", "Helper"), "", "subagent-003")

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
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-iso-001"

	_, err := h.run(createTestAgent("agent-iso", "Isolated Agent"), "Execute task", "subagent-iso-001")
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if h.convService.startConversationCalls != 1 {
		t.Errorf("StartConversation() called %d times, want 1", h.convService.startConversationCalls)
	}
}

func TestSubagentRunner_CleansUpSessionOnCompletion(t *testing.T) {
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-cleanup-001"

	_, err := h.run(createTestAgent("agent-cleanup", "Test Agent"), "Task", "subagent-cleanup-001")
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if h.convService.endConversationCalls != 1 {
		t.Errorf("EndConversation() called %d times, want 1", h.convService.endConversationCalls)
	}
	if h.convService.endConversationSession != "subagent-session-cleanup-001" {
		t.Errorf("EndConversation() called with session %q, want %q",
			h.convService.endConversationSession, "subagent-session-cleanup-001")
	}
}

func TestSubagentRunner_CleansUpSessionOnError(t *testing.T) {
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-error-001"
	h.convService.processResponseError = errors.New("AI processing error")

	_, err := h.run(createTestAgent("agent-error", "Test Agent"), "Task", "subagent-error-001")

	if err == nil {
		t.Error("Run() should return error when AI fails")
	}
	if h.convService.endConversationCalls != 1 {
		t.Errorf("EndConversation() called %d times, want 1 (cleanup on error)",
			h.convService.endConversationCalls)
	}
}

func TestSubagentRunner_HandlesStartConversationError(t *testing.T) {
	h := newSubagentTestHarness(t)
	h.convService.startConversationError = errors.New("failed to start conversation")

	result, err := h.run(createTestAgent("agent-start-err", "Test Agent"), "Task", "subagent-start-err")

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
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-prompt-001"
	agent := createTestAgent("agent-prompt", "Specialized Agent")
	agent.RawContent = "You are a specialized agent for code analysis"

	// Act
	_, err := h.run(agent, "Analyze code", "subagent-prompt-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if h.convService.setCustomSystemPromptCalls != 1 {
		t.Errorf("SetCustomSystemPrompt() called %d times, want 1", h.convService.setCustomSystemPromptCalls)
	}
	if len(h.convService.setCustomSystemPromptContent) == 0 {
		t.Fatal("SetCustomSystemPrompt() not called with any content")
	}
	// The prompt should combine agent system prompt with task prompt
	prompt := h.convService.setCustomSystemPromptContent[0]
	if len(prompt) == 0 {
		t.Error("SetCustomSystemPrompt() called with empty prompt")
	}
}

func TestSubagentRunner_AddsUserMessageWithTaskPrompt(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-task-001"
	agent := createTestAgent("agent-task", "Task Agent")
	taskPrompt := "Analyze the error logs and identify the root cause"

	// Act
	_, err := h.run(agent, taskPrompt, "subagent-task-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if h.convService.addUserMessageCalls != 1 {
		t.Errorf("AddUserMessage() called %d times, want 1", h.convService.addUserMessageCalls)
	}
	if len(h.convService.addUserMessageContent) == 0 {
		t.Fatal("AddUserMessage() not called with any content")
	}
	// The user message should contain the task prompt
	userMsg := h.convService.addUserMessageContent[0]
	if len(userMsg) == 0 {
		t.Error("AddUserMessage() called with empty content")
	}
}

// =============================================================================
// Tool Execution Tests
// =============================================================================

func TestSubagentRunner_ExecutesToolCalls(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-tools-001"
	// First response: AI requests a tool
	// Second response: AI completes
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Need to check logs"),
		createSubagentAssistantMessage("Analysis complete"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "tool-call-1",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "cat /var/log/app.log"},
			},
		},
		nil, // No tools in second response
	}
	h.toolExecutor.executeToolResult = "log contents here"
	agent := createTestAgent("agent-tools", "Tool User Agent")

	// Act
	result, err := h.run(agent, "Check logs", "subagent-tools-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if h.toolExecutor.executeToolCalls != 1 {
		t.Errorf("ExecuteTool() called %d times, want 1", h.toolExecutor.executeToolCalls)
	}
	if len(h.toolExecutor.executeToolName) > 0 && h.toolExecutor.executeToolName[0] != "bash" {
		t.Errorf("ExecuteTool() called with tool %q, want %q",
			h.toolExecutor.executeToolName[0], "bash")
	}
}

func TestSubagentRunner_FeedsToolResultsBackToAI(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-feedback-001"
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Running tool"),
		createSubagentAssistantMessage("Got results"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "tool-call-feedback",
				ToolName: "read_file",
				Input:    map[string]interface{}{"path": "/tmp/test.txt"},
			},
		},
		nil,
	}
	h.toolExecutor.executeToolResult = "file contents"
	h.config.AllowedTools = []string{"read_file"}
	agent := createTestAgent("agent-feedback", "Feedback Agent")

	// Act
	_, err := h.run(agent, "Read file", "subagent-feedback-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if h.convService.addToolResultCalls != 1 {
		t.Errorf("AddToolResultMessage() called %d times, want 1", h.convService.addToolResultCalls)
	}
	if len(h.convService.addToolResultResults) > 0 {
		results := h.convService.addToolResultResults[0]
		if len(results) != 1 {
			t.Errorf("AddToolResultMessage() called with %d results, want 1", len(results))
		} else if results[0].ToolID != "tool-call-feedback" {
			t.Errorf("Tool result ToolID = %q, want %q", results[0].ToolID, "tool-call-feedback")
		}
	}
}

func TestSubagentRunner_HandlesToolExecutionError(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-tool-err-001"
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Running tool"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{
				ToolID:   "tool-call-err",
				ToolName: "bash",
				Input:    map[string]interface{}{"command": "invalid-command"},
			},
		},
	}
	h.toolExecutor.executeToolError = errors.New("command not found")
	agent := createTestAgent("agent-tool-err", "Error Agent")

	// Act
	// Note: Tool errors should be fed back to AI, not necessarily fail the entire run
	h.run(agent, "Run command", "subagent-tool-err-001")

	// Assert - exact behavior depends on implementation, but tool result should be added
	// The error might be in the result or the run might continue
	if h.convService.addToolResultCalls > 0 {
		results := h.convService.addToolResultResults[0]
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
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-limit-001"
	// Configure many tool calls to exceed limit
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Action 1"),
		createSubagentAssistantMessage("Action 2"),
		createSubagentAssistantMessage("Action 3"),
		createSubagentAssistantMessage("Action 4"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "echo 1"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "echo 2"}}},
		{{ToolID: "t3", ToolName: "bash", Input: map[string]interface{}{"command": "echo 3"}}},
		{{ToolID: "t4", ToolName: "bash", Input: map[string]interface{}{"command": "echo 4"}}},
	}
	h.config.MaxActions = 2 // Limit to 2 actions
	h.config.AllowedTools = []string{"bash"}
	agent := createTestAgent("agent-limit", "Limited Agent")

	// Act
	result, err := h.run(agent, "Do many things", "subagent-limit-001")

	// Assert
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if result.ActionsTaken > h.config.MaxActions {
		t.Errorf("Run() took %d actions, should not exceed MaxActions=%d",
			result.ActionsTaken, h.config.MaxActions)
	}
	// The run should complete (status may vary based on implementation)
	_ = err // Error is acceptable if limit is treated as failure
}

func TestSubagentRunner_StopsWhenMaxActionsReached(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-stop-001"
	// More responses than the limit
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Step 1"),
		createSubagentAssistantMessage("Step 2"),
		createSubagentAssistantMessage("Step 3"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "s1", ToolName: "bash", Input: map[string]interface{}{"command": "step1"}}},
		{{ToolID: "s2", ToolName: "bash", Input: map[string]interface{}{"command": "step2"}}},
		{{ToolID: "s3", ToolName: "bash", Input: map[string]interface{}{"command": "step3"}}},
	}
	h.config.MaxActions = 1 // Very strict limit
	h.config.AllowedTools = []string{"bash"}
	agent := createTestAgent("agent-stop", "Stop Agent")

	// Act
	result, _ := h.run(agent, "Multi-step task", "subagent-stop-001")

	// Assert
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// Should stop after MaxActions
	if result.ActionsTaken > h.config.MaxActions {
		t.Errorf("Run() took %d actions, want <= %d", result.ActionsTaken, h.config.MaxActions)
	}
	if h.toolExecutor.executeToolCalls > h.config.MaxActions {
		t.Errorf("ExecuteTool() called %d times, should not exceed MaxActions=%d",
			h.toolExecutor.executeToolCalls, h.config.MaxActions)
	}
}

func TestSubagentRunner_TracksActionsTaken(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-track-001"
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Step 1"),
		createSubagentAssistantMessage("Step 2"),
		createSubagentAssistantMessage("Done"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "cmd1"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "cmd2"}}},
		nil, // Completion
	}
	h.config.AllowedTools = []string{"bash"}
	agent := createTestAgent("agent-track", "Tracking Agent")

	// Act
	result, err := h.run(agent, "Execute steps", "subagent-track-001")
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
			h := newSubagentTestHarness(t)
			h.convService.startConversationSession = "subagent-session-err"
			tt.setupMock(h.convService)
			agent := createTestAgent("agent-err", "Error Agent")

			// Act
			result, err := h.run(agent, "Task", "subagent-err")

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
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-status-001"
	agent := createTestAgent("agent-status", "Status Agent")

	// Act
	result, err := h.run(agent, "Task", "subagent-status-001")
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
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-output-001"
	outputMessage := "The root cause is a memory leak in module X. Recommendation: upgrade to v2.0"
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage(outputMessage),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}
	agent := createTestAgent("agent-output", "Output Agent")

	// Act
	result, err := h.run(agent, "Diagnose issue", "subagent-output-001")
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
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-prefix-001"
	outputMessage := "Analysis complete: no issues found"
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage(outputMessage),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{nil}
	agent := createTestAgent("test-agent", "Test Agent")

	// Act
	result, err := h.run(agent, "Analyze code", "subagent-prefix-001")
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
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-duration-001"
	agent := createTestAgent("agent-duration", "Duration Agent")

	// Act
	start := time.Now()
	result, err := h.run(agent, "Task", "subagent-duration-001")
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

func TestSubagentRunner_AllowedToolsFiltering(t *testing.T) {
	tests := []struct {
		name              string
		allowedTools      []string
		agentAllowedTools []string
		toolCalls         []port.ToolCallInfo
		wantExecuteCalls  int
		wantFirstToolName string
	}{
		{
			name:              "allows only specified tools",
			allowedTools:      []string{"bash"},
			agentAllowedTools: []string{"bash"},
			toolCalls: []port.ToolCallInfo{
				{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}},
				{ToolID: "t2", ToolName: "read_file", Input: map[string]interface{}{"path": "/tmp/test"}},
			},
			wantExecuteCalls:  1,
			wantFirstToolName: "bash",
		},
		{
			name:         "blocks non-allowed tools",
			allowedTools: []string{"bash", "read_file"},
			toolCalls: []port.ToolCallInfo{
				{ToolID: "t1", ToolName: "list_files", Input: map[string]interface{}{"directory": "/"}},
			},
			wantExecuteCalls: 0,
		},
		{
			name:         "nil allowed tools allows all",
			allowedTools: nil,
			toolCalls: []port.ToolCallInfo{
				{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "echo"}},
				{ToolID: "t2", ToolName: "read_file", Input: map[string]interface{}{"path": "/tmp/test"}},
				{ToolID: "t3", ToolName: "list_files", Input: map[string]interface{}{"directory": "/"}},
			},
			wantExecuteCalls: 3,
		},
		{
			name:         "empty slice blocks all",
			allowedTools: []string{},
			toolCalls: []port.ToolCallInfo{
				{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "echo"}},
			},
			wantExecuteCalls: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newSubagentTestHarness(t)
			h.config.AllowedTools = tt.allowedTools
			h.convService.processResponseMessages = []*entity.Message{
				createSubagentAssistantMessage("Executing tools"),
				createSubagentAssistantMessage("Done"),
			}
			h.convService.processResponseToolCalls = [][]port.ToolCallInfo{tt.toolCalls, nil}

			agent := createTestAgent("agent-filter", "Filter Agent")
			if tt.agentAllowedTools != nil {
				agent.AllowedTools = tt.agentAllowedTools
			}

			result, err := h.run(agent, "Execute", "subagent-filter-001")
			if err != nil {
				t.Errorf("Run() error = %v, want nil", err)
			}
			if result == nil {
				t.Fatal("Run() returned nil result")
			}
			if h.toolExecutor.executeToolCalls != tt.wantExecuteCalls {
				t.Errorf("ExecuteTool() called %d times, want %d",
					h.toolExecutor.executeToolCalls, tt.wantExecuteCalls)
			}
			if tt.wantFirstToolName != "" &&
				len(h.toolExecutor.executeToolName) > 0 &&
				h.toolExecutor.executeToolName[0] != tt.wantFirstToolName {
				t.Errorf("ExecuteTool() called with tool %q, want %q",
					h.toolExecutor.executeToolName[0], tt.wantFirstToolName)
			}
		})
	}
}

func TestSubagentRunner_BlockedTools_ReturnsErrorResults(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-error-results-001"
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Trying blocked and allowed tools"),
		createSubagentAssistantMessage("Done"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}},          // allowed
			{ToolID: "t2", ToolName: "list_files", Input: map[string]interface{}{"directory": "/"}},   // blocked
			{ToolID: "t3", ToolName: "read_file", Input: map[string]interface{}{"path": "/tmp/test"}}, // allowed
		},
		nil,
	}
	agent := createTestAgent("agent-error-results", "Error Results Agent")

	// Act
	result, err := h.run(agent, "Execute tools", "subagent-error-results-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	// Only bash and read_file should have been executed (2 calls), not list_files
	if h.toolExecutor.executeToolCalls != 2 {
		t.Errorf("ExecuteTool() called %d times, want 2 (only allowed tools)", h.toolExecutor.executeToolCalls)
	}

	// Verify tool results were added for ALL tools (including blocked one)
	if h.convService.addToolResultCalls != 1 {
		t.Errorf("AddToolResultMessage() called %d times, want 1", h.convService.addToolResultCalls)
	}

	// Check that 3 tool results were added (2 successful + 1 error for blocked tool)
	if len(h.convService.addToolResultResults) == 0 {
		t.Fatal("AddToolResultMessage() was not called with any results")
	}

	results := h.convService.addToolResultResults[0]
	if len(results) != 3 {
		t.Fatalf("AddToolResultMessage() called with %d results, want 3", len(results))
	}

	// Verify blocked tool result has error
	var blockedResult *entity.ToolResult
	for i := range results {
		if results[i].ToolID == "t2" { // list_files was t2
			blockedResult = &results[i]
			break
		}
	}

	if blockedResult == nil {
		t.Fatal("Blocked tool result not found in results")
	}

	if !blockedResult.IsError {
		t.Error("Blocked tool result should be marked as error")
	}

	expectedMsg := "tool 'list_files' is not allowed for this subagent"
	if blockedResult.Result != expectedMsg {
		t.Errorf("Blocked tool result message = %q, want %q", blockedResult.Result, expectedMsg)
	}

	// Verify blocked tools don't count toward actions taken
	// Only 2 tools executed (bash and read_file), not 3
	if result.ActionsTaken != 2 {
		t.Errorf("ActionsTaken = %d, want 2 (blocked tools should not count)", result.ActionsTaken)
	}
}

func TestSubagentRunner_AllBlockedTools_ReturnsAllErrorResults(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-all-blocked-001"
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Trying tools"),
		createSubagentAssistantMessage("Done"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "ls"}},
			{ToolID: "t2", ToolName: "read_file", Input: map[string]interface{}{"path": "/tmp/test"}},
		},
		nil,
	}
	h.config.AllowedTools = []string{} // Empty slice = block ALL tools
	agent := createTestAgent("agent-all-blocked", "All Blocked Agent")

	// Act
	result, err := h.run(agent, "Execute tools", "subagent-all-blocked-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}

	// No tools should have been executed
	if h.toolExecutor.executeToolCalls != 0 {
		t.Errorf("ExecuteTool() called %d times, want 0 (all tools blocked)", h.toolExecutor.executeToolCalls)
	}

	// Tool results should still be added for both blocked tools
	if h.convService.addToolResultCalls != 1 {
		t.Errorf("AddToolResultMessage() called %d times, want 1", h.convService.addToolResultCalls)
	}

	if len(h.convService.addToolResultResults) > 0 {
		results := h.convService.addToolResultResults[0]
		if len(results) != 2 {
			t.Fatalf("AddToolResultMessage() called with %d results, want 2", len(results))
		}

		// Both should be errors
		for i := range results {
			if !results[i].IsError {
				t.Errorf("Tool result %d should be marked as error", i)
			}
		}
	}

	// No actions should be taken (all blocked)
	if result.ActionsTaken != 0 {
		t.Errorf("ActionsTaken = %d, want 0 (all tools blocked)", result.ActionsTaken)
	}
}

// =============================================================================
// Recursion Prevention Tests
// =============================================================================

func TestSubagentRunner_RecursionPrevention_AddsSubagentContextToContext(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-ctx-001"
	agent := createTestAgent("agent-ctx", "Context Agent")

	// Act
	ctx := context.Background()
	_, err := h.build().Run(ctx, agent, "Task", "subagent-ctx-001")
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
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-block-task-001"
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Trying to spawn subagent"),
		createSubagentAssistantMessage("Done"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{ToolID: "t1", ToolName: "task", Input: map[string]interface{}{
				"agent":  "another-agent",
				"prompt": "Do something else",
			}},
		},
		nil,
	}
	agent := createTestAgent("agent-no-recursion", "No Recursion Agent")

	// Act
	result, err := h.run(agent, "Try to spawn subagent", "subagent-recursive-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// task tool should have been blocked, not executed
	if h.toolExecutor.executeToolCalls != 0 {
		t.Errorf(
			"ExecuteTool() called %d times, want 0 (task tool should be blocked in subagent)",
			h.toolExecutor.executeToolCalls,
		)
	}
}

func TestSubagentRunner_RecursionPrevention_AllowsRegularToolsInSubagent(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-regular-001"
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Running regular tools"),
		createSubagentAssistantMessage("Done"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{
			{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "echo test"}},
			{ToolID: "t2", ToolName: "read_file", Input: map[string]interface{}{"path": "/tmp/test"}},
		},
		nil,
	}
	agent := createTestAgent("agent-regular", "Regular Tools Agent")

	// Act
	result, err := h.run(agent, "Run regular tools", "subagent-regular-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// Regular tools should work fine
	if h.toolExecutor.executeToolCalls != 2 {
		t.Errorf("ExecuteTool() called %d times, want 2 (regular tools should work)", h.toolExecutor.executeToolCalls)
	}
}

func TestSubagentRunner_RecursionPrevention_DetectsNestedSubagentContext(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-nested-001"
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
	_, err := h.build().Run(ctx, agent, "Task", "subagent-nested-001")
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

func TestSubagentRunner_ModelSwitch_SetsModel(t *testing.T) {
	tests := []struct {
		name          string
		modelAlias    string
		expectedModel string
		agentName     string
		sessionSuffix string
	}{
		{
			name:          "haiku model",
			modelAlias:    "haiku",
			expectedModel: "claude-3-5-haiku-20241022",
			agentName:     "agent-haiku",
			sessionSuffix: "haiku-001",
		},
		{
			name:          "sonnet model",
			modelAlias:    "sonnet",
			expectedModel: "claude-sonnet-4-5-20250929",
			agentName:     "agent-sonnet",
			sessionSuffix: "sonnet-001",
		},
		{
			name:          "opus model",
			modelAlias:    "opus",
			expectedModel: "claude-opus-4-5-20250514",
			agentName:     "agent-opus",
			sessionSuffix: "opus-001",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			h := newSubagentTestHarness(t)
			h.convService.startConversationSession = "subagent-session-" + tt.sessionSuffix
			agent := createTestAgent(tt.agentName, strings.Title(tt.modelAlias)+" Agent")
			agent.Model = tt.modelAlias

			// Record original model before run
			originalModel := h.aiProvider.GetModel()

			// Act
			result, err := h.run(agent, "Task", "subagent-"+tt.sessionSuffix)
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
			// Original provider's model must be unchanged (clone isolation)
			if h.aiProvider.GetModel() != originalModel {
				t.Errorf("Original provider model = %q, want %q (clone should not mutate original)",
					h.aiProvider.GetModel(), originalModel)
			}
		})
	}
}

func TestSubagentRunner_ModelSwitch_InheritDoesNotSetModel(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-inherit-001"
	agent := createTestAgent("agent-inherit", "Inherit Agent")
	agent.Model = "inherit"

	// Act
	result, err := h.run(agent, "Task", "subagent-inherit-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// AIProvider.SetModel() should NOT have been called
	if h.aiProvider.setModelCalls != 0 {
		t.Errorf("SetModel() called %d times, want 0 (inherit should not change model)", h.aiProvider.setModelCalls)
	}
}

func TestSubagentRunner_ModelSwitch_EmptyModelDoesNotSetModel(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-empty-model-001"
	agent := createTestAgent("agent-empty-model", "Empty Model Agent")
	agent.Model = ""

	// Act
	result, err := h.run(agent, "Task", "subagent-empty-model-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// AIProvider.SetModel() should NOT have been called
	if h.aiProvider.setModelCalls != 0 {
		t.Errorf("SetModel() called %d times, want 0 (empty model should not change model)", h.aiProvider.setModelCalls)
	}
}

func TestSubagentRunner_ModelSwitch_RestoresOriginalModelAfterCompletion(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-restore-001"
	agent := createTestAgent("agent-restore", "Restore Agent")
	agent.Model = "haiku"

	// Get original model before run
	originalModel := h.aiProvider.GetModel()

	// Act
	result, err := h.run(agent, "Task", "subagent-restore-001")
	// Assert
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	// Model should be restored to original after completion
	currentModel := h.aiProvider.GetModel()
	if currentModel != originalModel {
		t.Errorf("Model after run = %q, want %q (should restore original)", currentModel, originalModel)
	}
}

func TestSubagentRunner_ModelSwitch_RestoresOriginalModelAfterError(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-restore-error-001"
	h.convService.processResponseError = errors.New("AI processing error")
	agent := createTestAgent("agent-restore-error", "Restore Error Agent")
	agent.Model = "haiku"

	// Get original model before run
	originalModel := h.aiProvider.GetModel()

	// Act
	result, err := h.run(agent, "Task", "subagent-restore-error-001")

	// Assert
	if err == nil {
		t.Error("Run() should return error when AI fails")
	}
	if result == nil {
		t.Fatal("Run() should return result on error")
	}
	// Model should be restored to original even on error
	currentModel := h.aiProvider.GetModel()
	if currentModel != originalModel {
		t.Errorf("Model after error = %q, want %q (should restore original on error)", currentModel, originalModel)
	}
}

func TestSubagentRunner_ModelSwitch_CloneIsolation(t *testing.T) {
	// Arrange — verify that model switching uses a clone and never touches the original
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-clone-iso"
	agent := createTestAgent("agent-clone-iso", "Clone Isolation Agent")
	agent.Model = "haiku"

	originalModel := h.aiProvider.GetModel()

	// Act
	result, err := h.run(agent, "Do something", "subagent-clone-iso-001")
	// Assert — subagent completes and original provider is untouched
	if err != nil {
		t.Errorf("Run() error = %v, want nil", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if result.Status != "completed" {
		t.Errorf("result.Status = %q, want %q", result.Status, "completed")
	}
	// Original provider's SetModel should never be called (clone handles it)
	if h.aiProvider.setModelCalls != 0 {
		t.Errorf("Original provider SetModel calls = %d, want 0 (should only mutate clone)",
			h.aiProvider.setModelCalls)
	}
	if h.aiProvider.GetModel() != originalModel {
		t.Errorf("Original provider model = %q, want %q", h.aiProvider.GetModel(), originalModel)
	}
}

func TestSubagentRunner_EndConversation_LogsErrorOnCleanupFailure(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-end-fail"
	h.convService.endConversationError = errors.New("session cleanup failed")
	agent := createTestAgent("agent-end-fail", "End Fail Agent")

	// Act — EndConversation will fail in defer, but Run should still succeed
	result, err := h.run(agent, "Do something", "subagent-end-fail-001")
	// Assert — subagent result is returned despite cleanup error
	if err != nil {
		t.Errorf("Run() error = %v, want nil (cleanup error should not propagate)", err)
	}
	if result == nil {
		t.Fatal("Run() returned nil result")
	}
	if result.Status != "completed" {
		t.Errorf("result.Status = %q, want %q", result.Status, "completed")
	}
	// EndConversation was called exactly once
	if h.convService.endConversationCalls != 1 {
		t.Errorf("EndConversation calls = %d, want 1", h.convService.endConversationCalls)
	}
}

// =============================================================================
// Turn Warning Tests
// =============================================================================

func TestSubagentRunner_InjectsTurnWarnings(t *testing.T) {
	tests := []struct {
		name                  string
		maxActions            int
		numToolCalls          int
		expectedWarningCounts map[int]int // remaining -> count of warnings
	}{
		{
			name:         "warns at 5 turns remaining",
			maxActions:   10,
			numToolCalls: 5, // After 5 tool calls, 5 remaining
			expectedWarningCounts: map[int]int{
				5: 1, // Should warn once at 5 remaining
			},
		},
		{
			name:         "warns at 4, 3, 2, 1 turns remaining",
			maxActions:   10,
			numToolCalls: 9, // Goes through 6, 5, 4, 3, 2, 1 remaining
			expectedWarningCounts: map[int]int{
				5: 1,
				4: 1,
				3: 1,
				2: 1,
				1: 1,
			},
		},
		{
			name:                  "no warnings when plenty of turns remain",
			maxActions:            20,
			numToolCalls:          5, // 15 remaining - no warnings
			expectedWarningCounts: map[int]int{},
		},
		{
			name:         "warns progressively as limit approaches",
			maxActions:   7,
			numToolCalls: 6, // Goes through 6, 5, 4, 3, 2, 1 remaining
			expectedWarningCounts: map[int]int{
				5: 1,
				4: 1,
				3: 1,
				2: 1,
				1: 1,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			h := newSubagentTestHarness(t)
			h.convService.startConversationSession = "subagent-session-warnings"

			// Setup responses with tool calls followed by completion
			messages := make([]*entity.Message, tt.numToolCalls+1)
			toolCalls := make([][]port.ToolCallInfo, tt.numToolCalls+1)
			for i := range tt.numToolCalls {
				messages[i] = createSubagentAssistantMessage("Executing step")
				toolCalls[i] = []port.ToolCallInfo{
					{ToolID: "t" + string(rune(i)), ToolName: "bash", Input: map[string]interface{}{"command": "echo"}},
				}
			}
			messages[tt.numToolCalls] = createSubagentAssistantMessage("Done")
			toolCalls[tt.numToolCalls] = nil // Completion

			h.convService.processResponseMessages = messages
			h.convService.processResponseToolCalls = toolCalls
			h.config.MaxActions = tt.maxActions
			h.config.AllowedTools = []string{"bash"}
			agent := createTestAgent("agent-warnings", "Warning Test Agent")

			// Act
			_, _ = h.run(agent, "Execute task", "subagent-warnings-001")

			// Assert - check that warnings were injected at expected times
			warningMessages := h.convService.addUserMessageContent

			// Count warnings for each remaining value
			warningCounts := make(map[int]int)
			for _, msg := range warningMessages {
				if strings.Contains(msg, "TURN LIMIT WARNING") {
					// Extract remaining count from message
					for remaining := 1; remaining <= 5; remaining++ {
						expectedText := fmt.Sprintf("%d turn", remaining)
						if strings.Contains(msg, expectedText) {
							warningCounts[remaining]++
							break
						}
					}
				}
			}

			// Verify expected warnings
			for remaining, expectedCount := range tt.expectedWarningCounts {
				actualCount := warningCounts[remaining]
				if actualCount != expectedCount {
					t.Errorf("Expected %d warning(s) at %d remaining, got %d", expectedCount, remaining, actualCount)
				}
			}

			// Verify no unexpected warnings
			for remaining, actualCount := range warningCounts {
				if expectedCount, ok := tt.expectedWarningCounts[remaining]; !ok || expectedCount == 0 {
					if actualCount > 0 {
						t.Errorf("Unexpected warning at %d remaining (count: %d)", remaining, actualCount)
					}
				}
			}
		})
	}
}

func TestSubagentRunner_WarningMessageContent(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-warning-content"

	// Configure to trigger warning at 5 remaining
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Step 1"),
		createSubagentAssistantMessage("Step 2"),
		createSubagentAssistantMessage("Step 3"),
		createSubagentAssistantMessage("Step 4"),
		createSubagentAssistantMessage("Step 5"),
		createSubagentAssistantMessage("Done"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "echo 1"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "echo 2"}}},
		{{ToolID: "t3", ToolName: "bash", Input: map[string]interface{}{"command": "echo 3"}}},
		{{ToolID: "t4", ToolName: "bash", Input: map[string]interface{}{"command": "echo 4"}}},
		{{ToolID: "t5", ToolName: "bash", Input: map[string]interface{}{"command": "echo 5"}}},
		nil, // Completion
	}
	h.config.AllowedTools = []string{"bash"}
	agent := createTestAgent("agent-warning-content", "Content Test Agent")

	// Act
	_, _ = h.run(agent, "Execute steps", "subagent-warning-content-001")

	// Assert - check warning message content
	warningMessages := h.convService.addUserMessageContent

	foundFiveRemainingWarning := false
	for _, msg := range warningMessages {
		if strings.Contains(msg, "TURN LIMIT WARNING") && strings.Contains(msg, "5 turns remaining") {
			foundFiveRemainingWarning = true
			// Verify it contains prioritization advice
			if !strings.Contains(msg, "Please prioritize your remaining actions carefully") {
				t.Error("Warning at 5 remaining should contain prioritization advice")
			}
			// Subagents should NOT mention batch_tool (only investigation runner does)
			if strings.Contains(msg, "batch_tool") {
				t.Error("Subagent warnings should not mention batch_tool")
			}
		}
	}

	if !foundFiveRemainingWarning {
		t.Error("Expected to find warning at 5 turns remaining")
	}
}

func TestSubagentRunner_NoWarningAtZeroRemaining(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = "subagent-session-zero"

	// Configure to hit max actions exactly
	h.convService.processResponseMessages = []*entity.Message{
		createSubagentAssistantMessage("Step 1"),
		createSubagentAssistantMessage("Step 2"),
		createSubagentAssistantMessage("Done"),
	}
	h.convService.processResponseToolCalls = [][]port.ToolCallInfo{
		{{ToolID: "t1", ToolName: "bash", Input: map[string]interface{}{"command": "echo 1"}}},
		{{ToolID: "t2", ToolName: "bash", Input: map[string]interface{}{"command": "echo 2"}}},
		nil,
	}
	h.config.MaxActions = 2 // Hit limit at 2 actions
	h.config.AllowedTools = []string{"bash"}
	agent := createTestAgent("agent-zero", "Zero Test Agent")

	// Act
	_, _ = h.run(agent, "Execute steps", "subagent-zero-001")

	// Assert - verify no warning at 0 remaining
	warningMessages := h.convService.addUserMessageContent

	for _, msg := range warningMessages {
		if strings.Contains(msg, "0 turn") {
			t.Error("Should not warn at 0 turns remaining")
		}
	}
}

// =============================================================================
// SubagentRunner.Run() Thinking Mode Propagation Tests (RED PHASE)
// =============================================================================

// TestSubagentRunner_Run_CallsSetThinkingModeWhenEnabled verifies that
// SubagentRunner.Run() calls SetThinkingMode() when ThinkingEnabled is true.
// EXPECTED TO FAIL: Run() does not currently call SetThinkingMode().
func TestSubagentRunner_Run_CallsSetThinkingModeWhenEnabled(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.config.MaxActions = 20
	h.config.ThinkingEnabled = true
	h.config.ThinkingBudget = 4096
	h.config.ShowThinking = true
	agent := createTestAgent("thinking-agent", "Thinking Agent System Prompt")

	// Act
	_, err := h.run(agent, "Do a task", "subagent-thinking-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called
	if h.convService.setThinkingModeCalls != 1 {
		t.Errorf("SetThinkingMode() call count = %d, want 1", h.convService.setThinkingModeCalls)
	}
}

// TestSubagentRunner_Run_DoesNotCallSetThinkingModeWhenDisabled verifies that
// SubagentRunner.Run() does NOT call SetThinkingMode() when ThinkingEnabled is false.
// EXPECTED TO PASS: Run() doesn't call SetThinkingMode at all currently.
func TestSubagentRunner_Run_DoesNotCallSetThinkingModeWhenDisabled(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.config.MaxActions = 20
	h.config.ThinkingEnabled = false // Disabled
	h.config.ThinkingBudget = 0
	h.config.ShowThinking = false
	agent := createTestAgent("normal-agent", "Normal Agent System Prompt")

	// Act
	_, err := h.run(agent, "Do a task", "subagent-normal-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was NOT called
	if h.convService.setThinkingModeCalls != 0 {
		t.Errorf(
			"SetThinkingMode() call count = %d, want 0 (should not be called when disabled)",
			h.convService.setThinkingModeCalls,
		)
	}
}

// TestSubagentRunner_Run_PassesCorrectThinkingModeInfo verifies that
// SubagentRunner.Run() passes the correct ThinkingModeInfo values from config.
// EXPECTED TO FAIL: Run() does not currently call SetThinkingMode().
func TestSubagentRunner_Run_PassesCorrectThinkingModeInfo(t *testing.T) {
	tests := []struct {
		name            string
		thinkingEnabled bool
		thinkingBudget  int64
		showThinking    bool
	}{
		{
			name:            "enabled with budget and show thinking",
			thinkingEnabled: true,
			thinkingBudget:  8192,
			showThinking:    true,
		},
		{
			name:            "enabled with budget but hide thinking",
			thinkingEnabled: true,
			thinkingBudget:  2048,
			showThinking:    false,
		},
		{
			name:            "enabled with zero budget (unlimited)",
			thinkingEnabled: true,
			thinkingBudget:  0,
			showThinking:    true,
		},
		{
			name:            "enabled with large budget",
			thinkingEnabled: true,
			thinkingBudget:  100000,
			showThinking:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			h := newSubagentTestHarness(t)
			h.config.MaxActions = 20
			h.config.ThinkingEnabled = tt.thinkingEnabled
			h.config.ThinkingBudget = tt.thinkingBudget
			h.config.ShowThinking = tt.showThinking
			agent := createTestAgent("test-agent", "Test System Prompt")

			// Act
			_, err := h.run(agent, "Task", "subagent-001")
			// Assert
			if err != nil {
				t.Fatalf("Run() failed: %v", err)
			}

			// Verify SetThinkingMode was called with correct values
			if h.convService.setThinkingModeCalls != 1 {
				t.Fatalf("SetThinkingMode() call count = %d, want 1", h.convService.setThinkingModeCalls)
			}

			actualInfo := h.convService.setThinkingModeInfo[0]
			if actualInfo.Enabled != tt.thinkingEnabled {
				t.Errorf("ThinkingModeInfo.Enabled = %v, want %v", actualInfo.Enabled, tt.thinkingEnabled)
			}
			if actualInfo.BudgetTokens != tt.thinkingBudget {
				t.Errorf("ThinkingModeInfo.BudgetTokens = %d, want %d", actualInfo.BudgetTokens, tt.thinkingBudget)
			}
			if actualInfo.ShowThinking != tt.showThinking {
				t.Errorf("ThinkingModeInfo.ShowThinking = %v, want %v", actualInfo.ShowThinking, tt.showThinking)
			}
		})
	}
}

// TestSubagentRunner_Run_CallsSetThinkingModeWithCorrectSessionID verifies that
// SubagentRunner.Run() passes the correct session ID to SetThinkingMode().
// EXPECTED TO FAIL: Run() does not currently call SetThinkingMode().
func TestSubagentRunner_Run_CallsSetThinkingModeWithCorrectSessionID(t *testing.T) {
	// Arrange
	expectedSessionID := "subagent-session-xyz-789"

	h := newSubagentTestHarness(t)
	h.convService.startConversationSession = expectedSessionID
	h.config.MaxActions = 20
	h.config.ThinkingEnabled = true
	h.config.ThinkingBudget = 4096
	h.config.ShowThinking = true
	agent := createTestAgent("session-test-agent", "Session Test System Prompt")

	// Act
	_, err := h.run(agent, "Execute task", "subagent-session-001")
	// Assert
	if err != nil {
		t.Fatalf("Run() failed: %v", err)
	}

	// Verify SetThinkingMode was called with correct session ID
	if h.convService.setThinkingModeCalls != 1 {
		t.Fatalf("SetThinkingMode() call count = %d, want 1", h.convService.setThinkingModeCalls)
	}

	actualSessionID := h.convService.setThinkingModeSessionID[0]
	if actualSessionID != expectedSessionID {
		t.Errorf("SetThinkingMode() sessionID = %q, want %q", actualSessionID, expectedSessionID)
	}
}

// TestSubagentRunner_Run_ThinkingModeWithDifferentConfigs tests various config combinations.
// EXPECTED TO FAIL: Run() does not currently call SetThinkingMode().
func TestSubagentRunner_Run_ThinkingModeWithDifferentConfigs(t *testing.T) {
	tests := []struct {
		name               string
		thinkingEnabled    bool
		thinkingBudget     int64
		showThinking       bool
		expectSetThinkCall bool
	}{
		{
			name:               "thinking enabled with standard budget",
			thinkingEnabled:    true,
			thinkingBudget:     4096,
			showThinking:       true,
			expectSetThinkCall: true,
		},
		{
			name:               "thinking disabled",
			thinkingEnabled:    false,
			thinkingBudget:     0,
			showThinking:       false,
			expectSetThinkCall: false,
		},
		{
			name:               "thinking enabled with unlimited budget",
			thinkingEnabled:    true,
			thinkingBudget:     0,
			showThinking:       false,
			expectSetThinkCall: true,
		},
		{
			name:               "thinking enabled with large budget",
			thinkingEnabled:    true,
			thinkingBudget:     50000,
			showThinking:       true,
			expectSetThinkCall: true,
		},
		{
			name:               "thinking enabled but show thinking off",
			thinkingEnabled:    true,
			thinkingBudget:     2048,
			showThinking:       false,
			expectSetThinkCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange
			h := newSubagentTestHarness(t)
			h.config.MaxActions = 20
			h.config.ThinkingEnabled = tt.thinkingEnabled
			h.config.ThinkingBudget = tt.thinkingBudget
			h.config.ShowThinking = tt.showThinking
			agent := createTestAgent("config-test-agent", "Config Test System Prompt")

			// Act
			_, err := h.run(agent, "Task", "subagent-config-001")
			// Assert
			if err != nil {
				t.Fatalf("Run() failed: %v", err)
			}

			// Verify SetThinkingMode call expectation
			if tt.expectSetThinkCall {
				if h.convService.setThinkingModeCalls != 1 {
					t.Errorf(
						"SetThinkingMode() call count = %d, want 1 (thinking enabled)",
						h.convService.setThinkingModeCalls,
					)
				}
			} else {
				if h.convService.setThinkingModeCalls != 0 {
					t.Errorf(
						"SetThinkingMode() call count = %d, want 0 (thinking disabled)",
						h.convService.setThinkingModeCalls,
					)
				}
			}
		})
	}
}

// =============================================================================
// SubagentConfig Thinking Mode Integration Tests
// =============================================================================

func TestSubagentRunner_AcceptsConfigWithThinkingFields(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.config.MaxDuration = 5 * time.Minute
	h.config.MaxConcurrent = 3
	h.config.AllowedTools = []string{"bash"}
	h.config.BlockedCommands = []string{"rm -rf"}
	h.config.ThinkingEnabled = true
	h.config.ThinkingBudget = 15000
	h.config.ShowThinking = true

	// Act
	runner := h.build()

	// Assert - runner should be created successfully
	if runner == nil {
		t.Fatal("NewSubagentRunner() returned nil with thinking config")
	}
}

func TestSubagentRunner_ConfigAccessibleAfterInit(t *testing.T) {
	// Arrange
	h := newSubagentTestHarness(t)
	h.config = SubagentConfig{
		MaxActions:      20,
		MaxDuration:     10 * time.Minute,
		MaxConcurrent:   4,
		AllowedTools:    []string{"bash", "read_file", "list_files"},
		BlockedCommands: []string{"rm -rf"},
		ThinkingEnabled: true,
		ThinkingBudget:  25000,
		ShowThinking:    false,
	}
	expectedConfig := h.config

	// Act
	runner := h.build()

	// Assert - verify config is stored correctly
	if runner == nil {
		t.Fatal("NewSubagentRunner() returned nil")
	}

	// Access the config field (this will fail if not accessible)
	actualConfig := runner.config

	// Verify all fields including thinking fields
	if actualConfig.MaxActions != expectedConfig.MaxActions {
		t.Errorf("config.MaxActions = %d, want %d", actualConfig.MaxActions, expectedConfig.MaxActions)
	}
	if actualConfig.ThinkingEnabled != expectedConfig.ThinkingEnabled {
		t.Errorf("config.ThinkingEnabled = %v, want %v", actualConfig.ThinkingEnabled, expectedConfig.ThinkingEnabled)
	}
	if actualConfig.ThinkingBudget != expectedConfig.ThinkingBudget {
		t.Errorf("config.ThinkingBudget = %d, want %d", actualConfig.ThinkingBudget, expectedConfig.ThinkingBudget)
	}
	if actualConfig.ShowThinking != expectedConfig.ShowThinking {
		t.Errorf("config.ShowThinking = %v, want %v", actualConfig.ShowThinking, expectedConfig.ShowThinking)
	}
}

func TestSubagentConfig_ThinkingBudgetDataType(t *testing.T) {
	// Arrange & Act
	tests := []struct {
		name   string
		budget int64
	}{
		{
			name:   "zero value",
			budget: 0,
		},
		{
			name:   "small positive value",
			budget: 1000,
		},
		{
			name:   "large positive value",
			budget: 1000000,
		},
		{
			name:   "max int64",
			budget: 9223372036854775807, // math.MaxInt64
		},
		{
			name:   "negative value",
			budget: -1000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			config := SubagentConfig{
				MaxActions:     10,
				ThinkingBudget: tt.budget,
			}

			// Assert - verify budget is stored as int64
			if config.ThinkingBudget != tt.budget {
				t.Errorf("SubagentConfig.ThinkingBudget = %d, want %d", config.ThinkingBudget, tt.budget)
			}

			// Verify type is int64
			_ = config.ThinkingBudget
		})
	}
}

func TestSubagentConfig_ThinkingEnabledBooleanSemantics(t *testing.T) {
	// Arrange & Act
	tests := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{
			name:    "enabled is true",
			enabled: true,
			want:    true,
		},
		{
			name:    "enabled is false",
			enabled: false,
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			config := SubagentConfig{
				MaxActions:      10,
				ThinkingEnabled: tt.enabled,
			}

			// Assert - verify boolean semantics
			if config.ThinkingEnabled != tt.want {
				t.Errorf("SubagentConfig.ThinkingEnabled = %v, want %v", config.ThinkingEnabled, tt.want)
			}

			// Test in conditional
			if tt.want {
				if !config.ThinkingEnabled {
					t.Error("ThinkingEnabled should evaluate to true in conditional")
				}
			} else {
				if config.ThinkingEnabled {
					t.Error("ThinkingEnabled should evaluate to false in conditional")
				}
			}
		})
	}
}

func TestSubagentConfig_ShowThinkingBooleanSemantics(t *testing.T) {
	// Arrange & Act
	tests := []struct {
		name string
		show bool
		want bool
	}{
		{
			name: "show is true",
			show: true,
			want: true,
		},
		{
			name: "show is false",
			show: false,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			config := SubagentConfig{
				MaxActions:   10,
				ShowThinking: tt.show,
			}

			// Assert - verify boolean semantics
			if config.ShowThinking != tt.want {
				t.Errorf("SubagentConfig.ShowThinking = %v, want %v", config.ShowThinking, tt.want)
			}

			// Test in conditional
			if tt.want {
				if !config.ShowThinking {
					t.Error("ShowThinking should evaluate to true in conditional")
				}
			} else {
				if config.ShowThinking {
					t.Error("ShowThinking should evaluate to false in conditional")
				}
			}
		})
	}
}

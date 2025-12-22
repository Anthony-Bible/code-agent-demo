package service

import (
	"code-editing-agent/test/domain/entity"
	"code-editing-agent/test/domain/port"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrToolNotFound         = errors.New("tool not found")
)

// ConversationService handles the core business logic for managing conversations.
// It orchestrates the flow of messages between users and AI, processes tool executions,
// maintains conversation state, and coordinates with the AI provider.
type ConversationService struct {
	aiProvider     port.AIProvider
	toolExecutor   port.ToolExecutor
	conversations  map[string]*entity.Conversation
	currentSession string
	processing     map[string]bool
}

// NewConversationService creates a new instance of ConversationService.
// It requires an AI provider and tool executor for operations.
func NewConversationService(aiProvider port.AIProvider, toolExecutor port.ToolExecutor) (*ConversationService, error) {
	if aiProvider == nil {
		return nil, errors.New("AI provider cannot be nil")
	}
	if toolExecutor == nil {
		return nil, errors.New("tool executor cannot be nil")
	}

	return &ConversationService{
		aiProvider:    aiProvider,
		toolExecutor:  toolExecutor,
		conversations: make(map[string]*entity.Conversation),
		processing:    make(map[string]bool),
	}, nil
}

// StartConversation creates a new conversation session with a unique identifier.
func (cs *ConversationService) StartConversation(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", context.Canceled
	default:
	}

	sessionID := generateSessionID()
	conversation, err := entity.NewConversation()
	if err != nil {
		return "", err
	}

	cs.conversations[sessionID] = conversation
	cs.currentSession = sessionID
	cs.processing[sessionID] = false

	return sessionID, nil
}

// AddUserMessage adds a user message to the current conversation.
func (cs *ConversationService) AddUserMessage(ctx context.Context, sessionID, content string) (*entity.Message, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
	}

	conversation, exists := cs.conversations[sessionID]
	if !exists {
		return nil, ErrConversationNotFound
	}

	message, err := entity.NewMessage(entity.RoleUser, content)
	if err != nil {
		return nil, err
	}

	err = conversation.AddMessage(*message)
	if err != nil {
		return nil, err
	}

	return message, nil
}

// ProcessAssistantResponse processes an AI assistant response, handling tools and text.
func (cs *ConversationService) ProcessAssistantResponse(
	ctx context.Context,
	sessionID string,
) (*entity.Message, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
	}

	conversation, exists := cs.conversations[sessionID]
	if !exists {
		return nil, ErrConversationNotFound
	}

	// Get conversation history for AI provider
	messages := conversation.GetMessages()
	messageParams := make([]port.MessageParam, len(messages))
	for i, msg := range messages {
		messageParams[i] = port.MessageParam{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	// Get available tools
	tools, err := cs.toolExecutor.ListTools()
	if err != nil {
		return nil, err
	}

	toolParams := make([]port.ToolParam, len(tools))
	for i, tool := range tools {
		toolParams[i] = port.ToolParam{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}

	// Send to AI provider
	response, err := cs.aiProvider.SendMessage(ctx, messageParams, toolParams)
	if err != nil {
		return nil, err
	}

	// Add response to conversation
	err = conversation.AddMessage(*response)
	if err != nil {
		return nil, err
	}

	// Check if response contains tool usage
	if cs.containsToolUse(response.Content) {
		cs.processing[sessionID] = true
	} else {
		cs.processing[sessionID] = false
	}

	return response, nil
}

// ExecuteToolsInResponse executes all tools requested in an assistant response.
func (cs *ConversationService) ExecuteToolsInResponse(
	ctx context.Context,
	sessionID string,
	assistantMessage *entity.Message,
) ([]string, error) {
	select {
	case <-ctx.Done():
		return nil, context.Canceled
	default:
	}

	_, exists := cs.conversations[sessionID]
	if !exists {
		return nil, errors.New("conversation not found")
	}

	toolRequests := cs.parseToolRequests(assistantMessage.Content)
	results := make([]string, 0, len(toolRequests))

	for _, request := range toolRequests {
		_, found := cs.toolExecutor.GetTool(request.Name)
		if !found {
			results = append(results, "tool not found")
			continue
		}

		result, err := cs.toolExecutor.ExecuteTool(ctx, request.Name, request.Input)
		if err != nil {
			return nil, err
		}

		results = append(results, result)
	}

	// Reset processing state after executing tools
	cs.processing[sessionID] = false

	return results, nil
}

// GetConversation retrieves a conversation by session ID.
func (cs *ConversationService) GetConversation(sessionID string) (*entity.Conversation, error) {
	conversation, exists := cs.conversations[sessionID]
	if !exists {
		return nil, ErrConversationNotFound
	}
	return conversation, nil
}

// GetCurrentSession returns the current active session ID.
func (cs *ConversationService) GetCurrentSession() (string, error) {
	return cs.currentSession, nil
}

// EndConversation concludes a conversation session, performing cleanup if needed.
func (cs *ConversationService) EndConversation(ctx context.Context, sessionID string) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	default:
	}

	_, exists := cs.conversations[sessionID]
	if !exists {
		return ErrConversationNotFound
	}

	// If ending current session, clear it
	if cs.currentSession == sessionID {
		cs.currentSession = ""
	}

	// Remove processing state
	delete(cs.processing, sessionID)

	return nil
}

// IsProcessing checks if the conversation is currently processing (waiting for tool results).
func (cs *ConversationService) IsProcessing(sessionID string) (bool, error) {
	_, exists := cs.conversations[sessionID]
	if !exists {
		return false, ErrConversationNotFound
	}
	return cs.processing[sessionID], nil
}

// SetProcessingState sets the processing state of a conversation.
func (cs *ConversationService) SetProcessingState(sessionID string, processing bool) error {
	_, exists := cs.conversations[sessionID]
	if !exists {
		return ErrConversationNotFound
	}
	cs.processing[sessionID] = processing
	return nil
}

// --- TESTS for ConversationService ---

func TestNewConversationService(t *testing.T) {
	tests := []struct {
		name         string
		aiProvider   port.AIProvider
		toolExecutor port.ToolExecutor
		expectError  bool
		errorMsg     string
	}{
		{
			name:         "valid dependencies",
			aiProvider:   &mockAIProvider{},
			toolExecutor: &mockToolExecutor{},
			expectError:  false,
		},
		{
			name:         "nil AI provider",
			aiProvider:   nil,
			toolExecutor: &mockToolExecutor{},
			expectError:  true,
			errorMsg:     "AI provider cannot be nil",
		},
		{
			name:         "nil tool executor",
			aiProvider:   &mockAIProvider{},
			toolExecutor: nil,
			expectError:  true,
			errorMsg:     "tool executor cannot be nil",
		},
		{
			name:         "both dependencies nil",
			aiProvider:   nil,
			toolExecutor: nil,
			expectError:  true,
			errorMsg:     "AI provider cannot be nil",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewConversationService(tt.aiProvider, tt.toolExecutor)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				} else if err.Error() != tt.errorMsg {
					t.Errorf("expected error message '%s' but got '%s'", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if service == nil {
					t.Errorf("expected service instance but got nil")
				}
				if service.aiProvider != tt.aiProvider {
					t.Errorf("AI provider not set correctly")
				}
				if service.toolExecutor != tt.toolExecutor {
					t.Errorf("Tool executor not set correctly")
				}
				if service.conversations == nil {
					t.Errorf("Conversations map not initialized")
				}
				if len(service.conversations) != 0 {
					t.Errorf("Expected empty conversations map but got %d items", len(service.conversations))
				}
				if service.currentSession != "" {
					t.Errorf("Expected empty current session but got '%s'", service.currentSession)
				}
			}
		})
	}
}

func TestStartConversation(t *testing.T) {
	service, err := NewConversationService(&mockAIProvider{}, &mockToolExecutor{})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()

	t.Run("successful conversation start", func(t *testing.T) {
		sessionID, err := service.StartConversation(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if sessionID == "" {
			t.Errorf("expected non-empty session ID")
		}

		// Check conversation was created
		conversation, err := service.GetConversation(sessionID)
		if err != nil {
			t.Errorf("failed to get created conversation: %v", err)
		}
		if conversation == nil {
			t.Errorf("expected conversation to exist")
		}
		if conversation.MessageCount() != 0 {
			t.Errorf("expected empty conversation but got %d messages", conversation.MessageCount())
		}

		// Check current session is set
		currentSession, err := service.GetCurrentSession()
		if err != nil {
			t.Errorf("failed to get current session: %v", err)
		}
		if currentSession != sessionID {
			t.Errorf("expected current session '%s' but got '%s'", sessionID, currentSession)
		}
	})

	t.Run("multiple conversations", func(t *testing.T) {
		sessionID1, err := service.StartConversation(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		sessionID2, err := service.StartConversation(ctx)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		if sessionID1 == sessionID2 {
			t.Errorf("expected unique session IDs but got duplicates")
		}

		// Both conversations should exist
		conv1, err := service.GetConversation(sessionID1)
		if err != nil || conv1 == nil {
			t.Errorf("first conversation not found")
		}
		conv2, err := service.GetConversation(sessionID2)
		if err != nil || conv2 == nil {
			t.Errorf("second conversation not found")
		}

		// Current session should be the latest one
		currentSession, err := service.GetCurrentSession()
		if err != nil {
			t.Errorf("failed to get current session: %v", err)
		}
		if currentSession != sessionID2 {
			t.Errorf("expected current session '%s' but got '%s'", sessionID2, currentSession)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, err := service.StartConversation(ctx)
		if err == nil {
			t.Errorf("expected error due to context cancellation")
		}
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error but got %v", err)
		}
	})
}

func TestAddUserMessage(t *testing.T) {
	service, err := NewConversationService(&mockAIProvider{}, &mockToolExecutor{})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()
	sessionID, _ := service.StartConversation(ctx)

	t.Run("valid user message", func(t *testing.T) {
		content := "Hello, AI assistant!"
		message, err := service.AddUserMessage(ctx, sessionID, content)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if message == nil {
			t.Errorf("expected message but got nil")
		}
		if !message.IsUser() {
			t.Errorf("expected user message but got role %s", message.Role)
		}
		if message.Content != content {
			t.Errorf("expected content '%s' but got '%s'", content, message.Content)
		}
		if message.Timestamp.IsZero() {
			t.Errorf("expected non-zero timestamp")
		}

		// Verify message in conversation
		conversation, _ := service.GetConversation(sessionID)
		if conversation.MessageCount() != 1 {
			t.Errorf("expected 1 message in conversation but got %d", conversation.MessageCount())
		}
	})

	t.Run("empty content", func(t *testing.T) {
		_, err := service.AddUserMessage(ctx, sessionID, "")

		if err == nil {
			t.Errorf("expected error for empty content")
		}
		if !errors.Is(err, entity.ErrEmptyContent) {
			t.Errorf("expected ErrEmptyContent but got %v", err)
		}
	})

	t.Run("whitespace only content", func(t *testing.T) {
		_, err := service.AddUserMessage(ctx, sessionID, "   \t\n  ")

		if err == nil {
			t.Errorf("expected error for whitespace-only content")
		}
		if !errors.Is(err, entity.ErrInvalidContent) {
			t.Errorf("expected ErrInvalidContent but got %v", err)
		}
	})

	t.Run("invalid session ID", func(t *testing.T) {
		_, err := service.AddUserMessage(ctx, "invalid-session", "test message")

		if err == nil {
			t.Errorf("expected error for invalid session")
		}
		if !errors.Is(err, ErrConversationNotFound) {
			t.Errorf("expected conversation not found error but got %v", err)
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := service.AddUserMessage(ctx, sessionID, "test")
		if !errors.Is(err, context.Canceled) {
			t.Errorf("expected context.Canceled error but got %v", err)
		}
	})
}

func TestProcessAssistantResponse(t *testing.T) {
	service, err := NewConversationService(&mockAIProvider{}, &mockToolExecutor{})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()
	sessionID, _ := service.StartConversation(ctx)

	// Add a user message first
	_, _ = service.AddUserMessage(ctx, sessionID, "What files are in the current directory?")

	t.Run("text response", func(t *testing.T) {
		aiProvider := &mockAIProvider{
			response: &entity.Message{
				Role:    entity.RoleAssistant,
				Content: "Here are the files in the current directory...",
			},
		}
		service.aiProvider = aiProvider

		response, err := service.ProcessAssistantResponse(ctx, sessionID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if response == nil {
			t.Errorf("expected response but got nil")
		}
		if !response.IsAssistant() {
			t.Errorf("expected assistant response but got role %s", response.Role)
		}
		if response.Content != "Here are the files in the current directory..." {
			t.Errorf("unexpected response content")
		}

		// Check processing state
		processing, _ := service.IsProcessing(sessionID)
		if processing {
			t.Errorf("expected processing to be false for text response")
		}

		// Verify message added to conversation
		conversation, _ := service.GetConversation(sessionID)
		if conversation.MessageCount() != 2 { // user + assistant
			t.Errorf("expected 2 messages but got %d", conversation.MessageCount())
		}
	})

	t.Run("tool use response", func(t *testing.T) {
		// Mock a response that requests tool execution
		toolResponse := &entity.Message{
			Role:    entity.RoleAssistant,
			Content: `{"type": "tool_use", "id": "tool_123", "name": "list_files", "input": {"path": "."}}`,
		}

		aiProvider := &mockAIProvider{
			response: toolResponse,
		}
		service.aiProvider = aiProvider

		response, err := service.ProcessAssistantResponse(ctx, sessionID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if response == nil {
			t.Errorf("expected response but got nil")
		}

		// Check processing state - should be true when tools are involved
		processing, _ := service.IsProcessing(sessionID)
		if !processing {
			t.Errorf("expected processing to be true for tool response")
		}
	})

	t.Run("AI provider error", func(t *testing.T) {
		aiProvider := &mockAIProvider{
			err: errors.New("AI service unavailable"),
		}
		service.aiProvider = aiProvider

		_, err := service.ProcessAssistantResponse(ctx, sessionID)

		if err == nil {
			t.Errorf("expected error from AI provider")
		}
		if err.Error() != "AI service unavailable" {
			t.Errorf("expected AI service error but got %v", err)
		}
	})

	t.Run("invalid session", func(t *testing.T) {
		_, err := service.ProcessAssistantResponse(ctx, "invalid-session")

		if err == nil {
			t.Errorf("expected error for invalid session")
		}
	})
}

func TestExecuteToolsInResponse(t *testing.T) {
	service, err := NewConversationService(&mockAIProvider{}, &mockToolExecutor{})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()
	sessionID, _ := service.StartConversation(ctx)

	// Register a test tool
	testTool, _ := entity.NewTool("test_tool", "list_files", "List files in directory")
	testTool.AddInputSchema(map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{"type": "string"},
		},
		"required": []string{"path"},
	}, []string{"path"})

	service.toolExecutor.RegisterTool(*testTool)

	t.Run("single tool execution", func(t *testing.T) {
		toolExecutor := &mockToolExecutor{
			results: map[string]string{
				"list_files": "file1.txt\nfile2.go\n",
			},
		}
		service.toolExecutor = toolExecutor

		assistantMessage := &entity.Message{
			Role:    entity.RoleAssistant,
			Content: `{"type": "tool_use", "id": "tool_123", "name": "list_files", "input": {"path": "."}}`,
		}

		results, err := service.ExecuteToolsInResponse(ctx, sessionID, assistantMessage)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 result but got %d", len(results))
		}
		if results[0] != "file1.txt\nfile2.go\n" {
			t.Errorf("unexpected tool result: %s", results[0])
		}
	})

	t.Run("multiple tool execution", func(t *testing.T) {
		toolExecutor := &mockToolExecutor{
			results: map[string]string{
				"list_files": "file1.txt\n",
				"read_file":  "file content",
			},
		}
		service.toolExecutor = toolExecutor

		// Create a message with multiple tool uses
		content := `[
			{"type": "tool_use", "id": "tool_1", "name": "list_files", "input": {"path": "."}},
			{"type": "tool_use", "id": "tool_2", "name": "read_file", "input": {"path": "file1.txt"}}
		]`

		assistantMessage := &entity.Message{
			Role:    entity.RoleAssistant,
			Content: content,
		}

		results, err := service.ExecuteToolsInResponse(ctx, sessionID, assistantMessage)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if len(results) != 2 {
			t.Errorf("expected 2 results but got %d", len(results))
		}
	})

	t.Run("tool execution error", func(t *testing.T) {
		toolExecutor := &mockToolExecutor{
			err: errors.New("tool execution failed"),
		}
		service.toolExecutor = toolExecutor

		assistantMessage := &entity.Message{
			Role:    entity.RoleAssistant,
			Content: `{"type": "tool_use", "id": "tool_123", "name": "list_files", "input": {"path": "."}}`,
		}

		results, err := service.ExecuteToolsInResponse(ctx, sessionID, assistantMessage)

		if err == nil {
			t.Errorf("expected error from tool execution")
		}
		if len(results) != 0 {
			t.Errorf("expected no results on error but got %d", len(results))
		}
	})

	t.Run("non-existent tool", func(t *testing.T) {
		assistantMessage := &entity.Message{
			Role:    entity.RoleAssistant,
			Content: `{"type": "tool_use", "id": "tool_123", "name": "non_existent_tool", "input": {}}`,
		}

		results, err := service.ExecuteToolsInResponse(ctx, sessionID, assistantMessage)
		if err != nil {
			t.Errorf("unexpected error for non-existent tool: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("expected 1 error result but got %d", len(results))
		}
		if results[0] != "tool not found" {
			t.Errorf("expected 'tool not found' error but got: %s", results[0])
		}
	})
}

func TestEndConversation(t *testing.T) {
	service, err := NewConversationService(&mockAIProvider{}, &mockToolExecutor{})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()
	sessionID1, _ := service.StartConversation(ctx)
	sessionID2, _ := service.StartConversation(ctx)

	t.Run("successful end", func(t *testing.T) {
		err := service.EndConversation(ctx, sessionID1)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Conversation should still exist but be marked as ended
		_, exists := service.conversations[sessionID1]
		if !exists {
			t.Errorf("conversation should still exist after ending")
		}
	})

	t.Run("end non-existent session", func(t *testing.T) {
		err := service.EndConversation(ctx, "non-existent")

		if err == nil {
			t.Errorf("expected error for non-existent session")
		}
	})

	t.Run("end current session", func(t *testing.T) {
		// sessionID2 is current session
		err := service.EndConversation(ctx, sessionID2)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Current session should be cleared
		currentSession, err := service.GetCurrentSession()
		if err != nil {
			t.Errorf("failed to get current session: %v", err)
		}
		if currentSession != "" {
			t.Errorf("expected empty current session but got '%s'", currentSession)
		}
	})
}

func TestProcessingState(t *testing.T) {
	service, err := NewConversationService(&mockAIProvider{}, &mockToolExecutor{})
	if err != nil {
		t.Fatalf("Failed to create service: %v", err)
	}

	ctx := context.Background()
	sessionID, _ := service.StartConversation(ctx)

	t.Run("initial processing state", func(t *testing.T) {
		processing, err := service.IsProcessing(sessionID)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		if processing {
			t.Errorf("expected initial processing state to be false")
		}
	})

	t.Run("set processing state", func(t *testing.T) {
		err := service.SetProcessingState(sessionID, true)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		processing, _ := service.IsProcessing(sessionID)
		if !processing {
			t.Errorf("expected processing state to be true")
		}

		// Set to false
		err = service.SetProcessingState(sessionID, false)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		processing, _ = service.IsProcessing(sessionID)
		if processing {
			t.Errorf("expected processing state to be false")
		}
	})

	t.Run("processing state for invalid session", func(t *testing.T) {
		_, err := service.IsProcessing("invalid-session")
		if err == nil {
			t.Errorf("expected error for invalid session")
		}

		err = service.SetProcessingState("invalid-session", true)
		if err == nil {
			t.Errorf("expected error for invalid session")
		}
	})
}

// --- Mock Implementations for Testing ---

type mockAIProvider struct {
	response *entity.Message
	err      error
	model    string
}

func (m *mockAIProvider) SendMessage(
	ctx context.Context,
	messages []port.MessageParam,
	tools []port.ToolParam,
) (*entity.Message, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.response == nil {
		return &entity.Message{
			Role:    entity.RoleAssistant,
			Content: "Mock response",
		}, nil
	}
	return m.response, nil
}

func (m *mockAIProvider) GenerateToolSchema() port.ToolInputSchemaParam {
	return port.ToolInputSchemaParam{
		"type": "object",
	}
}

func (m *mockAIProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockAIProvider) SetModel(model string) error {
	m.model = model
	return nil
}

func (m *mockAIProvider) GetModel() string {
	return m.model
}

type mockToolExecutor struct {
	tools   map[string]entity.Tool
	results map[string]string
	err     error
}

func (m *mockToolExecutor) RegisterTool(tool entity.Tool) error {
	if m.tools == nil {
		m.tools = make(map[string]entity.Tool)
	}
	m.tools[tool.Name] = tool
	return nil
}

func (m *mockToolExecutor) UnregisterTool(name string) error {
	if m.tools != nil {
		delete(m.tools, name)
	}
	return nil
}

func (m *mockToolExecutor) ExecuteTool(ctx context.Context, name string, input interface{}) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	if m.results != nil {
		if result, exists := m.results[name]; exists {
			return result, nil
		}
	}
	return "mock result", nil
}

func (m *mockToolExecutor) ListTools() ([]entity.Tool, error) {
	if m.tools == nil {
		return []entity.Tool{}, nil
	}
	tools := make([]entity.Tool, 0, len(m.tools))
	for _, tool := range m.tools {
		tools = append(tools, tool)
	}
	return tools, nil
}

func (m *mockToolExecutor) GetTool(name string) (entity.Tool, bool) {
	if m.tools == nil {
		m.tools = make(map[string]entity.Tool)
	}

	// Check if tool exists in tools map
	tool, exists := m.tools[name]
	if exists {
		return tool, true
	}

	// For testing: Create mock tools for common test scenarios
	// If there are results for this tool, create a mock tool
	if m.results != nil {
		if _, hasResult := m.results[name]; hasResult {
			mockTool := entity.Tool{
				ID:          name,
				Name:        name,
				Description: "Mock tool for testing",
			}
			return mockTool, true
		}
	}

	// If there's an error set, also create a mock tool for error testing scenarios
	if m.err != nil {
		// Only create tools for specific test cases to avoid false positives
		if name == "list_files" || name == "read_file" {
			mockTool := entity.Tool{
				ID:          name,
				Name:        name,
				Description: "Mock tool for testing",
			}
			return mockTool, true
		}
	}

	return entity.Tool{}, false
}

func (m *mockToolExecutor) ValidateToolInput(name string, input interface{}) error {
	return nil
}

// Helper methods for ConversationService

// generateSessionID generates a unique session ID using crypto/rand.
func generateSessionID() string {
	bytes := make([]byte, 16)
	_, _ = rand.Read(bytes) // Ignore error for test implementation
	return hex.EncodeToString(bytes)
}

// ToolRequest represents a parsed tool request from AI response.
type ToolRequest struct {
	Name  string      `json:"name"`
	Input interface{} `json:"input"`
}

// containsToolUse checks if the content contains tool usage requests.
func (cs *ConversationService) containsToolUse(content string) bool {
	return strings.Contains(content, `"type": "tool_use"`) || strings.Contains(content, `"tool_use"`)
}

// parseToolRequests parses tool requests from AI response content.
func (cs *ConversationService) parseToolRequests(content string) []ToolRequest {
	var requests []ToolRequest

	// Try parsing as single object
	var singleRequest ToolRequest
	if err := json.Unmarshal([]byte(content), &singleRequest); err == nil {
		if singleRequest.Name != "" {
			requests = append(requests, singleRequest)
		}
	}

	// Try parsing as array
	var arrayRequests []ToolRequest
	if err := json.Unmarshal([]byte(content), &arrayRequests); err == nil {
		requests = append(requests, arrayRequests...)
	}

	return requests
}

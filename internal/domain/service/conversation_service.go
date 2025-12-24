package service

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
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

// AddToolResultMessage adds tool execution results to the conversation.
func (cs *ConversationService) AddToolResultMessage(
	ctx context.Context,
	sessionID string,
	toolResults []entity.ToolResult,
) error {
	select {
	case <-ctx.Done():
		return context.Canceled
	default:
	}

	conversation, exists := cs.conversations[sessionID]
	if !exists {
		return ErrConversationNotFound
	}

	message, err := entity.NewToolResultMessage(entity.RoleUser, toolResults)
	if err != nil {
		return err
	}

	return conversation.AddMessage(*message)
}

// ProcessAssistantResponse processes an AI assistant response, handling tools and text.
func (cs *ConversationService) ProcessAssistantResponse(
	ctx context.Context,
	sessionID string,
) (*entity.Message, []port.ToolCallInfo, error) {
	select {
	case <-ctx.Done():
		return nil, nil, context.Canceled
	default:
	}

	conversation, exists := cs.conversations[sessionID]
	if !exists {
		return nil, nil, ErrConversationNotFound
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
		return nil, nil, err
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
	response, toolCalls, err := cs.aiProvider.SendMessage(ctx, messageParams, toolParams)
	if err != nil {
		return nil, nil, err
	}

	// Add response to conversation
	err = conversation.AddMessage(*response)
	if err != nil {
		return nil, nil, err
	}

	// Check if response contains tool usage
	if len(toolCalls) > 0 {
		cs.processing[sessionID] = true
	} else {
		cs.processing[sessionID] = false
	}

	return response, toolCalls, nil
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

package service

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

var (
	ErrConversationNotFound = errors.New("conversation not found")
	ErrToolNotFound         = errors.New("tool not found")
)

// ConversationService handles the core business logic for managing conversations.
// It orchestrates the flow of messages between users and AI, processes tool executions,
// maintains conversation state, and coordinates with the AI provider.
type ConversationService struct {
	aiProvider             port.AIProvider
	toolExecutor           port.ToolExecutor
	conversations          map[string]*entity.Conversation
	currentSession         string
	processing             map[string]bool
	sessionModes           map[string]bool
	sessionModesMu         sync.RWMutex // Protects sessionModes map for concurrent access
	sessionSystemPrompts   map[string]string
	sessionSystemPromptsMu sync.RWMutex // Protects sessionSystemPrompts map for concurrent access
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
		aiProvider:           aiProvider,
		toolExecutor:         toolExecutor,
		conversations:        make(map[string]*entity.Conversation),
		processing:           make(map[string]bool),
		sessionModes:         make(map[string]bool),
		sessionSystemPrompts: make(map[string]string),
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
		return nil, nil, fmt.Errorf("context cancelled before AI call: %w", ctx.Err())
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
		// Convert ToolCalls from entity to port
		var toolCallParams []port.ToolCallParam
		if len(msg.ToolCalls) > 0 {
			toolCallParams = make([]port.ToolCallParam, len(msg.ToolCalls))
			for j, tc := range msg.ToolCalls {
				toolCallParams[j] = port.ToolCallParam{
					ToolID:   tc.ToolID,
					ToolName: tc.ToolName,
					Input:    tc.Input,
				}
			}
		}

		// Convert ToolResults from entity to port
		var toolResultParams []port.ToolResultParam
		if len(msg.ToolResults) > 0 {
			toolResultParams = make([]port.ToolResultParam, len(msg.ToolResults))
			for j, tr := range msg.ToolResults {
				toolResultParams[j] = port.ToolResultParam{
					ToolID:  tr.ToolID,
					Result:  tr.Result,
					IsError: tr.IsError,
				}
			}
		}

		messageParams[i] = port.MessageParam{
			Role:        msg.Role,
			Content:     msg.Content,
			ToolCalls:   toolCallParams,
			ToolResults: toolResultParams,
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

	// Add plan mode info to context if enabled
	isPlanMode, _ := cs.IsPlanMode(sessionID)
	if isPlanMode {
		planInfo := port.PlanModeInfo{
			Enabled:   true,
			SessionID: sessionID,
			PlanPath:  fmt.Sprintf(".agent/plans/%s.md", sessionID),
		}
		ctx = port.WithPlanMode(ctx, planInfo)
	}

	// Add custom system prompt to context if set
	if customPrompt, ok := cs.GetCustomSystemPrompt(sessionID); ok {
		ctx = port.WithCustomSystemPrompt(ctx, port.CustomSystemPromptInfo{
			Prompt:    customPrompt,
			SessionID: sessionID,
		})
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

		// Add sessionID to context so PlanningExecutorAdapter can check plan mode
		ctxWithSession := port.WithSessionID(ctx, sessionID)
		result, err := cs.toolExecutor.ExecuteTool(ctxWithSession, request.Name, request.Input)
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
// It removes session-specific state including processing flags and mode settings.
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

	// Remove mode state
	cs.sessionModesMu.Lock()
	delete(cs.sessionModes, sessionID)
	cs.sessionModesMu.Unlock()

	// Remove custom system prompt
	cs.sessionSystemPromptsMu.Lock()
	delete(cs.sessionSystemPrompts, sessionID)
	cs.sessionSystemPromptsMu.Unlock()

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

// SetPlanMode sets the plan mode state for a session.
// When plan mode is enabled, tool executions are written to plan files instead of being executed.
// The operation is thread-safe.
func (cs *ConversationService) SetPlanMode(sessionID string, enabled bool) error {
	_, exists := cs.conversations[sessionID]
	if !exists {
		return ErrConversationNotFound
	}
	cs.sessionModesMu.Lock()
	cs.sessionModes[sessionID] = enabled
	cs.sessionModesMu.Unlock()
	return nil
}

// IsPlanMode returns whether plan mode is enabled for a session.
// Returns false for non-existent sessions.
// The operation is thread-safe for concurrent reads.
func (cs *ConversationService) IsPlanMode(sessionID string) (bool, error) {
	_, exists := cs.conversations[sessionID]
	if !exists {
		return false, ErrConversationNotFound
	}
	cs.sessionModesMu.RLock()
	defer cs.sessionModesMu.RUnlock()
	return cs.sessionModes[sessionID], nil
}

// SetCustomSystemPrompt sets a custom system prompt for a session.
// This allows overriding the default AI system prompt with session-specific instructions.
// The custom prompt is included in the context when calling the AI provider.
// The operation is thread-safe.
func (cs *ConversationService) SetCustomSystemPrompt(sessionID, prompt string) error {
	_, exists := cs.conversations[sessionID]
	if !exists {
		return ErrConversationNotFound
	}
	cs.sessionSystemPromptsMu.Lock()
	cs.sessionSystemPrompts[sessionID] = prompt
	cs.sessionSystemPromptsMu.Unlock()
	return nil
}

// GetCustomSystemPrompt retrieves the custom system prompt for a session.
// Returns the prompt and true if set, or empty string and false if not set.
// Returns false for non-existent sessions (graceful handling).
// The operation is thread-safe for concurrent reads.
func (cs *ConversationService) GetCustomSystemPrompt(sessionID string) (string, bool) {
	cs.sessionSystemPromptsMu.RLock()
	defer cs.sessionSystemPromptsMu.RUnlock()
	prompt, ok := cs.sessionSystemPrompts[sessionID]
	return prompt, ok
}

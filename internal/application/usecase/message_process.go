// Package usecase provides use case implementations for the application layer.
// These use cases orchestrate the flow of data between domain services and
// infrastructure adapters, following hexagonal architecture principles.
package usecase

import (
	"code-editing-agent/internal/application/dto"
	"code-editing-agent/internal/domain/port"
	"code-editing-agent/internal/domain/service"
	"context"
	"errors"
	"fmt"
	"time"
)

var (
	// ErrConversationServiceRequired is returned when ConversationService is nil.
	ErrConversationServiceRequired = errors.New("conversation service is required")

	// ErrSessionNotFound is returned when a session is not found.
	ErrSessionNotFound = errors.New("session not found")
)

// MessageProcessUseCase handles the processing of messages through the chat system.
// It orchestrates the flow from user input through AI processing to response generation,
// handling any tool execution requests along the way.
//
// This use case coordinates between the domain ConversationService and various ports
// (AIProvider, ToolExecutor) to provide a clean abstraction over the chat flow.
type MessageProcessUseCase struct {
	conversationService *service.ConversationService
	userInterface       port.UserInterface
}

// NewMessageProcessUseCase creates a new MessageProcessUseCase.
//
// Parameters:
//   - convService: The domain conversation service for managing conversations
//   - ui: The user interface port for displaying messages and getting input
//
// Returns:
//   - *MessageProcessUseCase: A new use case instance
//   - error: An error if the conversation service is nil
func NewMessageProcessUseCase(
	convService *service.ConversationService,
	ui port.UserInterface,
) (*MessageProcessUseCase, error) {
	if convService == nil {
		return nil, ErrConversationServiceRequired
	}

	return &MessageProcessUseCase{
		conversationService: convService,
		userInterface:       ui,
	}, nil
}

// ProcessUserMessage processes a user message through the chat system.
// This is the main entry point for handling user input and getting AI responses.
//
// The flow is:
// 1. Validate the request
// 2. Add the user message to the conversation
// 3. Get the AI's response
// 4. Handle any tool use requests
// 5. Display the response
// 6. Return the response with metadata
//
// Parameters:
//   - ctx: Context for the operation
//   - req: The send message request containing session ID and message content
//
// Returns:
//   - *dto.SendMessageResponse: The AI's response with metadata
//   - error: An error if processing fails
func (uc *MessageProcessUseCase) ProcessUserMessage(
	ctx context.Context,
	req dto.SendMessageRequest,
) (*dto.SendMessageResponse, error) {
	// Validate the request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// Add user message to conversation
	_, err := uc.conversationService.AddUserMessage(ctx, req.SessionID, req.Message)
	if err != nil {
		return nil, fmt.Errorf("failed to add user message: %w", err)
	}

	// Get conversation for state info
	conv, err := uc.conversationService.GetConversation(req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	// Process the assistant response
	assistantMsg, toolCalls, err := uc.processAssistantMessage(ctx, req.SessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to process assistant message: %w", err)
	}

	// Check if processing (has tools)
	isProcessing, _ := uc.conversationService.IsProcessing(req.SessionID)

	return &dto.SendMessageResponse{
		SessionID:    req.SessionID,
		AssistantMsg: assistantMsg,
		HasTools:     isProcessing,
		ToolCalls:    toolCalls,
		IsFinished:   !isProcessing,
		MessageCount: conv.MessageCount(),
	}, nil
}

// ContinueChat continues a chat session without new user input.
// This is used after tool execution to get the AI's final response.
//
// Parameters:
//   - ctx: Context for the operation
//   - sessionID: The session ID to continue
//
// Returns:
//   - *dto.ContinueChatResponse: The AI's response
//   - error: An error if continuation fails
func (uc *MessageProcessUseCase) ContinueChat(
	ctx context.Context,
	sessionID string,
) (*dto.ContinueChatResponse, error) {
	if sessionID == "" {
		return nil, dto.ErrEmptySessionID
	}

	// Validate session exists
	_, err := uc.conversationService.GetConversation(sessionID)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}

	// Process the assistant response
	assistantMsg, _, err := uc.processAssistantMessage(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to process assistant message: %w", err)
	}

	// Check if processing (has tools)
	isProcessing, _ := uc.conversationService.IsProcessing(sessionID)

	return &dto.ContinueChatResponse{
		SessionID:    sessionID,
		AssistantMsg: assistantMsg,
		HasTools:     isProcessing,
		IsFinished:   !isProcessing,
	}, nil
}

// processAssistantMessage handles the core message processing flow.
// It calls the AI provider, parses the response, and extracts tool use requests.
//
// Parameters:
//   - ctx: Context for the operation
//   - sessionID: The session ID to process
//
// Returns:
//   - *dto.AssistantMessage: The parsed assistant message
//   - []dto.ToolCallInfo: List of tool calls requested by the AI
//   - error: An error if processing fails
func (uc *MessageProcessUseCase) processAssistantMessage(
	ctx context.Context,
	sessionID string,
) (*dto.AssistantMessage, []dto.ToolCallInfo, error) {
	// Get AI response via domain service
	response, err := uc.conversationService.ProcessAssistantResponse(ctx, sessionID)
	if err != nil {
		return nil, nil, fmt.Errorf("AI provider error: %w", err)
	}

	// Convert to DTO
	assistantMsg := dto.NewAssistantMessageFromEntity(response)

	// Extract tool use requests from response
	toolCalls := uc.extractToolCalls(response.Content)

	return assistantMsg, toolCalls, nil
}

// extractToolCalls parses the assistant message content to find tool use requests.
// It looks for tool_use blocks in the content and extracts tool information.
//
// Parameters:
//   - content: The assistant message content
//
// Returns:
//   - []dto.ToolCallInfo: List of tool calls found in the content
func (uc *MessageProcessUseCase) extractToolCalls(content string) []dto.ToolCallInfo {
	// Check if content contains tool use indicators
	if !containsToolUse(content) {
		return nil
	}

	// Parse tool requests from the content
	// This uses the domain service's parsing logic
	// We'll look for patterns like tool_use{tool_name} or JSON tool specifications
	return parseToolCallInfo(content)
}

// StartNewSession starts a new chat session.
//
// Parameters:
//   - ctx: Context for the operation
//   - req: The start chat request (optional initial message)
//
// Returns:
//   - *dto.StartChatResponse: The new session information
//   - error: An error if session creation fails
func (uc *MessageProcessUseCase) StartNewSession(
	ctx context.Context,
	req dto.StartChatRequest,
) (*dto.StartChatResponse, error) {
	// Start a new conversation in the domain service
	sessionID, err := uc.conversationService.StartConversation(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start conversation: %w", err)
	}

	// Get the conversation for timing
	conv, err := uc.conversationService.GetConversation(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	response := &dto.StartChatResponse{
		SessionID: sessionID,
		StartedAt: conv.StartedAt,
	}

	// Add initial message if provided
	if req.InitialMessage != "" {
		_, err := uc.conversationService.AddUserMessage(ctx, sessionID, req.InitialMessage)
		if err != nil {
			return nil, fmt.Errorf("failed to add initial message: %w", err)
		}
	}

	return response, nil
}

// EndSession ends a chat session.
//
// Parameters:
//   - ctx: Context for the operation
//   - sessionID: The session ID to end
//
// Returns:
//   - *dto.EndChatResponse: Session termination information
//   - error: An error if session termination fails
func (uc *MessageProcessUseCase) EndSession(
	ctx context.Context,
	sessionID string,
) (*dto.EndChatResponse, error) {
	if sessionID == "" {
		return nil, dto.ErrEmptySessionID
	}

	// Get conversation before ending
	conv, err := uc.conversationService.GetConversation(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	// End the session
	err = uc.conversationService.EndConversation(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to end conversation: %w", err)
	}

	now := time.Now()
	return &dto.EndChatResponse{
		SessionID:    sessionID,
		EndedAt:      now,
		MessageCount: conv.MessageCount(),
		DurationSecs: now.Sub(conv.StartedAt).Seconds(),
	}, nil
}

// GetConversationState retrieves the current state of a conversation.
//
// Parameters:
//   - sessionID: The session ID to query
//
// Returns:
//   - *dto.ConversationState: The current conversation state
//   - error: An error if state retrieval fails
func (uc *MessageProcessUseCase) GetConversationState(
	sessionID string,
) (*dto.ConversationState, error) {
	if sessionID == "" {
		return nil, dto.ErrEmptySessionID
	}

	conv, err := uc.conversationService.GetConversation(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get conversation: %w", err)
	}

	isProcessing, err := uc.conversationService.IsProcessing(sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get processing state: %w", err)
	}

	return dto.NewConversationState(sessionID, conv, isProcessing), nil
}

// Helper functions

// containsToolUse checks if the content contains tool use indicators.
func containsToolUse(content string) bool {
	return containsSubstrings(content,
		`"type": "tool_use"`,
		`"tool_use"`,
		`tool_use{`,
	)
}

// containsSubstrings checks if any of the given substrings are in the content.
func containsSubstrings(content string, substrings ...string) bool {
	for _, sub := range substrings {
		if len(content) >= len(sub) {
			for i := 0; i <= len(content)-len(sub); i++ {
				if content[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}

// parseToolCallInfo parses tool call information from content.
// This is a simplified implementation that looks for tool_use{tool_name} patterns.
func parseToolCallInfo(content string) []dto.ToolCallInfo {
	var toolCalls []dto.ToolCallInfo

	// Look for tool_use{tool_name} pattern
	// This is a simplified parser - in production, you'd use proper JSON parsing
	toolPattern := "tool_use{"
	start := 0

	for {
		idx := findSubstring(content, toolPattern, start)
		if idx == -1 {
			break
		}

		// Extract tool name
		toolNameStart := idx + len(toolPattern)
		toolNameEnd := findChar(content, '}', toolNameStart)
		if toolNameEnd != -1 {
			toolName := content[toolNameStart:toolNameEnd]
			toolCalls = append(toolCalls, dto.ToolCallInfo{
				ToolID:       toolName,
				ToolName:     toolName,
				Input:        nil,
				InputJSON:    "",
				CallPriority: len(toolCalls),
			})
		}

		start = idx + 1
	}

	return toolCalls
}

// findSubstring finds the first occurrence of a substring after a start position.
func findSubstring(s, sub string, start int) int {
	if len(s) < len(sub)+start {
		return -1
	}
	for i := start; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// findChar finds the first occurrence of a character after a start position.
func findChar(s string, c byte, start int) int {
	if len(s) <= start {
		return -1
	}
	for i := start; i < len(s); i++ {
		if s[i] == c {
			return i
		}
	}
	return -1
}

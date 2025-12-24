// Package dto provides Data Transfer Objects for the application layer.
package dto

import (
	"code-editing-agent/internal/domain/entity"
	"time"
)

// SendMessageResponse represents the response after sending a user message.
// It contains the AI's response and information about the conversation state.
type SendMessageResponse struct {
	SessionID    string            `json:"session_id"`    // The conversation session ID
	AssistantMsg *AssistantMessage `json:"assistant_msg"` // The AI's assistant message
	HasTools     bool              `json:"has_tools"`     // Whether the response contains tool calls
	ToolCalls    []ToolCallInfo    `json:"tool_calls"`    // Information about tools that were requested
	IsFinished   bool              `json:"is_finished"`   // Whether the AI's response is complete
	MessageCount int               `json:"message_count"` // Total messages in the conversation
}

// AssistantMessage contains details about the AI's response.
type AssistantMessage struct {
	ID        string    `json:"id"`        // Unique message identifier
	Content   string    `json:"content"`   // The message content (text and/or tool info)
	Role      string    `json:"role"`      // The message role (always "assistant")
	Timestamp time.Time `json:"timestamp"` // When the message was created
}

// ToolCallInfo contains information about a tool that was requested by the AI.
type ToolCallInfo struct {
	ToolID       string      `json:"tool_id"`       // The tool identifier
	ToolName     string      `json:"tool_name"`     // The human-readable tool name
	Input        interface{} `json:"input"`         // The input parameters passed to the tool
	InputJSON    string      `json:"input_json"`    // JSON representation of the input
	CallPriority int         `json:"call_priority"` // Order of execution (0-indexed)
}

// StartChatResponse represents the response when starting a new chat session.
type StartChatResponse struct {
	SessionID  string            `json:"session_id"`            // The new session identifier
	StartedAt  time.Time         `json:"started_at"`            // When the session was created
	WelcomeMsg *AssistantMessage `json:"welcome_msg,omitempty"` // Optional welcome message
}

// EndChatResponse represents the response when ending a chat session.
type EndChatResponse struct {
	SessionID    string    `json:"session_id"`    // The session that was ended
	EndedAt      time.Time `json:"ended_at"`      // When the session was ended
	MessageCount int       `json:"message_count"` // Total messages in the session
	DurationSecs float64   `json:"duration_secs"` // Duration of the session in seconds
}

// ToolExecutionResponse represents the result of executing a tool.
type ToolExecutionResponse struct {
	SessionID  string    `json:"session_id"`  // The conversation session ID
	ToolName   string    `json:"tool_name"`   // Name of the tool that was executed
	Success    bool      `json:"success"`     // Whether the tool execution succeeded
	Result     string    `json:"result"`      // The tool's result (if successful)
	Error      string    `json:"error"`       // Error message (if failed)
	ExecutedAt time.Time `json:"executed_at"` // When the tool was executed
	DurationMs int64     `json:"duration_ms"` // Execution time in milliseconds
}

// ToolExecutionBatchResponse represents the result of executing multiple tools.
type ToolExecutionBatchResponse struct {
	SessionID       string                  `json:"session_id"`        // The conversation session ID
	Results         []ToolExecutionResponse `json:"results"`           // Individual tool execution results
	TotalTools      int                     `json:"total_tools"`       // Total number of tools executed
	SuccessfulCount int                     `json:"successful_count"`  // Number of successful executions
	FailedCount     int                     `json:"failed_count"`      // Number of failed executions
	TotalDurationMs int64                   `json:"total_duration_ms"` // Total execution time in milliseconds
}

// ConversationState represents the current state of a conversation.
type ConversationState struct {
	SessionID      string    `json:"session_id"`       // The session identifier
	MessageCount   int       `json:"message_count"`    // Number of messages in the conversation
	IsProcessing   bool      `json:"is_processing"`    // Whether currently processing (waiting for tools)
	StartedAt      time.Time `json:"started_at"`       // When the conversation started
	LastActivityAt time.Time `json:"last_activity_at"` // Last time a message was added
	DurationSecs   float64   `json:"duration_secs"`    // Duration of the conversation in seconds
}

// ContinueChatResponse represents the response when continuing a chat session
// (e.g., after tool execution without new user input).
type ContinueChatResponse struct {
	SessionID    string            `json:"session_id"`    // The conversation session ID
	AssistantMsg *AssistantMessage `json:"assistant_msg"` // The AI's response
	HasTools     bool              `json:"has_tools"`     // Whether the response contains tool calls
	ToolCalls    []ToolCallInfo    `json:"tool_calls"`    // Information about tools that were requested
	IsFinished   bool              `json:"is_finished"`   // Whether the response is complete
}

// ChatErrorResponse represents a standardized error response for chat operations.
type ChatErrorResponse struct {
	SessionID string `json:"session_id"` // The session ID (if applicable)

	Message string `json:"message"` // Human-readable error message
	Code    string `json:"code"`    // Error code for programmatic handling
	Details string `json:"details"` // Additional error details
}

// NewAssistantMessageFromEntity creates an AssistantMessage from a domain Message entity.
func NewAssistantMessageFromEntity(msg *entity.Message) *AssistantMessage {
	if msg == nil {
		return nil
	}
	return &AssistantMessage{
		ID:        generateMessageID(),
		Content:   msg.Content,
		Role:      msg.Role,
		Timestamp: msg.Timestamp,
	}
}

// NewChatErrorResponse creates a ChatErrorResponse from an error.
func NewChatErrorResponse(err error) *ChatErrorResponse {
	if err == nil {
		return nil
	}
	return &ChatErrorResponse{
		Message: err.Error(),
		Code:    getErrorCode(err),
	}
}

// NewToolExecutionResponse creates a ToolExecutionResponse from execution results.
func NewToolExecutionResponse(
	sessionID, toolName string,
	result string,
	execErr error,
	duration time.Duration,
) *ToolExecutionResponse {
	resp := &ToolExecutionResponse{
		SessionID:  sessionID,
		ToolName:   toolName,
		ExecutedAt: time.Now(),
		DurationMs: duration.Milliseconds(),
	}
	if execErr != nil {
		resp.Success = false
		resp.Error = execErr.Error()
	} else {
		resp.Success = true
		resp.Result = result
	}
	return resp
}

// NewConversationState creates a ConversationState from a domain Conversation entity.
func NewConversationState(sessionID string, conv *entity.Conversation, isProcessing bool) *ConversationState {
	now := time.Now()
	state := &ConversationState{
		SessionID:      sessionID,
		MessageCount:   conv.MessageCount(),
		IsProcessing:   isProcessing,
		StartedAt:      conv.StartedAt,
		LastActivityAt: now,
		DurationSecs:   now.Sub(conv.StartedAt).Seconds(),
	}
	return state
}

// Helper functions

// generateMessageID generates a unique message identifier.
func generateMessageID() string {
	return time.Now().Format("20060102150405.000000000") + "_" + string(rune(time.Now().UnixNano()))
}

// getErrorCode extracts a machine-readable error code from an error.
func getErrorCode(err error) string {
	if err == nil {
		return "UNKNOWN"
	}
	switch {
	case isValidationError(err):
		return "VALIDATION_ERROR"
	case isContextError(err):
		return "CONTEXT_ERROR"
	default:
		return "INTERNAL_ERROR"
	}
}

// isValidationError checks if an error is a validation error.
func isValidationError(err error) bool {
	if err == nil {
		return false
	}
	// Check if error message starts with "validation error:"
	errMsg := err.Error()
	if len(errMsg) > 16 && errMsg[:16] == "validation error" {
		return true
	}
	return false
}

// isContextError checks if an error is a context error.
func isContextError(err error) bool {
	// Check if the error unwraps to context.Canceled or context.DeadlineExceeded
	if err == nil {
		return false
	}
	// This is a simplified check - in production you'd be more thorough
	return err.Error() == "context canceled" || err.Error() == "context deadline exceeded"
}

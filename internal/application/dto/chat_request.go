// Package dto provides Data Transfer Objects for the application layer.
// These objects facilitate data transfer between layers while maintaining
// clean separation of concerns in the hexagonal architecture.
package dto

// SendMessageRequest represents a request to send a user message.
// It contains the session ID for tracking the conversation and the
// user's input message content.
type SendMessageRequest struct {
	SessionID string `json:"session_id"` // Unique identifier for the conversation session
	Message   string `json:"message"`    // The user's message content
}

// Validate checks if the SendMessageRequest is valid.
// Returns an error if session ID or message is empty.
func (r *SendMessageRequest) Validate() error {
	if r.SessionID == "" {
		return ErrEmptySessionID
	}
	if r.Message == "" {
		return ErrEmptyMessage
	}
	return nil
}

// ContinueChatRequest represents a request to continue processing a chat
// without new user input (typically after tool execution).
type ContinueChatRequest struct {
	SessionID string `json:"session_id"` // Unique identifier for the conversation session
}

// Validate checks if the ContinueChatRequest is valid.
// Returns an error if session ID is empty.
func (r *ContinueChatRequest) Validate() error {
	if r.SessionID == "" {
		return ErrEmptySessionID
	}
	return nil
}

// StartChatRequest represents a request to start a new chat session.
// It can optionally include an initial system message or context.
type StartChatRequest struct {
	InitialMessage string `json:"initial_message,omitempty"` // Optional initial message from the user
}

// Validate checks if the StartChatRequest is valid.
// This request is always valid since all fields are optional.
func (r *StartChatRequest) Validate() error {
	return nil
}

// EndChatRequest represents a request to end a chat session.
type EndChatRequest struct {
	SessionID string `json:"session_id"` // Unique identifier for the conversation session to end
}

// Validate checks if the EndChatRequest is valid.
// Returns an error if session ID is empty.
func (r *EndChatRequest) Validate() error {
	if r.SessionID == "" {
		return ErrEmptySessionID
	}
	return nil
}

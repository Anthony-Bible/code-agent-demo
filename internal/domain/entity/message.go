package entity

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	RoleUser      = "user"
	RoleAssistant = "assistant"
	RoleSystem    = "system"
)

var (
	ErrEmptyRole      = errors.New("role cannot be empty")
	ErrEmptyContent   = errors.New("content cannot be empty")
	ErrInvalidRole    = errors.New("invalid role")
	ErrZeroTimestamp  = errors.New("timestamp cannot be zero")
	ErrInvalidContent = errors.New("content cannot be whitespace only")
)

// Message represents a chat message with role, content, and timestamp.
// It is an immutable entity that represents a single message in a conversation.
type Message struct {
	Role      string    `json:"role"`      // The role of the message sender (user, assistant, or system)
	Content   string    `json:"content"`   // The actual content of the message
	Timestamp time.Time `json:"timestamp"` // When the message was created
}

// NewMessage creates a new message with the given role and content.
// The timestamp is automatically set to the current time.
func NewMessage(role, content string) (*Message, error) {
	if role == "" {
		return nil, ErrEmptyRole
	}
	if content == "" {
		return nil, ErrEmptyContent
	}
	if strings.TrimSpace(content) == "" {
		return nil, ErrInvalidContent
	}
	if role != RoleUser && role != RoleAssistant && role != RoleSystem {
		return nil, ErrInvalidRole
	}

	return &Message{
		Role:      role,
		Content:   content,
		Timestamp: time.Now(),
	}, nil
}

// IsUser returns true if the message is from a user.
func (m *Message) IsUser() bool {
	return m.Role == RoleUser
}

// IsAssistant returns true if the message is from an assistant.
func (m *Message) IsAssistant() bool {
	return m.Role == RoleAssistant
}

// IsSystem returns true if the message is a system message.
func (m *Message) IsSystem() bool {
	return m.Role == RoleSystem
}

// Validate checks if the message is valid.
// It returns an error if any required field is empty or invalid.
func (m *Message) Validate() error {
	if m.Role == "" {
		return ErrEmptyRole
	}
	if m.Content == "" {
		return ErrEmptyContent
	}
	if strings.TrimSpace(m.Content) == "" {
		return ErrInvalidContent
	}
	if m.Role != RoleUser && m.Role != RoleAssistant && m.Role != RoleSystem {
		return ErrInvalidRole
	}
	if m.Timestamp.IsZero() {
		return ErrZeroTimestamp
	}
	return nil
}

// UpdateContent updates the message content with the provided text.
// It returns an error if the new content is empty or contains only whitespace.
func (m *Message) UpdateContent(newContent string) error {
	if newContent == "" {
		return ErrEmptyContent
	}
	if strings.TrimSpace(newContent) == "" {
		return ErrInvalidContent
	}
	m.Content = newContent
	return nil
}

// GetAge returns the duration elapsed since the message was created.
func (m *Message) GetAge() time.Duration {
	return time.Since(m.Timestamp)
}

// IsValid checks if the message is valid without returning an error.
// Returns true if the message passes all validation checks.
func (m *Message) IsValid() bool {
	return m.Validate() == nil
}

// String returns a string representation of the message.
func (m *Message) String() string {
	return fmt.Sprintf("Message[%s]: %s", m.Role, m.Content)
}

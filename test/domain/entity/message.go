package entity

import (
	"errors"
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

type Message struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

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

func (m *Message) IsUser() bool {
	return m.Role == RoleUser
}

func (m *Message) IsAssistant() bool {
	return m.Role == RoleAssistant
}

func (m *Message) IsSystem() bool {
	return m.Role == RoleSystem
}

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

func (m *Message) GetAge() time.Duration {
	return time.Since(m.Timestamp)
}

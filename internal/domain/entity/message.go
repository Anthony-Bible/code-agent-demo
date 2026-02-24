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
	ErrEmptyRole       = errors.New("role cannot be empty")
	ErrEmptyContent    = errors.New("content cannot be empty")
	ErrInvalidRole     = errors.New("invalid role")
	ErrZeroTimestamp   = errors.New("timestamp cannot be zero")
	ErrInvalidContent  = errors.New("content cannot be whitespace only")
	ErrNoContentOrTool = errors.New("message must have either content or tool calls/results")
)

// ToolCall represents a tool use block in an assistant message.
type ToolCall struct {
	ToolID           string                 `json:"tool_id"`
	ToolName         string                 `json:"tool_name"`
	Input            map[string]interface{} `json:"input"`
	ThoughtSignature string                 `json:"thought_signature,omitempty"` // Gemini thought signature via Bifrost
}

// ToolResult represents a tool result block in a user message.
type ToolResult struct {
	ToolID           string `json:"tool_id"`
	Result           string `json:"result"`
	IsError          bool   `json:"is_error"`
	ThoughtSignature string `json:"thought_signature,omitempty"` // Gemini thought signature (via Bifrost)
}

// ThinkingBlock represents a thinking block in a message.
type ThinkingBlock struct {
	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

// Message represents a chat message with role, content, and timestamp.
// It is an immutable entity that represents a single message in a conversation.
type Message struct {
	Role           string          `json:"role"`                      // The role of the message sender (user, assistant, or system)
	Content        string          `json:"content"`                   // The actual content of the message
	Timestamp      time.Time       `json:"timestamp"`                 // When the message was created
	ToolCalls      []ToolCall      `json:"tool_calls,omitempty"`      // Tool calls from assistant messages
	ToolResults    []ToolResult    `json:"tool_results,omitempty"`    // Tool results from user messages
	ThinkingBlocks []ThinkingBlock `json:"thinking_blocks,omitempty"` // Thinking blocks
}

// validateRole checks if the provided role is valid.
// Returns an error if the role is empty or not one of the valid role constants.
func validateRole(role string) error {
	if role == "" {
		return ErrEmptyRole
	}
	if role != RoleUser && role != RoleAssistant && role != RoleSystem {
		return ErrInvalidRole
	}
	return nil
}

// NewMessage creates a new message with the given role and content.
// The timestamp is automatically set to the current time.
// Content can be empty if the message contains tool calls or results.
func NewMessage(role, content string) (*Message, error) {
	if err := validateRole(role); err != nil {
		return nil, err
	}
	// For backwards compatibility, if content is empty for a non-tool message, return error
	// The tool fields will be added separately after creation via a different method
	if content == "" {
		return nil, ErrEmptyContent
	}
	if strings.TrimSpace(content) == "" {
		return nil, ErrInvalidContent
	}

	return &Message{
		Role:        role,
		Content:     content,
		Timestamp:   time.Now(),
		ToolCalls:   nil,
		ToolResults: nil,
	}, nil
}

// NewToolCallMessage creates a new message with tool calls.
// Content can be empty since the message contains tool calls.
func NewToolCallMessage(role string, toolCalls []ToolCall) (*Message, error) {
	if err := validateRole(role); err != nil {
		return nil, err
	}
	if len(toolCalls) == 0 {
		return nil, ErrNoContentOrTool
	}

	return &Message{
		Role:        role,
		Content:     "",
		Timestamp:   time.Now(),
		ToolCalls:   toolCalls,
		ToolResults: nil,
	}, nil
}

// NewToolResultMessage creates a new message with tool results.
// Content can be empty since the message contains tool results.
func NewToolResultMessage(role string, toolResults []ToolResult) (*Message, error) {
	if err := validateRole(role); err != nil {
		return nil, err
	}
	if len(toolResults) == 0 {
		return nil, ErrNoContentOrTool
	}

	return &Message{
		Role:        role,
		Content:     "",
		Timestamp:   time.Now(),
		ToolCalls:   nil,
		ToolResults: toolResults,
	}, nil
}

// NewMessageWithThinkingBlocks creates a new message with thinking blocks.
// Content can be empty if thinking blocks are present.
func NewMessageWithThinkingBlocks(role, content string, thinkingBlocks []ThinkingBlock) (*Message, error) {
	if err := validateRole(role); err != nil {
		return nil, err
	}
	if content == "" && len(thinkingBlocks) == 0 {
		return nil, ErrEmptyContent
	}
	if content != "" && strings.TrimSpace(content) == "" {
		return nil, ErrInvalidContent
	}

	return &Message{
		Role:           role,
		Content:        content,
		Timestamp:      time.Now(),
		ToolCalls:      nil,
		ToolResults:    nil,
		ThinkingBlocks: thinkingBlocks,
	}, nil
}

// hasToolContent returns true if the message has either tool calls or tool results.
func (m *Message) hasToolContent() bool {
	return len(m.ToolCalls) > 0 || len(m.ToolResults) > 0 || len(m.ThinkingBlocks) > 0
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
// A message must have either content or tool calls/results.
func (m *Message) Validate() error {
	if m.Role == "" {
		return ErrEmptyRole
	}
	// Allow empty content if the message has tool calls or results
	if m.Content == "" && !m.hasToolContent() {
		return ErrEmptyContent
	}
	// Check for whitespace-only content (always invalid, even with tool content)
	if m.Content != "" && strings.TrimSpace(m.Content) == "" {
		return ErrInvalidContent
	}
	// Ensure at least one of content or tool content is present
	if m.Content == "" && strings.TrimSpace(m.Content) == "" && !m.hasToolContent() {
		return ErrNoContentOrTool
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

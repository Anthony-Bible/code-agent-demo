package entity

import (
	"time"
)

// Conversation represents a collection of messages in chronological order.
// It provides operations to manage the conversation state.
type Conversation struct {
	Messages  []Message `json:"messages"`
	StartedAt time.Time `json:"started_at"`
}

// NewConversation creates an empty conversation with the current timestamp.
func NewConversation() (*Conversation, error) {
	return &Conversation{
		Messages:  []Message{},
		StartedAt: time.Now(),
	}, nil
}

// AddMessage adds a validated message to the conversation.
// If the message timestamp is zero, it will be set to the current time.
func (c *Conversation) AddMessage(message Message) error {
	// Set timestamp if zero for struct-created messages
	if message.Timestamp.IsZero() {
		message.Timestamp = time.Now()
	}
	if err := message.Validate(); err != nil {
		return err
	}
	c.Messages = append(c.Messages, message)
	return nil
}

// GetMessages returns a defensive copy of all messages in the conversation.
// The returned slice can be safely modified without affecting the conversation.
func (c *Conversation) GetMessages() []Message {
	result := make([]Message, len(c.Messages))
	copy(result, c.Messages)
	return result
}

// GetLastMessage returns the last message in the conversation, if any.
// Returns a pointer to the message and true, or nil and false if empty.
func (c *Conversation) GetLastMessage() (*Message, bool) {
	if len(c.Messages) == 0 {
		return nil, false
	}
	last := c.Messages[len(c.Messages)-1]
	return &last, true
}

// Clear removes all messages from the conversation.
func (c *Conversation) Clear() {
	c.Messages = []Message{}
}

// MessageCount returns the number of messages in the conversation.
func (c *Conversation) MessageCount() int {
	return len(c.Messages)
}

// IsEmpty returns true if the conversation has no messages.
func (c *Conversation) IsEmpty() bool {
	return len(c.Messages) == 0
}

// HasMessages returns true if the conversation contains at least one message.
func (c *Conversation) HasMessages() bool {
	return len(c.Messages) > 0
}

// GetDuration returns the duration elapsed since the conversation started.
func (c *Conversation) GetDuration() time.Duration {
	return time.Since(c.StartedAt)
}

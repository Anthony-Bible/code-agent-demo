package entity

import (
	"time"
)

type Conversation struct {
	Messages  []Message `json:"messages"`
	StartedAt time.Time `json:"started_at"`
}

func NewConversation() (*Conversation, error) {
	return &Conversation{
		Messages:  []Message{},
		StartedAt: time.Now(),
	}, nil
}

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

func (c *Conversation) GetMessages() []Message {
	result := make([]Message, len(c.Messages))
	copy(result, c.Messages)
	return result
}

func (c *Conversation) GetLastMessage() (*Message, bool) {
	if len(c.Messages) == 0 {
		return nil, false
	}
	last := c.Messages[len(c.Messages)-1]
	return &last, true
}

func (c *Conversation) Clear() {
	c.Messages = []Message{}
}

func (c *Conversation) MessageCount() int {
	return len(c.Messages)
}

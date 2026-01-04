// Package entity provides core domain entities for the conversation system.
//
// This package contains the fundamental data structures that represent conversations,
// messages, and related concepts in the domain model. These entities are designed to be
// pure domain objects with minimal external dependencies, focusing on business logic
// and data integrity.
//
// Key entities include:
//   - Conversation: Manages a chronological collection of messages
//   - Message: Represents individual messages with roles and content
//   - Tool: Represents available tools and their metadata
//
// The package follows Domain-Driven Design principles with entities that contain
// business logic, validation, and ensure data consistency. All entities are
// designed to be thread-safe through proper encapsulation and defensive copying.
//
// Basic usage:
//
//     conv, err := entity.NewConversation()
//     if err != nil {
//         return fmt.Errorf("failed to create conversation: %w", err)
//     }
//
//     msg, err := entity.NewMessage("user", "Hello, world!")
//     if err != nil {
//         return fmt.Errorf("failed to create message: %w", err)
//     }
//
//     if err := conv.AddMessage(*msg); err != nil {
//         return fmt.Errorf("failed to add message: %w", err)
//     }
//
//     fmt.Printf("Conversation has %d messages\n", conv.MessageCount())
//
package entity

import (
	"time"
)

// Conversation represents a collection of messages in chronological order.
//
// A conversation serves as the primary container for managing the state and history
// of a chat interaction. It maintains messages in the order they were added and
// provides operations for message management, querying, and state inspection.
//
// The conversation encapsulates business logic for ensuring data integrity,
// including message validation, timestamp management, and safe access patterns.
// It follows defensive programming principles by returning copies of internal
// data structures to prevent external modification.
//
// Key features:
//   - Thread-safe message addition and retrieval
//   - Automatic timestamp management for messages
//   - Message validation before insertion
//   - Defensive copying to prevent internal state corruption
//   - Query methods for conversation state inspection
//
// The conversation is designed to be a long-lived object that can accumulate
// many messages over time. It tracks when the conversation started and can
// report the total duration of the conversation.
//
// Example usage:
//
//     // Create a new conversation
//     conv, err := NewConversation()
//     if err != nil {
//         return fmt.Errorf("failed to create conversation: %w", err)
//     }
//
//     // Create and add messages
//     userMsg, _ := NewMessage("user", "Hello!")
//     conv.AddMessage(*userMsg)
//
//     assistantMsg, _ := NewMessage("assistant", "Hi there! How can I help?")
//     conv.AddMessage(*assistantMsg)
//
//     // Query conversation state
//     if conv.HasMessages() {
//         fmt.Printf("Conversation started at: %v\n", conv.StartedAt)
//         fmt.Printf("Total messages: %d\n", conv.MessageCount())
//         fmt.Printf("Duration: %v\n", conv.GetDuration())
//     }
//
type Conversation struct {
	// Messages is the chronological collection of all messages in this conversation.
	// Messages are stored in the order they were added and maintain their original
	// Message objects with all metadata intact.
	Messages []Message `json:"messages"`
	
	// StartedAt marks when this conversation was first created.
	// It is automatically set during conversation creation and provides
	// a reference point for calculating conversation duration.
	StartedAt time.Time `json:"started_at"`
}

// NewConversation creates an empty conversation with the current timestamp.
//
// This factory function initializes a new conversation with no messages and
// sets the StartedAt field to the current time. The returned conversation
// is ready to receive messages and track conversation state.
//
// The function always returns a non-nil Conversation pointer and a nil error,
// making it safe to use without error checking in most scenarios. The error
// return is maintained for API consistency and potential future extensions.
//
// Returns:
//   - *Conversation: A new, empty conversation instance
//   - error: Always nil for now, maintained for future compatibility
//
// Example:
//
//     conv, err := NewConversation()
//     if err != nil {
//         return fmt.Errorf("failed to create conversation: %w", err)
//     }
//     fmt.Printf("New conversation started at: %v\n", conv.StartedAt)
//
func NewConversation() (*Conversation, error) {
	return &Conversation{
		Messages:  []Message{},
		StartedAt: time.Now(),
	}, nil
}

// AddMessage adds a validated message to the conversation.
//
// This method performs message validation before adding it to ensure data integrity.
// If the message has no timestamp (zero time), it will be automatically set to
// the current time. The message is always appended to maintain chronological order.
//
// Validation ensures that the message has valid role, content, and timestamp
// according to the Message entity's validation rules. If validation fails,
// the conversation remains unchanged and the validation error is returned.
//
// Parameters:
//   - message: The Message to add to the conversation
//
// Returns:
//   - error: Validation error if the message is invalid, nil on success
//
// Error conditions:
//   - Empty role
//   - Empty content (unless message has tool calls/results)
//   - Invalid role
//   - Invalid timestamp after auto-setting
//
// Example:
//
//     msg, err := NewMessage("user", "Hello, world!")
//     if err != nil {
//         return fmt.Errorf("failed to create message: %w", err)
//     }
//
//     if err := conv.AddMessage(*msg); err != nil {
//         return fmt.Errorf("failed to add message: %w", err)
//     }
//     fmt.Printf("Added message. Total: %d\n", conv.MessageCount())
//
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
//
// This method creates and returns a deep copy of the internal messages slice
// to prevent external modification of the conversation's internal state.
// Modifications to the returned slice will not affect the conversation.
//
// The returned messages maintain their original chronological order and
// contain all message data including content, role, timestamp, and any
// tool calls or results.
//
// Returns:
//   - []Message: A copy of all messages in the conversation
//
// Performance considerations:
//   - Creates a new slice and copies all message elements
//   - Complexity: O(n) where n is the number of messages
//   - Use MessageCount() to check message count before calling if needed
//
// Example:
//
//     messages := conv.GetMessages()
//     for i, msg := range messages {
//         fmt.Printf("%d: [%s] %s\n", i, msg.Role, msg.Content)
//     }
//     // Safe: modifying messages slice doesn't affect conversation
//     messages = append(messages, Message{Role: "user", Content: "temp"})
//
func (c *Conversation) GetMessages() []Message {
	result := make([]Message, len(c.Messages))
	copy(result, c.Messages)
	return result
}

// GetLastMessage returns the last message in the conversation, if any.
//
// This method provides efficient access to the most recent message without
// needing to retrieve all messages. It returns a copy of the last message
// to maintain defensive programming principles.
//
// The method uses the "comma ok" idiom to safely indicate whether a message
// exists. This allows callers to distinguish between an empty conversation
// and one with messages.
//
// Returns:
//   - *Message: A pointer to a copy of the last message, or nil if empty
//   - bool: true if a message exists, false if the conversation is empty
//
// Performance:
//   - Complexity: O(1) - constant time access
//   - No slice iteration required
//
// Example:
//
//     if lastMsg, ok := conv.GetLastMessage(); ok {
//         fmt.Printf("Last message from %s: %s\n", lastMsg.Role, lastMsg.Content)
//         fmt.Printf("Sent at: %v\n", lastMsg.Timestamp)
//     } else {
//         fmt.Println("No messages in conversation yet")
//     }
//
//     // Common pattern: checking for assistant response
//     if lastMsg, ok := conv.GetLastMessage(); ok && lastMsg.Role == RoleAssistant {
//         fmt.Println("Assistant has already responded")
//     }
//
func (c *Conversation) GetLastMessage() (*Message, bool) {
	if len(c.Messages) == 0 {
		return nil, false
	}
	last := c.Messages[len(c.Messages)-1]
	return &last, true
}

// Clear removes all messages from the conversation.
//
// This method resets the conversation to an empty state while preserving
// the StartedAt timestamp. This is useful for reusing a conversation object
// or implementing conversation reset functionality.
//
// After calling Clear(), the conversation will return true for IsEmpty()
// and MessageCount() will return 0, but StartedAt remains unchanged to
// maintain the original conversation start time.
//
// Note: This operation cannot be undone. Consider keeping a backup of
// messages if you need to restore them later.
//
// Example:
//
//     fmt.Printf("Before clear: %d messages\n", conv.MessageCount())
//     conv.Clear()
//     fmt.Printf("After clear: %d messages\n", conv.MessageCount())
//     fmt.Printf("Conversation originally started at: %v\n", conv.StartedAt)
//
//     // Common pattern: clearing conversation while preserving start time
//     if conv.MessageCount() > 100 {
//         fmt.Println("Conversation getting too long, clearing...")
//         conv.Clear()
//     }
//
func (c *Conversation) Clear() {
	c.Messages = []Message{}
}

// MessageCount returns the number of messages in the conversation.
//
// This method provides efficient access to the total message count without
// needing to retrieve all messages. It's useful for pagination, UI displays,
// and conversation state queries.
//
// Returns:
//   - int: The total number of messages (0 or greater)
//
// Example:
//
//     count := conv.MessageCount()
//     fmt.Printf("Conversation has %d messages\n", count)
//
//     // Common pattern: checking conversation length
//     if conv.MessageCount() > 10 {
//         fmt.Println("Long conversation detected")
//     }
//
//     // Pagination example
//     pageSize := 10
//     totalPages := (conv.MessageCount() + pageSize - 1) / pageSize
//     fmt.Printf("Total pages: %d\n", totalPages)
//
func (c *Conversation) MessageCount() int {
	return len(c.Messages)
}

// IsEmpty returns true if the conversation has no messages.
//
// This method provides a semantic way to check if a conversation is empty,
// which is more readable than checking MessageCount() == 0. It's the inverse
// of HasMessages().
//
// Returns:
//   - bool: true if the conversation has zero messages, false otherwise
//
// Example:
//
//     if conv.IsEmpty() {
//         fmt.Println("Starting a new conversation")
//         // Add welcome message
//     }
//
//     // More readable than:
//     // if conv.MessageCount() == 0 { ... }
//
func (c *Conversation) IsEmpty() bool {
	return len(c.Messages) == 0
}

// HasMessages returns true if the conversation contains at least one message.
//
// This method provides a semantic way to check if a conversation has any content.
// It's the inverse of IsEmpty() and is often more readable in contexts where
// you're checking for message existence.
//
// Returns:
//   - bool: true if the conversation has one or more messages, false otherwise
//
// Example:
//
//     if conv.HasMessages() {
//         fmt.Printf("Conversation started %v ago\n", conv.GetDuration())
//         lastMsg, _ := conv.GetLastMessage()
//         fmt.Printf("Last message: %s\n", lastMsg.Content)
//     }
//
//     // Common pattern: conditional processing
//     if !conv.HasMessages() {
//         return errors.New("cannot process empty conversation")
//     }
//
func (c *Conversation) HasMessages() bool {
	return len(c.Messages) > 0
}

// GetDuration returns the duration elapsed since the conversation started.
//
// This method calculates the time span from when the conversation was created
// (StartedAt) to the current time. It's useful for tracking conversation age,
// implementing timeouts, or displaying conversation metadata.
//
// The duration is calculated at call time, so repeated calls will return
// increasing values. The duration is based on wall-clock time and may be
// affected by system clock changes.
//
// Returns:
//   - time.Duration: The elapsed time since StartedAt
//
// Example:
//
//     duration := conv.GetDuration()
//     fmt.Printf("Conversation age: %v\n", duration)
//
//     // Common pattern: timeout checking
//     if conv.GetDuration() > 30*time.Minute {
//         fmt.Println("Conversation is stale, consider cleanup")
//     }
//
//     // Formatting examples
//     minutes := conv.GetDuration().Minutes()
//     fmt.Printf("Conversation duration: %.1f minutes\n", minutes)
//
//     // Session timeout example
//     maxSessionDuration := 2 * time.Hour
//     if conv.GetDuration() > maxSessionDuration {
//         return errors.New("session expired")
//     }
//


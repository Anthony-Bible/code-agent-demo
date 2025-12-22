package domain

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Message represents a message in the conversation
type Message interface {
	GetID() string
	GetRole() string
	GetContent() interface{}
	GetTimestamp() time.Time
	ToJSON() ([]byte, error)
}

// Tool represents a tool that can be used in the conversation
type Tool interface {
	GetName() string
	GetDescription() string
	GetInputSchema() map[string]interface{}
	Execute(input json.RawMessage) (string, error)
}

// Conversation manages conversation state
type Conversation interface {
	AddMessage(msg Message) error
	GetMessages() []Message
	GetLastMessage() (Message, bool)
	GetMessagesByRole(role string) []Message
	GetToolUseMessages() []Message
	GetToolResultMessages() []Message
	ClearMessages()
	GetID() string
	GetCreatedAt() time.Time
	GetUpdatedAt() time.Time
	SetTools(tools []Tool)
	GetTools() []Tool
	FindTool(name string) (Tool, bool)
	GetMessageCount() int
	GetMessageCountByRole(role string) int
}

// TestConversationCreation tests creating a new conversation
func TestConversationCreation(t *testing.T) {
	tests := []struct {
		name          string
		expectedID    string
		expectError   bool
		expectedError string
	}{
		{
			name:          "successful conversation creation",
			expectedID:    "test-conv-123",
			expectError:   false,
			expectedError: "",
		},
		{
			name:          "empty ID should fail",
			expectedID:    "",
			expectError:   true,
			expectedError: "conversation ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			conv, err := NewConversation(tt.expectedID)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
				assert.Nil(t, conv)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, conv)
				assert.Equal(t, tt.expectedID, conv.GetID())
				assert.Empty(t, conv.GetMessages())
				assert.Zero(t, conv.GetMessageCount())
				assert.True(t, conv.GetCreatedAt().Equal(conv.GetUpdatedAt()))
			}
		})
	}
}

// TestAddingMessages tests adding various types of messages to conversation
func TestAddingMessages(t *testing.T) {
	conv, err := NewConversation("test-conv")
	assert.NoError(t, err)

	now := time.Now()

	tests := []struct {
		name        string
		message     Message
		expectError bool
		errorMsg    string
	}{
		{
			name: "add user message",
			message: &UserMessage{
				ID:        "msg-1",
				Role:      "user",
				Content:   "Hello, world!",
				Timestamp: now,
			},
			expectError: false,
			errorMsg:    "",
		},
		{
			name: "add assistant message",
			message: &AssistantMessage{
				ID:        "msg-2",
				Role:      "assistant",
				Content:   "Hello! How can I help you?",
				Timestamp: now,
			},
			expectError: false,
			errorMsg:    "",
		},
		{
			name: "add tool use message",
			message: &ToolUseMessage{
				ID:        "msg-3",
				Role:      "assistant",
				ToolName:  "read_file",
				Input:     json.RawMessage(`{"path": "test.txt"}`),
				Timestamp: now,
			},
			expectError: false,
			errorMsg:    "",
		},
		{
			name: "add tool result message",
			message: &ToolResultMessage{
				ID:        "msg-4",
				Role:      "user",
				ToolUseID: "msg-3",
				Result:    "File content here",
				IsError:   false,
				Timestamp: now,
			},
			expectError: false,
			errorMsg:    "",
		},
		{
			name:        "nil message should fail",
			message:     nil,
			expectError: true,
			errorMsg:    "message cannot be nil",
		},
		{
			name: "empty message ID should fail",
			message: &UserMessage{
				ID:        "",
				Role:      "user",
				Content:   "test",
				Timestamp: now,
			},
			expectError: true,
			errorMsg:    "message ID cannot be empty",
		},
		{
			name: "invalid role should fail",
			message: &UserMessage{
				ID:        "msg-5",
				Role:      "invalid",
				Content:   "test",
				Timestamp: now,
			},
			expectError: true,
			errorMsg:    "invalid message role: invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := conv.AddMessage(tt.message)

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, 1, conv.GetMessageCount())
				lastMsg, exists := conv.GetLastMessage()
				assert.True(t, exists)
				assert.Equal(t, tt.message.GetID(), lastMsg.GetID())
			}
		})
	}
}

// TestMessageRetrieval tests various message retrieval methods
func TestMessageRetrieval(t *testing.T) {
	conv, err := NewConversation("test-conv")
	assert.NoError(t, err)

	now := time.Now()

	// Add test messages
	messages := []Message{
		&UserMessage{
			ID:        "msg-1",
			Role:      "user",
			Content:   "First message",
			Timestamp: now,
		},
		&AssistantMessage{
			ID:        "msg-2",
			Role:      "assistant",
			Content:   "Second message",
			Timestamp: now,
		},
		&ToolUseMessage{
			ID:        "msg-3",
			Role:      "assistant",
			ToolName:  "read_file",
			Input:     json.RawMessage(`{"path": "test.txt"}`),
			Timestamp: now,
		},
		&UserMessage{
			ID:        "msg-4",
			Role:      "user",
			Content:   "Third message",
			Timestamp: now,
		},
	}

	for _, msg := range messages {
		err := conv.AddMessage(msg)
		assert.NoError(t, err)
	}

	t.Run("get all messages", func(t *testing.T) {
		allMessages := conv.GetMessages()
		assert.Len(t, allMessages, 4)

		for i, msg := range allMessages {
			assert.Equal(t, messages[i].GetID(), msg.GetID())
		}
	})

	t.Run("get last message", func(t *testing.T) {
		lastMsg, exists := conv.GetLastMessage()
		assert.True(t, exists)
		assert.Equal(t, "msg-4", lastMsg.GetID())
		assert.Equal(t, "user", lastMsg.GetRole())
	})

	t.Run("get messages by role - user", func(t *testing.T) {
		userMessages := conv.GetMessagesByRole("user")
		assert.Len(t, userMessages, 2)
		assert.Equal(t, "msg-1", userMessages[0].GetID())
		assert.Equal(t, "msg-4", userMessages[1].GetID())
	})

	t.Run("get messages by role - assistant", func(t *testing.T) {
		assistantMessages := conv.GetMessagesByRole("assistant")
		assert.Len(t, assistantMessages, 2)
		assert.Equal(t, "msg-2", assistantMessages[0].GetID())
		assert.Equal(t, "msg-3", assistantMessages[1].GetID())
	})

	t.Run("get empty messages for non-existent role", func(t *testing.T) {
		systemMessages := conv.GetMessagesByRole("system")
		assert.Len(t, systemMessages, 0)
	})

	t.Run("get message count", func(t *testing.T) {
		assert.Equal(t, 4, conv.GetMessageCount())
		assert.Equal(t, 2, conv.GetMessageCountByRole("user"))
		assert.Equal(t, 2, conv.GetMessageCountByRole("assistant"))
		assert.Equal(t, 0, conv.GetMessageCountByRole("system"))
	})

	t.Run("get last message on empty conversation", func(t *testing.T) {
		emptyConv, _ := NewConversation("empty")
		_, exists := emptyConv.GetLastMessage()
		assert.False(t, exists)
	})
}

// TestToolManagement tests adding and finding tools in conversation
func TestToolManagement(t *testing.T) {
	conv, err := NewConversation("test-conv")
	assert.NoError(t, err)

	tools := []Tool{
		&MockTool{
			name:        "read_file",
			description: "Read a file",
		},
		&MockTool{
			name:        "write_file",
			description: "Write a file",
		},
	}

	t.Run("set tools", func(t *testing.T) {
		conv.SetTools(tools)
		retrievedTools := conv.GetTools()
		assert.Len(t, retrievedTools, 2)
		assert.Equal(t, "read_file", retrievedTools[0].GetName())
		assert.Equal(t, "write_file", retrievedTools[1].GetName())
	})

	t.Run("find existing tool", func(t *testing.T) {
		tool, found := conv.FindTool("read_file")
		assert.True(t, found)
		assert.NotNil(t, tool)
		assert.Equal(t, "read_file", tool.GetName())
	})

	t.Run("find non-existent tool", func(t *testing.T) {
		tool, found := conv.FindTool("delete_file")
		assert.False(t, found)
		assert.Nil(t, tool)
	})

	t.Run("find tool in empty tool set", func(t *testing.T) {
		emptyConv, _ := NewConversation("empty")
		tool, found := emptyConv.FindTool("read_file")
		assert.False(t, found)
		assert.Nil(t, tool)
	})
}

// TestClearMessages tests clearing all messages from conversation
func TestClearMessages(t *testing.T) {
	conv, err := NewConversation("test-conv")
	assert.NoError(t, err)

	// Add some messages
	msg1 := &UserMessage{
		ID:        "msg-1",
		Role:      "user",
		Content:   "Hello",
		Timestamp: time.Now(),
	}

	err = conv.AddMessage(msg1)
	assert.NoError(t, err)
	assert.Equal(t, 1, conv.GetMessageCount())

	// Clear messages
	conv.ClearMessages()
	assert.Equal(t, 0, conv.GetMessageCount())
	assert.Empty(t, conv.GetMessages())

	// Verify last message is not found
	_, exists := conv.GetLastMessage()
	assert.False(t, exists)

	// Verify updated timestamp
	assert.True(t, conv.GetUpdatedAt().After(conv.GetCreatedAt()))
}

// TestToolUseMessageFiltering tests filtering tool use messages
func TestToolUseMessageFiltering(t *testing.T) {
	conv, err := NewConversation("test-conv")
	assert.NoError(t, err)

	now := time.Now()

	// Add mixed messages
	messages := []Message{
		&UserMessage{
			ID:        "msg-1",
			Role:      "user",
			Content:   "Please read the file",
			Timestamp: now,
		},
		&ToolUseMessage{
			ID:        "msg-2",
			Role:      "assistant",
			ToolName:  "read_file",
			Input:     json.RawMessage(`{"path": "test.txt"}`),
			Timestamp: now,
		},
		&AssistantMessage{
			ID:        "msg-3",
			Role:      "assistant",
			Content:   "Here's the file content",
			Timestamp: now,
		},
		&ToolUseMessage{
			ID:        "msg-4",
			Role:      "assistant",
			ToolName:  "write_file",
			Input:     json.RawMessage(`{"path": "output.txt", "content": "data"}`),
			Timestamp: now,
		},
	}

	for _, msg := range messages {
		err := conv.AddMessage(msg)
		assert.NoError(t, err)
	}

	t.Run("get tool use messages", func(t *testing.T) {
		toolMessages := conv.GetToolUseMessages()
		assert.Len(t, toolMessages, 2)
		assert.Equal(t, "msg-2", toolMessages[0].GetID())
		assert.Equal(t, "msg-4", toolMessages[1].GetID())

		// Verify they're ToolUseMessage type
		for _, msg := range toolMessages {
			assert.IsType(t, &ToolUseMessage{}, msg)
		}
	})

	t.Run("get no tool use messages from regular conversation", func(t *testing.T) {
		emptyConv, _ := NewConversation("no-tools")
		msg := &UserMessage{
			ID:        "msg-1",
			Role:      "user",
			Content:   "Hello",
			Timestamp: now,
		}
		emptyConv.AddMessage(msg)

		toolMessages := emptyConv.GetToolUseMessages()
		assert.Len(t, toolMessages, 0)
	})
}

// TestConversationTimestamps tests timestamp management
func TestConversationTimestamps(t *testing.T) {
	before := time.Now()
	conv, err := NewConversation("test-conv")
	assert.NoError(t, err)
	after := time.Now()

	// Check created timestamp
	assert.True(t, conv.GetCreatedAt().After(before) || conv.GetCreatedAt().Equal(before))
	assert.True(t, conv.GetCreatedAt().Before(after) || conv.GetCreatedAt().Equal(after))

	// Initially, updated should equal created
	assert.Equal(t, conv.GetCreatedAt(), conv.GetUpdatedAt())

	// Add a message and check updated timestamp changes
	msg := &UserMessage{
		ID:        "msg-1",
		Role:      "user",
		Content:   "Hello",
		Timestamp: time.Now(),
	}

	time.Sleep(10 * time.Millisecond) // Ensure different timestamp
	err = conv.AddMessage(msg)
	assert.NoError(t, err)

	assert.True(t, conv.GetUpdatedAt().After(conv.GetCreatedAt()))

	// Clear messages and check updated timestamp changes again
	time.Sleep(10 * time.Millisecond)
	conv.ClearMessages()

	assert.True(t, conv.GetUpdatedAt().After(conv.GetCreatedAt()))
}

// TestConcurrentAccess tests thread safety of conversation operations
func TestConcurrentAccess(t *testing.T) {
	conv, err := NewConversation("concurrent-test")
	assert.NoError(t, err)

	const numGoroutines = 10
	const messagesPerGoroutine = 5

	errChan := make(chan error, numGoroutines)

	// Launch multiple goroutines adding messages concurrently
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < messagesPerGoroutines; j++ {
				msg := &UserMessage{
					ID:        fmt.Sprintf("msg-%d-%d", goroutineID, j),
					Role:      "user",
					Content:   fmt.Sprintf("Message %d from goroutine %d", j, goroutineID),
					Timestamp: time.Now(),
				}

				if err := conv.AddMessage(msg); err != nil {
					errChan <- fmt.Errorf("goroutine %d, message %d: %w", goroutineID, j, err)
					return
				}
			}
			errChan <- nil
		}(i)
	}

	// Collect errors
	for i := 0; i < numGoroutines; i++ {
		select {
		case err := <-errChan:
			assert.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for goroutines")
		}
	}

	// Verify all messages were added
	assert.Equal(t, numGoroutines*messagesPerGoroutine, conv.GetMessageCount())

	// Verify all message IDs are unique
	messageIDs := make(map[string]bool)
	for _, msg := range conv.GetMessages() {
		assert.False(t, messageIDs[msg.GetID()], "Duplicate message ID found: %s", msg.GetID())
		messageIDs[msg.GetID()] = true
	}

	assert.Len(t, messageIDs, numGoroutines*messagesPerGoroutine)
}

// Mock implementations for testing

type UserMessage struct {
	ID        string
	Role      string
	Content   string
	Timestamp time.Time
}

func (m *UserMessage) GetID() string           { return m.ID }
func (m *UserMessage) GetRole() string         { return m.Role }
func (m *UserMessage) GetContent() interface{} { return m.Content }
func (m *UserMessage) GetTimestamp() time.Time { return m.Timestamp }
func (m *UserMessage) ToJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        m.ID,
		"role":      m.Role,
		"content":   m.Content,
		"timestamp": m.Timestamp,
	})
}

type AssistantMessage struct {
	ID        string
	Role      string
	Content   string
	Timestamp time.Time
}

func (m *AssistantMessage) GetID() string           { return m.ID }
func (m *AssistantMessage) GetRole() string         { return m.Role }
func (m *AssistantMessage) GetContent() interface{} { return m.Content }
func (m *AssistantMessage) GetTimestamp() time.Time { return m.Timestamp }
func (m *AssistantMessage) ToJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        m.ID,
		"role":      m.Role,
		"content":   m.Content,
		"timestamp": m.Timestamp,
	})
}

type ToolUseMessage struct {
	ID        string
	Role      string
	ToolName  string
	Input     json.RawMessage
	Timestamp time.Time
}

func (m *ToolUseMessage) GetID() string   { return m.ID }
func (m *ToolUseMessage) GetRole() string { return m.Role }
func (m *ToolUseMessage) GetContent() interface{} {
	return map[string]interface{}{
		"tool_name": m.ToolName,
		"input":     m.Input,
	}
}
func (m *ToolUseMessage) GetTimestamp() time.Time { return m.Timestamp }
func (m *ToolUseMessage) ToJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        m.ID,
		"role":      m.Role,
		"content":   m.GetContent(),
		"timestamp": m.Timestamp,
	})
}

type ToolResultMessage struct {
	ID        string
	Role      string
	ToolUseID string
	Result    string
	IsError   bool
	Timestamp time.Time
}

func (m *ToolResultMessage) GetID() string   { return m.ID }
func (m *ToolResultMessage) GetRole() string { return m.Role }
func (m *ToolResultMessage) GetContent() interface{} {
	return map[string]interface{}{
		"tool_use_id": m.ToolUseID,
		"result":      m.Result,
		"is_error":    m.IsError,
	}
}
func (m *ToolResultMessage) GetTimestamp() time.Time { return m.Timestamp }
func (m *ToolResultMessage) GetJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        m.ID,
		"role":      m.Role,
		"content":   m.GetContent(),
		"timestamp": m.Timestamp,
	})
}

type MockTool struct {
	name        string
	description string
}

func (t *MockTool) GetName() string        { return t.name }
func (t *MockTool) GetDescription() string { return t.description }
func (t *MockTool) GetInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type": "string",
			},
		},
	}
}

func (t *MockTool) Execute(input json.RawMessage) (string, error) {
	return "mock result", nil
}

// Constructor function for interface
func NewConversation(id string) (Conversation, error) {
	return nil, fmt.Errorf("Not implemented - this is a red phase test")
}

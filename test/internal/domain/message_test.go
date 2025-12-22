package domain

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// MessageTypes
const (
	MessageRoleUser       = "user"
	MessageRoleAssistant  = "assistant"
	MessageRoleSystem     = "system"
	MessageTypeText       = "text"
	MessageTypeToolUse    = "tool_use"
	MessageTypeToolResult = "tool_result"
)

// TestMessageCreation tests creating different types of messages
func TestMessageCreation(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		createMsg   func() (Message, error)
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, msg Message)
	}{
		{
			name: "create user message",
			createMsg: func() (Message, error) {
				return NewUserMessage("user-msg-1", "Hello, world!", now)
			},
			expectError: false,
			errorMsg:    "",
			validate: func(t *testing.T, msg Message) {
				assert.Equal(t, "user-msg-1", msg.GetID())
				assert.Equal(t, MessageRoleUser, msg.GetRole())
				assert.Equal(t, "Hello, world!", msg.GetContent())
				assert.Equal(t, now, msg.GetTimestamp())
			},
		},
		{
			name: "create user message with empty ID should fail",
			createMsg: func() (Message, error) {
				return NewUserMessage("", "Hello, world!", now)
			},
			expectError: true,
			errorMsg:    "message ID cannot be empty",
			validate:    nil,
		},
		{
			name: "create assistant message",
			createMsg: func() (Message, error) {
				return NewAssistantMessage("assistant-msg-1", "Hello! How can I help?", now)
			},
			expectError: false,
			errorMsg:    "",
			validate: func(t *testing.T, msg Message) {
				assert.Equal(t, "assistant-msg-1", msg.GetID())
				assert.Equal(t, MessageRoleAssistant, msg.GetRole())
				assert.Equal(t, "Hello! How can I help?", msg.GetContent())
				assert.Equal(t, now, msg.GetTimestamp())
			},
		},
		{
			name: "create system message",
			createMsg: func() (Message, error) {
				return NewSystemMessage("system-msg-1", "You are a helpful assistant.", now)
			},
			expectError: false,
			errorMsg:    "",
			validate: func(t *testing.T, msg Message) {
				assert.Equal(t, "system-msg-1", msg.GetID())
				assert.Equal(t, MessageRoleSystem, msg.GetRole())
				assert.Equal(t, "You are a helpful assistant.", msg.GetContent())
				assert.Equal(t, now, msg.GetTimestamp())
			},
		},
		{
			name: "create tool use message",
			createMsg: func() (Message, error) {
				input := json.RawMessage(`{"path": "test.txt"}`)
				return NewToolUseMessage("tool-msg-1", "read_file", input, now)
			},
			expectError: false,
			errorMsg:    "",
			validate: func(t *testing.T, msg Message) {
				assert.Equal(t, "tool-msg-1", msg.GetID())
				assert.Equal(t, MessageRoleAssistant, msg.GetRole())

				content := msg.GetContent().(map[string]interface{})
				assert.Equal(t, "read_file", content["tool_name"])
				assert.Equal(t, json.RawMessage(`{"path": "test.txt"}`), content["input"])
				assert.Equal(t, now, msg.GetTimestamp())
			},
		},
		{
			name: "create tool use message with empty tool name should fail",
			createMsg: func() (Message, error) {
				input := json.RawMessage(`{"path": "test.txt"}`)
				return NewToolUseMessage("tool-msg-1", "", input, now)
			},
			expectError: true,
			errorMsg:    "tool name cannot be empty",
			validate:    nil,
		},
		{
			name: "create tool result message",
			createMsg: func() (Message, error) {
				return NewToolResultMessage("tool-result-1", "tool-msg-1", "File content here", false, now)
			},
			expectError: false,
			errorMsg:    "",
			validate: func(t *testing.T, msg Message) {
				assert.Equal(t, "tool-result-1", msg.GetID())
				assert.Equal(t, MessageRoleUser, msg.GetRole())

				content := msg.GetContent().(map[string]interface{})
				assert.Equal(t, "tool-msg-1", content["tool_use_id"])
				assert.Equal(t, "File content here", content["result"])
				assert.Equal(t, false, content["is_error"])
				assert.Equal(t, now, msg.GetTimestamp())
			},
		},
		{
			name: "create error tool result message",
			createMsg: func() (Message, error) {
				return NewToolResultMessage("tool-error-1", "tool-msg-1", "File not found", true, now)
			},
			expectError: false,
			errorMsg:    "",
			validate: func(t *testing.T, msg Message) {
				assert.Equal(t, "tool-error-1", msg.GetID())
				assert.Equal(t, MessageRoleUser, msg.GetRole())

				content := msg.GetContent().(map[string]interface{})
				assert.Equal(t, "tool-msg-1", content["tool_use_id"])
				assert.Equal(t, "File not found", content["result"])
				assert.Equal(t, true, content["is_error"])
				assert.Equal(t, now, msg.GetTimestamp())
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, err := tt.createMsg()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, msg)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, msg)
				if tt.validate != nil {
					tt.validate(t, msg)
				}
			}
		})
	}
}

// TestMessageSerialization tests JSON serialization and deserialization of messages
func TestMessageSerialization(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		message   Message
		expectErr bool
		validate  func(t *testing.T, data []byte, original Message)
	}{
		{
			name: "serialize user message",
			message: &mockUserMessage{
				id:        "user-1",
				role:      MessageRoleUser,
				content:   "Hello world",
				timestamp: now,
			},
			expectErr: false,
			validate: func(t *testing.T, data []byte, original Message) {
				var result map[string]interface{}
				err := json.Unmarshal(data, &result)
				assert.NoError(t, err)
				assert.Equal(t, "user-1", result["id"])
				assert.Equal(t, MessageRoleUser, result["role"])
				assert.Equal(t, "Hello world", result["content"])
			},
		},
		{
			name: "serialize assistant message",
			message: &mockAssistantMessage{
				id:        "assistant-1",
				role:      MessageRoleAssistant,
				content:   "Hi there!",
				timestamp: now,
			},
			expectErr: false,
			validate: func(t *testing.T, data []byte, original Message) {
				var result map[string]interface{}
				err := json.Unmarshal(data, &result)
				assert.NoError(t, err)
				assert.Equal(t, "assistant-1", result["id"])
				assert.Equal(t, MessageRoleAssistant, result["role"])
				assert.Equal(t, "Hi there!", result["content"])
			},
		},
		{
			name: "serialize tool use message",
			message: &mockToolUseMessage{
				id:        "tool-1",
				role:      MessageRoleAssistant,
				toolName:  "read_file",
				input:     json.RawMessage(`{"path": "test.txt"}`),
				timestamp: now,
			},
			expectErr: false,
			validate: func(t *testing.T, data []byte, original Message) {
				var result map[string]interface{}
				err := json.Unmarshal(data, &result)
				assert.NoError(t, err)
				assert.Equal(t, "tool-1", result["id"])
				assert.Equal(t, MessageRoleAssistant, result["role"])

				content := result["content"].(map[string]interface{})
				assert.Equal(t, "read_file", content["tool_name"])
				assert.Equal(t, json.RawMessage(`{"path": "test.txt"}`), content["input"])
			},
		},
		{
			name: "serialize tool result message",
			message: &mockToolResultMessage{
				id:        "result-1",
				role:      MessageRoleUser,
				toolUseID: "tool-1",
				result:    "Success",
				isError:   false,
				timestamp: now,
			},
			expectErr: false,
			validate: func(t *testing.T, data []byte, original Message) {
				var result map[string]interface{}
				err := json.Unmarshal(data, &result)
				assert.NoError(t, err)
				assert.Equal(t, "result-1", result["id"])
				assert.Equal(t, MessageRoleUser, result["role"])

				content := result["content"].(map[string]interface{})
				assert.Equal(t, "tool-1", content["tool_use_id"])
				assert.Equal(t, "Success", content["result"])
				assert.Equal(t, false, content["is_error"])
			},
		},
		{
			name: "serialize message with invalid data",
			message: &mockInvalidMessage{
				id: "invalid-1",
			},
			expectErr: true,
			validate:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := tt.message.ToJSON()

			if tt.expectErr {
				assert.Error(t, err)
				assert.Nil(t, data)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, data)
				if tt.validate != nil {
					tt.validate(t, data, tt.message)
				}
			}
		})
	}
}

// TestMessageValidation tests message validation logic
func TestMessageValidation(t *testing.T) {
	tests := []struct {
		name        string
		message     Message
		expectValid bool
		errorMsg    string
	}{
		{
			name: "valid user message",
			message: &mockUserMessage{
				id:        "user-1",
				role:      MessageRoleUser,
				content:   "Hello",
				timestamp: time.Now(),
			},
			expectValid: true,
			errorMsg:    "",
		},
		{
			name: "empty ID should be invalid",
			message: &mockUserMessage{
				id:        "",
				role:      MessageRoleUser,
				content:   "Hello",
				timestamp: time.Now(),
			},
			expectValid: false,
			errorMsg:    "message ID cannot be empty",
		},
		{
			name: "invalid role should be invalid",
			message: &mockUserMessage{
				id:        "user-1",
				role:      "invalid_role",
				content:   "Hello",
				timestamp: time.Now(),
			},
			expectValid: false,
			errorMsg:    "invalid message role",
		},
		{
			name: "zero timestamp should be invalid",
			message: &mockUserMessage{
				id:        "user-1",
				role:      MessageRoleUser,
				content:   "Hello",
				timestamp: time.Time{},
			},
			expectValid: false,
			errorMsg:    "message timestamp cannot be zero",
		},
		{
			name: "valid tool use message",
			message: &mockToolUseMessage{
				id:        "tool-1",
				role:      MessageRoleAssistant,
				toolName:  "read_file",
				input:     json.RawMessage(`{}`),
				timestamp: time.Now(),
			},
			expectValid: true,
			errorMsg:    "",
		},
		{
			name: "tool use message with empty tool name should be invalid",
			message: &mockToolUseMessage{
				id:        "tool-1",
				role:      MessageRoleAssistant,
				toolName:  "",
				input:     json.RawMessage(`{}`),
				timestamp: time.Now(),
			},
			expectValid: false,
			errorMsg:    "tool name cannot be empty",
		},
		{
			name: "valid tool result message",
			message: &mockToolResultMessage{
				id:        "result-1",
				role:      MessageRoleUser,
				toolUseID: "tool-1",
				result:    "Success",
				isError:   false,
				timestamp: time.Now(),
			},
			expectValid: true,
			errorMsg:    "",
		},
		{
			name: "tool result message with empty tool use ID should be invalid",
			message: &mockToolResultMessage{
				id:        "result-1",
				role:      MessageRoleUser,
				toolUseID: "",
				result:    "Success",
				isError:   false,
				timestamp: time.Now(),
			},
			expectValid: false,
			errorMsg:    "tool use ID cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMessage(tt.message)

			if tt.expectValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			}
		})
	}
}

// TestMessageTypeDetection tests type detection for different message kinds
func TestMessageTypeDetection(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name         string
		message      Message
		expectedType string
	}{
		{
			name: "detect user message",
			message: &mockUserMessage{
				id:        "user-1",
				role:      MessageRoleUser,
				content:   "Hello",
				timestamp: now,
			},
			expectedType: MessageTypeText,
		},
		{
			name: "detect assistant message",
			message: &mockAssistantMessage{
				id:        "assistant-1",
				role:      MessageRoleAssistant,
				content:   "Hi",
				timestamp: now,
			},
			expectedType: MessageTypeText,
		},
		{
			name: "detect tool use message",
			message: &mockToolUseMessage{
				id:        "tool-1",
				role:      MessageRoleAssistant,
				toolName:  "read_file",
				input:     json.RawMessage(`{}`),
				timestamp: now,
			},
			expectedType: MessageTypeToolUse,
		},
		{
			name: "detect tool result message",
			message: &mockToolResultMessage{
				id:        "result-1",
				role:      MessageRoleUser,
				toolUseID: "tool-1",
				result:    "Success",
				isError:   false,
				timestamp: now,
			},
			expectedType: MessageTypeToolResult,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msgType := DetectMessageType(tt.message)
			assert.Equal(t, tt.expectedType, msgType)
		})
	}
}

// TestMessageEquality tests equality comparison between messages
func TestMessageEquality(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		msg1     Message
		msg2     Message
		areEqual bool
	}{
		{
			name: "identical user messages should be equal",
			msg1: &mockUserMessage{
				id:        "user-1",
				role:      MessageRoleUser,
				content:   "Hello",
				timestamp: now,
			},
			msg2: &mockUserMessage{
				id:        "user-1",
				role:      MessageRoleUser,
				content:   "Hello",
				timestamp: now,
			},
			areEqual: true,
		},
		{
			name: "messages with different IDs should not be equal",
			msg1: &mockUserMessage{
				id:        "user-1",
				role:      MessageRoleUser,
				content:   "Hello",
				timestamp: now,
			},
			msg2: &mockUserMessage{
				id:        "user-2",
				role:      MessageRoleUser,
				content:   "Hello",
				timestamp: now,
			},
			areEqual: false,
		},
		{
			name: "different message types should not be equal",
			msg1: &mockUserMessage{
				id:        "user-1",
				role:      MessageRoleUser,
				content:   "Hello",
				timestamp: now,
			},
			msg2: &mockAssistantMessage{
				id:        "user-1",
				role:      MessageRoleAssistant,
				content:   "Hello",
				timestamp: now,
			},
			areEqual: false,
		},
		{
			name: "identical tool use messages should be equal",
			msg1: &mockToolUseMessage{
				id:        "tool-1",
				role:      MessageRoleAssistant,
				toolName:  "read_file",
				input:     json.RawMessage(`{"path": "test"}`),
				timestamp: now,
			},
			msg2: &mockToolUseMessage{
				id:        "tool-1",
				role:      MessageRoleAssistant,
				toolName:  "read_file",
				input:     json.RawMessage(`{"path": "test"}`),
				timestamp: now,
			},
			areEqual: true,
		},
		{
			name: "tool use messages with different inputs should not be equal",
			msg1: &mockToolUseMessage{
				id:        "tool-1",
				role:      MessageRoleAssistant,
				toolName:  "read_file",
				input:     json.RawMessage(`{"path": "test1"}`),
				timestamp: now,
			},
			msg2: &mockToolUseMessage{
				id:        "tool-1",
				role:      MessageRoleAssistant,
				toolName:  "read_file",
				input:     json.RawMessage(`{"path": "test2"}`),
				timestamp: now,
			},
			areEqual: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			equal := MessagesEqual(tt.msg1, tt.msg2)
			assert.Equal(t, tt.areEqual, equal)
		})
	}
}

// TestMessageContentAccess tests content access methods for different message types
func TestMessageContentAccess(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		message  Message
		accessor func(Message) interface{}
		expected interface{}
	}{
		{
			name:     "get text content from user message",
			message:  &mockUserMessage{id: "1", role: MessageRoleUser, content: "Hello", timestamp: now},
			accessor: func(m Message) interface{} { return GetTextContent(m) },
			expected: "Hello",
		},
		{
			name:     "get text content from assistant message",
			message:  &mockAssistantMessage{id: "1", role: MessageRoleAssistant, content: "Hi there", timestamp: now},
			accessor: func(m Message) interface{} { return GetTextContent(m) },
			expected: "Hi there",
		},
		{
			name:     "get text content from system message",
			message:  &mockSystemMessage{id: "1", role: MessageRoleSystem, content: "System prompt", timestamp: now},
			accessor: func(m Message) interface{} { return GetTextContent(m) },
			expected: "System prompt",
		},
		{
			name: "get tool info from tool use message",
			message: &mockToolUseMessage{
				id:        "tool-1",
				role:      MessageRoleAssistant,
				toolName:  "read_file",
				input:     json.RawMessage(`{"path": "test.txt"}`),
				timestamp: now,
			},
			accessor: func(m Message) interface{} { return GetToolInfo(m) },
			expected: map[string]interface{}{
				"tool_name": "read_file",
				"input":     json.RawMessage(`{"path": "test.txt"}`),
			},
		},
		{
			name: "get tool result info from tool result message",
			message: &mockToolResultMessage{
				id:        "result-1",
				role:      MessageRoleUser,
				toolUseID: "tool-1",
				result:    "File content",
				isError:   false,
				timestamp: now,
			},
			accessor: func(m Message) interface{} { return GetToolResultInfo(m) },
			expected: map[string]interface{}{
				"tool_use_id": "tool-1",
				"result":      "File content",
				"is_error":    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.accessor(tt.message)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Mock implementations for testing

type mockUserMessage struct {
	id        string
	role      string
	content   string
	timestamp time.Time
}

func (m *mockUserMessage) GetID() string           { return m.id }
func (m *mockUserMessage) GetRole() string         { return m.role }
func (m *mockUserMessage) GetContent() interface{} { return m.content }
func (m *mockUserMessage) GetTimestamp() time.Time { return m.timestamp }
func (m *mockUserMessage) ToJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        m.id,
		"role":      m.role,
		"content":   m.content,
		"timestamp": m.timestamp,
	})
}

type mockAssistantMessage struct {
	id        string
	role      string
	content   string
	timestamp time.Time
}

func (m *mockAssistantMessage) GetID() string           { return m.id }
func (m *mockAssistantMessage) GetRole() string         { return m.role }
func (m *mockAssistantMessage) GetContent() interface{} { return m.content }
func (m *mockAssistantMessage) GetTimestamp() time.Time { return m.timestamp }
func (m *mockAssistantMessage) ToJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        m.id,
		"role":      m.role,
		"content":   m.content,
		"timestamp": m.timestamp,
	})
}

type mockSystemMessage struct {
	id        string
	role      string
	content   string
	timestamp time.Time
}

func (m *mockSystemMessage) GetID() string           { return m.id }
func (m *mockSystemMessage) GetRole() string         { return m.role }
func (m *mockSystemMessage) GetContent() interface{} { return m.content }
func (m *mockSystemMessage) GetTimestamp() time.Time { return m.timestamp }
func (m *mockSystemMessage) ToJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        m.id,
		"role":      m.role,
		"content":   m.content,
		"timestamp": m.timestamp,
	})
}

type mockToolUseMessage struct {
	id        string
	role      string
	toolName  string
	input     json.RawMessage
	timestamp time.Time
}

func (m *mockToolUseMessage) GetID() string   { return m.id }
func (m *mockToolUseMessage) GetRole() string { return m.role }
func (m *mockToolUseMessage) GetContent() interface{} {
	return map[string]interface{}{
		"tool_name": m.toolName,
		"input":     m.input,
	}
}
func (m *mockToolUseMessage) GetTimestamp() time.Time { return m.timestamp }
func (m *mockToolUseMessage) ToJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        m.id,
		"role":      m.role,
		"content":   m.GetContent(),
		"timestamp": m.timestamp,
	})
}

type mockToolResultMessage struct {
	id        string
	role      string
	toolUseID string
	result    string
	isError   bool
	timestamp time.Time
}

func (m *mockToolResultMessage) GetID() string   { return m.id }
func (m *mockToolResultMessage) GetRole() string { return m.role }
func (m *mockToolResultMessage) GetContent() interface{} {
	return map[string]interface{}{
		"tool_use_id": m.toolUseID,
		"result":      m.result,
		"is_error":    m.isError,
	}
}
func (m *mockToolResultMessage) GetTimestamp() time.Time { return m.timestamp }
func (m *mockToolResultMessage) ToJSON() ([]byte, error) {
	return json.Marshal(map[string]interface{}{
		"id":        m.id,
		"role":      m.role,
		"content":   m.GetContent(),
		"timestamp": m.timestamp,
	})
}

type mockInvalidMessage struct {
	id string
}

func (m *mockInvalidMessage) GetID() string           { return m.id }
func (m *mockInvalidMessage) GetRole() string         { return "invalid" }
func (m *mockInvalidMessage) GetContent() interface{} { return nil }
func (m *mockInvalidMessage) GetTimestamp() time.Time { return time.Now() }
func (m *mockInvalidMessage) ToJSON() ([]byte, error) {
	return nil, fmt.Errorf("JSON serialization failed")
}

// Constructor functions that will fail
func NewUserMessage(id, content string, timestamp time.Time) (Message, error) {
	return nil, fmt.Errorf("Not implemented - this is a red phase test")
}

func NewAssistantMessage(id, content string, timestamp time.Time) (Message, error) {
	return nil, fmt.Errorf("Not implemented - this is a red phase test")
}

func NewSystemMessage(id, content string, timestamp time.Time) (Message, error) {
	return nil, fmt.Errorf("Not implemented - this is a red phase test")
}

func NewToolUseMessage(id, toolName string, input json.RawMessage, timestamp time.Time) (Message, error) {
	return nil, fmt.Errorf("Not implemented - this is a red phase test")
}

func NewToolResultMessage(id, toolUseID, result string, isError bool, timestamp time.Time) (Message, error) {
	return nil, fmt.Errorf("Not implemented - this is a red phase test")
}

// Utility functions that will fail
func ValidateMessage(msg Message) error {
	return fmt.Errorf("Not implemented - this is a red phase test")
}

func DetectMessageType(msg Message) string {
	return "not_implemented"
}

func MessagesEqual(msg1, msg2 Message) bool {
	return false
}

func GetTextContent(msg Message) string {
	return ""
}

func GetToolInfo(msg Message) map[string]interface{} {
	return nil
}

func GetToolResultInfo(msg Message) map[string]interface{} {
	return nil
}

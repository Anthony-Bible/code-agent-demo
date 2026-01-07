package entity

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestMessage_NewMessage(t *testing.T) {
	tests := []struct {
		name    string
		role    string
		content string
		want    *Message
		wantErr bool
	}{
		{
			name:    "should create valid user message",
			role:    "user",
			content: "Hello, how are you?",
			want:    &Message{Role: "user", Content: "Hello, how are you?", Timestamp: time.Now()},
			wantErr: false,
		},
		{
			name:    "should create valid assistant message",
			role:    "assistant",
			content: "I'm doing well, thank you!",
			want:    &Message{Role: "assistant", Content: "I'm doing well, thank you!", Timestamp: time.Now()},
			wantErr: false,
		},
		{
			name:    "should create valid system message",
			role:    "system",
			content: "You are a helpful assistant.",
			want:    &Message{Role: "system", Content: "You are a helpful assistant.", Timestamp: time.Now()},
			wantErr: false,
		},
		{
			name:    "should reject message with empty role",
			role:    "",
			content: "Hello",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "should reject message with empty content",
			role:    "user",
			content: "",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "should reject message with whitespace-only content",
			role:    "user",
			content: "   ",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "should reject message with invalid role",
			role:    "invalid",
			content: "Hello",
			want:    nil,
			wantErr: true,
		},
		{
			name:    "should reject message with mixed case role",
			role:    "User",
			content: "Hello",
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMessage(tt.role, tt.content)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got == nil {
					t.Error("NewMessage() returned nil message")
					return
				}
				if got.Role != tt.role {
					t.Errorf("NewMessage() role = %v, want %v", got.Role, tt.role)
				}
				if got.Content != tt.content {
					t.Errorf("NewMessage() content = %v, want %v", got.Content, tt.content)
				}
				if got.Timestamp.IsZero() {
					t.Error("NewMessage() timestamp should not be zero")
				}
			} else if got != nil {
				t.Errorf("NewMessage() returned non-nil message on error: %+v", got)
			}
		})
	}
}

func TestMessage_IsUser(t *testing.T) {
	tests := []struct {
		name string
		role string
		want bool
	}{
		{
			name: "user message should return true",
			role: "user",
			want: true,
		},
		{
			name: "assistant message should return false",
			role: "assistant",
			want: false,
		},
		{
			name: "system message should return false",
			role: "system",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{Role: tt.role}
			got := m.IsUser()
			if got != tt.want {
				t.Errorf("Message.IsUser() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_IsAssistant(t *testing.T) {
	tests := []struct {
		name string
		role string
		want bool
	}{
		{
			name: "assistant message should return true",
			role: "assistant",
			want: true,
		},
		{
			name: "user message should return false",
			role: "user",
			want: false,
		},
		{
			name: "system message should return false",
			role: "system",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{Role: tt.role}
			got := m.IsAssistant()
			if got != tt.want {
				t.Errorf("Message.IsAssistant() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_IsSystem(t *testing.T) {
	tests := []struct {
		name string
		role string
		want bool
	}{
		{
			name: "system message should return true",
			role: "system",
			want: true,
		},
		{
			name: "user message should return false",
			role: "user",
			want: false,
		},
		{
			name: "assistant message should return false",
			role: "assistant",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{Role: tt.role}
			got := m.IsSystem()
			if got != tt.want {
				t.Errorf("Message.IsSystem() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMessage_Validate(t *testing.T) {
	tests := []struct {
		name    string
		message Message
		wantErr bool
	}{
		{
			name:    "valid user message should pass validation",
			message: Message{Role: "user", Content: "Hello", Timestamp: time.Now()},
			wantErr: false,
		},
		{
			name:    "valid assistant message should pass validation",
			message: Message{Role: "assistant", Content: "Hi there!", Timestamp: time.Now()},
			wantErr: false,
		},
		{
			name:    "valid system message should pass validation",
			message: Message{Role: "system", Content: "System message", Timestamp: time.Now()},
			wantErr: false,
		},
		{
			name:    "message with empty role should fail validation",
			message: Message{Role: "", Content: "Hello", Timestamp: time.Now()},
			wantErr: true,
		},
		{
			name:    "message with empty content should fail validation",
			message: Message{Role: "user", Content: "", Timestamp: time.Now()},
			wantErr: true,
		},
		{
			name:    "message with whitespace content should fail validation",
			message: Message{Role: "user", Content: "   ", Timestamp: time.Now()},
			wantErr: true,
		},
		{
			name:    "message with invalid role should fail validation",
			message: Message{Role: "invalid", Content: "Hello", Timestamp: time.Now()},
			wantErr: true,
		},
		{
			name:    "message with zero timestamp should fail validation",
			message: Message{Role: "user", Content: "Hello", Timestamp: time.Time{}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.message.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Message.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMessage_UpdateContent(t *testing.T) {
	type fields struct {
		Role      string
		Content   string
		Timestamp time.Time
	}
	type args struct {
		newContent string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:    "should update content successfully",
			fields:  fields{Role: "user", Content: "Hello", Timestamp: time.Now()},
			args:    args{newContent: "Hello, world!"},
			wantErr: false,
		},
		{
			name:    "should reject empty content",
			fields:  fields{Role: "user", Content: "Hello", Timestamp: time.Now()},
			args:    args{newContent: ""},
			wantErr: true,
		},
		{
			name:    "should reject whitespace-only content",
			fields:  fields{Role: "user", Content: "Hello", Timestamp: time.Now()},
			args:    args{newContent: "   "},
			wantErr: true,
		},
		{
			name:    "should allow updating to same role's content",
			fields:  fields{Role: "assistant", Content: "Previous response", Timestamp: time.Now()},
			args:    args{newContent: "Updated response"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{
				Role:      tt.fields.Role,
				Content:   tt.fields.Content,
				Timestamp: tt.fields.Timestamp,
			}
			err := m.UpdateContent(tt.args.newContent)
			if (err != nil) != tt.wantErr {
				t.Errorf("Message.UpdateContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if m.Content != tt.args.newContent {
					t.Errorf("Message.UpdateContent() content = %v, want %v", m.Content, tt.args.newContent)
				}
			}
		})
	}
}

func TestMessage_GetAge(t *testing.T) {
	pastTime := time.Now().Add(-1 * time.Hour)
	recentTime := time.Now().Add(-5 * time.Minute)

	tests := []struct {
		name      string
		timestamp time.Time
		want      time.Duration
	}{
		{
			name:      "should return age for hour-old message",
			timestamp: pastTime,
			want:      time.Hour - (time.Since(pastTime) - time.Hour),
		},
		{
			name:      "should return age for recent message",
			timestamp: recentTime,
			want:      5*time.Minute - (time.Since(recentTime) - 5*time.Minute),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &Message{Timestamp: tt.timestamp}
			got := m.GetAge()
			// Allow for small time differences
			if got < tt.want-time.Second || got > tt.want+time.Second {
				t.Errorf("Message.GetAge() = %v, want ~%v", got, tt.want)
			}
		})
	}
}

// TestThinkingBlock_Creation tests the creation of ThinkingBlock structs.
func TestThinkingBlock_Creation(t *testing.T) {
	tests := []struct {
		name      string
		thinking  string
		signature string
		want      ThinkingBlock
	}{
		{
			name:      "should create thinking block with both fields populated",
			thinking:  "Let me analyze this problem step by step...",
			signature: "sig_abc123",
			want: ThinkingBlock{
				Thinking:  "Let me analyze this problem step by step...",
				Signature: "sig_abc123",
			},
		},
		{
			name:      "should create thinking block with empty thinking field",
			thinking:  "",
			signature: "sig_def456",
			want: ThinkingBlock{
				Thinking:  "",
				Signature: "sig_def456",
			},
		},
		{
			name:      "should create thinking block with empty signature field",
			thinking:  "Some analysis text",
			signature: "",
			want: ThinkingBlock{
				Thinking:  "Some analysis text",
				Signature: "",
			},
		},
		{
			name:      "should create thinking block with both fields empty",
			thinking:  "",
			signature: "",
			want: ThinkingBlock{
				Thinking:  "",
				Signature: "",
			},
		},
		{
			name:      "should create thinking block with multiline thinking",
			thinking:  "First step: analyze requirements\nSecond step: design solution\nThird step: implement",
			signature: "sig_multi",
			want: ThinkingBlock{
				Thinking:  "First step: analyze requirements\nSecond step: design solution\nThird step: implement",
				Signature: "sig_multi",
			},
		},
		{
			name:      "should create thinking block with special characters",
			thinking:  "Analysis: {\"key\": \"value\"} & <html>",
			signature: "sig_special_!@#$%",
			want: ThinkingBlock{
				Thinking:  "Analysis: {\"key\": \"value\"} & <html>",
				Signature: "sig_special_!@#$%",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ThinkingBlock{
				Thinking:  tt.thinking,
				Signature: tt.signature,
			}
			if got.Thinking != tt.want.Thinking {
				t.Errorf("ThinkingBlock.Thinking = %v, want %v", got.Thinking, tt.want.Thinking)
			}
			if got.Signature != tt.want.Signature {
				t.Errorf("ThinkingBlock.Signature = %v, want %v", got.Signature, tt.want.Signature)
			}
		})
	}
}

// TestThinkingBlock_JSONSerialization tests JSON serialization and deserialization of ThinkingBlock.
func TestThinkingBlock_JSONSerialization(t *testing.T) {
	tests := []struct {
		name     string
		block    ThinkingBlock
		wantJSON string
	}{
		{
			name: "should serialize thinking block with both fields",
			block: ThinkingBlock{
				Thinking:  "Analysis text",
				Signature: "sig_123",
			},
			wantJSON: `{"thinking":"Analysis text","signature":"sig_123"}`,
		},
		{
			name: "should serialize thinking block with empty thinking",
			block: ThinkingBlock{
				Thinking:  "",
				Signature: "sig_456",
			},
			wantJSON: `{"thinking":"","signature":"sig_456"}`,
		},
		{
			name: "should serialize thinking block with empty signature",
			block: ThinkingBlock{
				Thinking:  "Some thinking",
				Signature: "",
			},
			wantJSON: `{"thinking":"Some thinking","signature":""}`,
		},
		{
			name: "should serialize thinking block with both fields empty",
			block: ThinkingBlock{
				Thinking:  "",
				Signature: "",
			},
			wantJSON: `{"thinking":"","signature":""}`,
		},
		{
			name: "should serialize thinking block with special characters",
			block: ThinkingBlock{
				Thinking:  "Text with \"quotes\" and \nnewlines",
				Signature: "sig_special",
			},
			wantJSON: `{"thinking":"Text with \"quotes\" and \nnewlines","signature":"sig_special"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Marshal
			gotJSON, err := json.Marshal(tt.block)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if string(gotJSON) != tt.wantJSON {
				t.Errorf("json.Marshal() = %v, want %v", string(gotJSON), tt.wantJSON)
			}

			// Test Unmarshal
			var gotBlock ThinkingBlock
			err = json.Unmarshal([]byte(tt.wantJSON), &gotBlock)
			if err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if gotBlock.Thinking != tt.block.Thinking {
				t.Errorf("Unmarshal: Thinking = %v, want %v", gotBlock.Thinking, tt.block.Thinking)
			}
			if gotBlock.Signature != tt.block.Signature {
				t.Errorf("Unmarshal: Signature = %v, want %v", gotBlock.Signature, tt.block.Signature)
			}
		})
	}
}

// TestThinkingBlock_JSONArray tests JSON serialization of ThinkingBlock arrays.
func TestThinkingBlock_JSONArray(t *testing.T) {
	tests := []struct {
		name     string
		blocks   []ThinkingBlock
		wantJSON string
	}{
		{
			name: "should serialize array of thinking blocks",
			blocks: []ThinkingBlock{
				{Thinking: "First thought", Signature: "sig_1"},
				{Thinking: "Second thought", Signature: "sig_2"},
			},
			wantJSON: `[{"thinking":"First thought","signature":"sig_1"},{"thinking":"Second thought","signature":"sig_2"}]`,
		},
		{
			name:     "should serialize empty array",
			blocks:   []ThinkingBlock{},
			wantJSON: `[]`,
		},
		{
			name:     "should serialize nil array as null",
			blocks:   nil,
			wantJSON: `null`,
		},
		{
			name: "should serialize single element array",
			blocks: []ThinkingBlock{
				{Thinking: "Only thought", Signature: "sig_only"},
			},
			wantJSON: `[{"thinking":"Only thought","signature":"sig_only"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotJSON, err := json.Marshal(tt.blocks)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if string(gotJSON) != tt.wantJSON {
				t.Errorf("json.Marshal() = %v, want %v", string(gotJSON), tt.wantJSON)
			}
		})
	}
}

// TestMessage_WithThinkingBlocks tests Message creation and manipulation with thinking blocks.
func TestMessage_WithThinkingBlocks(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		content        string
		thinkingBlocks []ThinkingBlock
		wantErr        bool
	}{
		{
			name:    "should create message with single thinking block",
			role:    RoleAssistant,
			content: "Here is my answer",
			thinkingBlocks: []ThinkingBlock{
				{Thinking: "Let me think about this...", Signature: "sig_1"},
			},
			wantErr: false,
		},
		{
			name:    "should create message with multiple thinking blocks",
			role:    RoleAssistant,
			content: "Final response",
			thinkingBlocks: []ThinkingBlock{
				{Thinking: "Step 1: analyze", Signature: "sig_1"},
				{Thinking: "Step 2: plan", Signature: "sig_2"},
				{Thinking: "Step 3: execute", Signature: "sig_3"},
			},
			wantErr: false,
		},
		{
			name:           "should create message with nil thinking blocks",
			role:           RoleAssistant,
			content:        "Response without thinking",
			thinkingBlocks: nil,
			wantErr:        false,
		},
		{
			name:           "should create message with empty thinking blocks array",
			role:           RoleAssistant,
			content:        "Response with empty array",
			thinkingBlocks: []ThinkingBlock{},
			wantErr:        false,
		},
		{
			name:    "should create user message with thinking blocks",
			role:    RoleUser,
			content: "User question",
			thinkingBlocks: []ThinkingBlock{
				{Thinking: "User thinking", Signature: "sig_user"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &Message{
				Role:           tt.role,
				Content:        tt.content,
				Timestamp:      time.Now(),
				ThinkingBlocks: tt.thinkingBlocks,
			}

			// Verify thinking blocks are stored correctly
			if tt.thinkingBlocks == nil && msg.ThinkingBlocks != nil {
				t.Errorf("Expected nil ThinkingBlocks, got %v", msg.ThinkingBlocks)
			}
			if tt.thinkingBlocks != nil {
				if len(msg.ThinkingBlocks) != len(tt.thinkingBlocks) {
					t.Errorf("ThinkingBlocks length = %d, want %d", len(msg.ThinkingBlocks), len(tt.thinkingBlocks))
				}
				for i, block := range tt.thinkingBlocks {
					if msg.ThinkingBlocks[i].Thinking != block.Thinking {
						t.Errorf(
							"ThinkingBlock[%d].Thinking = %v, want %v",
							i,
							msg.ThinkingBlocks[i].Thinking,
							block.Thinking,
						)
					}
					if msg.ThinkingBlocks[i].Signature != block.Signature {
						t.Errorf(
							"ThinkingBlock[%d].Signature = %v, want %v",
							i,
							msg.ThinkingBlocks[i].Signature,
							block.Signature,
						)
					}
				}
			}
		})
	}
}

// TestMessage_ThinkingBlocksJSONIntegration tests JSON serialization of Message with ThinkingBlocks.
func TestMessage_ThinkingBlocksJSONIntegration(t *testing.T) {
	tests := []struct {
		name    string
		message Message
		wantErr bool
	}{
		{
			name: "should serialize message with thinking blocks",
			message: Message{
				Role:      RoleAssistant,
				Content:   "My response",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				ThinkingBlocks: []ThinkingBlock{
					{Thinking: "Analysis step 1", Signature: "sig_1"},
					{Thinking: "Analysis step 2", Signature: "sig_2"},
				},
			},
			wantErr: false,
		},
		{
			name: "should serialize message with nil thinking blocks",
			message: Message{
				Role:           RoleAssistant,
				Content:        "Response",
				Timestamp:      time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				ThinkingBlocks: nil,
			},
			wantErr: false,
		},
		{
			name: "should serialize message with empty thinking blocks",
			message: Message{
				Role:           RoleAssistant,
				Content:        "Response",
				Timestamp:      time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				ThinkingBlocks: []ThinkingBlock{},
			},
			wantErr: false,
		},
		{
			name: "should serialize complete message with all fields",
			message: Message{
				Role:      RoleAssistant,
				Content:   "Complete response",
				Timestamp: time.Date(2024, 1, 1, 12, 0, 0, 0, time.UTC),
				ToolCalls: []ToolCall{
					{ToolID: "tool_1", ToolName: "read_file", Input: map[string]interface{}{"path": "/test"}},
				},
				ThinkingBlocks: []ThinkingBlock{
					{Thinking: "Before tool call", Signature: "sig_before"},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test Marshal
			gotJSON, err := json.Marshal(tt.message)
			if (err != nil) != tt.wantErr {
				t.Fatalf("json.Marshal() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}

			// Test Unmarshal
			var gotMessage Message
			err = json.Unmarshal(gotJSON, &gotMessage)
			if err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}

			// Verify thinking blocks round-trip correctly
			if len(gotMessage.ThinkingBlocks) != len(tt.message.ThinkingBlocks) {
				t.Errorf(
					"Unmarshal: ThinkingBlocks length = %d, want %d",
					len(gotMessage.ThinkingBlocks),
					len(tt.message.ThinkingBlocks),
				)
			}
			for i, block := range tt.message.ThinkingBlocks {
				if i >= len(gotMessage.ThinkingBlocks) {
					break
				}
				if gotMessage.ThinkingBlocks[i].Thinking != block.Thinking {
					t.Errorf(
						"Unmarshal: ThinkingBlock[%d].Thinking = %v, want %v",
						i,
						gotMessage.ThinkingBlocks[i].Thinking,
						block.Thinking,
					)
				}
				if gotMessage.ThinkingBlocks[i].Signature != block.Signature {
					t.Errorf(
						"Unmarshal: ThinkingBlock[%d].Signature = %v, want %v",
						i,
						gotMessage.ThinkingBlocks[i].Signature,
						block.Signature,
					)
				}
			}
		})
	}
}

// TestMessage_ThinkingBlocksEdgeCases tests edge cases for thinking blocks.
func TestMessage_ThinkingBlocksEdgeCases(t *testing.T) {
	tests := []struct {
		name           string
		thinkingBlocks []ThinkingBlock
		description    string
	}{
		{
			name: "should handle thinking block with very long text",
			thinkingBlocks: []ThinkingBlock{
				{
					Thinking:  string(make([]byte, 10000)),
					Signature: "sig_long",
				},
			},
			description: "10KB of thinking text",
		},
		{
			name: "should handle thinking block with unicode characters",
			thinkingBlocks: []ThinkingBlock{
				{
					Thinking:  "思考: 这是一个测试。Réflexion: Ceci est un test. 🤔💭",
					Signature: "sig_unicode",
				},
			},
			description: "Unicode and emoji characters",
		},
		{
			name: "should handle thinking block with escape sequences",
			thinkingBlocks: []ThinkingBlock{
				{
					Thinking:  "Line 1\nLine 2\tTabbed\r\nWindows line",
					Signature: "sig_escape",
				},
			},
			description: "Newlines, tabs, carriage returns",
		},
		{
			name: "should handle many thinking blocks",
			thinkingBlocks: func() []ThinkingBlock {
				blocks := make([]ThinkingBlock, 100)
				for i := range 100 {
					blocks[i] = ThinkingBlock{
						Thinking:  "Thought number " + string(rune(i)),
						Signature: "sig_" + string(rune(i)),
					}
				}
				return blocks
			}(),
			description: "100 thinking blocks",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &Message{
				Role:           RoleAssistant,
				Content:        "Response",
				Timestamp:      time.Now(),
				ThinkingBlocks: tt.thinkingBlocks,
			}

			// Test JSON round-trip
			jsonData, err := json.Marshal(msg)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v for %s", err, tt.description)
			}

			var gotMsg Message
			err = json.Unmarshal(jsonData, &gotMsg)
			if err != nil {
				t.Fatalf("json.Unmarshal() error = %v for %s", err, tt.description)
			}

			// Verify thinking blocks survived round-trip
			if len(gotMsg.ThinkingBlocks) != len(tt.thinkingBlocks) {
				t.Errorf(
					"Round-trip: ThinkingBlocks length = %d, want %d for %s",
					len(gotMsg.ThinkingBlocks),
					len(tt.thinkingBlocks),
					tt.description,
				)
			}
		})
	}
}

// TestNewMessageWithThinkingBlocks tests a constructor for creating messages with thinking blocks.
func TestNewMessageWithThinkingBlocks(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		content        string
		thinkingBlocks []ThinkingBlock
		wantErr        bool
		expectedError  error
	}{
		{
			name:    "should create assistant message with thinking blocks",
			role:    RoleAssistant,
			content: "My answer",
			thinkingBlocks: []ThinkingBlock{
				{Thinking: "Analysis", Signature: "sig_1"},
			},
			wantErr: false,
		},
		{
			name:           "should create message with nil thinking blocks",
			role:           RoleAssistant,
			content:        "Answer",
			thinkingBlocks: nil,
			wantErr:        false,
		},
		{
			name:           "should reject empty role",
			role:           "",
			content:        "Content",
			thinkingBlocks: []ThinkingBlock{{Thinking: "Think", Signature: "sig"}},
			wantErr:        true,
			expectedError:  ErrEmptyRole,
		},
		{
			name:           "should reject empty content with no thinking blocks",
			role:           RoleAssistant,
			content:        "",
			thinkingBlocks: nil,
			wantErr:        true,
			expectedError:  ErrEmptyContent,
		},
		{
			name:    "should allow empty content if thinking blocks present",
			role:    RoleAssistant,
			content: "",
			thinkingBlocks: []ThinkingBlock{
				{Thinking: "Just thinking, no response yet", Signature: "sig_thinking"},
			},
			wantErr: false,
		},
		{
			name:    "should reject whitespace-only content even with thinking blocks",
			role:    RoleAssistant,
			content: "   ",
			thinkingBlocks: []ThinkingBlock{
				{Thinking: "Some analysis", Signature: "sig_whitespace"},
			},
			wantErr:       true,
			expectedError: ErrInvalidContent,
		},
		{
			name:    "should reject whitespace content with multiple thinking blocks",
			role:    RoleAssistant,
			content: "\t\n  \n",
			thinkingBlocks: []ThinkingBlock{
				{Thinking: "Step 1", Signature: "sig_1"},
				{Thinking: "Step 2", Signature: "sig_2"},
			},
			wantErr:       true,
			expectedError: ErrInvalidContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewMessageWithThinkingBlocks(tt.role, tt.content, tt.thinkingBlocks)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMessageWithThinkingBlocks() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if tt.expectedError != nil && !errors.Is(err, tt.expectedError) {
					t.Errorf("NewMessageWithThinkingBlocks() error = %v, expectedError %v", err, tt.expectedError)
				}
				return
			}

			if got == nil {
				t.Fatal("NewMessageWithThinkingBlocks() returned nil message")
			}
			if got.Role != tt.role {
				t.Errorf("NewMessageWithThinkingBlocks() role = %v, want %v", got.Role, tt.role)
			}
			if got.Content != tt.content {
				t.Errorf("NewMessageWithThinkingBlocks() content = %v, want %v", got.Content, tt.content)
			}
			if len(got.ThinkingBlocks) != len(tt.thinkingBlocks) {
				t.Errorf(
					"NewMessageWithThinkingBlocks() thinking blocks length = %d, want %d",
					len(got.ThinkingBlocks),
					len(tt.thinkingBlocks),
				)
			}
			if got.Timestamp.IsZero() {
				t.Error("NewMessageWithThinkingBlocks() timestamp should not be zero")
			}
		})
	}
}

// TestMessage_Validate_ThinkingBlockBug tests the validation bug where Validate() doesn't consider ThinkingBlocks
// when checking if empty content is allowed. This test suite exposes the bug where hasToolContent() only checks
// ToolCalls and ToolResults but ignores ThinkingBlocks.
func TestMessage_Validate_ThinkingBlockBug(t *testing.T) {
	tests := []struct {
		name           string
		message        Message
		wantErr        bool
		expectedError  error
		bugDescription string
	}{
		{
			name: "should pass validation for empty content with thinking blocks",
			message: Message{
				Role:      RoleAssistant,
				Content:   "",
				Timestamp: time.Now(),
				ThinkingBlocks: []ThinkingBlock{
					{Thinking: "Let me analyze this problem step by step...", Signature: "sig_analysis"},
				},
			},
			wantErr:        false,
			bugDescription: "Bug: Validate() returns ErrEmptyContent even though ThinkingBlocks are present",
		},
		{
			name: "should pass validation for empty content with multiple thinking blocks",
			message: Message{
				Role:      RoleAssistant,
				Content:   "",
				Timestamp: time.Now(),
				ThinkingBlocks: []ThinkingBlock{
					{Thinking: "Step 1: Understand the requirements", Signature: "sig_1"},
					{Thinking: "Step 2: Design the solution", Signature: "sig_2"},
					{Thinking: "Step 3: Consider edge cases", Signature: "sig_3"},
				},
			},
			wantErr:        false,
			bugDescription: "Bug: Validate() fails for multiple ThinkingBlocks with empty content",
		},
		{
			name: "should fail validation for whitespace content even with thinking blocks",
			message: Message{
				Role:      RoleAssistant,
				Content:   "   ",
				Timestamp: time.Now(),
				ThinkingBlocks: []ThinkingBlock{
					{Thinking: "Analysis complete", Signature: "sig_whitespace"},
				},
			},
			wantErr:        true,
			expectedError:  ErrInvalidContent,
			bugDescription: "Whitespace-only content should still be invalid even with ThinkingBlocks",
		},
		{
			name: "should fail validation for whitespace content with multiple thinking blocks",
			message: Message{
				Role:      RoleAssistant,
				Content:   "\t\n  \r\n",
				Timestamp: time.Now(),
				ThinkingBlocks: []ThinkingBlock{
					{Thinking: "Thought 1", Signature: "sig_1"},
					{Thinking: "Thought 2", Signature: "sig_2"},
				},
			},
			wantErr:        true,
			expectedError:  ErrInvalidContent,
			bugDescription: "Various whitespace characters should be invalid even with ThinkingBlocks",
		},
		{
			name: "should fail validation for empty content with nil thinking blocks",
			message: Message{
				Role:           RoleAssistant,
				Content:        "",
				Timestamp:      time.Now(),
				ThinkingBlocks: nil,
			},
			wantErr:        true,
			expectedError:  ErrEmptyContent,
			bugDescription: "Empty content with nil ThinkingBlocks should correctly fail",
		},
		{
			name: "should fail validation for empty content with empty thinking blocks array",
			message: Message{
				Role:           RoleAssistant,
				Content:        "",
				Timestamp:      time.Now(),
				ThinkingBlocks: []ThinkingBlock{},
			},
			wantErr:        true,
			expectedError:  ErrEmptyContent,
			bugDescription: "Empty content with empty ThinkingBlocks array should fail",
		},
		{
			name: "should pass validation for message with both content and thinking blocks",
			message: Message{
				Role:      RoleAssistant,
				Content:   "Here is my analysis",
				Timestamp: time.Now(),
				ThinkingBlocks: []ThinkingBlock{
					{Thinking: "I considered multiple approaches", Signature: "sig_combined"},
				},
			},
			wantErr:        false,
			bugDescription: "Valid content with ThinkingBlocks should pass",
		},
		{
			name: "should pass validation for user message with empty content and thinking blocks",
			message: Message{
				Role:      RoleUser,
				Content:   "",
				Timestamp: time.Now(),
				ThinkingBlocks: []ThinkingBlock{
					{Thinking: "User internal thought process", Signature: "sig_user"},
				},
			},
			wantErr:        false,
			bugDescription: "Bug: User messages with ThinkingBlocks should also support empty content",
		},
		{
			name: "should pass validation for system message with empty content and thinking blocks",
			message: Message{
				Role:      RoleSystem,
				Content:   "",
				Timestamp: time.Now(),
				ThinkingBlocks: []ThinkingBlock{
					{Thinking: "System reasoning", Signature: "sig_system"},
				},
			},
			wantErr:        false,
			bugDescription: "Bug: System messages with ThinkingBlocks should support empty content",
		},
		{
			name: "should pass validation for thinking block with empty thinking text but valid signature",
			message: Message{
				Role:      RoleAssistant,
				Content:   "",
				Timestamp: time.Now(),
				ThinkingBlocks: []ThinkingBlock{
					{Thinking: "", Signature: "sig_empty_thinking"},
				},
			},
			wantErr:        false,
			bugDescription: "Bug: ThinkingBlock presence should allow empty content even if thinking is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.message.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf(
					"Message.Validate() error = %v, wantErr %v\nBug: %s",
					err,
					tt.wantErr,
					tt.bugDescription,
				)
			}
			if tt.wantErr && tt.expectedError != nil && !errors.Is(err, tt.expectedError) {
				t.Errorf(
					"Message.Validate() error = %v, expectedError %v\nBug: %s",
					err,
					tt.expectedError,
					tt.bugDescription,
				)
			}
		})
	}
}

// TestMessage_Validate_ThinkingBlocksVsToolContent tests that ThinkingBlocks should be treated
// equivalently to ToolCalls/ToolResults for content validation purposes.
func TestMessage_Validate_ThinkingBlocksVsToolContent(t *testing.T) {
	timestamp := time.Now()

	tests := []struct {
		name    string
		message Message
		wantErr bool
	}{
		{
			name: "empty content with tool calls should pass",
			message: Message{
				Role:      RoleAssistant,
				Content:   "",
				Timestamp: timestamp,
				ToolCalls: []ToolCall{
					{ToolID: "tool_1", ToolName: "test_tool", Input: map[string]interface{}{"key": "value"}},
				},
			},
			wantErr: false,
		},
		{
			name: "empty content with tool results should pass",
			message: Message{
				Role:      RoleUser,
				Content:   "",
				Timestamp: timestamp,
				ToolResults: []ToolResult{
					{ToolID: "tool_1", Result: "success", IsError: false},
				},
			},
			wantErr: false,
		},
		{
			name: "empty content with thinking blocks should pass (BUG)",
			message: Message{
				Role:      RoleAssistant,
				Content:   "",
				Timestamp: timestamp,
				ThinkingBlocks: []ThinkingBlock{
					{Thinking: "Reasoning process", Signature: "sig_reasoning"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty content with all three types should pass",
			message: Message{
				Role:      RoleAssistant,
				Content:   "",
				Timestamp: timestamp,
				ToolCalls: []ToolCall{
					{ToolID: "tool_1", ToolName: "test", Input: map[string]interface{}{}},
				},
				ThinkingBlocks: []ThinkingBlock{
					{Thinking: "Thinking before tool call", Signature: "sig_before"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty content with no tools, results, or thinking should fail",
			message: Message{
				Role:      RoleAssistant,
				Content:   "",
				Timestamp: timestamp,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.message.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Message.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

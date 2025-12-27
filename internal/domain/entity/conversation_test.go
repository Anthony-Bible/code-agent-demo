package entity

import (
	"testing"
	"time"
)

func TestConversation_NewConversation(t *testing.T) {
	tests := []struct {
		name    string
		want    *Conversation
		wantErr bool
	}{
		{
			name:    "should create empty conversation successfully",
			want:    &Conversation{Messages: []Message{}, StartedAt: time.Now()},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewConversation()
			if (err != nil) != tt.wantErr {
				t.Errorf("NewConversation() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got == nil {
				t.Error("NewConversation() returned nil conversation")
				return
			}
			if len(got.Messages) != 0 {
				t.Errorf("NewConversation() messages length = %v, want %v", len(got.Messages), 0)
			}
		})
	}
}

func TestConversation_AddMessage(t *testing.T) {
	type fields struct {
		Messages []Message
	}
	type args struct {
		message Message
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:    "should add user message to empty conversation",
			fields:  fields{Messages: []Message{}},
			args:    args{message: Message{Role: "user", Content: "Hello"}},
			wantErr: false,
		},
		{
			name:    "should add assistant message to conversation",
			fields:  fields{Messages: []Message{}},
			args:    args{message: Message{Role: "assistant", Content: "Hi there!"}},
			wantErr: false,
		},
		{
			name:    "should add multiple messages in sequence",
			fields:  fields{Messages: []Message{{Role: "user", Content: "Hello"}}},
			args:    args{message: Message{Role: "assistant", Content: "Hi there!"}},
			wantErr: false,
		},
		{
			name:    "should reject message with empty content",
			fields:  fields{Messages: []Message{}},
			args:    args{message: Message{Role: "user", Content: ""}},
			wantErr: true,
		},
		{
			name:    "should reject message with invalid role",
			fields:  fields{Messages: []Message{}},
			args:    args{message: Message{Role: "invalid", Content: "Hello"}},
			wantErr: true,
		},
		{
			name:    "should reject message with empty role",
			fields:  fields{Messages: []Message{}},
			args:    args{message: Message{Role: "", Content: "Hello"}},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conversation{
				Messages: tt.fields.Messages,
			}
			beforeLen := len(c.Messages)
			err := c.AddMessage(tt.args.message)
			if (err != nil) != tt.wantErr {
				t.Errorf("Conversation.AddMessage() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr {
				if len(c.Messages) != beforeLen+1 {
					t.Errorf("Conversation.AddMessage() messages length before = %v, after = %v, want %v",
						beforeLen, len(c.Messages), beforeLen+1)
				}
				lastMessage := c.Messages[len(c.Messages)-1]
				if lastMessage.Role != tt.args.message.Role {
					t.Errorf("Conversation.AddMessage() last message role = %v, want %v",
						lastMessage.Role, tt.args.message.Role)
				}
				if lastMessage.Content != tt.args.message.Content {
					t.Errorf("Conversation.AddMessage() last message content = %v, want %v",
						lastMessage.Content, tt.args.message.Content)
				}
			} else if len(c.Messages) != beforeLen {
				t.Errorf("Conversation.AddMessage() should not add message on error, length before = %v, after = %v",
					beforeLen, len(c.Messages))
			}
		})
	}
}

func TestConversation_GetMessages(t *testing.T) {
	type fields struct {
		Messages []Message
	}
	tests := []struct {
		name   string
		fields fields
		want   []Message
	}{
		{
			name:   "should return empty slice for empty conversation",
			fields: fields{Messages: []Message{}},
			want:   []Message{},
		},
		{
			name: "should return all messages in conversation",
			fields: fields{Messages: []Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "How are you?"},
			}},
			want: []Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "How are you?"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conversation{
				Messages: tt.fields.Messages,
			}
			got := c.GetMessages()
			if len(got) != len(tt.want) {
				t.Errorf("Conversation.GetMessages() returned %v messages, want %v",
					len(got), len(tt.want))
				return
			}
			for i := range got {
				if got[i].Role != tt.want[i].Role {
					t.Errorf("Conversation.GetMessages() message %d role = %v, want %v",
						i, got[i].Role, tt.want[i].Role)
				}
				if got[i].Content != tt.want[i].Content {
					t.Errorf("Conversation.GetMessages() message %d content = %v, want %v",
						i, got[i].Content, tt.want[i].Content)
				}
			}
		})
	}
}

func TestConversation_GetLastMessage(t *testing.T) {
	type fields struct {
		Messages []Message
	}
	tests := []struct {
		name      string
		fields    fields
		want      *Message
		wantFound bool
	}{
		{
			name:      "should return not found for empty conversation",
			fields:    fields{Messages: []Message{}},
			want:      nil,
			wantFound: false,
		},
		{
			name:      "should return last message for single message conversation",
			fields:    fields{Messages: []Message{{Role: "user", Content: "Hello"}}},
			want:      &Message{Role: "user", Content: "Hello"},
			wantFound: true,
		},
		{
			name: "should return last message for multiple message conversation",
			fields: fields{Messages: []Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "How are you?"},
			}},
			want:      &Message{Role: "user", Content: "How are you?"},
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conversation{
				Messages: tt.fields.Messages,
			}
			got, found := c.GetLastMessage()
			if found != tt.wantFound {
				t.Errorf("Conversation.GetLastMessage() found = %v, wantFound %v", found, tt.wantFound)
				return
			}

			if tt.wantFound {
				if got == nil {
					t.Error("Conversation.GetLastMessage() returned nil message when expected")
					return
				}
				if got.Role != tt.want.Role {
					t.Errorf("Conversation.GetLastMessage() role = %v, want %v", got.Role, tt.want.Role)
				}
				if got.Content != tt.want.Content {
					t.Errorf("Conversation.GetLastMessage() content = %v, want %v", got.Content, tt.want.Content)
				}
			} else if got != nil {
				t.Errorf("Conversation.GetLastMessage() returned non-nil message when not expected: %+v", got)
			}
		})
	}
}

func TestConversation_Clear(t *testing.T) {
	type fields struct {
		Messages []Message
	}
	tests := []struct {
		name   string
		fields fields
	}{
		{
			name:   "should clear empty conversation",
			fields: fields{Messages: []Message{}},
		},
		{
			name: "should clear conversation with messages",
			fields: fields{Messages: []Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
			}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conversation{
				Messages: tt.fields.Messages,
			}
			c.Clear()
			if len(c.Messages) != 0 {
				t.Errorf("Conversation.Clear() messages length = %v, want 0", len(c.Messages))
			}
		})
	}
}

func TestConversation_MessageCount(t *testing.T) {
	type fields struct {
		Messages []Message
	}
	tests := []struct {
		name   string
		fields fields
		want   int
	}{
		{
			name:   "should return 0 for empty conversation",
			fields: fields{Messages: []Message{}},
			want:   0,
		},
		{
			name:   "should return 1 for single message conversation",
			fields: fields{Messages: []Message{{Role: "user", Content: "Hello"}}},
			want:   1,
		},
		{
			name: "should return 3 for three message conversation",
			fields: fields{Messages: []Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there!"},
				{Role: "user", Content: "How are you?"},
			}},
			want: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Conversation{
				Messages: tt.fields.Messages,
			}
			got := c.MessageCount()
			if got != tt.want {
				t.Errorf("Conversation.MessageCount() = %v, want %v", got, tt.want)
			}
		})
	}
}

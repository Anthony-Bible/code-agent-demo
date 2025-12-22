package entity

import (
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
			} else {
				if got != nil {
					t.Errorf("NewMessage() returned non-nil message on error: %+v", got)
				}
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

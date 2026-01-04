package subagent

import (
	"errors"
	"testing"
)

func TestValidateAgentName(t *testing.T) {
	tests := []struct {
		name      string
		agentName string
		wantErr   error
	}{
		// Valid names
		{name: "valid simple name", agentName: "test-agent", wantErr: nil},
		{name: "valid with numbers", agentName: "agent123", wantErr: nil},
		{name: "valid all lowercase", agentName: "myagent", wantErr: nil},
		{name: "valid with hyphen", agentName: "my-cool-agent", wantErr: nil},
		{name: "valid single char", agentName: "a", wantErr: nil},
		{
			name:      "valid 64 chars",
			agentName: "a123456789012345678901234567890123456789012345678901234567890123",
			wantErr:   nil,
		},

		// Path traversal attempts
		{name: "path traversal with ../", agentName: "../etc/passwd", wantErr: ErrInvalidAgentName},
		{name: "path traversal with ..", agentName: "..agent", wantErr: ErrInvalidAgentName},
		{name: "path traversal with /", agentName: "agent/subdir", wantErr: ErrInvalidAgentName},
		{name: "absolute path", agentName: "/etc/passwd", wantErr: ErrInvalidAgentName},
		{name: "windows path", agentName: "C:\\Windows", wantErr: ErrInvalidAgentName},
		{name: "backslash", agentName: "agent\\name", wantErr: ErrInvalidAgentName},
		{name: "null byte", agentName: "agent\x00name", wantErr: ErrInvalidAgentName},

		// Invalid characters
		{name: "uppercase letters", agentName: "MyAgent", wantErr: ErrInvalidAgentName},
		{name: "spaces", agentName: "my agent", wantErr: ErrInvalidAgentName},
		{name: "underscore", agentName: "my_agent", wantErr: ErrInvalidAgentName},
		{name: "dot", agentName: "my.agent", wantErr: ErrInvalidAgentName},
		{name: "special char @", agentName: "agent@name", wantErr: ErrInvalidAgentName},
		{name: "special char #", agentName: "agent#1", wantErr: ErrInvalidAgentName},
		{name: "special char $", agentName: "agent$", wantErr: ErrInvalidAgentName},

		// Hyphen rules
		{name: "starts with hyphen", agentName: "-agent", wantErr: ErrAgentNameHyphen},
		{name: "ends with hyphen", agentName: "agent-", wantErr: ErrAgentNameHyphen},
		{name: "consecutive hyphens", agentName: "my--agent", wantErr: ErrAgentNameConsecHyphen},
		{name: "triple consecutive hyphens", agentName: "my---agent", wantErr: ErrAgentNameConsecHyphen},

		// Length and empty
		{name: "empty name", agentName: "", wantErr: ErrAgentNameEmpty},
		{
			name:      "too long name (65 chars)",
			agentName: "a1234567890123456789012345678901234567890123456789012345678901234",
			wantErr:   ErrAgentNameTooLong,
		},
		{
			name:      "too long name (100 chars)",
			agentName: "a123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789",
			wantErr:   ErrAgentNameTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgentName(tt.agentName)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("validateAgentName(%q) unexpected error: %v", tt.agentName, err)
				}
			} else {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("validateAgentName(%q) error = %v, want %v", tt.agentName, err, tt.wantErr)
				}
			}
		})
	}
}

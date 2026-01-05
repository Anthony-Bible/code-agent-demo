package subagent

import (
	"testing"
)

func TestValidateAgentName(t *testing.T) {
	tests := []struct {
		name      string
		agentName string
		wantErr   bool
	}{
		// Valid names
		{name: "valid simple name", agentName: "test-agent", wantErr: false},
		{name: "valid with numbers", agentName: "agent123", wantErr: false},
		{name: "valid all lowercase", agentName: "myagent", wantErr: false},
		{name: "valid with hyphen", agentName: "my-cool-agent", wantErr: false},
		{name: "valid single char", agentName: "a", wantErr: false},
		{
			name:      "valid 64 chars",
			agentName: "a123456789012345678901234567890123456789012345678901234567890123",
			wantErr:   false,
		},

		// Path traversal attempts
		{name: "path traversal with ../", agentName: "../etc/passwd", wantErr: true},
		{name: "path traversal with ..", agentName: "..agent", wantErr: true},
		{name: "path traversal with /", agentName: "agent/subdir", wantErr: true},
		{name: "absolute path", agentName: "/etc/passwd", wantErr: true},
		{name: "windows path", agentName: "C:\\Windows", wantErr: true},
		{name: "backslash", agentName: "agent\\name", wantErr: true},
		{name: "null byte", agentName: "agent\x00name", wantErr: true},

		// Invalid characters
		{name: "uppercase letters", agentName: "MyAgent", wantErr: true},
		{name: "spaces", agentName: "my agent", wantErr: true},
		{name: "underscore", agentName: "my_agent", wantErr: true},
		{name: "dot", agentName: "my.agent", wantErr: true},
		{name: "special char @", agentName: "agent@name", wantErr: true},
		{name: "special char #", agentName: "agent#1", wantErr: true},
		{name: "special char $", agentName: "agent$", wantErr: true},

		// Hyphen rules
		{name: "starts with hyphen", agentName: "-agent", wantErr: true},
		{name: "ends with hyphen", agentName: "agent-", wantErr: true},
		{name: "consecutive hyphens", agentName: "my--agent", wantErr: true},
		{name: "triple consecutive hyphens", agentName: "my---agent", wantErr: true},

		// Length and empty
		{name: "empty name", agentName: "", wantErr: true},
		{
			name:      "too long name (65 chars)",
			agentName: "a1234567890123456789012345678901234567890123456789012345678901234",
			wantErr:   true,
		},
		{
			name:      "too long name (100 chars)",
			agentName: "a123456789012345678901234567890123456789012345678901234567890123456789012345678901234567890123456789",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAgentName(tt.agentName)
			if tt.wantErr && err == nil {
				t.Errorf("validateAgentName(%q) expected error but got nil", tt.agentName)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("validateAgentName(%q) unexpected error: %v", tt.agentName, err)
			}
		})
	}
}

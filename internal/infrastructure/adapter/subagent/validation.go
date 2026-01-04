package subagent

import (
	"code-editing-agent/internal/domain/entity"
	"errors"
)

var (
	ErrAgentNotFound          = errors.New("agent not found")
	ErrAgentFileNotFound      = errors.New("AGENT.md file not found in agent directory")
	ErrAgentAlreadyRegistered = errors.New("agent is already registered")
	ErrInvalidAgent           = errors.New("invalid agent: agent cannot be nil")
)

// validateAgentName validates an agent name to prevent path traversal attacks.
// This is a security boundary - validates the name parameter before using it in file paths.
// Uses domain validation rules as the single source of truth.
func validateAgentName(name string) error {
	return entity.ValidateSubagentName(name)
}

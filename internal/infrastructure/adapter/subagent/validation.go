package subagent

import "errors"

var (
	ErrAgentNotFound     = errors.New("agent not found")
	ErrAgentFileNotFound = errors.New("AGENT.md file not found in agent directory")
	ErrInvalidAgentName  = errors.New(
		"invalid agent name: must contain only lowercase letters, numbers, and hyphens",
	)
	ErrAgentNameEmpty         = errors.New("agent name cannot be empty")
	ErrAgentNameTooLong       = errors.New("agent name must be 64 characters or less")
	ErrAgentNameHyphen        = errors.New("agent name cannot start or end with a hyphen")
	ErrAgentNameConsecHyphen  = errors.New("agent name cannot contain consecutive hyphens")
	ErrAgentAlreadyRegistered = errors.New("agent is already registered")
	ErrInvalidAgent           = errors.New("invalid agent: agent cannot be nil")
)

// validateAgentName validates an agent name to prevent path traversal attacks.
// Agent names must match the agentskills.io spec: 1-64 lowercase alphanumeric
// characters and hyphens, cannot start/end with hyphen or have consecutive hyphens.
func validateAgentName(name string) error {
	if name == "" {
		return ErrAgentNameEmpty
	}
	if len(name) > 64 {
		return ErrAgentNameTooLong
	}
	if name[0] == '-' || name[len(name)-1] == '-' {
		return ErrAgentNameHyphen
	}

	prevChar := byte(0)
	for i := range len(name) {
		c := name[i]
		if c == '-' && prevChar == '-' {
			return ErrAgentNameConsecHyphen
		}
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return ErrInvalidAgentName
		}
		prevChar = c
	}
	return nil
}

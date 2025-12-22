package port

import (
	"context"
	"errors"
)

var (
	ErrInvalidPrompt = errors.New("invalid prompt")
	ErrInvalidColor  = errors.New("invalid color scheme")
)

// ColorScheme defines the color configuration for the user interface.
type ColorScheme struct {
	User      string `json:"user"`      // Color for user messages
	Assistant string `json:"assistant"` // Color for assistant messages
	System    string `json:"system"`    // Color for system messages
	Error     string `json:"error"`     // Color for error messages
	Tool      string `json:"tool"`      // Color for tool results
	Prompt    string `json:"prompt"`    // Color for user prompt
}

// UserInterface defines the interface for CLI interactions.
// This port represents the inbound dependency for user interactions and follows
// hexagonal architecture principles by abstracting user interface implementations.
type UserInterface interface {
	// GetUserInput gets input from the user with context support.
	// Returns the input string and a boolean indicating if the conversation should continue.
	GetUserInput(ctx context.Context) (string, bool)

	// DisplayMessage displays a message with the specified role.
	DisplayMessage(message string, messageRole string) error

	// DisplayError displays an error message.
	DisplayError(err error) error

	// DisplayToolResult displays the result of a tool execution.
	DisplayToolResult(toolName string, input string, result string) error

	// DisplaySystemMessage displays a system message.
	DisplaySystemMessage(message string) error

	// SetPrompt sets the user input prompt.
	SetPrompt(prompt string) error

	// ClearScreen clears the terminal screen.
	ClearScreen() error

	// SetColorScheme sets the color scheme for the interface.
	SetColorScheme(scheme ColorScheme) error
}

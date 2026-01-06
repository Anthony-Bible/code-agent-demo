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

	// DisplayStreamingText displays a chunk of streaming text without a newline.
	// This is used to show text as it arrives in real-time from the AI provider.
	DisplayStreamingText(text string) error

	// DisplayError displays an error message.
	DisplayError(err error) error

	// DisplayToolResult displays the result of a tool execution.
	DisplayToolResult(toolName string, input string, result string) error

	// DisplaySystemMessage displays a system message.
	DisplaySystemMessage(message string) error

	// DisplayThinking displays extended thinking content from the AI.
	// Used when extended thinking mode is enabled with ShowThinking flag.
	// The content contains the AI's internal reasoning process before generating a response.
	DisplayThinking(content string) error

	// DisplaySubagentStatus displays a status message for subagent execution.
	// Used to show when subagents start, complete, or execute tools during delegated tasks.
	// Parameters:
	//   - agentName: The name of the subagent (e.g., "test-writer")
	//   - status: Current status (e.g., "Starting", "Completed", "Executing read_file")
	//   - details: Additional details (e.g., "5 actions, 2.3s" or "")
	DisplaySubagentStatus(agentName string, status string, details string) error

	// SetPrompt sets the user input prompt.
	SetPrompt(prompt string) error

	// ClearScreen clears the terminal screen.
	ClearScreen() error

	// SetColorScheme sets the color scheme for the interface.
	SetColorScheme(scheme ColorScheme) error

	// ConfirmBashCommand prompts the user to confirm a bash command before execution.
	// Parameters:
	//   - command: The bash command to be executed
	//   - isDangerous: Whether the command matches dangerous patterns
	//   - reason: If dangerous, describes why (e.g., "destructive rm command"); empty for standard commands
	//   - description: AI's rationale for running the command; displayed before the command when non-empty
	// Returns true if the user confirms execution, false otherwise.
	ConfirmBashCommand(command string, isDangerous bool, reason string, description string) bool
}

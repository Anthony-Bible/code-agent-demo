package ui

import (
	"bufio"
	"code-editing-agent/internal/domain/port"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// CLIAdapter implements the UserInterface port using the command line.
type CLIAdapter struct {
	input            io.Reader
	output           io.Writer
	prompt           string
	colors           port.ColorScheme
	scanner          *bufio.Scanner
	truncationConfig TruncationConfig
}

// defaultColorScheme returns the default ANSI color scheme for CLI output.
func defaultColorScheme() port.ColorScheme {
	return port.ColorScheme{
		User:      "\x1b[94m", // Blue
		Assistant: "\x1b[93m", // Yellow
		System:    "\x1b[96m", // Cyan
		Error:     "\x1b[91m", // Red
		Tool:      "\x1b[92m", // Green
		Prompt:    "\x1b[94m", // Blue
	}
}

// NewCLIAdapter creates a new CLIAdapter with default I/O (stdin/stdout).
func NewCLIAdapter() *CLIAdapter {
	return &CLIAdapter{
		input:            os.Stdin,
		output:           os.Stdout,
		prompt:           "> ",
		colors:           defaultColorScheme(),
		truncationConfig: DefaultTruncationConfig(),
	}
}

// NewCLIAdapterWithIO creates a new CLIAdapter with custom I/O for testing.
func NewCLIAdapterWithIO(input io.Reader, output io.Writer) *CLIAdapter {
	return &CLIAdapter{
		input:            input,
		output:           output,
		prompt:           "> ",
		colors:           defaultColorScheme(),
		truncationConfig: DefaultTruncationConfig(),
	}
}

// GetUserInput gets input from the user with context support.
func (c *CLIAdapter) GetUserInput(ctx context.Context) (string, bool) {
	if c.scanner == nil {
		c.scanner = bufio.NewScanner(c.input)
	}

	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return "", false
	default:
		// Continue
	}

	// Display prompt
	if _, err := fmt.Fprint(c.output, c.colors.Prompt+"Claude"+c.colors.Prompt+": "); err != nil {
		return "", false
	}

	// Read input
	if !c.scanner.Scan() {
		// EOF reached
		return "", false
	}

	input := c.scanner.Text()
	return input, true
}

// DisplayMessage displays a message with the specified role.
func (c *CLIAdapter) DisplayMessage(message string, messageRole string) error {
	var color string

	switch strings.ToLower(messageRole) {
	case "user":
		color = c.colors.User
	case "assistant":
		color = c.colors.Assistant
	case "system":
		color = c.colors.System
	default:
		// Default to user color for unknown roles
		color = c.colors.User
	}

	_, err := fmt.Fprintf(c.output, "%s%s\x1b[0m\n", color, message)
	return err
}

// DisplayError displays an error message.
func (c *CLIAdapter) DisplayError(err error) error {
	if err == nil {
		return nil
	}

	_, writeErr := fmt.Fprintf(c.output, "%sError: %s\x1b[0m\n", c.colors.Error, err.Error())
	if writeErr != nil {
		return writeErr
	}
	return nil
}

// DisplayToolResult displays the result of a tool execution.
// Large outputs are automatically truncated according to the truncation configuration.
//
// The bash tool receives special handling: its JSON output (containing stdout, stderr,
// and exit_code fields) is parsed, and stdout/stderr are truncated independently
// before the JSON is reassembled. Other tools use plain text truncation.
func (c *CLIAdapter) DisplayToolResult(toolName string, input string, result string) error {
	truncatedResult := c.truncateToolOutput(toolName, result)

	_, err := fmt.Fprintf(c.output, "%sTool [%s] on %s\x1b[0m\n%s\x1b[0m\n",
		c.colors.Tool, toolName, input, truncatedResult)
	return err
}

// DisplaySystemMessage displays a system message.
func (c *CLIAdapter) DisplaySystemMessage(message string) error {
	_, err := fmt.Fprintf(c.output, "%sSystem: %s\x1b[0m\n", c.colors.System, message)
	return err
}

// SetPrompt sets the user input prompt.
func (c *CLIAdapter) SetPrompt(prompt string) error {
	if prompt == "" {
		return port.ErrInvalidPrompt
	}
	c.prompt = prompt
	return nil
}

// ClearScreen clears the terminal screen.
func (c *CLIAdapter) ClearScreen() error {
	// ANSI clear screen and move cursor to top-left
	_, err := fmt.Fprintf(c.output, "\x1b[2J\x1b[H")
	return err
}

// SetColorScheme sets the color scheme for the interface.
func (c *CLIAdapter) SetColorScheme(scheme port.ColorScheme) error {
	// Basic validation - ensure at least one color is set
	if scheme.User == "" && scheme.Assistant == "" && scheme.System == "" &&
		scheme.Error == "" && scheme.Tool == "" && scheme.Prompt == "" {
		return port.ErrInvalidColor
	}

	// Only set non-empty fields (partial scheme support)
	if scheme.User != "" {
		c.colors.User = scheme.User
	}
	if scheme.Assistant != "" {
		c.colors.Assistant = scheme.Assistant
	}
	if scheme.System != "" {
		c.colors.System = scheme.System
	}
	if scheme.Error != "" {
		c.colors.Error = scheme.Error
	}
	if scheme.Tool != "" {
		c.colors.Tool = scheme.Tool
	}
	if scheme.Prompt != "" {
		c.colors.Prompt = scheme.Prompt
	}

	return nil
}

// truncateToolOutput applies the appropriate truncation strategy based on tool type.
// Bash tool output uses JSON-aware truncation; other tools use plain text truncation.
func (c *CLIAdapter) truncateToolOutput(toolName, result string) string {
	if toolName == "bash" {
		truncated, _ := TruncateBashOutput(result, c.truncationConfig)
		return truncated
	}
	truncated, _ := TruncateOutput(result, c.truncationConfig)
	return truncated
}

// SetTruncationConfig sets the truncation configuration for tool output display.
// The configuration controls how large outputs are truncated when displayed via
// DisplayToolResult. This allows preserving the beginning (head) and end (tail)
// of output while omitting the middle section for readability.
//
// Changes take effect immediately for subsequent DisplayToolResult calls.
// Pass a config with Enabled=false to disable truncation entirely.
func (c *CLIAdapter) SetTruncationConfig(config TruncationConfig) {
	c.truncationConfig = config
}

// GetTruncationConfig returns the current truncation configuration.
// The returned value is a copy; modifying it does not affect the adapter's
// internal configuration. Use SetTruncationConfig to apply changes.
//
// New adapters are initialized with DefaultTruncationConfig values:
// HeadLines=20, TailLines=10, Enabled=true.
func (c *CLIAdapter) GetTruncationConfig() TruncationConfig {
	return c.truncationConfig
}

// ConfirmBashCommand prompts the user to confirm a bash command before execution.
// It displays the command with appropriate styling and waits for user input.
//
// Parameters:
//   - command: The bash command to be confirmed
//   - isDangerous: If true, displays a red warning header instead of standard cyan
//   - reason: Explanation shown with dangerous command warnings (ignored if not dangerous)
//   - description: Optional description displayed above the command
//
// Returns true only if the user enters "y" or "yes" (case-insensitive).
// Returns false for any other input, empty input, or EOF (safe default).
func (c *CLIAdapter) ConfirmBashCommand(command string, isDangerous bool, reason string, description string) bool {
	// Display header based on danger level
	if isDangerous {
		fmt.Fprintf(c.output, "%s[DANGEROUS COMMAND] %s\x1b[0m\n", c.colors.Error, reason)
	}
	// Display description if provided
	if description != "" {
		fmt.Fprintf(c.output, "%s\x1b[0m\n", description)
	}
	// Display standard prefix for non-dangerous commands
	if !isDangerous {
		fmt.Fprintf(c.output, "%s[BASH COMMAND]\x1b[0m\n", c.colors.System)
	}

	// Display command in green with indentation
	fmt.Fprintf(c.output, "  %s%s\x1b[0m\n", c.colors.Tool, command)

	// Display confirmation prompt
	fmt.Fprint(c.output, "Execute? [y/N]: ")

	// Initialize scanner if needed
	if c.scanner == nil {
		c.scanner = bufio.NewScanner(c.input)
	}

	// Read user input
	if !c.scanner.Scan() {
		return false
	}

	input := strings.TrimSpace(strings.ToLower(c.scanner.Text()))
	return input == "y" || input == "yes"
}

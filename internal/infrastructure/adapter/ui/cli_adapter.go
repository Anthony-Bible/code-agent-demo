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
	input   io.Reader
	output  io.Writer
	prompt  string
	colors  port.ColorScheme
	scanner *bufio.Scanner
}

// NewCLIAdapter creates a new CLIAdapter with default I/O.
func NewCLIAdapter() *CLIAdapter {
	return &CLIAdapter{
		input:  os.Stdin,
		output: os.Stdout,
		prompt: "> ",
		colors: port.ColorScheme{
			User:      "\x1b[94m", // Blue
			Assistant: "\x1b[93m", // Yellow
			System:    "\x1b[96m", // Cyan
			Error:     "\x1b[91m", // Red
			Tool:      "\x1b[92m", // Green
			Prompt:    "\x1b[94m", // Blue
		},
	}
}

// NewCLIAdapterWithIO creates a new CLIAdapter with custom I/O for testing.
func NewCLIAdapterWithIO(input io.Reader, output io.Writer) *CLIAdapter {
	return &CLIAdapter{
		input:  input,
		output: output,
		prompt: "> ",
		colors: port.ColorScheme{
			User:      "\x1b[94m", // Blue
			Assistant: "\x1b[93m", // Yellow
			System:    "\x1b[96m", // Cyan
			Error:     "\x1b[91m", // Red
			Tool:      "\x1b[92m", // Green
			Prompt:    "\x1b[94m", // Blue
		},
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
func (c *CLIAdapter) DisplayToolResult(toolName string, input string, result string) error {
	_, err := fmt.Fprintf(c.output, "%sTool [%s] on %s\x1b[0m\n%s\x1b[0m\n",
		c.colors.Tool, toolName, input, result)
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

// ConfirmBashCommand prompts the user to confirm a bash command before execution.
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

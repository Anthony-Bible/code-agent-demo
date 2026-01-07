package ui

import (
	"bufio"
	"code-editing-agent/internal/domain/port"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/chzyer/readline"
)

// Key constants for special key handling.
const (
	KeyTab      = "tab"
	KeyShiftTab = "shift+tab"
)

// CLIAdapter implements the UserInterface port using the command line.
type CLIAdapter struct {
	input              io.Reader
	output             io.Writer
	prompt             string
	colors             port.ColorScheme
	scanner            *bufio.Scanner
	truncationConfig   TruncationConfig
	useInteractive     bool
	historyFile        string
	readlineInstance   *readline.Instance
	modeToggleCallback func()
	planMode           bool
	sessionID          string
	mu                 sync.RWMutex
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
		Thinking:  "\x1b[95m", // Bright Magenta
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
		useInteractive:   IsTerminal(os.Stdin),
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

// NewCLIAdapterWithHistory creates a new CLIAdapter configured for interactive
// mode with command history support. The historyFile parameter specifies the
// path to the file where command history will be persisted.
//
// If historyFile is empty, history will not be persisted to disk.
//
// The returned adapter is always in interactive mode (IsInteractive() returns true).
func NewCLIAdapterWithHistory(historyFile string) *CLIAdapter {
	// Expand tilde in history file path if present
	expandedPath := expandPath(historyFile)

	return &CLIAdapter{
		input:            os.Stdin,
		output:           os.Stdout,
		prompt:           "> ",
		colors:           defaultColorScheme(),
		truncationConfig: DefaultTruncationConfig(),
		useInteractive:   true,
		historyFile:      expandedPath,
	}
}

// expandPath expands a tilde prefix to the user's home directory.
// It handles two cases:
//   - "~" alone expands to the home directory
//   - "~/..." expands to home directory joined with the rest of the path
//
// Paths without a leading tilde, or with ~username format, are returned unchanged.
// If the home directory cannot be determined, the original path is returned.
func expandPath(path string) string {
	if path == "" {
		return ""
	}

	if path == "~" {
		return getHomeDir(path)
	}

	if strings.HasPrefix(path, "~/") {
		homeDir := getHomeDir(path)
		if homeDir == path {
			// getHomeDir returned original path due to error
			return path
		}
		// Use filepath.Join for cross-platform path concatenation
		return filepath.Join(homeDir, path[2:])
	}

	return path
}

// getHomeDir returns the user's home directory, or the fallback value if unavailable.
func getHomeDir(fallback string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fallback
	}
	return homeDir
}

// GetUserInput gets input from the user with context support.
// When in interactive mode, uses readline for arrow key navigation and history.
// When in non-interactive mode, uses bufio.Scanner for simple line input.
func (c *CLIAdapter) GetUserInput(ctx context.Context) (string, bool) {
	// Check if context is cancelled
	select {
	case <-ctx.Done():
		return "", false
	default:
		// Continue
	}

	// Use readline for interactive mode with history support
	if c.useInteractive && c.historyFile != "" {
		return c.getInteractiveInput(ctx)
	}

	// Fall back to bufio.Scanner for non-interactive mode
	return c.getScannerInput()
}

// getInteractiveInput uses readline for feature-rich terminal input with context support.
func (c *CLIAdapter) getInteractiveInput(ctx context.Context) (string, bool) {
	// Initialize readline instance if not already created
	if c.readlineInstance == nil {
		config := &readline.Config{
			Prompt:          c.colors.Prompt + "Claude: " + "\x1b[0m",
			HistoryFile:     c.historyFile,
			InterruptPrompt: "^C",
			EOFPrompt:       "exit",
		}

		var err error
		c.readlineInstance, err = readline.NewEx(config)
		if err != nil {
			// Fall back to scanner on error
			return c.getScannerInput()
		}
	}

	// Use a goroutine to read input and support context cancellation
	type result struct {
		line string
		err  error
	}
	resultCh := make(chan result, 1)

	go func() {
		line, err := c.readlineInstance.Readline()
		resultCh <- result{line, err}
	}()

	// Wait for input or context cancellation
	select {
	case <-ctx.Done():
		// Context cancelled - close readline to unblock the goroutine
		_ = c.readlineInstance.Close()
		c.readlineInstance = nil
		return "", false
	case res := <-resultCh:
		if res.err != nil {
			// EOF or error
			return "", false
		}

		return res.line, true
	}
}

// getInteractiveConfirmation uses readline for Y/N confirmation in interactive mode.
// Returns the user's input string (to be checked by caller).
// Ctrl+C returns empty string, which is treated as "no" (safe default).
func (c *CLIAdapter) getInteractiveConfirmation() string {
	// Create a simple readline instance for confirmation
	rl, err := readline.NewEx(&readline.Config{
		Prompt:          c.colors.Error + "Execute? [y/N]: " + "\x1b[0m",
		InterruptPrompt: "^C",
	})
	if err != nil {
		// Fall back to simple input
		fmt.Fprint(c.output, "Execute? [y/N]: ")
		if c.scanner == nil {
			c.scanner = bufio.NewScanner(c.input)
		}
		if c.scanner.Scan() {
			return c.scanner.Text()
		}
		return ""
	}
	defer rl.Close()

	line, err := rl.Readline()
	if err != nil {
		return ""
	}
	return line
}

// getScannerInput uses bufio.Scanner for non-interactive input.
func (c *CLIAdapter) getScannerInput() (string, bool) {
	if c.scanner == nil {
		c.scanner = bufio.NewScanner(c.input)
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

	return c.scanner.Text(), true
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

// BeginStreamingResponse starts a streaming response with color setup.
func (c *CLIAdapter) BeginStreamingResponse() error {
	_, err := fmt.Fprint(c.output, c.colors.Assistant)
	return err
}

// EndStreamingResponse ends a streaming response with color teardown and newline.
func (c *CLIAdapter) EndStreamingResponse() error {
	_, err := fmt.Fprint(c.output, "\x1b[0m\n")
	return err
}

// DisplayStreamingText displays a chunk of streaming text without a newline.
// This is used to show text as it arrives in real-time from the AI provider.
// The text is displayed without color codes - the caller should handle color setup/teardown.
func (c *CLIAdapter) DisplayStreamingText(text string) error {
	// Use direct write to avoid any potential buffering from fmt package
	_, err := c.output.Write([]byte(text))
	if err != nil {
		return err
	}

	// Flush the output to ensure streaming text appears immediately
	// This is needed because stdout is typically line-buffered when connected to a terminal
	return c.flushOutput()
}

// flushOutput attempts to flush the output writer if it supports flushing.
// For *os.File (like os.Stdout), this is a no-op since we can't reliably flush C stdio buffers from Go.
// However, this works for bufio.Writer and other flushable writers.
func (c *CLIAdapter) flushOutput() error {
	type flusher interface {
		Flush() error
	}

	if f, ok := c.output.(flusher); ok {
		return f.Flush()
	}

	// For os.File/os.Stdout, we can't force a flush of the C library's stdio buffers
	// from Go code. However, writes to os.Stdout are typically unbuffered or line-buffered,
	// and calling Write() should make the data available to the OS immediately.
	// The buffering issue is at the C stdio layer, not the Go layer.
	return nil
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
//
// File read operations (read_file, list_files) display compact indicators like
// read(path) or list(path) instead of full contents to keep the screen clean.
func (c *CLIAdapter) DisplayToolResult(toolName string, input string, result string) error {
	// Compact display for file/directory read operations
	switch toolName {
	case "read_file":
		return c.displayCompactFileRead(input)
	case "list_files":
		return c.displayCompactListFiles(input)
	}

	// Default behavior for other tools
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

// DisplayThinking displays extended thinking content from the AI.
// Uses thinking color from the color scheme to distinguish from regular responses.
func (c *CLIAdapter) DisplayThinking(content string) error {
	// Use the thinking color from the color scheme and format with clear separation
	_, err := fmt.Fprintf(c.output, "%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\x1b[0m\n", c.colors.Thinking)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.output, "%sClaude is thinking...\x1b[0m\n", c.colors.Thinking)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(c.output, "%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\x1b[0m\n", c.colors.Thinking)
	if err != nil {
		return err
	}
	// Indent the thinking content for better visual separation
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		_, err = fmt.Fprintf(c.output, "%s  %s\x1b[0m\n", c.colors.Thinking, line)
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(c.output, "%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━\x1b[0m\n\n", c.colors.Thinking)
	return err
}

// DisplaySubagentStatus displays a status message for subagent execution.
// Uses magenta color (ANSI code 35) to distinguish from regular system messages.
func (c *CLIAdapter) DisplaySubagentStatus(agentName string, status string, details string) error {
	prefix := fmt.Sprintf("[SUBAGENT: %s]", agentName)
	msg := fmt.Sprintf("%s %s", prefix, status)
	if details != "" {
		msg += " - " + details
	}
	// Magenta color for subagent status
	_, err := fmt.Fprintf(c.output, "\x1b[35m%s\x1b[0m\n", msg)
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
		scheme.Error == "" && scheme.Tool == "" && scheme.Prompt == "" && scheme.Thinking == "" {
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
	if scheme.Thinking != "" {
		c.colors.Thinking = scheme.Thinking
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

// displayCompactFileRead displays a compact indicator for file read operations.
// Shows "read(path)" or "read(path:start-end)" for line ranges.
func (c *CLIAdapter) displayCompactFileRead(input string) error {
	var readInput struct {
		Path      string `json:"path"`
		StartLine *int   `json:"start_line,omitempty"`
		EndLine   *int   `json:"end_line,omitempty"`
	}

	if err := json.Unmarshal([]byte(input), &readInput); err != nil {
		_, err := fmt.Fprintf(c.output, "%sread(%s)\x1b[0m\n", c.colors.Tool, input)
		return err
	}

	display := readInput.Path
	if readInput.StartLine != nil || readInput.EndLine != nil {
		start := 1
		end := "end"
		if readInput.StartLine != nil {
			start = *readInput.StartLine
		}
		if readInput.EndLine != nil {
			end = strconv.Itoa(*readInput.EndLine)
		}
		display = fmt.Sprintf("%s:%d-%s", readInput.Path, start, end)
	}

	_, err := fmt.Fprintf(c.output, "%sread(%s)\x1b[0m\n", c.colors.Tool, display)
	return err
}

// displayCompactListFiles displays a compact indicator for directory listing operations.
// Shows "list(path)" instead of the full directory contents.
func (c *CLIAdapter) displayCompactListFiles(input string) error {
	var listInput struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal([]byte(input), &listInput); err != nil {
		_, err := fmt.Fprintf(c.output, "%slist(%s)\x1b[0m\n", c.colors.Tool, input)
		return err
	}

	_, err := fmt.Fprintf(c.output, "%slist(%s)\x1b[0m\n", c.colors.Tool, listInput.Path)
	return err
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

// =============================================================================
// Terminal Detection
// =============================================================================

// IsTerminal checks if the given io.Reader is connected to a terminal.
// It returns true if the reader is an *os.File that represents a terminal
// (character device), false otherwise.
//
// This is used to determine whether to use interactive input (go-prompt)
// or non-interactive input (bufio.Scanner).
func IsTerminal(r io.Reader) bool {
	if r == nil {
		return false
	}
	f, ok := r.(*os.File)
	if !ok {
		return false
	}
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

// IsInteractive returns whether the adapter is in interactive mode.
// When true, the adapter uses go-prompt for input with features like
// command history, auto-completion, and line editing.
// When false, the adapter uses bufio.Scanner for simple line-based input.
func (c *CLIAdapter) IsInteractive() bool {
	return c.useInteractive
}

// SetInteractive sets whether the adapter should use interactive mode.
// This allows forcing interactive or non-interactive mode regardless of
// terminal detection, which is useful for testing.
func (c *CLIAdapter) SetInteractive(interactive bool) {
	c.useInteractive = interactive
}

// InputModeString returns a string description of the current input mode.
// Returns "interactive" if IsInteractive() is true, "non-interactive" otherwise.
// This is useful for debugging and logging.
func (c *CLIAdapter) InputModeString() string {
	if c.useInteractive {
		return "interactive"
	}
	return "non-interactive"
}

// GetHistoryFile returns the path to the command history file.
// Returns an empty string if no history file is configured.
func (c *CLIAdapter) GetHistoryFile() string {
	return c.historyFile
}

// GetMaxHistoryEntries returns the maximum number of history entries to store.
// Returns 0 if using the default value.
func (c *CLIAdapter) GetMaxHistoryEntries() int {
	return 0 // readline handles this internally
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

	var input string

	// Use go-prompt for interactive mode, bufio.Scanner for non-interactive
	if c.useInteractive && c.historyFile != "" {
		// Interactive mode: use go-prompt to avoid stdin conflict
		input = c.getInteractiveConfirmation()
	} else {
		// Non-interactive mode: use bufio.Scanner
		fmt.Fprint(c.output, "Execute? [y/N]: ")
		if c.scanner == nil {
			c.scanner = bufio.NewScanner(c.input)
		}
		if !c.scanner.Scan() {
			return false
		}
		input = c.scanner.Text()
	}

	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

// GetPromptPrefix returns the current prompt prefix string displayed before user input.
// This is the string set via SetPrompt, or the default "> " if not explicitly set.
// The prompt prefix is used by go-prompt to display the input prompt in interactive mode.
func (c *CLIAdapter) GetPromptPrefix() string {
	return c.prompt
}

// SetModeToggleCallback sets the callback function to invoke when Shift+Tab is pressed.
// This allows external code to handle mode toggling via keyboard shortcuts.
// The callback is invoked in a thread-safe manner.
func (c *CLIAdapter) SetModeToggleCallback(callback func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.modeToggleCallback = callback
}

// SetSessionID sets the session ID for the adapter.
// The session ID is displayed in the prompt when set.
// Thread-safe for concurrent access.
func (c *CLIAdapter) SetSessionID(sessionID string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sessionID = sessionID
}

// SetPlanMode sets the plan mode state for the adapter.
// When plan mode is enabled, a "[PLAN MODE]" prefix is displayed in the prompt.
// Thread-safe for concurrent access.
func (c *CLIAdapter) SetPlanMode(enabled bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.planMode = enabled
}

// GetPrompt returns the current prompt string with mode indicator if applicable.
// The prompt format depends on the current plan mode and session ID:
//   - Normal mode: "Claude> [sessionID]"
//   - Plan mode: "[PLAN MODE] Claude> [sessionID]"
//
// Thread-safe for concurrent reads.
func (c *CLIAdapter) GetPrompt() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := c.prompt
	if c.planMode {
		result = "[PLAN MODE] " + result
	}
	if c.sessionID != "" {
		result = result + " [" + c.sessionID + "]"
	}
	return result
}

// HandleKeyPress processes a key press event.
// If the key is Shift+Tab and a mode toggle callback is registered, invokes the callback.
// This is typically called by the input handler to respond to keyboard shortcuts.
// Thread-safe for concurrent access.
func (c *CLIAdapter) HandleKeyPress(key string) {
	if key == KeyShiftTab {
		c.mu.RLock()
		callback := c.modeToggleCallback
		c.mu.RUnlock()
		if callback != nil {
			callback()
		}
	}
}

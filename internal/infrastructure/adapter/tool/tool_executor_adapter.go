package tool

import (
	"bytes"
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
	fileadapter "code-editing-agent/internal/infrastructure/adapter/file"
)

// DangerousCommandCallback is called when a dangerous command is detected.
// It receives the command and reason, and returns true if execution should proceed.
type DangerousCommandCallback func(command, reason string) bool

// CommandConfirmationCallback is called before executing any bash command.
// It receives the command, whether it's dangerous, the reason if dangerous, and a description.
// Returns true if execution should proceed, false to block.
type CommandConfirmationCallback func(command string, isDangerous bool, reason string, description string) bool

// ExecutorAdapter implements the ToolExecutor port using the FileManager for file operations.
type ExecutorAdapter struct {
	fileManager                 port.FileManager
	tools                       map[string]entity.Tool
	mu                          sync.RWMutex
	dangerousCommandCallback    DangerousCommandCallback
	commandConfirmationCallback CommandConfirmationCallback
}

// toRawMessage converts various input types to json.RawMessage for validation.
func toRawMessage(input interface{}) (json.RawMessage, error) {
	switch v := input.(type) {
	case string:
		return json.RawMessage(v), nil
	case json.RawMessage:
		return v, nil
	case []byte:
		return v, nil
	default:
		rawInput, err := json.Marshal(input)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal input: %w", err)
		}
		return rawInput, nil
	}
}

// wrapFileOperationError wraps file operation errors and prints a warning for path traversal attempts.
func wrapFileOperationError(operation string, err error) error {
	if err == nil {
		return nil
	}

	// Check for path traversal error in the error chain
	if errors.Is(err, fileadapter.ErrPathTraversal) {
		// Print a security warning to stderr
		fmt.Fprintf(os.Stderr, "\x1b[91m[SECURITY WARNING] Path traversal attempt detected and blocked!\x1b[0m\n")
		return fmt.Errorf("%s blocked due to potential security threat: %w", operation, err)
	}

	// Check for PathValidationError which has detailed reason
	var pathErr *fileadapter.PathValidationError
	if errors.As(err, &pathErr) && pathErr.Reason == "path traversal attempt detected" {
		// Print a security warning to stderr
		fmt.Fprintf(os.Stderr, "\x1b[91m[SECURITY WARNING] Path traversal attempt detected and blocked!\x1b[0m\n")
		return fmt.Errorf("%s blocked due to potential security threat: %w", operation, err)
	}

	return fmt.Errorf("%s: %w", operation, err)
}

// NewExecutorAdapter creates a new ExecutorAdapter with the provided FileManager.
// It also registers the default tools (read_file, list_files, edit_file).
func NewExecutorAdapter(fileManager port.FileManager) *ExecutorAdapter {
	adapter := &ExecutorAdapter{
		fileManager: fileManager,
		tools:       make(map[string]entity.Tool),
	}

	// Register default tools
	adapter.registerDefaultTools()

	return adapter
}

// SetDangerousCommandCallback sets the callback for dangerous command confirmation.
func (a *ExecutorAdapter) SetDangerousCommandCallback(cb DangerousCommandCallback) {
	a.dangerousCommandCallback = cb
}

// SetCommandConfirmationCallback sets the callback for all command confirmation.
func (a *ExecutorAdapter) SetCommandConfirmationCallback(cb CommandConfirmationCallback) {
	a.commandConfirmationCallback = cb
}

// RegisterTool registers a new tool with the executor.
func (a *ExecutorAdapter) RegisterTool(tool entity.Tool) error {
	if err := tool.Validate(); err != nil {
		return fmt.Errorf("invalid tool: %w", err)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.tools[tool.Name] = tool
	return nil
}

// UnregisterTool removes a tool from the executor by name.
func (a *ExecutorAdapter) UnregisterTool(name string) error {
	if name == "" {
		return errors.New("tool name cannot be empty")
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.tools, name)
	return nil
}

// ExecuteTool executes a tool with the given name and input.
func (a *ExecutorAdapter) ExecuteTool(ctx context.Context, name string, input interface{}) (string, error) {
	a.mu.RLock()
	tool, exists := a.tools[name]
	a.mu.RUnlock()

	if !exists {
		return "", fmt.Errorf("tool not found: %s", name)
	}

	// Convert input to JSON for validation
	rawInput, err := toRawMessage(input)
	if err != nil {
		return "", err
	}

	// Validate input against tool's schema
	if err := tool.ValidateInput(rawInput); err != nil {
		return "", fmt.Errorf("invalid input for tool %s: %w", name, err)
	}

	// Execute the tool
	return a.executeByName(ctx, name, rawInput)
}

// ListTools returns a list of all registered tools.
func (a *ExecutorAdapter) ListTools() ([]entity.Tool, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tools := make([]entity.Tool, 0, len(a.tools))
	for _, tool := range a.tools {
		tools = append(tools, tool)
	}
	return tools, nil
}

// GetTool retrieves a specific tool by name.
// Returns the tool and a boolean indicating if it was found.
func (a *ExecutorAdapter) GetTool(name string) (entity.Tool, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	tool, exists := a.tools[name]
	return tool, exists
}

// ValidateToolInput validates input for a specific tool.
func (a *ExecutorAdapter) ValidateToolInput(name string, input interface{}) error {
	a.mu.RLock()
	tool, exists := a.tools[name]
	a.mu.RUnlock()

	if !exists {
		return fmt.Errorf("tool not found: %s", name)
	}

	rawInput, err := toRawMessage(input)
	if err != nil {
		return err
	}

	return tool.ValidateInput(rawInput)
}

// registerDefaultTools registers the built-in tools.
func (a *ExecutorAdapter) registerDefaultTools() {
	// Register read_file tool
	readFileTool := entity.Tool{
		ID:          "read_file",
		Name:        "read_file",
		Description: "Reads the contents of a given relative file path, use this when you want to see what's inside a file. Do not use this with directory names.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The relative path to the file to read in the working directory..",
				},
			},
			"required": []string{"path"},
		},
		RequiredFields: []string{"path"},
	}
	a.tools[readFileTool.Name] = readFileTool

	// Register list_files tool
	listFilesTool := entity.Tool{
		ID:          "list_files",
		Name:        "list_files",
		Description: "Lists files and directories at a given path. If no path is provided, lists files in the current working directory.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The relative path to the directory to list files in. If not provided, lists files in the current working directory.",
				},
			},
		},
		RequiredFields: []string{},
	}
	a.tools[listFilesTool.Name] = listFilesTool

	// Register edit_file tool
	editFileTool := entity.Tool{
		ID:          "edit_file",
		Name:        "edit_file",
		Description: "Makes edits to a text file. Replaces 'old_str' with 'new_str' in the given file. 'old_str' and 'new_str' MUST be different from each other. If the file specified with path doesn't exist, it will be created.The old_stribg must match exactly including whitespace and new lines. Include a few lines before to avoid editing a string with multiple matches.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "The relative path to the file to edit.",
				},
				"old_str": map[string]interface{}{
					"type":        "string",
					"description": "The string to replace.",
				},
				"new_str": map[string]interface{}{
					"type":        "string",
					"description": "The string to replace 'old_str' with.",
				},
			},
			"required": []string{"path"},
		},
		RequiredFields: []string{"path"},
	}
	a.tools[editFileTool.Name] = editFileTool

	// Register bash tool
	bashTool := entity.Tool{
		ID:          "bash",
		Name:        "bash",
		Description: "Executes shell commands and returns stdout, stderr, and exit code. Dangerous commands require user confirmation.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The shell command to execute",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "A brief description of what this command does and why it's being run",
				},
				"timeout_ms": map[string]interface{}{
					"type":        "integer",
					"description": "Timeout in milliseconds (default: 30000)",
				},
			},
			"required": []string{"command"},
		},
		RequiredFields: []string{"command"},
	}
	a.tools[bashTool.Name] = bashTool

	// Register fetch tool
	fetchTool := entity.Tool{
		ID:          "fetch",
		Name:        "fetch",
		Description: "Fetches web resources via HTTP/HTTPS. Prefer this to bash-isms like curl/wget",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"url": map[string]interface{}{
					"type":        "string",
					"description": "Full URL to fetch, e.g. https://...",
				},
				"includeMarkup": map[string]interface{}{
					"type":        "boolean",
					"description": "Include the HTML markup? Defaults to false. By default or when set to false, markup will be stripped and converted to plain text. Prefer markup stripping, and only set this to true if the output is confusing: otherwise you may download a massive amount of data",
				},
			},
			"required": []string{"url"},
		},
		RequiredFields: []string{"url"},
	}
	a.tools[fetchTool.Name] = fetchTool
}

// executeByName executes the appropriate tool function based on the tool name.
func (a *ExecutorAdapter) executeByName(ctx context.Context, name string, input json.RawMessage) (string, error) {
	switch name {
	case "read_file":
		return a.executeReadFile(input)
	case "list_files":
		return a.executeListFiles(input)
	case "edit_file":
		return a.executeEditFile(input)
	case "bash":
		return a.executeBash(ctx, input)
	case "fetch":
		return a.executeFetch(ctx, input)
	default:
		return "", fmt.Errorf("no implementation available for tool: %s", name)
	}
}

// readFileInput represents the input for the read_file tool.
type readFileInput struct {
	Path string `json:"path"`
}

// executeReadFile executes the read_file tool.
func (a *ExecutorAdapter) executeReadFile(input json.RawMessage) (string, error) {
	var in readFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to unmarshal read_file input: %w", err)
	}

	content, err := a.fileManager.ReadFile(in.Path)
	if err != nil {
		return "", wrapFileOperationError("Failed to read file", err)
	}

	return content, nil
}

// listFilesInput represents the input for the list_files tool.
type listFilesInput struct {
	Path string `json:"path"`
}

// executeListFiles executes the list_files tool.
func (a *ExecutorAdapter) executeListFiles(input json.RawMessage) (string, error) {
	var in listFilesInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to unmarshal list_files input: %w", err)
	}

	dir := "."
	if in.Path != "" {
		dir = in.Path
	}

	// Exclude .git directories by default for cleaner AI output
	files, err := a.fileManager.ListFiles(dir, true, false)
	if err != nil {
		return "", wrapFileOperationError("Failed to list files", err)
	}

	// Convert relative paths to exclude the base directory for cleaner output
	var resultFiles []string
	for _, file := range files {
		relPath := strings.TrimPrefix(file, dir)
		relPath = strings.TrimPrefix(relPath, "/")
		if relPath != "." && relPath != "" {
			resultFiles = append(resultFiles, relPath)
		}
	}

	result, err := json.Marshal(resultFiles)
	if err != nil {
		return "", fmt.Errorf("failed to marshal files result: %w", err)
	}

	return string(result), nil
}

// editFileInput represents the input for the edit_file tool.
type editFileInput struct {
	Path   string `json:"path"`
	OldStr string `json:"old_str"`
	NewStr string `json:"new_str"`
}

// executeEditFile executes the edit_file tool.
func (a *ExecutorAdapter) executeEditFile(input json.RawMessage) (string, error) {
	var in editFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to unmarshal edit_file input: %w", err)
	}

	// Validate input
	if in.Path == "" || in.OldStr == in.NewStr {
		return "", errors.New("invalid input parameters: path is required and old_str must differ from new_str")
	}

	// Check if file exists
	exists, err := a.fileManager.FileExists(in.Path)
	if err != nil {
		return "", wrapFileOperationError("Failed to check if file exists", err)
	}

	// If file doesn't exist and old_str is empty, create a new file
	if !exists && in.OldStr == "" {
		return a.createNewFile(in.Path, in.NewStr)
	}

	// Read existing file content
	content, err := a.fileManager.ReadFile(in.Path)
	if err != nil {
		return "", wrapFileOperationError("Failed to read file", err)
	}

	oldContent := content
	newContent := strings.ReplaceAll(oldContent, in.OldStr, in.NewStr)

	// Check if replacement occurred
	if oldContent == newContent && in.OldStr != "" {
		return "", errors.New("old string not found in file")
	}

	// Write the modified content
	if err := a.fileManager.WriteFile(in.Path, newContent); err != nil {
		return "", wrapFileOperationError("Failed to write file", err)
	}

	return "OK", nil
}

// createNewFile creates a new file with the given content.
func (a *ExecutorAdapter) createNewFile(filePath, content string) (string, error) {
	// Create directory if needed
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		if err := a.fileManager.CreateDirectory(dir); err != nil {
			return "", wrapFileOperationError(fmt.Sprintf("Failed to create directory %s", dir), err)
		}
	}

	// Write the new file content
	if err := a.fileManager.WriteFile(filePath, content); err != nil {
		return "", wrapFileOperationError(fmt.Sprintf("Failed to create file %s", filePath), err)
	}

	return fmt.Sprintf("Created file %s", filePath), nil
}

// bashInput represents the input for the bash tool.
type bashInput struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
	TimeoutMs   int    `json:"timeout_ms,omitempty"`
}

// fetchInput represents the input for the fetch tool.
type fetchInput struct {
	URL           string `json:"url"`
	IncludeMarkup bool   `json:"includeMarkup,omitempty"`
}

// bashOutput represents the output from the bash tool.
type bashOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// defaultBashTimeout is the default timeout for bash command execution.
const defaultBashTimeout = 30 * time.Second

// dangerousPattern represents a pattern that indicates a dangerous command.
type dangerousPattern struct {
	pattern *regexp.Regexp
	reason  string
}

// dangerousPatterns contains patterns for detecting dangerous commands.
//
//nolint:gochecknoglobals // This is intentionally a package-level constant for dangerous command detection
var dangerousPatterns = []dangerousPattern{
	// Matches rm with any flags followed by dangerous paths (/, ~, *)
	{regexp.MustCompile(`rm\s+(-\w+\s+)*[/~*]`), "destructive rm command"},
	{regexp.MustCompile(`sudo\s+`), "sudo command"},
	{regexp.MustCompile(`chmod\s+777`), "insecure chmod"},
	{regexp.MustCompile(`mkfs\.`), "filesystem format"},
	{regexp.MustCompile(`dd\s+if=`), "low-level disk operation"},
	{regexp.MustCompile(`>\s*/dev/`), "write to device"},
}

// isDangerousCommand checks if a command matches any dangerous patterns.
func isDangerousCommand(cmd string) (bool, string) {
	for _, dp := range dangerousPatterns {
		if dp.pattern.MatchString(cmd) {
			return true, dp.reason
		}
	}
	return false, ""
}

// checkCommandConfirmation checks if a command should be allowed to execute.
func (a *ExecutorAdapter) checkCommandConfirmation(command string, description string) error {
	isDangerous, reason := isDangerousCommand(command)

	switch {
	case a.commandConfirmationCallback != nil:
		if !a.commandConfirmationCallback(command, isDangerous, reason, description) {
			if isDangerous {
				return fmt.Errorf("dangerous command denied by user: %s (%s)", reason, command)
			}
			return fmt.Errorf("command denied by user: %s", command)
		}
	case a.dangerousCommandCallback != nil && isDangerous:
		// Backward compatibility: use old callback for dangerous commands
		if !a.dangerousCommandCallback(command, reason) {
			return fmt.Errorf("dangerous command denied by user: %s (%s)", reason, command)
		}
	case isDangerous:
		// No callback set and command is dangerous - block it
		return fmt.Errorf("dangerous command blocked: %s (%s)", reason, command)
	}
	return nil
}

// executeBash executes a bash command and returns the output.
func (a *ExecutorAdapter) executeBash(ctx context.Context, input json.RawMessage) (string, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to unmarshal bash input: %w", err)
	}

	if in.Command == "" {
		return "", errors.New("command is required")
	}

	// Check command confirmation
	if err := a.checkCommandConfirmation(in.Command, in.Description); err != nil {
		return "", err
	}

	// Set timeout
	timeout := defaultBashTimeout
	if in.TimeoutMs > 0 {
		timeout = time.Duration(in.TimeoutMs) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	//nolint:gosec // G204: This is intentionally executing user-provided commands (bash tool)
	cmd := exec.CommandContext(
		ctx,
		"bash",
		"-c",
		in.Command,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := bashOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("command timeout after %v", timeout)
		}
		// Get exit code from error
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			output.ExitCode = exitErr.ExitCode()
		} else {
			return "", fmt.Errorf("failed to execute command: %w", err)
		}
	}

	result, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}

	return string(result), nil
}

// defaultFetchTimeout is the default timeout for fetch operations.
const defaultFetchTimeout = 30 * time.Second

// maxResponseSize defines an upper bound on the number of bytes we will accept
// in an HTTP response body. The 10MB limit is a compromise: it is large enough
// to cover typical HTML pages and JSON responses used by tools, while still
// preventing unbounded memory growth if a server returns an unexpectedly large
// payload. Callers that use this constant to bound response reads should stop
// reading and treat the operation as failed (for example, by returning an error)
// when the response exceeds this limit, instead of loading the entire body into
// memory.
const maxResponseSize = 10 << 20
// validateURL validates that the URL is safe to fetch.
func validateURL(rawURL string) error {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Only allow http and https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return fmt.Errorf("URL must use http or https protocol, got: %s", parsedURL.Scheme)
	}

	// Ensure URL has a host
	if parsedURL.Host == "" {
		return fmt.Errorf("URL must have a host")
	}

	return nil
}

// htmlToText converts HTML content to plain text.
func htmlToText(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", fmt.Errorf("failed to parse HTML: %w", err)
	}

	var result strings.Builder

	// Recursively extract text from nodes
	var extractText func(*html.Node)
	extractText = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			text := strings.TrimSpace(n.Data)
			if text != "" {
				result.WriteString(text)
				result.WriteString(" ")
			}

		case html.ElementNode:
			// Add newline for block elements
			switch n.Data {
			case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "br":
				result.WriteString(" ")
			case "li":
				result.WriteString(" ")
			}

			// Recursively process children
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				extractText(c)
			}
		case html.DocumentNode:
			// Process all children of document node
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				extractText(c)
			}
		}
	}

	extractText(doc)

	// Clean up whitespace
	text := result.String()
	text = strings.Join(strings.Fields(text), " ")

	return text, nil
}

// executeFetch executes the fetch tool.
func (a *ExecutorAdapter) executeFetch(ctx context.Context, input json.RawMessage) (string, error) {
	var in fetchInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to unmarshal fetch input: %w", err)
	}

	// Validate URL
	if err := validateURL(in.URL); err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	// Set timeout, but do not extend an existing earlier parent deadline.
	if deadline, ok := ctx.Deadline(); ok {
		// If the existing deadline is further in the future than our default timeout,
		// apply a new timeout to cap it at defaultFetchTimeout. Otherwise, keep the
		// tighter parent deadline.
		if time.Until(deadline) > defaultFetchTimeout {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultFetchTimeout)
			defer cancel()
		}
	} else {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, defaultFetchTimeout)
		defer cancel()
	}

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", in.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set user agent
	req.Header.Set("User-Agent", "code-editing-agent/1.0")

	// Make HTTP request using a dedicated client with timeout
	client := &http.Client{
		Timeout: defaultFetchTimeout,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode >= 400 {
		respText := resp.Status
		if resp.StatusCode == 403 {
			respText = "authorization required"
		}
		return "", fmt.Errorf("HTTP %d (%s)", resp.StatusCode, respText)
	}

	// Check content length
	if resp.ContentLength > maxResponseSize {
		return "", fmt.Errorf("response too large: %d bytes (max: %d)", resp.ContentLength, maxResponseSize)
	}

	// Read and limit response body
	limitedReader := io.LimitReader(resp.Body, maxResponseSize)
	bodyBytes, err := io.ReadAll(limitedReader)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check if we hit the limit
	if len(bodyBytes) == maxResponseSize {
		return "", fmt.Errorf("response truncated due to size limit (max: %d bytes)", maxResponseSize)
	}

	content := string(bodyBytes)

	// Convert HTML to text if includeMarkup is false
	if !in.IncludeMarkup && strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		converted, err := htmlToText(content)
		if err != nil {
			return "", fmt.Errorf("failed to convert HTML to text: %w", err)
		}
		content = converted
	}

	return content, nil
}

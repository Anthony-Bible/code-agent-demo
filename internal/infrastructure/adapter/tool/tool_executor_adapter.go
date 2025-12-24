package tool

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
)

// ExecutorAdapter implements the ToolExecutor port using the FileManager for file operations.
type ExecutorAdapter struct {
	fileManager port.FileManager
	tools       map[string]entity.Tool
	mu          sync.RWMutex
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
	var rawInput json.RawMessage
	switch v := input.(type) {
	case string:
		rawInput = json.RawMessage(v)
	case json.RawMessage:
		rawInput = v
	case []byte:
		rawInput = v
	default:
		var err error
		rawInput, err = json.Marshal(input)
		if err != nil {
			return "", fmt.Errorf("failed to marshal input: %w", err)
		}
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

	var rawInput json.RawMessage
	switch v := input.(type) {
	case string:
		rawInput = json.RawMessage(v)
	case json.RawMessage:
		rawInput = v
	case []byte:
		rawInput = v
	default:
		var err error
		rawInput, err = json.Marshal(input)
		if err != nil {
			return fmt.Errorf("failed to marshal input: %w", err)
		}
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
		Description: "Makes edits to a text file. Replaces 'old_str' with 'new_str' in the given file. 'old_str' and 'new_str' MUST be different from each other. If the file specified with path doesn't exist, it will be created.",
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
}

// executeByName executes the appropriate tool function based on the tool name.
func (a *ExecutorAdapter) executeByName(_ context.Context, name string, input json.RawMessage) (string, error) {
	switch name {
	case "read_file":
		return a.executeReadFile(input)
	case "list_files":
		return a.executeListFiles(input)
	case "edit_file":
		return a.executeEditFile(input)
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
		return "", fmt.Errorf("failed to read file: %w", err)
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
		return "", fmt.Errorf("failed to list files: %w", err)
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
		return "", fmt.Errorf("failed to check if file exists: %w", err)
	}

	// If file doesn't exist and old_str is empty, create a new file
	if !exists && in.OldStr == "" {
		return a.createNewFile(in.Path, in.NewStr)
	}

	// Read existing file content
	content, err := a.fileManager.ReadFile(in.Path)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	oldContent := content
	newContent := strings.ReplaceAll(oldContent, in.OldStr, in.NewStr)

	// Check if replacement occurred
	if oldContent == newContent && in.OldStr != "" {
		return "", errors.New("old string not found in file")
	}

	// Write the modified content
	if err := a.fileManager.WriteFile(in.Path, newContent); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return "OK", nil
}

// createNewFile creates a new file with the given content.
func (a *ExecutorAdapter) createNewFile(filePath, content string) (string, error) {
	// Create directory if needed
	dir := filepath.Dir(filePath)
	if dir != "." && dir != "" {
		if err := a.fileManager.CreateDirectory(dir); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	// Write the new file content
	if err := a.fileManager.WriteFile(filePath, content); err != nil {
		return "", fmt.Errorf("failed to create file %s: %w", filePath, err)
	}

	return fmt.Sprintf("Created file %s", filePath), nil
}

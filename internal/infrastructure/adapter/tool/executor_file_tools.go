package tool

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	fileadapter "github.com/anthony-bible/code-agent-demo/internal/infrastructure/adapter/file"
)

// readFileInput represents the input for the read_file tool.
type readFileInput struct {
	Path      string `json:"path"`
	StartLine *int   `json:"start_line"`
	EndLine   *int   `json:"end_line"`
}

// validateLineRange validates start_line and end_line parameters.
// Returns an error if the values are invalid.
func (in *readFileInput) validateLineRange() error {
	if in.StartLine != nil && *in.StartLine < 1 {
		return fmt.Errorf("start_line must be >= 1, got %d", *in.StartLine)
	}
	if in.EndLine != nil && *in.EndLine < 1 {
		return fmt.Errorf("end_line must be >= 1, got %d", *in.EndLine)
	}
	if in.StartLine != nil && in.EndLine != nil && *in.StartLine > *in.EndLine {
		return fmt.Errorf("start_line (%d) must be <= end_line (%d)", *in.StartLine, *in.EndLine)
	}
	return nil
}

// maxReadFileLines is the default line cap when no explicit range is requested.
// Prevents sending excessively large outputs to the LLM.
const maxReadFileLines = 2000

// preAllocBuilder pre-allocates a strings.Builder based on file size, capped at 10MB.
func preAllocBuilder(b *strings.Builder, f *os.File) {
	const maxPreAlloc = 10 << 20
	if info, err := f.Stat(); err == nil {
		preAlloc := int(info.Size()) + 1024
		if preAlloc > maxPreAlloc {
			preAlloc = maxPreAlloc
		}
		b.Grow(preAlloc)
	}
}

// appendTruncationNotice counts remaining lines via scanner and appends a truncation message.
func appendTruncationNotice(result *strings.Builder, scanner *bufio.Scanner, linesRead int) {
	totalLines := linesRead
	for scanner.Scan() {
		totalLines++
	}
	fmt.Fprintf(
		result,
		"\n--- Output truncated at %d lines (file has %d total). Use start_line/end_line to read more. ---\n",
		maxReadFileLines,
		totalLines,
	)
}

// registerFileTools registers file-related tools.
func (a *ExecutorAdapter) registerFileTools() {
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
					"description": "The relative path to the file to read in the working directory.",
				},
				"start_line": map[string]interface{}{
					"type":        "integer",
					"description": "The 1-based line number to start reading from. If not provided, reads from the beginning.",
				},
				"end_line": map[string]interface{}{
					"type":        "integer",
					"description": "The 1-based line number to stop reading at (inclusive). If not provided, reads to the end.",
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
		Description: "Makes edits to a text file. Replaces 'old_str' with 'new_str' in the given file. 'old_str' and 'new_str' MUST be different from each other. If the file specified with path doesn't exist, it will be created. The old_str must match exactly including whitespace and new lines. Include a few lines before to avoid editing a string with multiple matches.",
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

// executeReadFile executes the read_file tool.
// It streams lines from the file to avoid loading the entire file into memory,
// only keeping the requested line range. When no range is specified, output is
// capped at maxReadFileLines with a truncation notice.
func (a *ExecutorAdapter) executeReadFile(input json.RawMessage) (string, error) {
	var in readFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to unmarshal read_file input: %w", err)
	}

	if err := in.validateLineRange(); err != nil {
		return "", err
	}

	path, err := a.fileManager.ResolvePath(in.Path)
	if err != nil {
		return "", wrapFileOperationError("Failed to read file", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return "", wrapFileOperationError("Failed to read file", err)
	}
	defer f.Close()

	startIdx := 1
	if in.StartLine != nil {
		startIdx = *in.StartLine
	}
	endIdx := 0 // 0 means "read to end"
	if in.EndLine != nil {
		endIdx = *in.EndLine
	}

	// Apply default cap when no explicit range is given
	noExplicitRange := in.StartLine == nil && in.EndLine == nil
	if noExplicitRange {
		endIdx = maxReadFileLines
	}

	var result strings.Builder
	preAllocBuilder(&result, f)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024) // up to 1MB lines
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		if lineNum < startIdx {
			continue
		}
		if endIdx > 0 && lineNum > endIdx {
			break
		}
		fmt.Fprintf(&result, "%d: %s\n", lineNum, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", wrapFileOperationError("Failed to read file", err)
	}

	if noExplicitRange && lineNum > maxReadFileLines {
		appendTruncationNotice(&result, scanner, lineNum)
	}

	return result.String(), nil
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

// maxEditFileSize is the maximum file size (in bytes) that edit_file will process.
// Files larger than this should be handled differently (e.g., targeted line-range edits).
const maxEditFileSize = 50 << 20 // 50MB

// executeEditFile executes the edit_file tool.
// Uses []byte throughout to avoid the string([]byte) copy that doubles memory.
func (a *ExecutorAdapter) executeEditFile(input json.RawMessage) (string, error) {
	var in editFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to unmarshal edit_file input: %w", err)
	}

	// Validate input
	if in.Path == "" || in.OldStr == in.NewStr {
		return "", errors.New("invalid input parameters: path is required and old_str must differ from new_str")
	}

	// Handle empty old_str: only valid for creating new files.
	// Must check before FileExists/ReadFile to avoid unnecessary I/O,
	// and before ReplaceAll which would OOM inserting new_str at every position.
	if in.OldStr == "" {
		exists, err := a.fileManager.FileExists(in.Path)
		if err != nil {
			return "", wrapFileOperationError("Failed to check if file exists", err)
		}
		if !exists {
			return a.createNewFile(in.Path, in.NewStr)
		}
		return "", errors.New("old_str must not be empty when editing an existing file")
	}

	// Resolve and validate the path
	path, err := a.fileManager.ResolvePath(in.Path)
	if err != nil {
		return "", wrapFileOperationError("Failed to read file", err)
	}

	// Check file size before reading to prevent OOM on huge files
	info, err := os.Stat(path)
	if err != nil {
		return "", wrapFileOperationError("Failed to read file", err)
	}
	if info.IsDir() {
		return "", wrapFileOperationError("Failed to read file", fileadapter.ErrIsDirectory)
	}
	if info.Size() > maxEditFileSize {
		return "", fmt.Errorf(
			"file too large for edit_file (%dMB > %dMB limit); use targeted line-range reads and smaller edits",
			info.Size()/(1024*1024),
			maxEditFileSize/(1024*1024),
		)
	}

	// Read as []byte directly to avoid string conversion copy
	oldContent, err := os.ReadFile(path)
	if err != nil {
		return "", wrapFileOperationError("Failed to read file", err)
	}

	newContent := bytes.ReplaceAll(oldContent, []byte(in.OldStr), []byte(in.NewStr))

	// Check if replacement occurred
	if bytes.Equal(oldContent, newContent) {
		return "", errors.New("old string not found in file")
	}

	// Write the modified content directly to avoid []byte→string→[]byte round-trip
	if err := os.WriteFile(path, newContent, info.Mode().Perm()); err != nil {
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

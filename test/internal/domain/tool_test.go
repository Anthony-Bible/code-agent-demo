package domain

import (
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestToolCreation tests creating different types of tools
func TestToolCreation(t *testing.T) {
	tests := []struct {
		name        string
		createTool  func() (Tool, error)
		expectError bool
		errorMsg    string
		validate    func(t *testing.T, tool Tool)
	}{
		{
			name: "create read file tool",
			createTool: func() (Tool, error) {
				return NewReadFileTool()
			},
			expectError: false,
			errorMsg:    "",
			validate: func(t *testing.T, tool Tool) {
				assert.Equal(t, "read_file", tool.GetName())
				assert.Equal(t, "Reads the contents of a given relative file path", tool.GetDescription())
				assert.NotNil(t, tool.GetInputSchema())
				assert.Contains(t, tool.GetInputSchema(), "properties")
			},
		},
		{
			name: "create list files tool",
			createTool: func() (Tool, error) {
				return NewListFilesTool()
			},
			expectError: false,
			errorMsg:    "",
			validate: func(t *testing.T, tool Tool) {
				assert.Equal(t, "list_files", tool.GetName())
				assert.Equal(t, "Lists files and directories at a given path", tool.GetDescription())
				assert.NotNil(t, tool.GetInputSchema())
			},
		},
		{
			name: "create edit file tool",
			createTool: func() (Tool, error) {
				return NewEditFileTool()
			},
			expectError: false,
			errorMsg:    "",
			validate: func(t *testing.T, tool Tool) {
				assert.Equal(t, "edit_file", tool.GetName())
				assert.Contains(t, tool.GetDescription(), "Makes edits to a text file")
				assert.NotNil(t, tool.GetInputSchema())
			},
		},
		{
			name: "create custom tool",
			createTool: func() (Tool, error) {
				input := json.RawMessage(
					`{"name": "custom_tool", "description": "A custom tool", "parameters": {"type": "object", "properties": {}}}`,
				)
				return NewCustomTool(input)
			},
			expectError: false,
			errorMsg:    "",
			validate: func(t *testing.T, tool Tool) {
				assert.Equal(t, "custom_tool", tool.GetName())
				assert.Equal(t, "A custom tool", tool.GetDescription())
				assert.NotNil(t, tool.GetInputSchema())
			},
		},
		{
			name: "create tool with invalid name should fail",
			createTool: func() (Tool, error) {
				input := json.RawMessage(`{"name": "", "description": "Invalid tool"}`)
				return NewCustomTool(input)
			},
			expectError: true,
			errorMsg:    "tool name cannot be empty",
			validate:    nil,
		},
		{
			name: "create tool with invalid JSON should fail",
			createTool: func() (Tool, error) {
				input := json.RawMessage(`invalid json}`)
				return NewCustomTool(input)
			},
			expectError: true,
			errorMsg:    "invalid tool definition JSON",
			validate:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tool, err := tt.createTool()

			if tt.expectError {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
				assert.Nil(t, tool)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, tool)
				if tt.validate != nil {
					tt.validate(t, tool)
				}
			}
		})
	}
}

// TestToolExecution tests tool execution with various inputs
func TestToolExecution(t *testing.T) {
	tests := []struct {
		name        string
		tool        Tool
		input       json.RawMessage
		expectError bool
		errorMsg    string
		expected    string
	}{
		{
			name:        "execute read file tool with valid input",
			tool:        &mockReadFileTool{},
			input:       json.RawMessage(`{"path": "test.txt"}`),
			expectError: false,
			errorMsg:    "",
			expected:    "test file content",
		},
		{
			name:        "execute read file tool with invalid input",
			tool:        &mockReadFileTool{},
			input:       json.RawMessage(`{"invalid": "input"}`),
			expectError: true,
			errorMsg:    "missing required field: path",
			expected:    "",
		},
		{
			name:        "execute read file tool with empty JSON",
			tool:        &mockReadFileTool{},
			input:       json.RawMessage(`{}`),
			expectError: true,
			errorMsg:    "missing required field: path",
			expected:    "",
		},
		{
			name:        "execute list files tool",
			tool:        &mockListFilesTool{},
			input:       json.RawMessage(`{"path": "/tmp"}`),
			expectError: false,
			errorMsg:    "",
			expected:    `["file1.txt", "file2.txt", "dir1/"]`,
		},
		{
			name:        "execute edit file tool",
			tool:        &mockEditFileTool{},
			input:       json.RawMessage(`{"path": "test.txt", "old_str": "hello", "new_str": "hello world"}`),
			expectError: false,
			errorMsg:    "",
			expected:    "OK",
		},
		{
			name:        "execute tool that returns error",
			tool:        &mockErrorTool{},
			input:       json.RawMessage(`{}`),
			expectError: true,
			errorMsg:    "tool execution failed",
			expected:    "",
		},
		{
			name:        "execute tool with malformed JSON",
			tool:        &mockReadFileTool{},
			input:       json.RawMessage(`{invalid json}`),
			expectError: true,
			errorMsg:    "invalid input JSON",
			expected:    "",
		},
		{
			name:        "execute tool with large input",
			tool:        &mockReadFileTool{},
			input:       json.RawMessage(`{"path": "` + string(make([]byte, 10000)) + `"}`),
			expectError: false,
			errorMsg:    "",
			expected:    "test file content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.tool.Execute(tt.input)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
				assert.Empty(t, result)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

// TestToolValidation tests tool validation logic
func TestToolValidation(t *testing.T) {
	tests := []struct {
		name        string
		tool        Tool
		expectValid bool
		errorMsg    string
	}{
		{
			name: "valid read file tool",
			tool: &mockReadFileTool{
				name:        "read_file",
				description: "Reads the contents of a file",
				schema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type": "string",
						},
					},
					"required": []string{"path"},
				},
			},
			expectValid: true,
			errorMsg:    "",
		},
		{
			name: "tool with empty name should be invalid",
			tool: &mockReadFileTool{
				name:        "",
				description: "Invalid tool",
				schema:      map[string]interface{}{},
			},
			expectValid: false,
			errorMsg:    "tool name cannot be empty",
		},
		{
			name: "tool with empty description should be invalid",
			tool: &mockReadFileTool{
				name:        "tool",
				description: "",
				schema:      map[string]interface{}{},
			},
			expectValid: false,
			errorMsg:    "tool description cannot be empty",
		},
		{
			name: "tool with nil schema should be invalid",
			tool: &mockReadFileTool{
				name:        "tool",
				description: "Tool with nil schema",
				schema:      nil,
			},
			expectValid: false,
			errorMsg:    "tool input schema cannot be nil",
		},
		{
			name: "tool with invalid schema should be invalid",
			tool: &mockReadFileTool{
				name:        "tool",
				description: "Tool with invalid schema",
				schema: map[string]interface{}{
					"invalid": "schema",
				},
			},
			expectValid: false,
			errorMsg:    "tool input schema must be type object",
		},
		{
			name: "valid tool with optional parameters",
			tool: &mockReadFileTool{
				name:        "list_files",
				description: "List files in directory",
				schema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"path": map[string]interface{}{
							"type":        "string",
							"description": "Directory path",
						},
					},
				},
			},
			expectValid: true,
			errorMsg:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateTool(tt.tool)

			if tt.expectValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			}
		})
	}
}

// TestToolSchemaValidation tests input validation against tool schemas
func TestToolSchemaValidation(t *testing.T) {
	tool := &mockReadFileTool{
		name:        "read_file",
		description: "Reads a file",
		schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path",
				},
				"offset": map[string]interface{}{
					"type":        "integer",
					"description": "Byte offset to start reading from",
				},
				"limit": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum number of bytes to read",
				},
			},
			"required": []string{"path"},
		},
	}

	tests := []struct {
		name        string
		input       json.RawMessage
		expectValid bool
		errorMsg    string
	}{
		{
			name:        "valid input with required field only",
			input:       json.RawMessage(`{"path": "test.txt"}`),
			expectValid: true,
			errorMsg:    "",
		},
		{
			name:        "valid input with all fields",
			input:       json.RawMessage(`{"path": "test.txt", "offset": 0, "limit": 100}`),
			expectValid: true,
			errorMsg:    "",
		},
		{
			name:        "missing required field",
			input:       json.RawMessage(`{"offset": 0}`),
			expectValid: false,
			errorMsg:    "missing required field: path",
		},
		{
			name:        "invalid field type",
			input:       json.RawMessage(`{"path": 123}`),
			expectValid: false,
			errorMsg:    "field 'path' must be string",
		},
		{
			name:        "additional properties should be valid",
			input:       json.RawMessage(`{"path": "test.txt", "extra": "field"}`),
			expectValid: false,
			errorMsg:    "unexpected field: extra",
		},
		{
			name:        "empty object should fail when required fields missing",
			input:       json.RawMessage(`{}`),
			expectValid: false,
			errorMsg:    "missing required field: path",
		},
		{
			name:        "null values for required fields",
			input:       json.RawMessage(`{"path": null}`),
			expectValid: false,
			errorMsg:    "field 'path' cannot be null",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateToolInput(tool, tt.input)

			if tt.expectValid {
				assert.NoError(t, err)
			} else {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errorMsg)
			}
		})
	}
}

// TestToolEquality tests equality comparison between tools
func TestToolEquality(t *testing.T) {
	schema1 := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{"type": "string"},
		},
		"required": []string{"path"},
	}

	schema2 := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{"type": "string"},
		},
		"required": []string{"path"},
	}

	schema3 := map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"content": map[string]interface{}{"type": "string"},
		},
		"required": []string{"content"},
	}

	tests := []struct {
		name     string
		tool1    Tool
		tool2    Tool
		areEqual bool
	}{
		{
			name: "identical tools should be equal",
			tool1: &mockReadFileTool{
				name:        "read_file",
				description: "Read a file",
				schema:      schema1,
			},
			tool2: &mockReadFileTool{
				name:        "read_file",
				description: "Read a file",
				schema:      schema2,
			},
			areEqual: true,
		},
		{
			name: "tools with different names should not be equal",
			tool1: &mockReadFileTool{
				name:        "read_file",
				description: "Read a file",
				schema:      schema1,
			},
			tool2: &mockReadFileTool{
				name:        "write_file",
				description: "Read a file",
				schema:      schema1,
			},
			areEqual: false,
		},
		{
			name: "tools with different descriptions should not be equal",
			tool1: &mockReadFileTool{
				name:        "read_file",
				description: "Read a file",
				schema:      schema1,
			},
			tool2: &mockReadFileTool{
				name:        "read_file",
				description: "Write a file",
				schema:      schema1,
			},
			areEqual: false,
		},
		{
			name: "tools with different schemas should not be equal",
			tool1: &mockReadFileTool{
				name:        "read_file",
				description: "Read a file",
				schema:      schema1,
			},
			tool2: &mockReadFileTool{
				name:        "read_file",
				description: "Read a file",
				schema:      schema3,
			},
			areEqual: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			equal := ToolsEqual(tt.tool1, tt.tool2)
			assert.Equal(t, tt.areEqual, equal)
		})
	}
}

// TestToolExecutionTimeout tests tool execution with timeout
func TestToolExecutionTimeout(t *testing.T) {
	tool := &mockSlowTool{
		delay: 2 * time.Second,
	}

	// Test with short timeout
	timeout := 100 * time.Millisecond
	input := json.RawMessage(`{}`)

	start := time.Now()
	_, err := ExecuteToolWithTimeout(tool, input, timeout)
	duration := time.Since(start)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "timeout")
	assert.Less(t, duration, timeout+100*time.Millisecond) // Should timeout quickly
}

// TestConcurrentToolExecution tests concurrent tool execution
func TestConcurrentToolExecution(t *testing.T) {
	tool := &mockCounterTool{}

	const numGoroutines = 10
	const executionsPerGoroutine = 5

	results := make(chan string, numGoroutines*executionsPerGoroutine)
	errors := make(chan error, numGoroutines*executionsPerGoroutines)

	// Launch multiple goroutines executing the tool
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < executionsPerGoroutine; j++ {
				input := json.RawMessage(fmt.Sprintf(`{"execution_id": "%d-%d"}`, goroutineID, j))
				result, err := tool.Execute(input)
				if err != nil {
					errors <- err
				} else {
					results <- result
				}
			}
		}(i)
	}

	// Collect results
	var allResults []string
	for i := 0; i < numGoroutines*executionsPerGoroutine; i++ {
		select {
		case result := <-results:
			allResults = append(allResults, result)
		case err := <-errors:
			t.Errorf("Unexpected error: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for tool executions")
		}
	}

	// Verify all executions succeeded
	assert.Len(t, allResults, numGoroutines*executionsPerGoroutine)

	// Verify all results are unique (no race conditions)
	uniqueResults := make(map[string]bool)
	for _, result := range allResults {
		assert.False(t, uniqueResults[result], "Duplicate result found: %s", result)
		uniqueResults[result] = true
	}
	assert.Len(t, uniqueResults, numGoroutines*executionsPerGoroutine)
}

// TestToolMetadata tests accessing tool metadata
func TestToolMetadata(t *testing.T) {
	tool := &mockReadFileTool{
		name:        "read_file",
		description: "Read the contents of a file",
		schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Path to the file to read",
				},
			},
			"required": []string{"path"},
		},
	}

	metadata := GetToolMetadata(tool)

	assert.Equal(t, "read_file", metadata["name"])
	assert.Equal(t, "Read the contents of a file", metadata["description"])
	assert.NotNil(t, metadata["input_schema"])
	assert.Equal(t, "object", metadata["input_schema"].(map[string]interface{})["type"])
}

// TestToolExecutionMetrics tests tool execution metrics collection
func TestToolExecutionMetrics(t *testing.T) {
	tool := &mockMetricsTool{}

	input := json.RawMessage(`{"test": "data"}`)

	// Execute tool multiple times
	for i := 0; i < 5; i++ {
		_, err := tool.Execute(input)
		assert.NoError(t, err)
	}

	metrics := tool.GetMetrics()
	assert.Equal(t, 5, metrics["execution_count"])
	assert.Greater(t, metrics["total_duration_ms"], int64(0))
	assert.GreaterOrEqual(t, metrics["average_duration_ms"], float64(0))
}

// Mock implementations for testing

type mockReadFileTool struct {
	name        string
	description string
	schema      map[string]interface{}
}

func (t *mockReadFileTool) GetName() string {
	if t.name != "" {
		return t.name
	}
	return "read_file"
}

func (t *mockReadFileTool) GetDescription() string {
	if t.description != "" {
		return t.description
	}
	return "Reads the contents of a file"
}

func (t *mockReadFileTool) GetInputSchema() map[string]interface{} {
	if t.schema != nil {
		return t.schema
	}
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Path to the file to read",
			},
		},
		"required": []string{"path"},
	}
}

func (t *mockReadFileTool) Execute(input json.RawMessage) (string, error) {
	var req struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(input, &req); err != nil {
		return "", fmt.Errorf("invalid input JSON: %w", err)
	}

	if req.Path == "" {
		return "", fmt.Errorf("missing required field: path")
	}

	// Mock successful read
	return "test file content", nil
}

type mockListFilesTool struct{}

func (t *mockListFilesTool) GetName() string        { return "list_files" }
func (t *mockListFilesTool) GetDescription() string { return "Lists files and directories" }
func (t *mockListFilesTool) GetInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path": map[string]interface{}{
				"type":        "string",
				"description": "Directory path",
			},
		},
	}
}

func (t *mockListFilesTool) Execute(input json.RawMessage) (string, error) {
	// Mock directory listing
	return `["file1.txt", "file2.txt", "dir1/"]`, nil
}

type mockEditFileTool struct{}

func (t *mockEditFileTool) GetName() string        { return "edit_file" }
func (t *mockEditFileTool) GetDescription() string { return "Makes edits to a text file" }
func (t *mockEditFileTool) GetInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"path":    map[string]interface{}{"type": "string"},
			"old_str": map[string]interface{}{"type": "string"},
			"new_str": map[string]interface{}{"type": "string"},
		},
		"required": []string{"path", "old_str", "new_str"},
	}
}

func (t *mockEditFileTool) Execute(input json.RawMessage) (string, error) {
	return "OK", nil
}

type mockErrorTool struct{}

func (t *mockErrorTool) GetName() string        { return "error_tool" }
func (t *mockErrorTool) GetDescription() string { return "Always returns an error" }
func (t *mockErrorTool) GetInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *mockErrorTool) Execute(json.RawMessage) (string, error) {
	return "", fmt.Errorf("tool execution failed")
}

type mockSlowTool struct {
	delay time.Duration
}

func (t *mockSlowTool) GetName() string        { return "slow_tool" }
func (t *mockSlowTool) GetDescription() string { return "Slow tool for testing timeouts" }
func (t *mockSlowTool) GetInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type":       "object",
		"properties": map[string]interface{}{},
	}
}

func (t *mockSlowTool) Execute(json.RawMessage) (string, error) {
	time.Sleep(t.delay)
	return "slow result", nil
}

type mockCounterTool struct{}

func (t *mockCounterTool) GetName() string        { return "counter_tool" }
func (t *mockCounterTool) GetDescription() string { return "Tool for testing concurrent access" }
func (t *mockCounterTool) GetInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"execution_id": map[string]interface{}{"type": "string"},
		},
	}
}

func (t *mockCounterTool) Execute(input json.RawMessage) (string, error) {
	var req struct {
		ExecutionID string `json:"execution_id"`
	}
	json.Unmarshal(input, &req)
	return fmt.Sprintf("executed-%s", req.ExecutionID), nil
}

type mockMetricsTool struct {
	executionCount int64
	totalDuration  int64
	mu             sync.RWMutex
}

func (t *mockMetricsTool) GetName() string        { return "metrics_tool" }
func (t *mockMetricsTool) GetDescription() string { return "Tool that tracks execution metrics" }
func (t *mockMetricsTool) GetInputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"test": map[string]interface{}{"type": "string"},
		},
	}
}

func (t *mockMetricsTool) Execute(input json.RawMessage) (string, error) {
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		t.mu.Lock()
		t.executionCount++
		t.totalDuration += duration.Nanoseconds()
		t.mu.Unlock()
	}()

	time.Sleep(10 * time.Millisecond) // Simulate some work
	return "metrics result", nil
}

func (t *mockMetricsTool) GetMetrics() map[string]interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	avgDuration := float64(0)
	if t.executionCount > 0 {
		avgDuration = float64(t.totalDuration) / float64(t.executionCount) / 1e6 // Convert to milliseconds
	}

	return map[string]interface{}{
		"execution_count":     t.executionCount,
		"total_duration_ms":   t.totalDuration / 1e6,
		"average_duration_ms": avgDuration,
	}
}

// Constructor functions that will fail
func NewReadFileTool() (Tool, error) {
	return nil, fmt.Errorf("Not implemented - this is a red phase test")
}

func NewListFilesTool() (Tool, error) {
	return nil, fmt.Errorf("Not implemented - this is a red phase test")
}

func NewEditFileTool() (Tool, error) {
	return nil, fmt.Errorf("Not implemented - this is a red phase test")
}

func NewCustomTool(definition json.RawMessage) (Tool, error) {
	return nil, fmt.Errorf("Not implemented - this is a red phase test")
}

// Utility functions that will fail
func ValidateTool(tool Tool) error {
	return fmt.Errorf("Not implemented - this is a red phase test")
}

func ValidateToolInput(tool Tool, input json.RawMessage) error {
	return fmt.Errorf("Not implemented - this is a red phase test")
}

func ToolsEqual(tool1, tool2 Tool) bool {
	return false
}

func ExecuteToolWithTimeout(tool Tool, input json.RawMessage, timeout time.Duration) (string, error) {
	return "", fmt.Errorf("Not implemented - this is a red phase test")
}

func GetToolMetadata(tool Tool) map[string]interface{} {
	return nil
}

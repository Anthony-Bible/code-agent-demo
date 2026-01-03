package tool

import (
	"code-editing-agent/internal/infrastructure/adapter/file"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// RED PHASE TDD TESTS - These tests WILL FAIL until batch_tool is implemented
// =============================================================================

func TestBatchTool_Registration(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	tools, err := adapter.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	found := false
	for _, tool := range tools {
		if tool.Name == "batch_tool" {
			found = true
			break
		}
	}

	if !found {
		t.Error("batch_tool should be registered")
	}

	// Verify the batch_tool has the correct schema
	batchTool, exists := adapter.GetTool("batch_tool")
	if !exists {
		t.Fatal("batch_tool should exist")
	}

	if !strings.Contains(batchTool.Description, "batch") {
		t.Errorf("Expected description to mention batch, got: %s", batchTool.Description)
	}

	// Verify required fields
	if len(batchTool.RequiredFields) != 1 || batchTool.RequiredFields[0] != "invocations" {
		t.Errorf("Expected required fields to be ['invocations'], got: %v", batchTool.RequiredFields)
	}
}

func TestBatchTool_BasicSequentialExecution(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create test files
	if err := fileManager.WriteFile("test1.txt", "content1"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer fileManager.DeleteFile("test1.txt")

	if err := fileManager.WriteFile("test2.txt", "content2"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer fileManager.DeleteFile("test2.txt")

	// Test batch execution of 2 read_file operations
	input := `{
		"invocations": [
			{
				"tool_name": "read_file",
				"arguments": {"path": "test1.txt"}
			},
			{
				"tool_name": "read_file",
				"arguments": {"path": "test2.txt"}
			}
		]
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify total invocations
	if output.TotalInvocations != 2 {
		t.Errorf("Expected total_invocations=2, got %d", output.TotalInvocations)
	}

	// Verify success count
	if output.SuccessCount != 2 {
		t.Errorf("Expected success_count=2, got %d", output.SuccessCount)
	}

	// Verify failed count
	if output.FailedCount != 0 {
		t.Errorf("Expected failed_count=0, got %d", output.FailedCount)
	}

	// Verify results array length
	if len(output.Results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(output.Results))
	}

	// Verify first result
	if output.Results[0].Index != 0 {
		t.Errorf("Expected first result index=0, got %d", output.Results[0].Index)
	}
	if output.Results[0].ToolName != "read_file" {
		t.Errorf("Expected first result tool_name='read_file', got %q", output.Results[0].ToolName)
	}
	if !output.Results[0].Success {
		t.Errorf("Expected first result success=true, got false")
	}
	if !strings.Contains(output.Results[0].Result, "content1") {
		t.Errorf("Expected first result to contain 'content1', got %q", output.Results[0].Result)
	}
	if output.Results[0].DurationMs <= 0 {
		t.Errorf("Expected first result duration_ms > 0, got %d", output.Results[0].DurationMs)
	}

	// Verify second result
	if output.Results[1].Index != 1 {
		t.Errorf("Expected second result index=1, got %d", output.Results[1].Index)
	}
	if output.Results[1].ToolName != "read_file" {
		t.Errorf("Expected second result tool_name='read_file', got %q", output.Results[1].ToolName)
	}
	if !output.Results[1].Success {
		t.Errorf("Expected second result success=true, got false")
	}
	if !strings.Contains(output.Results[1].Result, "content2") {
		t.Errorf("Expected second result to contain 'content2', got %q", output.Results[1].Result)
	}
	if output.Results[1].DurationMs <= 0 {
		t.Errorf("Expected second result duration_ms > 0, got %d", output.Results[1].DurationMs)
	}
}

func TestBatchTool_EmptyInvocations(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	input := `{"invocations": []}`
	_, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err == nil {
		t.Fatal("Expected error for empty invocations array, got nil")
	}

	if !strings.Contains(err.Error(), "empty") && !strings.Contains(err.Error(), "at least one") {
		t.Errorf("Expected error to mention empty invocations, got: %v", err)
	}
}

func TestBatchTool_MixedSuccessAndFailure(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create one test file
	if err := fileManager.WriteFile("exists.txt", "content"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer fileManager.DeleteFile("exists.txt")

	// Test batch with one success and one failure
	input := `{
		"invocations": [
			{
				"tool_name": "read_file",
				"arguments": {"path": "exists.txt"}
			},
			{
				"tool_name": "read_file",
				"arguments": {"path": "nonexistent.txt"}
			}
		]
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify counts
	if output.TotalInvocations != 2 {
		t.Errorf("Expected total_invocations=2, got %d", output.TotalInvocations)
	}
	if output.SuccessCount != 1 {
		t.Errorf("Expected success_count=1, got %d", output.SuccessCount)
	}
	if output.FailedCount != 1 {
		t.Errorf("Expected failed_count=1, got %d", output.FailedCount)
	}

	// Verify first result succeeded
	if !output.Results[0].Success {
		t.Errorf("Expected first result to succeed")
	}
	if output.Results[0].Error != "" {
		t.Errorf("Expected first result to have no error, got %q", output.Results[0].Error)
	}

	// Verify second result failed
	if output.Results[1].Success {
		t.Errorf("Expected second result to fail")
	}
	if output.Results[1].Error == "" {
		t.Errorf("Expected second result to have error message")
	}
	if output.Results[1].Result != "" {
		t.Errorf("Expected second result to have empty result, got %q", output.Results[1].Result)
	}
}

func TestBatchTool_MultipleToolTypes(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create test file
	if err := fileManager.WriteFile("test.txt", "test content"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer fileManager.DeleteFile("test.txt")

	// Test batch with different tool types
	input := `{
		"invocations": [
			{
				"tool_name": "read_file",
				"arguments": {"path": "test.txt"}
			},
			{
				"tool_name": "list_files",
				"arguments": {"path": "."}
			},
			{
				"tool_name": "bash",
				"arguments": {"command": "echo hello"}
			}
		]
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify all succeeded
	if output.TotalInvocations != 3 {
		t.Errorf("Expected total_invocations=3, got %d", output.TotalInvocations)
	}
	if output.SuccessCount != 3 {
		t.Errorf("Expected success_count=3, got %d", output.SuccessCount)
	}

	// Verify tool names
	if output.Results[0].ToolName != "read_file" {
		t.Errorf("Expected first tool_name='read_file', got %q", output.Results[0].ToolName)
	}
	if output.Results[1].ToolName != "list_files" {
		t.Errorf("Expected second tool_name='list_files', got %q", output.Results[1].ToolName)
	}
	if output.Results[2].ToolName != "bash" {
		t.Errorf("Expected third tool_name='bash', got %q", output.Results[2].ToolName)
	}
}

func TestBatchTool_MalformedInput(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	testCases := []struct {
		name  string
		input string
	}{
		{
			name:  "invalid json",
			input: `{"invocations": not valid json}`,
		},
		{
			name:  "invocations not array",
			input: `{"invocations": "not an array"}`,
		},
		{
			name:  "missing tool_name",
			input: `{"invocations": [{"arguments": {}}]}`,
		},
		{
			name:  "missing arguments",
			input: `{"invocations": [{"tool_name": "read_file"}]}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := adapter.ExecuteTool(context.Background(), "batch_tool", tc.input)
			if err == nil {
				t.Errorf("Expected error for %s, got nil", tc.name)
			}
		})
	}
}

func TestBatchTool_InvalidArgumentsForTool(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// read_file requires 'path' argument
	input := `{
		"invocations": [
			{
				"tool_name": "read_file",
				"arguments": {"wrong_field": "value"}
			}
		]
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Should report failure for individual invocation
	if output.FailedCount != 1 {
		t.Errorf("Expected failed_count=1, got %d", output.FailedCount)
	}
	if output.Results[0].Success {
		t.Errorf("Expected result to indicate failure")
	}
}

func TestBatchTool_SequentialExecutionOrder(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Test that batch executes sequentially by creating a file, then reading it
	input := `{
		"invocations": [
			{
				"tool_name": "edit_file",
				"arguments": {
					"path": "sequential_test.txt",
					"old_str": "",
					"new_str": "sequential content"
				}
			},
			{
				"tool_name": "read_file",
				"arguments": {"path": "sequential_test.txt"}
			}
		]
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}
	defer fileManager.DeleteFile("sequential_test.txt")

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Both should succeed
	if output.SuccessCount != 2 {
		t.Errorf("Expected success_count=2, got %d", output.SuccessCount)
	}

	// Second result should contain the content written by first
	if !strings.Contains(output.Results[1].Result, "sequential content") {
		t.Errorf("Expected second result to contain written content, got %q", output.Results[1].Result)
	}
}

func TestBatchTool_ContextCancellation(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create a context that will be cancelled immediately
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	// Wait for context to expire
	time.Sleep(10 * time.Millisecond)

	input := `{
		"invocations": [
			{
				"tool_name": "bash",
				"arguments": {"command": "sleep 1"}
			}
		]
	}`

	_, err := adapter.ExecuteTool(ctx, "batch_tool", input)
	if err == nil {
		t.Fatal("Expected error due to context cancellation, got nil")
	}

	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("Expected error to mention context/deadline, got: %v", err)
	}
}

func TestBatchTool_DurationTracking(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create test file
	if err := fileManager.WriteFile("duration_test.txt", "content"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer fileManager.DeleteFile("duration_test.txt")

	input := `{
		"invocations": [
			{
				"tool_name": "read_file",
				"arguments": {"path": "duration_test.txt"}
			}
		]
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify duration is tracked
	if output.Results[0].DurationMs <= 0 {
		t.Errorf("Expected duration_ms > 0, got %d", output.Results[0].DurationMs)
	}

	// Duration should be reasonable (less than 1 second for a simple read)
	if output.Results[0].DurationMs > 1000 {
		t.Errorf("Expected duration_ms < 1000ms for simple read, got %d", output.Results[0].DurationMs)
	}
}

func TestBatchTool_LargeNumberOfInvocations(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create test file
	if err := fileManager.WriteFile("large_batch.txt", "content"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer fileManager.DeleteFile("large_batch.txt")

	// Build a batch with 10 invocations
	invocations := make([]map[string]interface{}, 10)
	for i := range 10 {
		invocations[i] = map[string]interface{}{
			"tool_name": "read_file",
			"arguments": map[string]interface{}{
				"path": "large_batch.txt",
			},
		}
	}

	inputMap := map[string]interface{}{
		"invocations": invocations,
	}
	inputBytes, _ := json.Marshal(inputMap)

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", string(inputBytes))
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if output.TotalInvocations != 10 {
		t.Errorf("Expected total_invocations=10, got %d", output.TotalInvocations)
	}
	if output.SuccessCount != 10 {
		t.Errorf("Expected success_count=10, got %d", output.SuccessCount)
	}
	if len(output.Results) != 10 {
		t.Errorf("Expected 10 results, got %d", len(output.Results))
	}

	// Verify indices are sequential
	for i, result := range output.Results {
		if result.Index != i {
			t.Errorf("Expected result[%d].Index=%d, got %d", i, i, result.Index)
		}
	}
}

func TestBatchTool_NestedBatchNotAllowed(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Attempt to call batch_tool from within batch_tool
	input := `{
		"invocations": [
			{
				"tool_name": "batch_tool",
				"arguments": {
					"invocations": [
						{
							"tool_name": "bash",
							"arguments": {"command": "echo nested"}
						}
					]
				}
			}
		]
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Should report failure for attempting nested batch
	if output.FailedCount != 1 {
		t.Errorf("Expected failed_count=1 for nested batch attempt, got %d", output.FailedCount)
	}
	if !strings.Contains(output.Results[0].Error, "nested") && !strings.Contains(output.Results[0].Error, "recursive") {
		t.Errorf("Expected error to mention nested/recursive, got %q", output.Results[0].Error)
	}
}

func TestBatchTool_AllFailures(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// All invocations fail
	input := `{
		"invocations": [
			{
				"tool_name": "read_file",
				"arguments": {"path": "nonexistent1.txt"}
			},
			{
				"tool_name": "read_file",
				"arguments": {"path": "nonexistent2.txt"}
			}
		]
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if output.SuccessCount != 0 {
		t.Errorf("Expected success_count=0, got %d", output.SuccessCount)
	}
	if output.FailedCount != 2 {
		t.Errorf("Expected failed_count=2, got %d", output.FailedCount)
	}

	// Both results should have errors
	for i, result := range output.Results {
		if result.Success {
			t.Errorf("Expected result[%d] to fail", i)
		}
		if result.Error == "" {
			t.Errorf("Expected result[%d] to have error message", i)
		}
	}
}

// =============================================================================
// CYCLE 2 RED PHASE - Error Handling and Validation Tests
// =============================================================================

func TestBatchTool_MaxInvocationsLimit_ExceedsLimit(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	if err := fileManager.WriteFile("limit_test.txt", "content"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer fileManager.DeleteFile("limit_test.txt")

	invocations := make([]map[string]interface{}, 21)
	for i := range 21 {
		invocations[i] = map[string]interface{}{
			"tool_name": "read_file",
			"arguments": map[string]interface{}{"path": "limit_test.txt"},
		}
	}

	inputBytes, _ := json.Marshal(map[string]interface{}{"invocations": invocations})
	_, err := adapter.ExecuteTool(context.Background(), "batch_tool", string(inputBytes))
	if err == nil {
		t.Fatal("Expected error for exceeding 20 invocations limit, got nil")
	}

	errMsg := err.Error()
	if !strings.Contains(errMsg, "20") && !strings.Contains(errMsg, "limit") &&
		!strings.Contains(errMsg, "maximum") && !strings.Contains(errMsg, "too many") {
		t.Errorf("Expected error to mention limit/maximum/20/too many, got: %v", err)
	}
}

func TestBatchTool_MaxInvocationsLimit_ExactlyAtLimit(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	if err := fileManager.WriteFile("limit_test.txt", "content"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer fileManager.DeleteFile("limit_test.txt")

	invocations := make([]map[string]interface{}, 20)
	for i := range 20 {
		invocations[i] = map[string]interface{}{
			"tool_name": "read_file",
			"arguments": map[string]interface{}{"path": "limit_test.txt"},
		}
	}

	inputBytes, _ := json.Marshal(map[string]interface{}{"invocations": invocations})
	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", string(inputBytes))
	if err != nil {
		t.Fatalf("ExecuteTool failed for 20 invocations (should succeed): %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if output.TotalInvocations != 20 {
		t.Errorf("Expected total_invocations=20, got %d", output.TotalInvocations)
	}
	if output.SuccessCount != 20 {
		t.Errorf("Expected success_count=20, got %d", output.SuccessCount)
	}
}

func TestBatchTool_ValidationErrors_TableDriven(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	tests := []struct {
		name           string
		input          string
		expectedErrKey string // Key phrase expected in error message
	}{
		{
			name:           "null invocations",
			input:          `{"invocations": null}`,
			expectedErrKey: "invocations",
		},
		{
			name:           "invocations is string",
			input:          `{"invocations": "not an array"}`,
			expectedErrKey: "invocations",
		},
		{
			name:           "invocations is object",
			input:          `{"invocations": {"key": "value"}}`,
			expectedErrKey: "invocations",
		},
		{
			name:           "empty object",
			input:          `{}`,
			expectedErrKey: "invocations",
		},
		{
			name:           "completely invalid json",
			input:          `this is not json at all`,
			expectedErrKey: "invalid", // Matches "invalid input" error
		},
		{
			name:           "empty string",
			input:          ``,
			expectedErrKey: "empty", // Matches "cannot be empty" error
		},
		{
			name:           "invocation with empty tool_name",
			input:          `{"invocations": [{"tool_name": "", "arguments": {}}]}`,
			expectedErrKey: "tool_name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := adapter.ExecuteTool(context.Background(), "batch_tool", tt.input)
			if err == nil {
				t.Fatalf("Expected error for %s, got nil", tt.name)
			}

			errMsg := strings.ToLower(err.Error())
			expectedKey := strings.ToLower(tt.expectedErrKey)
			if !strings.Contains(errMsg, expectedKey) {
				t.Errorf("Expected error to contain %q, got: %v", tt.expectedErrKey, err)
			}
		})
	}
}

func TestBatchTool_RecursionPrevention_DirectNesting(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	input := `{
		"invocations": [{
			"tool_name": "batch_tool",
			"arguments": {
				"invocations": [{"tool_name": "bash", "arguments": {"command": "echo nested"}}]
			}
		}]
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool should not fail at top level: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if output.FailedCount != 1 || output.TotalInvocations != 1 {
		t.Errorf("Expected 1 failed invocation, got failed=%d total=%d", output.FailedCount, output.TotalInvocations)
	}

	errMsg := strings.ToLower(output.Results[0].Error)
	if !strings.Contains(errMsg, "nested") && !strings.Contains(errMsg, "recursive") {
		t.Errorf("Expected error to mention nested/recursive, got %q", output.Results[0].Error)
	}
}

func TestBatchTool_RecursionPrevention_MultipleBatchCalls(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	input := `{
		"invocations": [
			{
				"tool_name": "batch_tool",
				"arguments": {"invocations": [{"tool_name": "bash", "arguments": {"command": "echo 1"}}]}
			},
			{
				"tool_name": "batch_tool",
				"arguments": {"invocations": [{"tool_name": "bash", "arguments": {"command": "echo 2"}}]}
			}
		]
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool should not fail at top level: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if output.FailedCount != 2 || output.TotalInvocations != 2 {
		t.Errorf("Expected 2 failed invocations, got failed=%d total=%d", output.FailedCount, output.TotalInvocations)
	}

	for i, res := range output.Results {
		errMsg := strings.ToLower(res.Error)
		if !strings.Contains(errMsg, "nested") && !strings.Contains(errMsg, "recursive") {
			t.Errorf("Expected result[%d] error to mention nested/recursive, got %q", i, res.Error)
		}
	}
}

// =============================================================================
// CYCLE 3 RED PHASE - Comprehensive stop_on_error Behavior Tests
// =============================================================================

func TestBatchTool_StopOnError_True(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	if err := fileManager.WriteFile("success.txt", "content"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer fileManager.DeleteFile("success.txt")

	// Test that stop_on_error stops after FIRST failure
	// 5 invocations: success, success, FAIL, would-fail, would-fail
	input := `{
		"invocations": [
			{"tool_name": "read_file", "arguments": {"path": "success.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "success.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "nonexistent1.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "nonexistent2.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "nonexistent3.txt"}}
		],
		"stop_on_error": true
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(output.Results) != 3 || output.SuccessCount != 2 || output.FailedCount != 1 {
		t.Errorf("Expected 3 results (2 success, 1 fail), got results=%d success=%d failed=%d",
			len(output.Results), output.SuccessCount, output.FailedCount)
	}
	if !output.StoppedEarly {
		t.Errorf("Expected stopped_early=true, got false")
	}
}

func TestBatchTool_StopOnError_False(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	if err := fileManager.WriteFile("exists.txt", "content"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer fileManager.DeleteFile("exists.txt")

	// Test that stop_on_error: false continues through all failures
	input := `{
		"invocations": [
			{"tool_name": "read_file", "arguments": {"path": "exists.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "nonexistent1.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "exists.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "nonexistent2.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "exists.txt"}}
		],
		"stop_on_error": false
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(output.Results) != 5 || output.SuccessCount != 3 || output.FailedCount != 2 {
		t.Errorf("Expected 5 results (3 success, 2 fail), got results=%d success=%d failed=%d",
			len(output.Results), output.SuccessCount, output.FailedCount)
	}
	if output.StoppedEarly {
		t.Errorf("Expected stopped_early=false, got true")
	}
}

func TestBatchTool_StopOnError_Default(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	if err := fileManager.WriteFile("default_test.txt", "content"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer fileManager.DeleteFile("default_test.txt")

	// Test that default (omitted) stop_on_error continues through failures
	input := `{
		"invocations": [
			{"tool_name": "read_file", "arguments": {"path": "default_test.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "nonexistent.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "default_test.txt"}}
		]
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(output.Results) != 3 || output.SuccessCount != 2 || output.FailedCount != 1 {
		t.Errorf("Expected 3 results (2 success, 1 fail), got results=%d success=%d failed=%d",
			len(output.Results), output.SuccessCount, output.FailedCount)
	}
	if output.StoppedEarly {
		t.Errorf("Expected stopped_early=false (default behavior), got true")
	}
}

func TestBatchTool_StoppedEarlyFlag(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create test file
	if err := fileManager.WriteFile("flag_test.txt", "content"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer fileManager.DeleteFile("flag_test.txt")

	// Table-driven test to verify stopped_early flag is set correctly
	tests := []struct {
		name            string
		input           string
		expectedStopped bool
		expectedResults int
		expectedSuccess int
		expectedFailed  int
		description     string
	}{
		{
			name: "stop_on_error true with error",
			input: `{
				"invocations": [
					{"tool_name": "read_file", "arguments": {"path": "nonexistent.txt"}},
					{"tool_name": "read_file", "arguments": {"path": "flag_test.txt"}}
				],
				"stop_on_error": true
			}`,
			expectedStopped: true,
			expectedResults: 1,
			expectedSuccess: 0,
			expectedFailed:  1,
			description:     "Should stop and set stopped_early=true when error occurs with stop_on_error=true",
		},
		{
			name: "stop_on_error true without error",
			input: `{
				"invocations": [
					{"tool_name": "read_file", "arguments": {"path": "flag_test.txt"}},
					{"tool_name": "read_file", "arguments": {"path": "flag_test.txt"}}
				],
				"stop_on_error": true
			}`,
			expectedStopped: false,
			expectedResults: 2,
			expectedSuccess: 2,
			expectedFailed:  0,
			description:     "Should not set stopped_early when all succeed even with stop_on_error=true",
		},
		{
			name: "stop_on_error false with error",
			input: `{
				"invocations": [
					{"tool_name": "read_file", "arguments": {"path": "nonexistent.txt"}},
					{"tool_name": "read_file", "arguments": {"path": "flag_test.txt"}}
				],
				"stop_on_error": false
			}`,
			expectedStopped: false,
			expectedResults: 2,
			expectedSuccess: 1,
			expectedFailed:  1,
			description:     "Should not set stopped_early when stop_on_error=false even with errors",
		},
		{
			name: "stop_on_error false without error",
			input: `{
				"invocations": [
					{"tool_name": "read_file", "arguments": {"path": "flag_test.txt"}},
					{"tool_name": "read_file", "arguments": {"path": "flag_test.txt"}}
				],
				"stop_on_error": false
			}`,
			expectedStopped: false,
			expectedResults: 2,
			expectedSuccess: 2,
			expectedFailed:  0,
			description:     "Should not set stopped_early when all succeed with stop_on_error=false",
		},
		{
			name: "default (omitted) with error",
			input: `{
				"invocations": [
					{"tool_name": "read_file", "arguments": {"path": "nonexistent.txt"}},
					{"tool_name": "read_file", "arguments": {"path": "flag_test.txt"}}
				]
			}`,
			expectedStopped: false,
			expectedResults: 2,
			expectedSuccess: 1,
			expectedFailed:  1,
			description:     "Should not set stopped_early when stop_on_error is omitted (defaults to false)",
		},
		{
			name: "stop_on_error true with error after success",
			input: `{
				"invocations": [
					{"tool_name": "read_file", "arguments": {"path": "flag_test.txt"}},
					{"tool_name": "read_file", "arguments": {"path": "nonexistent.txt"}},
					{"tool_name": "read_file", "arguments": {"path": "flag_test.txt"}}
				],
				"stop_on_error": true
			}`,
			expectedStopped: true,
			expectedResults: 2,
			expectedSuccess: 1,
			expectedFailed:  1,
			description:     "Should stop after second invocation fails with stop_on_error=true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := adapter.ExecuteTool(context.Background(), "batch_tool", tt.input)
			if err != nil {
				t.Fatalf("ExecuteTool failed: %v", err)
			}

			var output batchToolOutput
			if err := json.Unmarshal([]byte(result), &output); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			// Verify stopped_early flag
			if output.StoppedEarly != tt.expectedStopped {
				t.Errorf("%s: Expected stopped_early=%v, got %v",
					tt.description, tt.expectedStopped, output.StoppedEarly)
			}

			// Verify result count
			if len(output.Results) != tt.expectedResults {
				t.Errorf("%s: Expected %d results, got %d",
					tt.description, tt.expectedResults, len(output.Results))
			}

			// Verify success/failed counts
			if output.SuccessCount != tt.expectedSuccess {
				t.Errorf("%s: Expected success_count=%d, got %d",
					tt.description, tt.expectedSuccess, output.SuccessCount)
			}
			if output.FailedCount != tt.expectedFailed {
				t.Errorf("%s: Expected failed_count=%d, got %d",
					tt.description, tt.expectedFailed, output.FailedCount)
			}
		})
	}
}

// =============================================================================
// CYCLE 4 RED PHASE - Parallel Execution Tests
// =============================================================================

func TestBatchTool_ParallelExecution_Basic(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Test parallel execution with sleep commands
	// If executed sequentially: total time = 0.3 + 0.3 + 0.3 + 0.3 = 1.2 seconds
	// If executed in parallel: total time should be ~0.3 seconds (plus overhead)
	input := `{
		"invocations": [
			{"tool_name": "bash", "arguments": {"command": "sleep 0.3"}},
			{"tool_name": "bash", "arguments": {"command": "sleep 0.3"}},
			{"tool_name": "bash", "arguments": {"command": "sleep 0.3"}},
			{"tool_name": "bash", "arguments": {"command": "sleep 0.3"}}
		],
		"parallel": true
	}`

	start := time.Now()
	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify all invocations completed successfully
	if output.TotalInvocations != 4 {
		t.Errorf("Expected total_invocations=4, got %d", output.TotalInvocations)
	}
	if output.SuccessCount != 4 {
		t.Errorf("Expected success_count=4, got %d", output.SuccessCount)
	}
	if len(output.Results) != 4 {
		t.Fatalf("Expected 4 results, got %d", len(output.Results))
	}

	// Verify parallel execution by checking total elapsed time
	// Should be much less than 1.2s (sequential), with generous margin for CI/flakiness
	// We expect around 0.3s for parallel, allow up to 0.8s to account for overhead
	maxSequentialTime := 1200 * time.Millisecond
	maxParallelTime := 800 * time.Millisecond

	if elapsed >= maxSequentialTime {
		t.Errorf("Execution appears sequential (took %v), expected parallel execution (~300ms)", elapsed)
	}

	if elapsed > maxParallelTime {
		t.Logf("Warning: Parallel execution took %v (expected ~300ms, allowed up to %v)",
			elapsed, maxParallelTime)
	}

	// All results should be present and successful
	for i, res := range output.Results {
		if !res.Success {
			t.Errorf("Expected result[%d] to succeed, got error: %s", i, res.Error)
		}
		if res.Index != i {
			t.Errorf("Expected result[%d].Index=%d, got %d", i, i, res.Index)
		}
		if res.ToolName != "bash" {
			t.Errorf("Expected result[%d].ToolName='bash', got %q", i, res.ToolName)
		}
	}
}

func TestBatchTool_ParallelExecution_MaintainsOrder(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create test files
	testFiles := []string{"parallel_order1.txt", "parallel_order2.txt", "parallel_order3.txt"}
	testContents := []string{"content1", "content2", "content3"}
	for i, filename := range testFiles {
		if err := fileManager.WriteFile(filename, testContents[i]); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
		defer fileManager.DeleteFile(filename)
	}

	// Execute different tools in parallel
	// Even though they execute in parallel, results should maintain invocation order
	input := `{
		"invocations": [
			{"tool_name": "read_file", "arguments": {"path": "parallel_order1.txt"}},
			{"tool_name": "bash", "arguments": {"command": "echo invocation2"}},
			{"tool_name": "read_file", "arguments": {"path": "parallel_order2.txt"}},
			{"tool_name": "bash", "arguments": {"command": "echo invocation4"}},
			{"tool_name": "read_file", "arguments": {"path": "parallel_order3.txt"}}
		],
		"parallel": true
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify all succeeded
	if output.TotalInvocations != 5 {
		t.Errorf("Expected total_invocations=5, got %d", output.TotalInvocations)
	}
	if output.SuccessCount != 5 {
		t.Errorf("Expected success_count=5, got %d", output.SuccessCount)
	}
	if len(output.Results) != 5 {
		t.Fatalf("Expected 5 results, got %d", len(output.Results))
	}

	// Define expected results
	expectedResults := []struct {
		index         int
		toolName      string
		shouldContain string
	}{
		{0, "read_file", "content1"},
		{1, "bash", "invocation2"},
		{2, "read_file", "content2"},
		{3, "bash", "invocation4"},
		{4, "read_file", "content3"},
	}

	// Verify results maintain order (index and content match invocation order)
	for _, expected := range expectedResults {
		result := output.Results[expected.index]

		if result.Index != expected.index {
			t.Errorf("Expected results[%d].Index=%d, got %d",
				expected.index, expected.index, result.Index)
		}
		if result.ToolName != expected.toolName {
			t.Errorf("Expected results[%d].ToolName=%q, got %q",
				expected.index, expected.toolName, result.ToolName)
		}
		if !strings.Contains(result.Result, expected.shouldContain) {
			t.Errorf("Expected results[%d] to contain %q, got %q",
				expected.index, expected.shouldContain, result.Result)
		}
	}
}

func TestBatchTool_ParallelExecution_WithFailures(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create one test file for successes
	if err := fileManager.WriteFile("parallel_success.txt", "success content"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer fileManager.DeleteFile("parallel_success.txt")

	// Test parallel execution with mixed success and failure
	// In parallel mode, ALL should execute despite failures (stop_on_error doesn't apply)
	input := `{
		"invocations": [
			{"tool_name": "read_file", "arguments": {"path": "parallel_success.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "nonexistent1.txt"}},
			{"tool_name": "bash", "arguments": {"command": "echo success"}},
			{"tool_name": "read_file", "arguments": {"path": "nonexistent2.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "parallel_success.txt"}}
		],
		"parallel": true
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Verify ALL invocations were executed (parallel doesn't stop on error)
	if output.TotalInvocations != 5 {
		t.Errorf("Expected total_invocations=5, got %d", output.TotalInvocations)
	}
	if len(output.Results) != 5 {
		t.Errorf("Expected 5 results (all should execute in parallel), got %d", len(output.Results))
	}

	// Verify counts
	if output.SuccessCount != 3 {
		t.Errorf("Expected success_count=3, got %d", output.SuccessCount)
	}
	if output.FailedCount != 2 {
		t.Errorf("Expected failed_count=2, got %d", output.FailedCount)
	}

	// Verify specific results
	if !output.Results[0].Success {
		t.Errorf("Expected result[0] to succeed")
	}
	if output.Results[1].Success {
		t.Errorf("Expected result[1] to fail (nonexistent file)")
	}
	if output.Results[1].Error == "" {
		t.Errorf("Expected result[1] to have error message")
	}
	if !output.Results[2].Success {
		t.Errorf("Expected result[2] to succeed")
	}
	if output.Results[3].Success {
		t.Errorf("Expected result[3] to fail (nonexistent file)")
	}
	if output.Results[3].Error == "" {
		t.Errorf("Expected result[3] to have error message")
	}
	if !output.Results[4].Success {
		t.Errorf("Expected result[4] to succeed")
	}

	// In parallel mode, stopped_early should be false even with errors
	if output.StoppedEarly {
		t.Errorf("Expected stopped_early=false in parallel mode, got true")
	}
}

func TestBatchTool_ParallelExecution_ContextCancellation(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create a context that will be cancelled after a short time
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// Multiple long-running operations (longer than context timeout)
	input := `{
		"invocations": [
			{"tool_name": "bash", "arguments": {"command": "sleep 5"}},
			{"tool_name": "bash", "arguments": {"command": "sleep 5"}},
			{"tool_name": "bash", "arguments": {"command": "sleep 5"}},
			{"tool_name": "bash", "arguments": {"command": "sleep 5"}}
		],
		"parallel": true
	}`

	_, err := adapter.ExecuteTool(ctx, "batch_tool", input)
	if err == nil {
		t.Fatal("Expected error due to context cancellation, got nil")
	}

	// Should mention context/timeout/deadline/cancelled
	errMsg := strings.ToLower(err.Error())
	if !strings.Contains(errMsg, "context") &&
		!strings.Contains(errMsg, "deadline") &&
		!strings.Contains(errMsg, "timeout") &&
		!strings.Contains(errMsg, "cancel") {
		t.Errorf("Expected error to mention context/deadline/timeout/cancel, got: %v", err)
	}
}

func TestBatchTool_ParallelMode_Defaults(t *testing.T) {
	tests := []struct {
		name            string
		parallelField   string // "" for omitted, "false" for explicit
		minExpectedTime time.Duration
	}{
		{
			name:            "default (omitted) is sequential",
			parallelField:   "",
			minExpectedTime: 500 * time.Millisecond,
		},
		{
			name:            "explicit false is sequential",
			parallelField:   `,"parallel": false`,
			minExpectedTime: 500 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileManager := file.NewLocalFileManager(".")
			adapter := NewExecutorAdapter(fileManager)

			input := `{
				"invocations": [
					{"tool_name": "bash", "arguments": {"command": "sleep 0.2"}},
					{"tool_name": "bash", "arguments": {"command": "sleep 0.2"}},
					{"tool_name": "bash", "arguments": {"command": "sleep 0.2"}}
				]` + tt.parallelField + `
			}`

			start := time.Now()
			result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("ExecuteTool failed: %v", err)
			}

			var output batchToolOutput
			if err := json.Unmarshal([]byte(result), &output); err != nil {
				t.Fatalf("Failed to unmarshal result: %v", err)
			}

			if output.SuccessCount != 3 {
				t.Errorf("Expected success_count=3, got %d", output.SuccessCount)
			}

			if elapsed < tt.minExpectedTime {
				t.Errorf(
					"Execution appears parallel (took %v), expected sequential (>= %v)",
					elapsed,
					tt.minExpectedTime,
				)
			}
		})
	}
}

func TestBatchTool_ParallelWithStopOnError_IgnoresStopOnError(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	if err := fileManager.WriteFile("parallel_stop_test.txt", "content"); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer func() { _ = fileManager.DeleteFile("parallel_stop_test.txt") }()

	// Test that parallel: true ignores stop_on_error
	input := `{
		"invocations": [
			{"tool_name": "read_file", "arguments": {"path": "parallel_stop_test.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "nonexistent.txt"}},
			{"tool_name": "read_file", "arguments": {"path": "parallel_stop_test.txt"}},
			{"tool_name": "bash", "arguments": {"command": "echo after error"}}
		],
		"parallel": true,
		"stop_on_error": true
	}`

	result, err := adapter.ExecuteTool(context.Background(), "batch_tool", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output batchToolOutput
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if len(output.Results) != 4 || output.SuccessCount != 3 || output.FailedCount != 1 {
		t.Errorf("Expected 4 results (3 success, 1 fail), got results=%d success=%d failed=%d",
			len(output.Results), output.SuccessCount, output.FailedCount)
	}

	if output.StoppedEarly {
		t.Errorf("Expected stopped_early=false in parallel mode, got true")
	}
}

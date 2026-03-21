package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
)

// batchToolInput represents the input for the batch_tool tool.
type batchToolInput struct {
	Invocations []batchInvocation `json:"invocations"`
	Parallel    bool              `json:"parallel,omitempty"`
	StopOnError bool              `json:"stop_on_error,omitempty"`
}

// batchInvocation represents a single tool invocation in a batch.
type batchInvocation struct {
	ToolName  string          `json:"tool_name"`
	Arguments json.RawMessage `json:"arguments"`
}

// batchToolOutput represents the output from the batch_tool tool.
type batchToolOutput struct {
	TotalInvocations int               `json:"total_invocations"`
	SuccessCount     int               `json:"success_count"`
	FailedCount      int               `json:"failed_count"`
	Results          []batchToolResult `json:"results"`
	StoppedEarly     bool              `json:"stopped_early,omitempty"`
}

// batchToolResult represents the result of a single tool execution in a batch.
type batchToolResult struct {
	Index      int    `json:"index"`
	ToolName   string `json:"tool_name"`
	Success    bool   `json:"success"`
	Result     string `json:"result,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// maxBatchInvocations is the maximum number of tool invocations allowed in a single batch.
const maxBatchInvocations = 20

// registerPlanAndBatchTools registers plan mode and batch tools.
func (a *ExecutorAdapter) registerPlanAndBatchTools() {
	// Register enter_plan_mode tool
	enterPlanModeTool := entity.Tool{
		ID:   "enter_plan_mode",
		Name: "enter_plan_mode",
		Description: `Use this tool proactively when you're about to start a non-trivial implementation task. Getting user sign-off on your approach before writing code prevents wasted effort and ensures alignment.

## When to Use This Tool

Use enter_plan_mode when ANY of these conditions apply:

1. **New Feature Implementation**: Adding meaningful new functionality
2. **Multiple Valid Approaches**: The task can be solved in several different ways
3. **Code Modifications**: Changes that affect existing behavior or structure
4. **Architectural Decisions**: The task requires choosing between patterns or technologies
5. **Multi-File Changes**: The task will likely touch more than 2-3 files
6. **Unclear Requirements**: You need to explore before understanding the full scope

## When NOT to Use This Tool

- Single-line or few-line fixes (typos, obvious bugs, small tweaks)
- Adding a single function with clear requirements
- Tasks where the user has given very specific, detailed instructions
- Pure research/exploration tasks

## What Happens in Plan Mode

In plan mode, you will:
1. Explore the codebase using read_file, list_files, and read-only bash commands
2. Mutating tools (edit_file, write commands) will write proposals to a plan file instead of executing
3. Design an implementation approach and present it to the user
4. Exit plan mode when ready to implement (user command)`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"reason": map[string]interface{}{
					"type":        "string",
					"description": "Brief explanation of why plan mode is needed for this task",
				},
			},
			"required": []string{"reason"},
		},
		RequiredFields: []string{"reason"},
	}
	a.tools[enterPlanModeTool.Name] = enterPlanModeTool

	// Register batch_tool
	batchToolTool := entity.Tool{
		ID:   "batch_tool",
		Name: "batch_tool",
		Description: `Execute multiple tool invocations in a single batch operation. Prefer this when running multiple tools.

Use this tool when you need to:
- Execute the same operation on multiple items
- Run multiple independent tool calls efficiently
- Perform a sequence of operations that should be tracked together

The tool supports both sequential and parallel execution modes:
- Sequential (default): Executes invocations one at a time, optionally stopping on first error
- Parallel: Executes all invocations concurrently for maximum performance

The tool returns aggregated results showing success/failure counts and individual results for each invocation.`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"invocations": map[string]interface{}{
					"type":        "array",
					"description": "List of tool invocations to execute",
					"items": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"tool_name": map[string]interface{}{
								"type":        "string",
								"description": "Name of the tool to invoke",
							},
							"arguments": map[string]interface{}{
								"type":        "object",
								"description": "Arguments to pass to the tool",
							},
						},
						"required": []string{"tool_name", "arguments"},
					},
					"maxItems": maxBatchInvocations,
					"minItems": 1,
				},
				"parallel": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to execute invocations in parallel (default: false)",
				},
				"stop_on_error": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether to stop execution on first error (only applies to sequential mode)",
				},
			},
			"required": []string{"invocations"},
		},
		RequiredFields: []string{"invocations"},
	}
	a.tools[batchToolTool.Name] = batchToolTool
}

// executeBatchTool executes the batch_tool tool.
func (a *ExecutorAdapter) executeBatchTool(ctx context.Context, input json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	var batchInput batchToolInput
	if err := json.Unmarshal(input, &batchInput); err != nil {
		return "", fmt.Errorf("failed to parse batch_tool input: %w", err)
	}

	// Validate invocations
	if err := validateBatchInvocations(batchInput.Invocations); err != nil {
		return "", err
	}

	// Initialize output
	output := batchToolOutput{
		TotalInvocations: len(batchInput.Invocations),
		Results:          make([]batchToolResult, 0, len(batchInput.Invocations)),
	}

	// Execute batch invocations
	if batchInput.Parallel {
		a.executeBatchParallel(ctx, batchInput.Invocations, &output)
		// Check if context was cancelled during parallel execution
		if err := ctx.Err(); err != nil {
			return "", err
		}
	} else {
		a.executeBatchSequential(ctx, batchInput.Invocations, batchInput.StopOnError, &output)
	}

	// Marshal output
	result, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal batch_tool output: %w", err)
	}

	return string(result), nil
}

// validateBatchInvocations validates the batch invocations input.
func validateBatchInvocations(invocations []batchInvocation) error {
	if len(invocations) == 0 {
		return errors.New("invocations must contain at least one tool invocation")
	}
	if len(invocations) > maxBatchInvocations {
		return fmt.Errorf("maximum %d invocations allowed, got %d", maxBatchInvocations, len(invocations))
	}

	for i, inv := range invocations {
		if inv.ToolName == "" {
			return fmt.Errorf("invocation[%d]: tool_name is required", i)
		}
		if inv.Arguments == nil {
			return fmt.Errorf("invocation[%d]: arguments is required", i)
		}
	}

	return nil
}

// executeBatchSequential executes batch invocations sequentially.
func (a *ExecutorAdapter) executeBatchSequential(
	ctx context.Context,
	invocations []batchInvocation,
	stopOnError bool,
	output *batchToolOutput,
) {
	for i, inv := range invocations {
		// Check context cancellation
		if err := ctx.Err(); err != nil {
			output.StoppedEarly = true
			return
		}

		result := a.executeSingleBatchInvocation(ctx, i, inv)
		output.Results = append(output.Results, result)

		// Update counts
		if result.Success {
			output.SuccessCount++
		} else {
			output.FailedCount++
			// Stop execution if stop_on_error is enabled
			if stopOnError {
				output.StoppedEarly = true
				return
			}
		}
	}
}

// executeBatchParallel executes batch invocations in parallel.
func (a *ExecutorAdapter) executeBatchParallel(
	ctx context.Context,
	invocations []batchInvocation,
	output *batchToolOutput,
) {
	var wg sync.WaitGroup
	var mu sync.Mutex

	// Pre-allocate results slice with correct length to maintain order
	output.Results = make([]batchToolResult, len(invocations))

	for i, inv := range invocations {
		wg.Add(1)
		go func(index int, invocation batchInvocation) {
			defer wg.Done()

			// Execute single invocation
			result := a.executeSingleBatchInvocation(ctx, index, invocation)

			// Store result at correct index and update counts with mutex
			mu.Lock()
			output.Results[index] = result
			if result.Success {
				output.SuccessCount++
			} else {
				output.FailedCount++
			}
			mu.Unlock()
		}(i, inv)
	}

	wg.Wait()
	output.StoppedEarly = false
}

// executeSingleBatchInvocation executes a single tool invocation within a batch.
func (a *ExecutorAdapter) executeSingleBatchInvocation(
	ctx context.Context,
	index int,
	inv batchInvocation,
) batchToolResult {
	result := batchToolResult{
		Index:    index,
		ToolName: inv.ToolName,
	}

	// Check for nested batch_tool invocations
	if inv.ToolName == "batch_tool" {
		result.Success = false
		result.Error = "nested batch_tool invocations are not allowed"
		result.DurationMs = 0
		return result
	}

	// Execute the tool and track duration
	startTime := time.Now()
	toolResult, err := a.executeByName(ctx, inv.ToolName, inv.Arguments)
	result.DurationMs = calculateDurationMs(startTime)

	if err != nil {
		result.Success = false
		result.Error = err.Error()
	} else {
		result.Success = true
		result.Result = toolResult
	}

	return result
}

// calculateDurationMs calculates the duration in milliseconds since startTime.
// Returns at least 1ms for very fast operations to ensure non-zero reporting.
func calculateDurationMs(startTime time.Time) int64 {
	duration := time.Since(startTime).Milliseconds()
	if duration == 0 {
		return 1
	}
	return duration
}

package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anthony-bible/code-agent-demo/internal/application/usecase"
	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// taskInput represents the input for the task tool.
type taskInput struct {
	AgentName string `json:"agent_name"`
	Prompt    string `json:"prompt"`
}

// delegateInput represents the input for the delegate tool.
type delegateInput struct {
	Name         string   `json:"name"`
	SystemPrompt string   `json:"system_prompt"`
	Task         string   `json:"task"`
	Model        string   `json:"model"`
	MaxActions   int      `json:"max_actions"`
	AllowedTools []string `json:"allowed_tools"`
}

// registerSubagentTools registers subagent-related tools.
func (a *ExecutorAdapter) registerSubagentTools() {
	// Register task tool (dynamically includes available agents if subagentManager is set)
	a.registerTaskTool()

	// Register delegate tool
	delegateTool := entity.Tool{
		ID:     "delegate",
		Name:   "delegate",
		Strict: true,
		Description: `Launch a dynamic agent to handle complex, multi-step tasks autonomously.

The delegate tool spawns a specialized agent (subprocess) that autonomously handles complex tasks in an isolated conversation context. You define the agent's role and behavior through a custom system prompt.

When to use the delegate tool:
- Complex multi-step tasks that would fill the context window
- Tasks requiring specialized focus or expertise you define
- Work that benefits from isolated context (e.g., analyzing large codebases)
- Breaking down larger problems into delegated subtasks

When NOT to use the delegate tool:
- Simple single-step operations (use tools directly)
- Tasks where you need to maintain conversation context
- Quick lookups or simple file reads

Usage notes:
- Provide a clear, detailed system_prompt defining the agent's role, approach, and expected output format
- The agent runs in its own conversation session - results are returned when done
- The agent's output is not visible to the user; summarize results in your response
- Use allowed_tools to restrict what the agent can do for safety
- Use max_actions to prevent runaway execution (default: 30)
- Model selection: haiku (fast), sonnet (balanced), opus (complex reasoning), inherit (same as parent)

Example system_prompt structure:
"You are a [role]. Your task is to:
1. [First step]
2. [Second step]
3. [Third step]

Focus on: [key areas]
Output format: [expected structure]"`,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{
					"type":        "string",
					"description": "Short identifier for the agent (3-5 words, for logging/tracking)",
				},
				"system_prompt": map[string]interface{}{
					"type":        "string",
					"description": "Instructions defining the agent's role, responsibilities, approach, and expected output format. Be detailed - this is the agent's only context about its purpose.",
				},
				"task": map[string]interface{}{
					"type":        "string",
					"description": "The specific task for the agent to complete. Provide all necessary context since the agent has no prior conversation history.",
				},
				"model": map[string]interface{}{
					"type":        "string",
					"enum":        []string{"haiku", "sonnet", "opus", "inherit"},
					"description": "AI model to use. haiku=fast/cheap, sonnet=balanced, opus=complex reasoning, inherit=same as parent (default: inherit)",
				},
				"max_actions": map[string]interface{}{
					"type":        "integer",
					"description": "Maximum tool calls before stopping. Prevents runaway execution (default: 30)",
				},
				"allowed_tools": map[string]interface{}{
					"type":        "array",
					"items":       map[string]interface{}{"type": "string"},
					"description": "Tools this agent can use. Omit for all tools, or specify a list to restrict capabilities for safety.",
				},
			},
			"required": []string{"name", "system_prompt", "task"},
		},
		RequiredFields: []string{"name", "system_prompt", "task"},
	}
	a.tools[delegateTool.Name] = delegateTool
}

// registerTaskTool registers the task tool with dynamic agent listing.
// If a subagentManager is available, it discovers agents and includes them in the tool description.
// This method is called during initialization and when SetSubagentManager is invoked.
func (a *ExecutorAdapter) registerTaskTool() {
	baseDescription := "Spawns a subagent to handle a delegated task. Returns the subagent's result when complete. Cannot be called from within a subagent (prevents recursion)."

	// Try to discover available agents if subagentManager is set
	var fullDescription strings.Builder
	fullDescription.WriteString(baseDescription)

	if a.subagentManager != nil {
		agents, err := a.subagentManager.DiscoverAgents(context.Background())
		if err == nil && agents.TotalCount > 0 {
			fullDescription.WriteString("\n\nAvailable agents:\n")
			for _, agent := range agents.Subagents {
				fmt.Fprintf(&fullDescription, "- %s: %s\n", agent.Name, agent.Description)
			}
		}
	}

	taskTool := entity.Tool{
		ID:          "task",
		Name:        "task",
		Description: fullDescription.String(),
		Strict:      true,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"agent_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the subagent to spawn (e.g., 'code-reviewer', 'test-writer')",
				},
				"prompt": map[string]interface{}{
					"type":        "string",
					"description": "Task description/instructions for the subagent to execute",
				},
			},
			"required": []string{"agent_name", "prompt"},
		},
		RequiredFields: []string{"agent_name", "prompt"},
	}
	a.tools[taskTool.Name] = taskTool
}

// checkSubagentPrerequisites validates that a subagent tool can be called.
// It checks for recursion prevention and use case availability.
func (a *ExecutorAdapter) checkSubagentPrerequisites(
	ctx context.Context,
	toolName string,
) (SubagentUseCaseInterface, error) {
	if port.IsSubagentContext(ctx) {
		return nil, fmt.Errorf(
			"%s tool cannot be called from within a subagent (prevents infinite recursion)",
			toolName,
		)
	}
	a.mu.RLock()
	uc := a.subagentUseCase
	a.mu.RUnlock()
	if uc == nil {
		return nil, errors.New("subagent use case not available")
	}
	return uc, nil
}

// formatSubagentResult formats a SubagentResult as indented JSON.
func formatSubagentResult(result *usecase.SubagentResult) (string, error) {
	if result == nil {
		return "", errors.New("subagent execution returned nil result")
	}
	resultJSON := map[string]any{
		"subagent_id":   result.SubagentID,
		"agent_name":    result.AgentName,
		"status":        result.Status,
		"output":        result.Output,
		"actions_taken": result.ActionsTaken,
		"duration_ms":   result.Duration.Milliseconds(),
	}
	if result.Error != nil {
		resultJSON["error"] = result.Error.Error()
	}
	resultBytes, err := json.MarshalIndent(resultJSON, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to format result: %w", err)
	}
	return string(resultBytes), nil
}

// executeTask spawns a subagent to handle a delegated task.
//
// This method implements the task tool, which allows the main agent to delegate
// work to specialized subagents. The execution flow is:
//
//  1. Recursion prevention: Blocks execution if called from within a subagent context
//  2. Availability check: Ensures the subagent use case has been configured
//  3. Input parsing: Extracts agent_name and prompt from the tool input
//  4. Input validation: Verifies required fields are non-empty
//  5. Subagent spawning: Delegates to the use case to create and run the subagent
//  6. Result formatting: Converts the SubagentResult to a JSON string
//
// Error cases:
//   - Returns "prevents infinite recursion" if called from a subagent context
//   - Returns "not available" if SetSubagentUseCase was never called
//   - Returns "required" errors if agent_name or prompt are empty
//   - Propagates errors from the subagent use case with "execution failed" wrapper
//   - Returns "nil result" if the use case returns success but nil result
//
// The result JSON includes: subagent_id, agent_name, status, output, actions_taken,
// duration_ms, and error (if the subagent encountered an error).
func (a *ExecutorAdapter) executeTask(ctx context.Context, input json.RawMessage) (string, error) {
	uc, err := a.checkSubagentPrerequisites(ctx, "task")
	if err != nil {
		return "", err
	}

	// Parse input
	var params taskInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse task input: %w", err)
	}

	// Validate inputs
	if params.AgentName == "" {
		return "", errors.New("agent_name is required")
	}
	if params.Prompt == "" {
		return "", errors.New("prompt is required")
	}

	// Spawn subagent
	result, err := uc.SpawnSubagent(ctx, params.AgentName, params.Prompt)
	if err != nil {
		return "", fmt.Errorf("subagent execution failed: %w", err)
	}

	return formatSubagentResult(result)
}

// executeDelegate executes the delegate tool to spawn a dynamic subagent.
func (a *ExecutorAdapter) executeDelegate(ctx context.Context, input json.RawMessage) (string, error) {
	uc, err := a.checkSubagentPrerequisites(ctx, "delegate")
	if err != nil {
		return "", err
	}

	// Parse input
	var params delegateInput
	if err := json.Unmarshal(input, &params); err != nil {
		return "", fmt.Errorf("failed to parse delegate input: %w", err)
	}

	// Validate required inputs
	if params.Name == "" {
		return "", errors.New("name is required")
	}
	if params.SystemPrompt == "" {
		return "", errors.New("system_prompt is required")
	}
	if params.Task == "" {
		return "", errors.New("task is required")
	}

	// Build DynamicSubagentConfig
	config := usecase.DynamicSubagentConfig{
		Name:         params.Name,
		SystemPrompt: params.SystemPrompt,
		Model:        params.Model,        // Empty string means default (inherit)
		MaxActions:   params.MaxActions,   // 0 means default (30)
		AllowedTools: params.AllowedTools, // nil means all tools
	}

	// Spawn dynamic subagent
	result, err := uc.SpawnDynamicSubagent(ctx, config, params.Task)
	if err != nil {
		return "", fmt.Errorf("dynamic subagent execution failed: %w", err)
	}

	return formatSubagentResult(result)
}

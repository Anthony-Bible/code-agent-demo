package tool

import (
	"bytes"
	"code-editing-agent/internal/application/usecase"
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	fileadapter "code-editing-agent/internal/infrastructure/adapter/file"

	"golang.org/x/net/html"
)

// SubagentUseCaseInterface defines the interface for spawning subagents.
//
// This interface enables the task tool to delegate work to specialized subagents,
// allowing the main agent to offload complex tasks to focused agents with specific
// capabilities. The interface abstracts the subagent spawning mechanism, allowing
// the tool executor to remain decoupled from the concrete use case implementation.
//
// Example usage:
//
//	result, err := useCase.SpawnSubagent(ctx, "code-reviewer", "Review PR #123")
//	if err != nil {
//	    // Handle error
//	}
//	fmt.Println(result.Output) // Subagent's analysis
type SubagentUseCaseInterface interface {
	SpawnSubagent(ctx context.Context, agentName string, prompt string) (*usecase.SubagentResult, error)
	SpawnDynamicSubagent(
		ctx context.Context,
		config usecase.DynamicSubagentConfig,
		taskPrompt string,
	) (*usecase.SubagentResult, error)
}

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
	skillManager                port.SkillManager
	subagentManager             port.SubagentManager
	subagentUseCase             SubagentUseCaseInterface
	tools                       map[string]entity.Tool
	mu                          sync.RWMutex
	dangerousCommandCallback    DangerousCommandCallback
	commandConfirmationCallback CommandConfirmationCallback
	investigationStates         map[string]string // tracks investigation_id -> status
	investigationMu             sync.Mutex
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
// SkillManager can be provided via SetSkillManager for skill-related functionality.
// SubagentManager can be provided via SetSubagentManager for subagent-related functionality.
// It also registers the default tools (read_file, list_files, edit_file, bash, fetch, activate_skill).
func NewExecutorAdapter(fileManager port.FileManager) *ExecutorAdapter {
	adapter := &ExecutorAdapter{
		fileManager:         fileManager,
		skillManager:        nil,
		subagentManager:     nil,
		tools:               make(map[string]entity.Tool),
		investigationStates: make(map[string]string),
	}

	// Register default tools
	adapter.registerDefaultTools()

	return adapter
}

// SetSkillManager sets the skill manager for skill-related functionality.
// This should be called after creation to enable skill activation features.
// It also rebuilds the activate_skill tool to include available skills in its description.
//
// This method is thread-safe but blocks tool operations momentarily while updating.
// Call once during initialization before starting the main execution loop.
func (a *ExecutorAdapter) SetSkillManager(sm port.SkillManager) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.skillManager = sm
	// Rebuild activate_skill tool with skill manager for dynamic description
	a.rebuildActivateSkillToolLocked()
}

// SetSubagentManager sets the subagent manager for agent discovery functionality.
// This should be called after creation to enable dynamic agent listing in tool descriptions.
// The subagent manager is used to discover available agents and include them in the task tool description.
//
// Blocking Behavior:
// This method is thread-safe but blocks ALL tool operations while executing. It acquires a write lock
// on the internal mutex, preventing concurrent access to ExecuteTool, ListTools, GetTool,
// ValidateToolInput, and SetSubagentUseCase. The method holds the lock while calling registerTaskTool(),
// which may perform I/O operations (DiscoverAgents) that could be slow.
//
// WARNING: Set the subagent manager once during initialization. Avoid calling this method frequently
// in hot paths or during active tool execution, as it will block all tool operations until complete.
// For optimal performance, configure the subagent manager before starting the main execution loop.
func (a *ExecutorAdapter) SetSubagentManager(sm port.SubagentManager) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.subagentManager = sm
	// Re-register the task tool with updated agent list
	a.registerTaskTool()
}

// SetSubagentUseCase sets the subagent use case for task delegation.
//
// This method must be called during initialization (typically in the DI container)
// to enable the task tool. Without a subagent use case, the task tool will return
// an error when invoked. The method is thread-safe and can be called multiple times
// to update the use case implementation.
//
// This design allows the tool executor to remain independent of the application layer
// while still supporting task delegation functionality.
func (a *ExecutorAdapter) SetSubagentUseCase(uc SubagentUseCaseInterface) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.subagentUseCase = uc
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
				"dangerous": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this command is potentially dangerous",
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

	// Register activate_skill tool (will be rebuilt with dynamic description if SetSkillManager is called)
	activateSkillTool := entity.Tool{
		ID:          "activate_skill",
		Name:        "activate_skill",
		Description: "Activates a skill by name and returns its full content. Use this to load detailed instructions for specific capabilities.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"skill_name": map[string]interface{}{
					"type":        "string",
					"description": "The name of the skill to activate",
				},
			},
			"required": []string{"skill_name"},
		},
		RequiredFields: []string{"skill_name"},
	}
	a.tools[activateSkillTool.Name] = activateSkillTool

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

	// Register task tool (dynamically includes available agents if subagentManager is set)
	a.registerTaskTool()

	// Register delegate tool
	delegateTool := entity.Tool{
		ID:   "delegate",
		Name: "delegate",
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

	// Register investigation tools
	a.registerInvestigationTools()
}

// rebuildActivateSkillToolLocked updates the activate_skill tool definition.
// REQUIRES: a.mu must be held by the caller.
func (a *ExecutorAdapter) rebuildActivateSkillToolLocked() {
	// Build description with available skills
	description := a.buildActivateSkillDescription()

	// Update the activate_skill tool with new description
	activateSkillTool := entity.Tool{
		ID:          "activate_skill",
		Name:        "activate_skill",
		Description: description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"skill_name": map[string]interface{}{
					"type":        "string",
					"description": "The name of the skill to activate",
				},
			},
			"required": []string{"skill_name"},
		},
		RequiredFields: []string{"skill_name"},
	}
	a.tools[activateSkillTool.Name] = activateSkillTool
}

// buildActivateSkillDescription builds the description for the activate_skill tool.
// If a skill manager is available, it includes available skills in the description.
func (a *ExecutorAdapter) buildActivateSkillDescription() string {
	baseDescription := "Execute a skill within the main conversation\n\n" +
		"When users ask you to perform tasks, check if any of the available skills below can help complete the task more effectively. " +
		"Skills provide specialized capabilities and domain knowledge.\n\n" +
		"Use this tool to load the full content of a skill when its capabilities are needed for the task at hand."

	// If no skill manager, return base description
	if a.skillManager == nil {
		return baseDescription
	}

	// Try to discover skills with timeout to prevent blocking indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	skills, err := a.skillManager.DiscoverSkills(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: failed to discover skills for tool description: %v\n", err)
		return baseDescription
	}
	if len(skills.Skills) == 0 {
		return baseDescription
	}

	// Build skills section following the example format
	var sb strings.Builder
	sb.WriteString(baseDescription)
	sb.WriteString("\n\n## Available Skills\n\n")

	for _, skill := range skills.Skills {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", skill.Name, skill.Description))
	}

	sb.WriteString("\nActivate a skill by providing its name to load detailed instructions and capabilities.")

	return sb.String()
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
	case "activate_skill":
		return a.executeActivateSkill(ctx, input)
	case "batch_tool":
		return a.executeBatchTool(ctx, input)
	case "task":
		return a.executeTask(ctx, input)
	case "delegate":
		return a.executeDelegate(ctx, input)
	case "complete_investigation":
		return a.executeCompleteInvestigation(ctx, input)
	case "escalate_investigation":
		return a.executeEscalateInvestigation(ctx, input)
	case "report_investigation":
		return a.executeReportInvestigation(ctx, input)
	default:
		return "", fmt.Errorf("tool not found: %s", name)
	}
}

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

// formatLinesWithNumbers formats file content as numbered lines within the specified range.
// startLine and endLine are 1-based line numbers. If nil, they default to the beginning and end of the file.
func formatLinesWithNumbers(content string, startLine, endLine *int) string {
	lines := strings.Split(content, "\n")
	// Remove trailing empty line if content ends with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Determine start and end indices (1-based to 0-based), clamped to valid range
	startIdx := 0
	if startLine != nil {
		startIdx = min(*startLine-1, len(lines))
	}

	endIdx := len(lines)
	if endLine != nil {
		endIdx = min(*endLine, len(lines))
	}

	// Build output with line numbers
	var result strings.Builder
	for i := startIdx; i < endIdx; i++ {
		result.WriteString(fmt.Sprintf("%d: %s\n", i+1, lines[i]))
	}

	return result.String()
}

// executeReadFile executes the read_file tool.
func (a *ExecutorAdapter) executeReadFile(input json.RawMessage) (string, error) {
	var in readFileInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to unmarshal read_file input: %w", err)
	}

	if err := in.validateLineRange(); err != nil {
		return "", err
	}

	content, err := a.fileManager.ReadFile(in.Path)
	if err != nil {
		return "", wrapFileOperationError("Failed to read file", err)
	}

	return formatLinesWithNumbers(content, in.StartLine, in.EndLine), nil
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
	Dangerous   bool   `json:"dangerous,omitempty"`
}

// fetchInput represents the input for the fetch tool.
type fetchInput struct {
	URL           string `json:"url"`
	IncludeMarkup bool   `json:"includeMarkup,omitempty"`
}

// activateSkillInput represents the input for the activate_skill tool.
type activateSkillInput struct {
	SkillName string `json:"skill_name"`
}

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

// bashOutput represents the output from the bash tool.
type bashOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// defaultBashTimeout is the default timeout for bash command execution.
const defaultBashTimeout = 30 * time.Second

// maxBatchInvocations is the maximum number of tool invocations allowed in a single batch.
const maxBatchInvocations = 20

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
// Special case: writing to /dev/null is allowed.
func isDangerousCommand(cmd string) (bool, string) {
	for _, dp := range dangerousPatterns {
		if dp.pattern.MatchString(cmd) {
			// Allow writes to /dev/null (common pattern for suppressing output)
			if dp.reason == "write to device" && strings.Contains(cmd, "/dev/null") {
				continue
			}
			return true, dp.reason
		}
	}
	return false, ""
}

// checkCommandConfirmation checks if a command should be allowed to execute.
func (a *ExecutorAdapter) checkCommandConfirmation(command string, description string, llmDangerous bool) error {
	isDangerous, reason := isDangerousCommand(command)

	// Combine: dangerous if either patterns match OR LLM says so
	if llmDangerous && !isDangerous {
		isDangerous = true
		reason = "marked dangerous by AI"
	}

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
	if err := a.checkCommandConfirmation(in.Command, in.Description, in.Dangerous); err != nil {
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

// isPrivateIP checks if an IP address is in a private/internal range.
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}

	if ip4 := ip.To4(); ip4 != nil {
		// Private IPv4 ranges
		privateIPv4Ranges := []struct {
			network *net.IPNet
		}{
			{network: mustParseCIDR("127.0.0.0/8")},    // Loopback
			{network: mustParseCIDR("10.0.0.0/8")},     // Private Class A
			{network: mustParseCIDR("172.16.0.0/12")},  // Private Class B
			{network: mustParseCIDR("192.168.0.0/16")}, // Private Class C
			{network: mustParseCIDR("169.254.0.0/16")}, // Link-local
			{network: mustParseCIDR("224.0.0.0/4")},    // Multicast
			{network: mustParseCIDR("0.0.0.0/8")},      // This network
		}

		for _, r := range privateIPv4Ranges {
			if r.network.Contains(ip4) {
				return true
			}
		}
	} else {
		// IPv6 private ranges
		privateIPv6Ranges := []struct {
			network *net.IPNet
		}{
			{network: mustParseCIDR("::1/128")},       // Loopback
			{network: mustParseCIDR("fc00::/7")},      // Unique local
			{network: mustParseCIDR("fe80::/10")},     // Link-local
			{network: mustParseCIDR("ff00::/8")},      // Multicast
			{network: mustParseCIDR("2000::/3")},      // Reserved for documentation
			{network: mustParseCIDR("2001:db8::/32")}, // NET-TEST example
		}

		for _, r := range privateIPv6Ranges {
			if r.network.Contains(ip) {
				return true
			}
		}
	}

	return false
}

// mustParseCIDR parses a CIDR string and panics on error.
func mustParseCIDR(cidr string) *net.IPNet {
	_, network, err := net.ParseCIDR(cidr)
	if err != nil {
		panic(fmt.Sprintf("failed to parse CIDR %s: %v", cidr, err))
	}
	return network
}

// validateURL validates that the URL is safe to fetch and blocks requests to private/internal resources.
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
		return errors.New("URL must have a host")
	}

	// Block credentials in URLs to prevent information disclosure
	if parsedURL.User != nil {
		return errors.New("URL contains credentials which are not allowed for security")
	}

	// Resolve hostname to IP addresses to check for private ranges
	host := parsedURL.Hostname()
	if host == "" {
		return errors.New("invalid hostname in URL")
	}

	// Check if host is an IP address
	hostIP := net.ParseIP(host)
	if hostIP != nil {
		// Direct IP address - check if it's private
		if isPrivateIP(hostIP) {
			return fmt.Errorf(
				"direct IP address %s is in a private/internal range and is blocked for security",
				hostIP.String(),
			)
		}
		return nil
	}

	// Hostname - resolve to IPs and check each one
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("failed to resolve hostname %s: %w", host, err)
	}

	// If no IPs resolve, block the request
	if len(ips) == 0 {
		return fmt.Errorf("hostname %s does not resolve to any IP address", host)
	}

	// Check all resolved IP addresses
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf(
				"hostname %s resolves to private IP address %s and is blocked for security",
				host,
				ip.String(),
			)
		}
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
		case html.ErrorNode, html.CommentNode, html.DoctypeNode, html.RawNode:
			// Skip these node types - they don't contain text content
			return
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, in.URL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set user agent
	req.Header.Set("User-Agent", "code-editing-agent/1.0")

	// Make HTTP request using a dedicated client with timeout and redirect policy
	client := &http.Client{
		Timeout: defaultFetchTimeout,
		// Configure redirect policy to prevent SSRF attacks and excessive redirects
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Limit to maximum 3 redirects to prevent excessive request chains
			if len(via) >= 3 {
				return errors.New("stopped after 3 redirects")
			}

			// Validate redirect URL to prevent SSRF attacks
			if err := validateURL(req.URL.String()); err != nil {
				return fmt.Errorf("redirect blocked due to security policy: %w", err)
			}

			return nil
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode >= 400 {
		respText := resp.Status
		if resp.StatusCode == http.StatusForbidden {
			respText = "authorization required"
		}
		return "", fmt.Errorf("HTTP %d (%s)", resp.StatusCode, respText)
	}

	// Check content length if available (may be -1 for chunked encoding)
	if resp.ContentLength > maxResponseSize {
		return "", fmt.Errorf("response too large: %d bytes (max: %d)", resp.ContentLength, maxResponseSize)
	}

	// Read response body with size tracking
	var bodyBuffer bytes.Buffer
	const maxChunkSize = 4096 // 4KB chunks for efficient memory usage

	// Track total bytes read to enforce size limit
	totalBytesRead := int64(0)
	chunk := make([]byte, maxChunkSize)

	for {
		// Calculate remaining bytes we can read
		remainingBytes := maxResponseSize - totalBytesRead
		if remainingBytes <= 0 {
			break // Stop reading if we've hit the limit
		}

		// Read next chunk, but limit it to remaining bytes
		chunkSize := uint64(maxChunkSize)
		if uint64(remainingBytes) < chunkSize {
			chunkSize = uint64(remainingBytes)
		}

		n, err := resp.Body.Read(chunk[:chunkSize])
		if n > 0 {
			bodyBuffer.Write(chunk[:n])
			totalBytesRead += int64(n)
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			return "", fmt.Errorf("failed to read response body: %w", err)
		}

		// If Content-Length was available and we've read all expected bytes, stop
		if resp.ContentLength >= 0 && totalBytesRead >= resp.ContentLength {
			break
		}
	}

	// Check if we hit the overall size limit while reading
	if totalBytesRead >= maxResponseSize {
		return "", fmt.Errorf(
			"response truncated due to size limit (max: %d bytes, read: %d bytes)",
			maxResponseSize,
			totalBytesRead,
		)
	}

	bodyBytes := bodyBuffer.Bytes()

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

// executeActivateSkill activates a skill by name and returns its full content.
// This allows the AI to load detailed instructions for specific capabilities.
// If no skill manager is set, returns an error.
func (a *ExecutorAdapter) executeActivateSkill(ctx context.Context, input json.RawMessage) (string, error) {
	// Check if skill manager is available
	if a.skillManager == nil {
		return "", errors.New("skill manager not available")
	}

	var in activateSkillInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to unmarshal activate_skill input: %w", err)
	}

	if in.SkillName == "" {
		return "", errors.New("skill_name parameter is required but was empty")
	}

	// Try to load the skill metadata first (avoids redundant filesystem scans).
	// If the skill is not found, refresh the discovered skills once and retry.
	skill, err := a.skillManager.LoadSkillMetadata(ctx, in.SkillName)
	if err != nil {
		// Attempt to refresh the skills list once
		if _, discoverErr := a.skillManager.DiscoverSkills(ctx); discoverErr != nil {
			return "", fmt.Errorf("failed to discover skills: %w", discoverErr)
		}

		// Retry loading the skill metadata after refreshing
		skill, err = a.skillManager.LoadSkillMetadata(ctx, in.SkillName)
		if err != nil {
			return "", fmt.Errorf("failed to load skill '%s': %w", in.SkillName, err)
		}
	}

	// Verify we have the full content (safety check for progressive disclosure)
	if skill.RawContent == "" {
		return "", fmt.Errorf("skill '%s' content not loaded", in.SkillName)
	}

	// Build result with frontmatter and content
	var result strings.Builder
	result.WriteString(fmt.Sprintf("---\nname: %s\ndescription: %s", skill.Name, skill.Description))
	if skill.License != "" {
		result.WriteString(fmt.Sprintf("\nlicense: %s", skill.License))
	}
	if skill.Compatibility != "" {
		result.WriteString(fmt.Sprintf("\ncompatibility: %s", skill.Compatibility))
	}
	if len(skill.AllowedTools) > 0 {
		result.WriteString(fmt.Sprintf("\nallowed-tools: %s", strings.Join(skill.AllowedTools, " ")))
	}
	if len(skill.Metadata) > 0 {
		result.WriteString("\nmetadata:")
		for key, value := range skill.Metadata {
			result.WriteString(fmt.Sprintf("\n  %s: %s", key, value))
		}
	}
	result.WriteString("\n---\n")
	result.WriteString(skill.RawContent)

	return result.String(), nil
}

// registerInvestigationTools registers the investigation-related tools.
func (a *ExecutorAdapter) registerInvestigationTools() {
	// Register complete_investigation tool
	completeInvestigationTool := entity.Tool{
		ID:          "complete_investigation",
		Name:        "complete_investigation",
		Description: "Completes an investigation with findings and confidence level.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"investigation_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the investigation to complete",
				},
				"confidence": map[string]interface{}{
					"type":        "number",
					"minimum":     float64(0),
					"maximum":     float64(1),
					"description": "Confidence level from 0 to 1",
				},
				"findings": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"description": "List of findings from the investigation",
				},
				"root_cause": map[string]interface{}{
					"type":        "string",
					"description": "The identified root cause (optional)",
				},
				"recommended_actions": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"description": "List of recommended actions (optional)",
				},
				"severity": map[string]interface{}{
					"type":        "string",
					"enum":        []interface{}{"info", "warning", "error", "critical"},
					"description": "Severity level of the findings",
				},
				"summary": map[string]interface{}{
					"type":        "string",
					"description": "Brief summary of the investigation",
				},
			},
			"required": []string{"confidence", "findings"},
		},
		RequiredFields: []string{"investigation_id", "confidence", "findings"},
	}
	a.tools[completeInvestigationTool.Name] = completeInvestigationTool

	// Register escalate_investigation tool
	escalateInvestigationTool := entity.Tool{
		ID:          "escalate_investigation",
		Name:        "escalate_investigation",
		Description: "Escalates an investigation to a higher priority or human review.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"investigation_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the investigation to escalate",
				},
				"reason": map[string]interface{}{
					"type":        "string",
					"description": "Reason for escalation",
				},
				"priority": map[string]interface{}{
					"type":        "string",
					"enum":        []interface{}{"low", "medium", "high", "critical"},
					"description": "Priority level for escalation",
				},
				"partial_findings": map[string]interface{}{
					"type": "array",
					"items": map[string]interface{}{
						"type": "string",
					},
					"description": "Partial findings gathered so far (optional)",
				},
				"blocking": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether this escalation is blocking",
				},
				"requires_acknowledgment": map[string]interface{}{
					"type":        "boolean",
					"description": "Whether acknowledgment is required",
				},
			},
			"required": []string{"investigation_id", "reason", "priority"},
		},
		RequiredFields: []string{"investigation_id", "reason", "priority"},
	}
	a.tools[escalateInvestigationTool.Name] = escalateInvestigationTool

	// Register report_investigation tool
	reportInvestigationTool := entity.Tool{
		ID:          "report_investigation",
		Name:        "report_investigation",
		Description: "Reports progress or status update during an ongoing investigation.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"investigation_id": map[string]interface{}{
					"type":        "string",
					"description": "The ID of the investigation to report on",
				},
				"message": map[string]interface{}{
					"type":        "string",
					"description": "Status message or progress update",
				},
				"progress": map[string]interface{}{
					"type":        "number",
					"minimum":     float64(0),
					"maximum":     float64(100),
					"description": "Progress percentage from 0 to 100",
				},
			},
			"required": []string{"investigation_id", "message"},
		},
		RequiredFields: []string{"investigation_id", "message"},
	}
	a.tools[reportInvestigationTool.Name] = reportInvestigationTool
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
				fullDescription.WriteString(fmt.Sprintf("- %s: %s\n", agent.Name, agent.Description))
			}
		}
	}

	taskTool := entity.Tool{
		ID:          "task",
		Name:        "task",
		Description: fullDescription.String(),
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

// Investigation status constants.
const (
	investigationStatusRunning   = "running"
	investigationStatusCompleted = "completed"
	investigationStatusEscalated = "escalated"
)

// RegisterInvestigation registers an investigation ID so it can be completed or escalated.
// This is primarily used for testing and by the investigation runner.
func (a *ExecutorAdapter) RegisterInvestigation(investigationID string) {
	if investigationID == "" || strings.TrimSpace(investigationID) == "" {
		return
	}
	a.investigationMu.Lock()
	defer a.investigationMu.Unlock()
	if _, exists := a.investigationStates[investigationID]; !exists {
		a.investigationStates[investigationID] = investigationStatusRunning
	}
}

// checkAndSetInvestigationStatus checks if an investigation can transition to newStatus.
// Returns nil if the transition is allowed, or an error if already in a terminal state.
// If investigationID is empty, the check is skipped.
func (a *ExecutorAdapter) checkAndSetInvestigationStatus(investigationID, newStatus string) error {
	if investigationID == "" || strings.TrimSpace(investigationID) == "" {
		return nil
	}

	a.investigationMu.Lock()
	defer a.investigationMu.Unlock()

	if status, exists := a.investigationStates[investigationID]; exists {
		if status == investigationStatusCompleted {
			return errors.New("investigation already completed")
		}
		if status == investigationStatusEscalated {
			return errors.New("investigation already escalated")
		}
	}
	a.investigationStates[investigationID] = newStatus
	return nil
}

// completeInvestigationInput represents the input for the complete_investigation tool.
type completeInvestigationInput struct {
	InvestigationID    string   `json:"investigation_id"`
	Confidence         *float64 `json:"confidence"`
	Findings           []string `json:"findings"`
	RootCause          string   `json:"root_cause,omitempty"`
	RecommendedActions []string `json:"recommended_actions,omitempty"`
}

// escalateInvestigationInput represents the input for the escalate_investigation tool.
type escalateInvestigationInput struct {
	InvestigationID string   `json:"investigation_id"`
	Reason          string   `json:"reason"`
	Priority        string   `json:"priority"`
	PartialFindings []string `json:"partial_findings,omitempty"`
}

// reportInvestigationInput represents the input for the report_investigation tool.
type reportInvestigationInput struct {
	InvestigationID string   `json:"investigation_id"`
	Message         string   `json:"message"`
	Progress        *float64 `json:"progress,omitempty"`
}

// executeCompleteInvestigation executes the complete_investigation tool.
func (a *ExecutorAdapter) executeCompleteInvestigation(ctx context.Context, input json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	var in completeInvestigationInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate investigation_id
	if in.InvestigationID == "" || strings.TrimSpace(in.InvestigationID) == "" {
		return "", errors.New("investigation_id is required and cannot be empty")
	}

	// Check if investigation exists
	a.investigationMu.Lock()
	_, exists := a.investigationStates[in.InvestigationID]
	a.investigationMu.Unlock()
	if !exists {
		return "", fmt.Errorf("investigation_id %q not found", in.InvestigationID)
	}

	// Validate confidence
	if in.Confidence == nil {
		return "", errors.New("confidence is required")
	}
	if *in.Confidence < 0 || *in.Confidence > 1 {
		return "", errors.New("confidence must be between 0 and 1")
	}

	// Validate findings
	if in.Findings == nil {
		return "", errors.New("findings is required")
	}
	if len(in.Findings) == 0 {
		return "", errors.New("findings cannot be empty")
	}

	// Check for duplicate completion (only if investigation_id provided)
	if err := a.checkAndSetInvestigationStatus(in.InvestigationID, investigationStatusCompleted); err != nil {
		return "", err
	}

	// Build output
	output := map[string]interface{}{
		"status":       investigationStatusCompleted,
		"confidence":   *in.Confidence,
		"findings":     in.Findings,
		"completed_at": time.Now().UTC().Format(time.RFC3339),
	}
	if in.InvestigationID != "" {
		output["investigation_id"] = in.InvestigationID
	}

	result, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}

	return string(result), nil
}

// executeEscalateInvestigation executes the escalate_investigation tool.
func (a *ExecutorAdapter) executeEscalateInvestigation(ctx context.Context, input json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	var in escalateInvestigationInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate investigation_id
	if in.InvestigationID == "" || strings.TrimSpace(in.InvestigationID) == "" {
		return "", errors.New("investigation_id is required and cannot be empty")
	}

	// Check if investigation exists
	a.investigationMu.Lock()
	_, exists := a.investigationStates[in.InvestigationID]
	a.investigationMu.Unlock()
	if !exists {
		return "", fmt.Errorf("investigation_id %q not found", in.InvestigationID)
	}

	// Validate reason
	if in.Reason == "" || strings.TrimSpace(in.Reason) == "" {
		return "", errors.New("reason is required and cannot be empty")
	}

	// Validate priority
	validPriorities := map[string]bool{"low": true, "medium": true, "high": true, "critical": true}
	if !validPriorities[in.Priority] {
		return "", errors.New("priority must be one of: low, medium, high, critical")
	}

	// Check for duplicate escalation (only if investigation_id provided)
	if err := a.checkAndSetInvestigationStatus(in.InvestigationID, investigationStatusEscalated); err != nil {
		return "", err
	}

	// Build output
	escalationID := fmt.Sprintf("esc-%d", time.Now().UnixNano())
	if in.InvestigationID != "" {
		escalationID = fmt.Sprintf("esc-%s-%d", in.InvestigationID, time.Now().UnixNano())
	}
	output := map[string]interface{}{
		"status":        investigationStatusEscalated,
		"escalation_id": escalationID,
		"reason":        in.Reason,
		"priority":      in.Priority,
		"escalated_at":  time.Now().UTC().Format(time.RFC3339),
	}
	if in.InvestigationID != "" {
		output["investigation_id"] = in.InvestigationID
	}

	result, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}

	return string(result), nil
}

// executeReportInvestigation executes the report_investigation tool.
func (a *ExecutorAdapter) executeReportInvestigation(ctx context.Context, input json.RawMessage) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	var in reportInvestigationInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to parse input: %w", err)
	}

	// Validate investigation_id
	if in.InvestigationID == "" || strings.TrimSpace(in.InvestigationID) == "" {
		return "", errors.New("investigation_id is required and cannot be empty")
	}

	// Validate message
	if in.Message == "" {
		return "", errors.New("message is required")
	}

	// Validate progress if provided
	if in.Progress != nil {
		if *in.Progress < 0 || *in.Progress > 100 {
			return "", errors.New("progress must be between 0 and 100")
		}
	}

	// Build output
	output := map[string]interface{}{
		"status":           "reported",
		"investigation_id": in.InvestigationID,
		"message":          in.Message,
		"reported_at":      time.Now().UTC().Format(time.RFC3339),
	}
	if in.Progress != nil {
		output["progress"] = *in.Progress
	}

	result, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}

	return string(result), nil
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
	// Check for recursion (subagents cannot spawn subagents)
	if port.IsSubagentContext(ctx) {
		return "", errors.New("task tool cannot be called from within a subagent (prevents infinite recursion)")
	}

	// Check if use case is set
	a.mu.RLock()
	useCase := a.subagentUseCase
	a.mu.RUnlock()

	if useCase == nil {
		return "", errors.New("subagent use case not available")
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
	result, err := useCase.SpawnSubagent(ctx, params.AgentName, params.Prompt)
	if err != nil {
		return "", fmt.Errorf("subagent execution failed: %w", err)
	}

	if result == nil {
		return "", errors.New("subagent execution returned nil result")
	}

	// Format result as JSON
	resultJSON := map[string]interface{}{
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

// executeDelegate executes the delegate tool to spawn a dynamic subagent.
func (a *ExecutorAdapter) executeDelegate(ctx context.Context, input json.RawMessage) (string, error) {
	// Check for recursion (subagents cannot spawn subagents)
	if port.IsSubagentContext(ctx) {
		return "", errors.New("delegate tool cannot be called from within a subagent (prevents infinite recursion)")
	}

	// Check if use case is set
	a.mu.RLock()
	useCase := a.subagentUseCase
	a.mu.RUnlock()

	if useCase == nil {
		return "", errors.New("subagent use case not available")
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
	result, err := useCase.SpawnDynamicSubagent(ctx, config, params.Task)
	if err != nil {
		return "", fmt.Errorf("dynamic subagent execution failed: %w", err)
	}

	if result == nil {
		return "", errors.New("dynamic subagent execution returned nil result")
	}

	// Format result as JSON
	resultJSON := map[string]interface{}{
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

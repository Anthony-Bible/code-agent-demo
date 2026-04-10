package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/anthony-bible/code-agent-demo/internal/application/usecase"
	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
	"github.com/anthony-bible/code-agent-demo/internal/domain/safety"

	fileadapter "github.com/anthony-bible/code-agent-demo/internal/infrastructure/adapter/file"
)

// Tool name constants used across multiple files in this package.
const (
	toolNameReadFile  = "read_file"
	toolNameBash      = "bash"
	toolNameBatchTool = "batch_tool"
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

// CommandConfirmationCallback is called before executing any bash command.
// It receives the command, whether it's dangerous, the reason if dangerous, and a description.
// Returns true if execution should proceed, false to block.
type CommandConfirmationCallback func(command string, isDangerous bool, reason string, description string) bool

// SkillActivationCallback is called when a skill is activated.
// It receives the session ID and the activated skill, allowing the handler to track active skills.
type SkillActivationCallback func(sessionID string, skill entity.Skill) error

// SkillDeactivationCallback is called when a skill is deactivated.
// It receives the session ID and the skill name, allowing the handler to remove it from active skills.
type SkillDeactivationCallback func(sessionID string, skillName string) error

// ExecutorAdapter implements the ToolExecutor port using the FileManager for file operations.
type ExecutorAdapter struct {
	fileManager                 port.FileManager
	skillManager                port.SkillManager
	subagentManager             port.SubagentManager
	subagentUseCase             SubagentUseCaseInterface
	tools                       map[string]entity.Tool
	mu                          sync.RWMutex
	commandConfirmationCallback CommandConfirmationCallback
	skillActivationCallback     SkillActivationCallback
	skillDeactivationCallback   SkillDeactivationCallback
	investigationStates         map[string]string // tracks investigation_id -> status
	investigationMu             sync.RWMutex

	fetchClient     *http.Client
	fetchClientOnce sync.Once

	// Command validation - uses sync.Once to ensure immutability after first use.
	// This prevents race conditions where SetValidationMode could swap the validator
	// while checkCommandConfirmation is using it.
	commandValidator     safety.CommandValidator
	validatorOnce        sync.Once
	pendingValidatorMode safety.CommandValidationMode
	pendingWhitelist     *safety.CommandWhitelist
	pendingAskLLM        bool
	validatorConfigured  bool // true if SetValidationMode was called before first use

	logger port.Logger
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

// NewExecutorAdapter creates a new ExecutorAdapter with the provided FileManager.
// SkillManager can be provided via SetSkillManager for skill-related functionality.
// SubagentManager can be provided via SetSubagentManager for subagent-related functionality.
// It also registers the default tools (read_file, list_files, edit_file, bash, fetch, activate_skill).
func NewExecutorAdapter(fileManager port.FileManager, log port.Logger) *ExecutorAdapter {
	adapter := &ExecutorAdapter{
		fileManager:         fileManager,
		skillManager:        nil,
		subagentManager:     nil,
		tools:               make(map[string]entity.Tool),
		investigationStates: make(map[string]string),
		logger:              port.SafeLogger(log),
	}

	// Register default tools
	adapter.registerDefaultTools()

	return adapter
}

// wrapFileOperationError wraps file operation errors with context and logs a security
// warning for path traversal attempts. It is a method on ExecutorAdapter so that it
// can use the injected logger rather than the global slog logger.
func (a *ExecutorAdapter) wrapFileOperationError(operation string, err error) error {
	if err == nil {
		return nil
	}

	// Check for path traversal error in the error chain
	if errors.Is(err, fileadapter.ErrPathTraversal) {
		a.logger.Error("path traversal attempt detected and blocked", "operation", operation)
		return fmt.Errorf("%s blocked due to potential security threat: %w", operation, err)
	}

	// Check for PathValidationError which has detailed reason
	var pathErr *fileadapter.PathValidationError
	if errors.As(err, &pathErr) && pathErr.Reason == "path traversal attempt detected" {
		a.logger.Error("path traversal attempt detected and blocked", "operation", operation)
		return fmt.Errorf("%s blocked due to potential security threat: %w", operation, err)
	}

	return fmt.Errorf("%s: %w", operation, err)
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

// SetCommandConfirmationCallback sets the callback for all command confirmation.
func (a *ExecutorAdapter) SetCommandConfirmationCallback(cb CommandConfirmationCallback) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.commandConfirmationCallback = cb
}

// SetSkillActivationCallback sets the callback for skill activation.
// This callback is called when a skill is activated, allowing the handler to track active skills.
func (a *ExecutorAdapter) SetSkillActivationCallback(cb SkillActivationCallback) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.skillActivationCallback = cb
}

// SetSkillDeactivationCallback sets the callback for skill deactivation.
// This callback is called when a skill is deactivated, allowing the handler to remove it from active skills.
func (a *ExecutorAdapter) SetSkillDeactivationCallback(cb SkillDeactivationCallback) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.skillDeactivationCallback = cb
}

// SetValidationMode configures command validation mode and whitelist.
// mode: "blacklist" (default) or "whitelist"
// whitelist: the CommandWhitelist to use when mode is "whitelist" (can be nil for blacklist mode)
// askLLMOnUnknown: whether to ask LLM before blocking non-whitelisted commands (only for whitelist mode).
//
// IMPORTANT: This must be called during initialization, before any command validation occurs.
// Once the validator is initialized (on first use), subsequent calls will log a warning and be ignored
// to prevent race conditions.
//
// Returns an error if whitelist mode is specified but no whitelist is provided.
func (a *ExecutorAdapter) SetValidationMode(
	mode safety.CommandValidationMode,
	whitelist *safety.CommandWhitelist,
	askLLMOnUnknown bool,
) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if validator has already been initialized (first use has occurred)
	if a.commandValidator != nil {
		a.logger.Warn(
			"SetValidationMode called after validator was already initialized; ignoring to prevent race condition",
			"mode", mode,
		)
		return nil
	}

	// Store the pending configuration - will be used on first validation
	a.pendingValidatorMode = mode
	a.pendingWhitelist = whitelist
	a.pendingAskLLM = askLLMOnUnknown
	a.validatorConfigured = true
	return nil
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
	a.registerFileTools()
	a.registerBashTool()
	a.registerFetchTool()
	a.registerSkillTools()
	a.registerPlanAndBatchTools()
	a.registerSubagentTools()
	a.registerInvestigationTools()
}

// executeByName executes the appropriate tool function based on the tool name.
func (a *ExecutorAdapter) executeByName(ctx context.Context, name string, input json.RawMessage) (string, error) {
	switch name {
	case toolNameReadFile:
		return a.executeReadFile(input)
	case "list_files":
		return a.executeListFiles(input)
	case "edit_file":
		return a.executeEditFile(input)
	case toolNameBash:
		return a.executeBash(ctx, input)
	case "fetch":
		return a.executeFetch(ctx, input)
	case "activate_skill":
		return a.executeActivateSkill(ctx, input)
	case "deactivate_skill":
		return a.executeDeactivateSkill(ctx, input)
	case toolNameBatchTool:
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

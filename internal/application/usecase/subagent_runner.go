// Package usecase contains application use cases.
package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
	"fmt"
	"os"
	"time"
)

// resolveModelShorthand converts shorthand model names to actual Anthropic model IDs.
// It supports:
//   - "haiku" -> "claude-3-5-haiku-20241022"
//   - "sonnet" -> "claude-sonnet-4-5-20250929"
//   - "opus" -> "claude-opus-4-5-20250514"
//   - "inherit" or "" -> "" (empty string signals to not change model)
//   - Any other value is returned as-is (assumed to be a full model ID)
func resolveModelShorthand(model string) string {
	switch model {
	case string(entity.ModelHaiku):
		return "claude-3-5-haiku-20241022"
	case string(entity.ModelSonnet):
		return "claude-sonnet-4-5-20250929"
	case string(entity.ModelOpus):
		return "claude-opus-4-5-20250514"
	case string(entity.ModelInherit), "":
		return "" // Empty means don't change model
	default:
		// Assume it's already a full model ID (e.g., "claude-sonnet-4-5")
		return model
	}
}

// SubagentConfig holds configuration for subagent execution.
type SubagentConfig struct {
	MaxActions      int
	MaxDuration     time.Duration
	MaxConcurrent   int
	AllowedTools    []string
	BlockedCommands []string
	ThinkingEnabled bool  // Enable extended thinking mode for subagent
	ThinkingBudget  int64 // Thinking token budget (0 = unlimited)
	ShowThinking    bool  // Display thinking output to user
}

// SubagentResult holds the result of a subagent execution.
type SubagentResult struct {
	SubagentID   string
	AgentName    string
	Status       string
	Output       string
	ActionsTaken int
	Duration     time.Duration
	Error        error
}

// GetSubagentID returns the subagent ID.
func (r *SubagentResult) GetSubagentID() string {
	return r.SubagentID
}

// GetAgentName returns the agent name.
func (r *SubagentResult) GetAgentName() string {
	return r.AgentName
}

// GetStatus returns the execution status.
func (r *SubagentResult) GetStatus() string {
	return r.Status
}

// GetOutput returns the output from the subagent.
func (r *SubagentResult) GetOutput() string {
	return r.Output
}

// GetActionsTaken returns the number of actions taken.
func (r *SubagentResult) GetActionsTaken() int {
	return r.ActionsTaken
}

// GetDuration returns the execution duration.
func (r *SubagentResult) GetDuration() time.Duration {
	return r.Duration
}

// GetError returns any error that occurred.
func (r *SubagentResult) GetError() error {
	return r.Error
}

// SubagentRunner orchestrates isolated subagent execution for task delegation.
type SubagentRunner struct {
	convService   ConversationServiceInterface
	toolExecutor  port.ToolExecutor
	aiProvider    port.AIProvider
	userInterface port.UserInterface
	config        SubagentConfig
}

// subagentRunContext holds state for a subagent execution run.
type subagentRunContext struct {
	ctx          context.Context
	agent        *entity.Subagent
	taskPrompt   string
	subagentID   string
	sessionID    string
	startTime    time.Time
	actionsTaken int
	maxActions   int
	lastMessage  *entity.Message
	runner       *SubagentRunner // Reference to runner for UI display
}

// NewSubagentRunner creates a new SubagentRunner with dependency validation.
func NewSubagentRunner(
	convService ConversationServiceInterface,
	toolExecutor port.ToolExecutor,
	aiProvider port.AIProvider,
	userInterface port.UserInterface,
	config SubagentConfig,
) *SubagentRunner {
	if convService == nil {
		panic("convService cannot be nil")
	}
	if toolExecutor == nil {
		panic("toolExecutor cannot be nil")
	}
	if aiProvider == nil {
		panic("aiProvider cannot be nil")
	}
	// userInterface is optional (can be nil for tests)

	return &SubagentRunner{
		convService:   convService,
		toolExecutor:  toolExecutor,
		aiProvider:    aiProvider,
		userInterface: userInterface,
		config:        config,
	}
}

// Run executes a subagent task with the given agent configuration.
//
// The subagent execution follows this flow:
//  1. Validate inputs (agent, taskPrompt, subagentID)
//  2. Start a new isolated conversation session
//  3. Set agent's custom system prompt
//  4. Send the task prompt as user message
//  5. Process AI responses in a loop:
//     - If AI requests tools: execute tools, feed results back
//     - If AI completes: extract output and return result
//     - If action limit exceeded: stop and return
//  6. Clean up conversation session
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - agent: The agent configuration to use
//   - taskPrompt: The task to execute
//   - subagentID: Unique identifier for this subagent execution
//
// Returns:
//   - *SubagentResult: Result of the subagent execution
//   - error: Any error that occurred during execution
func (r *SubagentRunner) Run(
	ctx context.Context,
	agent *entity.Subagent,
	taskPrompt string,
	subagentID string,
) (*SubagentResult, error) {
	if err := r.validateInputs(agent, taskPrompt); err != nil {
		return r.validationFailedResult(subagentID, agent, err), err
	}

	// Model switching: Resolve shorthand and set agent model if specified
	resolvedModel := resolveModelShorthand(agent.Model)
	if resolvedModel != "" {
		originalModel := r.aiProvider.GetModel()
		if err := r.aiProvider.SetModel(resolvedModel); err != nil {
			return r.validationFailedResult(subagentID, agent, err), err
		}
		defer func() { _ = r.aiProvider.SetModel(originalModel) }()
	}

	// Wrap context with subagent info for recursion prevention
	ctx = port.WithSubagentContext(ctx, port.SubagentContextInfo{
		SubagentID:      subagentID,
		ParentSessionID: "",
		IsSubagent:      true,
		Depth:           1,
	})

	rc := &subagentRunContext{
		ctx:        ctx,
		agent:      agent,
		taskPrompt: taskPrompt,
		subagentID: subagentID,
		startTime:  time.Now(),
		maxActions: r.config.MaxActions,
		runner:     r,
	}
	if rc.maxActions == 0 {
		rc.maxActions = 20
	}

	sessionID, err := r.convService.StartConversation(ctx)
	if err != nil {
		return rc.failedResult(err), err
	}
	rc.sessionID = sessionID
	defer func() { _ = r.convService.EndConversation(ctx, sessionID) }()

	// Propagate thinking mode from config if enabled
	if r.config.ThinkingEnabled {
		thinkingInfo := port.ThinkingModeInfo{
			Enabled:      true,
			BudgetTokens: r.config.ThinkingBudget,
			ShowThinking: r.config.ShowThinking,
		}
		_ = r.convService.SetThinkingMode(sessionID, thinkingInfo)
		// Ignore error - thinking mode is optional, continue execution
	}

	if err := r.setupAgentSession(rc); err != nil {
		return rc.failedResult(err), err
	}

	// Display subagent starting
	r.displayStatus(agent.Name, "Starting", "")

	return r.runExecutionLoop(rc)
}

// validateInputs validates the input parameters for subagent execution.
func (r *SubagentRunner) validateInputs(agent *entity.Subagent, taskPrompt string) error {
	if agent == nil {
		return errors.New("nil agent")
	}
	if taskPrompt == "" {
		return errors.New("empty task prompt")
	}
	return nil
}

// validationFailedResult creates a failed result for validation errors.
func (r *SubagentRunner) validationFailedResult(
	subagentID string,
	agent *entity.Subagent,
	err error,
) *SubagentResult {
	agentName := ""
	if agent != nil {
		agentName = agent.Name
	}
	return &SubagentResult{
		SubagentID: subagentID,
		AgentName:  agentName,
		Status:     "failed",
		Error:      err,
	}
}

// failedResult creates a failed result from the run context.
func (rc *subagentRunContext) failedResult(err error) *SubagentResult {
	// Display failure status
	rc.runner.displayStatus(rc.agent.Name, "Failed", err.Error())

	return &SubagentResult{
		SubagentID:   rc.subagentID,
		AgentName:    rc.agent.Name,
		Status:       "failed",
		ActionsTaken: rc.actionsTaken,
		Duration:     time.Since(rc.startTime),
		Error:        err,
	}
}

// completedResult creates a successful completion result from the run context.
func (rc *subagentRunContext) completedResult() *SubagentResult {
	output := ""
	if rc.lastMessage != nil {
		// Prefix output with subagent identifier for clarity
		output = "[SUBAGENT: " + rc.agent.Name + "]\n\n" + rc.lastMessage.Content
	}

	duration := time.Since(rc.startTime)

	// Display completion status with details
	details := fmt.Sprintf("%d actions, %.1fs", rc.actionsTaken, duration.Seconds())
	rc.runner.displayStatus(rc.agent.Name, "Completed", details)

	return &SubagentResult{
		SubagentID:   rc.subagentID,
		AgentName:    rc.agent.Name,
		Status:       "completed",
		Output:       output,
		ActionsTaken: rc.actionsTaken,
		Duration:     duration,
	}
}

// setupAgentSession configures the agent's system prompt and sends the initial task message.
func (r *SubagentRunner) setupAgentSession(rc *subagentRunContext) error {
	// Set custom system prompt from agent configuration
	systemPrompt := rc.agent.RawContent
	if err := r.convService.SetCustomSystemPrompt(rc.ctx, rc.sessionID, systemPrompt); err != nil {
		return err
	}

	// Add user message with task prompt
	if _, err := r.convService.AddUserMessage(rc.ctx, rc.sessionID, rc.taskPrompt); err != nil {
		return err
	}

	return nil
}

// runExecutionLoop runs the main tool execution loop until completion or limit.
func (r *SubagentRunner) runExecutionLoop(rc *subagentRunContext) (*SubagentResult, error) {
	for rc.actionsTaken < rc.maxActions {
		// Process assistant response
		msg, toolCalls, err := r.convService.ProcessAssistantResponse(rc.ctx, rc.sessionID)
		if err != nil {
			return rc.failedResult(err), err
		}

		rc.lastMessage = msg

		// No tool calls means completion
		if len(toolCalls) == 0 {
			break
		}

		// Execute tools and feed results back
		if err := r.processToolCalls(rc, toolCalls); err != nil {
			return rc.failedResult(err), err
		}

		// Inject turn warning if approaching limit
		r.injectTurnWarningIfNeeded(rc)

		// Stop at MaxActions
		if rc.actionsTaken >= rc.maxActions {
			break
		}
	}

	return rc.completedResult(), nil
}

// processToolCalls executes tool calls and feeds results back to the conversation.
func (r *SubagentRunner) processToolCalls(rc *subagentRunContext, toolCalls []port.ToolCallInfo) error {
	var toolResults []entity.ToolResult
	for _, tc := range toolCalls {
		if !r.isToolCallAllowed(tc) {
			// Blocked tools return error but DON'T count toward action limit
			toolResults = append(toolResults, entity.ToolResult{
				ToolID:  tc.ToolID,
				Result:  fmt.Sprintf("tool '%s' is not allowed for this subagent", tc.ToolName),
				IsError: true,
			})
			continue
		}

		// Execute allowed tool
		r.displayToolExecution(rc.agent.Name, tc.ToolName)
		result := r.executeToolCall(rc.ctx, tc)
		toolResults = append(toolResults, result)
		r.displayToolResult(rc.agent.Name, tc.ToolName, result.IsError)

		// NOTE: actionsTaken increments are safe because tool execution is currently sequential.
		// If tool execution becomes concurrent in the future, use atomic.AddInt32() instead.
		rc.actionsTaken++ // Only executed tools count
	}

	if len(toolResults) > 0 {
		return r.convService.AddToolResultMessage(rc.ctx, rc.sessionID, toolResults)
	}
	return nil
}

// isToolCallAllowed checks if a tool call is allowed based on config's AllowedTools.
func (r *SubagentRunner) isToolCallAllowed(tc port.ToolCallInfo) bool {
	if r.config.AllowedTools == nil {
		return true // nil = allow all
	}
	if len(r.config.AllowedTools) == 0 {
		return false // empty slice = block all
	}
	return r.isToolAllowed(r.config.AllowedTools, tc.ToolName)
}

// isToolAllowed checks if a tool is in the allowed list.
func (r *SubagentRunner) isToolAllowed(allowedTools []string, toolName string) bool {
	for _, allowed := range allowedTools {
		if allowed == toolName {
			return true
		}
	}
	return false
}

// executeToolCall executes a single tool call and returns the result.
func (r *SubagentRunner) executeToolCall(ctx context.Context, tc port.ToolCallInfo) entity.ToolResult {
	// Recursion prevention: block "task" tool in subagent context
	if tc.ToolName == "task" && port.IsSubagentContext(ctx) {
		return entity.ToolResult{
			ToolID:  tc.ToolID,
			Result:  "task tool is blocked in subagent context to prevent recursion",
			IsError: true,
		}
	}

	result, execErr := r.toolExecutor.ExecuteTool(ctx, tc.ToolName, tc.Input)
	if execErr != nil {
		return entity.ToolResult{
			ToolID:  tc.ToolID,
			Result:  execErr.Error(),
			IsError: true,
		}
	}
	return entity.ToolResult{
		ToolID:  tc.ToolID,
		Result:  result,
		IsError: false,
	}
}

// displayStatus displays a status message for the subagent if UI is available.
func (r *SubagentRunner) displayStatus(agentName string, status string, details string) {
	if r.userInterface != nil {
		_ = r.userInterface.DisplaySubagentStatus(agentName, status, details)
	}
}

// displayToolExecution displays a message before tool execution.
func (r *SubagentRunner) displayToolExecution(agentName string, toolName string) {
	if r.userInterface != nil {
		_ = r.userInterface.DisplaySubagentStatus(agentName, fmt.Sprintf("Executing %s", toolName), "")
	}
}

// displayToolResult displays a message after tool execution.
func (r *SubagentRunner) displayToolResult(agentName string, toolName string, isError bool) {
	if r.userInterface != nil {
		status := "Tool completed"
		if isError {
			status = "Tool failed"
		}
		_ = r.userInterface.DisplaySubagentStatus(agentName, status, toolName)
	}
}

// injectTurnWarningIfNeeded injects a warning message if the subagent is approaching the turn limit.
func (r *SubagentRunner) injectTurnWarningIfNeeded(rc *subagentRunContext) {
	remaining := rc.maxActions - rc.actionsTaken
	cfg := DefaultTurnWarningConfig()
	// Subagents don't use batch_tool, so no hint
	warningMsg := BuildTurnWarningMessage(remaining, cfg)
	if warningMsg != "" {
		if _, err := r.convService.AddUserMessage(rc.ctx, rc.sessionID, warningMsg); err != nil {
			// Log error but don't fail execution - warnings are non-critical
			fmt.Fprintf(os.Stderr, "[SubagentRunner] Failed to inject turn warning: %v\n", err)
		}
	}
}

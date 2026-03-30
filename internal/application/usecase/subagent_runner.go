// Package usecase contains application use cases.
package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

const (
	// Subagent status constants for display messages.
	statusStarting  = "Starting"
	statusCompleted = "Completed"
	statusFailed    = "Failed"
	statusThinking  = "Thinking"
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

// ConversationServiceFactory creates a ConversationService with the given AIProvider.
// Used by SubagentRunner to create isolated conversation services per concurrent execution.
type ConversationServiceFactory func(aiProvider port.AIProvider) (ConversationServiceInterface, error)

// SubagentRunner orchestrates isolated subagent execution for task delegation.
// Run is not safe for concurrent use on the same instance when subagents use different
// models; create one SubagentRunner per concurrent execution via the convFactory.
type SubagentRunner struct {
	BaseRunner

	aiProvider    port.AIProvider
	userInterface port.UserInterface
	config        SubagentConfig
	convFactory   ConversationServiceFactory
}

// subagentRunContext holds state for a subagent execution run.
type subagentRunContext struct {
	BaseRunContext

	agent         *entity.Subagent
	taskPrompt    string
	subagentID    string
	lastMessage   *entity.Message
	runner        *SubagentRunner // Reference to runner for UI display
	originalModel string          // Original model before any switching
}

// NewSubagentRunner creates a new SubagentRunner with dependency validation.
//
// An optional logger can be provided; if omitted a no-op logger is used.
func NewSubagentRunner(
	convService ConversationServiceInterface,
	toolExecutor port.ToolExecutor,
	aiProvider port.AIProvider,
	userInterface port.UserInterface,
	config SubagentConfig,
	convFactory ConversationServiceFactory,
	loggers ...port.Logger,
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
	if convFactory == nil {
		panic("convFactory cannot be nil")
	}
	// userInterface is optional (can be nil for tests)

	log := port.FirstOrNop(loggers...)
	return &SubagentRunner{
		BaseRunner:    newBaseRunner(convService, toolExecutor, &AllowedListPermissionChecker{AllowedTools: config.AllowedTools}, log),
		aiProvider:    aiProvider,
		userInterface: userInterface,
		config:        config,
		convFactory:   convFactory,
	}
}

// Run executes a subagent task with the given agent configuration.
// Each call clones the AIProvider and creates an isolated ConversationService,
// making Run safe for concurrent use across multiple goroutines.
//
// The subagent execution follows this flow:
//  1. Validate inputs (agent, taskPrompt, subagentID)
//  2. Clone AIProvider and create isolated ConversationService
//  3. Start a new isolated conversation session
//  4. Set agent's custom system prompt
//  5. Send the task prompt as user message
//  6. Process AI responses in a loop:
//     - If AI requests tools: execute tools, feed results back
//     - If AI completes: extract output and return result
//     - If action limit exceeded: stop and return
//  7. Clean up conversation session
func (r *SubagentRunner) Run(
	ctx context.Context,
	agent *entity.Subagent,
	taskPrompt string,
	subagentID string,
) (*SubagentResult, error) {
	if err := r.validateInputs(agent, taskPrompt); err != nil {
		return r.validationFailedResult(subagentID, agent, err), err
	}

	// Clone AIProvider for concurrency safety — each subagent gets independent model state.
	localProvider := r.aiProvider.Clone()
	originalModel := localProvider.GetModel()

	// Model switching: Resolve shorthand and set agent model on the clone
	resolvedModel := resolveModelShorthand(agent.Model)
	if resolvedModel != "" {
		if err := localProvider.SetModel(resolvedModel); err != nil {
			return r.validationFailedResult(subagentID, agent, err), err
		}
	}

	// Create isolated ConversationService wired to the cloned provider
	localConvService, err := r.convFactory(localProvider)
	if err != nil {
		return r.validationFailedResult(subagentID, agent, err), err
	}

	// Build a local runner so all BaseRunner methods use the isolated conv service
	localRunner := &SubagentRunner{
		BaseRunner:    newBaseRunner(localConvService, r.ToolExecutor, r.PermissionChecker, r.log()),
		aiProvider:    localProvider,
		userInterface: r.userInterface,
		config:        r.config,
		convFactory:   r.convFactory,
	}

	// Wrap context with subagent info for recursion prevention
	ctx = port.WithSubagentContext(ctx, port.SubagentContextInfo{
		SubagentID:      subagentID,
		ParentSessionID: "",
		IsSubagent:      true,
		Depth:           1,
	})

	rc := &subagentRunContext{
		BaseRunContext: BaseRunContext{
			Ctx:        ctx,
			StartTime:  time.Now(),
			MaxActions: r.config.MaxActions,
		},
		agent:         agent,
		taskPrompt:    taskPrompt,
		subagentID:    subagentID,
		runner:        localRunner,
		originalModel: originalModel,
	}
	if rc.MaxActions == 0 {
		rc.MaxActions = 20
	}

	sessionID, err := localConvService.StartConversation(ctx)
	if err != nil {
		return rc.failedResult(err), err
	}
	rc.SessionID = sessionID
	defer localRunner.CleanupConversation(sessionID, agent.Name, "agent")

	// Subagents use their own configuration or inherit from static config.
	thinkingInfo := port.ThinkingModeInfo{
		Enabled:      r.config.ThinkingEnabled,
		BudgetTokens: r.config.ThinkingBudget,
		ShowThinking: r.config.ShowThinking,
	}

	// Agent-specific override: if agent specifies thinking config in AGENT.md, use it
	if agent.ThinkingEnabled != nil {
		thinkingInfo.Enabled = *agent.ThinkingEnabled
	}
	if agent.ThinkingBudget > 0 {
		thinkingInfo.BudgetTokens = agent.ThinkingBudget
	}

	if thinkingInfo.Enabled {
		if err := localConvService.SetThinkingMode(sessionID, thinkingInfo); err != nil {
			// Log warning but don't fail - thinking mode is optional
			r.log().Warn("failed to set thinking mode",
				"error", err,
				"agent", rc.agent.Name,
				"session_id", sessionID,
				"enabled", thinkingInfo.Enabled,
				"budget", thinkingInfo.BudgetTokens,
			)
		}
	}

	if err := localRunner.setupAgentSession(rc); err != nil {
		return rc.failedResult(err), err
	}

	// Display subagent starting
	localRunner.displayStatus(agent.Name, statusStarting, "")

	return localRunner.runExecutionLoop(rc)
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
	rc.runner.displayStatus(rc.agent.Name, statusFailed, err.Error())

	return &SubagentResult{
		SubagentID:   rc.subagentID,
		AgentName:    rc.agent.Name,
		Status:       "failed",
		ActionsTaken: rc.ActionsTaken,
		Duration:     time.Since(rc.StartTime),
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

	duration := time.Since(rc.StartTime)

	// Display completion status with details
	details := fmt.Sprintf("%d actions, %.1fs", rc.ActionsTaken, duration.Seconds())
	rc.runner.displayStatus(rc.agent.Name, statusCompleted, details)

	return &SubagentResult{
		SubagentID:   rc.subagentID,
		AgentName:    rc.agent.Name,
		Status:       "completed",
		Output:       output,
		ActionsTaken: rc.ActionsTaken,
		Duration:     duration,
	}
}

// setupAgentSession configures the agent's system prompt and sends the initial task message.
func (r *SubagentRunner) setupAgentSession(rc *subagentRunContext) error {
	// Set custom system prompt from agent configuration
	systemPrompt := rc.agent.RawContent
	if err := r.ConvService.SetCustomSystemPrompt(rc.Ctx, rc.SessionID, systemPrompt); err != nil {
		return err
	}

	// Add user message with task prompt
	if _, err := r.ConvService.AddUserMessage(rc.Ctx, rc.SessionID, rc.taskPrompt); err != nil {
		return err
	}

	return nil
}

// runExecutionLoop runs the main tool execution loop until completion or limit.
func (r *SubagentRunner) runExecutionLoop(rc *subagentRunContext) (*SubagentResult, error) {
	for rc.ActionsTaken < rc.MaxActions {
		// Add thinking mode indicator if enabled for this session
		ctx := rc.Ctx
		thinkingInfo, _ := rc.runner.ConvService.GetThinkingMode(rc.SessionID)
		if thinkingInfo.Enabled {
			// Display thinking status indicator.
			// Note: Thinking content itself is never displayed for subagents,
			// only the status indicator to show the AI is processing.
			rc.runner.displayStatus(rc.agent.Name, statusThinking, "")
		}

		// Process assistant response
		msg, toolCalls, err := r.processAssistantResponseWithFallback(ctx, rc)
		if err != nil {
			return rc.failedResult(err), err
		}

		rc.lastMessage = msg

		// No tool calls means completion
		if len(toolCalls) == 0 {
			break
		}

		// Cap tool calls to remaining action budget so MaxActions is a strict upper bound
		toolCalls = r.LimitToolCalls(&rc.BaseRunContext, toolCalls)
		if len(toolCalls) == 0 {
			break
		}

		// Execute tools and feed results back
		executor := func(ctx context.Context, tc port.ToolCallInfo) entity.ToolResult {
			r.displayToolExecution(rc.agent.Name, tc.ToolName)
			result := r.executeToolCall(ctx, tc)
			r.displayToolResult(rc.agent.Name, tc.ToolName, result.IsError)
			return result
		}
		if err := r.ProcessToolCalls(&rc.BaseRunContext, toolCalls, "is not allowed for this subagent", executor); err != nil {
			return rc.failedResult(err), err
		}

		// Inject turn warning if approaching limit
		r.InjectTurnWarningIfNeeded(&rc.BaseRunContext, DefaultTurnWarningConfig())
	}

	return rc.completedResult(), nil
}

// processAssistantResponseWithFallback processes the assistant response with model fallback handling.
func (r *SubagentRunner) processAssistantResponseWithFallback(
	ctx context.Context,
	rc *subagentRunContext,
) (*entity.Message, []port.ToolCallInfo, error) {
	msg, toolCalls, err := r.ConvService.ProcessAssistantResponse(ctx, rc.SessionID)
	if err == nil {
		return msg, toolCalls, nil
	}

	// Check if this is a model-related 400 error and we tried to switch models
	if !r.isModelError(err) || r.aiProvider.GetModel() == "" || r.aiProvider.GetModel() == rc.originalModel {
		return nil, nil, err
	}

	// Log warning and fall back to parent model
	fallbackMsg := fmt.Sprintf(
		"Model '%s' not available for subagent, falling back to parent model '%s': %v",
		r.aiProvider.GetModel(),
		rc.originalModel,
		err,
	)
	if r.userInterface != nil {
		_ = r.userInterface.DisplaySubagentStatus(rc.agent.Name, "Model fallback", fallbackMsg)
	}

	// Restore original model and retry
	if modelErr := r.aiProvider.SetModel(rc.originalModel); modelErr != nil {
		return nil, nil, fmt.Errorf("failed to restore original model: %w (original error: %w)", modelErr, err)
	}

	// Retry with parent model
	return r.ConvService.ProcessAssistantResponse(ctx, rc.SessionID)
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

	return r.ExecuteToolCall(ctx, tc)
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

// isModelError checks if an error is a model-related API error (400 status).
func (r *SubagentRunner) isModelError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Check for 400 Bad Request and model-related error messages
	return strings.Contains(errStr, "400") &&
		(strings.Contains(errStr, "model") || strings.Contains(errStr, "Bad Request"))
}

// Package usecase contains application use cases.
package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// Special tool names for investigation control.
const (
	toolCompleteInvestigation = "complete_investigation"
	toolEscalateInvestigation = "escalate_investigation"
	toolBash                  = "bash"
)

// statusEscalated is the investigation status when the AI escalates for human review.
const statusEscalated = "escalated"

// RCAServiceInterface defines the interface for Root Cause Analysis correlation.
type RCAServiceInterface interface {
	Correlate(ctx context.Context, findings []entity.InvestigationFinding) ([]entity.RCAFinding, error)
}

// InvestigationRunner orchestrates AI-driven alert investigations.
// It manages the conversation loop with an AI provider, executes tools,
// and tracks investigation progress.
type InvestigationRunner struct {
	BaseRunner

	safetyEnforcer SafetyEnforcer
	promptBuilder  PromptBuilderRegistry
	skillManager   port.SkillManager
	rcaService     RCAServiceInterface
	store          InvestigationStoreWriter
	uiAdapter      port.UserInterface
	config         AlertInvestigationUseCaseConfig
}

// NewInvestigationRunner creates a new InvestigationRunner with the required dependencies.
//
// Parameters:
//   - convService: Service for managing AI conversation sessions
//   - toolExecutor: Executor for running investigation tools
//   - safetyEnforcer: Enforcer for safety policies during investigation (optional, can be nil)
//   - promptBuilder: Registry for building investigation prompts
//   - skillManager: Manager for discovering and loading skills (optional, can be nil)
//   - rcaService: Service for Root Cause Analysis correlation (optional, can be nil)
//   - uiAdapter: User interface for displaying thinking and messages (optional, can be nil)
//   - config: Configuration for investigation limits and behavior
//   - logger: Structured logger for logging investigation events
//
// Panics if required dependencies (convService, toolExecutor, promptBuilder) are nil.
func NewInvestigationRunner(
	convService ConversationServiceInterface,
	toolExecutor port.ToolExecutor,
	safetyEnforcer SafetyEnforcer,
	promptBuilder PromptBuilderRegistry,
	skillManager port.SkillManager,
	rcaService RCAServiceInterface,
	uiAdapter port.UserInterface,
	config AlertInvestigationUseCaseConfig,
	logger port.Logger,
) *InvestigationRunner {
	if convService == nil {
		panic("convService cannot be nil")
	}
	if toolExecutor == nil {
		panic("toolExecutor cannot be nil")
	}
	if promptBuilder == nil {
		panic("promptBuilder cannot be nil")
	}
	// safetyEnforcer, skillManager, rcaService, and uiAdapter are optional and can be nil

	return &InvestigationRunner{
		BaseRunner:     newBaseRunner(convService, toolExecutor, newSafetyPermissionChecker(safetyEnforcer), logger),
		safetyEnforcer: safetyEnforcer,
		promptBuilder:  promptBuilder,
		skillManager:   skillManager,
		rcaService:     rcaService,
		uiAdapter:      uiAdapter,
		config:         config,
	}
}

// NewInvestigationRunnerWithStore creates a new InvestigationRunner with persistence support.
//
// Parameters:
//   - convService: Service for managing AI conversation sessions
//   - toolExecutor: Executor for running investigation tools
//   - safetyEnforcer: Enforcer for safety policies during investigation (optional, can be nil)
//   - promptBuilder: Registry for building investigation prompts
//   - skillManager: Manager for discovering and loading skills (optional, can be nil)
//   - rcaService: Service for Root Cause Analysis correlation (optional, can be nil)
//   - uiAdapter: User interface for displaying thinking and messages (optional, can be nil)
//   - store: Store for persisting investigation state
//   - config: Configuration for investigation limits and behavior
//   - logger: Structured logger for logging investigation events
//
// Panics if required dependencies (convService, toolExecutor, promptBuilder) are nil.
func NewInvestigationRunnerWithStore(
	convService ConversationServiceInterface,
	toolExecutor port.ToolExecutor,
	safetyEnforcer SafetyEnforcer,
	promptBuilder PromptBuilderRegistry,
	skillManager port.SkillManager,
	rcaService RCAServiceInterface,
	uiAdapter port.UserInterface,
	store InvestigationStoreWriter,
	config AlertInvestigationUseCaseConfig,
	logger port.Logger,
) *InvestigationRunner {
	if convService == nil {
		panic("convService cannot be nil")
	}
	if toolExecutor == nil {
		panic("toolExecutor cannot be nil")
	}
	if promptBuilder == nil {
		panic("promptBuilder cannot be nil")
	}
	// safetyEnforcer, skillManager, rcaService, and uiAdapter are optional and can be nil

	return &InvestigationRunner{
		BaseRunner:     newBaseRunner(convService, toolExecutor, newSafetyPermissionChecker(safetyEnforcer), logger),
		safetyEnforcer: safetyEnforcer,
		promptBuilder:  promptBuilder,
		skillManager:   skillManager,
		rcaService:     rcaService,
		uiAdapter:      uiAdapter,
		store:          store,
		config:         config,
	}
}

// SetRCAService sets the RCA service. InvestigationRunner is not shared across
// goroutines; each investigation creates a new runner instance.
func (r *InvestigationRunner) SetRCAService(rcaService RCAServiceInterface) {
	r.rcaService = rcaService
}

// runContext holds state for an investigation run.
type runContext struct {
	BaseRunContext

	alert           *AlertForInvestigation
	investigationID string
	logger          port.Logger
}

// failedResult creates a failed investigation result.
func (rc *runContext) failedResult(err error) *InvestigationResult {
	return &InvestigationResult{
		InvestigationID: rc.investigationID,
		AlertID:         rc.alert.ID(),
		Status:          "failed",
		ActionsTaken:    rc.ActionsTaken,
		Duration:        time.Since(rc.StartTime),
		Error:           err,
	}
}

func (r *InvestigationRunner) logToolResult(log port.Logger, tc port.ToolCallInfo, result entity.ToolResult) {
	log = log.With("tool", tc.ToolName, "tool_id", tc.ToolID)
	if result.IsError {
		log.Error("tool execution failed", "error_message", result.Result)
		return
	}
	log.Info("tool execution completed", "result_bytes", len(result.Result))
	log.Debug("tool execution result preview", "result_preview", truncateForLog(result.Result, 500))
}

func truncateForLog(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

// checkCommandSafety validates command-level safety for bash tools.
// Tool-level permission checking is handled by ProcessToolCalls via PermissionChecker.
// Returns nil if safe, or an error describing the block reason.
func (r *InvestigationRunner) checkCommandSafety(tc port.ToolCallInfo) error {
	if r.safetyEnforcer == nil {
		return nil
	}

	// For bash tools, check command safety
	if tc.ToolName == toolBash {
		if cmd := extractCommandFromInput(tc.Input); cmd != "" {
			if err := r.safetyEnforcer.CheckCommandAllowed(cmd); err != nil {
				return fmt.Errorf("command blocked: %w", err)
			}
		}
	}

	return nil
}

// extractCommandFromInput extracts the command string from bash tool input.
func extractCommandFromInput(input map[string]interface{}) string {
	if input == nil {
		return ""
	}
	if cmd, ok := input["command"].(string); ok {
		return cmd
	}
	return ""
}

// processToolCalls delegates to BaseRunner.ProcessToolCalls with safety-checked execution.
func (r *InvestigationRunner) processToolCalls(rc *runContext, toolCalls []port.ToolCallInfo) error {
	executor := func(ctx context.Context, tc port.ToolCallInfo) entity.ToolResult {
		if err := r.checkCommandSafety(tc); err != nil {
			blocked := entity.ToolResult{ToolID: tc.ToolID, Result: err.Error(), IsError: true}
			r.logToolResult(rc.logger, tc, blocked)
			return blocked
		}
		result := r.ExecuteToolCall(ctx, tc)
		r.logToolResult(rc.logger, tc, result)
		return result
	}
	return r.ProcessToolCalls(&rc.BaseRunContext, toolCalls, "is not allowed for this investigation", executor)
}

// Run executes an investigation for the given alert.
//
// The investigation follows this flow:
//  1. Validate inputs (alert, investigationID)
//  2. Start a new conversation session
//  3. Build investigation prompt using the prompt builder
//  4. Send the prompt to the AI
//  5. Process AI responses in a loop:
//     - If AI requests tools: execute allowed tools, feed results back
//     - If AI completes: extract findings and return result
//     - If budget/timeout exceeded: escalate
//  6. Clean up conversation session
//
// Parameters:
//   - ctx: Context for cancellation and timeout
//   - alert: The alert to investigate
//   - investigationID: Unique identifier for this investigation
//
// Returns:
//   - *InvestigationResult: Result of the investigation
//   - error: Any error that occurred during investigation
func (r *InvestigationRunner) Run(
	ctx context.Context,
	alert *AlertForInvestigation,
	investigationID string,
) (*InvestigationResult, error) {
	if err := r.validateInputs(ctx, alert, investigationID); err != nil {
		return r.validationFailedResult(investigationID, alert, err), err
	}

	// Determine maxActions from SafetyEnforcer if available, otherwise use default
	maxActions := 50
	if r.safetyEnforcer != nil {
		maxActions = r.safetyEnforcer.GetMaxActions()
	}

	log := r.logger.With("investigation_id", investigationID, "alert_id", alert.ID())

	rc := &runContext{
		BaseRunContext: BaseRunContext{
			Ctx:        ctx,
			StartTime:  time.Now(),
			MaxActions: maxActions,
		},
		alert:           alert,
		investigationID: investigationID,
		logger:          log,
	}

	sessionID, err := r.ConvService.StartConversation(ctx)
	if err != nil {
		return rc.failedResult(err), err
	}
	rc.SessionID = sessionID
	// Defers run in LIFO order: deregister investigation runs before session cleanup.
	// This is safe because CleanupConversation doesn't invoke investigation tools.
	defer r.CleanupConversation(sessionID, investigationID, "investigation_id")
	cleanup := r.registerInvestigation(investigationID)
	defer cleanup()

	// Configure extended thinking mode if enabled
	if r.config.ExtendedThinking {
		thinkingBudget := r.config.ThinkingBudget
		if thinkingBudget == 0 {
			thinkingBudget = 10000 // Default budget
		}
		thinkingInfo := port.ThinkingModeInfo{
			Enabled:      true,
			BudgetTokens: thinkingBudget,
			ShowThinking: r.config.ShowThinking,
		}
		_ = r.ConvService.SetThinkingMode(rc.SessionID, thinkingInfo)
	}

	if err := r.sendInitialPrompt(rc); err != nil {
		return rc.failedResult(err), err
	}

	result, err := r.runInvestigationLoop(rc)

	// Perform Root Cause Analysis if findings were gathered and RCA service is available
	if result != nil && (result.Status == "completed" || result.Status == statusEscalated) && len(result.Findings) > 0 && r.rcaService != nil {
		rc.logger.Info("findings gathered, triggering RCA correlation", "findings_count", len(result.Findings))

		// Convert findings to InvestigationFinding entities for the RCA service
		// In a real scenario, we might want to store InvestigationFinding entities in the result directly.
		// For now, we'll create simple observation findings from the string findings.
		invFindings := make([]entity.InvestigationFinding, len(result.Findings))
		for i, f := range result.Findings {
			invFindings[i] = entity.InvestigationFinding{
				Type:        "observation",
				Description: f,
				Severity:    "info",
				Timestamp:   time.Now(),
			}
		}

		rcaFindings, rcaErr := r.rcaService.Correlate(ctx, invFindings)
		if rcaErr != nil {
			rc.logger.Error("RCA correlation failed", "error", rcaErr)
		} else if len(rcaFindings) == 0 {
			rc.logger.Warn("RCA correlation returned no findings")
		} else {
			result.RCAFindings = rcaFindings
			rc.logger.Info("RCA correlation successful", "rca_findings_count", len(rcaFindings))
		}
	}

	// Persist result to store if configured
	if r.store != nil && result != nil {
		stub := &simpleInvestigationRecord{
			id:             result.InvestigationID,
			alertID:        result.AlertID,
			sessionID:      rc.SessionID,
			status:         result.Status,
			startedAt:      rc.StartTime,
			completedAt:    time.Now(),
			findings:       result.Findings,
			actionsTaken:   result.ActionsTaken,
			durationNanos:  int64(result.Duration),
			confidence:     result.Confidence,
			escalated:      result.Escalated,
			escalateReason: result.EscalateReason,
		}
		if err := r.store.Store(ctx, stub); err != nil {
			rc.logger.Error("failed to store result", "error", err)
		}
	}

	return result, err
}

func (r *InvestigationRunner) validateInputs(ctx context.Context, alert *AlertForInvestigation, invID string) error {
	if alert == nil {
		return errors.New("nil alert")
	}
	if alert.ID() == "" {
		return errors.New("empty alert ID")
	}
	if strings.TrimSpace(invID) == "" {
		return errors.New("empty investigation ID")
	}
	return ctx.Err()
}

// registerInvestigation registers the investigation ID with the tool executor (if it implements
// InvestigationRegistrar) and returns a cleanup function that deregisters it on all exit paths.
// Call as: defer r.registerInvestigation(id)().
func (r *InvestigationRunner) registerInvestigation(investigationID string) func() {
	if registrar, ok := r.ToolExecutor.(port.InvestigationRegistrar); ok {
		registrar.RegisterInvestigation(investigationID)
	}
	return func() {
		if deregistrar, ok := r.ToolExecutor.(port.InvestigationDeregistrar); ok {
			deregistrar.DeregisterInvestigation(investigationID)
		}
	}
}

func (r *InvestigationRunner) validationFailedResult(
	invID string,
	alert *AlertForInvestigation,
	err error,
) *InvestigationResult {
	alertID := ""
	if alert != nil {
		alertID = alert.ID()
	}
	return &InvestigationResult{InvestigationID: invID, AlertID: alertID, Status: "failed", Error: err}
}

func (r *InvestigationRunner) sendInitialPrompt(rc *runContext) error {
	alertView := r.createAlertView(rc.alert, rc.investigationID)

	// Get available tools for this investigation
	tools, err := r.getInvestigationTools()
	if err != nil {
		return err
	}

	// Get available skills if skill manager is configured
	var skills []port.SkillInfo
	if r.skillManager != nil {
		result, err := r.skillManager.DiscoverSkills(rc.Ctx)
		if err == nil && result != nil {
			skills = result.Skills
		}
		// Silently ignore skill discovery errors - skills are optional
	}

	// Build investigation prompt with full context and instructions
	prompt, err := r.promptBuilder.BuildPromptForAlert(alertView, tools, skills)
	if err != nil {
		return err
	}

	// Set the full investigation prompt as a custom system prompt.
	// This keeps the detailed instructions, tool descriptions, and guidelines
	// in the system context rather than cluttering the conversation history.
	if err := r.ConvService.SetCustomSystemPrompt(rc.Ctx, rc.SessionID, prompt); err != nil {
		return err
	}

	// Send a minimal user message to trigger the investigation.
	// The system prompt contains full context including the investigation ID.
	// This trigger message reinforces the ID and alert identifiers to start the conversation.
	userMessage := r.formatTriggerMessage(rc.alert, rc.investigationID)
	if _, err := r.ConvService.AddUserMessage(rc.Ctx, rc.SessionID, userMessage); err != nil {
		return err
	}

	return nil
}

// createAlertView converts an AlertForInvestigation into an AlertView for prompt building.
// The investigationID is embedded so the prompt builder can include it for the AI.
func (r *InvestigationRunner) createAlertView(alert *AlertForInvestigation, investigationID string) *AlertView {
	return &AlertView{
		id:              alert.ID(),
		source:          alert.Source(),
		severity:        alert.Severity(),
		title:           alert.Title(),
		description:     alert.Description(),
		labels:          alert.Labels(),
		investigationID: investigationID,
	}
}

// formatTriggerMessage creates a minimal user message to trigger the investigation.
// This message contains only the investigation ID and essential alert identifiers since
// the full context is already provided in the system prompt.
func (r *InvestigationRunner) formatTriggerMessage(alert *AlertForInvestigation, investigationID string) string {
	return fmt.Sprintf("Investigation ID: %s\nAlert ID: %s\nTitle: %s", investigationID, alert.ID(), alert.Title())
}

// getInvestigationTools returns the filtered list of tools for investigation prompts.
// It filters based on the SafetyEnforcer's allowed tools policy.
func (r *InvestigationRunner) getInvestigationTools() ([]entity.Tool, error) {
	allTools, err := r.ToolExecutor.ListTools()
	if err != nil {
		return nil, err
	}

	// If no safety enforcer, return all tools
	if r.safetyEnforcer == nil {
		return allTools, nil
	}

	// Filter to only tools allowed by safety enforcer
	filtered := make([]entity.Tool, 0, len(allTools))
	for _, tool := range allTools {
		if r.safetyEnforcer.CheckToolAllowed(tool.Name) == nil {
			filtered = append(filtered, tool)
		}
	}
	return filtered, nil
}

// separatedToolCalls holds tool calls separated into regular and special categories.
type separatedToolCalls struct {
	regular    []port.ToolCallInfo
	completion *port.ToolCallInfo
	escalation *port.ToolCallInfo
}

// separateToolCalls separates tool calls into regular tools and special completion/escalation tools.
func separateToolCalls(toolCalls []port.ToolCallInfo) separatedToolCalls {
	var result separatedToolCalls
	for i := range toolCalls {
		switch toolCalls[i].ToolName {
		case toolCompleteInvestigation:
			result.completion = &toolCalls[i]
		case toolEscalateInvestigation:
			result.escalation = &toolCalls[i]
		default:
			result.regular = append(result.regular, toolCalls[i])
		}
	}
	return result
}

// extractStringSlice extracts a []string from a []interface{} in tool input.
func extractStringSlice(input map[string]interface{}, key string) []string {
	items, ok := input[key].([]interface{})
	if !ok {
		return nil
	}
	var result []string
	for _, item := range items {
		if s, ok := item.(string); ok {
			result = append(result, s)
		}
	}
	return result
}

// buildCompletionResult creates a result from complete_investigation tool input.
func (rc *runContext) buildCompletionResult(input map[string]interface{}) *InvestigationResult {
	result := &InvestigationResult{
		InvestigationID: rc.investigationID,
		AlertID:         rc.alert.ID(),
		Status:          "completed",
		ActionsTaken:    rc.ActionsTaken,
		Duration:        time.Since(rc.StartTime),
	}
	if confidence, ok := input["confidence"].(float64); ok {
		result.Confidence = confidence
	}
	result.Findings = extractStringSlice(input, "findings")
	return result
}

// buildEscalationResult creates a result from escalate_investigation tool input.
func (rc *runContext) buildEscalationResult(input map[string]interface{}) *InvestigationResult {
	result := &InvestigationResult{
		InvestigationID: rc.investigationID,
		AlertID:         rc.alert.ID(),
		Status:          statusEscalated,
		Escalated:       true,
		ActionsTaken:    rc.ActionsTaken,
		Duration:        time.Since(rc.StartTime),
	}
	if reason, ok := input["reason"].(string); ok {
		result.EscalateReason = reason
	}
	result.Findings = extractStringSlice(input, "partial_findings")
	return result
}

// checkSafetyTimeout checks if the safety enforcer reports a timeout.
func (r *InvestigationRunner) checkSafetyTimeout(rc *runContext) error {
	if r.safetyEnforcer == nil {
		return nil
	}
	return r.safetyEnforcer.CheckTimeout(rc.Ctx)
}

// checkSafetyBudget checks if the safety enforcer reports budget exhaustion.
func (r *InvestigationRunner) checkSafetyBudget(rc *runContext) error {
	if r.safetyEnforcer == nil {
		return nil
	}
	return r.safetyEnforcer.CheckActionBudget(rc.ActionsTaken)
}

// checkConfidenceEscalation checks if the AI's confidence is below the escalation threshold.
// Returns an escalation result if confidence is low, nil otherwise.
// NOTE: Confidence-based escalation is currently disabled as it requires policy configuration
// that is not part of the SafetyEnforcer interface. This can be re-enabled by extending
// the interface or adding a separate escalation policy.
func (r *InvestigationRunner) checkConfidenceEscalation(_ *runContext, _ *entity.Message) *InvestigationResult {
	// Confidence-based escalation disabled - safety enforcement handles action limits
	return nil
}

// escalatedResult creates a failed result with escalation info.
func (rc *runContext) escalatedResult(err error, reason string) *InvestigationResult {
	result := rc.failedResult(err)
	result.Escalated = true
	result.EscalateReason = reason
	return result
}

func (r *InvestigationRunner) runInvestigationLoop(rc *runContext) (*InvestigationResult, error) {
	for {
		if err := rc.Ctx.Err(); err != nil {
			return nil, err
		}

		if err := r.checkSafetyTimeout(rc); err != nil {
			return rc.escalatedResult(err, "timeout: "+err.Error()), err
		}

		msg, toolCalls, err := r.getNextToolCalls(rc)
		if err != nil {
			return rc.failedResult(err), err
		}

		if len(toolCalls) == 0 {
			return r.handleNoToolCalls(rc, msg)
		}

		if err := r.checkSafetyBudget(rc); err != nil {
			return rc.escalatedResult(err, "action budget exceeded: "+err.Error()), err
		}

		result, done, err := r.processLoopIteration(rc, toolCalls)
		if done {
			return result, err
		}

		r.InjectTurnWarningIfNeeded(&rc.BaseRunContext, TurnWarningConfig{WarningThreshold: 5, BatchToolHint: "batch_tool"})

		if rc.ActionsTaken >= rc.MaxActions {
			if err := r.handleMaxActionsReached(rc); err != nil {
				rc.logger.Error("error handling max actions", "error", err)
			}
			break
		}
	}
	rc.logger.Info("investigation loop ended naturally")
	return rc.completedResult(), nil
}

// handleNoToolCalls handles the case where AI responds without requesting any tools.
// Returns the appropriate investigation result based on confidence checks.
func (r *InvestigationRunner) handleNoToolCalls(rc *runContext, msg *entity.Message) (*InvestigationResult, error) {
	// Check for low confidence escalation before completing
	if result := r.checkConfidenceEscalation(rc, msg); result != nil {
		return result, nil
	}

	// AI responded without tool calls - log the message
	msgContent := ""
	if msg != nil {
		msgContent = msg.Content
		if len(msgContent) > 200 {
			msgContent = msgContent[:200] + "..."
		}
	}
	rc.logger.Info("AI responded without tool calls", "message_preview", msgContent)

	// End loop naturally and return completed result
	return rc.completedResult(), nil
}

// handleMaxActionsReached handles the scenario where max actions limit is reached.
// Sends a summary request and allows one final AI response.
func (r *InvestigationRunner) handleMaxActionsReached(rc *runContext) error {
	rc.logger.Info("max actions limit reached, requesting summary",
		"actions_taken", rc.ActionsTaken,
		"max_actions", rc.MaxActions)

	summaryMsg := "TURN LIMIT REACHED: You have reached the maximum number of allowed turns for this investigation. Please provide a summary of your findings and conclusions based on the investigation performed so far."
	if _, err := r.ConvService.AddUserMessage(rc.Ctx, rc.SessionID, summaryMsg); err != nil {
		rc.logger.Error("failed to add summary request", "error", err)
		return err
	}

	_, _, err := r.ConvService.ProcessAssistantResponse(rc.Ctx, rc.SessionID)
	if err != nil {
		rc.logger.Error("error processing final summary response", "error", err)
		return err
	}

	return nil
}

// displayThinkingContent outputs thinking content to the UI adapter or stderr.
func (r *InvestigationRunner) displayThinkingContent(content string) {
	if content == "" {
		return
	}
	if r.uiAdapter != nil {
		_ = r.uiAdapter.DisplayThinking(content)
		return
	}
	// Fallback to slog if no UI adapter - log ONLY the length for security
	r.logger.Debug("THINKING_CONTENT_RECEIVED", "length", len(content))
}

// getNextToolCalls retrieves and limits the next batch of tool calls.
// Also returns the AI message for confidence analysis.
// When ShowThinking is enabled, uses streaming to display thinking output.
func (r *InvestigationRunner) getNextToolCalls(rc *runContext) (*entity.Message, []port.ToolCallInfo, error) {
	var msg *entity.Message
	var toolCalls []port.ToolCallInfo
	var err error

	if r.config.ShowThinking {
		var thinkingContent strings.Builder
		thinkingCallback := func(thinking string) error {
			thinkingContent.WriteString(thinking)
			return nil
		}
		msg, toolCalls, err = r.ConvService.ProcessAssistantResponseStreaming(
			rc.Ctx, rc.SessionID, nil, thinkingCallback,
		)
		r.displayThinkingContent(thinkingContent.String())
	} else {
		msg, toolCalls, err = r.ConvService.ProcessAssistantResponse(rc.Ctx, rc.SessionID)
	}

	if err != nil {
		return nil, nil, err
	}
	return msg, r.LimitToolCalls(&rc.BaseRunContext, toolCalls), nil
}

// processLoopIteration handles one iteration of tool processing.
// Returns (result, done, err) - result and err on exit, (nil, false, nil) to continue.
func (r *InvestigationRunner) processLoopIteration(
	rc *runContext,
	toolCalls []port.ToolCallInfo,
) (*InvestigationResult, bool, error) {
	separated := separateToolCalls(toolCalls)

	if len(separated.regular) > 0 {
		if err := r.processToolCalls(rc, separated.regular); err != nil {
			return rc.failedResult(err), true, err
		}
	}

	if separated.completion != nil {
		// Log the truncated input for debugging without leaking full sensitive findings
		inputJSON, _ := json.Marshal(separated.completion.Input)
		inputPreview := string(inputJSON)
		if len(inputPreview) > 200 {
			inputPreview = inputPreview[:200] + "..."
		}
		rc.logger.Info("complete_investigation called",
			"tool_id", separated.completion.ToolID,
			"input_preview", inputPreview,
		)

		return rc.buildCompletionResult(separated.completion.Input), true, nil
	}

	if separated.escalation != nil {
		rc.logger.Info("escalate_investigation called", "tool_id", separated.escalation.ToolID)
		return rc.buildEscalationResult(separated.escalation.Input), true, nil
	}

	return nil, false, nil
}

// completedResult creates a successful completion result.
func (rc *runContext) completedResult() *InvestigationResult {
	return &InvestigationResult{
		InvestigationID: rc.investigationID,
		AlertID:         rc.alert.ID(),
		Status:          "completed",
		ActionsTaken:    rc.ActionsTaken,
		Duration:        time.Since(rc.StartTime),
	}
}

// Package usecase contains application use cases.
package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

// Special tool names for investigation control.
const (
	toolCompleteInvestigation = "complete_investigation"
	toolEscalateInvestigation = "escalate_investigation"
	toolBash                  = "bash"
)

// InvestigationRunner orchestrates AI-driven alert investigations.
// It manages the conversation loop with an AI provider, executes tools,
// and tracks investigation progress.
type InvestigationRunner struct {
	convService    ConversationServiceInterface
	toolExecutor   port.ToolExecutor
	safetyEnforcer SafetyEnforcer
	promptBuilder  PromptBuilderRegistry
	skillManager   port.SkillManager
	store          InvestigationStoreWriter
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
//   - config: Configuration for investigation limits and behavior
//
// Panics if required dependencies (convService, toolExecutor, promptBuilder) are nil.
func NewInvestigationRunner(
	convService ConversationServiceInterface,
	toolExecutor port.ToolExecutor,
	safetyEnforcer SafetyEnforcer,
	promptBuilder PromptBuilderRegistry,
	skillManager port.SkillManager,
	config AlertInvestigationUseCaseConfig,
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
	// safetyEnforcer and skillManager are optional and can be nil

	return &InvestigationRunner{
		convService:    convService,
		toolExecutor:   toolExecutor,
		safetyEnforcer: safetyEnforcer,
		promptBuilder:  promptBuilder,
		skillManager:   skillManager,
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
//   - store: Store for persisting investigation state
//   - config: Configuration for investigation limits and behavior
//
// Panics if required dependencies (convService, toolExecutor, promptBuilder) are nil.
func NewInvestigationRunnerWithStore(
	convService ConversationServiceInterface,
	toolExecutor port.ToolExecutor,
	safetyEnforcer SafetyEnforcer,
	promptBuilder PromptBuilderRegistry,
	skillManager port.SkillManager,
	store InvestigationStoreWriter,
	config AlertInvestigationUseCaseConfig,
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
	// safetyEnforcer and skillManager are optional and can be nil

	return &InvestigationRunner{
		convService:    convService,
		toolExecutor:   toolExecutor,
		safetyEnforcer: safetyEnforcer,
		promptBuilder:  promptBuilder,
		skillManager:   skillManager,
		store:          store,
		config:         config,
	}
}

// runContext holds state for an investigation run.
type runContext struct {
	ctx             context.Context
	alert           *AlertForInvestigation
	investigationID string
	sessionID       string
	startTime       time.Time
	actionsTaken    int
	maxActions      int
}

// failedResult creates a failed investigation result.
func (rc *runContext) failedResult(err error) *InvestigationResult {
	return &InvestigationResult{
		InvestigationID: rc.investigationID,
		AlertID:         rc.alert.ID(),
		Status:          "failed",
		ActionsTaken:    rc.actionsTaken,
		Duration:        time.Since(rc.startTime),
		Error:           err,
	}
}

// executeToolCall executes a single tool call and returns the result.
func (r *InvestigationRunner) executeToolCall(ctx context.Context, tc port.ToolCallInfo) entity.ToolResult {
	// Check safety enforcer if configured
	if err := r.checkToolSafety(tc); err != nil {
		return entity.ToolResult{ToolID: tc.ToolID, Result: err.Error(), IsError: true}
	}

	result, execErr := r.toolExecutor.ExecuteTool(ctx, tc.ToolName, tc.Input)
	if execErr != nil {
		return entity.ToolResult{ToolID: tc.ToolID, Result: execErr.Error(), IsError: true}
	}
	return entity.ToolResult{ToolID: tc.ToolID, Result: result, IsError: false}
}

// checkToolSafety validates tool and command safety using the safety enforcer.
// Returns nil if safe, or an error describing the block reason.
func (r *InvestigationRunner) checkToolSafety(tc port.ToolCallInfo) error {
	if r.safetyEnforcer == nil {
		return nil
	}

	if err := r.safetyEnforcer.CheckToolAllowed(tc.ToolName); err != nil {
		return errors.New("Tool blocked: " + err.Error())
	}

	// For bash tools, also check command safety
	if tc.ToolName == toolBash {
		if cmd := extractCommandFromInput(tc.Input); cmd != "" {
			if err := r.safetyEnforcer.CheckCommandAllowed(cmd); err != nil {
				return errors.New("Command blocked: " + err.Error())
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

// processToolCalls executes tool calls and feeds results back.
func (r *InvestigationRunner) processToolCalls(rc *runContext, toolCalls []port.ToolCallInfo) error {
	var toolResults []entity.ToolResult
	for _, tc := range toolCalls {
		toolResults = append(toolResults, r.executeToolCall(rc.ctx, tc))
		rc.actionsTaken++
	}
	if len(toolResults) > 0 {
		return r.convService.AddToolResultMessage(rc.ctx, rc.sessionID, toolResults)
	}
	return nil
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

	rc := &runContext{
		ctx:             ctx,
		alert:           alert,
		investigationID: investigationID,
		startTime:       time.Now(),
		maxActions:      r.config.MaxActions,
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

	if err := r.sendInitialPrompt(rc); err != nil {
		return rc.failedResult(err), err
	}

	result, err := r.runInvestigationLoop(rc)

	// Persist result to store if configured
	if r.store != nil && result != nil {
		stub := &investigationRecordForStore{
			id:             result.InvestigationID,
			alertID:        result.AlertID,
			sessionID:      rc.sessionID,
			status:         result.Status,
			startedAt:      rc.startTime,
			completedAt:    time.Now(),
			findings:       result.Findings,
			actionsTaken:   result.ActionsTaken,
			durationNanos:  int64(result.Duration),
			confidence:     result.Confidence,
			escalated:      result.Escalated,
			escalateReason: result.EscalateReason,
		}
		if err := r.store.Store(ctx, stub); err != nil {
			fmt.Fprintf(
				os.Stderr,
				"[InvestigationRunner] Failed to store result for %s: %v\n",
				result.InvestigationID,
				err,
			)
		}
	}

	return result, err
}

// investigationRecordForStore implements InvestigationRecordData for persistence.
type investigationRecordForStore struct {
	id, alertID, sessionID, status string
	startedAt                      time.Time
	completedAt                    time.Time
	findings                       []string
	actionsTaken                   int
	durationNanos                  int64
	confidence                     float64
	escalated                      bool
	escalateReason                 string
}

func (s *investigationRecordForStore) ID() string        { return s.id }
func (s *investigationRecordForStore) AlertID() string   { return s.alertID }
func (s *investigationRecordForStore) SessionID() string { return s.sessionID }
func (s *investigationRecordForStore) Status() string    { return s.status }
func (s *investigationRecordForStore) StartedAt() time.Time {
	if s.startedAt.IsZero() {
		return time.Now()
	}
	return s.startedAt
}
func (s *investigationRecordForStore) CompletedAt() time.Time  { return s.completedAt }
func (s *investigationRecordForStore) Findings() []string      { return s.findings }
func (s *investigationRecordForStore) ActionsTaken() int       { return s.actionsTaken }
func (s *investigationRecordForStore) Duration() time.Duration { return time.Duration(s.durationNanos) }
func (s *investigationRecordForStore) Confidence() float64     { return s.confidence }
func (s *investigationRecordForStore) Escalated() bool         { return s.escalated }
func (s *investigationRecordForStore) EscalateReason() string  { return s.escalateReason }

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
	// Create alert view for prompt building
	alertView := r.createAlertView(rc.alert)

	// Get available tools for this investigation
	tools, err := r.getInvestigationTools()
	if err != nil {
		return err
	}

	// Get available skills if skill manager is configured
	var skills []port.SkillInfo
	if r.skillManager != nil {
		result, err := r.skillManager.DiscoverSkills(rc.ctx)
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
	if err := r.convService.SetCustomSystemPrompt(rc.ctx, rc.sessionID, prompt); err != nil {
		return err
	}

	// Send a minimal user message to trigger the investigation.
	// Since the system prompt already contains all context, we only need
	// basic alert identifiers here to start the conversation.
	userMessage := r.formatTriggerMessage(rc.alert)
	if _, err := r.convService.AddUserMessage(rc.ctx, rc.sessionID, userMessage); err != nil {
		return err
	}

	return nil
}

// createAlertView converts an AlertForInvestigation into an AlertView for prompt building.
func (r *InvestigationRunner) createAlertView(alert *AlertForInvestigation) *AlertView {
	return &AlertView{
		id:          alert.ID(),
		source:      alert.Source(),
		severity:    alert.Severity(),
		title:       alert.Title(),
		description: alert.Description(),
		labels:      alert.Labels(),
	}
}

// formatTriggerMessage creates a minimal user message to trigger the investigation.
// This message contains only the essential alert identifiers since the full context
// is already provided in the system prompt.
func (r *InvestigationRunner) formatTriggerMessage(alert *AlertForInvestigation) string {
	return fmt.Sprintf("Alert ID: %s\nTitle: %s", alert.ID(), alert.Title())
}

// getInvestigationTools returns the filtered list of tools for investigation prompts.
// It filters based on the AllowedTools configuration.
func (r *InvestigationRunner) getInvestigationTools() ([]entity.Tool, error) {
	allTools, err := r.toolExecutor.ListTools()
	if err != nil {
		return nil, err
	}

	// If no allowed tools configured, return all tools
	if len(r.config.AllowedTools) == 0 {
		return allTools, nil
	}

	// Filter to only allowed tools
	allowedSet := make(map[string]bool, len(r.config.AllowedTools))
	for _, t := range r.config.AllowedTools {
		allowedSet[t] = true
	}

	filtered := make([]entity.Tool, 0, len(allTools))
	for _, tool := range allTools {
		if allowedSet[tool.Name] {
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
		ActionsTaken:    rc.actionsTaken,
		Duration:        time.Since(rc.startTime),
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
		Status:          "escalated",
		Escalated:       true,
		ActionsTaken:    rc.actionsTaken,
		Duration:        time.Since(rc.startTime),
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
	return r.safetyEnforcer.CheckTimeout(rc.ctx)
}

// checkSafetyBudget checks if the safety enforcer reports budget exhaustion.
func (r *InvestigationRunner) checkSafetyBudget(rc *runContext) error {
	if r.safetyEnforcer == nil {
		return nil
	}
	return r.safetyEnforcer.CheckActionBudget(rc.actionsTaken)
}

// checkConfidenceEscalation checks if the AI's confidence is below the escalation threshold.
// Returns an escalation result if confidence is low, nil otherwise.
func (r *InvestigationRunner) checkConfidenceEscalation(rc *runContext, msg *entity.Message) *InvestigationResult {
	if r.config.EscalateOnConfidence <= 0 || msg == nil {
		return nil
	}

	confidence := parseConfidenceFromMessage(msg.Content)
	if confidence >= 0 && confidence < r.config.EscalateOnConfidence {
		result := rc.completedResult()
		result.Escalated = true
		result.Confidence = confidence
		result.EscalateReason = "confidence below threshold"
		return result
	}
	return nil
}

// parseConfidenceFromMessage extracts a confidence value from message text.
// Looks for patterns like "Confidence: 0.5" or "confidence: 0.5".
// Returns -1 if no confidence found.
func parseConfidenceFromMessage(content string) float64 {
	// Look for "Confidence: X.X" pattern (case-insensitive)
	lower := strings.ToLower(content)
	idx := strings.Index(lower, "confidence:")
	if idx == -1 {
		return -1
	}

	// Extract the number after "confidence:"
	remaining := strings.TrimSpace(content[idx+len("confidence:"):])
	var confidence float64
	_, err := fmt.Sscanf(remaining, "%f", &confidence)
	if err != nil {
		return -1
	}

	// Validate range
	if confidence < 0 || confidence > 1 {
		return -1
	}
	return confidence
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
		if err := rc.ctx.Err(); err != nil {
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

		r.injectTurnWarningIfNeeded(rc)

		if rc.actionsTaken >= rc.maxActions {
			if err := r.handleMaxActionsReached(rc); err != nil {
				fmt.Fprintf(os.Stderr, "[InvestigationRunner] Error handling max actions: %v\n", err)
			}
			break
		}
	}
	fmt.Fprintf(
		os.Stderr,
		"[InvestigationRunner] Investigation loop ended naturally (no complete_investigation call). Using default completedResult.\n",
	)
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
	fmt.Fprintf(os.Stderr, "[InvestigationRunner] AI responded without tool calls. Message: %s\n", msgContent)

	// End loop naturally and return completed result
	fmt.Fprintf(
		os.Stderr,
		"[InvestigationRunner] Investigation loop ended naturally (no complete_investigation call). Using default completedResult.\n",
	)
	return rc.completedResult(), nil
}

// injectTurnWarningIfNeeded injects a warning message if the agent is approaching the turn limit.
func (r *InvestigationRunner) injectTurnWarningIfNeeded(rc *runContext) {
	remaining := rc.maxActions - rc.actionsTaken
	warningMsg := r.buildTurnWarningMessage(remaining)
	if warningMsg != "" {
		if _, err := r.convService.AddUserMessage(rc.ctx, rc.sessionID, warningMsg); err != nil {
			fmt.Fprintf(os.Stderr, "[InvestigationRunner] Failed to add warning message: %v\n", err)
		}
	}
}

// handleMaxActionsReached handles the scenario where max actions limit is reached.
// Sends a summary request and allows one final AI response.
func (r *InvestigationRunner) handleMaxActionsReached(rc *runContext) error {
	fmt.Fprintf(
		os.Stderr,
		"[InvestigationRunner] Max actions limit reached (%d/%d). Requesting summary.\n",
		rc.actionsTaken,
		rc.maxActions,
	)

	summaryMsg := "TURN LIMIT REACHED: You have reached the maximum number of allowed turns for this investigation. Please provide a summary of your findings and conclusions based on the investigation performed so far."
	if _, err := r.convService.AddUserMessage(rc.ctx, rc.sessionID, summaryMsg); err != nil {
		fmt.Fprintf(os.Stderr, "[InvestigationRunner] Failed to add summary request: %v\n", err)
		return err
	}

	_, _, err := r.convService.ProcessAssistantResponse(rc.ctx, rc.sessionID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[InvestigationRunner] Error processing final summary response: %v\n", err)
		return err
	}

	return nil
}

// getNextToolCalls retrieves and limits the next batch of tool calls.
// Also returns the AI message for confidence analysis.
func (r *InvestigationRunner) getNextToolCalls(rc *runContext) (*entity.Message, []port.ToolCallInfo, error) {
	msg, toolCalls, err := r.convService.ProcessAssistantResponse(rc.ctx, rc.sessionID)
	if err != nil {
		return nil, nil, err
	}
	return msg, r.limitToolCalls(rc, toolCalls), nil
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
		// Log the raw input for debugging
		inputJSON, _ := json.Marshal(separated.completion.Input)
		fmt.Fprintf(os.Stderr, "[InvestigationRunner] complete_investigation called with input: %s\n", inputJSON)

		return rc.buildCompletionResult(separated.completion.Input), true, nil
	}

	if separated.escalation != nil {
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
		ActionsTaken:    rc.actionsTaken,
		Duration:        time.Since(rc.startTime),
	}
}

func (r *InvestigationRunner) limitToolCalls(rc *runContext, toolCalls []port.ToolCallInfo) []port.ToolCallInfo {
	// First filter by allowed tools
	toolCalls = r.filterToolsByAllowedList(toolCalls)

	remaining := rc.maxActions - rc.actionsTaken
	if remaining <= 0 {
		return nil
	}
	if len(toolCalls) > remaining {
		return toolCalls[:remaining]
	}
	return toolCalls
}

// filterToolsByAllowedList filters tool calls to only include tools in the allowed list.
// If AllowedTools is nil, all tools are allowed; if empty slice, no tools are allowed.
func (r *InvestigationRunner) filterToolsByAllowedList(toolCalls []port.ToolCallInfo) []port.ToolCallInfo {
	// nil means no restriction, empty slice means block all
	if r.config.AllowedTools == nil {
		return toolCalls
	}
	if len(r.config.AllowedTools) == 0 {
		return nil
	}

	allowedSet := make(map[string]bool, len(r.config.AllowedTools))
	for _, t := range r.config.AllowedTools {
		allowedSet[t] = true
	}

	filtered := make([]port.ToolCallInfo, 0, len(toolCalls))
	for _, tc := range toolCalls {
		if allowedSet[tc.ToolName] {
			filtered = append(filtered, tc)
		}
	}
	return filtered
}

// buildTurnWarningMessage generates a warning message based on remaining actions.
// Returns empty string if no warning should be displayed.
func (r *InvestigationRunner) buildTurnWarningMessage(remaining int) string {
	if remaining == 5 {
		return `TURN LIMIT WARNING: You have 5 turns remaining before the investigation reaches its turn limit.

Please prioritize your remaining actions carefully. Consider using the batch_tool to execute multiple operations efficiently in a single turn.`
	}
	if remaining == 1 {
		return "TURN LIMIT WARNING: You have 1 turn remaining."
	}
	if remaining >= 2 && remaining <= 4 {
		return fmt.Sprintf("TURN LIMIT WARNING: You have %d turns remaining.", remaining)
	}
	return ""
}

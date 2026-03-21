package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/safety"
)

// Investigation status constants.
const (
	investigationStatusRunning   = "running"
	investigationStatusCompleted = "completed"
	investigationStatusEscalated = "escalated"
)

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

// registerInvestigationTools registers the investigation-related tools.
func (a *ExecutorAdapter) registerInvestigationTools() {
	a.registerCompleteInvestigationTool()
	a.registerEscalateInvestigationTool()
	a.registerReportInvestigationTool()
}

func (a *ExecutorAdapter) registerCompleteInvestigationTool() {
	tool := entity.Tool{
		ID:          "complete_investigation",
		Name:        "complete_investigation",
		Description: "Completes an investigation with findings and confidence level.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"investigation_id": map[string]any{
					"type":        "string",
					"description": "The ID of the investigation to complete",
				},
				"confidence": map[string]any{
					"type":        "number",
					"minimum":     safety.ConfidenceMin,
					"maximum":     safety.ConfidenceMax,
					"description": "Confidence level from 0 to 1",
				},
				"findings": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "List of findings from the investigation",
				},
				"root_cause": map[string]any{
					"type":        "string",
					"description": "The identified root cause (optional)",
				},
				"recommended_actions": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "List of recommended actions (optional)",
				},
				"severity": map[string]any{
					"type":        "string",
					"enum":        []any{"info", "warning", "error", "critical"},
					"description": "Severity level of the findings",
				},
				"summary": map[string]any{
					"type":        "string",
					"description": "Brief summary of the investigation",
				},
			},
			"required": []string{"investigation_id", "confidence", "findings"},
		},
		RequiredFields: []string{"investigation_id", "confidence", "findings"},
	}
	a.tools[tool.Name] = tool
}

func (a *ExecutorAdapter) registerEscalateInvestigationTool() {
	tool := entity.Tool{
		ID:          "escalate_investigation",
		Name:        "escalate_investigation",
		Description: "Escalates an investigation to a higher priority or human review.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"investigation_id": map[string]any{
					"type":        "string",
					"description": "The ID of the investigation to escalate",
				},
				"reason": map[string]any{
					"type":        "string",
					"description": "Reason for escalation",
				},
				"priority": map[string]any{
					"type":        "string",
					"enum":        []any{"low", "medium", "high", "critical"},
					"description": "Priority level for escalation",
				},
				"partial_findings": map[string]any{
					"type": "array",
					"items": map[string]any{
						"type": "string",
					},
					"description": "Partial findings gathered so far (optional)",
				},
				"blocking": map[string]any{
					"type":        "boolean",
					"description": "Whether this escalation is blocking",
				},
				"requires_acknowledgment": map[string]any{
					"type":        "boolean",
					"description": "Whether acknowledgment is required",
				},
			},
			"required": []string{"investigation_id", "reason", "priority"},
		},
		RequiredFields: []string{"investigation_id", "reason", "priority"},
	}
	a.tools[tool.Name] = tool
}

func (a *ExecutorAdapter) registerReportInvestigationTool() {
	tool := entity.Tool{
		ID:          "report_investigation",
		Name:        "report_investigation",
		Description: "Reports progress or status update during an ongoing investigation.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"investigation_id": map[string]any{
					"type":        "string",
					"description": "The ID of the investigation to report on",
				},
				"message": map[string]any{
					"type":        "string",
					"description": "Status message or progress update",
				},
				"progress": map[string]any{
					"type":        "number",
					"minimum":     float64(safety.ProgressMin),
					"maximum":     float64(safety.ProgressMax),
					"description": "Progress percentage from 0 to 100",
				},
			},
			"required": []string{"investigation_id", "message"},
		},
		RequiredFields: []string{"investigation_id", "message"},
	}
	a.tools[tool.Name] = tool
}

// isValidPriority checks if the given priority is a valid escalation priority.
func isValidPriority(priority string) bool {
	switch priority {
	case "low", "medium", "high", "critical":
		return true
	default:
		return false
	}
}

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

// validateInvestigationID validates that an investigation ID is non-empty.
func validateInvestigationID(id string) error {
	if id == "" || strings.TrimSpace(id) == "" {
		return errors.New("investigation_id is required and cannot be empty")
	}
	return nil
}

// requireInvestigationExists checks if an investigation exists and returns an error if not.
func (a *ExecutorAdapter) requireInvestigationExists(investigationID string) error {
	a.investigationMu.Lock()
	_, exists := a.investigationStates[investigationID]
	a.investigationMu.Unlock()
	if !exists {
		return fmt.Errorf("investigation_id %q not found", investigationID)
	}
	return nil
}

// marshalInvestigationOutput marshals the output map to JSON, optionally adding investigation_id.
func marshalInvestigationOutput(output map[string]any, investigationID string) (string, error) {
	if investigationID != "" {
		output["investigation_id"] = investigationID
	}
	result, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}
	return string(result), nil
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

	if err := validateInvestigationID(in.InvestigationID); err != nil {
		return "", err
	}

	if err := a.requireInvestigationExists(in.InvestigationID); err != nil {
		return "", err
	}

	// Validate confidence
	if in.Confidence == nil {
		return "", errors.New("confidence is required")
	}
	if *in.Confidence < safety.ConfidenceMin || *in.Confidence > safety.ConfidenceMax {
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
	output := map[string]any{
		"status":       investigationStatusCompleted,
		"confidence":   *in.Confidence,
		"findings":     in.Findings,
		"completed_at": time.Now().UTC().Format(time.RFC3339),
	}

	return marshalInvestigationOutput(output, in.InvestigationID)
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

	if err := validateInvestigationID(in.InvestigationID); err != nil {
		return "", err
	}

	if err := a.requireInvestigationExists(in.InvestigationID); err != nil {
		return "", err
	}

	// Validate reason
	if in.Reason == "" || strings.TrimSpace(in.Reason) == "" {
		return "", errors.New("reason is required and cannot be empty")
	}

	// Validate priority
	if !isValidPriority(in.Priority) {
		return "", errors.New("priority must be one of: low, medium, high, critical")
	}

	// Check for duplicate escalation (only if investigation_id provided)
	if err := a.checkAndSetInvestigationStatus(in.InvestigationID, investigationStatusEscalated); err != nil {
		return "", err
	}

	// Build output
	escalationID := fmt.Sprintf("esc-%s-%d", in.InvestigationID, time.Now().UnixNano())
	output := map[string]any{
		"status":        investigationStatusEscalated,
		"escalation_id": escalationID,
		"reason":        in.Reason,
		"priority":      in.Priority,
		"escalated_at":  time.Now().UTC().Format(time.RFC3339),
	}

	return marshalInvestigationOutput(output, in.InvestigationID)
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

	if err := validateInvestigationID(in.InvestigationID); err != nil {
		return "", err
	}

	if err := a.requireInvestigationExists(in.InvestigationID); err != nil {
		return "", err
	}

	// Validate message
	if in.Message == "" {
		return "", errors.New("message is required")
	}

	// Validate progress if provided
	if in.Progress != nil {
		if *in.Progress < float64(safety.ProgressMin) || *in.Progress > float64(safety.ProgressMax) {
			return "", errors.New("progress must be between 0 and 100")
		}
	}

	// Build output
	output := map[string]any{
		"status":           "reported",
		"investigation_id": in.InvestigationID,
		"message":          in.Message,
		"reported_at":      time.Now().UTC().Format(time.RFC3339),
	}
	if in.Progress != nil {
		output["progress"] = *in.Progress
	}

	return marshalInvestigationOutput(output, "")
}

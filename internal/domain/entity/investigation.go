// Package entity contains the core domain entities for the code-editing-agent.
package entity

import (
	"errors"
	"strings"
	"time"
)

// Investigation status constants define the lifecycle states of an investigation.
// An investigation progresses through these states:
//   - started: Initial state when investigation is created
//   - running: Investigation is actively gathering information
//   - completed: Investigation finished successfully with findings
//   - failed: Investigation encountered an unrecoverable error
//   - escalated: Investigation requires human intervention
const (
	InvestigationStatusStarted   = "started"
	InvestigationStatusRunning   = "running"
	InvestigationStatusCompleted = "completed"
	InvestigationStatusFailed    = "failed"
	InvestigationStatusEscalated = "escalated"
)

// Sentinel errors for Investigation validation.
// These errors are returned when validation fails during investigation operations.
var (
	// ErrEmptyInvestigationID is returned when an investigation ID is empty or whitespace-only.
	ErrEmptyInvestigationID = errors.New("investigation ID cannot be empty")
	// ErrEmptySessionID is returned when a session ID is empty or whitespace-only.
	ErrEmptySessionID = errors.New("session ID cannot be empty")
	// ErrInvalidStatus is returned when attempting to set an unrecognized status value.
	ErrInvalidStatus = errors.New("invalid investigation status")
	// ErrInvalidConfidence is returned when confidence is outside the valid range [0.0, 1.0].
	ErrInvalidConfidence = errors.New("confidence must be between 0.0 and 1.0")
)

// InvestigationFinding represents a discovery made during an investigation.
// Findings capture important observations, potential root causes, or diagnostic
// information gathered during the investigation process.
type InvestigationFinding struct {
	// Type categorizes the finding (e.g., "observation", "failure", "escalation").
	Type string
	// Description provides a human-readable explanation of the finding.
	Description string
	// Severity indicates the importance level (e.g., "info", "warning", "error", "high").
	Severity string
	// Timestamp records when this finding was discovered.
	Timestamp time.Time
}

// InvestigationAction represents an action taken during an investigation.
// Actions track the tools executed and their results, providing an audit trail
// of the investigation process.
type InvestigationAction struct {
	// ToolName is the identifier of the tool that was executed.
	ToolName string
	// Input contains the parameters passed to the tool.
	Input map[string]interface{}
	// Output holds the result or response from the tool execution.
	Output string
	// Timestamp records when this action was executed.
	Timestamp time.Time
	// Duration tracks how long the action took to complete.
	Duration time.Duration
}

// Investigation represents an ongoing or completed investigation of an alert.
// It tracks the full lifecycle from start to completion, including all findings
// and actions taken. Investigation is designed to be immutable after creation,
// with state changes managed through dedicated methods.
type Investigation struct {
	id          string                 // Unique identifier for this investigation
	alertID     string                 // ID of the alert being investigated
	sessionID   string                 // Session context for the investigation
	status      string                 // Current lifecycle status
	findings    []InvestigationFinding // Discoveries made during investigation
	actions     []InvestigationAction  // Tools executed during investigation
	confidence  float64                // Confidence level in the investigation outcome [0.0, 1.0]
	isEscalated bool                   // Whether investigation was escalated to humans
	startedAt   time.Time              // When the investigation began
	completedAt time.Time              // When the investigation finished (zero if ongoing)
}

// NewInvestigation creates a new Investigation with the required fields.
// All string parameters are trimmed of leading/trailing whitespace before validation.
// The investigation starts in the "started" status with the current timestamp.
//
// Returns an error if:
//   - id is empty or whitespace-only (ErrEmptyInvestigationID)
//   - alertID is empty or whitespace-only (ErrEmptyAlertID)
//   - sessionID is empty or whitespace-only (ErrEmptySessionID)
func NewInvestigation(id, alertID, sessionID string) (*Investigation, error) {
	id = strings.TrimSpace(id)
	alertID = strings.TrimSpace(alertID)
	sessionID = strings.TrimSpace(sessionID)

	if id == "" {
		return nil, ErrEmptyInvestigationID
	}
	if alertID == "" {
		return nil, ErrEmptyAlertID
	}
	if sessionID == "" {
		return nil, ErrEmptySessionID
	}

	return &Investigation{
		id:        id,
		alertID:   alertID,
		sessionID: sessionID,
		status:    InvestigationStatusStarted,
		findings:  []InvestigationFinding{},
		actions:   []InvestigationAction{},
		startedAt: time.Now(),
	}, nil
}

// ID returns the unique investigation identifier.
func (i *Investigation) ID() string { return i.id }

// AlertID returns the ID of the alert being investigated.
func (i *Investigation) AlertID() string { return i.alertID }

// SessionID returns the session context for this investigation.
func (i *Investigation) SessionID() string { return i.sessionID }

// Status returns the current lifecycle status of the investigation.
// See InvestigationStatus* constants for valid values.
func (i *Investigation) Status() string { return i.status }

// Findings returns the list of discoveries made during the investigation.
// The returned slice should be treated as read-only.
func (i *Investigation) Findings() []InvestigationFinding { return i.findings }

// Actions returns the list of tool executions performed during the investigation.
// The returned slice should be treated as read-only.
func (i *Investigation) Actions() []InvestigationAction { return i.actions }

// ActionCount returns the total number of actions taken during the investigation.
// This is useful for enforcing action budget limits.
func (i *Investigation) ActionCount() int { return len(i.actions) }

// Confidence returns the confidence level in the investigation outcome.
// The value ranges from 0.0 (no confidence) to 1.0 (full confidence).
func (i *Investigation) Confidence() float64 { return i.confidence }

// IsEscalated returns true if the investigation was escalated to a human operator.
func (i *Investigation) IsEscalated() bool { return i.isEscalated }

// StartedAt returns the timestamp when the investigation was created.
func (i *Investigation) StartedAt() time.Time { return i.startedAt }

// CompletedAt returns the timestamp when the investigation finished.
// Returns a zero time if the investigation is still in progress.
func (i *Investigation) CompletedAt() time.Time { return i.completedAt }

// Duration returns the elapsed time of the investigation.
// For ongoing investigations, returns time since start.
// For completed investigations, returns the total investigation time.
func (i *Investigation) Duration() time.Duration {
	if i.completedAt.IsZero() {
		return time.Since(i.startedAt)
	}
	return i.completedAt.Sub(i.startedAt)
}

// IsComplete returns true if the investigation has reached a terminal state.
// Terminal states are: completed, failed, or escalated.
func (i *Investigation) IsComplete() bool {
	switch i.status {
	case InvestigationStatusCompleted, InvestigationStatusFailed, InvestigationStatusEscalated:
		return true
	}
	return false
}

// SetStatus updates the investigation status.
// Returns ErrInvalidStatus if the provided status is not a valid InvestigationStatus* constant.
func (i *Investigation) SetStatus(status string) error {
	if !isValidInvestigationStatus(status) {
		return ErrInvalidStatus
	}
	i.status = status
	return nil
}

// AddFinding appends a new finding to the investigation's findings list.
// Findings are not validated; callers should ensure finding fields are populated.
func (i *Investigation) AddFinding(finding InvestigationFinding) {
	i.findings = append(i.findings, finding)
}

// AddAction appends a new action to the investigation's action history.
// Actions provide an audit trail of all tool executions performed.
func (i *Investigation) AddAction(action InvestigationAction) {
	i.actions = append(i.actions, action)
}

// SetConfidence updates the confidence level for the investigation outcome.
// Returns ErrInvalidConfidence if the value is outside the range [0.0, 1.0].
func (i *Investigation) SetConfidence(confidence float64) error {
	if confidence < 0.0 || confidence > 1.0 {
		return ErrInvalidConfidence
	}
	i.confidence = confidence
	return nil
}

// Complete marks the investigation as successfully completed.
// Sets the status to "completed" and records the current time as completion time.
func (i *Investigation) Complete() {
	i.status = InvestigationStatusCompleted
	i.completedAt = time.Now()
}

// Fail marks the investigation as failed and records the reason.
// Sets the status to "failed", records completion time, and adds a failure finding.
func (i *Investigation) Fail(reason string) {
	i.status = InvestigationStatusFailed
	i.completedAt = time.Now()
	i.AddFinding(InvestigationFinding{
		Type:        "failure",
		Description: reason,
		Severity:    "error",
		Timestamp:   time.Now(),
	})
}

// Escalate marks the investigation as requiring human intervention.
// Sets the status to "escalated", marks the escalation flag, records completion time,
// and adds an escalation finding with high severity.
func (i *Investigation) Escalate(reason string) {
	i.status = InvestigationStatusEscalated
	i.isEscalated = true
	i.completedAt = time.Now()
	i.AddFinding(InvestigationFinding{
		Type:        "escalation",
		Description: reason,
		Severity:    "high",
		Timestamp:   time.Now(),
	})
}

// isValidInvestigationStatus checks if the given string is a recognized status constant.
func isValidInvestigationStatus(s string) bool {
	switch s {
	case InvestigationStatusStarted,
		InvestigationStatusRunning,
		InvestigationStatusCompleted,
		InvestigationStatusFailed,
		InvestigationStatusEscalated:
		return true
	}
	return false
}

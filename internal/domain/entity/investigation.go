// Package entity contains the core domain entities for the code-editing-agent.
package entity

import (
	"errors"
	"strings"
	"time"
)

// Investigation status constants.
const (
	InvestigationStatusStarted   = "started"
	InvestigationStatusRunning   = "running"
	InvestigationStatusCompleted = "completed"
	InvestigationStatusFailed    = "failed"
	InvestigationStatusEscalated = "escalated"
)

// Sentinel errors for Investigation validation.
var (
	ErrEmptyInvestigationID = errors.New("investigation ID cannot be empty")
	ErrEmptySessionID       = errors.New("session ID cannot be empty")
	ErrInvalidStatus        = errors.New("invalid investigation status")
	ErrInvalidConfidence    = errors.New("confidence must be between 0.0 and 1.0")
)

// InvestigationFinding represents a finding from the investigation.
type InvestigationFinding struct {
	Type        string
	Description string
	Severity    string
	Timestamp   time.Time
}

// InvestigationAction represents an action taken during investigation.
type InvestigationAction struct {
	ToolName  string
	Input     map[string]interface{}
	Output    string
	Timestamp time.Time
	Duration  time.Duration
}

// Investigation represents an ongoing or completed investigation of an alert.
type Investigation struct {
	id          string
	alertID     string
	sessionID   string
	status      string
	findings    []InvestigationFinding
	actions     []InvestigationAction
	confidence  float64
	isEscalated bool
	startedAt   time.Time
	completedAt time.Time
}

// NewInvestigation creates a new Investigation with the required fields.
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

// ID returns the investigation identifier.
func (i *Investigation) ID() string { return i.id }

// AlertID returns the associated alert ID.
func (i *Investigation) AlertID() string { return i.alertID }

// SessionID returns the session ID.
func (i *Investigation) SessionID() string { return i.sessionID }

// Status returns the current status.
func (i *Investigation) Status() string { return i.status }

// Findings returns the list of findings.
func (i *Investigation) Findings() []InvestigationFinding { return i.findings }

// Actions returns the list of actions taken.
func (i *Investigation) Actions() []InvestigationAction { return i.actions }

// ActionCount returns the number of actions taken.
func (i *Investigation) ActionCount() int { return len(i.actions) }

// Confidence returns the confidence level.
func (i *Investigation) Confidence() float64 { return i.confidence }

// IsEscalated returns whether the investigation was escalated.
func (i *Investigation) IsEscalated() bool { return i.isEscalated }

// StartedAt returns when the investigation started.
func (i *Investigation) StartedAt() time.Time { return i.startedAt }

// CompletedAt returns when the investigation completed.
func (i *Investigation) CompletedAt() time.Time { return i.completedAt }

// Duration returns how long the investigation has been running.
func (i *Investigation) Duration() time.Duration {
	if i.completedAt.IsZero() {
		return time.Since(i.startedAt)
	}
	return i.completedAt.Sub(i.startedAt)
}

// IsComplete returns true if the investigation has reached a terminal state.
func (i *Investigation) IsComplete() bool {
	switch i.status {
	case InvestigationStatusCompleted, InvestigationStatusFailed, InvestigationStatusEscalated:
		return true
	}
	return false
}

// SetStatus sets the investigation status.
func (i *Investigation) SetStatus(status string) error {
	if !isValidInvestigationStatus(status) {
		return ErrInvalidStatus
	}
	i.status = status
	return nil
}

// AddFinding adds a finding to the investigation.
func (i *Investigation) AddFinding(finding InvestigationFinding) {
	i.findings = append(i.findings, finding)
}

// AddAction adds an action to the investigation.
func (i *Investigation) AddAction(action InvestigationAction) {
	i.actions = append(i.actions, action)
}

// SetConfidence sets the confidence level.
func (i *Investigation) SetConfidence(confidence float64) error {
	if confidence < 0.0 || confidence > 1.0 {
		return ErrInvalidConfidence
	}
	i.confidence = confidence
	return nil
}

// Complete marks the investigation as completed.
func (i *Investigation) Complete() {
	i.status = InvestigationStatusCompleted
	i.completedAt = time.Now()
}

// Fail marks the investigation as failed with a reason.
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

// Escalate marks the investigation as escalated with a reason.
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

// isValidInvestigationStatus checks if the given string is a valid status.
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

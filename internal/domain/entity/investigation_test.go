package entity

import (
	"errors"
	"testing"
	"time"
)

// =============================================================================
// Investigation Entity Tests - RED PHASE
// These tests define the expected behavior of the Investigation entity.
// All tests should FAIL until the implementation is complete.
// =============================================================================

// Sentinel errors expected to be defined in investigation.go.
var (
	ErrEmptyInvestigationID = errors.New("investigation ID cannot be empty")
	ErrEmptySessionID       = errors.New("session ID cannot be empty")
	ErrInvalidStatus        = errors.New("invalid investigation status")
	ErrInvalidConfidence    = errors.New("confidence must be between 0.0 and 1.0")
)

// Investigation status constants expected to be defined in investigation.go.
const (
	InvestigationStatusStarted   = "started"
	InvestigationStatusRunning   = "running"
	InvestigationStatusCompleted = "completed"
	InvestigationStatusFailed    = "failed"
	InvestigationStatusEscalated = "escalated"
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
type Investigation struct{}

// Stub function - to be implemented.
func NewInvestigation(_, _, _ string) (*Investigation, error) {
	return nil, errors.New("not implemented")
}

// Stub methods - to be implemented.
func (i *Investigation) ID() string                       { return "" }
func (i *Investigation) AlertID() string                  { return "" }
func (i *Investigation) SessionID() string                { return "" }
func (i *Investigation) Status() string                   { return "" }
func (i *Investigation) Findings() []InvestigationFinding { return nil }
func (i *Investigation) Actions() []InvestigationAction   { return nil }
func (i *Investigation) ActionCount() int                 { return 0 }
func (i *Investigation) Confidence() float64              { return 0 }
func (i *Investigation) IsEscalated() bool                { return false }
func (i *Investigation) StartedAt() time.Time             { return time.Time{} }
func (i *Investigation) CompletedAt() time.Time           { return time.Time{} }
func (i *Investigation) Duration() time.Duration          { return 0 }
func (i *Investigation) IsComplete() bool                 { return false }
func (i *Investigation) SetStatus(_ string) error         { return errors.New("not implemented") }
func (i *Investigation) AddFinding(_ InvestigationFinding) {
}
func (i *Investigation) AddAction(_ InvestigationAction) {}
func (i *Investigation) SetConfidence(_ float64) error   { return errors.New("not implemented") }
func (i *Investigation) Complete()                       {}
func (i *Investigation) Fail(_ string)                   {}
func (i *Investigation) Escalate(_ string)               {}

func TestNewInvestigation_WithValidFields(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv == nil {
		t.Error("NewInvestigation() returned nil without error")
	}
}

func TestNewInvestigation_WithEmptyID(t *testing.T) {
	_, err := NewInvestigation("", "alert-001", "session-001")
	if !errors.Is(err, ErrEmptyInvestigationID) {
		t.Errorf("NewInvestigation() error = %v, want %v", err, ErrEmptyInvestigationID)
	}
}

func TestNewInvestigation_WithWhitespaceID(t *testing.T) {
	_, err := NewInvestigation("   ", "alert-001", "session-001")
	if !errors.Is(err, ErrEmptyInvestigationID) {
		t.Errorf("NewInvestigation() error = %v, want %v", err, ErrEmptyInvestigationID)
	}
}

func TestNewInvestigation_WithEmptyAlertID(t *testing.T) {
	_, err := NewInvestigation("inv-001", "", "session-001")
	if !errors.Is(err, ErrEmptyAlertID) {
		t.Errorf("NewInvestigation() error = %v, want %v", err, ErrEmptyAlertID)
	}
}

func TestNewInvestigation_WithWhitespaceAlertID(t *testing.T) {
	_, err := NewInvestigation("inv-001", "  \t  ", "session-001")
	if !errors.Is(err, ErrEmptyAlertID) {
		t.Errorf("NewInvestigation() error = %v, want %v", err, ErrEmptyAlertID)
	}
}

func TestNewInvestigation_WithEmptySessionID(t *testing.T) {
	_, err := NewInvestigation("inv-001", "alert-001", "")
	if !errors.Is(err, ErrEmptySessionID) {
		t.Errorf("NewInvestigation() error = %v, want %v", err, ErrEmptySessionID)
	}
}

func TestNewInvestigation_WithWhitespaceSessionID(t *testing.T) {
	_, err := NewInvestigation("inv-001", "alert-001", "   ")
	if !errors.Is(err, ErrEmptySessionID) {
		t.Errorf("NewInvestigation() error = %v, want %v", err, ErrEmptySessionID)
	}
}

func TestInvestigation_InitialStatusIsStarted(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv.Status() != InvestigationStatusStarted {
		t.Errorf("Status() = %v, want %v", inv.Status(), InvestigationStatusStarted)
	}
}

func TestInvestigation_InitialFindingsEmpty(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv.Findings() == nil {
		t.Error("Findings() should not be nil")
	}
	if len(inv.Findings()) != 0 {
		t.Errorf("Findings() should be empty, got %v items", len(inv.Findings()))
	}
}

func TestInvestigation_InitialActionsEmpty(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv.Actions() == nil {
		t.Error("Actions() should not be nil")
	}
	if len(inv.Actions()) != 0 {
		t.Errorf("Actions() should be empty, got %v items", len(inv.Actions()))
	}
}

func TestInvestigation_InitialActionCountZero(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv.ActionCount() != 0 {
		t.Errorf("ActionCount() = %v, want 0", inv.ActionCount())
	}
}

func TestInvestigation_InitialConfidenceZero(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv.Confidence() != 0.0 {
		t.Errorf("Confidence() = %v, want 0.0", inv.Confidence())
	}
}

func TestInvestigation_InitiallyNotEscalated(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv.IsEscalated() {
		t.Error("IsEscalated() should be false initially")
	}
}

func TestInvestigation_StartedAtSetOnCreation(t *testing.T) {
	before := time.Now()
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	after := time.Now()

	startedAt := inv.StartedAt()
	if startedAt.Before(before) || startedAt.After(after) {
		t.Errorf("StartedAt() = %v, should be between %v and %v", startedAt, before, after)
	}
}

func TestInvestigation_CompletedAtZeroInitially(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if !inv.CompletedAt().IsZero() {
		t.Errorf("CompletedAt() = %v, should be zero", inv.CompletedAt())
	}
}

func TestInvestigation_SetStatusToRunning(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	err = inv.SetStatus(InvestigationStatusRunning)
	if err != nil {
		t.Errorf("SetStatus() error = %v", err)
	}
	if inv.Status() != InvestigationStatusRunning {
		t.Errorf("Status() = %v, want %v", inv.Status(), InvestigationStatusRunning)
	}
}

func TestInvestigation_SetStatusToCompleted(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	err = inv.SetStatus(InvestigationStatusCompleted)
	if err != nil {
		t.Errorf("SetStatus() error = %v", err)
	}
	if inv.Status() != InvestigationStatusCompleted {
		t.Errorf("Status() = %v, want %v", inv.Status(), InvestigationStatusCompleted)
	}
}

func TestInvestigation_SetStatusToInvalid(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	err = inv.SetStatus("invalid")
	if err == nil {
		t.Error("SetStatus() should return error for invalid status")
	}
}

func TestInvestigation_SetStatusToEmpty(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	err = inv.SetStatus("")
	if err == nil {
		t.Error("SetStatus() should return error for empty status")
	}
}

func TestInvestigation_AddFinding(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}

	finding := InvestigationFinding{
		Type:        "observation",
		Description: "High CPU usage detected",
		Severity:    "warning",
		Timestamp:   time.Now(),
	}

	inv.AddFinding(finding)

	findings := inv.Findings()
	if len(findings) != 1 {
		t.Errorf("Findings() len = %v, want 1", len(findings))
	}
}

func TestInvestigation_AddMultipleFindings(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}

	inv.AddFinding(InvestigationFinding{Type: "observation", Description: "First"})
	inv.AddFinding(InvestigationFinding{Type: "conclusion", Description: "Second"})
	inv.AddFinding(InvestigationFinding{Type: "recommendation", Description: "Third"})

	if len(inv.Findings()) != 3 {
		t.Errorf("Findings() len = %v, want 3", len(inv.Findings()))
	}
}

func TestInvestigation_AddAction(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}

	action := InvestigationAction{
		ToolName:  "bash",
		Input:     map[string]interface{}{"command": "top -b -n 1"},
		Output:    "CPU output here...",
		Timestamp: time.Now(),
		Duration:  500 * time.Millisecond,
	}

	inv.AddAction(action)

	if len(inv.Actions()) != 1 {
		t.Errorf("Actions() len = %v, want 1", len(inv.Actions()))
	}
}

func TestInvestigation_AddActionIncrementsCount(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}

	if inv.ActionCount() != 0 {
		t.Errorf("Initial ActionCount() = %v, want 0", inv.ActionCount())
	}

	inv.AddAction(InvestigationAction{ToolName: "bash"})
	if inv.ActionCount() != 1 {
		t.Errorf("ActionCount() after first = %v, want 1", inv.ActionCount())
	}

	inv.AddAction(InvestigationAction{ToolName: "read_file"})
	if inv.ActionCount() != 2 {
		t.Errorf("ActionCount() after second = %v, want 2", inv.ActionCount())
	}
}

func TestInvestigation_IsCompleteForStarted(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv.IsComplete() {
		t.Error("IsComplete() should be false for started status")
	}
}

func TestInvestigation_IsCompleteForRunning(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	_ = inv.SetStatus(InvestigationStatusRunning)
	if inv.IsComplete() {
		t.Error("IsComplete() should be false for running status")
	}
}

func TestInvestigation_IsCompleteForCompleted(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	_ = inv.SetStatus(InvestigationStatusCompleted)
	if !inv.IsComplete() {
		t.Error("IsComplete() should be true for completed status")
	}
}

func TestInvestigation_IsCompleteForFailed(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	_ = inv.SetStatus(InvestigationStatusFailed)
	if !inv.IsComplete() {
		t.Error("IsComplete() should be true for failed status")
	}
}

func TestInvestigation_IsCompleteForEscalated(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	_ = inv.SetStatus(InvestigationStatusEscalated)
	if !inv.IsComplete() {
		t.Error("IsComplete() should be true for escalated status")
	}
}

func TestInvestigation_Duration(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	time.Sleep(10 * time.Millisecond)
	if inv.Duration() <= 0 {
		t.Errorf("Duration() = %v, want positive", inv.Duration())
	}
}

func TestInvestigation_CompleteSetsStatus(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	inv.Complete()
	if inv.Status() != InvestigationStatusCompleted {
		t.Errorf("Status() = %v, want %v", inv.Status(), InvestigationStatusCompleted)
	}
}

func TestInvestigation_CompleteSetsCompletedAt(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	before := time.Now()
	inv.Complete()
	after := time.Now()

	completedAt := inv.CompletedAt()
	if completedAt.IsZero() {
		t.Error("CompletedAt() should not be zero after Complete()")
	}
	if completedAt.Before(before) || completedAt.After(after) {
		t.Errorf("CompletedAt() = %v, should be between %v and %v", completedAt, before, after)
	}
}

func TestInvestigation_FailSetsStatus(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	inv.Fail("Unable to access resources")
	if inv.Status() != InvestigationStatusFailed {
		t.Errorf("Status() = %v, want %v", inv.Status(), InvestigationStatusFailed)
	}
}

func TestInvestigation_FailAddsReasonFinding(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	inv.Fail("Connection timeout")
	if len(inv.Findings()) == 0 {
		t.Error("Fail() should add a finding with the reason")
	}
}

func TestInvestigation_FailSetsCompletedAt(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	inv.Fail("Some failure")
	if inv.CompletedAt().IsZero() {
		t.Error("CompletedAt() should not be zero after Fail()")
	}
}

func TestInvestigation_EscalateSetsStatus(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	inv.Escalate("Requires human intervention")
	if inv.Status() != InvestigationStatusEscalated {
		t.Errorf("Status() = %v, want %v", inv.Status(), InvestigationStatusEscalated)
	}
}

func TestInvestigation_EscalateSetsFlag(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv.IsEscalated() {
		t.Error("IsEscalated() should be false before Escalate()")
	}
	inv.Escalate("Complex issue")
	if !inv.IsEscalated() {
		t.Error("IsEscalated() should be true after Escalate()")
	}
}

func TestInvestigation_EscalateAddsReasonFinding(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	inv.Escalate("Potential security breach")
	if len(inv.Findings()) == 0 {
		t.Error("Escalate() should add a finding with the reason")
	}
}

func TestInvestigation_SetConfidenceValid(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	err = inv.SetConfidence(0.85)
	if err != nil {
		t.Errorf("SetConfidence(0.85) error = %v", err)
	}
	if inv.Confidence() != 0.85 {
		t.Errorf("Confidence() = %v, want 0.85", inv.Confidence())
	}
}

func TestInvestigation_SetConfidenceZero(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	err = inv.SetConfidence(0.0)
	if err != nil {
		t.Errorf("SetConfidence(0.0) error = %v", err)
	}
}

func TestInvestigation_SetConfidenceOne(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	err = inv.SetConfidence(1.0)
	if err != nil {
		t.Errorf("SetConfidence(1.0) error = %v", err)
	}
}

func TestInvestigation_SetConfidenceNegative(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	err = inv.SetConfidence(-0.1)
	if err == nil {
		t.Error("SetConfidence(-0.1) should return error")
	}
}

func TestInvestigation_SetConfidenceGreaterThanOne(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	err = inv.SetConfidence(1.1)
	if err == nil {
		t.Error("SetConfidence(1.1) should return error")
	}
}

func TestInvestigation_GetterID(t *testing.T) {
	inv, err := NewInvestigation("inv-getter", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv.ID() != "inv-getter" {
		t.Errorf("ID() = %v, want inv-getter", inv.ID())
	}
}

func TestInvestigation_GetterAlertID(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-getter", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv.AlertID() != "alert-getter" {
		t.Errorf("AlertID() = %v, want alert-getter", inv.AlertID())
	}
}

func TestInvestigation_GetterSessionID(t *testing.T) {
	inv, err := NewInvestigation("inv-001", "alert-001", "session-getter")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv.SessionID() != "session-getter" {
		t.Errorf("SessionID() = %v, want session-getter", inv.SessionID())
	}
}

func TestInvestigationErrors_NotNil(t *testing.T) {
	if ErrEmptyInvestigationID == nil {
		t.Error("ErrEmptyInvestigationID should not be nil")
	}
	if ErrEmptySessionID == nil {
		t.Error("ErrEmptySessionID should not be nil")
	}
	if ErrInvalidStatus == nil {
		t.Error("ErrInvalidStatus should not be nil")
	}
	if ErrInvalidConfidence == nil {
		t.Error("ErrInvalidConfidence should not be nil")
	}
}

func TestInvestigationErrors_HaveMessages(t *testing.T) {
	if ErrEmptyInvestigationID.Error() == "" {
		t.Error("ErrEmptyInvestigationID should have a message")
	}
	if ErrEmptySessionID.Error() == "" {
		t.Error("ErrEmptySessionID should have a message")
	}
	if ErrInvalidStatus.Error() == "" {
		t.Error("ErrInvalidStatus should have a message")
	}
	if ErrInvalidConfidence.Error() == "" {
		t.Error("ErrInvalidConfidence should have a message")
	}
}

func TestInvestigationStatusConstants(t *testing.T) {
	if InvestigationStatusStarted != "started" {
		t.Errorf("InvestigationStatusStarted = %v, want started", InvestigationStatusStarted)
	}
	if InvestigationStatusRunning != "running" {
		t.Errorf("InvestigationStatusRunning = %v, want running", InvestigationStatusRunning)
	}
	if InvestigationStatusCompleted != "completed" {
		t.Errorf("InvestigationStatusCompleted = %v, want completed", InvestigationStatusCompleted)
	}
	if InvestigationStatusFailed != "failed" {
		t.Errorf("InvestigationStatusFailed = %v, want failed", InvestigationStatusFailed)
	}
	if InvestigationStatusEscalated != "escalated" {
		t.Errorf("InvestigationStatusEscalated = %v, want escalated", InvestigationStatusEscalated)
	}
}

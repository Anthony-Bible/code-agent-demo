package usecase

import (
	"context"
	"errors"
	"testing"
	"time"
)

// =============================================================================
// EscalationHandler Tests
// These tests verify the behavior of EscalationHandler implementations.
// =============================================================================

// =============================================================================
// LogEscalationHandler Tests
// =============================================================================

func TestNewLogEscalationHandler_NotNil(t *testing.T) {
	handler := NewLogEscalationHandler()
	if handler == nil {
		t.Error("NewLogEscalationHandler() should not return nil")
	}
}

func TestLogEscalationHandler_Escalate_Success(t *testing.T) {
	handler := NewLogEscalationHandler()
	if handler == nil {
		t.Skip("NewLogEscalationHandler() returned nil")
	}

	inv := &EscalationInvestigationView{
		id:        "inv-001",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "running",
		findings:  []string{"High CPU detected", "Process X consuming 90%"},
	}

	req := EscalationRequest{
		Investigation: inv,
		Reason:        "Unable to determine root cause",
		Priority:      EscalationPriorityMedium,
	}

	result, err := handler.Escalate(context.Background(), req)
	if err != nil {
		t.Errorf("Escalate() error = %v", err)
	}
	if result == nil {
		t.Error("Escalate() returned nil result")
	}
	if result != nil && !result.Success {
		t.Error("Escalate() result.Success = false, want true")
	}
}

func TestLogEscalationHandler_Escalate_NilInvestigation(t *testing.T) {
	handler := NewLogEscalationHandler()
	if handler == nil {
		t.Skip("NewLogEscalationHandler() returned nil")
	}

	req := EscalationRequest{
		Investigation: nil,
		Reason:        "Test reason",
		Priority:      EscalationPriorityLow,
	}

	_, err := handler.Escalate(context.Background(), req)
	if err == nil {
		t.Error("Escalate() with nil investigation should return error")
	}
}

func TestLogEscalationHandler_Escalate_SetsTimestamp(t *testing.T) {
	handler := NewLogEscalationHandler()
	if handler == nil {
		t.Skip("NewLogEscalationHandler() returned nil")
	}

	inv := &EscalationInvestigationView{
		id:        "inv-002",
		alertID:   "alert-002",
		sessionID: "session-002",
		status:    "running",
	}

	req := EscalationRequest{
		Investigation: inv,
		Reason:        "Test escalation",
		Priority:      EscalationPriorityHigh,
	}

	before := time.Now()
	result, err := handler.Escalate(context.Background(), req)
	after := time.Now()

	if err != nil {
		t.Fatalf("Escalate() error = %v", err)
	}

	if result.EscalatedAt.Before(before) || result.EscalatedAt.After(after) {
		t.Errorf("EscalatedAt = %v, should be between %v and %v", result.EscalatedAt, before, after)
	}
}

func TestLogEscalationHandler_CanEscalate_NotEscalated(t *testing.T) {
	handler := NewLogEscalationHandler()
	if handler == nil {
		t.Skip("NewLogEscalationHandler() returned nil")
	}

	inv := &EscalationInvestigationView{
		id:          "inv-003",
		isEscalated: false,
	}

	if !handler.CanEscalate(inv) {
		t.Error("CanEscalate() = false, want true for non-escalated investigation")
	}
}

func TestLogEscalationHandler_CanEscalate_AlreadyEscalated(t *testing.T) {
	handler := NewLogEscalationHandler()
	if handler == nil {
		t.Skip("NewLogEscalationHandler() returned nil")
	}

	inv := &EscalationInvestigationView{
		id:          "inv-004",
		isEscalated: true,
	}

	if handler.CanEscalate(inv) {
		t.Error("CanEscalate() = true, want false for already escalated investigation")
	}
}

func TestLogEscalationHandler_CanEscalate_NilInvestigation(t *testing.T) {
	handler := NewLogEscalationHandler()
	if handler == nil {
		t.Skip("NewLogEscalationHandler() returned nil")
	}

	if handler.CanEscalate(nil) {
		t.Error("CanEscalate(nil) = true, want false")
	}
}

func TestLogEscalationHandler_GetEscalationHistory_Empty(t *testing.T) {
	handler := NewLogEscalationHandler()
	if handler == nil {
		t.Skip("NewLogEscalationHandler() returned nil")
	}

	history := handler.GetEscalationHistory("inv-nonexistent")
	if history == nil {
		t.Error("GetEscalationHistory() should return empty slice, not nil")
	}
	if len(history) != 0 {
		t.Errorf("GetEscalationHistory() len = %v, want 0", len(history))
	}
}

func TestLogEscalationHandler_GetEscalationHistory_AfterEscalation(t *testing.T) {
	handler := NewLogEscalationHandler()
	if handler == nil {
		t.Skip("NewLogEscalationHandler() returned nil")
	}

	inv := &EscalationInvestigationView{
		id:        "inv-history-test",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "running",
	}

	req := EscalationRequest{
		Investigation: inv,
		Reason:        "Test escalation",
		Priority:      EscalationPriorityMedium,
	}

	_, err := handler.Escalate(context.Background(), req)
	if err != nil {
		t.Fatalf("Escalate() error = %v", err)
	}

	history := handler.GetEscalationHistory("inv-history-test")
	if len(history) != 1 {
		t.Errorf("GetEscalationHistory() len = %v, want 1", len(history))
	}
}

// =============================================================================
// EscalationRequest Tests
// =============================================================================

func TestEscalationRequest_Priority_Validation(t *testing.T) {
	validPriorities := []EscalationPriority{
		EscalationPriorityLow,
		EscalationPriorityMedium,
		EscalationPriorityHigh,
		EscalationPriorityCritical,
	}

	for _, priority := range validPriorities {
		t.Run(string(priority), func(t *testing.T) {
			req := EscalationRequest{
				Priority: priority,
			}
			if req.Priority != priority {
				t.Errorf("Priority = %v, want %v", req.Priority, priority)
			}
		})
	}
}

func TestEscalationRequest_Context_Data(t *testing.T) {
	req := EscalationRequest{
		Investigation: &EscalationInvestigationView{id: "inv-001"},
		Reason:        "Test reason",
		Priority:      EscalationPriorityMedium,
		Context: map[string]string{
			"user":   "operator@example.com",
			"action": "investigating_cpu",
		},
	}

	if req.Context["user"] != "operator@example.com" {
		t.Error("Context should contain user data")
	}
	if req.Context["action"] != "investigating_cpu" {
		t.Error("Context should contain action data")
	}
}

// =============================================================================
// EscalationResult Tests
// =============================================================================

func TestEscalationResult_Success(t *testing.T) {
	result := EscalationResult{
		Success:     true,
		EscalatedAt: time.Now(),
		Target:      "conversation-session-123",
		MessageID:   "msg-456",
	}

	if !result.Success {
		t.Error("Success = false, want true")
	}
	if result.Target == "" {
		t.Error("Target should not be empty")
	}
	if result.MessageID == "" {
		t.Error("MessageID should not be empty")
	}
}

func TestEscalationResult_Failure(t *testing.T) {
	result := EscalationResult{
		Success: false,
		Error:   errors.New("connection failed"),
	}

	if result.Success {
		t.Error("Success = true, want false")
	}
	if result.Error == nil {
		t.Error("Error should not be nil on failure")
	}
}

// =============================================================================
// Priority Constants Tests
// =============================================================================

func TestEscalationPriority_Constants(t *testing.T) {
	if EscalationPriorityLow != "low" {
		t.Errorf("EscalationPriorityLow = %v, want low", EscalationPriorityLow)
	}
	if EscalationPriorityMedium != "medium" {
		t.Errorf("EscalationPriorityMedium = %v, want medium", EscalationPriorityMedium)
	}
	if EscalationPriorityHigh != "high" {
		t.Errorf("EscalationPriorityHigh = %v, want high", EscalationPriorityHigh)
	}
	if EscalationPriorityCritical != "critical" {
		t.Errorf("EscalationPriorityCritical = %v, want critical", EscalationPriorityCritical)
	}
}

// =============================================================================
// Error Constants Tests
// =============================================================================

func TestEscalationErrors_NotNil(t *testing.T) {
	if ErrNilInvestigation == nil {
		t.Error("ErrNilInvestigation should not be nil")
	}
	if ErrEscalationFailed == nil {
		t.Error("ErrEscalationFailed should not be nil")
	}
	if ErrNoEscalationTarget == nil {
		t.Error("ErrNoEscalationTarget should not be nil")
	}
	if ErrEscalationAlreadySent == nil {
		t.Error("ErrEscalationAlreadySent should not be nil")
	}
	if ErrEscalationRateLimited == nil {
		t.Error("ErrEscalationRateLimited should not be nil")
	}
	if ErrInvalidEscalationPriority == nil {
		t.Error("ErrInvalidEscalationPriority should not be nil")
	}
}

func TestEscalationErrors_HaveMessages(t *testing.T) {
	if ErrNilInvestigation.Error() == "" {
		t.Error("ErrNilInvestigation should have a message")
	}
	if ErrEscalationFailed.Error() == "" {
		t.Error("ErrEscalationFailed should have a message")
	}
	if ErrNoEscalationTarget.Error() == "" {
		t.Error("ErrNoEscalationTarget should have a message")
	}
	if ErrEscalationAlreadySent.Error() == "" {
		t.Error("ErrEscalationAlreadySent should have a message")
	}
	if ErrEscalationRateLimited.Error() == "" {
		t.Error("ErrEscalationRateLimited should have a message")
	}
	if ErrInvalidEscalationPriority.Error() == "" {
		t.Error("ErrInvalidEscalationPriority should have a message")
	}
}

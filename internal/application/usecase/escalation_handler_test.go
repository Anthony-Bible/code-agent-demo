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

	inv := &InvestigationStubForEscalation{
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

	inv := &InvestigationStubForEscalation{
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

	inv := &InvestigationStubForEscalation{
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

	inv := &InvestigationStubForEscalation{
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

	inv := &InvestigationStubForEscalation{
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
// ConversationEscalationHandler Tests
// =============================================================================

func TestNewConversationEscalationHandler_NotNil(t *testing.T) {
	handler := NewConversationEscalationHandler()
	if handler == nil {
		t.Error("NewConversationEscalationHandler() should not return nil")
	}
}

func TestNewConversationEscalationHandlerWithConfig_NotNil(t *testing.T) {
	config := EscalationConfig{
		MaxEscalationsPerInvestigation: 3,
		CooldownPeriod:                 5 * time.Minute,
		DefaultPriority:                EscalationPriorityMedium,
	}

	handler := NewConversationEscalationHandlerWithConfig(config)
	if handler == nil {
		t.Error("NewConversationEscalationHandlerWithConfig() should not return nil")
	}
}

func TestConversationEscalationHandler_Escalate_Success(t *testing.T) {
	handler := NewConversationEscalationHandler()
	if handler == nil {
		t.Skip("NewConversationEscalationHandler() returned nil")
	}

	handler.SetSessionID("test-session")

	inv := &InvestigationStubForEscalation{
		id:        "inv-conv-001",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "running",
		findings:  []string{"Network latency spike detected"},
	}

	req := EscalationRequest{
		Investigation: inv,
		Reason:        "Requires network team expertise",
		Priority:      EscalationPriorityHigh,
	}

	result, err := handler.Escalate(context.Background(), req)
	if err != nil {
		t.Errorf("Escalate() error = %v", err)
	}
	if result == nil {
		t.Error("Escalate() returned nil result")
	}
}

func TestConversationEscalationHandler_Escalate_NoSessionID(t *testing.T) {
	handler := NewConversationEscalationHandler()
	if handler == nil {
		t.Skip("NewConversationEscalationHandler() returned nil")
	}

	// Don't set session ID

	inv := &InvestigationStubForEscalation{
		id:        "inv-conv-002",
		alertID:   "alert-002",
		sessionID: "session-002",
		status:    "running",
	}

	req := EscalationRequest{
		Investigation: inv,
		Reason:        "Test",
		Priority:      EscalationPriorityLow,
	}

	_, err := handler.Escalate(context.Background(), req)
	if err == nil {
		t.Error("Escalate() without session ID should return error")
	}
}

func TestConversationEscalationHandler_Escalate_RateLimited(t *testing.T) {
	config := EscalationConfig{
		EnableRateLimiting: true,
		RateLimitPerMinute: 1,
	}

	handler := NewConversationEscalationHandlerWithConfig(config)
	if handler == nil {
		t.Skip("NewConversationEscalationHandlerWithConfig() returned nil")
	}

	handler.SetSessionID("test-session")

	inv := &InvestigationStubForEscalation{
		id:        "inv-rate-001",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "running",
	}

	req := EscalationRequest{
		Investigation: inv,
		Reason:        "First escalation",
		Priority:      EscalationPriorityMedium,
	}

	// First escalation should succeed
	_, err := handler.Escalate(context.Background(), req)
	if err != nil {
		t.Fatalf("First Escalate() error = %v", err)
	}

	// Second immediate escalation should be rate limited
	inv2 := &InvestigationStubForEscalation{
		id:        "inv-rate-002",
		alertID:   "alert-002",
		sessionID: "session-002",
		status:    "running",
	}
	req2 := EscalationRequest{
		Investigation: inv2,
		Reason:        "Second escalation",
		Priority:      EscalationPriorityMedium,
	}

	_, err = handler.Escalate(context.Background(), req2)
	if err == nil {
		t.Error("Second immediate Escalate() should be rate limited")
	}
}

func TestConversationEscalationHandler_Escalate_CancelledContext(t *testing.T) {
	handler := NewConversationEscalationHandler()
	if handler == nil {
		t.Skip("NewConversationEscalationHandler() returned nil")
	}

	handler.SetSessionID("test-session")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inv := &InvestigationStubForEscalation{
		id:        "inv-ctx-001",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "running",
	}

	req := EscalationRequest{
		Investigation: inv,
		Reason:        "Test",
		Priority:      EscalationPriorityLow,
	}

	_, err := handler.Escalate(ctx, req)
	if err == nil {
		t.Error("Escalate() with cancelled context should return error")
	}
}

// =============================================================================
// CompositeEscalationHandler Tests
// =============================================================================

func TestNewCompositeEscalationHandler_NotNil(t *testing.T) {
	handler := NewCompositeEscalationHandler()
	if handler == nil {
		t.Error("NewCompositeEscalationHandler() should not return nil")
	}
}

func TestNewCompositeEscalationHandler_WithHandlers(t *testing.T) {
	logHandler := NewLogEscalationHandler()
	convHandler := NewConversationEscalationHandler()

	if logHandler == nil || convHandler == nil {
		t.Skip("Sub-handlers returned nil")
	}

	composite := NewCompositeEscalationHandler(logHandler, convHandler)
	if composite == nil {
		t.Error("NewCompositeEscalationHandler(handlers...) should not return nil")
	}
}

func TestCompositeEscalationHandler_Escalate_CallsAllHandlers(t *testing.T) {
	logHandler := NewLogEscalationHandler()
	if logHandler == nil {
		t.Skip("NewLogEscalationHandler() returned nil")
	}

	composite := NewCompositeEscalationHandler(logHandler)
	if composite == nil {
		t.Skip("NewCompositeEscalationHandler() returned nil")
	}

	inv := &InvestigationStubForEscalation{
		id:        "inv-composite-001",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "running",
	}

	req := EscalationRequest{
		Investigation: inv,
		Reason:        "Composite test",
		Priority:      EscalationPriorityMedium,
	}

	result, err := composite.Escalate(context.Background(), req)
	if err != nil {
		t.Errorf("Escalate() error = %v", err)
	}
	if result == nil {
		t.Error("Escalate() returned nil result")
	}
}

func TestCompositeEscalationHandler_AddHandler(t *testing.T) {
	composite := NewCompositeEscalationHandler()
	if composite == nil {
		t.Skip("NewCompositeEscalationHandler() returned nil")
	}

	logHandler := NewLogEscalationHandler()
	if logHandler == nil {
		t.Skip("NewLogEscalationHandler() returned nil")
	}

	composite.AddHandler(logHandler)

	inv := &InvestigationStubForEscalation{
		id:        "inv-add-handler",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "running",
	}

	req := EscalationRequest{
		Investigation: inv,
		Reason:        "Added handler test",
		Priority:      EscalationPriorityLow,
	}

	result, err := composite.Escalate(context.Background(), req)
	if err != nil {
		t.Errorf("Escalate() after AddHandler() error = %v", err)
	}
	if result == nil {
		t.Error("Escalate() returned nil result")
	}
}

func TestCompositeEscalationHandler_CanEscalate_AllTrue(t *testing.T) {
	composite := NewCompositeEscalationHandler()
	if composite == nil {
		t.Skip("NewCompositeEscalationHandler() returned nil")
	}

	logHandler := NewLogEscalationHandler()
	if logHandler == nil {
		t.Skip("NewLogEscalationHandler() returned nil")
	}

	composite.AddHandler(logHandler)

	inv := &InvestigationStubForEscalation{
		id:          "inv-can-escalate",
		isEscalated: false,
	}

	if !composite.CanEscalate(inv) {
		t.Error("CanEscalate() = false, want true when all handlers can escalate")
	}
}

func TestCompositeEscalationHandler_GetEscalationHistory_Combined(t *testing.T) {
	composite := NewCompositeEscalationHandler()
	if composite == nil {
		t.Skip("NewCompositeEscalationHandler() returned nil")
	}

	logHandler := NewLogEscalationHandler()
	if logHandler == nil {
		t.Skip("NewLogEscalationHandler() returned nil")
	}

	composite.AddHandler(logHandler)

	inv := &InvestigationStubForEscalation{
		id:        "inv-history-combined",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "running",
	}

	req := EscalationRequest{
		Investigation: inv,
		Reason:        "History test",
		Priority:      EscalationPriorityMedium,
	}

	_, _ = composite.Escalate(context.Background(), req)

	history := composite.GetEscalationHistory("inv-history-combined")
	// Should have at least one entry from the log handler
	if len(history) < 1 {
		t.Errorf("GetEscalationHistory() len = %v, want >= 1", len(history))
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
		Investigation: &InvestigationStubForEscalation{id: "inv-001"},
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

package entity

import (
	"errors"
	"testing"
)

// =============================================================================
// Investigation State Machine Tests
// These tests verify the state transition logic for Investigation entities.
// The state machine follows this diagram:
//
//   started -> running -> completed|failed|escalated
//   (terminal states: completed, failed, escalated have no outgoing transitions)
//
// RED PHASE: These tests are expected to FAIL until the state machine methods
// are implemented in investigation.go.
// =============================================================================

// ErrInvalidTransition is expected to be defined in investigation.go
// for invalid state transitions.
var _ = errors.New("placeholder for compile check")

// =============================================================================
// TransitionTo Tests - Valid Transitions
// =============================================================================

func TestInvestigation_TransitionTo_StartedToRunning(t *testing.T) {
	// Arrange: create an investigation in "started" status
	inv, err := NewInvestigation("inv-trans-001", "alert-001", "session-001")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv.Status() != InvestigationStatusStarted {
		t.Fatalf("Initial status = %v, want %v", inv.Status(), InvestigationStatusStarted)
	}

	// Act: transition to running
	err = inv.TransitionTo(InvestigationStatusRunning)
	// Assert: transition should succeed
	if err != nil {
		t.Errorf("TransitionTo(running) error = %v, want nil", err)
	}
	if inv.Status() != InvestigationStatusRunning {
		t.Errorf("Status() after transition = %v, want %v", inv.Status(), InvestigationStatusRunning)
	}
}

func TestInvestigation_TransitionTo_RunningToCompleted(t *testing.T) {
	// Arrange: create investigation and move to running
	inv, err := NewInvestigation("inv-trans-002", "alert-002", "session-002")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	_ = inv.SetStatus(InvestigationStatusRunning)

	// Act: transition to completed
	err = inv.TransitionTo(InvestigationStatusCompleted)
	// Assert: transition should succeed
	if err != nil {
		t.Errorf("TransitionTo(completed) error = %v, want nil", err)
	}
	if inv.Status() != InvestigationStatusCompleted {
		t.Errorf("Status() after transition = %v, want %v", inv.Status(), InvestigationStatusCompleted)
	}
}

func TestInvestigation_TransitionTo_RunningToFailed(t *testing.T) {
	// Arrange: create investigation and move to running
	inv, err := NewInvestigation("inv-trans-003", "alert-003", "session-003")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	_ = inv.SetStatus(InvestigationStatusRunning)

	// Act: transition to failed
	err = inv.TransitionTo(InvestigationStatusFailed)
	// Assert: transition should succeed
	if err != nil {
		t.Errorf("TransitionTo(failed) error = %v, want nil", err)
	}
	if inv.Status() != InvestigationStatusFailed {
		t.Errorf("Status() after transition = %v, want %v", inv.Status(), InvestigationStatusFailed)
	}
}

func TestInvestigation_TransitionTo_RunningToEscalated(t *testing.T) {
	// Arrange: create investigation and move to running
	inv, err := NewInvestigation("inv-trans-004", "alert-004", "session-004")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	_ = inv.SetStatus(InvestigationStatusRunning)

	// Act: transition to escalated
	err = inv.TransitionTo(InvestigationStatusEscalated)
	// Assert: transition should succeed
	if err != nil {
		t.Errorf("TransitionTo(escalated) error = %v, want nil", err)
	}
	if inv.Status() != InvestigationStatusEscalated {
		t.Errorf("Status() after transition = %v, want %v", inv.Status(), InvestigationStatusEscalated)
	}
}

// =============================================================================
// TransitionTo Tests - Invalid Transitions from Terminal States
// =============================================================================

func TestInvestigation_TransitionTo_InvalidFromCompleted(t *testing.T) {
	tests := []struct {
		name      string
		toStatus  string
		wantError bool
	}{
		{"completed to started", InvestigationStatusStarted, true},
		{"completed to running", InvestigationStatusRunning, true},
		{"completed to failed", InvestigationStatusFailed, true},
		{"completed to escalated", InvestigationStatusEscalated, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: create investigation in completed state
			inv, err := NewInvestigation("inv-term-completed", "alert-term", "session-term")
			if err != nil {
				t.Fatalf("NewInvestigation() error = %v", err)
			}
			inv.Complete() // puts in completed state

			// Act: attempt invalid transition
			err = inv.TransitionTo(tt.toStatus)

			// Assert: should return error
			if tt.wantError && err == nil {
				t.Errorf("TransitionTo(%s) from completed = nil, want error", tt.toStatus)
			}
		})
	}
}

func TestInvestigation_TransitionTo_InvalidFromFailed(t *testing.T) {
	tests := []struct {
		name      string
		toStatus  string
		wantError bool
	}{
		{"failed to started", InvestigationStatusStarted, true},
		{"failed to running", InvestigationStatusRunning, true},
		{"failed to completed", InvestigationStatusCompleted, true},
		{"failed to escalated", InvestigationStatusEscalated, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: create investigation in failed state
			inv, err := NewInvestigation("inv-term-failed", "alert-term", "session-term")
			if err != nil {
				t.Fatalf("NewInvestigation() error = %v", err)
			}
			inv.Fail("test failure") // puts in failed state

			// Act: attempt invalid transition
			err = inv.TransitionTo(tt.toStatus)

			// Assert: should return error
			if tt.wantError && err == nil {
				t.Errorf("TransitionTo(%s) from failed = nil, want error", tt.toStatus)
			}
		})
	}
}

func TestInvestigation_TransitionTo_InvalidFromEscalated(t *testing.T) {
	tests := []struct {
		name      string
		toStatus  string
		wantError bool
	}{
		{"escalated to started", InvestigationStatusStarted, true},
		{"escalated to running", InvestigationStatusRunning, true},
		{"escalated to completed", InvestigationStatusCompleted, true},
		{"escalated to failed", InvestigationStatusFailed, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: create investigation in escalated state
			inv, err := NewInvestigation("inv-term-escalated", "alert-term", "session-term")
			if err != nil {
				t.Fatalf("NewInvestigation() error = %v", err)
			}
			inv.Escalate("test escalation") // puts in escalated state

			// Act: attempt invalid transition
			err = inv.TransitionTo(tt.toStatus)

			// Assert: should return error
			if tt.wantError && err == nil {
				t.Errorf("TransitionTo(%s) from escalated = nil, want error", tt.toStatus)
			}
		})
	}
}

// =============================================================================
// TransitionTo Tests - Invalid Skip Transitions
// =============================================================================

func TestInvestigation_TransitionTo_InvalidSkipRunning(t *testing.T) {
	tests := []struct {
		name      string
		toStatus  string
		wantError bool
	}{
		{"started to completed (skips running)", InvestigationStatusCompleted, true},
		{"started to failed (skips running)", InvestigationStatusFailed, true},
		{"started to escalated (skips running)", InvestigationStatusEscalated, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: create investigation in started state
			inv, err := NewInvestigation("inv-skip", "alert-skip", "session-skip")
			if err != nil {
				t.Fatalf("NewInvestigation() error = %v", err)
			}

			// Act: attempt to skip the running state
			err = inv.TransitionTo(tt.toStatus)

			// Assert: should return error (cannot skip running)
			if tt.wantError && err == nil {
				t.Errorf("TransitionTo(%s) from started = nil, want error (should not skip running)", tt.toStatus)
			}
		})
	}
}

func TestInvestigation_TransitionTo_InvalidBackToStarted(t *testing.T) {
	// Arrange: create investigation and move to running
	inv, err := NewInvestigation("inv-back", "alert-back", "session-back")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	_ = inv.SetStatus(InvestigationStatusRunning)

	// Act: attempt to go back to started
	err = inv.TransitionTo(InvestigationStatusStarted)

	// Assert: should return error (cannot go backwards)
	if err == nil {
		t.Error("TransitionTo(started) from running = nil, want error (cannot transition backwards)")
	}
}

// =============================================================================
// CanTransitionTo Tests
// =============================================================================

func TestInvestigation_CanTransitionTo_ValidTransitions(t *testing.T) {
	tests := []struct {
		name       string
		fromStatus string
		toStatus   string
		want       bool
	}{
		// Valid transitions
		{"started can go to running", InvestigationStatusStarted, InvestigationStatusRunning, true},
		{"running can go to completed", InvestigationStatusRunning, InvestigationStatusCompleted, true},
		{"running can go to failed", InvestigationStatusRunning, InvestigationStatusFailed, true},
		{"running can go to escalated", InvestigationStatusRunning, InvestigationStatusEscalated, true},

		// Invalid transitions from started
		{"started cannot go to completed", InvestigationStatusStarted, InvestigationStatusCompleted, false},
		{"started cannot go to failed", InvestigationStatusStarted, InvestigationStatusFailed, false},
		{"started cannot go to escalated", InvestigationStatusStarted, InvestigationStatusEscalated, false},

		// Invalid transitions from running
		{"running cannot go to started", InvestigationStatusRunning, InvestigationStatusStarted, false},

		// Invalid transitions from terminal states
		{"completed cannot go anywhere", InvestigationStatusCompleted, InvestigationStatusRunning, false},
		{"failed cannot go anywhere", InvestigationStatusFailed, InvestigationStatusRunning, false},
		{"escalated cannot go anywhere", InvestigationStatusEscalated, InvestigationStatusRunning, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Arrange: create investigation and set to fromStatus
			inv, err := NewInvestigation("inv-can", "alert-can", "session-can")
			if err != nil {
				t.Fatalf("NewInvestigation() error = %v", err)
			}
			_ = inv.SetStatus(tt.fromStatus)

			// Act
			got := inv.CanTransitionTo(tt.toStatus)

			// Assert
			if got != tt.want {
				t.Errorf("CanTransitionTo(%s) from %s = %v, want %v", tt.toStatus, tt.fromStatus, got, tt.want)
			}
		})
	}
}

func TestInvestigation_CanTransitionTo_SameStatus(t *testing.T) {
	// Arrange
	inv, err := NewInvestigation("inv-same", "alert-same", "session-same")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}

	// Act: check if can transition to current status
	got := inv.CanTransitionTo(InvestigationStatusStarted)

	// Assert: transitioning to same state should return false
	if got {
		t.Error("CanTransitionTo(same status) = true, want false")
	}
}

func TestInvestigation_CanTransitionTo_InvalidStatus(t *testing.T) {
	// Arrange
	inv, err := NewInvestigation("inv-invalid", "alert-invalid", "session-invalid")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}

	// Act: check transition to invalid status
	got := inv.CanTransitionTo("nonexistent_status")

	// Assert: should return false for invalid status
	if got {
		t.Error("CanTransitionTo(invalid status) = true, want false")
	}
}

// =============================================================================
// Start() Method Tests
// =============================================================================

func TestInvestigation_Start_TransitionsToRunning(t *testing.T) {
	// Arrange
	inv, err := NewInvestigation("inv-start", "alert-start", "session-start")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	if inv.Status() != InvestigationStatusStarted {
		t.Fatalf("Initial status = %v, want %v", inv.Status(), InvestigationStatusStarted)
	}

	// Act
	err = inv.Start()
	// Assert
	if err != nil {
		t.Errorf("Start() error = %v, want nil", err)
	}
	if inv.Status() != InvestigationStatusRunning {
		t.Errorf("Status() after Start() = %v, want %v", inv.Status(), InvestigationStatusRunning)
	}
}

func TestInvestigation_Start_FailsFromRunning(t *testing.T) {
	// Arrange: investigation already running
	inv, err := NewInvestigation("inv-start-running", "alert-start", "session-start")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	_ = inv.SetStatus(InvestigationStatusRunning)

	// Act
	err = inv.Start()

	// Assert: should return error, already running
	if err == nil {
		t.Error("Start() from running status = nil, want error")
	}
}

func TestInvestigation_Start_FailsFromCompleted(t *testing.T) {
	// Arrange: investigation already completed
	inv, err := NewInvestigation("inv-start-completed", "alert-start", "session-start")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	inv.Complete()

	// Act
	err = inv.Start()

	// Assert: should return error, already in terminal state
	if err == nil {
		t.Error("Start() from completed status = nil, want error")
	}
}

func TestInvestigation_Start_FailsFromFailed(t *testing.T) {
	// Arrange: investigation already failed
	inv, err := NewInvestigation("inv-start-failed", "alert-start", "session-start")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	inv.Fail("previous failure")

	// Act
	err = inv.Start()

	// Assert: should return error, already in terminal state
	if err == nil {
		t.Error("Start() from failed status = nil, want error")
	}
}

func TestInvestigation_Start_FailsFromEscalated(t *testing.T) {
	// Arrange: investigation already escalated
	inv, err := NewInvestigation("inv-start-escalated", "alert-start", "session-start")
	if err != nil {
		t.Fatalf("NewInvestigation() error = %v", err)
	}
	inv.Escalate("previous escalation")

	// Act
	err = inv.Start()

	// Assert: should return error, already in terminal state
	if err == nil {
		t.Error("Start() from escalated status = nil, want error")
	}
}

// =============================================================================
// Error Constant Tests
// =============================================================================

func TestInvestigationStateErrors_ErrInvalidTransition(t *testing.T) {
	// This test verifies that ErrInvalidTransition error exists
	// RED PHASE: This will fail because ErrInvalidTransition is not defined yet
	if ErrInvalidTransition == nil {
		t.Error("ErrInvalidTransition should not be nil")
	}
	if ErrInvalidTransition.Error() == "" {
		t.Error("ErrInvalidTransition should have a message")
	}
}

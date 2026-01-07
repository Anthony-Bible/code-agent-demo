package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
	"testing"
	"time"
)

// =============================================================================
// AlertInvestigationUseCase Tests
// These tests verify the behavior of AlertInvestigationUseCase.
// =============================================================================

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewAlertInvestigationUseCase_NotNil(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Error("NewAlertInvestigationUseCase() should not return nil")
	}
}

func TestNewAlertInvestigationUseCaseWithConfig_NotNil(t *testing.T) {
	config := AlertInvestigationUseCaseConfig{
		MaxActions:    20,
		MaxDuration:   15 * time.Minute,
		MaxConcurrent: 5,
		AllowedTools:  []string{"bash", "read_file"},
	}

	uc := NewAlertInvestigationUseCaseWithConfig(config)
	if uc == nil {
		t.Error("NewAlertInvestigationUseCaseWithConfig() should not return nil")
	}
}

func TestNewAlertInvestigationUseCase_InitialActiveCountZero(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}
	if uc.GetActiveCount() != 0 {
		t.Errorf("GetActiveCount() = %v, want 0", uc.GetActiveCount())
	}
}

// =============================================================================
// HandleAlert Tests
// =============================================================================

func TestAlertInvestigationUseCase_HandleAlert_Success(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}
	uc.SetConversationService(newInvestigationRunnerConvServiceMock())
	uc.SetToolExecutor(newInvestigationRunnerToolExecutorMock())
	uc.SetPromptBuilderRegistry(newInvestigationRunnerPromptBuilderMock())

	alert := &AlertForInvestigation{
		id:       "alert-001",
		source:   "prometheus",
		severity: "warning",
		title:    "High CPU Usage",
		labels:   map[string]string{"instance": "web-01"},
	}

	result, err := uc.HandleAlert(context.Background(), alert)
	if err != nil {
		t.Errorf("HandleAlert() error = %v", err)
	}
	if result == nil {
		t.Error("HandleAlert() returned nil result")
	}
}

func TestAlertInvestigationUseCase_HandleAlert_NilAlert(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	_, err := uc.HandleAlert(context.Background(), nil)
	if err == nil {
		t.Error("HandleAlert(nil) should return error")
	}
}

func TestAlertInvestigationUseCase_HandleAlert_CancelledContext(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	alert := &AlertForInvestigation{
		id:       "alert-cancelled",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	_, err := uc.HandleAlert(ctx, alert)
	if err == nil {
		t.Error("HandleAlert() with cancelled context should return error")
	}
}

func TestAlertInvestigationUseCase_HandleAlert_Timeout(t *testing.T) {
	config := AlertInvestigationUseCaseConfig{
		MaxDuration: 1 * time.Millisecond, // Very short timeout
	}

	uc := NewAlertInvestigationUseCaseWithConfig(config)
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCaseWithConfig() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-timeout",
		source:   "prometheus",
		severity: "critical",
		title:    "Long Running Alert",
	}

	result, err := uc.HandleAlert(context.Background(), alert)
	// Should either timeout or complete quickly
	if err != nil && result == nil {
		// Timeout is expected
		return
	}
	if result != nil && result.Status == "failed" {
		// Failed due to timeout is also acceptable
		return
	}
}

func TestAlertInvestigationUseCase_HandleAlert_CriticalAlert(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}
	uc.SetConversationService(newInvestigationRunnerConvServiceMock())
	uc.SetToolExecutor(newInvestigationRunnerToolExecutorMock())
	uc.SetPromptBuilderRegistry(newInvestigationRunnerPromptBuilderMock())

	alert := &AlertForInvestigation{
		id:       "alert-critical",
		source:   "prometheus",
		severity: "critical",
		title:    "Critical System Failure",
		labels:   map[string]string{"instance": "db-01"},
	}

	result, err := uc.HandleAlert(context.Background(), alert)
	if err != nil {
		t.Errorf("HandleAlert() error = %v", err)
	}
	if result == nil {
		t.Error("HandleAlert() returned nil for critical alert")
	}
}

func TestAlertInvestigationUseCase_HandleAlert_ReturnsInvestigationID(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}
	uc.SetConversationService(newInvestigationRunnerConvServiceMock())
	uc.SetToolExecutor(newInvestigationRunnerToolExecutorMock())
	uc.SetPromptBuilderRegistry(newInvestigationRunnerPromptBuilderMock())

	alert := &AlertForInvestigation{
		id:       "alert-id-test",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	result, err := uc.HandleAlert(context.Background(), alert)
	if err != nil {
		t.Fatalf("HandleAlert() error = %v", err)
	}

	if result.InvestigationID == "" {
		t.Error("HandleAlert() result should have InvestigationID")
	}
	if result.AlertID != "alert-id-test" {
		t.Errorf("HandleAlert() AlertID = %v, want alert-id-test", result.AlertID)
	}
}

// =============================================================================
// StartInvestigation Tests
// =============================================================================

func TestAlertInvestigationUseCase_StartInvestigation_Success(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-start",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	invID, err := uc.StartInvestigation(context.Background(), alert)
	if err != nil {
		t.Errorf("StartInvestigation() error = %v", err)
	}
	if invID == "" {
		t.Error("StartInvestigation() should return investigation ID")
	}
}

func TestAlertInvestigationUseCase_StartInvestigation_IncrementsActiveCount(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	if uc.GetActiveCount() != 0 {
		t.Fatalf("Initial GetActiveCount() = %v, want 0", uc.GetActiveCount())
	}

	alert := &AlertForInvestigation{
		id:       "alert-count",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	_, err := uc.StartInvestigation(context.Background(), alert)
	if err != nil {
		t.Fatalf("StartInvestigation() error = %v", err)
	}

	if uc.GetActiveCount() != 1 {
		t.Errorf("GetActiveCount() after start = %v, want 1", uc.GetActiveCount())
	}
}

func TestAlertInvestigationUseCase_StartInvestigation_MaxConcurrent(t *testing.T) {
	config := AlertInvestigationUseCaseConfig{
		MaxConcurrent: 2,
		MaxDuration:   1 * time.Hour, // Long duration so they stay active
	}

	uc := NewAlertInvestigationUseCaseWithConfig(config)
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCaseWithConfig() returned nil")
	}

	// Start max concurrent investigations
	for i := range 2 {
		alert := &AlertForInvestigation{
			id:       "alert-max-" + string(rune('a'+i)),
			source:   "prometheus",
			severity: "warning",
			title:    "Test Alert",
		}
		_, err := uc.StartInvestigation(context.Background(), alert)
		if err != nil {
			t.Fatalf("StartInvestigation() %d error = %v", i, err)
		}
	}

	// Third should fail
	alert := &AlertForInvestigation{
		id:       "alert-max-overflow",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}
	_, err := uc.StartInvestigation(context.Background(), alert)
	if err == nil {
		t.Error("StartInvestigation() should fail when max concurrent reached")
	}
}

func TestAlertInvestigationUseCase_StartInvestigation_DuplicateAlert(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-dup",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	_, err := uc.StartInvestigation(context.Background(), alert)
	if err != nil {
		t.Fatalf("First StartInvestigation() error = %v", err)
	}

	// Start same alert again
	_, err = uc.StartInvestigation(context.Background(), alert)
	if err == nil {
		t.Error("StartInvestigation() should fail for duplicate alert")
	}
}

// =============================================================================
// StopInvestigation Tests
// =============================================================================

func TestAlertInvestigationUseCase_StopInvestigation_Success(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-stop",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	invID, err := uc.StartInvestigation(context.Background(), alert)
	if err != nil {
		t.Fatalf("StartInvestigation() error = %v", err)
	}

	err = uc.StopInvestigation(context.Background(), invID)
	if err != nil {
		t.Errorf("StopInvestigation() error = %v", err)
	}
}

func TestAlertInvestigationUseCase_StopInvestigation_DecrementsActiveCount(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-stop-count",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	invID, _ := uc.StartInvestigation(context.Background(), alert)

	if uc.GetActiveCount() != 1 {
		t.Fatalf("GetActiveCount() after start = %v, want 1", uc.GetActiveCount())
	}

	_ = uc.StopInvestigation(context.Background(), invID)

	if uc.GetActiveCount() != 0 {
		t.Errorf("GetActiveCount() after stop = %v, want 0", uc.GetActiveCount())
	}
}

func TestAlertInvestigationUseCase_StopInvestigation_NotFound(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	err := uc.StopInvestigation(context.Background(), "nonexistent")
	if err == nil {
		t.Error("StopInvestigation() should return error for nonexistent ID")
	}
}

// =============================================================================
// GetInvestigationStatus Tests
// =============================================================================

func TestAlertInvestigationUseCase_GetInvestigationStatus_Exists(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-status",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	invID, err := uc.StartInvestigation(context.Background(), alert)
	if err != nil {
		t.Fatalf("StartInvestigation() error = %v", err)
	}

	status, err := uc.GetInvestigationStatus(context.Background(), invID)
	if err != nil {
		t.Errorf("GetInvestigationStatus() error = %v", err)
	}
	if status == nil {
		t.Error("GetInvestigationStatus() returned nil")
	}
}

func TestAlertInvestigationUseCase_GetInvestigationStatus_NotFound(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	_, err := uc.GetInvestigationStatus(context.Background(), "nonexistent")
	if err == nil {
		t.Error("GetInvestigationStatus() should return error for nonexistent ID")
	}
}

// =============================================================================
// ListActiveInvestigations Tests
// =============================================================================

func TestAlertInvestigationUseCase_ListActiveInvestigations_Empty(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	list, err := uc.ListActiveInvestigations(context.Background())
	if err != nil {
		t.Errorf("ListActiveInvestigations() error = %v", err)
	}
	if list == nil {
		t.Error("ListActiveInvestigations() should return empty slice, not nil")
	}
	if len(list) != 0 {
		t.Errorf("ListActiveInvestigations() len = %v, want 0", len(list))
	}
}

func TestAlertInvestigationUseCase_ListActiveInvestigations_WithActive(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	for i := range 3 {
		alert := &AlertForInvestigation{
			id:       "alert-list-" + string(rune('a'+i)),
			source:   "prometheus",
			severity: "warning",
			title:    "Test Alert",
		}
		_, _ = uc.StartInvestigation(context.Background(), alert)
	}

	list, err := uc.ListActiveInvestigations(context.Background())
	if err != nil {
		t.Errorf("ListActiveInvestigations() error = %v", err)
	}
	if len(list) != 3 {
		t.Errorf("ListActiveInvestigations() len = %v, want 3", len(list))
	}
}

// =============================================================================
// Tool/Command Safety Tests
// =============================================================================

func TestAlertInvestigationUseCase_IsToolAllowed_InConfig(t *testing.T) {
	config := AlertInvestigationUseCaseConfig{
		AllowedTools: []string{"bash", "read_file", "list_files"},
	}

	uc := NewAlertInvestigationUseCaseWithConfig(config)
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCaseWithConfig() returned nil")
	}

	if !uc.IsToolAllowed("bash") {
		t.Error("IsToolAllowed('bash') = false, want true")
	}
	if !uc.IsToolAllowed("read_file") {
		t.Error("IsToolAllowed('read_file') = false, want true")
	}
}

func TestAlertInvestigationUseCase_IsToolAllowed_NotInConfig(t *testing.T) {
	config := AlertInvestigationUseCaseConfig{
		AllowedTools: []string{"bash", "read_file"},
	}

	uc := NewAlertInvestigationUseCaseWithConfig(config)
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCaseWithConfig() returned nil")
	}

	if uc.IsToolAllowed("edit_file") {
		t.Error("IsToolAllowed('edit_file') = true, want false")
	}
	if uc.IsToolAllowed("execute_sql") {
		t.Error("IsToolAllowed('execute_sql') = true, want false")
	}
}

func TestAlertInvestigationUseCase_IsCommandBlocked_DangerousCommands(t *testing.T) {
	config := AlertInvestigationUseCaseConfig{
		BlockedCommands: []string{"rm -rf", "dd if=", "mkfs"},
	}

	uc := NewAlertInvestigationUseCaseWithConfig(config)
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCaseWithConfig() returned nil")
	}

	if !uc.IsCommandBlocked("rm -rf /") {
		t.Error("IsCommandBlocked('rm -rf /') = false, want true")
	}
	if !uc.IsCommandBlocked("dd if=/dev/zero of=/dev/sda") {
		t.Error("IsCommandBlocked('dd if=...') = false, want true")
	}
}

func TestAlertInvestigationUseCase_IsCommandBlocked_SafeCommands(t *testing.T) {
	config := AlertInvestigationUseCaseConfig{
		BlockedCommands: []string{"rm -rf", "dd if="},
	}

	uc := NewAlertInvestigationUseCaseWithConfig(config)
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCaseWithConfig() returned nil")
	}

	if uc.IsCommandBlocked("ls -la") {
		t.Error("IsCommandBlocked('ls -la') = true, want false")
	}
	if uc.IsCommandBlocked("top -b -n 1") {
		t.Error("IsCommandBlocked('top -b -n 1') = true, want false")
	}
}

// =============================================================================
// Escalation Integration Tests
// =============================================================================

func TestAlertInvestigationUseCase_SetEscalationHandler(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	handler := NewLogEscalationHandler()
	if handler == nil {
		t.Skip("NewLogEscalationHandler() returned nil")
	}

	// Should not panic
	uc.SetEscalationHandler(handler)
}

func TestAlertInvestigationUseCase_SetPromptBuilderRegistry(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	registry := NewPromptBuilderRegistry()
	if registry == nil {
		t.Skip("NewPromptBuilderRegistry() returned nil")
	}

	// Should not panic
	uc.SetPromptBuilderRegistry(registry)
}

// =============================================================================
// Shutdown Tests
// =============================================================================

func TestAlertInvestigationUseCase_Shutdown_Success(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	err := uc.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestAlertInvestigationUseCase_Shutdown_StopsActiveInvestigations(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	// Start some investigations
	for i := range 3 {
		alert := &AlertForInvestigation{
			id:       "alert-shutdown-" + string(rune('a'+i)),
			source:   "prometheus",
			severity: "warning",
			title:    "Test Alert",
		}
		_, _ = uc.StartInvestigation(context.Background(), alert)
	}

	err := uc.Shutdown(context.Background())
	if err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}

	if uc.GetActiveCount() != 0 {
		t.Errorf("GetActiveCount() after shutdown = %v, want 0", uc.GetActiveCount())
	}
}

func TestAlertInvestigationUseCase_Shutdown_WithTimeout(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := uc.Shutdown(ctx)
	// Should either succeed or timeout
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Logf("Shutdown() with timeout error = %v (acceptable)", err)
	}
}

// =============================================================================
// InvestigationResult Tests
// =============================================================================

func TestInvestigationResult_Fields(t *testing.T) {
	result := InvestigationResult{
		InvestigationID: "inv-001",
		AlertID:         "alert-001",
		Status:          "completed",
		Findings:        []string{"Root cause identified", "High load from process X"},
		ActionsTaken:    5,
		Duration:        2 * time.Minute,
		Confidence:      0.85,
		Escalated:       false,
	}

	if result.InvestigationID != "inv-001" {
		t.Errorf("InvestigationID = %v, want inv-001", result.InvestigationID)
	}
	if result.AlertID != "alert-001" {
		t.Errorf("AlertID = %v, want alert-001", result.AlertID)
	}
	if result.Status != "completed" {
		t.Errorf("Status = %v, want completed", result.Status)
	}
	if len(result.Findings) != 2 {
		t.Errorf("Findings len = %v, want 2", len(result.Findings))
	}
	if result.ActionsTaken != 5 {
		t.Errorf("ActionsTaken = %v, want 5", result.ActionsTaken)
	}
	if result.Duration != 2*time.Minute {
		t.Errorf("Duration = %v, want 2m", result.Duration)
	}
	if result.Confidence != 0.85 {
		t.Errorf("Confidence = %v, want 0.85", result.Confidence)
	}
	if result.Escalated {
		t.Error("Escalated = true, want false")
	}
}

func TestInvestigationResult_Escalated(t *testing.T) {
	result := InvestigationResult{
		InvestigationID: "inv-002",
		AlertID:         "alert-002",
		Status:          "escalated",
		Escalated:       true,
		EscalateReason:  "Low confidence in root cause",
		Confidence:      0.45,
	}

	if !result.Escalated {
		t.Error("Escalated = false, want true")
	}
	if result.EscalateReason == "" {
		t.Error("EscalateReason should not be empty when escalated")
	}
}

func TestInvestigationResult_Failed(t *testing.T) {
	result := InvestigationResult{
		InvestigationID: "inv-003",
		AlertID:         "alert-003",
		Status:          "failed",
		Error:           errors.New("connection refused"),
	}

	if result.Status != "failed" {
		t.Errorf("Status = %v, want failed", result.Status)
	}
	if result.Error == nil {
		t.Error("Error should not be nil for failed investigation")
	}
}

// =============================================================================
// Error Constants Tests
// =============================================================================

func TestAlertInvestigationErrors_NotNil(t *testing.T) {
	if ErrAlertNil == nil {
		t.Error("ErrAlertNil should not be nil")
	}
	if ErrInvestigationAlreadyRunning == nil {
		t.Error("ErrInvestigationAlreadyRunning should not be nil")
	}
	if ErrMaxConcurrentReached == nil {
		t.Error("ErrMaxConcurrentReached should not be nil")
	}
	if ErrInvestigationTimeout == nil {
		t.Error("ErrInvestigationTimeout should not be nil")
	}
	if ErrActionBudgetExceeded == nil {
		t.Error("ErrActionBudgetExceeded should not be nil")
	}
	if ErrToolNotAllowed == nil {
		t.Error("ErrToolNotAllowed should not be nil")
	}
	if ErrCommandBlocked == nil {
		t.Error("ErrCommandBlocked should not be nil")
	}
}

func TestAlertInvestigationErrors_HaveMessages(t *testing.T) {
	if ErrAlertNil.Error() == "" {
		t.Error("ErrAlertNil should have a message")
	}
	if ErrInvestigationAlreadyRunning.Error() == "" {
		t.Error("ErrInvestigationAlreadyRunning should have a message")
	}
	if ErrMaxConcurrentReached.Error() == "" {
		t.Error("ErrMaxConcurrentReached should have a message")
	}
	if ErrInvestigationTimeout.Error() == "" {
		t.Error("ErrInvestigationTimeout should have a message")
	}
	if ErrActionBudgetExceeded.Error() == "" {
		t.Error("ErrActionBudgetExceeded should have a message")
	}
	if ErrToolNotAllowed.Error() == "" {
		t.Error("ErrToolNotAllowed should have a message")
	}
	if ErrCommandBlocked.Error() == "" {
		t.Error("ErrCommandBlocked should have a message")
	}
}

// =============================================================================
// Safety Enforcer Integration Tests (Phase 4)
// These tests verify that AlertInvestigationUseCase integrates with SafetyEnforcer.
// =============================================================================

func TestAlertInvestigationUseCase_SetSafetyEnforcer(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	enforcer := NewMockSafetyEnforcer()
	if enforcer == nil {
		t.Skip("NewMockSafetyEnforcer() returned nil")
	}

	// Should not panic
	uc.SetSafetyEnforcer(enforcer)
}

func TestAlertInvestigationUseCase_HandleAlert_WithSafetyEnforcer_BlockedTool(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	// Create a mock enforcer that blocks all tools
	enforcer := NewMockSafetyEnforcerWithBlockedTools([]string{"bash", "read_file", "list_files"})
	if enforcer == nil {
		t.Skip("NewMockSafetyEnforcerWithBlockedTools() returned nil")
	}
	uc.SetSafetyEnforcer(enforcer)

	alert := &AlertForInvestigation{
		id:       "alert-blocked-tool",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	result, err := uc.HandleAlert(context.Background(), alert)
	// Should fail or escalate due to blocked tools
	if err == nil && result != nil && result.Status == "completed" {
		t.Error("HandleAlert() with blocked tools should not complete successfully")
	}
	if result != nil && !result.Escalated && result.Status != "failed" {
		t.Error("HandleAlert() with blocked tools should either escalate or fail")
	}
}

func TestAlertInvestigationUseCase_HandleAlert_WithSafetyEnforcer_BlockedCommand(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	// Create a mock enforcer that blocks dangerous commands
	enforcer := NewMockSafetyEnforcerWithBlockedCommands([]string{"rm -rf", "dd if="})
	if enforcer == nil {
		t.Skip("NewMockSafetyEnforcerWithBlockedCommands() returned nil")
	}
	uc.SetSafetyEnforcer(enforcer)

	alert := &AlertForInvestigation{
		id:       "alert-blocked-command",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	// The investigation itself should start, but if it tries to execute
	// a blocked command, it should be rejected
	result, err := uc.HandleAlert(context.Background(), alert)
	// We're testing that the enforcer is wired in - actual behavior depends on implementation
	if err != nil {
		t.Logf("HandleAlert() with enforcer error = %v (may be expected)", err)
	}
	if result != nil {
		t.Logf("HandleAlert() status = %v", result.Status)
	}
}

func TestAlertInvestigationUseCase_HandleAlert_WithSafetyEnforcer_ActionBudgetExhausted(t *testing.T) {
	config := AlertInvestigationUseCaseConfig{
		MaxActions:    3, // Very low action budget
		MaxDuration:   15 * time.Minute,
		MaxConcurrent: 5,
		AllowedTools:  []string{"bash", "read_file"},
	}

	uc := NewAlertInvestigationUseCaseWithConfig(config)
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCaseWithConfig() returned nil")
	}

	// Create a mock enforcer with the same low action budget
	enforcer := NewMockSafetyEnforcerWithActionBudget(3)
	if enforcer == nil {
		t.Skip("NewMockSafetyEnforcerWithActionBudget() returned nil")
	}
	uc.SetSafetyEnforcer(enforcer)

	alert := &AlertForInvestigation{
		id:       "alert-budget-test",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	result, err := uc.HandleAlert(context.Background(), alert)
	// When action budget is exhausted, investigation should stop
	if result != nil && result.ActionsTaken > 3 {
		t.Errorf("HandleAlert() ActionsTaken = %d, should not exceed budget of 3", result.ActionsTaken)
	}
	if err != nil {
		t.Logf("HandleAlert() with budget limit error = %v (may be expected)", err)
	}
}

func TestAlertInvestigationUseCase_HandleAlert_WithSafetyEnforcer_Timeout(t *testing.T) {
	config := AlertInvestigationUseCaseConfig{
		MaxDuration:   50 * time.Millisecond, // Very short timeout
		MaxConcurrent: 5,
		AllowedTools:  []string{"bash", "read_file"},
	}

	uc := NewAlertInvestigationUseCaseWithConfig(config)
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCaseWithConfig() returned nil")
	}

	// Create a mock enforcer that respects timeout
	enforcer := NewMockSafetyEnforcer()
	if enforcer == nil {
		t.Skip("NewMockSafetyEnforcer() returned nil")
	}
	uc.SetSafetyEnforcer(enforcer)

	alert := &AlertForInvestigation{
		id:       "alert-timeout-test",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	result, err := uc.HandleAlert(ctx, alert)
	// Should timeout or complete quickly
	if err != nil {
		t.Logf("HandleAlert() with timeout error = %v (expected for timeout)", err)
	}
	if result != nil && result.Status == "running" {
		t.Error("HandleAlert() should not still be running after timeout")
	}
}

// =============================================================================
// Investigation Store Integration Tests (Phase 4)
// These tests verify that AlertInvestigationUseCase integrates with InvestigationStore.
// =============================================================================

func TestAlertInvestigationUseCase_SetInvestigationStore(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	store := NewMockInvestigationStore()
	if store == nil {
		t.Skip("NewMockInvestigationStore() returned nil")
	}

	// Should not panic
	uc.SetInvestigationStore(store)
}

func TestAlertInvestigationUseCase_HandleAlert_WithStore_PersistsInvestigation(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}
	uc.SetConversationService(newInvestigationRunnerConvServiceMock())
	uc.SetToolExecutor(newInvestigationRunnerToolExecutorMock())
	uc.SetPromptBuilderRegistry(newInvestigationRunnerPromptBuilderMock())

	store := NewMockInvestigationStore()
	if store == nil {
		t.Skip("NewMockInvestigationStore() returned nil")
	}
	uc.SetInvestigationStore(store)

	alert := &AlertForInvestigation{
		id:       "alert-persist-test",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	result, err := uc.HandleAlert(context.Background(), alert)
	if err != nil {
		t.Fatalf("HandleAlert() error = %v", err)
	}

	// Verify the investigation was persisted to the store
	stored, err := store.Get(context.Background(), result.InvestigationID)
	if err != nil {
		t.Errorf("Store.Get() error = %v, investigation should be persisted", err)
	}
	if stored == nil {
		t.Error("Store.Get() returned nil, investigation should be persisted")
	}
	if stored != nil && stored.AlertID() != "alert-persist-test" {
		t.Errorf("Stored investigation AlertID = %v, want alert-persist-test", stored.AlertID())
	}
}

func TestAlertInvestigationUseCase_HandleAlert_WithStore_UpdatesStatus(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}
	uc.SetConversationService(newInvestigationRunnerConvServiceMock())
	uc.SetToolExecutor(newInvestigationRunnerToolExecutorMock())
	uc.SetPromptBuilderRegistry(newInvestigationRunnerPromptBuilderMock())

	store := NewMockInvestigationStore()
	if store == nil {
		t.Skip("NewMockInvestigationStore() returned nil")
	}
	uc.SetInvestigationStore(store)

	alert := &AlertForInvestigation{
		id:       "alert-status-update",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	result, err := uc.HandleAlert(context.Background(), alert)
	if err != nil {
		t.Fatalf("HandleAlert() error = %v", err)
	}

	// Verify the final status was updated in the store
	stored, err := store.Get(context.Background(), result.InvestigationID)
	if err != nil {
		t.Fatalf("Store.Get() error = %v", err)
	}

	// Status should match the result status
	if stored != nil && stored.Status() != result.Status {
		t.Errorf("Stored status = %v, result status = %v, should match", stored.Status(), result.Status)
	}
}

func TestAlertInvestigationUseCase_StartInvestigation_WithStore_PersistsInitialState(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	store := NewMockInvestigationStore()
	if store == nil {
		t.Skip("NewMockInvestigationStore() returned nil")
	}
	uc.SetInvestigationStore(store)

	alert := &AlertForInvestigation{
		id:       "alert-initial-state",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	invID, err := uc.StartInvestigation(context.Background(), alert)
	if err != nil {
		t.Fatalf("StartInvestigation() error = %v", err)
	}

	// Verify investigation was stored with "started" or "running" status
	stored, err := store.Get(context.Background(), invID)
	if err != nil {
		t.Fatalf("Store.Get() error = %v", err)
	}

	if stored == nil {
		t.Fatal("Store.Get() returned nil")
	}

	status := stored.Status()
	if status != "started" && status != "running" {
		t.Errorf("Initial status = %v, want 'started' or 'running'", status)
	}
}

func TestAlertInvestigationUseCase_StopInvestigation_WithStore_UpdatesStatus(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	store := NewMockInvestigationStore()
	if store == nil {
		t.Skip("NewMockInvestigationStore() returned nil")
	}
	uc.SetInvestigationStore(store)

	alert := &AlertForInvestigation{
		id:       "alert-stop-update",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
	}

	invID, err := uc.StartInvestigation(context.Background(), alert)
	if err != nil {
		t.Fatalf("StartInvestigation() error = %v", err)
	}

	err = uc.StopInvestigation(context.Background(), invID)
	if err != nil {
		t.Fatalf("StopInvestigation() error = %v", err)
	}

	// Verify status was updated to stopped/cancelled
	stored, err := store.Get(context.Background(), invID)
	if err != nil {
		t.Fatalf("Store.Get() error = %v", err)
	}

	if stored == nil {
		t.Fatal("Store.Get() returned nil")
	}

	status := stored.Status()
	if status != "stopped" && status != "cancelled" && status != "completed" {
		t.Errorf("Status after stop = %v, want 'stopped', 'cancelled', or 'completed'", status)
	}
}

// =============================================================================
// RunInvestigation Cleanup Tests
// These tests verify that RunInvestigation properly cleans up tracking maps
// after completion, preventing memory leaks and duplicate investigation errors.
// =============================================================================

func TestAlertInvestigationUseCase_RunInvestigation_CleansUpTrackingOnSuccess(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-cleanup-success",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert for Cleanup",
	}

	// Start the investigation
	invID, err := uc.StartInvestigation(context.Background(), alert)
	if err != nil {
		t.Fatalf("StartInvestigation() error = %v", err)
	}

	// Verify investigation is tracked
	if uc.GetActiveCount() != 1 {
		t.Fatalf("GetActiveCount() after start = %v, want 1", uc.GetActiveCount())
	}

	// Run the investigation (which should complete)
	_, err = uc.RunInvestigation(context.Background(), alert, invID)
	if err != nil {
		t.Logf("RunInvestigation() error = %v (may be acceptable)", err)
	}

	// CRITICAL: After RunInvestigation completes, active count should be 0
	if uc.GetActiveCount() != 0 {
		t.Errorf(
			"GetActiveCount() after RunInvestigation = %v, want 0 (investigation should be cleaned up)",
			uc.GetActiveCount(),
		)
	}
}

func TestAlertInvestigationUseCase_RunInvestigation_AllowsNewInvestigationAfterCompletion(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-restart-test",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert for Restart",
	}

	// First investigation
	invID1, err := uc.StartInvestigation(context.Background(), alert)
	if err != nil {
		t.Fatalf("First StartInvestigation() error = %v", err)
	}

	_, err = uc.RunInvestigation(context.Background(), alert, invID1)
	if err != nil {
		t.Logf("First RunInvestigation() error = %v (may be acceptable)", err)
	}

	// CRITICAL: After first investigation completes, we should be able to start a new one for the same alert
	invID2, err := uc.StartInvestigation(context.Background(), alert)
	if err != nil {
		t.Errorf(
			"Second StartInvestigation() error = %v, want nil (should allow new investigation after first completes)",
			err,
		)
	}
	if invID2 == "" {
		t.Error("Second StartInvestigation() returned empty ID, should return valid investigation ID")
	}
	if invID2 == invID1 {
		t.Error("Second investigation ID should be different from first")
	}
}

func TestAlertInvestigationUseCase_RunInvestigation_CleansUpTrackingOnError(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	// Create a mock safety enforcer that blocks all tools to force failure/escalation
	enforcer := NewMockSafetyEnforcerWithBlockedTools([]string{"bash", "read_file", "list_files"})
	if enforcer != nil {
		uc.SetSafetyEnforcer(enforcer)
	}

	alert := &AlertForInvestigation{
		id:       "alert-cleanup-error",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert for Error Cleanup",
	}

	invID, err := uc.StartInvestigation(context.Background(), alert)
	if err != nil {
		t.Fatalf("StartInvestigation() error = %v", err)
	}

	// Verify investigation is tracked
	if uc.GetActiveCount() != 1 {
		t.Fatalf("GetActiveCount() after start = %v, want 1", uc.GetActiveCount())
	}

	// Run investigation (which should fail or escalate due to blocked tools)
	result, runErr := uc.RunInvestigation(context.Background(), alert, invID)
	// Error is acceptable here
	if runErr != nil {
		t.Logf("RunInvestigation() error = %v (acceptable for blocked tools test)", runErr)
	}
	if result != nil && result.Status != "failed" && !result.Escalated {
		t.Logf("RunInvestigation() status = %v, escalated = %v", result.Status, result.Escalated)
	}

	// CRITICAL: Even on error/escalation, active count should be 0
	if uc.GetActiveCount() != 0 {
		t.Errorf(
			"GetActiveCount() after RunInvestigation error = %v, want 0 (investigation should be cleaned up even on failure)",
			uc.GetActiveCount(),
		)
	}
}

func TestAlertInvestigationUseCase_RunInvestigation_CleansUpTrackingOnTimeout(t *testing.T) {
	config := AlertInvestigationUseCaseConfig{
		MaxDuration:   1 * time.Millisecond, // Very short timeout to force timeout
		MaxConcurrent: 5,
		AllowedTools:  []string{"bash", "read_file"},
	}

	uc := NewAlertInvestigationUseCaseWithConfig(config)
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCaseWithConfig() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-cleanup-timeout",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert for Timeout Cleanup",
	}

	invID, startErr := uc.StartInvestigation(context.Background(), alert)
	if startErr != nil {
		t.Fatalf("StartInvestigation() error = %v", startErr)
	}

	// Verify investigation is tracked
	if uc.GetActiveCount() != 1 {
		t.Fatalf("GetActiveCount() after start = %v, want 1", uc.GetActiveCount())
	}

	// Run investigation with very short timeout context
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	result, runErr := uc.RunInvestigation(ctx, alert, invID)
	// Timeout error is expected and acceptable
	if runErr != nil {
		t.Logf("RunInvestigation() error = %v (expected for timeout test)", runErr)
	}
	if result != nil {
		t.Logf("RunInvestigation() status = %v", result.Status)
	}

	// CRITICAL: Even on timeout, active count should be 0
	if uc.GetActiveCount() != 0 {
		t.Errorf(
			"GetActiveCount() after RunInvestigation timeout = %v, want 0 (investigation should be cleaned up even on timeout)",
			uc.GetActiveCount(),
		)
	}
}

func TestAlertInvestigationUseCase_HandleAlert_CleansUpTrackingOnCompletion(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-handlealert-cleanup",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert for HandleAlert Cleanup",
	}

	// HandleAlert should start, run, and cleanup the investigation
	_, err := uc.HandleAlert(context.Background(), alert)
	if err != nil {
		t.Logf("HandleAlert() error = %v (may be acceptable)", err)
	}

	// CRITICAL: After HandleAlert completes, active count should be 0
	if uc.GetActiveCount() != 0 {
		t.Errorf(
			"GetActiveCount() after HandleAlert = %v, want 0 (investigation should be cleaned up)",
			uc.GetActiveCount(),
		)
	}
}

func TestAlertInvestigationUseCase_HandleAlert_AllowsConsecutiveInvestigations(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-consecutive",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert for Consecutive Investigations",
	}

	// First investigation via HandleAlert
	_, err := uc.HandleAlert(context.Background(), alert)
	if err != nil {
		t.Logf("First HandleAlert() error = %v (may be acceptable)", err)
	}

	// CRITICAL: Second investigation should succeed (no "already running" error)
	_, err = uc.HandleAlert(context.Background(), alert)
	if err != nil {
		// If we get ErrInvestigationAlreadyRunning, that means cleanup didn't happen
		if errors.Is(err, ErrInvestigationAlreadyRunning) {
			t.Errorf(
				"Second HandleAlert() error = %v, should not be 'already running' error (first investigation should have been cleaned up)",
				err,
			)
		} else {
			t.Logf("Second HandleAlert() error = %v (acceptable if not 'already running' error)", err)
		}
	}
}

func TestAlertInvestigationUseCase_RunInvestigation_RemovesAlertFromTracking(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-tracking-removal",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert for Tracking Removal",
	}

	invID, err := uc.StartInvestigation(context.Background(), alert)
	if err != nil {
		t.Fatalf("StartInvestigation() error = %v", err)
	}

	// Verify we can't start duplicate while running
	_, err = uc.StartInvestigation(context.Background(), alert)
	if err == nil {
		t.Error("Second StartInvestigation() should fail with 'already running' while first is active")
	}
	if err != nil && !errors.Is(err, ErrInvestigationAlreadyRunning) {
		t.Errorf("Second StartInvestigation() error = %v, want ErrInvestigationAlreadyRunning", err)
	}

	// Run the investigation
	_, err = uc.RunInvestigation(context.Background(), alert, invID)
	if err != nil {
		t.Logf("RunInvestigation() error = %v (may be acceptable)", err)
	}

	// CRITICAL: After RunInvestigation completes, we should be able to start a new investigation
	// This verifies the alert was removed from alertToInvestigation map
	invID2, err := uc.StartInvestigation(context.Background(), alert)
	if err != nil {
		if errors.Is(err, ErrInvestigationAlreadyRunning) {
			t.Errorf(
				"Third StartInvestigation() error = %v, should not be 'already running' (alert should be removed from tracking)",
				err,
			)
		} else {
			t.Logf("Third StartInvestigation() error = %v (acceptable if not 'already running' error)", err)
		}
	}
	if invID2 == "" {
		t.Error("Third StartInvestigation() returned empty ID after cleanup")
	}
}

// =============================================================================
// Combined Safety Enforcer + Store Integration Tests
// =============================================================================

func TestAlertInvestigationUseCase_WithEnforcerAndStore_Integration(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	store := NewMockInvestigationStore()
	if store == nil {
		t.Skip("NewMockInvestigationStore() returned nil")
	}
	uc.SetInvestigationStore(store)

	enforcer := NewMockSafetyEnforcer()
	if enforcer == nil {
		t.Skip("NewMockSafetyEnforcer() returned nil")
	}
	uc.SetSafetyEnforcer(enforcer)

	alert := &AlertForInvestigation{
		id:       "alert-full-integration",
		source:   "prometheus",
		severity: "critical",
		title:    "Full Integration Test",
	}

	result, err := uc.HandleAlert(context.Background(), alert)
	if err != nil {
		t.Logf("HandleAlert() error = %v (may be expected)", err)
	}

	if result != nil {
		// Verify both enforcer and store were used
		stored, storeErr := store.Get(context.Background(), result.InvestigationID)
		if storeErr != nil {
			t.Errorf("Store.Get() error = %v", storeErr)
		}
		if stored != nil {
			t.Logf("Investigation persisted with status: %v", stored.Status())
		}
	}
}

// =============================================================================
// Mock Types for Testing (These will be implemented in GREEN phase)
// =============================================================================

// MockSafetyEnforcer is a test double for SafetyEnforcer interface.
// It will be created in the GREEN phase.
// func NewMockSafetyEnforcer() *MockSafetyEnforcer
// func NewMockSafetyEnforcerWithBlockedTools(tools []string) *MockSafetyEnforcer
// func NewMockSafetyEnforcerWithBlockedCommands(commands []string) *MockSafetyEnforcer
// func NewMockSafetyEnforcerWithActionBudget(budget int) *MockSafetyEnforcer

// MockInvestigationStore is a test double for InvestigationStore interface.
// It will be created in the GREEN phase.
// func NewMockInvestigationStore() *MockInvestigationStore

// =============================================================================
// ConversationServiceInterface Thinking Mode Tests (RED PHASE - Step 11)
// These tests verify that ConversationServiceInterface includes thinking mode methods.
// =============================================================================

// TestConversationServiceInterfaceHasThinkingMethods verifies that the
// ConversationServiceInterface includes methods for managing thinking mode.
// This test ensures the interface is complete for use by SubagentRunner.
func TestConversationServiceInterfaceHasThinkingMethods(t *testing.T) {
	t.Run("interface has SetThinkingMode method", func(t *testing.T) {
		// This test will fail if SetThinkingMode is not in the interface
		// because mockConversationServiceWithThinking won't satisfy the interface
		var _ ConversationServiceInterface = &mockConversationServiceWithThinking{}
	})

	t.Run("interface has GetThinkingMode method", func(t *testing.T) {
		// This test will fail if GetThinkingMode is not in the interface
		// because mockConversationServiceWithThinking won't satisfy the interface
		var _ ConversationServiceInterface = &mockConversationServiceWithThinking{}
	})
}

// TestConversationServiceInterfaceThinkingMethodSignatures verifies that the
// thinking mode methods have the correct signatures.
func TestConversationServiceInterfaceThinkingMethodSignatures(t *testing.T) {
	ctx := context.Background()
	mock := &mockConversationServiceWithThinking{
		thinkingModes: make(map[string]port.ThinkingModeInfo),
	}

	// Test SetThinkingMode signature
	t.Run("SetThinkingMode accepts sessionID and ThinkingModeInfo", func(t *testing.T) {
		sessionID := "test-session"
		info := port.ThinkingModeInfo{
			Enabled:      true,
			BudgetTokens: 1000,
			ShowThinking: true,
		}

		err := mock.SetThinkingMode(sessionID, info)
		if err != nil {
			t.Errorf("SetThinkingMode returned unexpected error: %v", err)
		}
	})

	// Test GetThinkingMode signature
	t.Run("GetThinkingMode accepts sessionID and returns ThinkingModeInfo", func(t *testing.T) {
		sessionID := "test-session"
		info := port.ThinkingModeInfo{
			Enabled:      true,
			BudgetTokens: 2000,
			ShowThinking: false,
		}

		// Set the thinking mode first
		_ = mock.SetThinkingMode(sessionID, info)

		// Get the thinking mode
		retrieved, err := mock.GetThinkingMode(sessionID)
		if err != nil {
			t.Errorf("GetThinkingMode returned unexpected error: %v", err)
		}

		// Verify the retrieved value matches what was set
		if retrieved.Enabled != info.Enabled {
			t.Errorf("GetThinkingMode Enabled = %v, want %v", retrieved.Enabled, info.Enabled)
		}
		if retrieved.BudgetTokens != info.BudgetTokens {
			t.Errorf("GetThinkingMode BudgetTokens = %v, want %v", retrieved.BudgetTokens, info.BudgetTokens)
		}
		if retrieved.ShowThinking != info.ShowThinking {
			t.Errorf("GetThinkingMode ShowThinking = %v, want %v", retrieved.ShowThinking, info.ShowThinking)
		}
	})

	// Verify methods are accessible through the interface
	t.Run("thinking mode methods accessible through interface", func(t *testing.T) {
		var iface ConversationServiceInterface = mock
		sessionID := "interface-test"
		info := port.ThinkingModeInfo{
			Enabled:      false,
			BudgetTokens: 500,
			ShowThinking: true,
		}

		// This will fail if methods aren't in the interface
		err := iface.SetThinkingMode(sessionID, info)
		if err != nil {
			t.Errorf("SetThinkingMode through interface returned error: %v", err)
		}

		retrieved, err := iface.GetThinkingMode(sessionID)
		if err != nil {
			t.Errorf("GetThinkingMode through interface returned error: %v", err)
		}

		if retrieved.Enabled != info.Enabled {
			t.Errorf("Retrieved Enabled = %v, want %v", retrieved.Enabled, info.Enabled)
		}
	})

	// Ensure context is not used in these methods (unlike SetCustomSystemPrompt)
	t.Run("thinking mode methods do not require context", func(t *testing.T) {
		// SetThinkingMode and GetThinkingMode should NOT take context
		// (unlike SetCustomSystemPrompt which does)
		sessionID := "no-ctx-test"
		info := port.ThinkingModeInfo{
			Enabled:      true,
			BudgetTokens: 100,
			ShowThinking: false,
		}

		// Should work without passing context
		_ = mock.SetThinkingMode(sessionID, info)
		_, _ = mock.GetThinkingMode(sessionID)

		// For comparison: SetCustomSystemPrompt requires context
		_ = mock.SetCustomSystemPrompt(ctx, sessionID, "test prompt")
	})
}

// TestThinkingModeMethodsBehavior tests the expected behavior of thinking mode methods.
func TestThinkingModeMethodsBehavior(t *testing.T) {
	tests := []struct {
		name      string
		sessionID string
		info      port.ThinkingModeInfo
		wantErr   bool
	}{
		{
			name:      "enabled with budget",
			sessionID: "session-1",
			info: port.ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 5000,
				ShowThinking: true,
			},
			wantErr: false,
		},
		{
			name:      "disabled with zero budget",
			sessionID: "session-2",
			info: port.ThinkingModeInfo{
				Enabled:      false,
				BudgetTokens: 0,
				ShowThinking: false,
			},
			wantErr: false,
		},
		{
			name:      "enabled without showing thinking",
			sessionID: "session-3",
			info: port.ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 10000,
				ShowThinking: false,
			},
			wantErr: false,
		},
		{
			name:      "enabled with showing thinking but low budget",
			sessionID: "session-4",
			info: port.ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 100,
				ShowThinking: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockConversationServiceWithThinking{
				thinkingModes: make(map[string]port.ThinkingModeInfo),
			}

			// Set thinking mode
			err := mock.SetThinkingMode(tt.sessionID, tt.info)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetThinkingMode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Get thinking mode
			retrieved, err := mock.GetThinkingMode(tt.sessionID)
			if err != nil {
				t.Errorf("GetThinkingMode() unexpected error = %v", err)
				return
			}

			// Verify all fields match
			if retrieved.Enabled != tt.info.Enabled {
				t.Errorf("Enabled = %v, want %v", retrieved.Enabled, tt.info.Enabled)
			}
			if retrieved.BudgetTokens != tt.info.BudgetTokens {
				t.Errorf("BudgetTokens = %v, want %v", retrieved.BudgetTokens, tt.info.BudgetTokens)
			}
			if retrieved.ShowThinking != tt.info.ShowThinking {
				t.Errorf("ShowThinking = %v, want %v", retrieved.ShowThinking, tt.info.ShowThinking)
			}
		})
	}
}

// TestThinkingModeIsolation verifies that thinking mode settings are isolated per session.
func TestThinkingModeIsolation(t *testing.T) {
	mock := &mockConversationServiceWithThinking{
		thinkingModes: make(map[string]port.ThinkingModeInfo),
	}

	session1 := "session-1"
	info1 := port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 1000,
		ShowThinking: true,
	}

	session2 := "session-2"
	info2 := port.ThinkingModeInfo{
		Enabled:      false,
		BudgetTokens: 500,
		ShowThinking: false,
	}

	// Set different thinking modes for different sessions
	_ = mock.SetThinkingMode(session1, info1)
	_ = mock.SetThinkingMode(session2, info2)

	// Retrieve and verify each session has its own settings
	retrieved1, err := mock.GetThinkingMode(session1)
	if err != nil {
		t.Fatalf("GetThinkingMode(session1) error = %v", err)
	}

	retrieved2, err := mock.GetThinkingMode(session2)
	if err != nil {
		t.Fatalf("GetThinkingMode(session2) error = %v", err)
	}

	// Verify session 1 settings
	if retrieved1.Enabled != info1.Enabled || retrieved1.BudgetTokens != info1.BudgetTokens {
		t.Errorf("Session 1 thinking mode not isolated: got %+v, want %+v", retrieved1, info1)
	}

	// Verify session 2 settings
	if retrieved2.Enabled != info2.Enabled || retrieved2.BudgetTokens != info2.BudgetTokens {
		t.Errorf("Session 2 thinking mode not isolated: got %+v, want %+v", retrieved2, info2)
	}
}

// TestThinkingModeUpdateBehavior verifies that thinking mode can be updated multiple times.
func TestThinkingModeUpdateBehavior(t *testing.T) {
	mock := &mockConversationServiceWithThinking{
		thinkingModes: make(map[string]port.ThinkingModeInfo),
	}

	sessionID := "update-test"

	// Initial setting
	initial := port.ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 1000,
		ShowThinking: true,
	}
	_ = mock.SetThinkingMode(sessionID, initial)

	// Update to different values
	updated := port.ThinkingModeInfo{
		Enabled:      false,
		BudgetTokens: 2000,
		ShowThinking: false,
	}
	_ = mock.SetThinkingMode(sessionID, updated)

	// Verify the update took effect
	retrieved, err := mock.GetThinkingMode(sessionID)
	if err != nil {
		t.Fatalf("GetThinkingMode error = %v", err)
	}

	if retrieved.Enabled != updated.Enabled {
		t.Errorf("After update: Enabled = %v, want %v", retrieved.Enabled, updated.Enabled)
	}
	if retrieved.BudgetTokens != updated.BudgetTokens {
		t.Errorf("After update: BudgetTokens = %v, want %v", retrieved.BudgetTokens, updated.BudgetTokens)
	}
	if retrieved.ShowThinking != updated.ShowThinking {
		t.Errorf("After update: ShowThinking = %v, want %v", retrieved.ShowThinking, updated.ShowThinking)
	}
}

// TestGetThinkingModeDefaultBehavior verifies behavior when getting thinking mode
// for a session where it was never set.
func TestGetThinkingModeDefaultBehavior(t *testing.T) {
	mock := &mockConversationServiceWithThinking{
		thinkingModes: make(map[string]port.ThinkingModeInfo),
	}

	sessionID := "never-set"

	// Get thinking mode for a session that never had it set
	retrieved, err := mock.GetThinkingMode(sessionID)
	// Should not error (similar to ConversationService behavior)
	if err != nil {
		t.Errorf("GetThinkingMode for unset session returned error: %v", err)
	}

	// Should return zero-value ThinkingModeInfo
	var zero port.ThinkingModeInfo
	if retrieved != zero {
		t.Errorf("GetThinkingMode for unset session = %+v, want zero value %+v", retrieved, zero)
	}
}

// =============================================================================
// Mock Implementation with Thinking Mode Methods
// =============================================================================

// mockConversationServiceWithThinking is a mock that implements
// ConversationServiceInterface including the thinking mode methods.
// This mock will fail to compile if the interface doesn't have these methods.
type mockConversationServiceWithThinking struct {
	thinkingModes map[string]port.ThinkingModeInfo
}

func (m *mockConversationServiceWithThinking) StartConversation(ctx context.Context) (string, error) {
	return "mock-session", nil
}

func (m *mockConversationServiceWithThinking) AddUserMessage(
	ctx context.Context,
	sessionID, content string,
) (*entity.Message, error) {
	return nil, nil
}

func (m *mockConversationServiceWithThinking) ProcessAssistantResponse(
	ctx context.Context,
	sessionID string,
) (*entity.Message, []port.ToolCallInfo, error) {
	return nil, nil, nil
}

func (m *mockConversationServiceWithThinking) AddToolResultMessage(
	ctx context.Context,
	sessionID string,
	toolResults []entity.ToolResult,
) error {
	return nil
}

func (m *mockConversationServiceWithThinking) EndConversation(ctx context.Context, sessionID string) error {
	return nil
}

func (m *mockConversationServiceWithThinking) SetCustomSystemPrompt(
	ctx context.Context,
	sessionID, prompt string,
) error {
	return nil
}

// SetThinkingMode sets the thinking mode for a session.
// This method MUST be in ConversationServiceInterface for the mock to compile.
func (m *mockConversationServiceWithThinking) SetThinkingMode(sessionID string, info port.ThinkingModeInfo) error {
	m.thinkingModes[sessionID] = info
	return nil
}

// GetThinkingMode gets the thinking mode for a session.
// This method MUST be in ConversationServiceInterface for the mock to compile.
func (m *mockConversationServiceWithThinking) GetThinkingMode(sessionID string) (port.ThinkingModeInfo, error) {
	info, exists := m.thinkingModes[sessionID]
	if !exists {
		// Return zero value if not set (matches ConversationService behavior)
		return port.ThinkingModeInfo{}, nil
	}
	return info, nil
}

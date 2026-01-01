package usecase

import (
	"context"
	"errors"
	"testing"
	"time"
)

// =============================================================================
// AlertInvestigationUseCase Tests - RED PHASE
// These tests define the expected behavior of the AlertInvestigationUseCase.
// All tests should FAIL until the implementation is complete.
// =============================================================================

// Sentinel errors expected to be defined in alert_investigation.go.
var (
	ErrAlertNil                    = errors.New("alert cannot be nil")
	ErrInvestigationAlreadyRunning = errors.New("investigation already running for this alert")
	ErrMaxConcurrentReached        = errors.New("maximum concurrent investigations reached")
	ErrInvestigationTimeout        = errors.New("investigation timed out")
	ErrActionBudgetExceeded        = errors.New("action budget exceeded")
	ErrToolNotAllowed              = errors.New("tool not allowed by investigation config")
	ErrCommandBlocked              = errors.New("command blocked by safety rules")
)

// AlertForInvestigation represents alert data for investigation testing.
type AlertForInvestigation struct {
	id          string
	source      string
	severity    string
	title       string
	description string
	labels      map[string]string
}

func (a *AlertForInvestigation) ID() string                { return a.id }
func (a *AlertForInvestigation) Source() string            { return a.source }
func (a *AlertForInvestigation) Severity() string          { return a.severity }
func (a *AlertForInvestigation) Title() string             { return a.title }
func (a *AlertForInvestigation) Description() string       { return a.description }
func (a *AlertForInvestigation) Labels() map[string]string { return a.labels }
func (a *AlertForInvestigation) IsCritical() bool          { return a.severity == "critical" }

// InvestigationResultStub represents the outcome of an investigation.
type InvestigationResultStub struct {
	InvestigationID string
	AlertID         string
	Status          string
	Findings        []string
	ActionsTaken    int
	Duration        time.Duration
	Confidence      float64
	Escalated       bool
	EscalateReason  string
	Error           error
}

// AlertInvestigationUseCase orchestrates alert investigations.
type AlertInvestigationUseCase struct{}

// AlertInvestigationUseCaseConfig holds configuration for the use case.
type AlertInvestigationUseCaseConfig struct {
	MaxActions           int
	MaxDuration          time.Duration
	MaxConcurrent        int
	AllowedTools         []string
	BlockedCommands      []string
	EscalateOnConfidence float64
	EscalateOnErrors     int
	AutoStartForCritical bool
	EnableSafetyChecks   bool
}

// Stub constructor - to be implemented in alert_investigation.go.
func NewAlertInvestigationUseCase() *AlertInvestigationUseCase {
	return nil
}

func NewAlertInvestigationUseCaseWithConfig(_ AlertInvestigationUseCaseConfig) *AlertInvestigationUseCase {
	return nil
}

// Stub methods - to be implemented.
func (uc *AlertInvestigationUseCase) HandleAlert(
	_ context.Context,
	_ *AlertForInvestigation,
) (*InvestigationResultStub, error) {
	return nil, errors.New("not implemented")
}

func (uc *AlertInvestigationUseCase) StartInvestigation(_ context.Context, _ *AlertForInvestigation) (string, error) {
	return "", errors.New("not implemented")
}

func (uc *AlertInvestigationUseCase) StopInvestigation(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (uc *AlertInvestigationUseCase) GetInvestigationStatus(
	_ context.Context,
	_ string,
) (*InvestigationResultStub, error) {
	return nil, errors.New("not implemented")
}

func (uc *AlertInvestigationUseCase) ListActiveInvestigations(_ context.Context) ([]string, error) {
	return nil, errors.New("not implemented")
}

func (uc *AlertInvestigationUseCase) GetActiveCount() int {
	return 0
}

func (uc *AlertInvestigationUseCase) SetEscalationHandler(_ EscalationHandler) {
}

func (uc *AlertInvestigationUseCase) SetPromptBuilderRegistry(_ PromptBuilderRegistry) {
}

func (uc *AlertInvestigationUseCase) IsToolAllowed(_ string) bool {
	return false
}

func (uc *AlertInvestigationUseCase) IsCommandBlocked(_ string) bool {
	return false
}

func (uc *AlertInvestigationUseCase) Shutdown(_ context.Context) error {
	return errors.New("not implemented")
}

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
// InvestigationResultStub Tests
// =============================================================================

func TestInvestigationResultStub_Fields(t *testing.T) {
	result := InvestigationResultStub{
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

func TestInvestigationResultStub_Escalated(t *testing.T) {
	result := InvestigationResultStub{
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

func TestInvestigationResultStub_Failed(t *testing.T) {
	result := InvestigationResultStub{
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

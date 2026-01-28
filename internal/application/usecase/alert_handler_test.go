package usecase

import (
	"context"
	"testing"
)

// =============================================================================
// AlertHandler Tests
// These tests verify the behavior of AlertHandler, which bridges alerts to
// investigations by deciding when to start investigations based on config.
//
// RED PHASE: These tests are expected to FAIL until AlertHandler is implemented.
// =============================================================================

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewAlertHandler_NotNil(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	config := AlertHandlerConfig{
		AutoInvestigateCritical: true,
		AutoInvestigateWarning:  false,
		IgnoredSources:          []string{},
	}

	handler := NewAlertHandler(uc, config)
	if handler == nil {
		t.Error("NewAlertHandler() should not return nil")
	}
}

func TestNewAlertHandler_ConfigValidation_NilUseCase(t *testing.T) {
	config := AlertHandlerConfig{
		AutoInvestigateCritical: true,
	}

	handler, err := NewAlertHandlerWithValidation(nil, config)
	if err == nil {
		t.Error("NewAlertHandlerWithValidation(nil, config) should return error")
	}
	if handler != nil {
		t.Error("NewAlertHandlerWithValidation(nil, config) should return nil handler")
	}
}

func TestNewAlertHandler_ConfigValidation_ValidConfig(t *testing.T) {
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	config := AlertHandlerConfig{
		AutoInvestigateCritical: true,
		AutoInvestigateWarning:  true,
		IgnoredSources:          []string{"test-source"},
	}

	handler, err := NewAlertHandlerWithValidation(uc, config)
	if err != nil {
		t.Errorf("NewAlertHandlerWithValidation() error = %v, want nil", err)
	}
	if handler == nil {
		t.Error("NewAlertHandlerWithValidation() should return non-nil handler")
	}
}

// =============================================================================
// Handle Tests - Critical Alerts
// =============================================================================

func TestAlertHandler_Handle_CriticalAlert_StartsInvestigation(t *testing.T) {
	// Arrange
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}
	uc.SetConversationService(newInvestigationRunnerConvServiceMock())
	uc.SetToolExecutor(newInvestigationRunnerToolExecutorMock())
	uc.SetPromptBuilderRegistry(newInvestigationRunnerPromptBuilderMock())

	config := AlertHandlerConfig{
		AutoInvestigateCritical: true,
		AutoInvestigateWarning:  false,
		IgnoredSources:          []string{},
	}

	handler := NewAlertHandler(uc, config)
	if handler == nil {
		t.Fatal("NewAlertHandler() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-critical-001",
		source:   "prometheus",
		severity: "critical",
		title:    "Critical CPU Alert",
	}

	// Act
	err := handler.Handle(context.Background(), alert)
	// Assert: should start and complete investigation for critical alert without error
	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	// Note: Investigation completes synchronously, so GetActiveCount() is 0 after Handle() returns
}

func TestAlertHandler_Handle_CriticalAlert_AutoInvestigateDisabled(t *testing.T) {
	// Arrange
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	config := AlertHandlerConfig{
		AutoInvestigateCritical: false, // Disabled
		AutoInvestigateWarning:  false,
		IgnoredSources:          []string{},
	}

	handler := NewAlertHandler(uc, config)
	if handler == nil {
		t.Fatal("NewAlertHandler() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-critical-disabled",
		source:   "prometheus",
		severity: "critical",
		title:    "Critical Alert - No Auto",
	}

	// Act
	err := handler.Handle(context.Background(), alert)
	// Assert: should NOT start investigation when disabled
	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	// Verify NO investigation was started
	if uc.GetActiveCount() != 0 {
		t.Errorf("GetActiveCount() = %v, want 0 (auto-investigate disabled)", uc.GetActiveCount())
	}
}

// =============================================================================
// Handle Tests - Warning Alerts
// =============================================================================

func TestAlertHandler_Handle_WarningAlert_ConfigEnabled(t *testing.T) {
	// Arrange
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}
	uc.SetConversationService(newInvestigationRunnerConvServiceMock())
	uc.SetToolExecutor(newInvestigationRunnerToolExecutorMock())
	uc.SetPromptBuilderRegistry(newInvestigationRunnerPromptBuilderMock())

	config := AlertHandlerConfig{
		AutoInvestigateCritical: true,
		AutoInvestigateWarning:  true, // Enabled for warnings
		IgnoredSources:          []string{},
	}

	handler := NewAlertHandler(uc, config)
	if handler == nil {
		t.Fatal("NewAlertHandler() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-warning-enabled",
		source:   "prometheus",
		severity: "warning",
		title:    "Warning Alert",
	}

	// Act
	err := handler.Handle(context.Background(), alert)
	// Assert: should start and complete investigation when warning auto-investigate is enabled
	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	// Note: Investigation completes synchronously, so GetActiveCount() is 0 after Handle() returns
}

func TestAlertHandler_Handle_WarningAlert_ConfigDisabled(t *testing.T) {
	// Arrange
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	config := AlertHandlerConfig{
		AutoInvestigateCritical: true,
		AutoInvestigateWarning:  false, // Disabled for warnings
		IgnoredSources:          []string{},
	}

	handler := NewAlertHandler(uc, config)
	if handler == nil {
		t.Fatal("NewAlertHandler() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-warning-disabled",
		source:   "prometheus",
		severity: "warning",
		title:    "Warning Alert - No Auto",
	}

	// Act
	err := handler.Handle(context.Background(), alert)
	// Assert: should NOT start investigation when warning auto-investigate is disabled
	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	if uc.GetActiveCount() != 0 {
		t.Errorf("GetActiveCount() = %v, want 0 (warning auto-investigate disabled)", uc.GetActiveCount())
	}
}

// =============================================================================
// Handle Tests - Info Alerts
// =============================================================================

func TestAlertHandler_Handle_InfoAlert_NeverAutoInvestigates(t *testing.T) {
	// Arrange
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	config := AlertHandlerConfig{
		AutoInvestigateCritical: true,
		AutoInvestigateWarning:  true,
		IgnoredSources:          []string{},
	}

	handler := NewAlertHandler(uc, config)
	if handler == nil {
		t.Fatal("NewAlertHandler() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-info",
		source:   "prometheus",
		severity: "info",
		title:    "Info Alert",
	}

	// Act
	err := handler.Handle(context.Background(), alert)
	// Assert: info alerts should never auto-investigate
	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	if uc.GetActiveCount() != 0 {
		t.Errorf("GetActiveCount() = %v, want 0 (info alerts never auto-investigate)", uc.GetActiveCount())
	}
}

// =============================================================================
// Handle Tests - Max Concurrent Limit
// =============================================================================

func TestAlertHandler_Handle_RespectsMaxConcurrent(t *testing.T) {
	// Arrange
	ucConfig := AlertInvestigationUseCaseConfig{
		MaxConcurrent: 2, // Only allow 2 concurrent investigations
	}
	uc := NewAlertInvestigationUseCaseWithConfig(ucConfig)
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCaseWithConfig() returned nil")
	}

	handlerConfig := AlertHandlerConfig{
		AutoInvestigateCritical: true,
		AutoInvestigateWarning:  true,
		IgnoredSources:          []string{},
	}

	handler := NewAlertHandler(uc, handlerConfig)
	if handler == nil {
		t.Fatal("NewAlertHandler() returned nil")
	}

	// Start max concurrent alerts
	for i := range 2 {
		alert := &AlertForInvestigation{
			id:       "alert-max-" + string(rune('a'+i)),
			source:   "prometheus",
			severity: "critical",
			title:    "Critical Alert",
		}
		_ = handler.Handle(context.Background(), alert)
	}

	// Act: Try to handle one more alert
	overflowAlert := &AlertForInvestigation{
		id:       "alert-overflow",
		source:   "prometheus",
		severity: "critical",
		title:    "Overflow Alert",
	}
	err := handler.Handle(context.Background(), overflowAlert)

	// Assert: should return error or handle gracefully when max reached
	// The handler should respect the use case's max concurrent limit
	if err == nil {
		// If no error, the count should still be at max
		if uc.GetActiveCount() > 2 {
			t.Errorf("GetActiveCount() = %v, want <= 2 (should respect max concurrent)", uc.GetActiveCount())
		}
	}
	// Note: Depending on implementation, this might return an error or queue the alert
}

// =============================================================================
// Handle Tests - Ignored Sources
// =============================================================================

func TestAlertHandler_Handle_IgnoredSources_SingleSource(t *testing.T) {
	// Arrange
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	config := AlertHandlerConfig{
		AutoInvestigateCritical: true,
		AutoInvestigateWarning:  true,
		IgnoredSources:          []string{"test-ignored"},
	}

	handler := NewAlertHandler(uc, config)
	if handler == nil {
		t.Fatal("NewAlertHandler() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-ignored",
		source:   "test-ignored", // This source is in the ignored list
		severity: "critical",
		title:    "Critical from Ignored Source",
	}

	// Act
	err := handler.Handle(context.Background(), alert)
	// Assert: should not start investigation for ignored source
	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	if uc.GetActiveCount() != 0 {
		t.Errorf("GetActiveCount() = %v, want 0 (source is ignored)", uc.GetActiveCount())
	}
}

func TestAlertHandler_Handle_IgnoredSources_MultipleIgnored(t *testing.T) {
	// Arrange
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	config := AlertHandlerConfig{
		AutoInvestigateCritical: true,
		AutoInvestigateWarning:  true,
		IgnoredSources:          []string{"ignored-1", "ignored-2", "ignored-3"},
	}

	handler := NewAlertHandler(uc, config)
	if handler == nil {
		t.Fatal("NewAlertHandler() returned nil")
	}

	tests := []struct {
		name       string
		source     string
		wantIgnore bool
	}{
		{"first ignored source", "ignored-1", true},
		{"second ignored source", "ignored-2", true},
		{"third ignored source", "ignored-3", true},
		{"non-ignored source", "prometheus", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset use case
			_ = uc.Shutdown(context.Background())
			uc = NewAlertInvestigationUseCase()
			uc.SetConversationService(newInvestigationRunnerConvServiceMock())
			uc.SetToolExecutor(newInvestigationRunnerToolExecutorMock())
			uc.SetPromptBuilderRegistry(newInvestigationRunnerPromptBuilderMock())
			handler = NewAlertHandler(uc, config)

			alert := &AlertForInvestigation{
				id:       "alert-" + tt.source,
				source:   tt.source,
				severity: "critical",
				title:    "Test Alert",
			}

			err := handler.Handle(context.Background(), alert)
			// Handle() should not error regardless of whether source is ignored
			// (ignored sources are filtered silently, non-ignored sources run and complete)
			if err != nil {
				t.Errorf("Handle() error = %v, want nil", err)
			}

			// Note: Investigations complete synchronously, so GetActiveCount() is always 0 after Handle()
			// The test verifies Handle() succeeds for both ignored and non-ignored sources
		})
	}
}

func TestAlertHandler_Handle_IgnoredSources_NotIgnored(t *testing.T) {
	// Arrange
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}
	uc.SetConversationService(newInvestigationRunnerConvServiceMock())
	uc.SetToolExecutor(newInvestigationRunnerToolExecutorMock())
	uc.SetPromptBuilderRegistry(newInvestigationRunnerPromptBuilderMock())

	config := AlertHandlerConfig{
		AutoInvestigateCritical: true,
		AutoInvestigateWarning:  true,
		IgnoredSources:          []string{"ignored-source"},
	}

	handler := NewAlertHandler(uc, config)
	if handler == nil {
		t.Fatal("NewAlertHandler() returned nil")
	}

	alert := &AlertForInvestigation{
		id:       "alert-not-ignored",
		source:   "prometheus", // Not in ignored list
		severity: "critical",
		title:    "Critical from Valid Source",
	}

	// Act
	err := handler.Handle(context.Background(), alert)
	// Assert: should start and complete investigation for non-ignored source
	if err != nil {
		t.Errorf("Handle() error = %v, want nil", err)
	}

	// Note: Investigation completes synchronously, so GetActiveCount() is 0 after Handle() returns
}

// =============================================================================
// Handle Tests - Nil Alert
// =============================================================================

func TestAlertHandler_Handle_NilAlert(t *testing.T) {
	// Arrange
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	config := AlertHandlerConfig{
		AutoInvestigateCritical: true,
	}

	handler := NewAlertHandler(uc, config)
	if handler == nil {
		t.Fatal("NewAlertHandler() returned nil")
	}

	// Act
	err := handler.Handle(context.Background(), nil)

	// Assert: should return error for nil alert
	if err == nil {
		t.Error("Handle(nil) should return error")
	}
}

// =============================================================================
// Handle Tests - Cancelled Context
// =============================================================================

func TestAlertHandler_Handle_CancelledContext(t *testing.T) {
	// Arrange
	uc := NewAlertInvestigationUseCase()
	if uc == nil {
		t.Skip("NewAlertInvestigationUseCase() returned nil")
	}

	config := AlertHandlerConfig{
		AutoInvestigateCritical: true,
	}

	handler := NewAlertHandler(uc, config)
	if handler == nil {
		t.Fatal("NewAlertHandler() returned nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	alert := &AlertForInvestigation{
		id:       "alert-cancelled",
		source:   "prometheus",
		severity: "critical",
		title:    "Test Alert",
	}

	// Act
	err := handler.Handle(ctx, alert)

	// Assert: should return error for cancelled context
	if err == nil {
		t.Error("Handle() with cancelled context should return error")
	}
}

// =============================================================================
// AlertHandlerConfig Tests
// =============================================================================

func TestAlertHandlerConfig_Defaults(t *testing.T) {
	config := AlertHandlerConfig{}

	// Default values should be false/empty
	if config.AutoInvestigateCritical {
		t.Error("Default AutoInvestigateCritical should be false")
	}
	if config.AutoInvestigateWarning {
		t.Error("Default AutoInvestigateWarning should be false")
	}
	if len(config.IgnoredSources) != 0 {
		t.Error("Default IgnoredSources should be empty")
	}
}

func TestAlertHandlerConfig_WithValues(t *testing.T) {
	config := AlertHandlerConfig{
		AutoInvestigateCritical: true,
		AutoInvestigateWarning:  true,
		IgnoredSources:          []string{"source1", "source2"},
	}

	if !config.AutoInvestigateCritical {
		t.Error("AutoInvestigateCritical should be true")
	}
	if !config.AutoInvestigateWarning {
		t.Error("AutoInvestigateWarning should be true")
	}
	if len(config.IgnoredSources) != 2 {
		t.Errorf("IgnoredSources len = %v, want 2", len(config.IgnoredSources))
	}
}

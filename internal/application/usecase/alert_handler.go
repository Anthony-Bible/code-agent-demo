package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"context"
	"errors"
)

// Alert severity constants for handler decisions.
const (
	severityWarning = "warning"
)

// ErrNilUseCase is returned when AlertHandler is created with a nil use case.
var ErrNilUseCase = errors.New("investigation use case cannot be nil")

// AlertHandlerConfig configures the alert handler behavior.
type AlertHandlerConfig struct {
	AutoInvestigateCritical bool     // Automatically investigate critical alerts
	AutoInvestigateWarning  bool     // Automatically investigate warning alerts
	IgnoredSources          []string // Sources to ignore (no auto-investigation)
}

// AlertHandler bridges incoming alerts to the investigation use case.
// It decides when to start investigations based on configuration.
type AlertHandler struct {
	investigationUseCase *AlertInvestigationUseCase
	config               AlertHandlerConfig
}

// NewAlertHandler creates a new AlertHandler with the given use case and config.
func NewAlertHandler(uc *AlertInvestigationUseCase, config AlertHandlerConfig) *AlertHandler {
	return &AlertHandler{
		investigationUseCase: uc,
		config:               config,
	}
}

// NewAlertHandlerWithValidation creates a new AlertHandler with validation.
// Returns an error if the use case is nil.
func NewAlertHandlerWithValidation(
	uc *AlertInvestigationUseCase,
	config AlertHandlerConfig,
) (*AlertHandler, error) {
	if uc == nil {
		return nil, ErrNilUseCase
	}
	return &AlertHandler{
		investigationUseCase: uc,
		config:               config,
	}, nil
}

// Handle processes an incoming alert and potentially starts an investigation.
// Returns an error if the alert is nil or if the context is cancelled.
func (h *AlertHandler) Handle(ctx context.Context, alert *AlertForInvestigation) error {
	if alert == nil {
		return ErrNilAlert
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// Check if source is ignored
	if h.isSourceIgnored(alert.Source()) {
		return nil
	}

	// Check if we should investigate based on severity and config
	if !h.shouldInvestigate(alert) {
		return nil
	}

	// Start the investigation
	_, err := h.investigationUseCase.HandleAlert(ctx, alert)
	return err
}

// isSourceIgnored checks if the alert source is in the ignored list.
func (h *AlertHandler) isSourceIgnored(source string) bool {
	for _, ignored := range h.config.IgnoredSources {
		if ignored == source {
			return true
		}
	}
	return false
}

// shouldInvestigate determines if an alert should trigger an investigation.
func (h *AlertHandler) shouldInvestigate(alert *AlertForInvestigation) bool {
	switch alert.Severity() {
	case string(EscalationPriorityCritical):
		return h.config.AutoInvestigateCritical
	case severityWarning:
		return h.config.AutoInvestigateWarning
	default:
		// Info and other severities never auto-investigate
		return false
	}
}

// HandleEntityAlert processes an entity.Alert and converts it for investigation.
// This method satisfies the port.AlertHandler signature.
func (h *AlertHandler) HandleEntityAlert(ctx context.Context, alert *entity.Alert) error {
	if alert == nil {
		return ErrNilAlert
	}
	// Convert entity.Alert to AlertForInvestigation
	invAlert := &AlertForInvestigation{
		id:          alert.ID(),
		source:      alert.Source(),
		severity:    alert.Severity(),
		title:       alert.Title(),
		description: alert.Description(),
		labels:      alert.Labels(),
	}
	return h.Handle(ctx, invAlert)
}

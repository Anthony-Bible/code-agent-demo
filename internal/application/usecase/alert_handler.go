// Package usecase contains application use cases that orchestrate domain logic.
// This file implements the AlertHandler which bridges alert ingestion to investigations.
package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"context"
	"errors"
)

// Alert severity constants used internally by the handler for decision making.
// These complement the domain severity constants with handler-specific values.
const (
	severityWarning = "warning"
)

// ErrNilUseCase is returned when AlertHandler is created with a nil use case.
var ErrNilUseCase = errors.New("investigation use case cannot be nil")

// AlertHandlerConfig configures the alert handler behavior.
// It determines which alerts trigger automatic investigations based on
// severity levels and source filters.
type AlertHandlerConfig struct {
	// AutoInvestigateCritical enables automatic investigation of critical severity alerts.
	// When true, critical alerts bypass manual review and start investigations immediately.
	AutoInvestigateCritical bool

	// AutoInvestigateWarning enables automatic investigation of warning severity alerts.
	// When true, warning alerts also trigger automatic investigations.
	// Info-level alerts never trigger automatic investigation regardless of this setting.
	AutoInvestigateWarning bool

	// IgnoredSources is a list of alert source names that should never trigger
	// automatic investigations, regardless of severity. Use this to filter out
	// noisy or low-priority alert sources.
	IgnoredSources []string
}

// AlertHandler bridges incoming alerts to the investigation use case.
// It implements the decision logic for when to start automatic investigations
// based on alert severity, source, and handler configuration.
//
// AlertHandler is designed to be used as a callback handler that receives alerts
// from various sources (webhooks, queues, etc.) and routes them appropriately.
// It is safe for concurrent use.
type AlertHandler struct {
	investigationUseCase *AlertInvestigationUseCase
	config               AlertHandlerConfig
}

// NewAlertHandler creates a new AlertHandler with the given use case and config.
//
// Note: This constructor does not validate the use case parameter.
// Use NewAlertHandlerWithValidation if nil checking is required.
func NewAlertHandler(uc *AlertInvestigationUseCase, config AlertHandlerConfig) *AlertHandler {
	return &AlertHandler{
		investigationUseCase: uc,
		config:               config,
	}
}

// NewAlertHandlerWithValidation creates a new AlertHandler with validation.
//
// This is the recommended constructor for production use as it validates
// that the use case dependency is not nil.
//
// Returns ErrNilUseCase if the use case is nil.
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
//
// The handler evaluates the alert against the configured rules:
//  1. Checks if the alert source is in the ignored list (returns nil if so)
//  2. Checks if the severity warrants investigation based on config
//  3. Starts an investigation if all checks pass
//
// Returns nil if the alert is silently ignored (source filtered or severity not configured).
// Returns ErrNilAlert if the alert is nil.
// Returns context.Canceled or context.DeadlineExceeded if the context is done.
// Returns any error from the underlying investigation use case.
func (h *AlertHandler) Handle(ctx context.Context, alert *AlertForInvestigation) error {
	if alert == nil {
		return ErrNilAlert
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// Check if source is ignored - silently skip these alerts
	if h.isSourceIgnored(alert.Source()) {
		return nil
	}

	// Check if we should investigate based on severity and config
	if !h.shouldInvestigate(alert) {
		return nil
	}

	// All checks passed - start the investigation
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
//
// This method adapts the domain entity.Alert to the use case's AlertForInvestigation
// type, enabling integration with the alert source infrastructure layer.
// It satisfies the port.AlertHandler function signature for use as a callback.
//
// Returns ErrNilAlert if the alert is nil.
// Returns any error from the Handle method.
func (h *AlertHandler) HandleEntityAlert(ctx context.Context, alert *entity.Alert) error {
	if alert == nil {
		return ErrNilAlert
	}
	// Convert domain entity to use case DTO for processing
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

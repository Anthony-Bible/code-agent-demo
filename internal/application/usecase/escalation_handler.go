package usecase

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Sentinel errors for escalation handler operations.
// These errors are returned when escalation attempts fail.
var (
	// ErrNilInvestigation is returned when Escalate is called with nil investigation.
	ErrNilInvestigation = errors.New("investigation cannot be nil")
	// ErrEscalationFailed is returned when the escalation operation fails.
	ErrEscalationFailed = errors.New("escalation failed")
	// ErrNoEscalationTarget is returned when no escalation target is configured.
	ErrNoEscalationTarget = errors.New("no escalation target configured")
	// ErrEscalationAlreadySent is returned when trying to escalate an already-escalated investigation.
	ErrEscalationAlreadySent = errors.New("escalation already sent for this investigation")
	// ErrEscalationRateLimited is returned when escalation rate limit is exceeded.
	ErrEscalationRateLimited = errors.New("escalation rate limited")
	// ErrInvalidEscalationPriority is returned for unrecognized priority values.
	ErrInvalidEscalationPriority = errors.New("invalid escalation priority")
)

// EscalationPriority represents the urgency level of an escalation.
// Higher priority escalations should be handled with greater urgency.
type EscalationPriority string

// Escalation priority constants in order of increasing urgency.
const (
	EscalationPriorityLow      EscalationPriority = "low"
	EscalationPriorityMedium   EscalationPriority = "medium"
	EscalationPriorityHigh     EscalationPriority = "high"
	EscalationPriorityCritical EscalationPriority = "critical"
)

// EscalationInvestigationView contains investigation data needed for escalation.
// It provides a lightweight view of an investigation suitable for escalation handlers.
type EscalationInvestigationView struct {
	id             string   // Unique investigation identifier
	alertID        string   // Associated alert ID
	sessionID      string   // Session context
	status         string   // Current investigation status
	findings       []string // Summary of findings (descriptions only)
	actions        []string // Summary of actions taken
	isEscalated    bool     // Whether already escalated
	escalateReason string   // Reason for escalation if escalated
}

// ID returns the unique investigation identifier.
func (i *EscalationInvestigationView) ID() string { return i.id }

// AlertID returns the ID of the alert being investigated.
func (i *EscalationInvestigationView) AlertID() string { return i.alertID }

// SessionID returns the session context for this investigation.
func (i *EscalationInvestigationView) SessionID() string { return i.sessionID }

// Status returns the current investigation status.
func (i *EscalationInvestigationView) Status() string { return i.status }

// Findings returns the list of finding descriptions.
func (i *EscalationInvestigationView) Findings() []string { return i.findings }

// Actions returns the list of action descriptions.
func (i *EscalationInvestigationView) Actions() []string { return i.actions }

// IsEscalated returns true if this investigation has already been escalated.
func (i *EscalationInvestigationView) IsEscalated() bool { return i.isEscalated }

// EscalateReason returns the reason for escalation, or empty if not escalated.
func (i *EscalationInvestigationView) EscalateReason() string { return i.escalateReason }

// EscalationRequest contains all information needed to escalate an investigation.
type EscalationRequest struct {
	// Investigation is the investigation being escalated.
	Investigation *EscalationInvestigationView
	// Reason explains why the investigation is being escalated.
	Reason string
	// Priority indicates the urgency of the escalation.
	Priority EscalationPriority
	// Context contains additional key-value metadata for the escalation.
	Context map[string]string
}

// EscalationResult contains the outcome of an escalation attempt.
type EscalationResult struct {
	// Success indicates whether the escalation was successful.
	Success bool
	// EscalatedAt is the timestamp when the escalation occurred.
	EscalatedAt time.Time
	// Target identifies where the escalation was sent (e.g., session ID, channel).
	Target string
	// MessageID is an identifier for the escalation message, if applicable.
	MessageID string
	// Error contains any error that occurred during escalation.
	Error error
}

// EscalationHandler defines the interface for handling investigation escalations.
// Implementations should be safe for concurrent use.
type EscalationHandler interface {
	// Escalate sends an escalation request. Returns ErrNilInvestigation if investigation is nil.
	Escalate(ctx context.Context, req EscalationRequest) (*EscalationResult, error)
	// CanEscalate checks if an investigation can be escalated (e.g., not already escalated).
	CanEscalate(inv *EscalationInvestigationView) bool
	// GetEscalationHistory returns all past escalations for an investigation.
	GetEscalationHistory(invID string) []EscalationResult
}

// EscalationConfig holds configuration for escalation behavior.
type EscalationConfig struct {
	// MaxEscalationsPerInvestigation limits escalations per investigation (0 = unlimited).
	MaxEscalationsPerInvestigation int
	// CooldownPeriod is the minimum time between escalations for the same investigation.
	CooldownPeriod time.Duration
	// DefaultPriority is used when no priority is specified in the request.
	DefaultPriority EscalationPriority
	// EnableRateLimiting enables rate limiting of escalations.
	EnableRateLimiting bool
	// RateLimitPerMinute is the maximum escalations per minute when rate limiting is enabled.
	RateLimitPerMinute int
}

// LogEscalationHandler is a simple escalation handler that records escalations.
// This handler is thread-safe.
type LogEscalationHandler struct {
	mu      sync.RWMutex // Protects history map
	history map[string][]EscalationResult
}

// NewLogEscalationHandler creates a new LogEscalationHandler instance.
func NewLogEscalationHandler() *LogEscalationHandler {
	return &LogEscalationHandler{
		history: make(map[string][]EscalationResult),
	}
}

// Escalate records an escalation in the history.
// Always succeeds unless the context is cancelled or investigation is nil.
// Returns ErrNilInvestigation if req.Investigation is nil.
func (h *LogEscalationHandler) Escalate(
	ctx context.Context,
	req EscalationRequest,
) (*EscalationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if req.Investigation == nil {
		return nil, ErrNilInvestigation
	}

	result := &EscalationResult{
		Success:     true,
		EscalatedAt: time.Now(),
		Target:      "log",
		MessageID:   "log-" + req.Investigation.ID(),
	}

	h.mu.Lock()
	h.history[req.Investigation.ID()] = append(h.history[req.Investigation.ID()], *result)
	h.mu.Unlock()

	return result, nil
}

// CanEscalate returns true if the investigation has not already been escalated.
// Returns false if inv is nil.
func (h *LogEscalationHandler) CanEscalate(inv *EscalationInvestigationView) bool {
	if inv == nil {
		return false
	}
	return !inv.IsEscalated()
}

// GetEscalationHistory returns the list of escalations for an investigation.
// Returns an empty slice if no escalations exist for the given ID.
func (h *LogEscalationHandler) GetEscalationHistory(invID string) []EscalationResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if history, exists := h.history[invID]; exists {
		return history
	}
	return []EscalationResult{}
}

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

// InvestigationStubForEscalation contains investigation data needed for escalation.
// It provides a lightweight view of an investigation suitable for escalation handlers.
type InvestigationStubForEscalation struct {
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
func (i *InvestigationStubForEscalation) ID() string { return i.id }

// AlertID returns the ID of the alert being investigated.
func (i *InvestigationStubForEscalation) AlertID() string { return i.alertID }

// SessionID returns the session context for this investigation.
func (i *InvestigationStubForEscalation) SessionID() string { return i.sessionID }

// Status returns the current investigation status.
func (i *InvestigationStubForEscalation) Status() string { return i.status }

// Findings returns the list of finding descriptions.
func (i *InvestigationStubForEscalation) Findings() []string { return i.findings }

// Actions returns the list of action descriptions.
func (i *InvestigationStubForEscalation) Actions() []string { return i.actions }

// IsEscalated returns true if this investigation has already been escalated.
func (i *InvestigationStubForEscalation) IsEscalated() bool { return i.isEscalated }

// EscalateReason returns the reason for escalation, or empty if not escalated.
func (i *InvestigationStubForEscalation) EscalateReason() string { return i.escalateReason }

// EscalationRequest contains all information needed to escalate an investigation.
type EscalationRequest struct {
	// Investigation is the investigation being escalated.
	Investigation *InvestigationStubForEscalation
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
	CanEscalate(inv *InvestigationStubForEscalation) bool
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
// It is primarily useful for testing and development. In production, use a handler
// that actually notifies operators (e.g., ConversationEscalationHandler).
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
func (h *LogEscalationHandler) CanEscalate(inv *InvestigationStubForEscalation) bool {
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

// ConversationEscalationHandler sends escalations to a conversation session.
// It supports optional rate limiting to prevent escalation storms.
// This handler is thread-safe.
type ConversationEscalationHandler struct {
	mu              sync.RWMutex     // Protects all fields below
	sessionID       string           // Target session for escalations
	config          EscalationConfig // Rate limiting configuration
	history         map[string][]EscalationResult
	lastEscalation  time.Time // Timestamp of last escalation (for rate limiting)
	escalationCount int       // Count of escalations in current minute (for rate limiting)
}

// NewConversationEscalationHandler creates a new handler with default configuration.
func NewConversationEscalationHandler() *ConversationEscalationHandler {
	return &ConversationEscalationHandler{
		history: make(map[string][]EscalationResult),
	}
}

// NewConversationEscalationHandlerWithConfig creates a handler with custom configuration.
// The config controls rate limiting behavior.
func NewConversationEscalationHandlerWithConfig(config EscalationConfig) *ConversationEscalationHandler {
	return &ConversationEscalationHandler{
		config:  config,
		history: make(map[string][]EscalationResult),
	}
}

// SetSessionID sets the target session ID for escalations.
// Must be set before Escalate is called.
func (h *ConversationEscalationHandler) SetSessionID(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessionID = sessionID
}

// Escalate sends an escalation to the configured session.
// Returns ErrNoEscalationTarget if no session ID is configured.
// Returns ErrEscalationRateLimited if rate limiting is enabled and limit is exceeded.
// Returns ErrNilInvestigation if req.Investigation is nil.
//
// Rate limiting behavior (when enabled):
//   - Tracks escalations per minute using a sliding window
//   - Resets counter after one minute of inactivity
//   - Returns ErrEscalationRateLimited when limit is reached
func (h *ConversationEscalationHandler) Escalate(
	ctx context.Context,
	req EscalationRequest,
) (*EscalationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if req.Investigation == nil {
		return nil, ErrNilInvestigation
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.sessionID == "" {
		return nil, ErrNoEscalationTarget
	}

	// Check rate limiting
	if h.config.EnableRateLimiting && h.config.RateLimitPerMinute > 0 {
		if time.Since(h.lastEscalation) < time.Minute {
			if h.escalationCount >= h.config.RateLimitPerMinute {
				return nil, ErrEscalationRateLimited
			}
		} else {
			h.escalationCount = 0
		}
		h.escalationCount++
		h.lastEscalation = time.Now()
	}

	result := &EscalationResult{
		Success:     true,
		EscalatedAt: time.Now(),
		Target:      h.sessionID,
		MessageID:   "conv-" + req.Investigation.ID(),
	}

	h.history[req.Investigation.ID()] = append(h.history[req.Investigation.ID()], *result)

	return result, nil
}

// CanEscalate returns true if the investigation has not already been escalated.
// Returns false if inv is nil.
func (h *ConversationEscalationHandler) CanEscalate(inv *InvestigationStubForEscalation) bool {
	if inv == nil {
		return false
	}
	return !inv.IsEscalated()
}

// GetEscalationHistory returns the list of escalations for an investigation.
// Returns an empty slice if no escalations exist for the given ID.
func (h *ConversationEscalationHandler) GetEscalationHistory(invID string) []EscalationResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if history, exists := h.history[invID]; exists {
		return history
	}
	return []EscalationResult{}
}

// CompositeEscalationHandler chains multiple escalation handlers together.
// When Escalate is called, it invokes all handlers in order. Handler failures
// are silently ignored to ensure all handlers get a chance to process the escalation.
// This is useful for sending escalations to multiple destinations (e.g., log + conversation).
// This handler is thread-safe.
type CompositeEscalationHandler struct {
	mu       sync.RWMutex // Protects handlers slice
	handlers []EscalationHandler
}

// NewCompositeEscalationHandler creates a new handler with the given handlers.
// Additional handlers can be added later with AddHandler.
func NewCompositeEscalationHandler(handlers ...EscalationHandler) *CompositeEscalationHandler {
	return &CompositeEscalationHandler{
		handlers: handlers,
	}
}

// AddHandler appends a handler to the chain.
// Handlers are called in the order they were added.
func (h *CompositeEscalationHandler) AddHandler(handler EscalationHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers = append(h.handlers, handler)
}

// Escalate invokes all handlers in the chain.
// Handler errors are silently ignored to ensure all handlers are attempted.
// Returns the result from the last successful handler, or a synthetic result
// if no handlers succeed. Returns ErrNilInvestigation if req.Investigation is nil.
func (h *CompositeEscalationHandler) Escalate(
	ctx context.Context,
	req EscalationRequest,
) (*EscalationResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if req.Investigation == nil {
		return nil, ErrNilInvestigation
	}

	h.mu.RLock()
	handlers := make([]EscalationHandler, len(h.handlers))
	copy(handlers, h.handlers)
	h.mu.RUnlock()

	var lastResult *EscalationResult
	for _, handler := range handlers {
		result, err := handler.Escalate(ctx, req)
		if err != nil {
			continue
		}
		lastResult = result
	}

	if lastResult == nil {
		return &EscalationResult{
			Success:     true,
			EscalatedAt: time.Now(),
			Target:      "composite",
			MessageID:   "composite-" + req.Investigation.ID(),
		}, nil
	}

	return lastResult, nil
}

// CanEscalate returns true if any handler in the chain can escalate the investigation.
// Returns true if the handler list is empty and the investigation is not already escalated.
// Returns false if inv is nil.
func (h *CompositeEscalationHandler) CanEscalate(inv *InvestigationStubForEscalation) bool {
	if inv == nil {
		return false
	}

	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, handler := range h.handlers {
		if handler.CanEscalate(inv) {
			return true
		}
	}

	return len(h.handlers) == 0 || !inv.IsEscalated()
}

// GetEscalationHistory returns combined history from all handlers in the chain.
// Results from all handlers are concatenated in handler order.
// Returns an empty slice if no escalations exist.
func (h *CompositeEscalationHandler) GetEscalationHistory(invID string) []EscalationResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	var combined []EscalationResult
	for _, handler := range h.handlers {
		combined = append(combined, handler.GetEscalationHistory(invID)...)
	}

	if combined == nil {
		combined = []EscalationResult{}
	}

	return combined
}

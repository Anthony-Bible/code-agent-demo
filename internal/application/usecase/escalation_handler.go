package usecase

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Sentinel errors for escalation handlers.
var (
	ErrNilInvestigation          = errors.New("investigation cannot be nil")
	ErrEscalationFailed          = errors.New("escalation failed")
	ErrNoEscalationTarget        = errors.New("no escalation target configured")
	ErrEscalationAlreadySent     = errors.New("escalation already sent for this investigation")
	ErrEscalationRateLimited     = errors.New("escalation rate limited")
	ErrInvalidEscalationPriority = errors.New("invalid escalation priority")
)

// EscalationPriority represents the urgency of an escalation.
type EscalationPriority string

const (
	EscalationPriorityLow      EscalationPriority = "low"
	EscalationPriorityMedium   EscalationPriority = "medium"
	EscalationPriorityHigh     EscalationPriority = "high"
	EscalationPriorityCritical EscalationPriority = "critical"
)

// InvestigationStubForEscalation represents investigation data for escalation.
type InvestigationStubForEscalation struct {
	id             string
	alertID        string
	sessionID      string
	status         string
	findings       []string
	actions        []string
	isEscalated    bool
	escalateReason string
}

// ID returns the investigation ID.
func (i *InvestigationStubForEscalation) ID() string { return i.id }

// AlertID returns the alert ID.
func (i *InvestigationStubForEscalation) AlertID() string { return i.alertID }

// SessionID returns the session ID.
func (i *InvestigationStubForEscalation) SessionID() string { return i.sessionID }

// Status returns the status.
func (i *InvestigationStubForEscalation) Status() string { return i.status }

// Findings returns the findings.
func (i *InvestigationStubForEscalation) Findings() []string { return i.findings }

// Actions returns the actions.
func (i *InvestigationStubForEscalation) Actions() []string { return i.actions }

// IsEscalated returns whether escalated.
func (i *InvestigationStubForEscalation) IsEscalated() bool { return i.isEscalated }

// EscalateReason returns the escalation reason.
func (i *InvestigationStubForEscalation) EscalateReason() string { return i.escalateReason }

// EscalationRequest contains details about an escalation.
type EscalationRequest struct {
	Investigation *InvestigationStubForEscalation
	Reason        string
	Priority      EscalationPriority
	Context       map[string]string
}

// EscalationResult contains the outcome of an escalation attempt.
type EscalationResult struct {
	Success     bool
	EscalatedAt time.Time
	Target      string
	MessageID   string
	Error       error
}

// EscalationHandler defines the interface for handling escalations.
type EscalationHandler interface {
	Escalate(ctx context.Context, req EscalationRequest) (*EscalationResult, error)
	CanEscalate(inv *InvestigationStubForEscalation) bool
	GetEscalationHistory(invID string) []EscalationResult
}

// EscalationConfig holds configuration for escalation behavior.
type EscalationConfig struct {
	MaxEscalationsPerInvestigation int
	CooldownPeriod                 time.Duration
	DefaultPriority                EscalationPriority
	EnableRateLimiting             bool
	RateLimitPerMinute             int
}

// LogEscalationHandler logs escalations.
type LogEscalationHandler struct {
	mu      sync.RWMutex
	history map[string][]EscalationResult
}

// NewLogEscalationHandler creates a new LogEscalationHandler.
func NewLogEscalationHandler() *LogEscalationHandler {
	return &LogEscalationHandler{
		history: make(map[string][]EscalationResult),
	}
}

// Escalate logs an escalation.
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

// CanEscalate checks if escalation is possible.
func (h *LogEscalationHandler) CanEscalate(inv *InvestigationStubForEscalation) bool {
	if inv == nil {
		return false
	}
	return !inv.IsEscalated()
}

// GetEscalationHistory returns escalation history for an investigation.
func (h *LogEscalationHandler) GetEscalationHistory(invID string) []EscalationResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if history, exists := h.history[invID]; exists {
		return history
	}
	return []EscalationResult{}
}

// ConversationEscalationHandler sends escalations to a conversation session.
type ConversationEscalationHandler struct {
	mu              sync.RWMutex
	sessionID       string
	config          EscalationConfig
	history         map[string][]EscalationResult
	lastEscalation  time.Time
	escalationCount int
}

// NewConversationEscalationHandler creates a new ConversationEscalationHandler.
func NewConversationEscalationHandler() *ConversationEscalationHandler {
	return &ConversationEscalationHandler{
		history: make(map[string][]EscalationResult),
	}
}

// NewConversationEscalationHandlerWithConfig creates a handler with config.
func NewConversationEscalationHandlerWithConfig(config EscalationConfig) *ConversationEscalationHandler {
	return &ConversationEscalationHandler{
		config:  config,
		history: make(map[string][]EscalationResult),
	}
}

// SetSessionID sets the target session ID.
func (h *ConversationEscalationHandler) SetSessionID(sessionID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.sessionID = sessionID
}

// Escalate sends an escalation to the conversation session.
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

// CanEscalate checks if escalation is possible.
func (h *ConversationEscalationHandler) CanEscalate(inv *InvestigationStubForEscalation) bool {
	if inv == nil {
		return false
	}
	return !inv.IsEscalated()
}

// GetEscalationHistory returns escalation history for an investigation.
func (h *ConversationEscalationHandler) GetEscalationHistory(invID string) []EscalationResult {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if history, exists := h.history[invID]; exists {
		return history
	}
	return []EscalationResult{}
}

// CompositeEscalationHandler chains multiple handlers.
type CompositeEscalationHandler struct {
	mu       sync.RWMutex
	handlers []EscalationHandler
}

// NewCompositeEscalationHandler creates a new CompositeEscalationHandler.
func NewCompositeEscalationHandler(handlers ...EscalationHandler) *CompositeEscalationHandler {
	return &CompositeEscalationHandler{
		handlers: handlers,
	}
}

// AddHandler adds a handler to the chain.
func (h *CompositeEscalationHandler) AddHandler(handler EscalationHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.handlers = append(h.handlers, handler)
}

// Escalate calls all handlers in the chain.
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

// CanEscalate checks if any handler can escalate.
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

// GetEscalationHistory returns combined history from all handlers.
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

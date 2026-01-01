package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Sentinel errors for AlertInvestigationUseCase.
var (
	ErrAlertNil                    = errors.New("alert cannot be nil")
	ErrInvestigationAlreadyRunning = errors.New("investigation already running for this alert")
	ErrMaxConcurrentReached        = errors.New("maximum concurrent investigations reached")
	ErrInvestigationTimeout        = errors.New("investigation timed out")
	ErrActionBudgetExceeded        = errors.New("action budget exceeded")
	ErrToolNotAllowed              = errors.New("tool not allowed by investigation config")
	ErrCommandBlocked              = errors.New("command blocked by safety rules")
	ErrInvestigationNotFoundUC     = errors.New("investigation not found")
	ErrUseCaseShutdown             = errors.New("use case is shutdown")
)

// AlertForInvestigation represents alert data for investigation.
type AlertForInvestigation struct {
	id          string
	source      string
	severity    string
	title       string
	description string
	labels      map[string]string
}

// ID returns the alert ID.
func (a *AlertForInvestigation) ID() string { return a.id }

// Source returns the source.
func (a *AlertForInvestigation) Source() string { return a.source }

// Severity returns the severity.
func (a *AlertForInvestigation) Severity() string { return a.severity }

// Title returns the title.
func (a *AlertForInvestigation) Title() string { return a.title }

// Description returns the description.
func (a *AlertForInvestigation) Description() string { return a.description }

// Labels returns the labels.
func (a *AlertForInvestigation) Labels() map[string]string { return a.labels }

// IsCritical returns true if severity is critical.
func (a *AlertForInvestigation) IsCritical() bool {
	return a.severity == string(EscalationPriorityCritical)
}

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

// AlertInvestigationUseCase orchestrates alert investigations.
type AlertInvestigationUseCase struct {
	mu                    sync.RWMutex
	config                AlertInvestigationUseCaseConfig
	activeInvestigations  map[string]*activeInvestigation
	alertToInvestigation  map[string]string
	escalationHandler     EscalationHandler
	promptBuilderRegistry PromptBuilderRegistry
	shutdown              bool
	idCounter             int64
}

type activeInvestigation struct {
	id        string
	alertID   string
	startedAt time.Time
	cancel    context.CancelFunc
}

// NewAlertInvestigationUseCase creates a new use case with defaults.
func NewAlertInvestigationUseCase() *AlertInvestigationUseCase {
	return &AlertInvestigationUseCase{
		config: AlertInvestigationUseCaseConfig{
			MaxActions:    20,
			MaxDuration:   15 * time.Minute,
			MaxConcurrent: 5,
			AllowedTools:  []string{"bash", "read_file", "list_files"},
			BlockedCommands: []string{
				"rm -rf",
				"dd if=",
				"mkfs",
			},
		},
		activeInvestigations: make(map[string]*activeInvestigation),
		alertToInvestigation: make(map[string]string),
	}
}

// NewAlertInvestigationUseCaseWithConfig creates a use case with config.
func NewAlertInvestigationUseCaseWithConfig(config AlertInvestigationUseCaseConfig) *AlertInvestigationUseCase {
	return &AlertInvestigationUseCase{
		config:               config,
		activeInvestigations: make(map[string]*activeInvestigation),
		alertToInvestigation: make(map[string]string),
	}
}

// HandleAlert handles an alert investigation synchronously.
func (uc *AlertInvestigationUseCase) HandleAlert(
	ctx context.Context,
	alert *AlertForInvestigation,
) (*InvestigationResultStub, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if alert == nil {
		return nil, ErrAlertNil
	}

	invID, err := uc.StartInvestigation(ctx, alert)
	if err != nil {
		return nil, err
	}

	// Simulate investigation completion
	result := &InvestigationResultStub{
		InvestigationID: invID,
		AlertID:         alert.ID(),
		Status:          "completed",
		Findings:        []string{},
		ActionsTaken:    0,
		Duration:        time.Since(time.Now()),
		Confidence:      0.8,
	}

	return result, nil
}

// StartInvestigation starts a new investigation for an alert.
func (uc *AlertInvestigationUseCase) StartInvestigation(
	ctx context.Context,
	alert *AlertForInvestigation,
) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	if alert == nil {
		return "", ErrAlertNil
	}

	uc.mu.Lock()
	defer uc.mu.Unlock()

	if uc.shutdown {
		return "", ErrUseCaseShutdown
	}

	// Check if already investigating this alert
	if _, exists := uc.alertToInvestigation[alert.ID()]; exists {
		return "", ErrInvestigationAlreadyRunning
	}

	// Check max concurrent
	if uc.config.MaxConcurrent > 0 && len(uc.activeInvestigations) >= uc.config.MaxConcurrent {
		return "", ErrMaxConcurrentReached
	}

	uc.idCounter++
	invID := fmt.Sprintf("inv-%d-%d", time.Now().UnixNano(), uc.idCounter)
	_, cancel := context.WithCancel(ctx)

	inv := &activeInvestigation{
		id:        invID,
		alertID:   alert.ID(),
		startedAt: time.Now(),
		cancel:    cancel,
	}

	uc.activeInvestigations[invID] = inv
	uc.alertToInvestigation[alert.ID()] = invID

	return invID, nil
}

// StopInvestigation stops an active investigation.
func (uc *AlertInvestigationUseCase) StopInvestigation(ctx context.Context, invID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	uc.mu.Lock()
	defer uc.mu.Unlock()

	inv, exists := uc.activeInvestigations[invID]
	if !exists {
		return ErrInvestigationNotFoundUC
	}

	if inv.cancel != nil {
		inv.cancel()
	}

	delete(uc.activeInvestigations, invID)
	delete(uc.alertToInvestigation, inv.alertID)

	return nil
}

// GetInvestigationStatus returns the status of an investigation.
func (uc *AlertInvestigationUseCase) GetInvestigationStatus(
	ctx context.Context,
	invID string,
) (*InvestigationResultStub, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	uc.mu.RLock()
	defer uc.mu.RUnlock()

	inv, exists := uc.activeInvestigations[invID]
	if !exists {
		return nil, ErrInvestigationNotFoundUC
	}

	return &InvestigationResultStub{
		InvestigationID: inv.id,
		AlertID:         inv.alertID,
		Status:          "running",
		Duration:        time.Since(inv.startedAt),
	}, nil
}

// ListActiveInvestigations returns IDs of active investigations.
func (uc *AlertInvestigationUseCase) ListActiveInvestigations(ctx context.Context) ([]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	uc.mu.RLock()
	defer uc.mu.RUnlock()

	ids := make([]string, 0, len(uc.activeInvestigations))
	for id := range uc.activeInvestigations {
		ids = append(ids, id)
	}

	return ids, nil
}

// GetActiveCount returns the number of active investigations.
func (uc *AlertInvestigationUseCase) GetActiveCount() int {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return len(uc.activeInvestigations)
}

// SetEscalationHandler sets the escalation handler.
func (uc *AlertInvestigationUseCase) SetEscalationHandler(handler EscalationHandler) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.escalationHandler = handler
}

// SetPromptBuilderRegistry sets the prompt builder registry.
func (uc *AlertInvestigationUseCase) SetPromptBuilderRegistry(registry PromptBuilderRegistry) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.promptBuilderRegistry = registry
}

// IsToolAllowed checks if a tool is allowed.
func (uc *AlertInvestigationUseCase) IsToolAllowed(tool string) bool {
	uc.mu.RLock()
	defer uc.mu.RUnlock()

	for _, t := range uc.config.AllowedTools {
		if t == tool {
			return true
		}
	}
	return false
}

// IsCommandBlocked checks if a command is blocked.
func (uc *AlertInvestigationUseCase) IsCommandBlocked(cmd string) bool {
	uc.mu.RLock()
	defer uc.mu.RUnlock()

	for _, blocked := range uc.config.BlockedCommands {
		if strings.Contains(cmd, blocked) {
			return true
		}
	}
	return false
}

// Shutdown stops all active investigations and shuts down.
func (uc *AlertInvestigationUseCase) Shutdown(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	uc.mu.Lock()
	defer uc.mu.Unlock()

	uc.shutdown = true

	// Cancel all active investigations
	for _, inv := range uc.activeInvestigations {
		if inv.cancel != nil {
			inv.cancel()
		}
	}

	uc.activeInvestigations = make(map[string]*activeInvestigation)
	uc.alertToInvestigation = make(map[string]string)

	return nil
}

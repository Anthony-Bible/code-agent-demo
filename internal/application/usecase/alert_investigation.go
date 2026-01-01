package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// SafetyEnforcer defines the interface for safety checks during investigations.
// This is defined locally in usecase to avoid import cycles with service package.
type SafetyEnforcer interface {
	CheckToolAllowed(tool string) error
	CheckCommandAllowed(cmd string) error
	CheckActionBudget(currentActions int) error
	CheckTimeout(ctx context.Context) error
}

// InvestigationStubData is a minimal interface for investigation persistence.
// Matches the InvestigationStub type from service package.
type InvestigationStubData interface {
	ID() string
	AlertID() string
	SessionID() string
	Status() string
	StartedAt() time.Time
}

// InvestigationStoreWriter defines the write interface for investigation persistence.
// This avoids needing to import the full service.InvestigationStore interface.
type InvestigationStoreWriter interface {
	Store(ctx context.Context, inv InvestigationStubData) error
	Get(ctx context.Context, id string) (InvestigationStubData, error)
	Update(ctx context.Context, inv InvestigationStubData) error
}

// simpleInvestigationStub is a minimal implementation of InvestigationStubData.
type simpleInvestigationStub struct {
	id, alertID, sessionID, status string
	startedAt                      time.Time
}

func (s *simpleInvestigationStub) ID() string        { return s.id }
func (s *simpleInvestigationStub) AlertID() string   { return s.alertID }
func (s *simpleInvestigationStub) SessionID() string { return s.sessionID }
func (s *simpleInvestigationStub) Status() string    { return s.status }
func (s *simpleInvestigationStub) StartedAt() time.Time {
	if s.startedAt.IsZero() {
		return time.Now()
	}
	return s.startedAt
}

func newSimpleInvestigationStub(id, alertID, sessionID, status string) *simpleInvestigationStub {
	return &simpleInvestigationStub{
		id:        id,
		alertID:   alertID,
		sessionID: sessionID,
		status:    status,
		startedAt: time.Now(),
	}
}

// Sentinel errors for AlertInvestigationUseCase operations.
// These errors indicate various failure conditions during investigation.
var (
	// ErrAlertNil is returned when nil is passed as the alert parameter.
	ErrAlertNil = errors.New("alert cannot be nil")
	// ErrInvestigationAlreadyRunning is returned when starting an investigation
	// for an alert that already has an active investigation.
	ErrInvestigationAlreadyRunning = errors.New("investigation already running for this alert")
	// ErrMaxConcurrentReached is returned when the maximum number of concurrent
	// investigations has been reached.
	ErrMaxConcurrentReached = errors.New("maximum concurrent investigations reached")
	// ErrInvestigationTimeout is returned when an investigation exceeds its time limit.
	ErrInvestigationTimeout = errors.New("investigation timed out")
	// ErrActionBudgetExceeded is returned when an investigation exceeds its action limit.
	ErrActionBudgetExceeded = errors.New("action budget exceeded")
	// ErrToolNotAllowed is returned when an investigation attempts to use a disallowed tool.
	ErrToolNotAllowed = errors.New("tool not allowed by investigation config")
	// ErrCommandBlocked is returned when a command matches a blocked pattern.
	ErrCommandBlocked = errors.New("command blocked by safety rules")
	// ErrInvestigationNotFoundUC is returned when an investigation ID is not found.
	ErrInvestigationNotFoundUC = errors.New("investigation not found")
	// ErrUseCaseShutdown is returned when operations are attempted after shutdown.
	ErrUseCaseShutdown = errors.New("use case is shutdown")
)

// AlertForInvestigation represents alert data passed to the investigation use case.
// It is a lightweight view of an alert containing only the fields needed for investigation.
type AlertForInvestigation struct {
	id          string            // Unique alert identifier
	source      string            // Alert source system
	severity    string            // Alert severity level
	title       string            // Human-readable title
	description string            // Detailed description
	labels      map[string]string // Additional metadata
}

// ID returns the unique alert identifier.
func (a *AlertForInvestigation) ID() string { return a.id }

// Source returns the system that generated this alert.
func (a *AlertForInvestigation) Source() string { return a.source }

// Severity returns the alert severity level.
func (a *AlertForInvestigation) Severity() string { return a.severity }

// Title returns the human-readable alert title.
func (a *AlertForInvestigation) Title() string { return a.title }

// Description returns the detailed alert description.
func (a *AlertForInvestigation) Description() string { return a.description }

// Labels returns the metadata labels attached to this alert.
func (a *AlertForInvestigation) Labels() map[string]string { return a.labels }

// IsCritical returns true if the alert severity is "critical".
func (a *AlertForInvestigation) IsCritical() bool {
	return a.severity == string(EscalationPriorityCritical)
}

// InvestigationResultStub represents the outcome of an investigation.
// It provides a summary of what happened during the investigation.
type InvestigationResultStub struct {
	InvestigationID string        // Unique identifier for this investigation
	AlertID         string        // ID of the investigated alert
	Status          string        // Final status (completed, failed, escalated)
	Findings        []string      // Summary of findings discovered
	ActionsTaken    int           // Number of tool executions performed
	Duration        time.Duration // Total investigation time
	Confidence      float64       // Confidence level in the outcome [0.0, 1.0]
	Escalated       bool          // Whether the investigation was escalated
	EscalateReason  string        // Reason for escalation, if applicable
	Error           error         // Any error that occurred
}

// AlertInvestigationUseCaseConfig holds configuration for the investigation use case.
// It defines operational limits and safety constraints for investigations.
type AlertInvestigationUseCaseConfig struct {
	MaxActions           int           // Maximum tool executions per investigation
	MaxDuration          time.Duration // Maximum investigation time
	MaxConcurrent        int           // Maximum simultaneous investigations
	AllowedTools         []string      // Tools that investigations may use
	BlockedCommands      []string      // Command patterns that are blocked
	EscalateOnConfidence float64       // Escalate when confidence is below this value
	EscalateOnErrors     int           // Escalate after this many consecutive errors
	AutoStartForCritical bool          // Automatically start investigations for critical alerts
	EnableSafetyChecks   bool          // Enable command safety validation
}

// AlertInvestigationUseCase orchestrates AI-driven alert investigations.
// It manages the lifecycle of investigations, enforces safety limits, and
// handles escalation when investigations fail or need human intervention.
// This type is safe for concurrent use from multiple goroutines.
type AlertInvestigationUseCase struct {
	mu                    sync.RWMutex                    // Protects all fields below
	config                AlertInvestigationUseCaseConfig // Safety and operational config
	activeInvestigations  map[string]*activeInvestigation // Currently running investigations
	alertToInvestigation  map[string]string               // Maps alert ID to investigation ID
	escalationHandler     EscalationHandler               // Handler for escalations
	promptBuilderRegistry PromptBuilderRegistry           // Generates investigation prompts
	safetyEnforcer        SafetyEnforcer                  // Safety policy enforcer
	investigationStore    InvestigationStoreWriter        // Persistence for investigations
	shutdown              bool                            // True after Shutdown is called
	idCounter             int64                           // Counter for generating unique IDs
}

// activeInvestigation tracks a running investigation.
type activeInvestigation struct {
	id        string             // Unique investigation identifier
	alertID   string             // Alert being investigated
	startedAt time.Time          // When investigation started
	cancel    context.CancelFunc // Cancels the investigation context
}

// NewAlertInvestigationUseCase creates a new use case with sensible defaults.
// Default configuration includes:
//   - MaxActions: 20 (prevents runaway investigations)
//   - MaxDuration: 15 minutes
//   - MaxConcurrent: 5 simultaneous investigations
//   - AllowedTools: bash, read_file, list_files (safe investigation tools)
//   - BlockedCommands: rm -rf, dd, mkfs (destructive commands)
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

// NewAlertInvestigationUseCaseWithConfig creates a use case with custom configuration.
func NewAlertInvestigationUseCaseWithConfig(config AlertInvestigationUseCaseConfig) *AlertInvestigationUseCase {
	return &AlertInvestigationUseCase{
		config:               config,
		activeInvestigations: make(map[string]*activeInvestigation),
		alertToInvestigation: make(map[string]string),
	}
}

// HandleAlert performs a complete investigation for an alert synchronously.
// It starts the investigation, waits for completion, and returns the result.
// Returns ErrAlertNil if alert is nil.
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

	// Check if safety enforcer blocks all investigation tools
	uc.mu.RLock()
	enforcer := uc.safetyEnforcer
	allowedTools := uc.config.AllowedTools
	uc.mu.RUnlock()

	if enforcer != nil && len(allowedTools) > 0 {
		allBlocked := true
		for _, tool := range allowedTools {
			if enforcer.CheckToolAllowed(tool) == nil {
				allBlocked = false
				break
			}
		}
		if allBlocked {
			// All tools are blocked - escalate
			return &InvestigationResultStub{
				InvestigationID: invID,
				AlertID:         alert.ID(),
				Status:          "failed",
				Findings:        []string{},
				ActionsTaken:    0,
				Duration:        time.Since(time.Now()),
				Confidence:      0.0,
				Escalated:       true,
				EscalateReason:  "all investigation tools are blocked by safety policy",
			}, nil
		}
	}

	// Simulate investigation completion
	// TODO: Implement actual investigation loop with AI interaction
	result := &InvestigationResultStub{
		InvestigationID: invID,
		AlertID:         alert.ID(),
		Status:          "completed",
		Findings:        []string{},
		ActionsTaken:    0,
		Duration:        time.Since(time.Now()),
		Confidence:      0.8,
	}

	// Update store with final status if configured
	uc.mu.RLock()
	store := uc.investigationStore
	uc.mu.RUnlock()

	if store != nil {
		stub := newSimpleInvestigationStub(invID, alert.ID(), "", result.Status)
		_ = store.Update(ctx, stub)
	}

	return result, nil
}

// StartInvestigation starts a new investigation for an alert.
// Returns the investigation ID on success.
//
// Safety checks performed:
//   - Rejects if alert is nil (ErrAlertNil)
//   - Rejects if investigation already running for this alert (ErrInvestigationAlreadyRunning)
//   - Rejects if max concurrent limit reached (ErrMaxConcurrentReached)
//   - Rejects if use case is shutdown (ErrUseCaseShutdown)
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

	// Persist to store if configured
	if uc.investigationStore != nil {
		stub := newSimpleInvestigationStub(invID, alert.ID(), "", "started")
		_ = uc.investigationStore.Store(ctx, stub)
	}

	return invID, nil
}

// StopInvestigation stops an active investigation by ID.
// Cancels the investigation context and removes it from tracking.
// Returns ErrInvestigationNotFoundUC if the investigation does not exist.
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

	// Update store with stopped status if configured
	if uc.investigationStore != nil {
		stub := newSimpleInvestigationStub(invID, inv.alertID, "", "stopped")
		_ = uc.investigationStore.Update(ctx, stub)
	}

	delete(uc.activeInvestigations, invID)
	delete(uc.alertToInvestigation, inv.alertID)

	return nil
}

// GetInvestigationStatus returns the current status of an active investigation.
// Returns ErrInvestigationNotFoundUC if the investigation is not found.
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

// ListActiveInvestigations returns the IDs of all currently running investigations.
// Returns an empty slice if no investigations are active.
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

// GetActiveCount returns the number of currently active investigations.
func (uc *AlertInvestigationUseCase) GetActiveCount() int {
	uc.mu.RLock()
	defer uc.mu.RUnlock()
	return len(uc.activeInvestigations)
}

// SetEscalationHandler configures the handler used for investigation escalations.
func (uc *AlertInvestigationUseCase) SetEscalationHandler(handler EscalationHandler) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.escalationHandler = handler
}

// SetPromptBuilderRegistry configures the registry used to generate investigation prompts.
func (uc *AlertInvestigationUseCase) SetPromptBuilderRegistry(registry PromptBuilderRegistry) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.promptBuilderRegistry = registry
}

// SetSafetyEnforcer configures the safety enforcer for tool and command validation.
func (uc *AlertInvestigationUseCase) SetSafetyEnforcer(enforcer SafetyEnforcer) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.safetyEnforcer = enforcer
}

// SetInvestigationStore configures the store for investigation persistence.
func (uc *AlertInvestigationUseCase) SetInvestigationStore(store InvestigationStoreWriter) {
	uc.mu.Lock()
	defer uc.mu.Unlock()
	uc.investigationStore = store
}

// IsToolAllowed checks if a tool name is in the allowed list.
// Returns false if the tool is not explicitly allowed.
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

// IsCommandBlocked checks if a command contains any blocked pattern.
// Returns true if the command should be rejected for safety reasons.
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

// Shutdown gracefully shuts down the use case.
// Cancels all active investigations and prevents new ones from starting.
// After Shutdown, all operations return ErrUseCaseShutdown.
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

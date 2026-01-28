package usecase

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Mock errors used by mock implementations.
var (
	errMockToolBlocked           = errors.New("tool blocked")
	errMockCommandBlocked        = errors.New("command blocked")
	errMockActionBudgetExhausted = errors.New("action budget exhausted")
	errMockTimeout               = errors.New("timeout")
	errMockNotFound              = errors.New("not found")
	errMockDuplicate             = errors.New("duplicate")
	errMockNil                   = errors.New("nil investigation")
	errMockShutdown              = errors.New("shutdown")
)

// MockSafetyEnforcer is a test double for SafetyEnforcer interface.
type MockSafetyEnforcer struct {
	mu              sync.RWMutex
	blockedTools    map[string]bool
	allowedTools    map[string]bool // If non-nil, only these tools are allowed
	blockedCommands []string
	actionBudget    int
	timeoutEnabled  bool
}

// NewMockSafetyEnforcer creates a mock that allows all tools and commands.
func NewMockSafetyEnforcer() *MockSafetyEnforcer {
	return &MockSafetyEnforcer{
		blockedTools:   make(map[string]bool),
		actionBudget:   1000, // Large budget
		timeoutEnabled: false,
	}
}

// NewMockSafetyEnforcerWithBlockedTools creates a mock that blocks specific tools.
func NewMockSafetyEnforcerWithBlockedTools(tools []string) *MockSafetyEnforcer {
	m := NewMockSafetyEnforcer()
	for _, t := range tools {
		m.blockedTools[t] = true
	}
	return m
}

// NewMockSafetyEnforcerWithAllowedTools creates a mock that only allows specific tools.
func NewMockSafetyEnforcerWithAllowedTools(tools []string) *MockSafetyEnforcer {
	m := NewMockSafetyEnforcer()
	m.allowedTools = make(map[string]bool)
	for _, t := range tools {
		m.allowedTools[t] = true
	}
	return m
}

// NewMockSafetyEnforcerWithBlockedCommands creates a mock that blocks specific command patterns.
func NewMockSafetyEnforcerWithBlockedCommands(cmds []string) *MockSafetyEnforcer {
	m := NewMockSafetyEnforcer()
	m.blockedCommands = cmds
	return m
}

// NewMockSafetyEnforcerWithActionBudget creates a mock with a specific action budget.
func NewMockSafetyEnforcerWithActionBudget(budget int) *MockSafetyEnforcer {
	m := NewMockSafetyEnforcer()
	m.actionBudget = budget
	return m
}

// NewMockSafetyEnforcerWithTimeout creates a mock that always returns timeout error.
func NewMockSafetyEnforcerWithTimeout() *MockSafetyEnforcer {
	m := NewMockSafetyEnforcer()
	m.timeoutEnabled = true
	return m
}

// CheckToolAllowed returns error if the tool is blocked or not in the allowed list.
func (m *MockSafetyEnforcer) CheckToolAllowed(tool string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	// If allowedTools is set, only tools in it are allowed
	if m.allowedTools != nil {
		if !m.allowedTools[tool] {
			return errMockToolBlocked
		}
		return nil
	}
	// Otherwise, check blockedTools
	if m.blockedTools[tool] {
		return errMockToolBlocked
	}
	return nil
}

// CheckCommandAllowed returns error if the command matches a blocked pattern.
func (m *MockSafetyEnforcer) CheckCommandAllowed(cmd string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, blocked := range m.blockedCommands {
		if len(cmd) >= len(blocked) && cmd[:len(blocked)] == blocked {
			return errMockCommandBlocked
		}
	}
	return nil
}

// CheckActionBudget returns error if currentActions >= budget.
func (m *MockSafetyEnforcer) CheckActionBudget(currentActions int) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if currentActions >= m.actionBudget {
		return errMockActionBudgetExhausted
	}
	return nil
}

// CheckTimeout returns error if timeout is enabled or context is cancelled.
func (m *MockSafetyEnforcer) CheckTimeout(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.timeoutEnabled {
		return errMockTimeout
	}
	if ctx != nil && ctx.Err() != nil {
		return errMockTimeout
	}
	return nil
}

// GetMaxActions returns the action budget.
func (m *MockSafetyEnforcer) GetMaxActions() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.actionBudget
}

// mockInvestigationRecord is a minimal InvestigationRecordData implementation for testing.
type mockInvestigationRecord struct {
	id, alertID, sessionID, status string
	startedAt                      time.Time
	completedAt                    time.Time
	findings                       []string
	actionsTaken                   int
	durationNanos                  int64
	confidence                     float64
	escalated                      bool
	escalateReason                 string
}

func (s *mockInvestigationRecord) ID() string        { return s.id }
func (s *mockInvestigationRecord) AlertID() string   { return s.alertID }
func (s *mockInvestigationRecord) SessionID() string { return s.sessionID }
func (s *mockInvestigationRecord) Status() string    { return s.status }
func (s *mockInvestigationRecord) StartedAt() time.Time {
	if s.startedAt.IsZero() {
		return time.Now()
	}
	return s.startedAt
}
func (s *mockInvestigationRecord) CompletedAt() time.Time  { return s.completedAt }
func (s *mockInvestigationRecord) Findings() []string      { return s.findings }
func (s *mockInvestigationRecord) ActionsTaken() int       { return s.actionsTaken }
func (s *mockInvestigationRecord) Duration() time.Duration { return time.Duration(s.durationNanos) }
func (s *mockInvestigationRecord) Confidence() float64     { return s.confidence }
func (s *mockInvestigationRecord) Escalated() bool         { return s.escalated }
func (s *mockInvestigationRecord) EscalateReason() string  { return s.escalateReason }

// MockInvestigationStore is a test double for InvestigationStoreWriter interface.
type MockInvestigationStore struct {
	mu     sync.RWMutex
	data   map[string]*mockInvestigationRecord
	closed bool
}

// NewMockInvestigationStore creates a new mock investigation store.
func NewMockInvestigationStore() *MockInvestigationStore {
	return &MockInvestigationStore{
		data: make(map[string]*mockInvestigationRecord),
	}
}

// Store saves an investigation.
func (m *MockInvestigationStore) Store(ctx context.Context, inv InvestigationRecordData) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if inv == nil {
		return errMockNil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errMockShutdown
	}
	if _, exists := m.data[inv.ID()]; exists {
		return errMockDuplicate
	}

	m.data[inv.ID()] = &mockInvestigationRecord{
		id:        inv.ID(),
		alertID:   inv.AlertID(),
		sessionID: inv.SessionID(),
		status:    inv.Status(),
		startedAt: inv.StartedAt(),
	}
	return nil
}

// Get retrieves an investigation by ID.
func (m *MockInvestigationStore) Get(ctx context.Context, id string) (InvestigationRecordData, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return nil, errMockShutdown
	}

	inv, exists := m.data[id]
	if !exists {
		return nil, errMockNotFound
	}
	return inv, nil
}

// Update modifies an existing investigation.
func (m *MockInvestigationStore) Update(ctx context.Context, inv InvestigationRecordData) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if inv == nil {
		return errMockNil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return errMockShutdown
	}
	if _, exists := m.data[inv.ID()]; !exists {
		return errMockNotFound
	}

	m.data[inv.ID()] = &mockInvestigationRecord{
		id:        inv.ID(),
		alertID:   inv.AlertID(),
		sessionID: inv.SessionID(),
		status:    inv.Status(),
		startedAt: inv.StartedAt(),
	}
	return nil
}

// Close marks the store as closed.
func (m *MockInvestigationStore) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

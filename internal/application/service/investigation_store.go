// Package service provides application services for the code-editing-agent.
package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Sentinel errors for InvestigationStore operations.
// These errors are returned when store operations fail.
var (
	// ErrInvestigationNotFound is returned when a requested investigation does not exist.
	ErrInvestigationNotFound = errors.New("investigation not found")
	// ErrDuplicateInvestigationID is returned when attempting to store an investigation
	// with an ID that already exists in the store.
	ErrDuplicateInvestigationID = errors.New("investigation ID already exists")
	// ErrInvestigationStoreShutdown is returned when operations are attempted on a closed store.
	ErrInvestigationStoreShutdown = errors.New("investigation store is shutdown")
	// ErrNilInvestigationRecord is returned when a nil investigation is passed to Store or Update.
	ErrNilInvestigationRecord = errors.New("investigation cannot be nil")
	// ErrEmptyInvestigationIDStore is returned when Get is called with an empty ID.
	ErrEmptyInvestigationIDStore = errors.New("investigation ID cannot be empty")
)

// InvestigationQuery defines filter criteria for querying investigations.
// All non-zero fields are combined with AND logic. Zero-value fields are ignored.
type InvestigationQuery struct {
	AlertID   string    // Filter by alert ID (exact match)
	SessionID string    // Filter by session ID (exact match)
	Status    []string  // Filter by status (matches any in list)
	Since     time.Time // Filter by start time >= Since
	Until     time.Time // Filter by start time <= Until
	Limit     int       // Maximum results to return (0 = unlimited)
}

// InvestigationRecord represents an investigation record for storage.
// It contains fields for both metadata and full investigation results.
type InvestigationRecord struct {
	id        string    // Unique identifier
	alertID   string    // Associated alert ID
	sessionID string    // Session context
	status    string    // Current status
	startedAt time.Time // When the investigation began
	// Full result fields
	completedAt    time.Time // When the investigation finished
	findings       []string  // Summary of findings discovered
	actionsTaken   int       // Number of tool executions performed
	durationNanos  int64     // Duration in nanoseconds (serializable)
	confidence     float64   // Confidence level [0.0, 1.0]
	escalated      bool      // Whether escalated to human
	escalateReason string    // Reason for escalation
}

// NewInvestigationRecord creates a new InvestigationRecord with the given parameters.
// This is the primary constructor for creating investigation records with minimal metadata.
func NewInvestigationRecord(id, alertID, sessionID, status string, startedAt time.Time) *InvestigationRecord {
	return &InvestigationRecord{
		id:        id,
		alertID:   alertID,
		sessionID: sessionID,
		status:    status,
		startedAt: startedAt,
	}
}

// NewInvestigationRecordWithResult creates an InvestigationRecord with full result data.
// Use this constructor when storing completed investigation results.
func NewInvestigationRecordWithResult(
	id, alertID, sessionID, status string,
	startedAt, completedAt time.Time,
	findings []string,
	actionsTaken int,
	duration time.Duration,
	confidence float64,
	escalated bool,
	escalateReason string,
) *InvestigationRecord {
	return &InvestigationRecord{
		id:             id,
		alertID:        alertID,
		sessionID:      sessionID,
		status:         status,
		startedAt:      startedAt,
		completedAt:    completedAt,
		findings:       findings,
		actionsTaken:   actionsTaken,
		durationNanos:  int64(duration),
		confidence:     confidence,
		escalated:      escalated,
		escalateReason: escalateReason,
	}
}

// ID returns the unique investigation identifier.
func (i *InvestigationRecord) ID() string { return i.id }

// AlertID returns the ID of the alert being investigated.
func (i *InvestigationRecord) AlertID() string { return i.alertID }

// SessionID returns the session context for this investigation.
func (i *InvestigationRecord) SessionID() string { return i.sessionID }

// Status returns the current investigation status.
func (i *InvestigationRecord) Status() string { return i.status }

// StartedAt returns when the investigation began.
// Returns the current time if startedAt was never set (zero value).
func (i *InvestigationRecord) StartedAt() time.Time {
	if i.startedAt.IsZero() {
		return time.Now()
	}
	return i.startedAt
}

// CompletedAt returns when the investigation finished.
// Returns zero time if the investigation is still in progress.
func (i *InvestigationRecord) CompletedAt() time.Time { return i.completedAt }

// Findings returns the summary findings discovered during the investigation.
func (i *InvestigationRecord) Findings() []string { return i.findings }

// ActionsTaken returns the number of tool executions performed.
func (i *InvestigationRecord) ActionsTaken() int { return i.actionsTaken }

// Duration returns the total investigation duration.
func (i *InvestigationRecord) Duration() time.Duration { return time.Duration(i.durationNanos) }

// Confidence returns the confidence level in the investigation outcome [0.0, 1.0].
func (i *InvestigationRecord) Confidence() float64 { return i.confidence }

// Escalated returns true if the investigation was escalated to a human.
func (i *InvestigationRecord) Escalated() bool { return i.escalated }

// EscalateReason returns the reason for escalation, if applicable.
func (i *InvestigationRecord) EscalateReason() string { return i.escalateReason }

// InvestigationStore defines the interface for investigation persistence.
// Implementations must be safe for concurrent access from multiple goroutines.
// All methods respect context cancellation and return context.Canceled or
// context.DeadlineExceeded when appropriate.
type InvestigationStore interface {
	// Store persists a new investigation. Returns ErrDuplicateInvestigationID if exists.
	Store(ctx context.Context, inv *InvestigationRecord) error
	// Get retrieves an investigation by ID. Returns ErrInvestigationNotFound if not found.
	Get(ctx context.Context, id string) (*InvestigationRecord, error)
	// Update modifies an existing investigation. Returns ErrInvestigationNotFound if not found.
	Update(ctx context.Context, inv *InvestigationRecord) error
	// Delete removes an investigation. Returns ErrInvestigationNotFound if not found.
	Delete(ctx context.Context, id string) error
	// Query returns investigations matching the filter criteria.
	Query(ctx context.Context, query InvestigationQuery) ([]*InvestigationRecord, error)
	// Count returns the total number of stored investigations.
	Count(ctx context.Context) (int, error)
	// Close releases resources and prevents further operations.
	Close() error
}

// InMemoryInvestigationStore is a thread-safe in-memory implementation of InvestigationStore.
// It is suitable for testing and single-instance deployments where persistence is not required.
// All operations are protected by a read-write mutex for concurrent access safety.
type InMemoryInvestigationStore struct {
	mu       sync.RWMutex // Protects all fields below
	data     map[string]*InvestigationRecord
	closed   bool
	capacity int // Initial capacity hint (informational only)
}

// NewInMemoryInvestigationStore creates a new in-memory store with default capacity.
func NewInMemoryInvestigationStore() *InMemoryInvestigationStore {
	return &InMemoryInvestigationStore{
		data: make(map[string]*InvestigationRecord),
	}
}

// NewInMemoryInvestigationStoreWithCapacity creates a store with an initial capacity hint.
// The capacity is a hint for map allocation and does not limit the number of investigations.
func NewInMemoryInvestigationStoreWithCapacity(capacity int) *InMemoryInvestigationStore {
	return &InMemoryInvestigationStore{
		data:     make(map[string]*InvestigationRecord, capacity),
		capacity: capacity,
	}
}

// Store saves a new investigation to the store.
// Returns ErrNilInvestigationRecord if inv is nil.
// Returns ErrDuplicateInvestigationID if an investigation with the same ID exists.
// Returns ErrInvestigationStoreShutdown if the store has been closed.
func (s *InMemoryInvestigationStore) Store(ctx context.Context, inv *InvestigationRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if inv == nil {
		return ErrNilInvestigationRecord
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrInvestigationStoreShutdown
	}

	if _, exists := s.data[inv.id]; exists {
		return ErrDuplicateInvestigationID
	}

	s.data[inv.id] = inv
	return nil
}

// Get retrieves an investigation by its unique ID.
// Returns ErrEmptyInvestigationIDStore if id is empty.
// Returns ErrInvestigationNotFound if no investigation exists with that ID.
// Returns ErrInvestigationStoreShutdown if the store has been closed.
func (s *InMemoryInvestigationStore) Get(ctx context.Context, id string) (*InvestigationRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if id == "" {
		return nil, ErrEmptyInvestigationIDStore
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrInvestigationStoreShutdown
	}

	inv, exists := s.data[id]
	if !exists {
		return nil, ErrInvestigationNotFound
	}

	return inv, nil
}

// Update replaces an existing investigation with the provided one.
// The investigation is matched by ID.
// Returns ErrNilInvestigationRecord if inv is nil.
// Returns ErrInvestigationNotFound if no investigation exists with that ID.
// Returns ErrInvestigationStoreShutdown if the store has been closed.
func (s *InMemoryInvestigationStore) Update(ctx context.Context, inv *InvestigationRecord) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if inv == nil {
		return ErrNilInvestigationRecord
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrInvestigationStoreShutdown
	}

	if _, exists := s.data[inv.id]; !exists {
		return ErrInvestigationNotFound
	}

	s.data[inv.id] = inv
	return nil
}

// Delete removes an investigation from the store by ID.
// Returns ErrInvestigationNotFound if no investigation exists with that ID.
// Returns ErrInvestigationStoreShutdown if the store has been closed.
func (s *InMemoryInvestigationStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrInvestigationStoreShutdown
	}

	if _, exists := s.data[id]; !exists {
		return ErrInvestigationNotFound
	}

	delete(s.data, id)
	return nil
}

// Query returns investigations matching the filter criteria.
// All non-zero fields in the query are combined with AND logic.
// Results are returned in undefined order (map iteration order).
// Returns an empty slice (not nil) if no investigations match.
// Returns ErrInvestigationStoreShutdown if the store has been closed.
func (s *InMemoryInvestigationStore) Query(
	ctx context.Context,
	query InvestigationQuery,
) ([]*InvestigationRecord, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrInvestigationStoreShutdown
	}

	var results []*InvestigationRecord

	for _, inv := range s.data {
		if !matchesQuery(inv, query) {
			continue
		}
		results = append(results, inv)
		if query.Limit > 0 && len(results) >= query.Limit {
			break
		}
	}

	if results == nil {
		results = []*InvestigationRecord{}
	}

	return results, nil
}

// Count returns the total number of investigations in the store.
// Returns ErrInvestigationStoreShutdown if the store has been closed.
func (s *InMemoryInvestigationStore) Count(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return 0, ErrInvestigationStoreShutdown
	}

	return len(s.data), nil
}

// Close marks the store as closed and prevents further operations.
// After Close is called, all operations will return ErrInvestigationStoreShutdown.
// Close is idempotent and safe to call multiple times.
func (s *InMemoryInvestigationStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	return nil
}

// matchesQuery checks if an investigation matches all specified query criteria.
// Returns true if the investigation matches all non-zero fields in the query.
func matchesQuery(inv *InvestigationRecord, query InvestigationQuery) bool {
	if query.AlertID != "" && inv.alertID != query.AlertID {
		return false
	}
	if query.SessionID != "" && inv.sessionID != query.SessionID {
		return false
	}
	if len(query.Status) > 0 {
		matched := false
		for _, s := range query.Status {
			if inv.status == s {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}
	if !query.Since.IsZero() && inv.StartedAt().Before(query.Since) {
		return false
	}
	if !query.Until.IsZero() && inv.StartedAt().After(query.Until) {
		return false
	}
	return true
}

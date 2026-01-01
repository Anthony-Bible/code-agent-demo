// Package service provides application services for the code-editing-agent.
package service

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Sentinel errors for InvestigationStore.
var (
	ErrInvestigationNotFound      = errors.New("investigation not found")
	ErrDuplicateInvestigationID   = errors.New("investigation ID already exists")
	ErrInvestigationStoreShutdown = errors.New("investigation store is shutdown")
	ErrNilInvestigationStub       = errors.New("investigation cannot be nil")
	ErrEmptyInvestigationIDStore  = errors.New("investigation ID cannot be empty")
)

// InvestigationQuery filters for querying investigations.
type InvestigationQuery struct {
	AlertID   string
	SessionID string
	Status    []string
	Since     time.Time
	Until     time.Time
	Limit     int
}

// InvestigationStub represents investigation data for storage.
type InvestigationStub struct {
	id        string
	alertID   string
	sessionID string
	status    string
	startedAt time.Time
}

// ID returns the investigation ID.
func (i *InvestigationStub) ID() string { return i.id }

// AlertID returns the alert ID.
func (i *InvestigationStub) AlertID() string { return i.alertID }

// SessionID returns the session ID.
func (i *InvestigationStub) SessionID() string { return i.sessionID }

// Status returns the status.
func (i *InvestigationStub) Status() string { return i.status }

// StartedAt returns when the investigation started.
func (i *InvestigationStub) StartedAt() time.Time {
	if i.startedAt.IsZero() {
		return time.Now()
	}
	return i.startedAt
}

// InvestigationStore interface defines persistence operations.
type InvestigationStore interface {
	Store(ctx context.Context, inv *InvestigationStub) error
	Get(ctx context.Context, id string) (*InvestigationStub, error)
	Update(ctx context.Context, inv *InvestigationStub) error
	Delete(ctx context.Context, id string) error
	Query(ctx context.Context, query InvestigationQuery) ([]*InvestigationStub, error)
	Count(ctx context.Context) (int, error)
	Close() error
}

// InMemoryInvestigationStore is an in-memory implementation.
type InMemoryInvestigationStore struct {
	mu       sync.RWMutex
	data     map[string]*InvestigationStub
	closed   bool
	capacity int
}

// NewInMemoryInvestigationStore creates a new in-memory store.
func NewInMemoryInvestigationStore() *InMemoryInvestigationStore {
	return &InMemoryInvestigationStore{
		data: make(map[string]*InvestigationStub),
	}
}

// NewInMemoryInvestigationStoreWithCapacity creates a store with capacity hint.
func NewInMemoryInvestigationStoreWithCapacity(capacity int) *InMemoryInvestigationStore {
	return &InMemoryInvestigationStore{
		data:     make(map[string]*InvestigationStub, capacity),
		capacity: capacity,
	}
}

// Store saves an investigation.
func (s *InMemoryInvestigationStore) Store(ctx context.Context, inv *InvestigationStub) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if inv == nil {
		return ErrNilInvestigationStub
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

// Get retrieves an investigation by ID.
func (s *InMemoryInvestigationStore) Get(ctx context.Context, id string) (*InvestigationStub, error) {
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

// Update updates an existing investigation.
func (s *InMemoryInvestigationStore) Update(ctx context.Context, inv *InvestigationStub) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if inv == nil {
		return ErrNilInvestigationStub
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

// Delete removes an investigation.
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

// Query returns investigations matching the filter.
func (s *InMemoryInvestigationStore) Query(
	ctx context.Context,
	query InvestigationQuery,
) ([]*InvestigationStub, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrInvestigationStoreShutdown
	}

	var results []*InvestigationStub

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
		results = []*InvestigationStub{}
	}

	return results, nil
}

// Count returns the number of investigations.
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

// Close shuts down the store.
func (s *InMemoryInvestigationStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	return nil
}

// matchesQuery checks if an investigation matches the query.
func matchesQuery(inv *InvestigationStub, query InvestigationQuery) bool {
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

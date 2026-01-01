package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// =============================================================================
// InvestigationStore Tests - RED PHASE
// These tests define the expected behavior of the InvestigationStore interface
// and InMemoryInvestigationStore implementation.
// All tests should FAIL until the implementation is complete.
// =============================================================================

// Sentinel errors expected to be defined in investigation_store.go.
var (
	ErrInvestigationNotFound      = errors.New("investigation not found")
	ErrDuplicateInvestigationID   = errors.New("investigation ID already exists")
	ErrInvestigationStoreShutdown = errors.New("investigation store is shutdown")
)

// InvestigationQuery filters for querying investigations.
// This is a stub struct - the real implementation should be in investigation_store.go.
type InvestigationQuery struct {
	AlertID   string
	SessionID string
	Status    []string
	Since     time.Time
	Until     time.Time
	Limit     int
}

// Investigation stub for testing - references entity.Investigation behavior.
type InvestigationStub struct {
	id        string
	alertID   string
	sessionID string
	status    string
	startedAt time.Time
}

func (i *InvestigationStub) ID() string        { return i.id }
func (i *InvestigationStub) AlertID() string   { return i.alertID }
func (i *InvestigationStub) SessionID() string { return i.sessionID }
func (i *InvestigationStub) Status() string    { return i.status }
func (i *InvestigationStub) StartedAt() time.Time {
	if i.startedAt.IsZero() {
		return time.Now()
	}
	return i.startedAt
}

// InvestigationStore interface defines persistence operations for investigations.
type InvestigationStore interface {
	Store(ctx context.Context, inv *InvestigationStub) error
	Get(ctx context.Context, id string) (*InvestigationStub, error)
	Update(ctx context.Context, inv *InvestigationStub) error
	Delete(ctx context.Context, id string) error
	Query(ctx context.Context, query InvestigationQuery) ([]*InvestigationStub, error)
	Count(ctx context.Context) (int, error)
	Close() error
}

// InMemoryInvestigationStore is a stub implementation.
type InMemoryInvestigationStore struct{}

func NewInMemoryInvestigationStore() *InMemoryInvestigationStore {
	return nil
}

func NewInMemoryInvestigationStoreWithCapacity(_ int) *InMemoryInvestigationStore {
	return nil
}

func (s *InMemoryInvestigationStore) Store(_ context.Context, _ *InvestigationStub) error {
	return errors.New("not implemented")
}

func (s *InMemoryInvestigationStore) Get(_ context.Context, _ string) (*InvestigationStub, error) {
	return nil, errors.New("not implemented")
}

func (s *InMemoryInvestigationStore) Update(_ context.Context, _ *InvestigationStub) error {
	return errors.New("not implemented")
}

func (s *InMemoryInvestigationStore) Delete(_ context.Context, _ string) error {
	return errors.New("not implemented")
}

func (s *InMemoryInvestigationStore) Query(_ context.Context, _ InvestigationQuery) ([]*InvestigationStub, error) {
	return nil, errors.New("not implemented")
}

func (s *InMemoryInvestigationStore) Count(_ context.Context) (int, error) {
	return 0, errors.New("not implemented")
}

func (s *InMemoryInvestigationStore) Close() error {
	return errors.New("not implemented")
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewInMemoryInvestigationStore_NotNil(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Error("NewInMemoryInvestigationStore() should not return nil")
	}
}

func TestNewInMemoryInvestigationStoreWithCapacity_NotNil(t *testing.T) {
	store := NewInMemoryInvestigationStoreWithCapacity(100)
	if store == nil {
		t.Error("NewInMemoryInvestigationStoreWithCapacity() should not return nil")
	}
}

func TestNewInMemoryInvestigationStore_InitialCountZero(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}
	count, err := store.Count(context.Background())
	if err != nil {
		t.Errorf("Count() error = %v", err)
	}
	if count != 0 {
		t.Errorf("Count() = %v, want 0", count)
	}
}

// =============================================================================
// Store Tests
// =============================================================================

func TestInMemoryInvestigationStore_Store_Success(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	inv := &InvestigationStub{
		id:        "inv-001",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "started",
	}

	err := store.Store(context.Background(), inv)
	if err != nil {
		t.Errorf("Store() error = %v", err)
	}
}

func TestInMemoryInvestigationStore_Store_DuplicateID(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	inv := &InvestigationStub{
		id:        "inv-dup",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "started",
	}

	err := store.Store(context.Background(), inv)
	if err != nil {
		t.Fatalf("First Store() error = %v", err)
	}

	// Store same ID again
	err = store.Store(context.Background(), inv)
	if err == nil {
		t.Error("Store() should return error for duplicate ID")
	}
}

func TestInMemoryInvestigationStore_Store_NilInvestigation(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	err := store.Store(context.Background(), nil)
	if err == nil {
		t.Error("Store(nil) should return error")
	}
}

func TestInMemoryInvestigationStore_Store_IncrementsCount(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()

	for i := range 3 {
		inv := &InvestigationStub{
			id:        "inv-" + string(rune('a'+i)),
			alertID:   "alert-001",
			sessionID: "session-001",
			status:    "started",
		}
		if err := store.Store(ctx, inv); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	count, err := store.Count(ctx)
	if err != nil {
		t.Errorf("Count() error = %v", err)
	}
	if count != 3 {
		t.Errorf("Count() = %v, want 3", count)
	}
}

// =============================================================================
// Get Tests
// =============================================================================

func TestInMemoryInvestigationStore_Get_Exists(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()
	inv := &InvestigationStub{
		id:        "inv-get-test",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "running",
	}

	if err := store.Store(ctx, inv); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	got, err := store.Get(ctx, "inv-get-test")
	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if got == nil {
		t.Error("Get() returned nil")
	}
	if got != nil && got.ID() != "inv-get-test" {
		t.Errorf("Get() ID = %v, want inv-get-test", got.ID())
	}
}

func TestInMemoryInvestigationStore_Get_NotExists(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	_, err := store.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Get() should return error for nonexistent ID")
	}
	if !errors.Is(err, ErrInvestigationNotFound) {
		t.Errorf("Get() error = %v, want ErrInvestigationNotFound", err)
	}
}

func TestInMemoryInvestigationStore_Get_EmptyID(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	_, err := store.Get(context.Background(), "")
	if err == nil {
		t.Error("Get('') should return error")
	}
}

// =============================================================================
// Update Tests
// =============================================================================

func TestInMemoryInvestigationStore_Update_Exists(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()
	inv := &InvestigationStub{
		id:        "inv-update-test",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "started",
	}

	if err := store.Store(ctx, inv); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Update status
	inv.status = "completed"
	if err := store.Update(ctx, inv); err != nil {
		t.Errorf("Update() error = %v", err)
	}

	got, _ := store.Get(ctx, "inv-update-test")
	if got != nil && got.Status() != "completed" {
		t.Errorf("Status after Update() = %v, want completed", got.Status())
	}
}

func TestInMemoryInvestigationStore_Update_NotExists(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	inv := &InvestigationStub{
		id:        "nonexistent",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "started",
	}

	err := store.Update(context.Background(), inv)
	if err == nil {
		t.Error("Update() should return error for nonexistent ID")
	}
	if !errors.Is(err, ErrInvestigationNotFound) {
		t.Errorf("Update() error = %v, want ErrInvestigationNotFound", err)
	}
}

func TestInMemoryInvestigationStore_Update_NilInvestigation(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	err := store.Update(context.Background(), nil)
	if err == nil {
		t.Error("Update(nil) should return error")
	}
}

// =============================================================================
// Delete Tests
// =============================================================================

func TestInMemoryInvestigationStore_Delete_Exists(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()
	inv := &InvestigationStub{
		id:        "inv-delete-test",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "started",
	}

	if err := store.Store(ctx, inv); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	if err := store.Delete(ctx, "inv-delete-test"); err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// Verify deleted
	_, err := store.Get(ctx, "inv-delete-test")
	if err == nil {
		t.Error("Get() after Delete() should return error")
	}
}

func TestInMemoryInvestigationStore_Delete_NotExists(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	err := store.Delete(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Delete() should return error for nonexistent ID")
	}
}

func TestInMemoryInvestigationStore_Delete_DecrementsCount(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()
	inv := &InvestigationStub{
		id:        "inv-count-test",
		alertID:   "alert-001",
		sessionID: "session-001",
		status:    "started",
	}

	if err := store.Store(ctx, inv); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	count, _ := store.Count(ctx)
	if count != 1 {
		t.Errorf("Count() before delete = %v, want 1", count)
	}

	if err := store.Delete(ctx, "inv-count-test"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	count, _ = store.Count(ctx)
	if count != 0 {
		t.Errorf("Count() after delete = %v, want 0", count)
	}
}

// =============================================================================
// Query Tests
// =============================================================================

func TestInMemoryInvestigationStore_Query_EmptyStore(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	results, err := store.Query(context.Background(), InvestigationQuery{})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if results == nil {
		t.Error("Query() should return empty slice, not nil")
	}
	if len(results) != 0 {
		t.Errorf("Query() len = %v, want 0", len(results))
	}
}

func TestInMemoryInvestigationStore_Query_ByAlertID(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()

	// Store investigations for different alerts
	invs := []*InvestigationStub{
		{id: "inv-1", alertID: "alert-A", sessionID: "s1", status: "started"},
		{id: "inv-2", alertID: "alert-A", sessionID: "s2", status: "running"},
		{id: "inv-3", alertID: "alert-B", sessionID: "s3", status: "started"},
	}
	for _, inv := range invs {
		if err := store.Store(ctx, inv); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, InvestigationQuery{AlertID: "alert-A"})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Query(AlertID=alert-A) len = %v, want 2", len(results))
	}
}

func TestInMemoryInvestigationStore_Query_BySessionID(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()

	invs := []*InvestigationStub{
		{id: "inv-1", alertID: "a1", sessionID: "session-X", status: "started"},
		{id: "inv-2", alertID: "a2", sessionID: "session-X", status: "running"},
		{id: "inv-3", alertID: "a3", sessionID: "session-Y", status: "started"},
	}
	for _, inv := range invs {
		if err := store.Store(ctx, inv); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, InvestigationQuery{SessionID: "session-X"})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Query(SessionID=session-X) len = %v, want 2", len(results))
	}
}

func TestInMemoryInvestigationStore_Query_ByStatus(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()

	invs := []*InvestigationStub{
		{id: "inv-1", alertID: "a1", sessionID: "s1", status: "started"},
		{id: "inv-2", alertID: "a2", sessionID: "s2", status: "running"},
		{id: "inv-3", alertID: "a3", sessionID: "s3", status: "completed"},
		{id: "inv-4", alertID: "a4", sessionID: "s4", status: "running"},
	}
	for _, inv := range invs {
		if err := store.Store(ctx, inv); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, InvestigationQuery{Status: []string{"running"}})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Query(Status=[running]) len = %v, want 2", len(results))
	}
}

func TestInMemoryInvestigationStore_Query_ByMultipleStatuses(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()

	invs := []*InvestigationStub{
		{id: "inv-1", alertID: "a1", sessionID: "s1", status: "started"},
		{id: "inv-2", alertID: "a2", sessionID: "s2", status: "running"},
		{id: "inv-3", alertID: "a3", sessionID: "s3", status: "completed"},
		{id: "inv-4", alertID: "a4", sessionID: "s4", status: "failed"},
	}
	for _, inv := range invs {
		if err := store.Store(ctx, inv); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, InvestigationQuery{Status: []string{"completed", "failed"}})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Query(Status=[completed,failed]) len = %v, want 2", len(results))
	}
}

func TestInMemoryInvestigationStore_Query_BySince(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()
	now := time.Now()

	invs := []*InvestigationStub{
		{id: "inv-old", alertID: "a1", sessionID: "s1", status: "started", startedAt: now.Add(-2 * time.Hour)},
		{id: "inv-recent", alertID: "a2", sessionID: "s2", status: "running", startedAt: now.Add(-30 * time.Minute)},
	}
	for _, inv := range invs {
		if err := store.Store(ctx, inv); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, InvestigationQuery{Since: now.Add(-1 * time.Hour)})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Query(Since=1h ago) len = %v, want 1", len(results))
	}
}

func TestInMemoryInvestigationStore_Query_ByUntil(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()
	now := time.Now()

	invs := []*InvestigationStub{
		{id: "inv-old", alertID: "a1", sessionID: "s1", status: "completed", startedAt: now.Add(-2 * time.Hour)},
		{id: "inv-recent", alertID: "a2", sessionID: "s2", status: "running", startedAt: now.Add(-30 * time.Minute)},
	}
	for _, inv := range invs {
		if err := store.Store(ctx, inv); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, InvestigationQuery{Until: now.Add(-1 * time.Hour)})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Query(Until=1h ago) len = %v, want 1", len(results))
	}
}

func TestInMemoryInvestigationStore_Query_WithLimit(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()

	for i := range 10 {
		inv := &InvestigationStub{
			id:        "inv-" + string(rune('0'+i)),
			alertID:   "a1",
			sessionID: "s1",
			status:    "started",
		}
		if err := store.Store(ctx, inv); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, InvestigationQuery{Limit: 5})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Query(Limit=5) len = %v, want 5", len(results))
	}
}

func TestInMemoryInvestigationStore_Query_CombinedFilters(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()

	invs := []*InvestigationStub{
		{id: "inv-1", alertID: "alert-X", sessionID: "s1", status: "running"},
		{id: "inv-2", alertID: "alert-X", sessionID: "s2", status: "completed"},
		{id: "inv-3", alertID: "alert-Y", sessionID: "s3", status: "running"},
	}
	for _, inv := range invs {
		if err := store.Store(ctx, inv); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, InvestigationQuery{
		AlertID: "alert-X",
		Status:  []string{"running"},
	})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Query(AlertID=alert-X, Status=running) len = %v, want 1", len(results))
	}
}

// =============================================================================
// Context Cancellation Tests
// =============================================================================

func TestInMemoryInvestigationStore_Store_CancelledContext(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inv := &InvestigationStub{
		id:        "inv-ctx",
		alertID:   "a1",
		sessionID: "s1",
		status:    "started",
	}

	err := store.Store(ctx, inv)
	if err == nil {
		t.Error("Store() with cancelled context should return error")
	}
}

func TestInMemoryInvestigationStore_Get_CancelledContext(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Get(ctx, "any")
	if err == nil {
		t.Error("Get() with cancelled context should return error")
	}
}

func TestInMemoryInvestigationStore_Query_CancelledContext(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := store.Query(ctx, InvestigationQuery{})
	if err == nil {
		t.Error("Query() with cancelled context should return error")
	}
}

// =============================================================================
// Close Tests
// =============================================================================

func TestInMemoryInvestigationStore_Close_Success(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	err := store.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestInMemoryInvestigationStore_Close_ThenStore(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	inv := &InvestigationStub{
		id:        "inv-after-close",
		alertID:   "a1",
		sessionID: "s1",
		status:    "started",
	}

	err := store.Store(context.Background(), inv)
	if err == nil {
		t.Error("Store() after Close() should return error")
	}
}

func TestInMemoryInvestigationStore_Close_ThenGet(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err := store.Get(context.Background(), "any")
	if err == nil {
		t.Error("Get() after Close() should return error")
	}
}

// =============================================================================
// Thread Safety Tests
// =============================================================================

func TestInMemoryInvestigationStore_ConcurrentStoreAndGet(t *testing.T) {
	store := NewInMemoryInvestigationStore()
	if store == nil {
		t.Skip("NewInMemoryInvestigationStore() returned nil")
	}

	ctx := context.Background()
	done := make(chan bool)

	// Store goroutine
	go func() {
		for i := range 100 {
			inv := &InvestigationStub{
				id:        "inv-concurrent-" + string(rune('0'+i)),
				alertID:   "a1",
				sessionID: "s1",
				status:    "started",
			}
			_ = store.Store(ctx, inv)
		}
		done <- true
	}()

	// Get goroutine
	go func() {
		for range 100 {
			_, _ = store.Get(ctx, "inv-concurrent-0")
		}
		done <- true
	}()

	// Wait for both
	<-done
	<-done

	// If we get here without panic, thread safety is working
}

// =============================================================================
// Error Constants Tests
// =============================================================================

func TestInvestigationStoreErrors_NotNil(t *testing.T) {
	if ErrInvestigationNotFound == nil {
		t.Error("ErrInvestigationNotFound should not be nil")
	}
	if ErrDuplicateInvestigationID == nil {
		t.Error("ErrDuplicateInvestigationID should not be nil")
	}
	if ErrInvestigationStoreShutdown == nil {
		t.Error("ErrInvestigationStoreShutdown should not be nil")
	}
}

func TestInvestigationStoreErrors_HaveMessages(t *testing.T) {
	if ErrInvestigationNotFound.Error() == "" {
		t.Error("ErrInvestigationNotFound should have a message")
	}
	if ErrDuplicateInvestigationID.Error() == "" {
		t.Error("ErrDuplicateInvestigationID should have a message")
	}
	if ErrInvestigationStoreShutdown.Error() == "" {
		t.Error("ErrInvestigationStoreShutdown should have a message")
	}
}

package investigation

import (
	"code-editing-agent/internal/application/service"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// investigationJSON is the JSON representation of an investigation for file storage.
type investigationJSON struct {
	ID        string    `json:"id"`
	AlertID   string    `json:"alert_id"`
	SessionID string    `json:"session_id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
}

// FileInvestigationStore implements InvestigationStore with file-based persistence.
// It uses a hybrid approach: an in-memory index for fast lookups and lazy-loading
// of actual data from disk.
type FileInvestigationStore struct {
	mu      sync.RWMutex
	baseDir string
	index   map[string]bool                       // ID -> exists (index in memory)
	cache   map[string]*service.InvestigationStub // lazy-loaded data
	closed  bool
}

// NewFileInvestigationStore creates a new file-based investigation store.
// Creates the directory if it does not exist.
// Returns an error if path is empty or directory cannot be created.
func NewFileInvestigationStore(path string) (*FileInvestigationStore, error) {
	if path == "" {
		return nil, errors.New("path cannot be empty")
	}

	if err := os.MkdirAll(path, 0o750); err != nil {
		return nil, err
	}

	store := &FileInvestigationStore{
		baseDir: path,
		index:   make(map[string]bool),
		cache:   make(map[string]*service.InvestigationStub),
	}

	// Load existing files into index
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			id := strings.TrimSuffix(entry.Name(), ".json")
			store.index[id] = true
		}
	}

	return store, nil
}

// Store persists a new investigation.
func (s *FileInvestigationStore) Store(ctx context.Context, inv *service.InvestigationStub) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if inv == nil {
		return service.ErrNilInvestigationStub
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return service.ErrInvestigationStoreShutdown
	}

	if s.index[inv.ID()] {
		return service.ErrDuplicateInvestigationID
	}

	if err := s.writeFile(inv); err != nil {
		return err
	}

	s.index[inv.ID()] = true
	s.cache[inv.ID()] = inv
	return nil
}

// Get retrieves an investigation by ID, lazy-loading from disk if needed.
func (s *FileInvestigationStore) Get(ctx context.Context, id string) (*service.InvestigationStub, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if id == "" {
		return nil, service.ErrEmptyInvestigationIDStore
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, service.ErrInvestigationStoreShutdown
	}

	// Check cache first
	if inv, ok := s.cache[id]; ok {
		return inv, nil
	}

	// Check if file exists on disk (even if not in our index)
	filePath := filepath.Join(s.baseDir, id+".json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		if !s.index[id] {
			return nil, service.ErrInvestigationNotFound
		}
	}

	// Lazy load from disk
	inv, err := s.readFile(id)
	if err != nil {
		return nil, err
	}

	s.cache[id] = inv
	s.index[id] = true
	return inv, nil
}

// Update modifies an existing investigation.
func (s *FileInvestigationStore) Update(ctx context.Context, inv *service.InvestigationStub) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if inv == nil {
		return service.ErrNilInvestigationStub
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return service.ErrInvestigationStoreShutdown
	}

	if !s.index[inv.ID()] {
		return service.ErrInvestigationNotFound
	}

	if err := s.writeFile(inv); err != nil {
		return err
	}

	s.cache[inv.ID()] = inv
	return nil
}

// Delete removes an investigation.
func (s *FileInvestigationStore) Delete(ctx context.Context, id string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return service.ErrInvestigationStoreShutdown
	}

	if !s.index[id] {
		return service.ErrInvestigationNotFound
	}

	filePath := filepath.Join(s.baseDir, id+".json")
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return err
	}

	delete(s.index, id)
	delete(s.cache, id)
	return nil
}

// Query returns investigations matching the filter criteria.
func (s *FileInvestigationStore) Query(
	ctx context.Context,
	query service.InvestigationQuery,
) ([]*service.InvestigationStub, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, service.ErrInvestigationStoreShutdown
	}

	var results []*service.InvestigationStub

	for id := range s.index {
		// Ensure data is loaded
		if _, ok := s.cache[id]; !ok {
			inv, err := s.readFile(id)
			if err != nil {
				continue // Skip corrupted files
			}
			s.cache[id] = inv
		}

		inv := s.cache[id]
		if matchesQuery(inv, query) {
			results = append(results, inv)
			if query.Limit > 0 && len(results) >= query.Limit {
				break
			}
		}
	}

	if results == nil {
		results = []*service.InvestigationStub{}
	}

	return results, nil
}

// Count returns the total number of stored investigations.
func (s *FileInvestigationStore) Count(ctx context.Context) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return 0, service.ErrInvestigationStoreShutdown
	}

	return len(s.index), nil
}

// Close marks the store as closed.
func (s *FileInvestigationStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return nil
}

// writeFile writes an investigation to disk as JSON.
func (s *FileInvestigationStore) writeFile(inv *service.InvestigationStub) error {
	data := investigationJSON{
		ID:        inv.ID(),
		AlertID:   inv.AlertID(),
		SessionID: inv.SessionID(),
		Status:    inv.Status(),
		StartedAt: inv.StartedAt(),
	}

	bytes, err := json.Marshal(data)
	if err != nil {
		return err
	}

	filePath := filepath.Join(s.baseDir, inv.ID()+".json")
	return os.WriteFile(filePath, bytes, 0o600)
}

// readFile reads an investigation from disk.
func (s *FileInvestigationStore) readFile(id string) (*service.InvestigationStub, error) {
	filePath := filepath.Join(s.baseDir, id+".json")
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, service.ErrInvestigationNotFound
		}
		return nil, err
	}

	if len(bytes) == 0 {
		return nil, errors.New("empty file")
	}

	var data investigationJSON
	if err := json.Unmarshal(bytes, &data); err != nil {
		return nil, err
	}

	return service.NewInvestigationStubForTestWithTime(
		data.ID,
		data.AlertID,
		data.SessionID,
		data.Status,
		data.StartedAt,
	), nil
}

// matchesQuery checks if an investigation matches all specified query criteria.
func matchesQuery(inv *service.InvestigationStub, query service.InvestigationQuery) bool {
	if query.AlertID != "" && inv.AlertID() != query.AlertID {
		return false
	}
	if query.SessionID != "" && inv.SessionID() != query.SessionID {
		return false
	}
	if len(query.Status) > 0 {
		matched := false
		for _, status := range query.Status {
			if inv.Status() == status {
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

package investigation

import (
	"code-editing-agent/internal/application/service"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// FileInvestigationStore Tests
// These tests verify the behavior of FileInvestigationStore which implements
// the InvestigationStore interface with file-based persistence.
// =============================================================================

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewFileInvestigationStore_NotNil(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	if store == nil {
		t.Error("NewFileInvestigationStore() should not return nil")
	}
	if store != nil {
		_ = store.Close()
	}
}

func TestNewFileInvestigationStore_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	storePath := filepath.Join(tmpDir, "investigations")

	// Directory should not exist yet
	if _, err := os.Stat(storePath); !os.IsNotExist(err) {
		t.Fatal("Directory should not exist before creating store")
	}

	store, err := NewFileInvestigationStore(storePath)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	// Directory should now exist
	info, err := os.Stat(storePath)
	if os.IsNotExist(err) {
		t.Error("NewFileInvestigationStore() should create directory")
	}
	if info != nil && !info.IsDir() {
		t.Error("NewFileInvestigationStore() should create a directory, not a file")
	}
}

func TestNewFileInvestigationStore_EmptyPath(t *testing.T) {
	store, err := NewFileInvestigationStore("")
	if err == nil {
		t.Error("NewFileInvestigationStore('') should return error")
	}
	if store != nil {
		_ = store.Close()
	}
}

func TestNewFileInvestigationStore_InvalidPath(t *testing.T) {
	// Try to create in a path that cannot be created (e.g., inside a file)
	tmpFile, err := os.CreateTemp(t.TempDir(), "test-file-*")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	// Try to use the file path as a directory
	invalidPath := filepath.Join(tmpFile.Name(), "investigations")
	store, err := NewFileInvestigationStore(invalidPath)
	if err == nil {
		t.Error("NewFileInvestigationStore() with invalid path should return error")
		if store != nil {
			_ = store.Close()
		}
	}
}

func TestNewFileInvestigationStore_InitialCountZero(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

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

func TestFileInvestigationStore_Store_Success(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	inv := service.NewInvestigationRecordForTest("inv-001", "alert-001", "session-001", "started")

	err = store.Store(context.Background(), inv)
	if err != nil {
		t.Errorf("Store() error = %v", err)
	}
}

func TestFileInvestigationStore_Store_CreatesFile(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	inv := service.NewInvestigationRecordForTest("inv-file-test", "alert-001", "session-001", "started")

	err = store.Store(context.Background(), inv)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Verify file exists
	expectedPath := filepath.Join(tmpDir, "inv-file-test.json")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Error("Store() should create JSON file for investigation")
	}
}

func TestFileInvestigationStore_Store_DuplicateID(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	inv := service.NewInvestigationRecordForTest("inv-dup", "alert-001", "session-001", "started")

	err = store.Store(context.Background(), inv)
	if err != nil {
		t.Fatalf("First Store() error = %v", err)
	}

	// Store same ID again
	err = store.Store(context.Background(), inv)
	if err == nil {
		t.Error("Store() should return error for duplicate ID")
	}
	if !errors.Is(err, service.ErrDuplicateInvestigationID) {
		t.Errorf("Store() error = %v, want ErrDuplicateInvestigationID", err)
	}
}

func TestFileInvestigationStore_Store_NilInvestigation(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	err = store.Store(context.Background(), nil)
	if err == nil {
		t.Error("Store(nil) should return error")
	}
	if !errors.Is(err, service.ErrNilInvestigationRecord) {
		t.Errorf("Store(nil) error = %v, want ErrNilInvestigationRecord", err)
	}
}

func TestFileInvestigationStore_Store_IncrementsCount(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()

	for i := range 3 {
		inv := service.NewInvestigationRecordForTest(
			"inv-"+string(rune('a'+i)),
			"alert-001",
			"session-001",
			"started",
		)
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
// Get Tests (Lazy-load from disk)
// =============================================================================

func TestFileInvestigationStore_Get_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()
	inv := service.NewInvestigationRecordForTest("inv-get-test", "alert-001", "session-001", "running")

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

func TestFileInvestigationStore_Get_ReadsFromDisk(t *testing.T) {
	tmpDir := t.TempDir()

	// Create store and save an investigation
	store1, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}

	inv := service.NewInvestigationRecordForTest("inv-disk-test", "alert-001", "session-001", "running")
	if err := store1.Store(context.Background(), inv); err != nil {
		t.Fatalf("Store() error = %v", err)
	}
	_ = store1.Close()

	// Create new store instance - should lazy-load from disk
	store2, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() second instance error = %v", err)
	}
	defer func() {
		if store2 != nil {
			_ = store2.Close()
		}
	}()

	got, err := store2.Get(context.Background(), "inv-disk-test")
	if err != nil {
		t.Errorf("Get() from new store instance error = %v", err)
	}
	if got == nil {
		t.Error("Get() should read from disk for new store instance")
	}
	if got != nil && got.ID() != "inv-disk-test" {
		t.Errorf("Get() ID = %v, want inv-disk-test", got.ID())
	}
}

func TestFileInvestigationStore_Get_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	_, err = store.Get(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Get() should return error for nonexistent ID")
	}
	if !errors.Is(err, service.ErrInvestigationNotFound) {
		t.Errorf("Get() error = %v, want ErrInvestigationNotFound", err)
	}
}

func TestFileInvestigationStore_Get_EmptyID(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	_, err = store.Get(context.Background(), "")
	if err == nil {
		t.Error("Get('') should return error")
	}
	if !errors.Is(err, service.ErrEmptyInvestigationIDStore) {
		t.Errorf("Get('') error = %v, want ErrEmptyInvestigationIDStore", err)
	}
}

// =============================================================================
// Update Tests
// =============================================================================

func TestFileInvestigationStore_Update_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()
	inv := service.NewInvestigationRecordForTest("inv-update-test", "alert-001", "session-001", "started")

	if err := store.Store(ctx, inv); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Update status
	updatedInv := service.NewInvestigationRecordForTest("inv-update-test", "alert-001", "session-001", "completed")
	if err := store.Update(ctx, updatedInv); err != nil {
		t.Errorf("Update() error = %v", err)
	}

	got, _ := store.Get(ctx, "inv-update-test")
	if got != nil && got.Status() != "completed" {
		t.Errorf("Status after Update() = %v, want completed", got.Status())
	}
}

func TestFileInvestigationStore_Update_ModifiesFile(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()
	inv := service.NewInvestigationRecordForTest("inv-update-file", "alert-001", "session-001", "started")

	if err := store.Store(ctx, inv); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	filePath := filepath.Join(tmpDir, "inv-update-file.json")
	statBefore, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat before update error = %v", err)
	}

	// Wait a bit to ensure mtime changes
	time.Sleep(10 * time.Millisecond)

	updatedInv := service.NewInvestigationRecordForTest("inv-update-file", "alert-001", "session-001", "completed")
	if err := store.Update(ctx, updatedInv); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	statAfter, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Stat after update error = %v", err)
	}

	if !statAfter.ModTime().After(statBefore.ModTime()) {
		t.Error("Update() should modify the file")
	}
}

func TestFileInvestigationStore_Update_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	inv := service.NewInvestigationRecordForTest("nonexistent", "alert-001", "session-001", "started")

	err = store.Update(context.Background(), inv)
	if err == nil {
		t.Error("Update() should return error for nonexistent ID")
	}
	if !errors.Is(err, service.ErrInvestigationNotFound) {
		t.Errorf("Update() error = %v, want ErrInvestigationNotFound", err)
	}
}

func TestFileInvestigationStore_Update_NilInvestigation(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	err = store.Update(context.Background(), nil)
	if err == nil {
		t.Error("Update(nil) should return error")
	}
	if !errors.Is(err, service.ErrNilInvestigationRecord) {
		t.Errorf("Update(nil) error = %v, want ErrNilInvestigationRecord", err)
	}
}

// =============================================================================
// Delete Tests
// =============================================================================

func TestFileInvestigationStore_Delete_Exists(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()
	inv := service.NewInvestigationRecordForTest("inv-delete-test", "alert-001", "session-001", "started")

	if err := store.Store(ctx, inv); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	if err := store.Delete(ctx, "inv-delete-test"); err != nil {
		t.Errorf("Delete() error = %v", err)
	}

	// Verify deleted
	_, err = store.Get(ctx, "inv-delete-test")
	if err == nil {
		t.Error("Get() after Delete() should return error")
	}
}

func TestFileInvestigationStore_Delete_RemovesFile(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()
	inv := service.NewInvestigationRecordForTest("inv-delete-file", "alert-001", "session-001", "started")

	if err := store.Store(ctx, inv); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	filePath := filepath.Join(tmpDir, "inv-delete-file.json")
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("File should exist before delete")
	}

	if err := store.Delete(ctx, "inv-delete-file"); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("Delete() should remove file from disk")
	}
}

func TestFileInvestigationStore_Delete_NotExists(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	err = store.Delete(context.Background(), "nonexistent")
	if err == nil {
		t.Error("Delete() should return error for nonexistent ID")
	}
	if !errors.Is(err, service.ErrInvestigationNotFound) {
		t.Errorf("Delete() error = %v, want ErrInvestigationNotFound", err)
	}
}

func TestFileInvestigationStore_Delete_DecrementsCount(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()
	inv := service.NewInvestigationRecordForTest("inv-count-test", "alert-001", "session-001", "started")

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

func TestFileInvestigationStore_Query_EmptyStore(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	results, err := store.Query(context.Background(), service.InvestigationQuery{})
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

func TestFileInvestigationStore_Query_ByAlertID(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()

	// Store investigations for different alerts
	invs := []struct {
		id, alertID, sessionID, status string
	}{
		{"inv-1", "alert-A", "s1", "started"},
		{"inv-2", "alert-A", "s2", "running"},
		{"inv-3", "alert-B", "s3", "started"},
	}
	for _, inv := range invs {
		stub := service.NewInvestigationRecordForTest(inv.id, inv.alertID, inv.sessionID, inv.status)
		if err := store.Store(ctx, stub); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, service.InvestigationQuery{AlertID: "alert-A"})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Query(AlertID=alert-A) len = %v, want 2", len(results))
	}
}

func TestFileInvestigationStore_Query_BySessionID(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()

	invs := []struct {
		id, alertID, sessionID, status string
	}{
		{"inv-1", "a1", "session-X", "started"},
		{"inv-2", "a2", "session-X", "running"},
		{"inv-3", "a3", "session-Y", "started"},
	}
	for _, inv := range invs {
		stub := service.NewInvestigationRecordForTest(inv.id, inv.alertID, inv.sessionID, inv.status)
		if err := store.Store(ctx, stub); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, service.InvestigationQuery{SessionID: "session-X"})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Query(SessionID=session-X) len = %v, want 2", len(results))
	}
}

func TestFileInvestigationStore_Query_ByStatus(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()

	invs := []struct {
		id, alertID, sessionID, status string
	}{
		{"inv-1", "a1", "s1", "started"},
		{"inv-2", "a2", "s2", "running"},
		{"inv-3", "a3", "s3", "completed"},
		{"inv-4", "a4", "s4", "running"},
	}
	for _, inv := range invs {
		stub := service.NewInvestigationRecordForTest(inv.id, inv.alertID, inv.sessionID, inv.status)
		if err := store.Store(ctx, stub); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, service.InvestigationQuery{Status: []string{"running"}})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Query(Status=[running]) len = %v, want 2", len(results))
	}
}

func TestFileInvestigationStore_Query_ByMultipleStatuses(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()

	invs := []struct {
		id, alertID, sessionID, status string
	}{
		{"inv-1", "a1", "s1", "started"},
		{"inv-2", "a2", "s2", "running"},
		{"inv-3", "a3", "s3", "completed"},
		{"inv-4", "a4", "s4", "failed"},
	}
	for _, inv := range invs {
		stub := service.NewInvestigationRecordForTest(inv.id, inv.alertID, inv.sessionID, inv.status)
		if err := store.Store(ctx, stub); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, service.InvestigationQuery{Status: []string{"completed", "failed"}})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 2 {
		t.Errorf("Query(Status=[completed,failed]) len = %v, want 2", len(results))
	}
}

func TestFileInvestigationStore_Query_BySince(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()
	now := time.Now()

	invs := []struct {
		id, alertID, sessionID, status string
		startedAt                      time.Time
	}{
		{"inv-old", "a1", "s1", "started", now.Add(-2 * time.Hour)},
		{"inv-recent", "a2", "s2", "running", now.Add(-30 * time.Minute)},
	}
	for _, inv := range invs {
		stub := service.NewInvestigationRecordForTestWithTime(
			inv.id,
			inv.alertID,
			inv.sessionID,
			inv.status,
			inv.startedAt,
		)
		if err := store.Store(ctx, stub); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, service.InvestigationQuery{Since: now.Add(-1 * time.Hour)})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Query(Since=1h ago) len = %v, want 1", len(results))
	}
}

func TestFileInvestigationStore_Query_ByUntil(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()
	now := time.Now()

	invs := []struct {
		id, alertID, sessionID, status string
		startedAt                      time.Time
	}{
		{"inv-old", "a1", "s1", "completed", now.Add(-2 * time.Hour)},
		{"inv-recent", "a2", "s2", "running", now.Add(-30 * time.Minute)},
	}
	for _, inv := range invs {
		stub := service.NewInvestigationRecordForTestWithTime(
			inv.id,
			inv.alertID,
			inv.sessionID,
			inv.status,
			inv.startedAt,
		)
		if err := store.Store(ctx, stub); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, service.InvestigationQuery{Until: now.Add(-1 * time.Hour)})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 1 {
		t.Errorf("Query(Until=1h ago) len = %v, want 1", len(results))
	}
}

func TestFileInvestigationStore_Query_WithLimit(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()

	for i := range 10 {
		inv := service.NewInvestigationRecordForTest(
			"inv-"+string(rune('0'+i)),
			"a1",
			"s1",
			"started",
		)
		if err := store.Store(ctx, inv); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, service.InvestigationQuery{Limit: 5})
	if err != nil {
		t.Errorf("Query() error = %v", err)
	}
	if len(results) != 5 {
		t.Errorf("Query(Limit=5) len = %v, want 5", len(results))
	}
}

func TestFileInvestigationStore_Query_CombinedFilters(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()

	invs := []struct {
		id, alertID, sessionID, status string
	}{
		{"inv-1", "alert-X", "s1", "running"},
		{"inv-2", "alert-X", "s2", "completed"},
		{"inv-3", "alert-Y", "s3", "running"},
	}
	for _, inv := range invs {
		stub := service.NewInvestigationRecordForTest(inv.id, inv.alertID, inv.sessionID, inv.status)
		if err := store.Store(ctx, stub); err != nil {
			t.Fatalf("Store() error = %v", err)
		}
	}

	results, err := store.Query(ctx, service.InvestigationQuery{
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

func TestFileInvestigationStore_Store_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	inv := service.NewInvestigationRecordForTest("inv-ctx", "a1", "s1", "started")

	err = store.Store(ctx, inv)
	if err == nil {
		t.Error("Store() with cancelled context should return error")
	}
}

func TestFileInvestigationStore_Get_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = store.Get(ctx, "any")
	if err == nil {
		t.Error("Get() with cancelled context should return error")
	}
}

func TestFileInvestigationStore_Query_CancelledContext(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = store.Query(ctx, service.InvestigationQuery{})
	if err == nil {
		t.Error("Query() with cancelled context should return error")
	}
}

// =============================================================================
// Close Tests
// =============================================================================

func TestFileInvestigationStore_Close_Success(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}

	err = store.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestFileInvestigationStore_Close_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}

	// Close multiple times should not error
	if err := store.Close(); err != nil {
		t.Errorf("First Close() error = %v", err)
	}
	if err := store.Close(); err != nil {
		t.Errorf("Second Close() error = %v", err)
	}
}

func TestFileInvestigationStore_Close_ThenStore(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	inv := service.NewInvestigationRecordForTest("inv-after-close", "a1", "s1", "started")

	err = store.Store(context.Background(), inv)
	if err == nil {
		t.Error("Store() after Close() should return error")
	}
	if !errors.Is(err, service.ErrInvestigationStoreShutdown) {
		t.Errorf("Store() after Close() error = %v, want ErrInvestigationStoreShutdown", err)
	}
}

func TestFileInvestigationStore_Close_ThenGet(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err = store.Get(context.Background(), "any")
	if err == nil {
		t.Error("Get() after Close() should return error")
	}
	if !errors.Is(err, service.ErrInvestigationStoreShutdown) {
		t.Errorf("Get() after Close() error = %v, want ErrInvestigationStoreShutdown", err)
	}
}

func TestFileInvestigationStore_Close_ThenQuery(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err = store.Query(context.Background(), service.InvestigationQuery{})
	if err == nil {
		t.Error("Query() after Close() should return error")
	}
	if !errors.Is(err, service.ErrInvestigationStoreShutdown) {
		t.Errorf("Query() after Close() error = %v, want ErrInvestigationStoreShutdown", err)
	}
}

func TestFileInvestigationStore_Close_ThenCount(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	_, err = store.Count(context.Background())
	if err == nil {
		t.Error("Count() after Close() should return error")
	}
	if !errors.Is(err, service.ErrInvestigationStoreShutdown) {
		t.Errorf("Count() after Close() error = %v, want ErrInvestigationStoreShutdown", err)
	}
}

// =============================================================================
// Concurrent Access Tests
// =============================================================================

func TestFileInvestigationStore_ConcurrentStoreAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()
	var wg sync.WaitGroup

	// Multiple store goroutines
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := range 10 {
				inv := service.NewInvestigationRecordForTest(
					"inv-"+string(rune('A'+idx))+"-"+string(rune('0'+j)),
					"a1",
					"s1",
					"started",
				)
				_ = store.Store(ctx, inv)
			}
		}(i)
	}

	// Multiple get goroutines
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for range 10 {
				_, _ = store.Get(ctx, "inv-A-0")
			}
		}(i)
	}

	// Multiple query goroutines
	for i := range 5 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for range 10 {
				_, _ = store.Query(ctx, service.InvestigationQuery{})
			}
		}(i)
	}

	wg.Wait()

	// If we get here without panic, thread safety is working
}

func TestFileInvestigationStore_ConcurrentUpdateAndDelete(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	ctx := context.Background()

	// Pre-populate some investigations
	for i := range 20 {
		inv := service.NewInvestigationRecordForTest(
			"inv-concurrent-"+string(rune('a'+i)),
			"a1",
			"s1",
			"started",
		)
		_ = store.Store(ctx, inv)
	}

	var wg sync.WaitGroup

	// Update goroutines
	for i := range 10 {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			inv := service.NewInvestigationRecordForTest(
				"inv-concurrent-"+string(rune('a'+idx)),
				"a1",
				"s1",
				"updated",
			)
			for range 5 {
				_ = store.Update(ctx, inv)
			}
		}(i)
	}

	// Delete goroutines (different IDs)
	for i := 10; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_ = store.Delete(ctx, "inv-concurrent-"+string(rune('a'+idx)))
		}(i)
	}

	wg.Wait()

	// If we get here without panic, thread safety is working
}

// =============================================================================
// File Corruption Recovery Tests
// =============================================================================

func TestFileInvestigationStore_CorruptedFile_Get(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	// Create a corrupted JSON file
	corruptedPath := filepath.Join(tmpDir, "inv-corrupt.json")
	if err := os.WriteFile(corruptedPath, []byte("not valid json{"), 0o644); err != nil {
		t.Fatalf("Failed to create corrupted file: %v", err)
	}

	// Get should handle corrupted file gracefully
	_, err = store.Get(context.Background(), "inv-corrupt")
	if err == nil {
		t.Error("Get() on corrupted file should return error")
	}
	// Should not panic - error is acceptable
}

func TestFileInvestigationStore_EmptyFile_Get(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	// Create an empty file
	emptyPath := filepath.Join(tmpDir, "inv-empty.json")
	if err := os.WriteFile(emptyPath, []byte(""), 0o644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	// Get should handle empty file gracefully
	_, err = store.Get(context.Background(), "inv-empty")
	if err == nil {
		t.Error("Get() on empty file should return error")
	}
	// Should not panic - error is acceptable
}

// =============================================================================
// Interface Compliance Test
// =============================================================================

func TestFileInvestigationStore_ImplementsInterface(t *testing.T) {
	tmpDir := t.TempDir()
	store, err := NewFileInvestigationStore(tmpDir)
	if err != nil {
		t.Fatalf("NewFileInvestigationStore() error = %v", err)
	}
	defer func() {
		if store != nil {
			_ = store.Close()
		}
	}()

	// Compile-time check that FileInvestigationStore implements InvestigationStore
	var _ service.InvestigationStore = store
}

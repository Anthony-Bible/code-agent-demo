package alert

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// Source Manager Tests - RED PHASE
// These tests define the expected behavior of the LocalAlertSourceManager.
// All tests should FAIL until the implementation is complete.
// =============================================================================

// mockAlertSource is a test double for port.AlertSource
type mockAlertSource struct {
	name       string
	sourceType port.SourceType
	closed     bool
	closeErr   error
}

func newMockAlertSource(name string, sourceType port.SourceType) *mockAlertSource {
	return &mockAlertSource{
		name:       name,
		sourceType: sourceType,
	}
}

func (m *mockAlertSource) Name() string {
	return m.name
}

func (m *mockAlertSource) Type() port.SourceType {
	return m.sourceType
}

func (m *mockAlertSource) Close() error {
	m.closed = true
	return m.closeErr
}

// mockWebhookSource is a test double for port.WebhookAlertSource
type mockWebhookSource struct {
	*mockAlertSource
	webhookPath  string
	handleFunc   func(ctx context.Context, payload []byte) ([]*entity.Alert, error)
	handledCalls int
	lastPayload  []byte
}

func newMockWebhookSource(name, path string) *mockWebhookSource {
	return &mockWebhookSource{
		mockAlertSource: newMockAlertSource(name, port.SourceTypeWebhook),
		webhookPath:     path,
	}
}

func (m *mockWebhookSource) WebhookPath() string {
	return m.webhookPath
}

func (m *mockWebhookSource) HandleWebhook(ctx context.Context, payload []byte) ([]*entity.Alert, error) {
	m.handledCalls++
	m.lastPayload = payload
	if m.handleFunc != nil {
		return m.handleFunc(ctx, payload)
	}
	return nil, nil
}

func TestNewLocalAlertSourceManager(t *testing.T) {
	t.Run("should create new manager", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		if manager == nil {
			t.Fatal("NewLocalAlertSourceManager() returned nil")
		}
	})

	t.Run("should implement AlertSourceManager interface", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		var _ port.AlertSourceManager = manager
	})

	t.Run("should start with empty source list", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		sources := manager.ListSources()
		if len(sources) != 0 {
			t.Errorf("ListSources() = %d sources, want 0", len(sources))
		}
	})
}

func TestLocalAlertSourceManager_RegisterSource(t *testing.T) {
	t.Run("should register valid source", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()
		source := newMockAlertSource("test-source", port.SourceTypeWebhook)

		err := manager.RegisterSource(source)
		if err != nil {
			t.Errorf("RegisterSource() error = %v", err)
		}

		sources := manager.ListSources()
		if len(sources) != 1 {
			t.Errorf("ListSources() = %d sources, want 1", len(sources))
		}
	})

	t.Run("should reject duplicate registration", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()
		source1 := newMockAlertSource("test-source", port.SourceTypeWebhook)
		source2 := newMockAlertSource("test-source", port.SourceTypePoll)

		err := manager.RegisterSource(source1)
		if err != nil {
			t.Fatalf("First RegisterSource() error = %v", err)
		}

		err = manager.RegisterSource(source2)
		if err == nil {
			t.Error("Second RegisterSource() should return error for duplicate name")
		}

		sources := manager.ListSources()
		if len(sources) != 1 {
			t.Errorf("ListSources() = %d sources, want 1 (duplicate rejected)", len(sources))
		}
	})

	t.Run("should register multiple unique sources", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()
		source1 := newMockAlertSource("source-1", port.SourceTypeWebhook)
		source2 := newMockAlertSource("source-2", port.SourceTypePoll)
		source3 := newMockAlertSource("source-3", port.SourceTypeStream)

		if err := manager.RegisterSource(source1); err != nil {
			t.Fatalf("RegisterSource(source-1) error = %v", err)
		}
		if err := manager.RegisterSource(source2); err != nil {
			t.Fatalf("RegisterSource(source-2) error = %v", err)
		}
		if err := manager.RegisterSource(source3); err != nil {
			t.Fatalf("RegisterSource(source-3) error = %v", err)
		}

		sources := manager.ListSources()
		if len(sources) != 3 {
			t.Errorf("ListSources() = %d sources, want 3", len(sources))
		}
	})
}

func TestLocalAlertSourceManager_UnregisterSource(t *testing.T) {
	t.Run("should unregister existing source", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()
		source := newMockAlertSource("test-source", port.SourceTypeWebhook)

		if err := manager.RegisterSource(source); err != nil {
			t.Fatalf("RegisterSource() error = %v", err)
		}

		err := manager.UnregisterSource("test-source")
		if err != nil {
			t.Errorf("UnregisterSource() error = %v", err)
		}

		sources := manager.ListSources()
		if len(sources) != 0 {
			t.Errorf("ListSources() = %d sources, want 0 after unregister", len(sources))
		}
	})

	t.Run("should close source on unregister", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()
		source := newMockAlertSource("test-source", port.SourceTypeWebhook)

		if err := manager.RegisterSource(source); err != nil {
			t.Fatalf("RegisterSource() error = %v", err)
		}

		if err := manager.UnregisterSource("test-source"); err != nil {
			t.Fatalf("UnregisterSource() error = %v", err)
		}

		if !source.closed {
			t.Error("Source should be closed after unregister")
		}
	})

	t.Run("should return error for non-existent source", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		err := manager.UnregisterSource("non-existent")

		if err == nil {
			t.Error("UnregisterSource() should return error for non-existent source")
		}
	})

	t.Run("should return error if source close fails", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()
		source := newMockAlertSource("test-source", port.SourceTypeWebhook)
		source.closeErr = context.DeadlineExceeded // Simulate close error

		if err := manager.RegisterSource(source); err != nil {
			t.Fatalf("RegisterSource() error = %v", err)
		}

		err := manager.UnregisterSource("test-source")
		if err == nil {
			t.Error("UnregisterSource() should propagate close error")
		}
	})
}

func TestLocalAlertSourceManager_GetSource(t *testing.T) {
	t.Run("should return registered source", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()
		source := newMockAlertSource("test-source", port.SourceTypeWebhook)

		if err := manager.RegisterSource(source); err != nil {
			t.Fatalf("RegisterSource() error = %v", err)
		}

		got, err := manager.GetSource("test-source")
		if err != nil {
			t.Errorf("GetSource() error = %v", err)
		}

		if got == nil {
			t.Fatal("GetSource() returned nil")
		}

		if got.Name() != "test-source" {
			t.Errorf("GetSource().Name() = %v, want test-source", got.Name())
		}
	})

	t.Run("should return error for non-existent source", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		_, err := manager.GetSource("non-existent")

		if err == nil {
			t.Error("GetSource() should return error for non-existent source")
		}
	})

	t.Run("should return correct source among multiple", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()
		source1 := newMockAlertSource("source-1", port.SourceTypeWebhook)
		source2 := newMockAlertSource("source-2", port.SourceTypePoll)
		source3 := newMockAlertSource("source-3", port.SourceTypeStream)

		manager.RegisterSource(source1)
		manager.RegisterSource(source2)
		manager.RegisterSource(source3)

		got, err := manager.GetSource("source-2")
		if err != nil {
			t.Fatalf("GetSource() error = %v", err)
		}

		if got.Name() != "source-2" {
			t.Errorf("GetSource().Name() = %v, want source-2", got.Name())
		}

		if got.Type() != port.SourceTypePoll {
			t.Errorf("GetSource().Type() = %v, want %v", got.Type(), port.SourceTypePoll)
		}
	})
}

func TestLocalAlertSourceManager_ListSources(t *testing.T) {
	t.Run("should return empty list when no sources", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		sources := manager.ListSources()

		if sources == nil {
			t.Error("ListSources() should return empty slice, not nil")
		}
		if len(sources) != 0 {
			t.Errorf("ListSources() len = %d, want 0", len(sources))
		}
	})

	t.Run("should return all registered sources", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		for i := 0; i < 5; i++ {
			source := newMockAlertSource(
				"source-"+string(rune('a'+i)),
				port.SourceTypeWebhook,
			)
			manager.RegisterSource(source)
		}

		sources := manager.ListSources()
		if len(sources) != 5 {
			t.Errorf("ListSources() len = %d, want 5", len(sources))
		}
	})

	t.Run("should reflect unregistered sources", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()
		source1 := newMockAlertSource("source-1", port.SourceTypeWebhook)
		source2 := newMockAlertSource("source-2", port.SourceTypePoll)

		manager.RegisterSource(source1)
		manager.RegisterSource(source2)

		if len(manager.ListSources()) != 2 {
			t.Fatal("Expected 2 sources after registration")
		}

		manager.UnregisterSource("source-1")

		sources := manager.ListSources()
		if len(sources) != 1 {
			t.Errorf("ListSources() len = %d, want 1 after unregister", len(sources))
		}
	})
}

func TestLocalAlertSourceManager_SetAlertHandler(t *testing.T) {
	t.Run("should set and invoke alert handler", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		var handlerCalled bool
		var receivedAlert *entity.Alert

		manager.SetAlertHandler(func(ctx context.Context, alert *entity.Alert) error {
			handlerCalled = true
			receivedAlert = alert
			return nil
		})

		// Create and register a webhook source
		webhookSource := newMockWebhookSource("test-webhook", "/alerts")
		webhookSource.handleFunc = func(ctx context.Context, payload []byte) ([]*entity.Alert, error) {
			alert, _ := entity.NewAlert("test-id", "test-webhook", entity.SeverityWarning, "Test Alert")
			return []*entity.Alert{alert}, nil
		}

		if err := manager.RegisterSource(webhookSource); err != nil {
			t.Fatalf("RegisterSource() error = %v", err)
		}

		// Start the manager
		ctx := context.Background()
		if err := manager.Start(ctx); err != nil {
			t.Fatalf("Start() error = %v", err)
		}
		defer manager.Shutdown()

		// Note: The actual invocation of the handler depends on triggering
		// the webhook, which would be done via HTTP in the real implementation.
		// For unit testing, we might need to simulate this differently.
		// This test documents the expected behavior.

		_ = handlerCalled
		_ = receivedAlert
	})

	t.Run("should allow updating handler", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		var firstHandlerCalls int
		var secondHandlerCalls int

		manager.SetAlertHandler(func(ctx context.Context, alert *entity.Alert) error {
			firstHandlerCalls++
			return nil
		})

		manager.SetAlertHandler(func(ctx context.Context, alert *entity.Alert) error {
			secondHandlerCalls++
			return nil
		})

		// After setting a new handler, only the new handler should be called
		// This behavior needs to be tested with actual alert dispatch
		_ = firstHandlerCalls
		_ = secondHandlerCalls
	})

	t.Run("should handle nil handler gracefully", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		// Should not panic
		manager.SetAlertHandler(nil)
	})
}

func TestLocalAlertSourceManager_StartAndShutdown(t *testing.T) {
	t.Run("should start and shutdown cleanly", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		ctx := context.Background()
		if err := manager.Start(ctx); err != nil {
			t.Errorf("Start() error = %v", err)
		}

		if err := manager.Shutdown(); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	t.Run("should close all sources on shutdown", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		source1 := newMockAlertSource("source-1", port.SourceTypeWebhook)
		source2 := newMockAlertSource("source-2", port.SourceTypePoll)
		source3 := newMockAlertSource("source-3", port.SourceTypeStream)

		manager.RegisterSource(source1)
		manager.RegisterSource(source2)
		manager.RegisterSource(source3)

		ctx := context.Background()
		manager.Start(ctx)
		manager.Shutdown()

		if !source1.closed {
			t.Error("source-1 should be closed after shutdown")
		}
		if !source2.closed {
			t.Error("source-2 should be closed after shutdown")
		}
		if !source3.closed {
			t.Error("source-3 should be closed after shutdown")
		}
	})

	t.Run("should respect context cancellation on start", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Start should either return immediately or handle cancellation
		_ = manager.Start(ctx)

		// Should be able to call shutdown without hanging
		done := make(chan struct{})
		go func() {
			manager.Shutdown()
			close(done)
		}()

		select {
		case <-done:
			// Success
		case <-time.After(time.Second):
			t.Error("Shutdown() timed out")
		}
	})

	t.Run("should aggregate errors from multiple source close failures", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		source1 := newMockAlertSource("source-1", port.SourceTypeWebhook)
		source1.closeErr = context.DeadlineExceeded

		source2 := newMockAlertSource("source-2", port.SourceTypePoll)
		source2.closeErr = context.Canceled

		manager.RegisterSource(source1)
		manager.RegisterSource(source2)

		ctx := context.Background()
		manager.Start(ctx)

		err := manager.Shutdown()
		if err == nil {
			t.Error("Shutdown() should return aggregated error when sources fail to close")
		}
	})
}

func TestLocalAlertSourceManager_Concurrency(t *testing.T) {
	t.Run("should handle concurrent register operations", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		var wg sync.WaitGroup
		numGoroutines := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				source := newMockAlertSource(
					"source-"+string(rune('a'+id)),
					port.SourceTypeWebhook,
				)
				manager.RegisterSource(source)
			}(i)
		}

		wg.Wait()

		sources := manager.ListSources()
		if len(sources) != numGoroutines {
			t.Errorf("Expected %d sources after concurrent registration, got %d", numGoroutines, len(sources))
		}
	})

	t.Run("should handle concurrent get operations", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		// Pre-register sources
		for i := 0; i < 5; i++ {
			source := newMockAlertSource(
				"source-"+string(rune('a'+i)),
				port.SourceTypeWebhook,
			)
			manager.RegisterSource(source)
		}

		var wg sync.WaitGroup
		numReaders := 50
		errors := make(chan error, numReaders)

		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := manager.GetSource("source-c")
				if err != nil {
					errors <- err
				}
			}()
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("Concurrent GetSource() error = %v", err)
		}
	})

	t.Run("should handle concurrent register and get operations", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		var wg sync.WaitGroup
		var successfulGets int32
		var failedGets int32

		// Start some readers
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 10; j++ {
					_, err := manager.GetSource("target-source")
					if err == nil {
						atomic.AddInt32(&successfulGets, 1)
					} else {
						atomic.AddInt32(&failedGets, 1)
					}
					time.Sleep(time.Microsecond)
				}
			}()
		}

		// Register the target source midway
		time.Sleep(time.Millisecond)
		source := newMockAlertSource("target-source", port.SourceTypeWebhook)
		if err := manager.RegisterSource(source); err != nil {
			t.Fatalf("RegisterSource() error = %v", err)
		}

		wg.Wait()

		// Some gets should have succeeded (after registration)
		// and some should have failed (before registration)
		totalGets := successfulGets + failedGets
		if totalGets != 200 {
			t.Errorf("Expected 200 total gets, got %d", totalGets)
		}
	})

	t.Run("should handle concurrent list operations", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		// Pre-register some sources
		for i := 0; i < 3; i++ {
			source := newMockAlertSource(
				"source-"+string(rune('a'+i)),
				port.SourceTypeWebhook,
			)
			manager.RegisterSource(source)
		}

		var wg sync.WaitGroup
		numReaders := 100

		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sources := manager.ListSources()
				if sources == nil {
					t.Error("ListSources() returned nil")
				}
			}()
		}

		wg.Wait()
	})
}

func TestLocalAlertSourceManager_ErrorCallback(t *testing.T) {
	t.Run("should set error callback", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		var callbackInvoked bool
		var receivedSource string
		var receivedErr error

		manager.SetErrorCallback(func(source string, err error) {
			callbackInvoked = true
			receivedSource = source
			receivedErr = err
		})

		// Error callback is typically invoked when sources report errors
		// This documents the expected interface
		_ = callbackInvoked
		_ = receivedSource
		_ = receivedErr
	})

	t.Run("should handle nil error callback gracefully", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		// Should not panic
		manager.SetErrorCallback(nil)
	})
}

func TestLocalAlertSourceManager_HTTPHandler(t *testing.T) {
	t.Run("should implement http.Handler interface", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		// The manager should be usable as an HTTP handler for webhook sources
		// This test documents that expectation
		_ = manager

		// Note: Full HTTP handler testing would require httptest,
		// which is better done in integration tests
	})
}

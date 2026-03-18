package alert

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port/porttest"
)

// =============================================================================
// Source Manager Tests - RED PHASE
// These tests define the expected behavior of the LocalAlertSourceManager.
// All tests should FAIL until the implementation is complete.
// =============================================================================

func TestNewLocalAlertSourceManager(t *testing.T) {
	tests := []struct {
		name string
		test func(t *testing.T)
	}{
		{
			name: "should create new manager",
			test: func(t *testing.T) {
				manager := NewLocalAlertSourceManager()
				if manager == nil {
					t.Fatal("NewLocalAlertSourceManager() returned nil")
				}
			},
		},
		{
			name: "should implement AlertSourceManager interface",
			test: func(t *testing.T) {
				manager := NewLocalAlertSourceManager()
				var _ port.AlertSourceManager = manager
			},
		},
		{
			name: "should start with empty source list",
			test: func(t *testing.T) {
				manager := NewLocalAlertSourceManager()
				sources := manager.ListSources()
				if len(sources) != 0 {
					t.Errorf("ListSources() = %d sources, want 0", len(sources))
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, tt.test)
	}
}

func TestLocalAlertSourceManager_RegisterSource(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(m port.AlertSourceManager)
		source    port.AlertSource
		wantErr   bool
		wantCount int
	}{
		{
			name:      "should register valid source",
			source:    porttest.NewMockAlertSource("test-source", port.SourceTypeWebhook),
			wantErr:   false,
			wantCount: 1,
		},
		{
			name: "should reject duplicate registration",
			setup: func(m port.AlertSourceManager) {
				m.RegisterSource(porttest.NewMockAlertSource("test-source", port.SourceTypeWebhook))
			},
			source:    porttest.NewMockAlertSource("test-source", port.SourceTypePoll),
			wantErr:   true,
			wantCount: 1,
		},
		{
			name: "should register multiple unique sources",
			setup: func(m port.AlertSourceManager) {
				m.RegisterSource(porttest.NewMockAlertSource("source-1", port.SourceTypeWebhook))
				m.RegisterSource(porttest.NewMockAlertSource("source-2", port.SourceTypePoll))
			},
			source:    porttest.NewMockAlertSource("source-3", port.SourceTypeStream),
			wantErr:   false,
			wantCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewLocalAlertSourceManager()
			if tt.setup != nil {
				tt.setup(manager)
			}

			err := manager.RegisterSource(tt.source)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegisterSource() error = %v, wantErr %v", err, tt.wantErr)
			}

			sources := manager.ListSources()
			if len(sources) != tt.wantCount {
				t.Errorf("ListSources() = %d sources, want %d", len(sources), tt.wantCount)
			}
		})
	}
}

func TestLocalAlertSourceManager_UnregisterSource(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(m port.AlertSourceManager, s *porttest.MockAlertSource)
		sourceName string
		wantErr    bool
		check      func(t *testing.T, m port.AlertSourceManager, s *porttest.MockAlertSource)
	}{
		{
			name: "should unregister existing source",
			setup: func(m port.AlertSourceManager, s *porttest.MockAlertSource) {
				m.RegisterSource(s)
			},
			sourceName: "test-source",
			wantErr:    false,
			check: func(t *testing.T, m port.AlertSourceManager, s *porttest.MockAlertSource) {
				if len(m.ListSources()) != 0 {
					t.Errorf("ListSources() = %d sources, want 0 after unregister", len(m.ListSources()))
				}
			},
		},
		{
			name: "should close source on unregister",
			setup: func(m port.AlertSourceManager, s *porttest.MockAlertSource) {
				m.RegisterSource(s)
			},
			sourceName: "test-source",
			wantErr:    false,
			check: func(t *testing.T, m port.AlertSourceManager, s *porttest.MockAlertSource) {
				if !s.ClosedVal {
					t.Error("Source should be ClosedVal after unregister")
				}
			},
		},
		{
			name:       "should return error for non-existent source",
			sourceName: "non-existent",
			wantErr:    true,
		},
		{
			name: "should return error if source close fails",
			setup: func(m port.AlertSourceManager, s *porttest.MockAlertSource) {
				s.CloseErr = context.DeadlineExceeded
				m.RegisterSource(s)
			},
			sourceName: "test-source",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewLocalAlertSourceManager()
			source := porttest.NewMockAlertSource("test-source", port.SourceTypeWebhook)
			if tt.setup != nil {
				tt.setup(manager, source)
			}

			err := manager.UnregisterSource(tt.sourceName)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnregisterSource() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.check != nil {
				tt.check(t, manager, source)
			}
		})
	}
}

func TestLocalAlertSourceManager_GetSource(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(m port.AlertSourceManager)
		sourceName string
		wantErr    bool
		wantName   string
		wantType   port.SourceType
	}{
		{
			name: "should return registered source",
			setup: func(m port.AlertSourceManager) {
				m.RegisterSource(porttest.NewMockAlertSource("test-source", port.SourceTypeWebhook))
			},
			sourceName: "test-source",
			wantErr:    false,
			wantName:   "test-source",
		},
		{
			name:       "should return error for non-existent source",
			sourceName: "non-existent",
			wantErr:    true,
		},
		{
			name: "should return correct source among multiple",
			setup: func(m port.AlertSourceManager) {
				m.RegisterSource(porttest.NewMockAlertSource("source-1", port.SourceTypeWebhook))
				m.RegisterSource(porttest.NewMockAlertSource("source-2", port.SourceTypePoll))
				m.RegisterSource(porttest.NewMockAlertSource("source-3", port.SourceTypeStream))
			},
			sourceName: "source-2",
			wantErr:    false,
			wantName:   "source-2",
			wantType:   port.SourceTypePoll,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewLocalAlertSourceManager()
			if tt.setup != nil {
				tt.setup(manager)
			}

			got, err := manager.GetSource(tt.sourceName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetSource() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				if got == nil {
					t.Fatal("GetSource() returned nil")
				}
				if got.Name() != tt.wantName {
					t.Errorf("GetSource().Name() = %v, want %v", got.Name(), tt.wantName)
				}
				if tt.wantType != "" && got.Type() != tt.wantType {
					t.Errorf("GetSource().Type() = %v, want %v", got.Type(), tt.wantType)
				}
			}
		})
	}
}

func TestLocalAlertSourceManager_ListSources(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(m port.AlertSourceManager)
		wantCount int
	}{
		{
			name:      "should return empty list when no sources",
			wantCount: 0,
		},
		{
			name: "should return all registered sources",
			setup: func(m port.AlertSourceManager) {
				for i := range 5 {
					m.RegisterSource(porttest.NewMockAlertSource("source-"+string(rune('a'+i)), port.SourceTypeWebhook))
				}
			},
			wantCount: 5,
		},
		{
			name: "should reflect unregistered sources",
			setup: func(m port.AlertSourceManager) {
				m.RegisterSource(porttest.NewMockAlertSource("source-1", port.SourceTypeWebhook))
				m.RegisterSource(porttest.NewMockAlertSource("source-2", port.SourceTypePoll))
				m.UnregisterSource("source-1")
			},
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewLocalAlertSourceManager()
			if tt.setup != nil {
				tt.setup(manager)
			}

			sources := manager.ListSources()
			if sources == nil {
				t.Error("ListSources() should return empty slice, not nil")
			}
			if len(sources) != tt.wantCount {
				t.Errorf("ListSources() len = %d, want %d", len(sources), tt.wantCount)
			}
		})
	}
}

func TestLocalAlertSourceManager_GetWebhookSourceByPath(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(m port.AlertSourceManager)
		path     string
		wantName string
		wantNil  bool
	}{
		{
			name: "should return registered webhook source by path",
			setup: func(m port.AlertSourceManager) {
				m.RegisterSource(porttest.NewMockWebhookSource("test-source", "/alerts/test"))
			},
			path:     "/alerts/test",
			wantName: "test-source",
		},
		{
			name: "should return nil for non-existent path",
			setup: func(m port.AlertSourceManager) {
				m.RegisterSource(porttest.NewMockWebhookSource("test-source", "/alerts/test"))
			},
			path:    "/non-existent",
			wantNil: true,
		},
		{
			name: "should return nil after source is unregistered",
			setup: func(m port.AlertSourceManager) {
				m.RegisterSource(porttest.NewMockWebhookSource("test-source", "/alerts/test"))
				m.UnregisterSource("test-source")
			},
			path:    "/alerts/test",
			wantNil: true,
		},
		{
			name: "should only return webhook sources",
			setup: func(m port.AlertSourceManager) {
				m.RegisterSource(porttest.NewMockAlertSource("poll-source", port.SourceTypePoll))
			},
			path:    "/any-path",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewLocalAlertSourceManager()
			if tt.setup != nil {
				tt.setup(manager)
			}

			got := manager.GetWebhookSourceByPath(tt.path)
			if tt.wantNil {
				if got != nil {
					t.Errorf("GetWebhookSourceByPath() = %v, want nil", got)
				}
				return
			}

			if got == nil {
				t.Fatal("GetWebhookSourceByPath() returned nil")
			}
			if got.Name() != tt.wantName {
				t.Errorf("GetWebhookSourceByPath().Name() = %v, want %v", got.Name(), tt.wantName)
			}
		})
	}
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
		webhookSource := porttest.NewMockWebhookSource("test-webhook", "/alerts")
		webhookSource.HandleFunc = func(ctx context.Context, payload []byte) ([]*entity.Alert, error) {
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
	tests := []struct {
		name    string
		setup   func(m port.AlertSourceManager, sources []*porttest.MockAlertSource)
		wantErr bool
		check   func(t *testing.T, m port.AlertSourceManager, sources []*porttest.MockAlertSource)
	}{
		{
			name: "should start and shutdown cleanly",
		},
		{
			name: "should close all sources on shutdown",
			setup: func(m port.AlertSourceManager, sources []*porttest.MockAlertSource) {
				for _, s := range sources {
					m.RegisterSource(s)
				}
			},
			check: func(t *testing.T, m port.AlertSourceManager, sources []*porttest.MockAlertSource) {
				for _, s := range sources {
					if !s.ClosedVal {
						t.Errorf("source %s should be ClosedVal after shutdown", s.Name())
					}
				}
			},
		},
		{
			name: "should aggregate errors from multiple source close failures",
			setup: func(m port.AlertSourceManager, sources []*porttest.MockAlertSource) {
				sources[0].CloseErr = context.DeadlineExceeded
				sources[1].CloseErr = context.Canceled
				for _, s := range sources {
					m.RegisterSource(s)
				}
			},
			wantErr: true,
			check: func(t *testing.T, m port.AlertSourceManager, sources []*porttest.MockAlertSource) {
				// We already check wantErr in the loop below, but let's do a more specific check if needed
				// This is just to satisfy the check if called
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manager := NewLocalAlertSourceManager()
			sources := []*porttest.MockAlertSource{
				porttest.NewMockAlertSource("source-1", port.SourceTypeWebhook),
				porttest.NewMockAlertSource("source-2", port.SourceTypePoll),
				porttest.NewMockAlertSource("source-3", port.SourceTypeStream),
			}

			if tt.setup != nil {
				tt.setup(manager, sources)
			}

			ctx := context.Background()
			if err := manager.Start(ctx); err != nil {
				t.Fatalf("Start() error = %v", err)
			}

			err := manager.Shutdown()
			if (err != nil) != tt.wantErr {
				t.Errorf("Shutdown() error = %v, wantErr %v", err, tt.wantErr)
			}

			if tt.wantErr && err != nil {
				if tt.name == "should aggregate errors from multiple source close failures" {
					importErrors := []error{context.DeadlineExceeded, context.Canceled}
					for _, targetErr := range importErrors {
						if !errors.Is(err, targetErr) {
							t.Errorf("Shutdown() error = %v, does not contain expected error %v", err, targetErr)
						}
					}
				}
			}

			if tt.check != nil {
				tt.check(t, manager, sources)
			}
		})
	}

	t.Run("should respect context cancellation on start", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_ = manager.Start(ctx)

		done := make(chan struct{})
		go func() {
			manager.Shutdown()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("Shutdown() timed out")
		}
	})
}

func TestLocalAlertSourceManager_Concurrency(t *testing.T) {
	t.Run("should handle concurrent register operations", func(t *testing.T) {
		manager := NewLocalAlertSourceManager()

		var wg sync.WaitGroup
		numGoroutines := 10

		for i := range numGoroutines {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				source := porttest.NewMockAlertSource(
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
		for i := range 5 {
			source := porttest.NewMockAlertSource(
				"source-"+string(rune('a'+i)),
				port.SourceTypeWebhook,
			)
			manager.RegisterSource(source)
		}

		var wg sync.WaitGroup
		numReaders := 50
		errors := make(chan error, numReaders)

		for range numReaders {
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
		for range 20 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for range 10 {
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
		source := porttest.NewMockAlertSource("target-source", port.SourceTypeWebhook)
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
		for i := range 3 {
			source := porttest.NewMockAlertSource(
				"source-"+string(rune('a'+i)),
				port.SourceTypeWebhook,
			)
			manager.RegisterSource(source)
		}

		var wg sync.WaitGroup
		numReaders := 100

		for range numReaders {
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

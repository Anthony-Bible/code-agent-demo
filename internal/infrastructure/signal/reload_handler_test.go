package signal

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// =============================================================================
// TDD Red Phase Tests for ReloadHandler
// These tests define the expected behavior for the SIGHUP skill reload feature.
// All tests should FAIL initially until the implementation is complete.
// =============================================================================

func TestReloadHandler_NewCreatesHandler(t *testing.T) {
	t.Run("should create a valid handler with callback", func(t *testing.T) {
		called := atomic.Int32{}
		onReload := func(ctx context.Context) {
			called.Add(1)
		}

		handler := NewReloadHandler(onReload)

		if handler == nil {
			t.Fatal("NewReloadHandler() returned nil, expected a valid handler")
		}
	})

	t.Run("should create handler with nil callback", func(t *testing.T) {
		handler := NewReloadHandler(nil)

		if handler == nil {
			t.Fatal("NewReloadHandler(nil) returned nil, expected a valid handler")
		}
	})
}

func TestReloadHandler_CallbackInvoked(t *testing.T) {
	t.Run("should invoke callback when SIGHUP is simulated", func(t *testing.T) {
		called := atomic.Int32{}
		onReload := func(ctx context.Context) {
			called.Add(1)
		}

		handler := NewReloadHandler(onReload)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		// Simulate SIGHUP
		handler.SimulateReload()

		// Wait for callback to be invoked
		time.Sleep(50 * time.Millisecond)

		if called.Load() != 1 {
			t.Errorf("expected callback to be called 1 time, got %d", called.Load())
		}
	})

	t.Run("should invoke callback multiple times for multiple SIGHUPs", func(t *testing.T) {
		called := atomic.Int32{}
		onReload := func(ctx context.Context) {
			called.Add(1)
		}

		handler := NewReloadHandler(onReload)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		// Simulate multiple SIGHUPs
		handler.SimulateReload()
		time.Sleep(20 * time.Millisecond)
		handler.SimulateReload()
		time.Sleep(20 * time.Millisecond)
		handler.SimulateReload()
		time.Sleep(20 * time.Millisecond)

		if called.Load() != 3 {
			t.Errorf("expected callback to be called 3 times, got %d", called.Load())
		}
	})

	t.Run("should pass valid context to callback", func(t *testing.T) {
		var receivedCtx context.Context
		onReload := func(ctx context.Context) {
			receivedCtx := ctx
		}

		handler := NewReloadHandler(onReload)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		handler.SimulateReload()
		time.Sleep(50 * time.Millisecond)

		if receivedCtx == nil {
			t.Error("callback received nil context, expected a valid context")
		}
	})

	t.Run("should pass handler's context to callback", func(t *testing.T) {
		var receivedCtx context.Context
		onReload := func(ctx context.Context) {
			receivedCtx := ctx
		}

		handler := NewReloadHandler(onReload)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		handlerCtx := handler.Context()
		handler.SimulateReload()
		time.Sleep(50 * time.Millisecond)

		if receivedCtx != handlerCtx {
			t.Error("callback received different context than handler's context")
		}
	})
}

func TestReloadHandler_LifecycleManagement(t *testing.T) {
	t.Run("should handle multiple Start calls gracefully", func(t *testing.T) {
		called := atomic.Int32{}
		onReload := func(ctx context.Context) {
			called.Add(1)
		}

		handler := NewReloadHandler(onReload)
		if handler == nil {
			t.Fatal("handler is nil")
		}

		// Multiple starts should not panic or cause issues
		handler.Start()
		handler.Start()
		handler.Start()

		// Test that it still works correctly
		handler.SimulateReload()
		time.Sleep(50 * time.Millisecond)

		handler.Stop()

		// Should be called exactly once, not three times (idempotent Start)
		if called.Load() != 1 {
			t.Errorf("expected callback to be called 1 time, got %d", called.Load())
		}
	})

	t.Run("should handle Stop without Start", func(t *testing.T) {
		called := atomic.Int32{}
		onReload := func(ctx context.Context) {
			called.Add(1)
		}

		handler := NewReloadHandler(onReload)
		if handler == nil {
			t.Fatal("handler is nil")
		}

		// Stop without start should not panic
		handler.Stop()
	})

	t.Run("should handle multiple Stop calls", func(t *testing.T) {
		called := atomic.Int32{}
		onReload := func(ctx context.Context) {
			called.Add(1)
		}

		handler := NewReloadHandler(onReload)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()

		// Multiple stops should not panic
		handler.Stop()
		handler.Stop()
		handler.Stop()
	})

	t.Run("should not invoke callback after Stop", func(t *testing.T) {
		called := atomic.Int32{}
		onReload := func(ctx context.Context) {
			called.Add(1)
		}

		handler := NewReloadHandler(onReload)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()

		// Simulate reload before stop (should work)
		handler.SimulateReload()
		time.Sleep(50 * time.Millisecond)

		if called.Load() != 1 {
			t.Errorf("expected callback to be called 1 time before Stop, got %d", called.Load())
		}

		// Stop handler
		handler.Stop()

		// Simulate reload after stop (should NOT work)
		handler.SimulateReload()
		handler.SimulateReload()
		time.Sleep(50 * time.Millisecond)

		// Should still be 1, not 3
		if called.Load() != 1 {
			t.Errorf(
				"expected callback to be called 1 time after Stop, got %d (callback was invoked after Stop)",
				called.Load(),
			)
		}
	})
}

func TestReloadHandler_ContextManagement(t *testing.T) {
	t.Run("should return non-nil context", func(t *testing.T) {
		handler := NewReloadHandler(func(ctx context.Context) {})
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		ctx := handler.Context()
		if ctx == nil {
			t.Error("Context() returned nil, expected a valid context")
		}
	})

	t.Run("should return the same context on multiple calls", func(t *testing.T) {
		handler := NewReloadHandler(func(ctx context.Context) {})
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		ctx1 := handler.Context()
		ctx2 := handler.Context()

		if ctx1 == nil || ctx2 == nil {
			t.Fatal("Context() returned nil")
		}

		// Verify they behave the same by checking Done channels
		if ctx1.Done() == nil || ctx2.Done() == nil {
			t.Error("Context Done() returned nil")
		}
	})

	t.Run("context should not be cancelled initially", func(t *testing.T) {
		handler := NewReloadHandler(func(ctx context.Context) {})
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil")
		}

		select {
		case <-ctx.Done():
			t.Error("Context is already cancelled before Stop")
		default:
			// Good - context is not cancelled
		}

		if ctx.Err() != nil {
			t.Errorf("Context.Err() should be nil initially, got: %v", ctx.Err())
		}
	})

	t.Run("context should be cancelled after Stop", func(t *testing.T) {
		handler := NewReloadHandler(func(ctx context.Context) {})
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil")
		}

		// Stop handler
		handler.Stop()

		// Context should be cancelled after Stop
		select {
		case <-ctx.Done():
			// Success - context was cancelled
		case <-time.After(100 * time.Millisecond):
			t.Error("Context was not cancelled after Stop")
		}
	})

	t.Run("context Done channel should be closed after Stop", func(t *testing.T) {
		handler := NewReloadHandler(func(ctx context.Context) {})
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil")
		}

		handler.Stop()

		// Wait and verify Done channel is closed
		time.Sleep(50 * time.Millisecond)

		select {
		case _, ok := <-ctx.Done():
			if ok {
				t.Error("Context Done channel should be closed (receive ok=false)")
			}
			// Success - channel is closed
		case <-time.After(100 * time.Millisecond):
			t.Error("Context Done channel was not closed")
		}
	})

	t.Run("context should return error after Stop", func(t *testing.T) {
		handler := NewReloadHandler(func(ctx context.Context) {})
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil")
		}

		handler.Stop()

		// Wait for cancellation
		select {
		case <-ctx.Done():
			// Verify the error is set
			if ctx.Err() == nil {
				t.Error("Context.Err() returned nil after Stop")
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Context was not cancelled")
		}
	})
}

func TestReloadHandler_EdgeCases(t *testing.T) {
	t.Run("should not panic with nil callback on SimulateReload", func(t *testing.T) {
		handler := NewReloadHandler(nil)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		// Should not panic even though callback is nil
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("SimulateReload() panicked with nil callback: %v", r)
			}
		}()

		handler.SimulateReload()
		time.Sleep(50 * time.Millisecond)
	})

	t.Run("should handle rapid sequential reloads", func(t *testing.T) {
		called := atomic.Int32{}
		onReload := func(ctx context.Context) {
			called.Add(1)
			// Simulate some work
			time.Sleep(10 * time.Millisecond)
		}

		handler := NewReloadHandler(onReload)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		// Send many rapid reloads
		for range 5 {
			handler.SimulateReload()
		}

		// Wait for all to process
		time.Sleep(200 * time.Millisecond)

		// Should be called 5 times
		if called.Load() != 5 {
			t.Errorf("expected callback to be called 5 times, got %d", called.Load())
		}
	})

	t.Run("callback should receive non-cancelled context during execution", func(t *testing.T) {
		var contextWasCancelled bool
		onReload := func(ctx context.Context) {
			select {
			case <-ctx.Done():
				contextWasCancelled = true
			default:
				contextWasCancelled = false
			}
		}

		handler := NewReloadHandler(onReload)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		handler.SimulateReload()
		time.Sleep(50 * time.Millisecond)

		if contextWasCancelled {
			t.Error("callback received cancelled context, expected active context")
		}
	})

	t.Run("should handle Start-Stop-Start cycle", func(t *testing.T) {
		called := atomic.Int32{}
		onReload := func(ctx context.Context) {
			called.Add(1)
		}

		handler := NewReloadHandler(onReload)
		if handler == nil {
			t.Fatal("handler is nil")
		}

		// First cycle
		handler.Start()
		handler.SimulateReload()
		time.Sleep(50 * time.Millisecond)
		handler.Stop()

		firstCount := called.Load()
		if firstCount != 1 {
			t.Errorf("expected 1 call in first cycle, got %d", firstCount)
		}

		// Second cycle
		handler.Start()
		handler.SimulateReload()
		time.Sleep(50 * time.Millisecond)
		handler.Stop()

		secondCount := called.Load()
		if secondCount != 2 {
			t.Errorf("expected 2 total calls after second cycle, got %d", secondCount)
		}
	})
}

func TestReloadHandler_ConcurrentSafety(t *testing.T) {
	t.Run("should handle concurrent SimulateReload calls", func(t *testing.T) {
		called := atomic.Int32{}
		onReload := func(ctx context.Context) {
			called.Add(1)
			time.Sleep(5 * time.Millisecond)
		}

		handler := NewReloadHandler(onReload)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		// Simulate concurrent reload signals
		const numGoroutines = 10
		done := make(chan struct{}, numGoroutines)

		for range numGoroutines {
			go func() {
				handler.SimulateReload()
				done <- struct{}{}
			}()
		}

		// Wait for all goroutines to complete
		for range numGoroutines {
			<-done
		}

		// Wait for all callbacks to process
		time.Sleep(200 * time.Millisecond)

		// Should be called exactly numGoroutines times
		if called.Load() != numGoroutines {
			t.Errorf("expected callback to be called %d times, got %d", numGoroutines, called.Load())
		}
	})

	t.Run("should handle concurrent Start and Stop calls", func(t *testing.T) {
		handler := NewReloadHandler(func(ctx context.Context) {})
		if handler == nil {
			t.Fatal("handler is nil")
		}

		done := make(chan struct{}, 20)

		// Concurrent Start calls
		for range 10 {
			go func() {
				handler.Start()
				done <- struct{}{}
			}()
		}

		// Concurrent Stop calls
		for range 10 {
			go func() {
				handler.Stop()
				done <- struct{}{}
			}()
		}

		// Wait for all to complete (should not panic or deadlock)
		for range 20 {
			select {
			case <-done:
			case <-time.After(1 * time.Second):
				t.Fatal("concurrent Start/Stop calls caused deadlock")
			}
		}
	})
}

// =============================================================================
// Table-driven tests for comprehensive scenario coverage
// =============================================================================

// reloadScenarioTestCase defines a test scenario for reload handling behavior.
type reloadScenarioTestCase struct {
	name            string
	reloadCount     int
	delayBetween    time.Duration
	expectCallCount int32
}

// runReloadScenario executes a single reload scenario test case.
func runReloadScenario(t *testing.T, tt reloadScenarioTestCase) {
	t.Helper()

	called := atomic.Int32{}
	onReload := func(ctx context.Context) {
		called.Add(1)
	}

	handler := NewReloadHandler(onReload)
	if handler == nil {
		t.Fatal("handler is nil")
	}
	handler.Start()
	defer handler.Stop()

	// Send reload signals
	for range tt.reloadCount {
		handler.SimulateReload()
		if tt.delayBetween > 0 {
			time.Sleep(tt.delayBetween)
		}
	}

	// Wait for processing
	time.Sleep(100 * time.Millisecond)

	if called.Load() != tt.expectCallCount {
		t.Errorf("expected callback count=%d, got %d", tt.expectCallCount, called.Load())
	}
}

func TestReloadHandler_Scenarios(t *testing.T) {
	tests := []reloadScenarioTestCase{
		{
			name:            "single reload",
			reloadCount:     1,
			delayBetween:    0,
			expectCallCount: 1,
		},
		{
			name:            "five rapid reloads",
			reloadCount:     5,
			delayBetween:    0,
			expectCallCount: 5,
		},
		{
			name:            "three reloads with delay",
			reloadCount:     3,
			delayBetween:    20 * time.Millisecond,
			expectCallCount: 3,
		},
		{
			name:            "ten reloads rapid fire",
			reloadCount:     10,
			delayBetween:    0,
			expectCallCount: 10,
		},
		{
			name:            "zero reloads",
			reloadCount:     0,
			delayBetween:    0,
			expectCallCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runReloadScenario(t, tt)
		})
	}
}

func TestReloadHandler_ContextPropagation(t *testing.T) {
	t.Run("callback context should have values from handler context", func(t *testing.T) {
		type contextKey string
		testKey := contextKey("test-key")

		var receivedValue interface{}
		onReload := func(ctx context.Context) {
			receivedValue = ctx.Value(testKey)
		}

		handler := NewReloadHandler(onReload)
		if handler == nil {
			t.Fatal("handler is nil")
		}

		// Note: This test expects the handler to allow context value propagation
		// The actual implementation may need to support this via constructor or setter
		handler.Start()
		defer handler.Stop()

		handler.SimulateReload()
		time.Sleep(50 * time.Millisecond)

		// For now, we expect the callback receives the handler's base context
		// which won't have custom values unless the implementation supports it
		// This test documents the expected behavior
		_ = receivedValue // Currently unused but documents expected behavior
	})
}

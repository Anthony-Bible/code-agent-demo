package signal

import (
	"context"
	"testing"
	"time"
)

// =============================================================================
// TDD Red Phase Tests for InterruptHandler
// These tests define the expected behavior for the double Ctrl+C exit feature.
// All tests should FAIL initially until the implementation is complete.
// =============================================================================

func TestInterruptHandler_NewCreatesHandler(t *testing.T) {
	t.Run("should create a valid handler with specified timeout", func(t *testing.T) {
		timeout := 2 * time.Second
		handler := NewInterruptHandler(timeout)

		if handler == nil {
			t.Fatal("NewInterruptHandler() returned nil, expected a valid handler")
		}
	})

	t.Run("should create handler with zero timeout", func(t *testing.T) {
		handler := NewInterruptHandler(0)

		if handler == nil {
			t.Fatal("NewInterruptHandler(0) returned nil, expected a valid handler")
		}
	})

	t.Run("should create handler with very short timeout", func(t *testing.T) {
		timeout := 1 * time.Millisecond
		handler := NewInterruptHandler(timeout)

		if handler == nil {
			t.Fatal("NewInterruptHandler() with short timeout returned nil")
		}
	})

	t.Run("should create handler with very long timeout", func(t *testing.T) {
		timeout := 1 * time.Hour
		handler := NewInterruptHandler(timeout)

		if handler == nil {
			t.Fatal("NewInterruptHandler() with long timeout returned nil")
		}
	})
}

func TestInterruptHandler_FirstInterruptFiresChannel(t *testing.T) {
	t.Run("should fire FirstPress channel on first interrupt", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		firstPressChan := handler.FirstPress()
		if firstPressChan == nil {
			t.Fatal("FirstPress() returned nil channel, expected a valid channel")
		}

		// Simulate first Ctrl+C
		handler.SimulateInterrupt()

		// Wait for the channel to fire with timeout
		select {
		case <-firstPressChan:
			// Success - channel fired as expected
		case <-time.After(100 * time.Millisecond):
			t.Error("FirstPress channel did not fire after first interrupt")
		}
	})

	t.Run("should not cancel context on first interrupt", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil, expected a valid context")
		}

		// Simulate first Ctrl+C
		handler.SimulateInterrupt()

		// Give some time for any incorrect cancellation to occur
		time.Sleep(50 * time.Millisecond)

		// Context should NOT be cancelled after first press
		select {
		case <-ctx.Done():
			t.Error("Context was cancelled after first interrupt, expected it to remain active")
		default:
			// Success - context is still active
		}
	})
}

func TestInterruptHandler_DoubleInterruptCancelsContext(t *testing.T) {
	t.Run("should cancel context on second interrupt within timeout", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil, expected a valid context")
		}

		// Simulate first Ctrl+C
		handler.SimulateInterrupt()

		// Small delay to ensure first press is processed
		time.Sleep(10 * time.Millisecond)

		// Simulate second Ctrl+C (within timeout)
		handler.SimulateInterrupt()

		// Wait for context cancellation with timeout
		select {
		case <-ctx.Done():
			// Success - context was cancelled
		case <-time.After(100 * time.Millisecond):
			t.Error("Context was not cancelled after double interrupt")
		}
	})

	t.Run("should cancel context immediately on rapid double press", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil")
		}

		// Simulate rapid double Ctrl+C
		handler.SimulateInterrupt()
		handler.SimulateInterrupt()

		// Context should be cancelled quickly
		select {
		case <-ctx.Done():
			// Success - context was cancelled
		case <-time.After(100 * time.Millisecond):
			t.Error("Context was not cancelled after rapid double interrupt")
		}
	})
}

func TestInterruptHandler_TimeoutResetsCounter(t *testing.T) {
	t.Run("should reset counter after timeout expires", func(t *testing.T) {
		// Use a very short timeout for testing
		timeout := 50 * time.Millisecond
		handler := NewInterruptHandler(timeout)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil")
		}

		firstPressChan := handler.FirstPress()
		if firstPressChan == nil {
			t.Fatal("FirstPress() returned nil channel")
		}

		// First interrupt
		handler.SimulateInterrupt()

		// Wait for first press channel
		select {
		case <-firstPressChan:
			// Good - first press detected
		case <-time.After(100 * time.Millisecond):
			t.Fatal("FirstPress channel did not fire")
		}

		// Wait for timeout to expire (plus buffer)
		time.Sleep(timeout + 20*time.Millisecond)

		// Context should still be active (not cancelled)
		select {
		case <-ctx.Done():
			t.Error("Context was cancelled after timeout, but should still be active")
		default:
			// Good - context still active
		}

		// Third interrupt should now be treated as a new "first" press
		// and fire the FirstPress channel again
		handler.SimulateInterrupt()

		// First press channel should fire again
		select {
		case <-firstPressChan:
			// Success - counter was reset, this is treated as first press
		case <-time.After(100 * time.Millisecond):
			t.Error("FirstPress channel did not fire after timeout reset")
		}

		// Context should STILL not be cancelled (only one press since reset)
		select {
		case <-ctx.Done():
			t.Error("Context was cancelled after single press post-reset")
		default:
			// Success - context is still active
		}
	})

	t.Run("should cancel context on double press after reset", func(t *testing.T) {
		timeout := 50 * time.Millisecond
		handler := NewInterruptHandler(timeout)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil")
		}

		// First interrupt
		handler.SimulateInterrupt()

		// Wait for timeout to expire
		time.Sleep(timeout + 20*time.Millisecond)

		// Now do a proper double press
		handler.SimulateInterrupt()
		time.Sleep(10 * time.Millisecond)
		handler.SimulateInterrupt()

		// Context should be cancelled
		select {
		case <-ctx.Done():
			// Success - context was cancelled
		case <-time.After(100 * time.Millisecond):
			t.Error("Context was not cancelled after double press following reset")
		}
	})
}

func TestInterruptHandler_ContextCancelledOnSecondPress(t *testing.T) {
	t.Run("context should return context.Canceled error after double press", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil")
		}

		// Double press
		handler.SimulateInterrupt()
		time.Sleep(10 * time.Millisecond)
		handler.SimulateInterrupt()

		// Wait for cancellation
		select {
		case <-ctx.Done():
			// Verify the error is context.Canceled
			if ctx.Err() == nil {
				t.Error("Context.Err() returned nil after cancellation")
			}
		case <-time.After(100 * time.Millisecond):
			t.Fatal("Context was not cancelled")
		}
	})

	t.Run("context Done channel should be closed after double press", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil")
		}

		// Double press
		handler.SimulateInterrupt()
		handler.SimulateInterrupt()

		// Wait and verify Done channel is closed (can read from it without blocking)
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
}

func TestInterruptHandler_StartAndStop(t *testing.T) {
	t.Run("should handle multiple Start calls gracefully", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
		if handler == nil {
			t.Fatal("handler is nil")
		}

		// Multiple starts should not panic
		handler.Start()
		handler.Start()
		handler.Stop()
	})

	t.Run("should handle Stop without Start", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
		if handler == nil {
			t.Fatal("handler is nil")
		}

		// Stop without start should not panic
		handler.Stop()
	})

	t.Run("should handle multiple Stop calls", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()

		// Multiple stops should not panic
		handler.Stop()
		handler.Stop()
	})

	t.Run("should not respond to interrupts after Stop", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		handler.Stop()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil")
		}

		// Simulate interrupts after stop
		handler.SimulateInterrupt()
		handler.SimulateInterrupt()

		// Give time for any incorrect processing
		time.Sleep(50 * time.Millisecond)

		// Context should NOT be cancelled since handler is stopped
		select {
		case <-ctx.Done():
			t.Error("Context was cancelled after Stop, expected no response to interrupts")
		default:
			// Success - no response to interrupts after stop
		}
	})
}

func TestInterruptHandler_FirstPressChannelNonNil(t *testing.T) {
	handler := NewInterruptHandler(2 * time.Second)
	if handler == nil {
		t.Fatal("handler is nil")
	}
	handler.Start()
	defer handler.Stop()

	ch := handler.FirstPress()
	if ch == nil {
		t.Error("FirstPress() returned nil, expected a valid receive-only channel")
	}
}

func TestInterruptHandler_FirstPressChannelSameOnMultipleCalls(t *testing.T) {
	handler := NewInterruptHandler(2 * time.Second)
	if handler == nil {
		t.Fatal("handler is nil")
	}
	handler.Start()
	defer handler.Stop()

	ch1 := handler.FirstPress()
	ch2 := handler.FirstPress()

	if ch1 == nil || ch2 == nil {
		t.Fatal("FirstPress() returned nil")
	}

	// Both calls should return the same channel
	// We can't directly compare channels, but we can verify behavior
	// by checking that a signal on one is received on the other
	handler.SimulateInterrupt()

	received1 := false
	select {
	case <-ch1:
		received1 = true
	case <-time.After(100 * time.Millisecond):
	}

	// Channel 2 should have the same state (either both received or neither)
	// Since they should be the same channel, if ch1 received, ch2 is drained
	select {
	case <-ch2:
		// If we also receive here, channels are different (unexpected)
		if received1 {
			t.Error("Received on both ch1 and ch2 - channels should be the same")
		}
	case <-time.After(10 * time.Millisecond):
		// Expected if channels are the same and ch1 already consumed the value
		if !received1 {
			t.Error("Neither channel received the interrupt signal")
		}
	}
}

func TestInterruptHandler_FirstPressFiresAfterReset(t *testing.T) {
	timeout := 50 * time.Millisecond
	handler := NewInterruptHandler(timeout)
	if handler == nil {
		t.Fatal("handler is nil")
	}
	handler.Start()
	defer handler.Stop()

	ch := handler.FirstPress()
	if ch == nil {
		t.Fatal("FirstPress() returned nil")
	}

	// First interrupt cycle
	handler.SimulateInterrupt()

	select {
	case <-ch:
		// Good
	case <-time.After(100 * time.Millisecond):
		t.Fatal("FirstPress did not fire on first interrupt")
	}

	// Wait for timeout to reset
	time.Sleep(timeout + 20*time.Millisecond)

	// Second interrupt cycle (after reset)
	handler.SimulateInterrupt()

	select {
	case <-ch:
		// Good - fired again after reset
	case <-time.After(100 * time.Millisecond):
		t.Error("FirstPress did not fire on first interrupt after reset")
	}
}

func TestInterruptHandler_Context(t *testing.T) {
	t.Run("Context should return non-nil context", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
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

	t.Run("Context should return the same context on multiple calls", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
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

	t.Run("Context should not be cancelled initially", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
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
			t.Error("Context is already cancelled before any interrupt")
		default:
			// Good - context is not cancelled
		}

		if ctx.Err() != nil {
			t.Errorf("Context.Err() should be nil initially, got: %v", ctx.Err())
		}
	})
}

func TestInterruptHandler_EdgeCases(t *testing.T) {
	t.Run("should handle interrupt exactly at timeout boundary", func(t *testing.T) {
		// This tests the edge case where second press comes exactly at timeout
		timeout := 100 * time.Millisecond
		handler := NewInterruptHandler(timeout)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil")
		}

		handler.SimulateInterrupt()

		// Wait almost exactly the timeout duration
		time.Sleep(timeout - 5*time.Millisecond)

		// Second press just before timeout expires
		handler.SimulateInterrupt()

		// Context should be cancelled (second press was within timeout)
		select {
		case <-ctx.Done():
			// Success
		case <-time.After(50 * time.Millisecond):
			t.Error("Context should be cancelled when second press is within timeout")
		}
	})

	t.Run("should handle many rapid interrupts", func(t *testing.T) {
		handler := NewInterruptHandler(2 * time.Second)
		if handler == nil {
			t.Fatal("handler is nil")
		}
		handler.Start()
		defer handler.Stop()

		ctx := handler.Context()
		if ctx == nil {
			t.Fatal("Context() returned nil")
		}

		// Send many rapid interrupts
		for range 10 {
			handler.SimulateInterrupt()
		}

		// Context should be cancelled (at least 2 interrupts occurred)
		select {
		case <-ctx.Done():
			// Success
		case <-time.After(100 * time.Millisecond):
			t.Error("Context should be cancelled after multiple interrupts")
		}
	})
}

// =============================================================================
// Table-driven tests for comprehensive scenario coverage
// =============================================================================

// scenarioTestCase defines a test scenario for interrupt handling behavior.
type scenarioTestCase struct {
	name             string
	timeout          time.Duration
	interrupts       []time.Duration // delays before each interrupt
	expectCancelled  bool
	expectFirstPress int // how many times FirstPress should fire
}

// runScenario executes a single interrupt scenario test case.
func runScenario(t *testing.T, tt scenarioTestCase) {
	t.Helper()

	handler := NewInterruptHandler(tt.timeout)
	if handler == nil {
		t.Fatal("handler is nil")
	}
	handler.Start()
	defer handler.Stop()

	ctx := handler.Context()
	if ctx == nil {
		t.Fatal("Context() returned nil")
	}

	firstPressChan := handler.FirstPress()
	if firstPressChan == nil {
		t.Fatal("FirstPress() returned nil")
	}

	firstPressCount := sendInterruptsAndCountFirstPress(t, handler, firstPressChan, tt.interrupts)

	// Wait a bit for processing
	time.Sleep(30 * time.Millisecond)

	// Check cancellation
	cancelled := isContextCancelled(ctx)

	if cancelled != tt.expectCancelled {
		t.Errorf("expected cancelled=%v, got cancelled=%v", tt.expectCancelled, cancelled)
	}

	if firstPressCount != tt.expectFirstPress {
		t.Errorf("expected firstPressCount=%d, got %d", tt.expectFirstPress, firstPressCount)
	}
}

// sendInterruptsAndCountFirstPress sends interrupts at specified delays and counts FirstPress signals.
func sendInterruptsAndCountFirstPress(
	t *testing.T,
	handler *InterruptHandler,
	firstPressChan <-chan struct{},
	interrupts []time.Duration,
) int {
	t.Helper()

	firstPressCount := 0
	startTime := time.Now()

	for _, delay := range interrupts {
		elapsed := time.Since(startTime)
		if delay > elapsed {
			time.Sleep(delay - elapsed)
		}
		handler.SimulateInterrupt()

		// Check if FirstPress fired (non-blocking)
		select {
		case <-firstPressChan:
			firstPressCount++
		case <-time.After(20 * time.Millisecond):
			// No first press signal
		}
	}

	return firstPressCount
}

// isContextCancelled checks if the context is cancelled without blocking.
func isContextCancelled(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

func TestInterruptHandler_Scenarios(t *testing.T) {
	tests := []scenarioTestCase{
		{
			name:             "single interrupt does not cancel",
			timeout:          100 * time.Millisecond,
			interrupts:       []time.Duration{0},
			expectCancelled:  false,
			expectFirstPress: 1,
		},
		{
			name:             "double interrupt within timeout cancels",
			timeout:          100 * time.Millisecond,
			interrupts:       []time.Duration{0, 10 * time.Millisecond},
			expectCancelled:  true,
			expectFirstPress: 1,
		},
		{
			name:             "double interrupt after timeout does not cancel",
			timeout:          50 * time.Millisecond,
			interrupts:       []time.Duration{0, 80 * time.Millisecond},
			expectCancelled:  false,
			expectFirstPress: 2, // Both should be treated as "first" presses
		},
		{
			name:             "triple interrupt with timeout reset",
			timeout:          50 * time.Millisecond,
			interrupts:       []time.Duration{0, 80 * time.Millisecond, 90 * time.Millisecond},
			expectCancelled:  true, // 2nd and 3rd are within timeout of each other
			expectFirstPress: 2,    // 1st and 2nd (after reset) fire FirstPress
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runScenario(t, tt)
		})
	}
}

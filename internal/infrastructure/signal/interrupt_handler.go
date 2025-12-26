// Package signal provides signal handling utilities for the CLI application.
package signal

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// InterruptHandler manages Ctrl+C (SIGINT) signals with a double-press exit pattern.
// On first press, it fires the FirstPress channel without cancelling the context.
// On second press within the timeout, it cancels the context (triggering exit).
// If the timeout expires without a second press, the counter resets.
type InterruptHandler struct {
	timeout       time.Duration
	ctx           context.Context
	cancel        context.CancelFunc
	firstPressCh  chan struct{}
	lastPressTime time.Time
	pressCount    int
	running       bool
	mu            sync.Mutex
	resetTimer    *time.Timer
	sigCh         chan os.Signal
	stopCh        chan struct{}
}

// NewInterruptHandler creates a new InterruptHandler with the specified timeout.
// The timeout determines how long to wait for a second Ctrl+C before resetting.
func NewInterruptHandler(timeout time.Duration) *InterruptHandler {
	ctx, cancel := context.WithCancel(context.Background())
	return &InterruptHandler{
		timeout:      timeout,
		ctx:          ctx,
		cancel:       cancel,
		firstPressCh: make(chan struct{}, 1),
	}
}

// Start begins listening for SIGINT signals.
// This method should be called once after creating the handler.
func (h *InterruptHandler) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return
	}

	h.running = true
	h.sigCh = make(chan os.Signal, 1)
	h.stopCh = make(chan struct{})

	signal.Notify(h.sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		for {
			select {
			case <-h.stopCh:
				return
			case <-h.sigCh:
				h.handleInterrupt()
			}
		}
	}()
}

// handleInterrupt processes a received interrupt signal.
// It implements the double-press detection logic with timeout-based reset.
func (h *InterruptHandler) handleInterrupt() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	now := time.Now()
	isSecondPressWithinTimeout := h.pressCount > 0 && now.Sub(h.lastPressTime) < h.timeout

	if isSecondPressWithinTimeout {
		h.handleSecondPress()
	} else {
		h.handleFirstPress(now)
	}
}

// handleSecondPress processes the second Ctrl+C within the timeout window.
// It cancels the context to trigger application exit.
// Caller must hold h.mu.
func (h *InterruptHandler) handleSecondPress() {
	h.cancel()
	h.pressCount = 0
	h.stopResetTimer()
}

// handleFirstPress processes the first Ctrl+C (or first after timeout reset).
// It fires the FirstPress channel and starts the reset timer.
// Caller must hold h.mu.
func (h *InterruptHandler) handleFirstPress(pressTime time.Time) {
	h.pressCount = 1
	h.lastPressTime = pressTime

	// Fire the first press channel (non-blocking send)
	select {
	case h.firstPressCh <- struct{}{}:
	default:
		// Channel buffer full; previous signal not consumed yet
	}

	// Stop any existing timer before starting a new one
	h.stopResetTimer()

	// Start reset timer to clear press count after timeout
	h.resetTimer = time.AfterFunc(h.timeout, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		h.pressCount = 0
	})
}

// stopResetTimer stops and clears the reset timer if it exists.
// Caller must hold h.mu.
func (h *InterruptHandler) stopResetTimer() {
	if h.resetTimer != nil {
		h.resetTimer.Stop()
		h.resetTimer = nil
	}
}

// Stop stops listening for signals and cleans up resources.
// This method should be called when the handler is no longer needed.
// It is safe to call Stop multiple times.
func (h *InterruptHandler) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	h.running = false
	h.stopSignalChannel()
	h.stopGoroutine()
	h.stopResetTimer()
}

// stopSignalChannel unregisters and closes the OS signal channel.
// Caller must hold h.mu.
func (h *InterruptHandler) stopSignalChannel() {
	if h.sigCh != nil {
		signal.Stop(h.sigCh)
		close(h.sigCh)
		h.sigCh = nil
	}
}

// stopGoroutine signals the listener goroutine to exit.
// Caller must hold h.mu.
func (h *InterruptHandler) stopGoroutine() {
	if h.stopCh != nil {
		close(h.stopCh)
		h.stopCh = nil
	}
}

// Context returns a context that will be cancelled when the user confirms exit
// by pressing Ctrl+C twice within the timeout period.
func (h *InterruptHandler) Context() context.Context {
	return h.ctx
}

// FirstPress returns a channel that receives a signal when the user presses
// Ctrl+C for the first time. This can be used to display a message like
// "Press Ctrl+C again to exit".
func (h *InterruptHandler) FirstPress() <-chan struct{} {
	return h.firstPressCh
}

// SimulateInterrupt simulates receiving a SIGINT signal.
// This method is intended for testing purposes only.
func (h *InterruptHandler) SimulateInterrupt() {
	h.handleInterrupt()
}

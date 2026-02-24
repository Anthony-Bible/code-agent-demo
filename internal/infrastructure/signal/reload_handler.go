// Package signal provides signal handling utilities for the CLI application.
package signal

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

// ReloadHandler manages SIGHUP signals for reloading configuration or skills.
// It invokes a callback function when SIGHUP is received.
type ReloadHandler struct {
	ctx      context.Context
	cancel   context.CancelFunc
	onReload func(ctx context.Context)
	running  bool
	mu       sync.Mutex
	sigCh    chan os.Signal
	stopCh   chan struct{}
}

// NewReloadHandler creates a new ReloadHandler with the specified callback.
// The callback will be invoked whenever SIGHUP is received.
// A nil callback is allowed and will simply be a no-op when signals are received.
func NewReloadHandler(onReload func(ctx context.Context)) *ReloadHandler {
	ctx, cancel := context.WithCancel(context.Background())
	return &ReloadHandler{
		ctx:      ctx,
		cancel:   cancel,
		onReload: onReload,
	}
}

// Start begins listening for SIGHUP signals.
// This method should be called once after creating the handler.
// Multiple calls to Start are safe and idempotent.
func (h *ReloadHandler) Start() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.running {
		return
	}

	h.running = true

	// Create new context if previous one was cancelled
	if h.ctx.Err() != nil {
		h.ctx, h.cancel = context.WithCancel(context.Background())
	}

	h.sigCh = make(chan os.Signal, 1)
	h.stopCh = make(chan struct{})

	signal.Notify(h.sigCh, syscall.SIGHUP)

	// Capture channel references to avoid race with Stop() setting them to nil
	sigCh := h.sigCh
	stopCh := h.stopCh

	go func() {
		for {
			select {
			case <-stopCh:
				return
			case <-sigCh:
				h.handleReload()
			}
		}
	}()
}

// handleReload processes a received SIGHUP signal.
// It invokes the callback if one is set and the handler is running.
func (h *ReloadHandler) handleReload() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	if h.onReload != nil {
		h.onReload(h.ctx)
	}
}

// Stop stops listening for signals and cleans up resources.
// This method should be called when the handler is no longer needed.
// It is safe to call Stop multiple times.
func (h *ReloadHandler) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	h.running = false
	h.stopGoroutine()
	h.stopSignalChannel()
	h.cancel()
}

// stopSignalChannel unregisters and closes the OS signal channel.
// Caller must hold h.mu.
func (h *ReloadHandler) stopSignalChannel() {
	if h.sigCh != nil {
		signal.Stop(h.sigCh)
		close(h.sigCh)
		h.sigCh = nil
	}
}

// stopGoroutine signals the listener goroutine to exit.
// Caller must hold h.mu.
func (h *ReloadHandler) stopGoroutine() {
	if h.stopCh != nil {
		close(h.stopCh)
		h.stopCh = nil
	}
}

// Context returns a context that will be cancelled when Stop is called.
// This allows long-running reload operations to be gracefully terminated.
func (h *ReloadHandler) Context() context.Context {
	return h.ctx
}

// SimulateReload simulates receiving a SIGHUP signal.
// This method is intended for testing purposes only.
func (h *ReloadHandler) SimulateReload() {
	h.handleReload()
}

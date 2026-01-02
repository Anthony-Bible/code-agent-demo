// Package webhook provides HTTP adapters for receiving webhook alerts.
// It implements an HTTP server that routes incoming webhooks to registered
// alert sources for processing.
package webhook

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// maxBodySize is the maximum allowed size for webhook request bodies (10MB).
const maxBodySize = 10 << 20

// HTTPAdapterConfig configures the webhook HTTP server.
type HTTPAdapterConfig struct {
	// Addr is the address to listen on (e.g., ":8080", "0.0.0.0:9090").
	Addr string
	// ReadTimeout is the maximum duration for reading the entire request.
	ReadTimeout time.Duration
	// WriteTimeout is the maximum duration for writing the response.
	WriteTimeout time.Duration
	// ShutdownTimeout is the grace period for graceful shutdown.
	ShutdownTimeout time.Duration
}

// DefaultConfig returns a configuration with sensible defaults.
func DefaultConfig() HTTPAdapterConfig {
	return HTTPAdapterConfig{
		Addr:            ":8080",
		ReadTimeout:     30 * time.Second,
		WriteTimeout:    30 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}
}

// HTTPAdapter provides HTTP endpoints for receiving webhook alerts.
// It implements graceful shutdown and integrates with AlertSourceManager.
type HTTPAdapter struct {
	sourceManager     port.AlertSourceManager
	alertHandler      port.AlertHandler
	asyncAlertHandler port.AsyncAlertHandler
	alertRunner       port.AlertRunner
	config            HTTPAdapterConfig
	server            *http.Server
	mux               *http.ServeMux
	mu                sync.RWMutex
	wg                sync.WaitGroup // tracks in-flight async investigations
	started           bool
}

// NewHTTPAdapter creates a new webhook HTTP adapter.
func NewHTTPAdapter(
	sourceManager port.AlertSourceManager,
	config HTTPAdapterConfig,
) *HTTPAdapter {
	adapter := &HTTPAdapter{
		sourceManager: sourceManager,
		config:        config,
		mux:           http.NewServeMux(),
	}
	adapter.registerRoutes()
	return adapter
}

// registerRoutes sets up the HTTP routes using Go 1.22+ syntax.
func (a *HTTPAdapter) registerRoutes() {
	// Health endpoints
	a.mux.HandleFunc("GET /health", a.handleHealth)
	a.mux.HandleFunc("GET /ready", a.handleReady)

	// Dynamic webhook routes based on registered sources
	// Using a catch-all pattern that routes to the appropriate source
	a.mux.HandleFunc("POST /alerts/{source...}", a.handleWebhook)
}

// handleHealth returns 200 OK if the server is running.
func (a *HTTPAdapter) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// handleReady returns 200 OK if at least one alert source is registered.
func (a *HTTPAdapter) handleReady(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	sources := a.sourceManager.ListSources()
	if len(sources) == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"no sources registered"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status":"ok","sources":%d}`, len(sources))
}

// handleWebhook routes incoming webhooks to the appropriate source.
func (a *HTTPAdapter) handleWebhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Reconstruct the full path from the wildcard
	sourcePath := r.PathValue("source")
	path := "/alerts/" + sourcePath

	// Find the matching webhook source
	source := a.findWebhookSource(path)
	if source == nil {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"unknown webhook path"}`))
		return
	}

	// Read request body with size limit
	r.Body = http.MaxBytesReader(w, r.Body, maxBodySize)

	payload, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"failed to read request body"}`))
		return
	}

	// Process the webhook
	ctx := r.Context()
	alerts, err := source.HandleWebhook(ctx, payload)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		resp, _ := json.Marshal(map[string]string{"error": fmt.Sprintf("failed to process webhook: %v", err)})
		_, _ = w.Write(resp)
		return
	}

	// Check if async handler is configured
	a.mu.RLock()
	asyncHandler := a.asyncAlertHandler
	runner := a.alertRunner
	syncHandler := a.alertHandler
	a.mu.RUnlock()

	// Use async dispatch if configured
	if asyncHandler != nil && runner != nil {
		a.handleWebhookAsync(w, alerts, asyncHandler, runner)
		return
	}

	// Fall back to sync dispatch
	var handlerErrors int
	for _, alert := range alerts {
		if syncHandler != nil {
			if err := syncHandler(ctx, alert); err != nil {
				handlerErrors++
			}
		}
	}

	// Return success
	w.WriteHeader(http.StatusOK)
	resp, _ := json.Marshal(map[string]interface{}{
		"status":   "ok",
		"received": len(alerts),
		"errors":   handlerErrors,
	})
	_, _ = w.Write(resp)
}

// handleWebhookAsync handles alerts asynchronously, returning 202 Accepted immediately.
func (a *HTTPAdapter) handleWebhookAsync(
	w http.ResponseWriter,
	alerts []*entity.Alert,
	asyncHandler port.AsyncAlertHandler,
	runner port.AlertRunner,
) {
	var lastInvID string
	var startErrors int

	for _, alert := range alerts {
		// Start investigation and get ID (non-blocking)
		invID, err := asyncHandler(context.Background(), alert)
		if err != nil {
			startErrors++
			continue
		}

		// Empty ID means alert was filtered out (ignored source/severity)
		if invID == "" {
			continue
		}

		lastInvID = invID

		// Run investigation in background
		a.wg.Add(1)
		go func(alert *entity.Alert, invID string) {
			defer a.wg.Done()
			// Use background context since HTTP request context will be cancelled
			_ = runner(context.Background(), alert, invID)
		}(alert, invID)
	}

	// Return 202 Accepted immediately
	if lastInvID != "" {
		w.WriteHeader(http.StatusAccepted)
		resp, _ := json.Marshal(map[string]interface{}{
			"status":           "accepted",
			"investigation_id": lastInvID,
		})
		_, _ = w.Write(resp)
		return
	}

	// No investigations started (all filtered or errors)
	if startErrors > 0 {
		w.WriteHeader(http.StatusInternalServerError)
		resp, _ := json.Marshal(map[string]interface{}{
			"error":  "failed to start investigations",
			"errors": startErrors,
		})
		_, _ = w.Write(resp)
		return
	}

	// All alerts filtered out
	w.WriteHeader(http.StatusOK)
	resp, _ := json.Marshal(map[string]interface{}{
		"status":  "ok",
		"message": "no investigations started (alerts filtered)",
	})
	_, _ = w.Write(resp)
}

// findWebhookSource finds a webhook source by its path.
func (a *HTTPAdapter) findWebhookSource(path string) port.WebhookAlertSource {
	sources := a.sourceManager.ListSources()
	for _, src := range sources {
		if webhookSrc, ok := src.(port.WebhookAlertSource); ok {
			if webhookSrc.WebhookPath() == path {
				return webhookSrc
			}
		}
	}
	return nil
}

// SetAlertHandler sets the callback for handling parsed alerts synchronously.
func (a *HTTPAdapter) SetAlertHandler(handler port.AlertHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.alertHandler = handler
}

// SetAsyncAlertHandler sets the async handler and runner for async alert processing.
// When set, handleWebhook will return 202 Accepted immediately and run investigations
// in background goroutines. The handler starts the investigation and returns the ID,
// while the runner executes the actual investigation.
func (a *HTTPAdapter) SetAsyncAlertHandler(handler port.AsyncAlertHandler, runner port.AlertRunner) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.asyncAlertHandler = handler
	a.alertRunner = runner
}

// Start begins listening for HTTP requests.
// This method blocks until the context is cancelled or an error occurs.
func (a *HTTPAdapter) Start(ctx context.Context) error {
	a.mu.Lock()
	if a.started {
		a.mu.Unlock()
		return nil
	}

	a.server = &http.Server{
		Addr:         a.config.Addr,
		Handler:      a.mux,
		ReadTimeout:  a.config.ReadTimeout,
		WriteTimeout: a.config.WriteTimeout,
	}
	a.started = true
	a.mu.Unlock()

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := a.server.ListenAndServe(); err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()

	// Wait for context cancellation or server error
	select {
	case <-ctx.Done():
		return a.Shutdown()
	case err := <-errCh:
		return err
	}
}

// Shutdown gracefully stops the HTTP server.
// It waits for in-flight async investigations to complete before closing.
func (a *HTTPAdapter) Shutdown() error {
	// Always wait for in-flight async investigations to complete
	a.wg.Wait()

	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.started || a.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
	defer cancel()

	err := a.server.Shutdown(ctx)
	a.started = false
	return err
}

// Addr returns the configured address.
func (a *HTTPAdapter) Addr() string {
	return a.config.Addr
}

// Mux returns the HTTP mux for testing purposes.
func (a *HTTPAdapter) Mux() *http.ServeMux {
	return a.mux
}

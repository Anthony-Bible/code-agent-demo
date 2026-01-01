// Package webhook provides HTTP adapters for receiving webhook alerts.
// It implements an HTTP server that routes incoming webhooks to registered
// alert sources for processing.
package webhook

import (
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
	sourceManager port.AlertSourceManager
	alertHandler  port.AlertHandler
	config        HTTPAdapterConfig
	server        *http.Server
	mux           *http.ServeMux
	mu            sync.RWMutex
	started       bool
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

	// Dispatch alerts to the handler
	a.mu.RLock()
	handler := a.alertHandler
	a.mu.RUnlock()

	var handlerErrors int
	for _, alert := range alerts {
		if handler != nil {
			if err := handler(ctx, alert); err != nil {
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

// SetAlertHandler sets the callback for handling parsed alerts.
func (a *HTTPAdapter) SetAlertHandler(handler port.AlertHandler) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.alertHandler = handler
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
func (a *HTTPAdapter) Shutdown() error {
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

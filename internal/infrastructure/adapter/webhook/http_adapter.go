// Package webhook provides HTTP adapters for receiving webhook alerts.
// It implements an HTTP server that routes incoming webhooks to registered
// alert sources for processing.
package webhook

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// maxBodySize is the maximum allowed size for webhook request bodies (10MB).
const maxBodySize = 10 << 20

// basicAuthEntry holds the expected credentials for a single webhook path.
// Credentials are never logged; only their presence is noted.
type basicAuthEntry struct {
	username string
	password string
}

// HTTPAdapterConfig configures the webhook HTTP server.
type HTTPAdapterConfig struct {
	// Addr is the address to listen on (e.g., ":8080", "0.0.0.0:9090").
	Addr string
	// InternalAddr is the address for the internal probe server (e.g., ":8081").
	// The /health and /ready endpoints are served exclusively on this address.
	// In Kubernetes, security is enforced by not exposing this port via a Service,
	// rather than by binding to loopback.
	InternalAddr string
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
		InternalAddr:    ":8081",
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
	internalServer    *http.Server
	mux               *http.ServeMux
	internalMux       *http.ServeMux
	mu                sync.RWMutex
	wg                sync.WaitGroup // tracks in-flight async investigations
	invCtx            context.Context
	invCancel         context.CancelFunc
	started           bool
	logger            port.Logger
	// basicAuth maps webhook paths to their required credentials.
	// Protected by credsMu because SetBasicAuth may be called concurrently.
	basicAuth map[string]basicAuthEntry
	credsMu   sync.RWMutex
	// warmupRequired marks readiness as gated on a successful warmup signal.
	// When true, /ready returns 503 until warmupReady flips to true. When
	// false (default), readiness is governed only by source registration —
	// matching legacy behavior for callers that don't do warmup.
	warmupRequired atomic.Bool
	// warmupReady is set by MarkWarm once the warmup goroutine completes
	// (success OR failure — a cold cache is a latency penalty, not a
	// correctness issue, so we don't wedge the server forever on warmup
	// errors).
	warmupReady atomic.Bool
}

// NewHTTPAdapter creates a new webhook HTTP adapter.
func NewHTTPAdapter(
	sourceManager port.AlertSourceManager,
	config HTTPAdapterConfig,
	log port.Logger,
) *HTTPAdapter {
	invCtx, invCancel := context.WithCancel(context.Background()) //nolint:gosec // G118: cancel is stored in invCancel and called in Shutdown
	adapter := &HTTPAdapter{
		sourceManager: sourceManager,
		config:        config,
		mux:           http.NewServeMux(),
		internalMux:   http.NewServeMux(),
		invCtx:        invCtx,
		invCancel:     invCancel,
		logger:        port.SafeLogger(log),
		basicAuth:     make(map[string]basicAuthEntry),
	}
	adapter.registerRoutes()
	adapter.registerInternalRoutes()
	return adapter
}

// registerRoutes sets up the HTTP routes using Go 1.22+ syntax.
func (a *HTTPAdapter) registerRoutes() {
	// Dynamic webhook routes based on registered sources
	// Using a catch-all pattern that routes to the appropriate source
	a.mux.HandleFunc("POST /alerts/{source...}", a.handleWebhook)
}

// registerInternalRoutes sets up the internal-only probe endpoints.
// These are served on a separate listener (InternalAddr) that is not
// exposed via the public-facing Service in Kubernetes.
func (a *HTTPAdapter) registerInternalRoutes() {
	a.internalMux.HandleFunc("GET /health", a.handleHealth)
	a.internalMux.HandleFunc("GET /ready", a.handleReady)
}

// handleHealth returns 200 OK if the server is running.
func (a *HTTPAdapter) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

// handleReady returns 200 OK if at least one alert source is registered AND
// any required warmup has completed. When warmup is gated (see
// SetWarmupRequired), the endpoint returns 503 with status "warming up"
// until MarkWarm is called.
func (a *HTTPAdapter) handleReady(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	sources := a.sourceManager.ListSources()
	if len(sources) == 0 {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"no sources registered"}`))
		return
	}

	if a.warmupRequired.Load() && !a.warmupReady.Load() {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"status":"warming up"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = fmt.Fprintf(w, `{"status":"ok","sources":%d}`, len(sources))
}

// SetWarmupRequired gates readiness on a successful MarkWarm signal. Call
// this BEFORE Start if startup includes an async cache-warmup step. When
// not called, readiness is governed solely by source registration (legacy
// behavior).
func (a *HTTPAdapter) SetWarmupRequired(required bool) {
	a.warmupRequired.Store(required)
}

// MarkWarm flips the readiness gate to allow /ready to report 200 OK.
// Safe to call multiple times. Should be called once the warmup goroutine
// finishes — succeed OR fail; a cold cache is only a latency cost, not a
// correctness one, so we don't wedge readiness on warmup errors.
func (a *HTTPAdapter) MarkWarm() {
	a.warmupReady.Store(true)
}

// SetBasicAuth registers HTTP Basic Auth credentials for a specific webhook path.
// When credentials are set for a path, every incoming request to that path must
// supply a matching Authorization header or it will be rejected with 401.
// Credentials are compared using constant-time equality to prevent timing attacks.
// Calling SetBasicAuth with an empty username and password removes any existing
// credentials for the path (making the endpoint unauthenticated again).
func (a *HTTPAdapter) SetBasicAuth(path, username, password string) {
	a.credsMu.Lock()
	defer a.credsMu.Unlock()
	if username == "" && password == "" {
		delete(a.basicAuth, path)
		return
	}
	if username == "" || password == "" {
		a.logger.Warn("SetBasicAuth called with partial credentials — both username and password must be non-empty; ignoring", "path", path)
		return
	}
	a.basicAuth[path] = basicAuthEntry{username: username, password: password}
}

// hashCredential hashes b with SHA-256 so that ConstantTimeCompare
// always operates on equal-length slices regardless of input length.
func hashCredential(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

// checkBasicAuth validates the request's Basic Auth credentials against the
// registered credentials for the given path.
// Returns true when the path has no auth requirement, or when the supplied
// credentials match exactly (constant-time comparison).
// Returns false when credentials are required but missing or incorrect; in that
// case the appropriate 401 response has already been written to w.
func (a *HTTPAdapter) checkBasicAuth(w http.ResponseWriter, r *http.Request, path string) bool {
	a.credsMu.RLock()
	entry, required := a.basicAuth[path]
	a.credsMu.RUnlock()

	if !required {
		return true
	}

	username, password, ok := r.BasicAuth()
	if !ok {
		// No credentials provided at all
		w.Header().Set("WWW-Authenticate", `Basic realm="webhook"`)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"authentication required"}`))
		// Log only that auth was missing, never the expected credentials
		a.logger.Warn("Webhook request rejected: missing Basic Auth credentials", "path", path)
		return false
	}

	// Constant-time comparison to prevent timing-based credential enumeration
	usernameMatch := subtle.ConstantTimeCompare(hashCredential([]byte(username)), hashCredential([]byte(entry.username)))
	passwordMatch := subtle.ConstantTimeCompare(hashCredential([]byte(password)), hashCredential([]byte(entry.password)))
	// Use bitwise & (not &&) to ensure both comparisons complete before combining,
	// preventing short-circuit evaluation that could leak which credential was wrong.
	if usernameMatch&passwordMatch != 1 {
		w.Header().Set("WWW-Authenticate", `Basic realm="webhook"`)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid credentials"}`))
		// Log only the path and that auth failed; never log the supplied or expected credentials
		a.logger.Warn("Webhook request rejected: invalid Basic Auth credentials", "path", path)
		return false
	}

	return true
}

// handleWebhook routes incoming webhooks to the appropriate source.
func (a *HTTPAdapter) handleWebhook(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Reconstruct the full path from the wildcard
	sourcePath := r.PathValue("source")
	path := "/alerts/" + sourcePath

	// Validate Basic Auth before any other processing.
	// This ensures credentials are checked before we reveal whether a path exists.
	if !a.checkBasicAuth(w, r, path) {
		return
	}

	// Find the matching webhook source
	source := a.sourceManager.GetWebhookSourceByPath(path)
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
		log := a.logger.With("alert_id", alert.ID())

		// Start investigation and get ID (non-blocking)
		invID, err := asyncHandler(context.Background(), alert)
		if err != nil {
			log.Error("Failed to start investigation for alert", "error", err)
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
		go func(invID string) {
			defer a.wg.Done()
			invLog := log.With("investigation_id", invID)
			// Use investigation context so it can be cancelled during shutdown
			if err := runner(a.invCtx, alert, invID); err != nil {
				invLog.Error("Async investigation failed", "error", err)
			}
		}(invID)
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

	a.server = a.newServer(a.config.Addr, a.mux)

	// Internal probe server is started alongside the main server when InternalAddr
	// is configured. In Kubernetes, port 8081 is intentionally not exposed via a
	// Service so only the kubelet can reach it via the Pod IP.
	if a.config.InternalAddr != "" {
		a.internalServer = a.newServer(a.config.InternalAddr, a.internalMux)
	}

	a.started = true
	a.mu.Unlock()

	// Start internal probe server in a goroutine (best-effort; a failure here
	// is logged but does not prevent the main server from running).
	if a.internalServer != nil {
		go func() {
			if err := a.internalServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				a.logger.Error("Internal probe server stopped unexpectedly", "error", err, "addr", a.config.InternalAddr)
			}
		}()
	}

	// Start main server in goroutine
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
// It cancels running investigations and waits up to 5 seconds for them to complete.
func (a *HTTPAdapter) Shutdown() error {
	// Cancel all running investigations
	a.invCancel()

	// Wait for investigations with timeout
	done := make(chan struct{})
	go func() {
		a.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// All investigations finished cleanly
	case <-time.After(5 * time.Second):
		// Timeout - proceed with server shutdown anyway
	}

	// Shut down HTTP server
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.started || a.server == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), a.config.ShutdownTimeout)
	defer cancel()

	// Stop the probe server first so load balancers see /ready fail and stop
	// routing new traffic before we drain in-flight requests on the main server.
	// 2s timeout is ample for trivially short probe connections.
	if a.internalServer != nil {
		probeCtx, probeCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer probeCancel()
		_ = a.internalServer.Shutdown(probeCtx)
	}

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

// InternalMux returns the internal-only HTTP mux (probe endpoints) for testing purposes.
func (a *HTTPAdapter) InternalMux() *http.ServeMux {
	return a.internalMux
}

func (a *HTTPAdapter) newServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  a.config.ReadTimeout,
		WriteTimeout: a.config.WriteTimeout,
	}
}

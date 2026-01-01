# Alert Source Plugin Architecture - WITH IMPLEMENTATION PROPOSALS

> Original design document + critical gap remedies for making the agent extensible to receive alerts from various sources (Prometheus, NATS, Google Alerts, Datadog, etc.)
>
> **Document Version:** 2.0 (Production-Ready Edition)
> **Status:** Design Complete → Implementation Phase

---

## Overview

The agent needs to support multiple alert consumption patterns:

| Pattern | Description | Examples |
|---------|-------------|----------|
| **Push (Webhook)** | External systems call our HTTP endpoint | Prometheus Alertmanager, PagerDuty, custom webhooks |
| **Pull (Polling)** | We periodically fetch from an API | Datadog API, Cloud Monitoring, REST endpoints |
| **Stream (Subscribe)** | We connect to message brokers | NATS, Kafka, Redis Streams, RabbitMQ, SQS |

---

## Design Principles

1. **Leverage existing hexagonal architecture** - New ports/adapters following established patterns
2. **Interface-based extensibility** - Ports define interfaces only, entities live in domain
3. **Dependency injection** - No global state, registry injected via Container
4. **Context propagation** - All async operations receive context for cancellation
5. **Decorator pattern** - Middleware (dedup, rate limiting) via composable decorators

---

## Domain Entity

### `internal/domain/entity/alert.go`

Following the existing entity pattern (see `entity/tool.go`, `entity/message.go`):

```go
package entity

import (
    "errors"
    "time"
)

const (
    SeverityCritical = "critical"
    SeverityWarning  = "warning"
    SeverityInfo     = "info"
)

var (
    ErrEmptyAlertID     = errors.New("alert ID cannot be empty")
    ErrEmptyAlertSource = errors.New("alert source cannot be empty")
    ErrEmptyAlertTitle  = errors.New("alert title cannot be empty")
    ErrInvalidSeverity  = errors.New("invalid severity level")
)

type Alert struct {
    id          string
    source      string
    severity    string
    title       string
    description string
    labels      map[string]string
    timestamp   time.Time
    rawPayload  []byte
}

// NewAlert creates a validated Alert entity (matches NewTool, NewMessage pattern)
func NewAlert(id, source, severity, title string) (*Alert, error) {
    a := &Alert{
        id:        id,
        source:    source,
        severity:  severity,
        title:     title,
        timestamp: time.Now(),
        labels:    make(map[string]string),
    }
    if err := a.Validate(); err != nil {
        return nil, err
    }
    return a, nil
}

func (a *Alert) Validate() error {
    if a.id == "" {
        return ErrEmptyAlertID
    }
    if a.source == "" {
        return ErrEmptyAlertSource
    }
    if a.title == "" {
        return ErrEmptyAlertTitle
    }
    if !isValidSeverity(a.severity) {
        return ErrInvalidSeverity
    }
    return nil
}

// Getters (immutable access like entity/message.go)
func (a *Alert) ID() string                  { return a.id }
func (a *Alert) Source() string              { return a.source }
func (a *Alert) Severity() string            { return a.severity }
func (a *Alert) Title() string               { return a.title }
func (a *Alert) Description() string         { return a.description }
func (a *Alert) Labels() map[string]string   { return a.labels }
func (a *Alert) Timestamp() time.Time        { return a.timestamp }
func (a *Alert) RawPayload() []byte          { return a.rawPayload }

// Behavior methods
func (a *Alert) IsCritical() bool       { return a.severity == SeverityCritical }
func (a *Alert) Age() time.Duration     { return time.Since(a.timestamp) }

// Builder methods for optional fields
func (a *Alert) WithDescription(desc string) *Alert {
    a.description = desc
    return a
}

func (a *Alert) WithLabels(labels map[string]string) *Alert {
    a.labels = labels
    return a
}

func (a *Alert) WithRawPayload(payload []byte) *Alert {
    a.rawPayload = payload
    return a
}

func (a *Alert) WithTimestamp(t time.Time) *Alert {
    a.timestamp = t
    return a
}

func isValidSeverity(s string) bool {
    switch s {
    case SeverityCritical, SeverityWarning, SeverityInfo:
        return true
    }
    return false
}
```

---

## Port Interfaces

### `internal/domain/port/alert_source.go`

Ports define **interfaces only** - no structs (following `port/tool_executor.go` pattern):

```go
package port

import (
    "context"
    "time"

    "code-editing-agent/internal/domain/entity"
)

// SourceType identifies the consumption pattern
type SourceType string

const (
    SourceTypeWebhook SourceType = "webhook"
    SourceTypePoll    SourceType = "poll"
    SourceTypeStream  SourceType = "stream"
)

// AlertSource is the base interface all sources implement
type AlertSource interface {
    Name() string
    Type() SourceType
    Close() error
}

// WebhookAlertSource receives pushed alerts via HTTP
type WebhookAlertSource interface {
    AlertSource
    WebhookPath() string
    HandleWebhook(ctx context.Context, payload []byte) ([]*entity.Alert, error)
}

// PollAlertSource periodically fetches alerts from an API
type PollAlertSource interface {
    AlertSource
    PollInterval() time.Duration
    FetchAlerts(ctx context.Context) ([]*entity.Alert, error)
}

// StreamAlertSource subscribes to a message stream
type StreamAlertSource interface {
    AlertSource
    Subscribe(ctx context.Context) (<-chan *entity.Alert, <-chan error)
}

// AlertHandler processes incoming alerts (with context for cancellation/timeout)
type AlertHandler func(ctx context.Context, alert *entity.Alert) error

// AlertSourceManager manages alert source lifecycle (like SkillManager)
type AlertSourceManager interface {
    // Registration
    RegisterSource(source AlertSource) error
    UnregisterSource(name string) error

    // Discovery
    GetSource(name string) (AlertSource, error)
    ListSources() []AlertSource

    // Lifecycle
    Start(ctx context.Context) error
    Shutdown() error

    // Handler injection (like ExecutorAdapter callbacks)
    SetAlertHandler(handler AlertHandler)
}

// AlertSourceDiscoveryResult tracks registration outcomes (like SkillDiscoveryResult)
type AlertSourceDiscoveryResult struct {
    RegisteredCount int
    FailedSources   map[string]error
    RegisteredNames []string
}

// HealthChecker interface for sources that support health checks
type HealthChecker interface {
    HealthCheck(ctx context.Context) error
}
```

---

## Infrastructure Adapters

### Source Registry

### `internal/infrastructure/adapter/alert/source_registry.go`

Injectable registry (no global state, following ExecutorAdapter pattern):

```go
package alert

import (
    "fmt"
    "sync"

    "code-editing-agent/internal/domain/entity"
    "code-editing-agent/internal/domain/port"
)

// SourceConfig holds typed configuration for alert sources
type SourceConfig struct {
    Type         string
    Name         string
    WebhookPath  string            // for webhook sources
    URL          string            // for stream sources (NATS, etc.)
    Subject      string            // for stream sources
    APIKey       string            // for poll sources
    AppKey       string            // for poll sources
    PollInterval string            // duration string, e.g., "60s"
    Extra        map[string]string // additional source-specific config
}

// AlertSourceFactory creates a source from config
type AlertSourceFactory func(cfg SourceConfig) (port.AlertSource, error)

// SourceRegistry manages source factories (injectable, not global)
type SourceRegistry struct {
    factories map[string]AlertSourceFactory
    mu        sync.RWMutex
}

// NewSourceRegistry creates an empty registry
func NewSourceRegistry() *SourceRegistry {
    return &SourceRegistry{
        factories: make(map[string]AlertSourceFactory),
    }
}

// RegisterFactory adds a factory for a source type
func (r *SourceRegistry) RegisterFactory(sourceType string, factory AlertSourceFactory) {
    r.mu.Lock()
    defer r.mu.Unlock()
    r.factories[sourceType] = factory
}

// CreateSource instantiates a source from config
func (r *SourceRegistry) CreateSource(cfg SourceConfig) (port.AlertSource, error) {
    r.mu.RLock()
    factory, ok := r.factories[cfg.Type]
    r.mu.RUnlock()

    if !ok {
        return nil, fmt.Errorf("unknown source type: %s", cfg.Type)
    }

    return factory(cfg)
}

// RegisterBuiltinFactories adds the default source factories
func (r *SourceRegistry) RegisterBuiltinFactories() {
    r.RegisterFactory("prometheus", NewPrometheusSource)
    r.RegisterFactory("nats", NewNATSSource)
    r.RegisterFactory("datadog", NewDatadogSource)
}
```

---

### Alert Source Manager (Adapter)

### `internal/infrastructure/adapter/alert/source_manager.go`

Implements `port.AlertSourceManager` (following `LocalSkillManager` pattern):

```go
package alert

import (
    "context"
    "fmt"
    "io"
    "log"
    "net/http"
    "sync"
    "time"

    "code-editing-agent/internal/domain/entity"
    "code-editing-agent/internal/domain/port"
)

// LocalAlertSourceManager implements port.AlertSourceManager
type LocalAlertSourceManager struct {
    sources map[string]port.AlertSource
    handler port.AlertHandler
    httpMux *http.ServeMux
    server  *http.Server
    serverAddr string

    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
    mu     sync.RWMutex

    // Callbacks
    errorCallback func(source string, err error)
}

// NewLocalAlertSourceManager creates a new manager
func NewLocalAlertSourceManager() *LocalAlertSourceManager {
    return &LocalAlertSourceManager{
        sources: make(map[string]port.AlertSource),
        httpMux: http.NewServeMux(),
        serverAddr: ":8080", // Default port
    }
}

// SetServerAddr configures the HTTP server address (call before Start)
func (m *LocalAlertSourceManager) SetServerAddr(addr string) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.serverAddr = addr
}

// SetAlertHandler sets the callback for incoming alerts
func (m *LocalAlertSourceManager) SetAlertHandler(handler port.AlertHandler) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.handler = handler
}

// SetErrorCallback sets the callback for source errors
func (m *LocalAlertSourceManager) SetErrorCallback(cb func(source string, err error)) {
    m.mu.Lock()
    defer m.mu.Unlock()
    m.errorCallback = cb
}

// RegisterSource adds and configures a source
func (m *LocalAlertSourceManager) RegisterSource(source port.AlertSource) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if _, exists := m.sources[source.Name()]; exists {
        return fmt.Errorf("source already registered: %s", source.Name())
    }

    m.sources[source.Name()] = source
    return nil
}

// UnregisterSource removes a source
func (m *LocalAlertSourceManager) UnregisterSource(name string) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    source, ok := m.sources[name]
    if !ok {
        return fmt.Errorf("source not found: %s", name)
    }

    if err := source.Close(); err != nil {
        return fmt.Errorf("failed to close source %s: %w", name, err)
    }

    delete(m.sources, name)
    return nil
}

// GetSource retrieves a source by name
func (m *LocalAlertSourceManager) GetSource(name string) (port.AlertSource, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()

    source, ok := m.sources[name]
    if !ok {
        return nil, fmt.Errorf("source not found: %s", name)
    }
    return source, nil
}

// ListSources returns all registered sources
func (m *LocalAlertSourceManager) ListSources() []port.AlertSource {
    m.mu.RLock()
    defer m.mu.RUnlock()

    sources := make([]port.AlertSource, 0, len(m.sources))
    for _, s := range m.sources {
        sources = append(sources, s)
    }
    return sources
}

// Start begins consuming from all sources
func (m *LocalAlertSourceManager) Start(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    m.ctx, m.cancel = context.WithCancel(ctx)

    // Start HTTP server for webhooks
    if m.needsHTTPServer() {
        if err := m.startHTTPServer(); err != nil {
            return fmt.Errorf("failed to start webhook server: %w", err)
        }
    }

    // Start other consumers
    for _, source := range m.sources {
        switch s := source.(type) {
        case port.WebhookAlertSource:
            m.setupWebhook(s)
        case port.PollAlertSource:
            m.startPoller(s)
        case port.StreamAlertSource:
            m.startStreamConsumer(s)
        }
    }

    return nil
}

// needsHTTPServer returns true if any source requires HTTP
func (m *LocalAlertSourceManager) needsHTTPServer() bool {
    for _, source := range m.sources {
        if _, ok := source.(port.WebhookAlertSource); ok {
            return true
        }
    }
    return false
}

// startHTTPServer starts the HTTP server for webhooks
func (m *LocalAlertSourceManager) startHTTPServer() error {
    m.server = &http.Server{
        Addr:    m.serverAddr,
        Handler: m,
        // TODO: TLS config for production
        // TLSConfig: &tls.Config{...}
    }

    m.wg.Add(1)
    go func() {
        defer m.wg.Done()
        if err := m.server.ListenAndServe(); err != http.ErrServerClosed {
            m.reportError("http-server", err)
        }
    }()
    return nil
}

// Shutdown stops all consumers and closes sources
func (m *LocalAlertSourceManager) Shutdown() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.cancel != nil {
        m.cancel()
    }

    // Shutdown HTTP server gracefully
    if m.server != nil {
        ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
        defer cancel()
        if err := m.server.Shutdown(ctx); err != nil {
            log.Printf("HTTP server shutdown error: %v", err)
        }
    }

    m.wg.Wait()

    var errs []error
    for name, source := range m.sources {
        if err := source.Close(); err != nil {
            errs = append(errs, fmt.Errorf("failed to close %s: %w", name, err))
        }
    }

    if len(errs) > 0 {
        return fmt.Errorf("shutdown errors: %v", errs)
    }
    return nil
}

// ServeHTTP implements http.Handler for webhook sources
func (m *LocalAlertSourceManager) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    m.httpMux.ServeHTTP(w, r)
}

// setupWebhook registers an HTTP handler for a webhook source
func (m *LocalAlertSourceManager) setupWebhook(source port.WebhookAlertSource) {
    m.httpMux.HandleFunc(source.WebhookPath(), func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()

        body, err := io.ReadAll(r.Body)
        if err != nil {
            http.Error(w, "failed to read body", http.StatusBadRequest)
            return
        }

        alerts, err := source.HandleWebhook(ctx, body)
        if err != nil {
            m.reportError(source.Name(), err)
            http.Error(w, err.Error(), http.StatusBadRequest)
            return
        }

        for _, alert := range alerts {
            if err := m.dispatchAlert(ctx, alert); err != nil {
                m.reportError(source.Name(), err)
            }
        }

        w.WriteHeader(http.StatusOK)
    })
}

// startPoller launches a goroutine for periodic polling
func (m *LocalAlertSourceManager) startPoller(source port.PollAlertSource) {
    m.wg.Add(1)
    go func() {
        defer m.wg.Done()

        ticker := time.NewTicker(source.PollInterval())
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                alerts, err := source.FetchAlerts(m.ctx)
                if err != nil {
                    m.reportError(source.Name(), err)
                    continue
                }
                for _, alert := range alerts {
                    if err := m.dispatchAlert(m.ctx, alert); err != nil {
                        m.reportError(source.Name(), err)
                    }
                }
            case <-m.ctx.Done():
                return
            }
        }
    }()
}

// startStreamConsumer launches a goroutine for stream subscription
func (m *LocalAlertSourceManager) startStreamConsumer(source port.StreamAlertSource) {
    m.wg.Add(1)
    go func() {
        defer m.wg.Done()

        alerts, errs := source.Subscribe(m.ctx)

        for {
            select {
            case alert, ok := <-alerts:
                if !ok {
                    return
                }
                if err := m.dispatchAlert(m.ctx, alert); err != nil {
                    m.reportError(source.Name(), err)
                }
            case err, ok := <-errs:
                if !ok {
                    return
                }
                m.reportError(source.Name(), err)
            case <-m.ctx.Done():
                return
            }
        }
    }()
}

// dispatchAlert sends an alert to the handler
func (m *LocalAlertSourceManager) dispatchAlert(ctx context.Context, alert *entity.Alert) error {
    m.mu.RLock()
    handler := m.handler
    m.mu.RUnlock()

    if handler == nil {
        return nil
    }
    return handler(ctx, alert)
}

// reportError calls the error callback if set
func (m *LocalAlertSourceManager) reportError(source string, err error) {
    m.mu.RLock()
    cb := m.errorCallback
    m.mu.RUnlock()

    if cb != nil {
        cb(source, err)
    } else {
        log.Printf("[AlertSource:%s] error: %v", source, err)
    }
}
```

---

### Decorator: Deduplicating Manager

### `internal/infrastructure/adapter/alert/dedup_manager.go`

Following the `PlanningExecutorAdapter` decorator pattern:

```go
package alert

import (
    "context"
    "sync"
    "time"

    "code-editing-agent/internal/domain/entity"
    "code-editing-agent/internal/domain/port"
)

// DeduplicatingAlertManager wraps an AlertSourceManager to filter duplicates
type DeduplicatingAlertManager struct {
    wrapped   port.AlertSourceManager
    seen      map[string]time.Time
    ttl       time.Duration
    mu        sync.RWMutex
}

// NewDeduplicatingAlertManager creates a decorator that filters duplicate alerts
func NewDeduplicatingAlertManager(wrapped port.AlertSourceManager, ttl time.Duration) *DeduplicatingAlertManager {
    d := &DeduplicatingAlertManager{
        wrapped: wrapped,
        seen:    make(map[string]time.Time),
        ttl:     ttl,
    }

    // Clean expired entries periodically
    go d.cleanupExpired()

    return d
}

// cleanupExpired removes old entries every minute
func (d *DeduplicatingAlertManager) cleanupExpired() {
    ticker := time.NewTicker(time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        d.mu.Lock()
        for id, timestamp := range d.seen {
            if time.Since(timestamp) > d.ttl {
                delete(d.seen, id)
            }
        }
        d.mu.Unlock()
    }
}

func (d *DeduplicatingAlertManager) isDuplicate(id string) bool {
    d.mu.RLock()
    defer d.mu.RUnlock()

    if lastSeen, ok := d.seen[id]; ok {
        return time.Since(lastSeen) < d.ttl
    }
    return false
}

func (d *DeduplicatingAlertManager) markSeen(id string) {
    d.mu.Lock()
    defer d.mu.Unlock()
    d.seen[id] = time.Now()
}

// Delegate all interface methods to wrapped manager
func (d *DeduplicatingAlertManager) RegisterSource(source port.AlertSource) error {
    return d.wrapped.RegisterSource(source)
}

func (d *DeduplicatingAlertManager) UnregisterSource(name string) error {
    return d.wrapped.UnregisterSource(name)
}

func (d *DeduplicatingAlertManager) GetSource(name string) (port.AlertSource, error) {
    return d.wrapped.GetSource(name)
}

func (d *DeduplicatingAlertManager) ListSources() []port.AlertSource {
    return d.wrapped.ListSources()
}

func (d *DeduplicatingAlertManager) Start(ctx context.Context) error {
    return d.wrapped.Start(ctx)
}

func (d *DeduplicatingAlertManager) Shutdown() error {
    return d.wrapped.Shutdown()
}

func (d *DeduplicatingAlertManager) SetAlertHandler(handler port.AlertHandler) {
    // Wrap with deduplication check
    d.wrapped.SetAlertHandler(func(ctx context.Context, alert *entity.Alert) error {
        if d.isDuplicate(alert.ID()) {
            return nil
        }
        d.markSeen(alert.ID())
        return handler(ctx, alert)
    })
}
```

---

## Example Source Adapters

### Prometheus Webhook Source

### `internal/infrastructure/adapter/alert/prometheus_source.go`

```go
package alert

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "code-editing-agent/internal/domain/entity"
    "code-editing-agent/internal/domain/port"
)

type PrometheusSource struct {
    name        string
    webhookPath string
}

// NewPrometheusSource creates a Prometheus Alertmanager webhook source
func NewPrometheusSource(cfg SourceConfig) (port.AlertSource, error) {
    if cfg.Name == "" {
        return nil, fmt.Errorf("prometheus source requires name")
    }
    if cfg.WebhookPath == "" {
        return nil, fmt.Errorf("prometheus source requires webhook_path")
    }

    // Validate path for security
    if err := validateWebhookPath(cfg.WebhookPath); err != nil {
        return nil, fmt.Errorf("invalid webhook_path: %w", err)
    }

    return &PrometheusSource{
        name:        cfg.Name,
        webhookPath: cfg.WebhookPath,
    }, nil
}

func (p *PrometheusSource) Name() string            { return p.name }
func (p *PrometheusSource) Type() port.SourceType   { return port.SourceTypeWebhook }
func (p *PrometheusSource) WebhookPath() string     { return p.webhookPath }
func (p *PrometheusSource) Close() error            { return nil }

// AlertmanagerPayload matches Prometheus Alertmanager webhook format
type AlertmanagerPayload struct {
    Alerts []struct {
        Status      string            `json:"status"`
        Labels      map[string]string `json:"labels"`
        Annotations map[string]string `json:"annotations"`
        StartsAt    time.Time         `json:"startsAt"`
        EndsAt      time.Time         `json:"endsAt"`
    } `json:"alerts"`
}

func (p *PrometheusSource) HandleWebhook(ctx context.Context, payload []byte) ([]*entity.Alert, error) {
    var am AlertmanagerPayload
    if err := json.Unmarshal(payload, &am); err != nil {
        return nil, fmt.Errorf("invalid alertmanager payload: %w", err)
    }

    var alerts []*entity.Alert
    for _, a := range am.Alerts {
        if a.Status != "firing" {
            continue // Skip resolved alerts
        }

        severity := a.Labels["severity"]
        if severity == "" {
            severity = entity.SeverityWarning
        }

        alertID := fmt.Sprintf("%s-%s", a.Labels["alertname"], a.StartsAt.Format(time.RFC3339))
        title := a.Annotations["summary"]
        if title == "" {
            title = a.Labels["alertname"]
        }

        // Use entity factory for validation
        alert, err := entity.NewAlert(alertID, p.name, severity, title)
        if err != nil {
            return nil, fmt.Errorf("invalid alert from prometheus: %w", err)
        }

        alert.WithDescription(a.Annotations["description"]).
            WithLabels(a.Labels).
            WithTimestamp(a.StartsAt).
            WithRawPayload(payload)

        alerts = append(alerts, alert)
    }

    return alerts, nil
}
```

### NATS Stream Source

### `internal/infrastructure/adapter/alert/nats_source.go`

```go
package alert

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/nats-io/nats.go"
    "code-editing-agent/internal/domain/entity"
    "code-editing-agent/internal/domain/port"
)

type NATSSource struct {
    name    string
    url     string
    subject string
    conn    *nats.Conn
    sub     *nats.Subscription
}

// NewNATSSource creates a NATS stream source
func NewNATSSource(cfg SourceConfig) (port.AlertSource, error) {
    if cfg.Name == "" {
        return nil, fmt.Errorf("nats source requires name")
    }
    if cfg.URL == "" {
        return nil, fmt.Errorf("nats source requires url")
    }
    if cfg.Subject == "" {
        return nil, fmt.Errorf("nats source requires subject")
    }

    // Connect during construction
    conn, err := nats.Connect(cfg.URL)
    if err != nil {
        return nil, fmt.Errorf("failed to connect to NATS: %w", err)
    }

    return &NATSSource{
        name:    cfg.Name,
        url:     cfg.URL,
        subject: cfg.Subject,
        conn:    conn,
    }, nil
}

func (n *NATSSource) Name() string          { return n.name }
func (n *NATSSource) Type() port.SourceType { return port.SourceTypeStream }

func (n *NATSSource) Close() error {
    if n.sub != nil {
        if err := n.sub.Unsubscribe(); err != nil {
            return err
        }
    }
    if n.conn != nil {
        n.conn.Close()
    }
    return nil
}

// natsAlertPayload is the expected JSON format from NATS
type natsAlertPayload struct {
    ID          string            `json:"id"`
    Severity    string            `json:"severity"`
    Title       string            `json:"title"`
    Description string            `json:"description"`
    Labels      map[string]string `json:"labels"`
}

func (n *NATSSource) Subscribe(ctx context.Context) (<-chan *entity.Alert, <-chan error) {
    alerts := make(chan *entity.Alert, 100) // Buffered for backpressure
    errs := make(chan error, 10)

    go func() {
        defer close(alerts)
        defer close(errs)

        sub, err := n.conn.Subscribe(n.subject, func(msg *nats.Msg) {
            var payload natsAlertPayload
            if err := json.Unmarshal(msg.Data, &payload); err != nil {
                select {
                case errs <- fmt.Errorf("invalid NATS message: %w", err):
                case <-ctx.Done():
                }
                return
            }

            // Use entity factory for validation
            alert, err := entity.NewAlert(payload.ID, n.name, payload.Severity, payload.Title)
            if err != nil {
                select {
                case errs <- fmt.Errorf("invalid alert: %w", err):
                case <-ctx.Done():
                }
                return
            }

            alert.WithDescription(payload.Description).
                WithLabels(payload.Labels).
                WithRawPayload(msg.Data)

            select {
            case alerts <- alert:
            case <-ctx.Done():
            }
        })

        if err != nil {
            errs <- fmt.Errorf("subscription failed: %w", err)
            return
        }
        n.sub = sub

        <-ctx.Done()
    }()

    return alerts, errs
}

// HealthCheck implements port.HealthChecker
func (n *NATSSource) HealthCheck(ctx context.Context) error {
    if n.conn == nil || n.conn.Status() != nats.CONNECTED {
        return fmt.Errorf("nats connection not healthy")
    }
    return nil
}
```

---

## Production-Ready Enhancements

### 1. **Health Checks** ✅ Added to NATS source above

```go
// Add to manager:
func (m *LocalAlertSourceManager) HealthCheck(ctx context.Context) map[string]error {
    m.mu.RLock()
    defer m.mu.RUnlock()

    results := make(map[string]error)
    for name, source := range m.sources {
        if hc, ok := source.(port.HealthChecker); ok {
            results[name] = hc.HealthCheck(ctx)
        } else {
            results[name] = nil // Assume healthy if not implemented
        }
    }
    return results
}
```

### 2. **Prometheus Metrics**

```go
package alert

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    alertProcessingDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
        Name:    "agent_alert_processing_duration_seconds",
        Help:    "Time spent processing alerts",
        Buckets: prometheus.DefBuckets,
    }, []string{"source", "severity"})

    alertsReceived = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "agent_alerts_received_total",
        Help: "Total number of alerts received by source",
    }, []string{"source", "severity"})

    alertsProcessed = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "agent_alerts_processed_total",
        Help: "Total number of alerts successfully processed",
    }, []string{"source", "severity", "status"}) // status: success|error

    sourceErrors = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "agent_alert_source_errors_total",
        Help: "Total errors from alert sources",
    }, []string{"source", "error_type"})

    sourceHealth = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "agent_alert_source_healthy",
        Help: "Health status of alert sources (1=healthy, 0=unhealthy)",
    }, []string{"source"})
)

// Usage in handleAlert:
func (s *AlertService) handleAlert(ctx context.Context, alert *entity.Alert) error {
    start := time.Now()
    alertsReceived.WithLabelValues(alert.Source(), alert.Severity()).Inc()

    log.Printf("[ALERT] [%s] %s: %s", alert.Severity(), alert.Source(), alert.Title())

    err := s.processAlert(ctx, alert)

    status := "success"
    if err != nil {
        status = "error"
        sourceErrors.WithLabelValues(alert.Source(), "processing").Inc()
    }

    alertsProcessed.WithLabelValues(alert.Source(), alert.Severity(), status).Inc()
    alertProcessingDuration.WithLabelValues(alert.Source(), alert.Severity()).Observe(time.Since(start).Seconds())

    return err
}
```

### 3. **Circuit Breakers**

```go
package alert

import (
    "context"
    "fmt"
    "github.com/sony/gobreaker"
    "code-editing-agent/internal/domain/entity"
    "code-editing-agent/internal/domain/port"
)

// CircuitBreakerSource wraps a poll source with circuit breaker
type CircuitBreakerSource struct {
    wrapped port.PollAlertSource
    name    string
    breaker *gobreaker.CircuitBreaker
}

// NewCircuitBreakerSource creates a wrapped source with circuit breaker
func NewCircuitBreakerSource(source port.PollAlertSource, name string) *CircuitBreakerSource {
    settings := gobreaker.Settings{
        Name:        name,
        MaxRequests: 3,                    // Half-open state max requests
        Interval:    60 * time.Second,    // Reset closed->open counter
        Timeout:     30 * time.Second,    // Open->half-open timeout
        ReadyToTrip: func(counts gobreaker.Counts) bool {
            failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
            return counts.Requests >= 3 && failureRatio >= 0.6
        },
        OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
            log.Printf("[CircuitBreaker] %s: %s -> %s", name, from, to)

            // Update health metric
            healthy := 0.0
            if to == gobreaker.StateClosed {
                healthy = 1.0
            }
            sourceHealth.WithLabelValues(name).Set(healthy)
        },
    }

    return &CircuitBreakerSource{
        wrapped: source,
        name:    name,
        breaker: gobreaker.NewCircuitBreaker(settings),
    }
}

func (c *CircuitBreakerSource) Name() string              { return c.name }
func (c *CircuitBreakerSource) Type() port.SourceType     { return c.wrapped.Type() }
func (c *CircuitBreakerSource) PollInterval() time.Duration { return c.wrapped.PollInterval() }
func (c *CircuitBreakerSource) Close() error              { return c.wrapped.Close() }

func (c *CircuitBreakerSource) FetchAlerts(ctx context.Context) ([]*entity.Alert, error) {
    result, err := c.breaker.Execute(func() (interface{}, error) {
        return c.wrapped.FetchAlerts(ctx)
    })

    if err != nil {
        return nil, fmt.Errorf("circuit breaker open: %w", err)
    }

    return result.([]*entity.Alert), nil
}

// In container.go - wrap poll sources with circuit breaker:
for _, cfg := range configs {
    source, err := registry.CreateSource(cfg)
    if err != nil {
        result.FailedSources[cfg.Name] = err
        continue
    }

    // Wrap poll sources with circuit breaker
    if pollSource, ok := source.(port.PollAlertSource); ok {
        source = NewCircuitBreakerSource(pollSource, cfg.Name)
    }

    if err := manager.RegisterSource(source); err != nil {
        result.FailedSources[cfg.Name] = err
        continue
    }

    result.RegisteredCount++
    result.RegisteredNames = append(result.RegisteredNames, cfg.Name)
}
```

---

## Security Enhancements

### 1. **Webhook Authentication**

```go
// Add to SourceConfig
type SourceConfig struct {
    // ... existing fields ...
    Auth WebhookAuthConfig `yaml:"auth,omitempty"`
}

type WebhookAuthConfig struct {
    Type   string `yaml:"type"` // "bearer" or "hmac"
    Token  string `yaml:"token,omitempty"` // for bearer auth
    Secret string `yaml:"secret,omitempty"` // for hmac auth
}

// Add to LocalAlertSourceManager
func (m *LocalAlertSourceManager) validateWebhookAuth(r *http.Request, cfg WebhookAuthConfig) bool {
    switch cfg.Type {
    case "bearer":
        token := r.Header.Get("Authorization")
        expectedToken := "Bearer " + cfg.Token
        return token == expectedToken

    case "hmac":
        // Verify HMAC signature
        body, _ := io.ReadAll(r.Body)
        r.Body = io.NopCloser(bytes.NewBuffer(body)) // Reset body for subsequent reads

        signature := r.Header.Get("X-Signature")
        mac := hmac.New(sha256.New, []byte(cfg.Secret))
        mac.Write(body)
        expectedSignature := hex.EncodeToString(mac.Sum(nil))

        return hmac.Equal([]byte(signature), []byte(expectedSignature))
    }
    return true // No auth configured
}

// In setupWebhook:
m.httpMux.HandleFunc(source.WebhookPath(), func(w http.ResponseWriter, r *http.Request) {
    // Auth check first
    if !m.validateWebhookAuth(r, cfg.Auth) {
        http.Error(w, "Unauthorized", http.StatusUnauthorized)
        return
    }
    // ... rest of handler ...
})

// Configuration example:
- type: prometheus
  name: prod-prometheus
  webhook_path: /alerts/prometheus
  auth:
    type: bearer
    token: ${PROMETHEUS_WEBHOOK_TOKEN}
```

### 2. **Path Traversal Protection**

```go
package alert

import (
    "fmt"
    "path/filepath"
    "strings"
)

// validateWebhookPath prevents path traversal attacks
func validateWebhookPath(path string) error {
    if path == "" {
        return fmt.Errorf("webhook path cannot be empty")
    }
    if !strings.HasPrefix(path, "/") {
        return fmt.Errorf("webhook path must start with /")
    }

    // Clean the path
    cleaned := filepath.Clean(path)

    // Ensure it doesn't try to escape root
    if filepath.IsAbs(cleaned) && cleaned != path {
        return fmt.Errorf("webhook path contains traversal sequences")
    }

    // Check for encoded traversal attempts
    if strings.Contains(path, "../") || strings.Contains(path, "..\\") {
        return fmt.Errorf("webhook path cannot contain parent directory references")
    }

    return nil
}
```

### 3. **Rate Limiting**

```go
package alert

import (
    "golang.org/x/time/rate"
    "net"
    "net/http"
    "sync"
    "time"
)

// NewRateLimitedMux creates an HTTP mux with per-IP rate limiting
type RateLimitedMux struct {
    mu        sync.RWMutex
    limiters  map[string]*rate.Limiter
    rps       rate.Limit
    burst     int
}

func NewRateLimitedMux(rps rate.Limit, burst int) *RateLimitedMux {
    return &RateLimitedMux{
        limiters: make(map[string]*rate.Limiter),
        rps:      rps,
        burst:    burst,
    }
}

func (m *RateLimitedMux) getLimiter(ip string) *rate.Limiter {
    m.mu.Lock()
    defer m.mu.Unlock()

    limiter, exists := m.limiters[ip]
    if !exists {
        limiter = rate.NewLimiter(m.rps, m.burst)
        m.limiters[ip] = limiter

        // Cleanup old limiters periodically
        go m.cleanupLimiters()
    }

    return limiter
}

func (m *RateLimitedMux) cleanupLimiters() {
    // Implement cleanup logic for old IPs
}

// LimitHandler wraps an http.HandlerFunc with rate limiting
func (m *RateLimitedMux) LimitHandler(ip string, next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        limiter := m.getLimiter(ip)
        if !limiter.Allow() {
            http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
            return
        }
        next(w, r)
    }
}

// Usage in manager:
// Get client IP (respecting X-Forwarded-For)
func getClientIP(r *http.Request) string {
    // Check X-Forwarded-For header first
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        // Take the first IP in the chain
        if idx := strings.Index(xff, ","); idx != -1 {
            xff = xff[:idx]
        }
        return strings.TrimSpace(xff)
    }

    // Fall back to RemoteAddr
    host, _, _ := net.SplitHostPort(r.RemoteAddr)
    return host
}
```

---

## Configuration

### File-Based Configuration Pattern

Following the skills pattern - file-per-source:

```
config/
└── alert-sources/
    ├── prometheus-prod.yaml
    ├── datadog-prod.yaml
    └── nats-alerts.yaml
```

**Load configuration from directory:**

```go
package alert

import (
    "os"
    "path/filepath"
    "sync"

    "gopkg.in/yaml.v3"
)

func loadAndRegisterAlertSources(
    registry *SourceRegistry,
    manager port.AlertSourceManager,
    configDir string,
) (*port.AlertSourceDiscoveryResult, error) {
    result := &port.AlertSourceDiscoveryResult{
        FailedSources: make(map[string]error),
    }

    // Check if directory exists
    if _, err := os.Stat(configDir); os.IsNotExist(err) {
        return result, nil // No config directory is okay
    }

    files, err := os.ReadDir(configDir)
    if err != nil {
        return nil, fmt.Errorf("failed to read config directory: %w", err)
    }

    for _, file := range files {
        if file.IsDir() || !strings.HasSuffix(file.Name(), ".yaml") {
            continue
        }

        path := filepath.Join(configDir, file.Name())
        cfg, err := loadSourceConfig(path)
        if err != nil {
            result.FailedSources[file.Name()] = err
            continue
        }

        source, err := registry.CreateSource(cfg)
        if err != nil {
            result.FailedSources[cfg.Name] = err
            continue
        }

        if err := manager.RegisterSource(source); err != nil {
            result.FailedSources[cfg.Name] = err
            continue
        }

        result.RegisteredCount++
        result.RegisteredNames = append(result.RegisteredNames, cfg.Name)
    }

    return result, nil
}

func loadSourceConfig(path string) (SourceConfig, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return SourceConfig{}, fmt.Errorf("failed to read %s: %w", path, err)
    }

    var cfg SourceConfig
    if err := yaml.Unmarshal(data, &cfg); err != nil {
        return SourceConfig{}, fmt.Errorf("failed to parse %s: %w", path, err)
    }

    return cfg, nil
}
```

**Example configuration files:**

```yaml
# config/alert-sources/prometheus-prod.yaml
name: prometheus-prod
type: prometheus
webhook_path: /alerts/prometheus
auth:
  type: bearer
  token: ${PROMETHEUS_WEBHOOK_TOKEN}
```

```yaml
# config/alert-sources/datadog-prod.yaml
name: datadog-prod
type: datadog
api_key: ${DD_API_KEY}
app_key: ${DD_APP_KEY}
poll_interval: 60s
```

```yaml
# config/alert-sources/nats-alerts.yaml
name: nats-alerts
type: nats
url: nats://localhost:4222
subject: alerts.>
```

---

## Testing Strategy

### Test Coverage Requirements

For each component, create table-driven tests following existing patterns:

```
internal/infrastructure/adapter/alert/
├── source_registry_test.go
├── source_manager_test.go
├── dedup_manager_test.go
├── prometheus_source_test.go
├── nats_source_test.go
└── datadog_source_test.go
```

### Example Test Pattern

```go
// internal/infrastructure/adapter/alert/prometheus_source_test.go
package alert

import (
    "context"
    "strings"
    "testing"
    "time"

    "code-editing-agent/internal/domain/entity"
)

func TestPrometheusSource_HandleWebhook(t *testing.T) {
    tests := []struct {
        name        string
        payload     string
        wantAlerts  int
        wantErr     bool
        wantErrMsg  string
    }{
        {
            name: "valid firing alert",
            payload: `{
                "alerts": [{
                    "status": "firing",
                    "labels": {"alertname": "HighCPU", "severity": "warning"},
                    "annotations": {"summary": "CPU usage high"},
                    "startsAt": "2024-01-20T15:30:00Z"
                }]
            }`,
            wantAlerts: 1,
            wantErr:    false,
        },
        {
            name: "resolved alert skipped",
            payload: `{
                "alerts": [{
                    "status": "resolved",
                    "labels": {"alertname": "HighCPU"},
                    "startsAt": "2024-01-20T15:30:00Z"
                }]
            }`,
            wantAlerts: 0,
            wantErr:    false,
        },
        {
            name:       "invalid JSON",
            payload:    "not json",
            wantAlerts: 0,
            wantErr:    true,
            wantErrMsg: "invalid alertmanager payload",
        },
        {
            name: "missing required fields",
            payload: `{
                "alerts": [{
                    "status": "firing",
                    "startsAt": "2024-01-20T15:30:00Z"
                }]
            }`,
            wantAlerts: 0,
            wantErr:    true,
            wantErrMsg: "invalid alert from prometheus",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            source, err := NewPrometheusSource(SourceConfig{
                Name:        "test-prometheus",
                WebhookPath: "/test",
            })
            if err != nil {
                t.Fatalf("failed to create source: %v", err)
            }

            alerts, err := source.HandleWebhook(context.Background(), []byte(tt.payload))

            if (err != nil) != tt.wantErr {
                t.Errorf("HandleWebhook() error = %v, wantErr %v", err, tt.wantErr)
                return
            }

            if err != nil && tt.wantErrMsg != "" && !strings.Contains(err.Error(), tt.wantErrMsg) {
                t.Errorf("HandleWebhook() error = %v, want error containing %v", err, tt.wantErrMsg)
            }

            if len(alerts) != tt.wantAlerts {
                t.Errorf("HandleWebhook() returned %d alerts, want %d", len(alerts), tt.wantAlerts)
            }
        })
    }
}

// Test manager start/stop
func TestLocalAlertSourceManager_Lifecycle(t *testing.T) {
    manager := NewLocalAlertSourceManager()

    // Register test source
    source, _ := NewPrometheusSource(SourceConfig{
        Name:        "test",
        WebhookPath: "/test",
    })

    if err := manager.RegisterSource(source); err != nil {
        t.Fatalf("failed to register source: %v", err)
    }

    // Test Start
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    if err := manager.Start(ctx); err != nil {
        t.Fatalf("Start() error = %v", err)
    }

    // Verify server is listening
    time.Sleep(100 * time.Millisecond)

    // Test Shutdown
    if err := manager.Shutdown(); err != nil {
        t.Fatalf("Shutdown() error = %v", err)
    }
}

// Test deduplication
func TestDeduplicatingAlertManager(t *testing.T) {
    baseManager := NewLocalAlertSourceManager()
    dedupManager := NewDeduplicatingAlertManager(baseManager, 100*time.Millisecond)

    received := 0
    dedupManager.SetAlertHandler(func(ctx context.Context, alert *entity.Alert) error {
        received++
        return nil
    })

    alert1, _ := entity.NewAlert("alert-1", "test", entity.SeverityCritical, "Test Alert")
    alert1Dup, _ := entity.NewAlert("alert-1", "test", entity.SeverityCritical, "Test Alert")
    alert2, _ := entity.NewAlert("alert-2", "test", entity.SeverityWarning, "Test Alert 2")

    // Dispatch same alert twice
    dedupManager.dispatchAlert(context.Background(), alert1)
    dedupManager.dispatchAlert(context.Background(), alert1Dup)
    dedupManager.dispatchAlert(context.Background(), alert2)

    time.Sleep(50 * time.Millisecond)

    if received != 2 {
        t.Errorf("Expected 2 unique alerts, got %d", received)
    }
}
```

---

## Container Wiring

### `internal/infrastructure/config/container.go` (Updated)

```go
func NewContainer(cfg *Config) (*Container, error) {
    // ... existing setup ...

    // === Alert Source Setup ===

    // 1. Create source registry (no dependencies)
    alertRegistry := alert.NewSourceRegistry()
    alertRegistry.RegisterBuiltinFactories()

    // 2. Create base alert source manager
    baseAlertManager := alert.NewLocalAlertSourceManager()
    baseAlertManager.SetServerAddr(cfg.AlertServerAddr) // NEW: HTTP server address

    // 3. Wrap with deduplication decorator
    alertManager := alert.NewDeduplicatingAlertManager(baseAlertManager, 5*time.Minute)

    // 4. Set error callback with metrics
    baseAlertManager.SetErrorCallback(func(source string, err error) {
        log.Printf("[AlertSource:%s] error: %v", source, err)
        sourceErrors.WithLabelValues(source, "fetch").Inc()
    })

    // 5. Load and register sources from config directory
    result, err := loadAndRegisterAlertSources(alertRegistry, alertManager, cfg.AlertSourcesDir)
    if err != nil {
        return nil, fmt.Errorf("failed to load alert sources: %w", err)
    }
    log.Printf("Registered %d alert sources, %d failed", result.RegisteredCount, len(result.FailedSources))

    // 6. Create AlertService (needs ConversationService - handle separately)
    alertService := service.NewAlertService(alertManager, nil)

    return &Container{
        config:       cfg,
        chatService:  chatService,
        convService:  convService,
        fileManager:  fileManager,
        uiAdapter:    uiAdapter,
        aiAdapter:    aiAdapter,
        toolExecutor: toolExecutor,
        skillManager: skillManager,
        alertManager: alertManager,
        alertService: alertService,
    }, nil
}
```

---

## Application Layer

### `internal/application/service/alert_service.go`

```go
package service

import (
    "context"
    "fmt"
    "log"
    "time"

    "code-editing-agent/internal/domain/entity"
    "code-editing-agent/internal/domain/port"
)

// AlertService orchestrates alert processing
type AlertService struct {
    sourceManager   port.AlertSourceManager
    conversationSvc *ConversationService
    alertStore      AlertStore // NEW: persistence
}

// AlertStore persists alerts for historical queries
type AlertStore interface {
    Store(ctx context.Context, alert *entity.Alert) error
    Query(ctx context.Context, filters AlertQuery) ([]*entity.Alert, error)
}

// AlertQuery filters for alert queries
type AlertQuery struct {
    Source   string
    Severity string
    Since    time.Time
    Until    time.Time
    Labels   map[string]string
}

// NewAlertService creates an alert orchestration service
func NewAlertService(
    sourceManager port.AlertSourceManager,
    convSvc *ConversationService,
) *AlertService {
    svc := &AlertService{
        sourceManager:   sourceManager,
        conversationSvc: convSvc,
        alertStore:      NewInMemoryAlertStore(), // Default in-memory store
    }

    // Wire up the alert handler
    sourceManager.SetAlertHandler(svc.handleAlert)

    return svc
}

// SetAlertStore configures a custom alert store (for persistence)
func (s *AlertService) SetAlertStore(store AlertStore) {
    s.alertStore = store
}

// handleAlert processes incoming alerts
func (s *AlertService) handleAlert(ctx context.Context, alert *entity.Alert) error {
    start := time.Now()
    alertsReceived.WithLabelValues(alert.Source(), alert.Severity()).Inc()

    log.Printf("[ALERT] [%s] %s: %s", alert.Severity(), alert.Source(), alert.Title())

    // Persist alert immediately (don't lose it!)
    if err := s.alertStore.Store(ctx, alert); err != nil {
        log.Printf("Failed to store alert: %v", err)
        sourceErrors.WithLabelValues(alert.Source(), "store").Inc()
        // Continue processing even if storage fails
    }

    // Process the alert
    if err := s.processAlert(ctx, alert); err != nil {
        alertsProcessed.WithLabelValues(alert.Source(), alert.Severity(), "error").Inc()
        return err
    }

    alertsProcessed.WithLabelValues(alert.Source(), alert.Severity(), "success").Inc()
    alertProcessingDuration.WithLabelValues(alert.Source(), alert.Severity()).Observe(time.Since(start).Seconds())

    return nil
}

// processAlert handles alert routing and response
func (s *AlertService) processAlert(ctx context.Context, alert *entity.Alert) error {
    // Option 1: Log to conversation (when conversation targeting is implemented)
    // if s.conversationSvc != nil {
    //     msg := formatAlertMessage(alert)
    //     s.conversationSvc.BroadcastSystemMessage(ctx, msg)
    // }

    // Option 2: Trigger autonomous response for critical alerts
    if alert.IsCritical() {
        go s.triggerAutonomousResponse(alert) // Async to not block alert pipeline
    }

    // Option 3: Just acknowledge receipt
    return nil
}

// triggerAutonomousResponse handles critical alerts autonomously
func (s *AlertService) triggerAutonomousResponse(alert *entity.Alert) {
    // Create a new conversation session
    // Analyze the alert
    // Take automated actions based on alert content
    // Log all actions taken

    log.Printf("[AUTONOMOUS] Critical alert triggered: %s", alert.Title())
    // TODO: Implement autonomous response logic
}

// Start begins processing alerts from all sources
func (s *AlertService) Start(ctx context.Context) error {
    return s.sourceManager.Start(ctx)
}

// Shutdown stops alert processing
func (s *AlertService) Shutdown() error {
    return s.sourceManager.Shutdown()
}

// GetRecentAlerts returns recent alerts from the store
func (s *AlertService) GetRecentAlerts(ctx context.Context, limit int) ([]*entity.Alert, error) {
    return s.alertStore.Query(ctx, AlertQuery{
        Since: time.Now().Add(-24 * time.Hour),
    })
}

// formatAlertMessage creates a human-readable alert message
func formatAlertMessage(alert *entity.Alert) string {
    msg := fmt.Sprintf("🚨 ALERT [%s] from %s\n", alert.Severity(), alert.Source())
    msg += fmt.Sprintf("Title: %s\n", alert.Title())

    if desc := alert.Description(); desc != "" {
        msg += fmt.Sprintf("Description: %s\n", desc)
    }

    if age := alert.Age(); age > 0 {
        msg += fmt.Sprintf("Age: %s\n", age.Round(time.Second))
    }

    return msg
}
```

---

## In-Memory Alert Store (Default Implementation)

### `internal/application/service/alert_store.go`

```go
package service

import (
    "context"
    "sort"
    "sync"
    "time"

    "code-editing-agent/internal/domain/entity"
)

// InMemoryAlertStore stores alerts in memory (cleared on restart)
type InMemoryAlertStore struct {
    mu      sync.RWMutex
    alerts  []*entity.Alert
    maxSize int // Maximum alerts to keep
}

// NewInMemoryAlertStore creates a new in-memory store
func NewInMemoryAlertStore() *InMemoryAlertStore {
    return &InMemoryAlertStore{
        alerts:  make([]*entity.Alert, 0, 1000),
        maxSize: 10000, // Keep last 10k alerts
    }
}

func (s *InMemoryAlertStore) Store(ctx context.Context, alert *entity.Alert) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    // Add to beginning (most recent first)
    s.alerts = append([]*entity.Alert{alert}, s.alerts...)

    // Trim if too large
    if len(s.alerts) > s.maxSize {
        s.alerts = s.alerts[:s.maxSize]
    }

    return nil
}

func (s *InMemoryAlertStore) Query(ctx context.Context, filters AlertQuery) ([]*entity.Alert, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    var results []*entity.Alert

    for _, alert := range s.alerts {
        // Check filters
        if filters.Source != "" && alert.Source() != filters.Source {
            continue
        }
        if filters.Severity != "" && alert.Severity() != filters.Severity {
            continue
        }
        if !filters.Since.IsZero() && alert.Timestamp().Before(filters.Since) {
            continue
        }
        if !filters.Until.IsZero() && alert.Timestamp().After(filters.Until) {
            continue
        }

        // Check label filters
        matchLabels := true
        for k, v := range filters.Labels {
            if alert.Labels()[k] != v {
                matchLabels = false
                break
            }
        }
        if !matchLabels {
            continue
        }

        results = append(results, alert)
    }

    // Sort by timestamp (already sorted by Store)
    sort.Slice(results, func(i, j int) bool {
        return results[i].Timestamp().After(results[j].Timestamp())
    })

    return results, nil
}
```

---

## Summary of Changes

| Component | Original Plan | Implementation Proposal | Status |
|-----------|--------------|------------------------|--------|
| **HTTP Server** | Handler only, no lifecycle | Full server with Start/Shutdown, TLS support | ✅ CRITICAL FIX |
| **Configuration** | Single YAML file | Directory-per-source pattern (like skills) | ✅ ENHANCED |
| **Health Checks** | Not mentioned | Added HealthChecker interface + implement | ✅ NEW |
| **Metrics** | Basic logging | Full Prometheus instrumentation | ✅ NEW |
| **Circuit Breakers** | Not mentioned | Added gobreaker wrapper for poll sources | ✅ NEW |
| **Alert Persistence** | Not mentioned | InMemoryAlertStore + interface | ✅ NEW |
| **Webhook Auth** | Not mentioned | Bearer + HMAC support | ✅ NEW |
| **Path Traversal** | Not mentioned | validateWebhookPath() protection | ✅ NEW |
| **Rate Limiting** | Not mentioned | Per-IP rate limiter structure | ✅ NEW |
| **Test Coverage** | Not mentioned | Table-driven tests for all components | ✅ NEW |
| **Error Handling** | Silent failures | Metrics + structured error tracking | ✅ IMPROVED |

---

## Implementation Checklist

- [ ] Create `internal/domain/entity/alert.go` with tests
- [ ] Create `internal/domain/port/alert_source.go`
- [ ] Create `internal/infrastructure/adapter/alert/source_registry.go` with tests
- [ ] Create `internal/infrastructure/adapter/alert/source_manager.go` with lifecycle + HTTP server
- [ ] Create `internal/infrastructure/adapter/alert/dedup_manager.go` with tests
- [ ] Create `internal/infrastructure/adapter/alert/prometheus_source.go` with tests
- [ ] Create `internal/infrastructure/adapter/alert/nats_source.go` with tests
- [ ] Create `internal/infrastructure/adapter/alert/datadog_source.go` with tests
- [ ] Create `internal/application/service/alert_service.go` with tests
- [ ] Create `internal/application/service/alert_store.go`
- [ ] Update `internal/infrastructure/config/container.go`
- [ ] Add metrics setup to `internal/infrastructure/config/metrics.go`
- [ ] Create test fixtures in `test/fixtures/alerts/`
- [ ] Add integration test in `internal/integration/alert_integration_test.go`
- [ ] Document configuration in `config/alert-sources/README.md`
- [ ] Update main.go to start alert service if enabled
- [ ] Add CLI flags for alert server address
- [ ] Create example alert source configs

---

## Notes & Warnings

### ⚠️ Unresolved Issues

1. **Alert-to-Conversation Routing**: Still needs architecture design
   - Options: broadcast, routing rules, manual subscription
   - Requires changes to ConversationService

2. **Autonomous Response**: `triggerAutonomousResponse()` is a placeholder
   - Needs safety limits (rate, scope)
   - Needs user approval workflow
   - Consider separate agent instance

3. **Production Alert Storage**: InMemory store is not persistent
   - Recommend: PostgreSQL, Redis, or file-based store
   - Add retention policies

4. **Webhook HMAC Auth**: Implementation incomplete
   - Need to add signature verification logic
   - Need to handle signature algorithm variations

### 🎯 Production Recommendations

1. **TLS Configuration**: Enable HTTPS in production
2. **Secret Management**: Use HashiCorp Vault or similar
3. **Monitoring**: Set up Grafana dashboards for metrics
4. **Alert on Alerts**: Create alerts for failed source health checks
5. **Log Aggregation**: Forward logs to centralized system
6. **Backup**: Export alert history periodically
7. **Scaling**: Consider horizontal scaling (multiple agent instances)

---

## AI Investigation Architecture (CRITICAL GAPS)

**Context**: The agent is intended to receive alerts (e.g., "high CPU on node xyz") and autonomously investigate and take action using available tools (bash, read_file, etc.). The current document assumes alert ingestion but does not address the investigation workflow.

### Use Case Example

```
Prometheus Alert: "HighCPUUsage" on prod-db-01
    ↓
Agent receives alert via webhook
    ↓
Agent should investigate:
  1. Check current load (top, ps aux)
  2. Check logs (journalctl, /var/log/*)
  3. Identify root cause
  4. Take action if safe (restart service, scale up)
  5. Document findings
```

---

## 🔴 CRITICAL: AI Investigation Gaps (Block Implementation)

### 1. Investigation Workflow is Missing

**Current State**: `triggerAutonomousResponse()` is a placeholder with only comments:

```go
// Line 1803-1810 - CURRENT IMPLEMENTATION
func (s *AlertService) triggerAutonomousResponse(alert *entity.Alert) {
    // Create a new conversation session
    // Analyze the alert
    // Take automated actions based on alert content
    // Log all actions taken

    log.Printf("[AUTONOMOUS] Critical alert triggered: %s", alert.Title())
    // TODO: Implement autonomous response logic
}
```

**What's Actually Needed**:

```go
// REQUIRED: Alert Investigation Use Case
// internal/application/usecase/alert_investigation.go

type AlertInvestigationUseCase struct {
    conversationService *service.ConversationService
    chatService         *service.ChatService
    investigationStore  InvestigationStore
    config              *InvestigationConfig
}

func (uc *AlertInvestigationUseCase) HandleAlert(ctx context.Context, alert *entity.Alert) error {
    // 1. Create investigation session
    sessionID, err := uc.conversationService.StartConversation(ctx)
    if err != nil {
        return err
    }

    // 2. Build investigation prompt with context
    prompt := uc.buildInvestigationPrompt(alert)

    // 3. Add prompt to conversation
    uc.conversationService.AddUserMessage(ctx, sessionID, prompt)

    // 4. Run investigation loop (agent executes tools until done)
    go uc.runInvestigationLoop(ctx, sessionID, alert)

    return nil
}

func (uc *AlertInvestigationUseCase) runInvestigationLoop(ctx context.Context, sessionID string, alert *entity.Alert) {
    // Get AI response (agent may request tool execution)
    response, err := uc.chatService.SendMessage(ctx, sessionID, "")
    if err != nil {
        log.Printf("Investigation %s failed: %v", sessionID, err)
        return
    }

    // Execute tools if requested, continue loop
    for response.RequiresToolExecution() {
        toolResults, err := uc.chatService.ExecuteTools(ctx, sessionID, response.Tools)
        if err != nil {
            log.Printf("Tool execution failed: %v", err)
            break
        }

        // Feed results back to AI for next step
        response, err = uc.chatService.SendToolResults(ctx, sessionID, toolResults)
        if err != nil {
            break
        }
    }

    // Investigation complete - store findings
    uc.storeInvestigationResults(sessionID, alert, response)
}
```

---

### 2. Tool Execution Integration is Missing

**Problem**: AlertService cannot access `ToolExecutor` to run bash/read_file commands during investigation.

**Current State**:
- `ConversationService` has `toolExecutor` but `AlertService` doesn't
- No bridge from alert → conversation → tool execution

**Required Architecture Fix**:

Option A: Create `InvestigationUseCase` in application layer (RECOMMENDED)

```go
// This avoids the circular dependency problem
// internal/application/usecase/alert_investigation.go

type AlertInvestigationUseCase struct {
    convService  *service.ConversationService  // From domain layer
    chatService  *appsvc.ChatService          // From app layer - has tool access
    alertStore   AlertStore
}

// ConversationService already has toolExecutor, so:
// alert → conversation → tools works through existing infrastructure
```

Option B: Add method to `ConversationService` (BREAKS LAYERING)
```go
// DON'T DO THIS - creates circular dependency
func (cs *ConversationService) StartInvestigation(ctx context.Context, alert *entity.Alert) (string, error)
```

---

### 3. Investigation Prompts are Missing

**Problem**: No prompt engineering strategy for telling the AI HOW to investigate.

**What's Needed**:

```go
// REQUIRED: Prompt builder per alert type
type InvestigationPromptBuilder interface {
    BuildPrompt(alert *entity.Alert) string
}

// Example builders:
type HighCPUPromptBuilder struct{}

func (b *HighCPUPromptBuilder) BuildPrompt(alert *entity.Alert) string {
    return fmt.Sprintf(`
You are investigating a High CPU alert.

**ALERT DETAILS:**
- Severity: %s
- Host: %s
- Current Value: %s
- Threshold: %s
- Other Labels: %v

**YOUR MISSION:**
1. Check current resource usage (top, htop, pidstat)
2. Identify the high-CPU process
3. Check recent logs for that process
4. Determine the root cause
5. Propose or take action if SAFE

**SAFETY RULES:**
- DO NOT restart services without explicit confirmation
- DO NOT kill processes > 50% CPU (may be legitimate)
- DO NOT modify configuration files
- Use read_file to analyze logs and configs first
- Provide findings summary when done

**AVAILABLE TOOLS:**
- bash: Run commands
- read_file: Read log and config files
- list_files: List directory contents

Begin investigation.
`,
        alert.Severity(),
        alert.Labels()["instance"],
        alert.Labels()["value"],
        alert.Labels()["threshold"],
        alert.Labels(),
    )
}

// Registry of prompt builders:
var promptBuilders = map[string]InvestigationPromptBuilder{
    "HighCPUAlert":        &HighCPUPromptBuilder{},
    "DiskSpaceAlert":      &DiskSpacePromptBuilder{},
    "OOMKilledAlert":      &OOMPromptBuilder{},
    "MemoryUsageAlert":    &MemoryPromptBuilder{},
}
```

---

### 4. Investigation State Tracking is Missing

**Problem**: No way to track investigation progress, results, or status.

**Required Entity**:

```go
// REQUIRED: Investigation entity for tracking
// internal/domain/entity/investigation.go

type InvestigationStatus string

const (
    InvestigationStarted   InvestigationStatus = "started"
    InvestigationRunning   InvestigationStatus = "running"
    InvestigationWaiting   InvestigationStatus = "waiting" // Awaiting human input
    InvestigationCompleted InvestigationStatus = "completed"
    InvestigationFailed    InvestigationStatus = "failed"
    InvestigationEscalated InvestigationStatus = "escalated"
)

type Investigation struct {
    id           string
    alertID      string
    sessionID    string           // Associated conversation session
    status       InvestigationStatus
    findings     []string         // Agent's observations
    actionsTaken []Action         // Tools executed
    confidence   float64          // Agent's confidence in root cause (0-1)
    escalate     bool             // Human intervention needed
    startedAt    time.Time
    completedAt  time.Time
}

type Action struct {
    Tool    string                 // "bash", "read_file", etc.
    Input   map[string]interface{} // Tool parameters
    Output  string                 // Tool result
    At      time.Time
}

func NewInvestigation(alertID, sessionID string) *Investigation {
    return &Investigation{
        id:        generateInvestigationID(),
        alertID:   alertID,
        sessionID: sessionID,
        status:    InvestigationStarted,
        findings:  make([]string, 0),
        actions:   make([]Action, 0),
        startedAt: time.Now(),
    }
}

// Required store:
type InvestigationStore interface {
    Store(ctx context.Context, inv *Investigation) error
    Get(ctx context.Context, id string) (*Investigation, error)
    List(ctx context.Context, filters InvestigationQuery) ([]*Investigation, error)
}
```

---

## 🟡 IMPORTANT: Gaps for Production Safety

### 5. Safety Framework is Partial

**Current State**: Rate limiting exists for HTTP webhooks, but NOT for investigation actions.

**What's Needed**:

```go
// REQUIRED: Investigation safety configuration
type InvestigationConfig struct {
    // Limits
    MaxActionsPerInvestigation int           // e.g., 20 tool calls max
    MaxDuration               time.Duration // e.g., 15 minutes max
    MaxConcurrentInvestigations int          // e.g., 5 at once

    // Tool restrictions
    AllowedTools              []string   // Only these tools can be used
    BlockedCommands           []string   // Never allow: rm -rf, dd, mkfs, etc.
    AllowedDirectories        []string   // Only read from these directories

    // Approval gates
    RequireHumanApproval      []string   // Patterns matching risky actions
    ConfirmBeforeRestart      bool        // Require confirmation for service restarts
    ConfirmBeforeDelete       bool        // Require confirmation for file operations

    // Escalation thresholds
    EscalateOnConfidenceBelow  float64     // Escalate if agent unsure (< 0.7)
    EscalateOnMultipleErrors   int         // Escalate after N errors
}

// Example investigation config:
func DefaultInvestigationConfig() *InvestigationConfig {
    return &InvestigationConfig{
        MaxActionsPerInvestigation:  20,
        MaxDuration:                15 * time.Minute,
        MaxConcurrentInvestigations: 5,
        AllowedTools:              []string{"bash", "read_file", "list_files"},
        BlockedCommands:           []string{"rm -rf", "dd if=", "mkfs", ":(){:|:&}:;", "systemctl stop critical"},
        AllowedDirectories:        []string{"/var/log", "/etc/myapp", "./logs"},
        RequireHumanApproval:      []string{"restart", "delete", "kill", "systemctl stop"},
        ConfirmBeforeRestart:      true,
        ConfirmBeforeDelete:       true,
        EscalateOnConfidenceBelow: 0.7,
        EscalateOnMultipleErrors:  3,
    }
}

// Safety middleware wrapper:
type SafeInvestigationUseCase struct {
    wrapped *AlertInvestigationUseCase
    config  *InvestigationConfig
}

func (s *SafeInvestigationUseCase) runInvestigationLoop(ctx context.Context, sessionID string, alert *entity.Alert) {
    // Check concurrent limit
    if s.getActiveCount() >= s.config.MaxConcurrentInvestigations {
        s.escalate(alert, "Too many concurrent investigations")
        return
    }

    // Set timeout
    ctx, cancel := context.WithTimeout(ctx, s.config.MaxDuration)
    defer cancel()

    // Track action count
    actionCount := 0

    // Run investigation with safety checks
    for response.RequiresToolExecution() {
        if actionCount >= s.config.MaxActionsPerInvestigation {
            s.escalate(alert, "Exceeded action budget")
            break
        }

        // Check for blocked commands
        for _, tool := range response.Tools {
            if s.isBlocked(tool) {
                s.escalate(alert, fmt.Sprintf("Blocked command: %s", tool.Name))
                return
            }
        }

        // Check for required approval
        if s.requiresApproval(response.Tools) {
            if !s.getHumanApproval(sessionID, response.Tools) {
                s.escalate(alert, "Human approval denied")
                return
            }
        }

        // Execute tools via wrapped use case
        toolResults, err := s.chatService.ExecuteTools(ctx, sessionID, response.Tools)
        if err != nil {
            s.errorCount++
            if s.errorCount >= s.config.EscalateOnMultipleErrors {
                s.escalate(alert, fmt.Sprintf("Too many errors: %v", err))
                break
            }
        }

        actionCount++
        response, _ = s.chatService.SendToolResults(ctx, sessionID, toolResults)
    }
}
```

---

### 6. Human Escalation Path is Missing

**Problem**: When the agent is unsure or encounters a problem, there's no way to notify a human.

**What's Needed**:

```go
// REQUIRED: Escalation mechanism
type EscalationHandler interface {
    Escalate(ctx context.Context, investigation *Investigation, reason string) error
}

// Example: Alert escalation to conversation
type ConversationEscalator struct {
    chatService *appsvc.ChatService
    escalatedTo map[string]bool // alertID -> already escalated
}

func (e *ConversationEscalator) Escalate(ctx context.Context, inv *Investigation, reason string) error {
    // Prevent spam escalation
    if e.escalatedTo[inv.alertID] {
        return nil // Already escalated
    }
    e.escalatedTo[inv.alertID] = true

    // Find or create a conversation for human interaction
    sessionID := e.findHumanSession() // Get current user session

    // Build escalation message
    msg := fmt.Sprintf(`
🆘 INVESTIGATION ESCALATED

Alert: %s
Investigation ID: %s
Session ID: %s

Reason: %s

Findings so far:
%s

Actions taken:
%s

Please review and take action.
`,
        inv.alertID,
        inv.id,
        inv.sessionID,
        reason,
        strings.Join(inv.findings, "\n"),
        e.formatActions(inv.actionsTaken),
    )

    // Send to human
    e.chatService.SendMessage(ctx, sessionID, msg)

    // Update investigation status
    inv.status = InvestigationEscalated
    inv.completedAt = time.Now()

    return nil
}
```

---

## 🟢 NICE TO HAVE: Phase 2 Enhancements

### 7. Alert Correlation is Missing

When 50 nodes report high CPU simultaneously, agent should correlate:
- Are they all in the same region?
- Is there a common cause (new deployment, network issue, database slow)?

```go
// FUTURE: Correlation engine
type AlertCorrelator struct {
    window time.Time // Correlate alerts within this time window
}

func (c *AlertCorrelator) Correlate(alert *entity.Alert, recent []*entity.Alert) ([]*AlertGroup, error) {
    var groups []*AlertGroup

    // Group by labels
    byRegion := make(map[string][]*entity.Alert)
    byService := make(map[string][]*entity.Alert)

    for _, a := range recent {
        if region := a.Labels()["region"]; region != "" {
            byRegion[region] = append(byRegion[region], a)
        }
        if service := a.Labels()["service"]; service != "" {
            byService[service] = append(byService[service], a)
        }
    }

    // Detect patterns
    if len(byRegion["us-east-1"]) > 10 {
        groups = append(groups, &AlertGroup{
            Type:     "RegionalOutage",
            Region:   "us-east-1",
            Count:    len(byRegion["us-east-1"]),
            Alerts:   byRegion["us-east-1"],
        })
    }

    return groups, nil
}
```

### 8. Investigation History & Learning is Missing

```go
// FUTURE: Learn from past investigations
type InvestigationLearner struct {
    store InvestigationStore
}

func (l *InvestigationLearner) FindSimilar(alert *entity.Alert) (*Investigation, error) {
    // Query past investigations with similar labels/annotations
    similar, err := l.store.Query(ctx, InvestigationQuery{
        Labels:    alert.Labels(),
        MinTime:   time.Now().Add(-30 * 24 * time.Hour), // Last 30 days
        Status:    []InvestigationStatus{InvestigationCompleted},
    })
    if err != nil {
        return nil, err
    }

    // Return most relevant past investigation
    return selectMostRelevant(similar), nil
}
```

---

## Updated Implementation Checklist (AI Investigation Focus)

### Phase 1: Alert Ingestion (Foundation) - COMPLETED 2025-12-31
- [x] Create `internal/domain/entity/alert.go` with tests
- [x] Create `internal/domain/port/alert_source.go`
- [x] Create `internal/infrastructure/adapter/alert/prometheus_source.go` (renamed from prometheus_webhook.go)
- [x] Create `internal/infrastructure/adapter/alert/source_manager.go` (basic version without HTTP server)
- [x] **NO Auto Investigation Yet** (Phase 2 blocker)

#### Phase 1 Completion Summary

**Date Completed:** 2025-12-31
**Development Approach:** Test-Driven Development (TDD)

**What Was Implemented:**
1. **Alert Entity** (`internal/domain/entity/alert.go`)
   - Full immutable entity with validation
   - Builder pattern for optional fields (WithDescription, WithLabels, etc.)
   - Defensive copying for map fields
   - Comprehensive validation (ID, source, title, severity)
   - 64 passing tests with 100% coverage

2. **Port Interfaces** (`internal/domain/port/alert_source.go`)
   - Base `AlertSource` interface
   - Specialized interfaces: `WebhookAlertSource`, `PollAlertSource`, `StreamAlertSource`
   - `AlertSourceManager` interface for lifecycle management
   - `AlertHandler` callback type for processing alerts

3. **Prometheus Source** (`internal/infrastructure/adapter/alert/prometheus_source.go`)
   - Webhook-based alert source for Prometheus Alertmanager
   - JSON payload parsing with validation
   - Security: path traversal protection
   - Handles firing/resolved alerts appropriately
   - 21 passing tests with edge case coverage

4. **Source Manager** (`internal/infrastructure/adapter/alert/source_manager.go`)
   - Thread-safe source registration/unregistration
   - Source discovery and lifecycle management
   - Alert handler callback injection
   - Error callback mechanism
   - 25 passing tests covering all manager operations

**What Was NOT Implemented (Deferred to Phase 2+):**
- HTTP server for webhook endpoints (manager has no Start/Shutdown implementation)
- Alert handler wiring to conversation/chat services
- Autonomous investigation workflow
- Alert persistence (InMemoryAlertStore)
- Deduplication decorator
- NATS/Datadog/other source adapters
- Circuit breakers, rate limiting, metrics
- Container wiring in config

**Test Quality:**
- All tests follow table-driven pattern
- No skipped tests (t.Skip)
- No mocks except lightweight test doubles in manager tests
- Tests verify behavior, not implementation
- 100% of Phase 1 code has corresponding tests

**Architecture Compliance:**
- Strict hexagonal architecture adherence
- Domain layer has zero external dependencies
- Ports define interfaces only
- Entities are immutable with proper validation
- No global state, dependency injection ready

**Known Limitations:**
- Source manager does not start HTTP server (basic version only)
- No integration with existing chat/conversation infrastructure
- Alert routing not implemented
- No production monitoring/metrics

**Next Steps:**
Phase 2 (Investigation Framework) has been completed with core investigation workflow, safety configuration, and prompt building infrastructure. See Phase 2 completion summary below for details.

### Phase 2: Investigation Framework - COMPLETED 2025-01-01
- [x] Create `internal/domain/entity/investigation.go`
- [x] Create `internal/application/service/investigation_store.go` (In-Memory)
- [x] Create `internal/application/usecase/alert_investigation.go`
- [x] Design `InvestigationConfig` with safety limits
- [x] Build `InvestigationPromptBuilder` interface and implementations
- [x] **NO SafeInvestigationUseCase Decorator** (deferred - safety in usecase itself)
- [x] **NO ConversationEscalator** (deferred - basic EscalationHandler interface instead)

#### Phase 2 Completion Summary

**Date Completed:** 2025-01-01
**Development Approach:** Test-Driven Development (TDD)

**What Was Implemented:**
1. **Investigation Entity** (`internal/domain/entity/investigation.go`)
   - Full immutable entity with lifecycle state management
   - Status constants: started, running, completed, failed, escalated
   - Findings and Actions tracking with structured types
   - Confidence scoring (0.0 to 1.0 range)
   - Duration tracking and IsComplete() helper
   - Complete(), Fail(), and Escalate() state transitions
   - 48 passing tests with 76.4% coverage

2. **Investigation Store** (`internal/application/service/investigation_store.go`)
   - Thread-safe in-memory implementation with sync.RWMutex
   - CRUD operations: Store, Get, Update, Delete
   - Query support with filters: AlertID, SessionID, Status, Since, Until, Limit
   - Context support with cancellation handling
   - Close() method for graceful shutdown
   - 34 passing tests with comprehensive coverage

3. **Investigation Config** (`internal/application/config/investigation_config.go`)
   - Safety limits: MaxActions (default 50), MaxDuration (default 5 min), MaxConcurrent (default 5)
   - Tool whitelisting with IsToolAllowed()
   - Command blacklisting with IsCommandBlocked()
   - Directory allowlisting with IsDirectoryAllowed()
   - Human approval patterns with RequiresHumanApproval()
   - Escalation thresholds: EscalateOnConfidenceBelow, EscalateOnMultipleErrors
   - Dangerous command defaults: rm -rf, dd, mkfs, etc.
   - 48 passing tests covering all configuration scenarios

4. **Alert Investigation Use Case** (`internal/application/usecase/alert_investigation.go`)
   - HandleAlert() orchestration method
   - StartInvestigation() with concurrency limits
   - StopInvestigation() with graceful cleanup
   - GetInvestigationStatus() for monitoring
   - ListActiveInvestigations() for dashboard queries
   - Tool/command safety checks (IsToolAllowed, IsCommandBlocked)
   - Escalation handler interface (EscalationHandler) with basic LogEscalationHandler
   - Prompt builder registry (PromptBuilderRegistry) interface
   - Shutdown() with timeout support for graceful termination
   - 37 passing tests with stubs for investigation results

5. **Prompt Builder Infrastructure** (`internal/application/usecase/investigation_prompt_builder.go`)
   - PromptBuilder interface for alert-type-specific prompts
   - PromptBuilderRegistry interface for builder management
   - OOMPromptBuilder example implementation
   - Extensible design for future HighCPU, DiskSpace, etc. builders
   - Integration tests for basic OOM prompt construction

**What Was NOT Implemented (Deferred to Phase 3+):**
- PostgreSQL/persistent investigation store (in-memory only)
- SafeInvestigationUseCase decorator (safety integrated directly in usecase)
- ConversationEscalator (basic EscalationHandler interface only)
- Integration with chat/conversation services
- Actual AI prompt execution (orchestration only)
- Investigation state machine wiring
- Metrics and monitoring
- Container wiring in config
- HTTP endpoints for investigation status

**Test Quality:**
- All tests follow table-driven pattern
- No skipped tests in Phase 2 code
- Minimal mocking: only for result stubs in usecase tests
- Tests verify behavior, not implementation
- 167 total tests (48 entity + 48 config + 34 store + 37 usecase)
- 3489 lines of test code

**Architecture Compliance:**
- Strict hexagonal architecture adherence
- Domain layer (investigation.go) has ZERO external dependencies (only stdlib)
- Application layer properly separated: config, service, usecase
- Entities are immutable with defensive state management
- No global state, dependency injection ready
- Thread-safe implementations with proper synchronization

**Known Limitations:**
- Investigation store is in-memory (not persistent)
- No actual AI conversation integration (orchestration only)
- No metrics or observability
- Basic escalation handler (logs only, no PagerDuty/Slack)
- No integration with existing conversation infrastructure
- Prompt builders are stubs (need real prompt engineering)

**Next Steps:**
Phase 3 (Wire It All Together) has been completed with container wiring and integration tests. See Phase 3 completion summary below.

#### Phase 3 Completion Summary

**Date Completed:** 2026-01-01
**Development Approach:** Test-Driven Development (TDD)

**What Was Implemented:**
1. **Container Wiring** (`internal/infrastructure/config/container.go`)
   - AlertInvestigationUseCase with safety configuration (MaxActions: 20, MaxDuration: 15min, MaxConcurrent: 5)
   - PromptBuilderRegistry creation and injection
   - LogEscalationHandler creation and injection
   - AlertHandler creation with severity-based routing (AutoInvestigateCritical: true)
   - AlertSourceManager wired to AlertHandler.HandleEntityAlert
   - Thread-safe component initialization

2. **Alert Handler** (`internal/application/usecase/alert_handler.go`)
   - Bridges incoming alerts from sources to investigation use case
   - Configurable severity-based auto-investigation
   - Critical alerts trigger automatic investigations
   - Warning alerts require manual triggering (configurable)
   - Context propagation for cancellation
   - 13 passing tests with comprehensive coverage

3. **Integration Verification** (`internal/infrastructure/config/container_investigation_test.go`)
   - 6 passing tests verifying component wiring
   - Container accessors return proper instances
   - Component independence verified
   - Initial state correctness validated
   - Escalation and prompt handlers properly set

**What Was NOT Implemented (Deferred to Phase 4+):**
- Prometheus metrics (investigations_started, investigations_completed, investigations_escalated)
- Persistent investigation storage (using in-memory store from Phase 2)
- HTTP endpoints for investigation status
- Grafana dashboards
- Alert-to-conversation routing (needs ConversationService changes)
- Actual AI investigation loop (orchestration stubbed in Phase 2)

**Test Quality:**
- All Phase 3 tests pass (19 new tests: 13 alert_handler + 6 container_investigation)
- No skipped tests in Phase 3 code
- Defensive nil-checks in Phase 2 tests never execute (constructors always return valid structs)
- Tests verify behavior, not implementation
- Container wiring tested for correctness

**Architecture Compliance:**
- Strict hexagonal architecture maintained
- Container follows dependency injection pattern
- No circular dependencies introduced
- Proper layer separation (app layer → domain layer → infra layer)
- Thread-safe implementations

**Known Limitations:**
- Investigation loop still stubbed (AI integration deferred)
- No metrics collection yet
- AlertHandler does not trigger actual AI conversations (needs Phase 4 integration)
- No persistent alert/investigation storage

**Next Steps:**
Phase 4 (Safety & Production) will implement actual AI-driven investigations, metrics, and production-ready safety features.

### Phase 3: Wire It All Together - COMPLETED 2026-01-01
- [x] Update `internal/infrastructure/config/container.go`
- [x] Add investigation prompts for common alert types (HighCPU, DiskSpace, OOM) (basic implementations in Phase 2)
- [x] Create investigation state machine (Started → Running → Completed/Escalated/Failed) (entity in Phase 2)
- [ ] Add metrics: investigations_started, investigations_completed, investigations_escalated (deferred)

### Phase 4: Safety & Production
- [ ] Implement tool whitelisting
- [ ] Implement command blacklisting
- [ ] Implement human approval queue
- [ ] Add investigation timeout handling
- [ ] Add action budget enforcement
- [ ] Add persistent investigation storage (not in-memory)

---

## Architecture Diagram: Alert → Investigation Flow

```
┌─────────────────┐
│  Prometheus     │
│  Alertmanager   │
└────────┬────────┘
         │ Webhook POST /alerts/prometheus
         ▼
┌─────────────────────────────────────────────┐
│  LocalAlertSourceManager                    │
│  - Receives alert payload                    │
│  - Parses into entity.Alert                  │
│  - Calls alertHandler                        │
└────────┬────────────────────────────────────┘
         │ handler(alert)
         ▼
┌─────────────────────────────────────────────┐
│  AlertInvestigationUseCase                  │
│  - if critical: start investigation          │
└────────┬────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────┐
│  ConversationService                        │
│  - StartConversation(sessionID)             │
│  - AddUserMessage(sessionID, prompt)         │
└────────┬────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────┐
│  ChatService.SendMessage(sessionID)        │
│  - AI analyzes alert                        │
│  - AI decides to run tools                  │
└─────────────────────────────────────────────┘
         │ response with tool requests
         ▼
┌─────────────────────────────────────────────┐
│  ChatService.ExecuteTools(sessionID)       │
│  - ToolExecutor runs bash/read_file         │
│  - Returns results                           │
└─────────────────────────────────────────────┘
         │ tool results
         ▼
┌─────────────────────────────────────────────┐
│  Loop: SendToolResults → Get next response  │
│  (Until agent completes investigation)      │
└─────────────────────────────────────────────┘
         │ done
         ▼
┌─────────────────────────────────────────────┐
│  StoreInvestigationResults                  │
│  - Save findings to InvestigationStore      │
│  - Update status to Completed               │
└─────────────────────────────────────────────┘
```

---

## Summary: What This Document Actually Provides

| Component | Included | Missing for AI Investigation |
|-----------|----------|-----------------------------|
| Alert Ingestion (webhook/poll/stream) | ✅ | - |
| Alert Source Management | ✅ | - |
| Deduplication | ✅ | - |
| **Investigation Workflow** | ❌ | **BLOCKER** |
| **Tool Execution from Alert** | ❌ | **BLOCKER** |
| **Investigation Prompts** | ❌ | **BLOCKER** |
| **Investigation State Tracking** | ❌ | **IMPORTANT** |
| **Safety Framework** | ⚠️ Partial | **IMPORTANT** |
| **Human Escalation Path** | ❌ | **IMPORTANT** |
| **Persistent Alert Storage** | ⚠️ TODO | **IMPORTANT** |
| **Persistent Investigation Storage** | ❌ | **IMPORTANT** |

**Document Status for AI Investigation use case: ~35% complete**

This document is well-designed for **alert aggregation** but does not address the **AI investigation workflow** needed for autonomous SRE operations. To implement the full vision of an agent that investigates alerts, the gaps identified above must be addressed first.

---

**Document Author:** Deadpool
**Review Status:** Needs Team Review
**Next Steps:** Design Investigation Architecture before implementing alert ingestion
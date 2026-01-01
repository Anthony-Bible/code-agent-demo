# Alert Source Plugin Architecture

> Design document for making the agent extensible to receive alerts from various sources (Prometheus, NATS, Google Alerts, Datadog, etc.)

## Overview

The agent needs to support multiple alert consumption patterns:

| Pattern | Description | Examples |
|---------|-------------|----------|
| **Push (Webhook)** | External systems call our HTTP endpoint | Prometheus Alertmanager, PagerDuty, custom webhooks |
| **Pull (Polling)** | We periodically fetch from an API | Datadog API, Cloud Monitoring, REST endpoints |
| **Stream (Subscribe)** | We connect to message brokers | NATS, Kafka, Redis Streams, RabbitMQ, SQS |

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

    ctx    context.Context
    cancel context.CancelFunc
    wg     sync.WaitGroup
    mu     sync.RWMutex

    // Callbacks (like ExecutorAdapter)
    errorCallback func(source string, err error)
}

// NewLocalAlertSourceManager creates a new manager
func NewLocalAlertSourceManager() *LocalAlertSourceManager {
    return &LocalAlertSourceManager{
        sources: make(map[string]port.AlertSource),
        httpMux: http.NewServeMux(),
    }
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

// Shutdown stops all consumers and closes sources
func (m *LocalAlertSourceManager) Shutdown() error {
    m.mu.Lock()
    defer m.mu.Unlock()

    if m.cancel != nil {
        m.cancel()
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

    // Wrap the handler to add deduplication
    wrapped.SetAlertHandler(func(ctx context.Context, alert *entity.Alert) error {
        if d.isDuplicate(alert.ID()) {
            return nil // Skip duplicate
        }
        d.markSeen(alert.ID())
        return nil // The actual handler is set later
    })

    return d
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
    alerts := make(chan *entity.Alert, 100)
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
```

### Datadog Poll Source

### `internal/infrastructure/adapter/alert/datadog_source.go`

```go
package alert

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"

    "code-editing-agent/internal/domain/entity"
    "code-editing-agent/internal/domain/port"
)

type DatadogSource struct {
    name         string
    apiKey       string
    appKey       string
    pollInterval time.Duration
    client       *http.Client
}

// NewDatadogSource creates a Datadog polling source
func NewDatadogSource(cfg SourceConfig) (port.AlertSource, error) {
    if cfg.Name == "" {
        return nil, fmt.Errorf("datadog source requires name")
    }
    if cfg.APIKey == "" {
        return nil, fmt.Errorf("datadog source requires api_key")
    }
    if cfg.AppKey == "" {
        return nil, fmt.Errorf("datadog source requires app_key")
    }

    interval := 60 * time.Second
    if cfg.PollInterval != "" {
        if d, err := time.ParseDuration(cfg.PollInterval); err == nil {
            interval = d
        }
    }

    return &DatadogSource{
        name:         cfg.Name,
        apiKey:       cfg.APIKey,
        appKey:       cfg.AppKey,
        pollInterval: interval,
        client:       &http.Client{Timeout: 30 * time.Second},
    }, nil
}

func (d *DatadogSource) Name() string              { return d.name }
func (d *DatadogSource) Type() port.SourceType     { return port.SourceTypePoll }
func (d *DatadogSource) PollInterval() time.Duration { return d.pollInterval }
func (d *DatadogSource) Close() error              { return nil }

func (d *DatadogSource) FetchAlerts(ctx context.Context) ([]*entity.Alert, error) {
    req, err := http.NewRequestWithContext(ctx, "GET", "https://api.datadoghq.com/api/v1/monitor", nil)
    if err != nil {
        return nil, err
    }

    req.Header.Set("DD-API-KEY", d.apiKey)
    req.Header.Set("DD-APPLICATION-KEY", d.appKey)

    resp, err := d.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("datadog API request failed: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("datadog API error: status %d", resp.StatusCode)
    }

    var monitors []struct {
        ID      int    `json:"id"`
        Name    string `json:"name"`
        Message string `json:"message"`
        State   struct {
            OverallState string `json:"overall_state"`
        } `json:"overall_state"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&monitors); err != nil {
        return nil, fmt.Errorf("failed to decode datadog response: %w", err)
    }

    var alerts []*entity.Alert
    for _, m := range monitors {
        if m.State.OverallState != "Alert" {
            continue
        }

        alertID := fmt.Sprintf("dd-%d", m.ID)

        // Use entity factory for validation
        alert, err := entity.NewAlert(alertID, d.name, entity.SeverityWarning, m.Name)
        if err != nil {
            continue // Skip invalid alerts
        }

        alert.WithDescription(m.Message)
        alerts = append(alerts, alert)
    }

    return alerts, nil
}
```

---

## Application Layer

### `internal/application/service/alert_service.go`

Orchestration belongs in application layer (like `ChatService`):

```go
package service

import (
    "context"
    "fmt"
    "log"

    "code-editing-agent/internal/domain/entity"
    "code-editing-agent/internal/domain/port"
)

// AlertService orchestrates alert processing
type AlertService struct {
    sourceManager   port.AlertSourceManager
    conversationSvc *ConversationService // Inject alerts into conversations
}

// NewAlertService creates an alert orchestration service
func NewAlertService(
    sourceManager port.AlertSourceManager,
    convSvc *ConversationService,
) *AlertService {
    svc := &AlertService{
        sourceManager:   sourceManager,
        conversationSvc: convSvc,
    }

    // Wire up the alert handler
    sourceManager.SetAlertHandler(svc.handleAlert)

    return svc
}

// handleAlert processes incoming alerts
func (s *AlertService) handleAlert(ctx context.Context, alert *entity.Alert) error {
    log.Printf("[ALERT] [%s] %s: %s", alert.Severity(), alert.Source(), alert.Title())

    // Option 1: Inject as system message to active conversation
    if s.conversationSvc != nil {
        msg := fmt.Sprintf("ALERT [%s] from %s: %s\n%s",
            alert.Severity(),
            alert.Source(),
            alert.Title(),
            alert.Description(),
        )
        // s.conversationSvc.InjectSystemMessage(ctx, msg)
        _ = msg
    }

    // Option 2: Trigger autonomous agent action for critical alerts
    if alert.IsCritical() {
        // s.triggerAutonomousResponse(ctx, alert)
    }

    return nil
}

// Start begins processing alerts from all sources
func (s *AlertService) Start(ctx context.Context) error {
    return s.sourceManager.Start(ctx)
}

// Shutdown stops alert processing
func (s *AlertService) Shutdown() error {
    return s.sourceManager.Shutdown()
}
```

---

## Container Wiring

### `internal/infrastructure/config/container.go` (additions)

Following the established wiring pattern with explicit dependency order:

```go
func NewContainer(cfg *Config) (*Container, error) {
    // ... existing infrastructure adapters ...

    // === Alert Source Setup ===

    // 1. Create source registry (no dependencies)
    alertRegistry := alert.NewSourceRegistry()
    alertRegistry.RegisterBuiltinFactories()

    // 2. Create base alert source manager
    baseAlertManager := alert.NewLocalAlertSourceManager()

    // 3. Wrap with deduplication decorator (like PlanningExecutorAdapter)
    alertManager := alert.NewDeduplicatingAlertManager(baseAlertManager, 5*time.Minute)

    // 4. Set error callback
    baseAlertManager.SetErrorCallback(func(source string, err error) {
        log.Printf("[AlertSource:%s] error: %v", source, err)
    })

    // 5. Load and register sources from config
    result, err := loadAndRegisterAlertSources(alertRegistry, alertManager, cfg.AlertSourcesPath)
    if err != nil {
        return nil, fmt.Errorf("failed to load alert sources: %w", err)
    }
    log.Printf("Registered %d alert sources, %d failed", result.RegisteredCount, len(result.FailedSources))

    // 6. Create AlertService after ConversationService exists
    // (alertService wires handler to inject alerts)
    alertService := service.NewAlertService(alertManager, convService)

    c.alertManager = alertManager
    c.alertService = alertService

    return c, nil
}

// loadAndRegisterAlertSources loads config and registers sources
func loadAndRegisterAlertSources(
    registry *alert.SourceRegistry,
    manager port.AlertSourceManager,
    configPath string,
) (*port.AlertSourceDiscoveryResult, error) {
    result := &port.AlertSourceDiscoveryResult{
        FailedSources: make(map[string]error),
    }

    configs, err := loadAlertSourceConfigs(configPath)
    if err != nil {
        return result, err
    }

    for _, cfg := range configs {
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
```

---

## Configuration

### `config/alert-sources.yaml`

```yaml
sources:
  # Webhook (push)
  - type: prometheus
    name: prod-prometheus
    webhook_path: /alerts/prometheus

  # Poll (pull)
  - type: datadog
    name: datadog-prod
    api_key: ${DD_API_KEY}
    app_key: ${DD_APP_KEY}
    poll_interval: 60s

  # Stream (subscribe)
  - type: nats
    name: nats-alerts
    url: nats://localhost:4222
    subject: alerts.>

  - type: kafka
    name: kafka-alerts
    url: kafka1:9092,kafka2:9092
    subject: alerts
    extra:
      consumer_group: code-agent

  - type: redis-stream
    name: redis-alerts
    url: redis://localhost:6379
    subject: alerts
```

---

## Adding a New Source (Plugin Author Guide)

### Step 1: Create the adapter

Create `internal/infrastructure/adapter/alert/mysource.go`:

```go
package alert

import (
    "fmt"
    "code-editing-agent/internal/domain/entity"
    "code-editing-agent/internal/domain/port"
)

type MySource struct {
    name string
    // ... config fields
}

func NewMySource(cfg SourceConfig) (port.AlertSource, error) {
    if cfg.Name == "" {
        return nil, fmt.Errorf("mysource requires name")
    }
    return &MySource{name: cfg.Name}, nil
}

func (m *MySource) Name() string          { return m.name }
func (m *MySource) Type() port.SourceType { return port.SourceTypeWebhook } // or Poll, Stream
func (m *MySource) Close() error          { return nil }

// Implement the appropriate interface methods...
```

### Step 2: Register the factory

In `container.go` or a dedicated registration file:

```go
alertRegistry.RegisterFactory("mysource", alert.NewMySource)
```

### Step 3: Add configuration

Add to `config/alert-sources.yaml`:

```yaml
- type: mysource
  name: my-alerts
  # source-specific config...
```

---

## File Structure

```
internal/
├── domain/
│   ├── entity/
│   │   └── alert.go                    # Alert entity with validation
│   └── port/
│       └── alert_source.go             # Interfaces only (no structs!)
│
├── application/
│   └── service/
│       └── alert_service.go            # Alert orchestration
│
└── infrastructure/
    ├── adapter/
    │   └── alert/
    │       ├── source_registry.go      # Injectable factory registry
    │       ├── source_manager.go       # LocalAlertSourceManager
    │       ├── dedup_manager.go        # Decorator for deduplication
    │       ├── prometheus_source.go    # Webhook adapter
    │       ├── nats_source.go          # Stream adapter
    │       └── datadog_source.go       # Poll adapter
    │
    └── config/
        └── container.go                # Wiring with explicit order

config/
└── alert-sources.yaml                  # Source configuration
```

---

## Summary

| Aspect | Pattern Used | Reference |
|--------|--------------|-----------|
| Entity | Factory + Validate() | `entity/tool.go` |
| Port | Interfaces only | `port/tool_executor.go` |
| Registry | Injectable struct | `ExecutorAdapter.tools` |
| Manager | Adapter implementing port | `LocalSkillManager` |
| Middleware | Decorator pattern | `PlanningExecutorAdapter` |
| Handler | Context + error return | `port.AlertHandler` |
| Discovery | Result with failures | `SkillDiscoveryResult` |
| Container | Explicit wiring order | `config/container.go` |

This design aligns with the existing codebase patterns and avoids the issues identified in the original proposal.
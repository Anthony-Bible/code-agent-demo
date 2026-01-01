// Package port defines the interfaces (ports) for the alert ingestion system.
// These interfaces follow the hexagonal architecture pattern, allowing different
// alert sources (adapters) to be plugged in without changing the core domain logic.
package port

import (
	"code-editing-agent/internal/domain/entity"
	"context"
)

// SourceType represents the type of alert source, determining how alerts are received.
type SourceType string

// Alert source types define the communication pattern used to receive alerts.
const (
	// SourceTypeWebhook indicates a source that receives alerts via HTTP webhooks.
	// Examples: Prometheus Alertmanager, Grafana webhooks.
	SourceTypeWebhook SourceType = "webhook"
	// SourceTypePoll indicates a source that polls an external system for alerts.
	// Examples: Polling a REST API, checking a database.
	SourceTypePoll SourceType = "poll"
	// SourceTypeStream indicates a source that receives alerts via a persistent stream.
	// Examples: Kafka consumer, WebSocket connection.
	SourceTypeStream SourceType = "stream"
)

// AlertSource is the base interface that all alert sources must implement.
// It provides common functionality for identification and lifecycle management.
type AlertSource interface {
	// Name returns the unique identifier for this alert source.
	Name() string
	// Type returns the source type, indicating how alerts are received.
	Type() SourceType
	// Close releases any resources held by the source.
	Close() error
}

// WebhookAlertSource extends AlertSource for sources that receive alerts via HTTP webhooks.
// Implementations should parse the incoming payload and convert it to domain Alert entities.
type WebhookAlertSource interface {
	AlertSource
	// WebhookPath returns the HTTP path where this source receives webhooks.
	// The path must start with a leading slash (e.g., "/alerts/prometheus").
	WebhookPath() string
	// HandleWebhook processes an incoming webhook payload and returns parsed alerts.
	// Returns an error if the payload cannot be parsed.
	HandleWebhook(ctx context.Context, payload []byte) ([]*entity.Alert, error)
}

// AlertHandler is a callback function that processes incoming alerts.
// It is called by the AlertSourceManager when new alerts are received.
type AlertHandler func(ctx context.Context, alert *entity.Alert) error

// AlertSourceManager manages the lifecycle and registration of alert sources.
// It provides a central registry for sources and dispatches alerts to handlers.
type AlertSourceManager interface {
	// RegisterSource adds a new alert source to the manager.
	// Returns an error if a source with the same name is already registered.
	RegisterSource(source AlertSource) error
	// UnregisterSource removes and closes an alert source by name.
	// Returns an error if the source is not found.
	UnregisterSource(name string) error
	// GetSource retrieves a registered source by name.
	// Returns an error if the source is not found.
	GetSource(name string) (AlertSource, error)
	// ListSources returns all registered alert sources.
	ListSources() []AlertSource
	// SetAlertHandler sets the callback function for processing incoming alerts.
	SetAlertHandler(handler AlertHandler)
}

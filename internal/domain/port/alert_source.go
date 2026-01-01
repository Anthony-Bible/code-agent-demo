package port

import (
	"code-editing-agent/internal/domain/entity"
	"context"
)

// SourceType represents the type of alert source.
type SourceType string

const (
	// SourceTypeWebhook indicates a webhook-based alert source.
	SourceTypeWebhook SourceType = "webhook"
	// SourceTypePoll indicates a polling-based alert source.
	SourceTypePoll SourceType = "poll"
	// SourceTypeStream indicates a streaming-based alert source.
	SourceTypeStream SourceType = "stream"
)

// AlertSource is the base interface for all alert sources.
type AlertSource interface {
	Name() string
	Type() SourceType
	Close() error
}

// WebhookAlertSource is an alert source that receives alerts via HTTP webhooks.
type WebhookAlertSource interface {
	AlertSource
	WebhookPath() string
	HandleWebhook(ctx context.Context, payload []byte) ([]*entity.Alert, error)
}

// AlertHandler is a function that processes incoming alerts.
type AlertHandler func(ctx context.Context, alert *entity.Alert) error

// AlertSourceManager manages the lifecycle of alert sources.
type AlertSourceManager interface {
	RegisterSource(source AlertSource) error
	UnregisterSource(name string) error
	GetSource(name string) (AlertSource, error)
	ListSources() []AlertSource
	SetAlertHandler(handler AlertHandler)
}

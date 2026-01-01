// Package alert provides adapters for various alert sources.
// It implements the port.AlertSource interface for different alerting systems.
package alert

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// Configuration errors for alert sources.
var (
	errSourceNameRequired   = errors.New("source name is required")
	errWebhookPathRequired  = errors.New("webhook path is required")
	errWebhookPathNoSlash   = errors.New("webhook path must start with a leading slash")
	errWebhookPathTraversal = errors.New("webhook path contains path traversal")
	errEmptyPayload         = errors.New("empty payload")
)

// SourceConfig contains configuration for creating an alert source.
// It provides a unified configuration structure for all alert source types.
type SourceConfig struct {
	// Type specifies the alert source type (e.g., "prometheus", "grafana").
	Type string
	// Name is the unique identifier for this source instance.
	Name string
	// WebhookPath is the HTTP path for receiving webhooks (required for webhook sources).
	WebhookPath string
	// Extra contains additional source-specific configuration options.
	Extra map[string]string
}

// PrometheusSource implements port.WebhookAlertSource for Prometheus Alertmanager.
// It parses Alertmanager webhook payloads and converts them to domain Alert entities.
type PrometheusSource struct {
	name        string
	webhookPath string
	extra       map[string]string
}

// alertmanagerPayload represents the JSON structure of Alertmanager webhooks.
// See: https://prometheus.io/docs/alerting/latest/configuration/#webhook_config
type alertmanagerPayload struct {
	Alerts []alertmanagerAlert `json:"alerts"`
}

// alertmanagerAlert represents a single alert in the Alertmanager webhook payload.
type alertmanagerAlert struct {
	Status      string            `json:"status"`
	Labels      map[string]string `json:"labels"`
	Annotations map[string]string `json:"annotations"`
	StartsAt    time.Time         `json:"startsAt"`
	EndsAt      time.Time         `json:"endsAt"`
}

// NewPrometheusSource creates a new Prometheus alert source from the given configuration.
// Returns an error if the name or webhook path is invalid.
func NewPrometheusSource(config SourceConfig) (port.AlertSource, error) {
	if strings.TrimSpace(config.Name) == "" {
		return nil, errSourceNameRequired
	}
	if strings.TrimSpace(config.WebhookPath) == "" {
		return nil, errWebhookPathRequired
	}
	if !strings.HasPrefix(config.WebhookPath, "/") {
		return nil, errWebhookPathNoSlash
	}
	if strings.Contains(config.WebhookPath, "..") {
		return nil, errWebhookPathTraversal
	}

	return &PrometheusSource{
		name:        config.Name,
		webhookPath: config.WebhookPath,
		extra:       config.Extra,
	}, nil
}

// Name returns the source name.
func (p *PrometheusSource) Name() string {
	return p.name
}

// Type returns the source type.
func (p *PrometheusSource) Type() port.SourceType {
	return port.SourceTypeWebhook
}

// Close closes the source.
func (p *PrometheusSource) Close() error {
	return nil
}

// WebhookPath returns the webhook path.
func (p *PrometheusSource) WebhookPath() string {
	return p.webhookPath
}

// HandleWebhook processes an Alertmanager webhook payload and returns parsed alerts.
// Resolved alerts are skipped. Returns an error if the payload is empty or invalid JSON.
func (p *PrometheusSource) HandleWebhook(_ context.Context, payload []byte) ([]*entity.Alert, error) {
	if len(payload) == 0 {
		return nil, errEmptyPayload
	}

	var amPayload alertmanagerPayload
	if err := json.Unmarshal(payload, &amPayload); err != nil {
		return nil, err
	}

	var alerts []*entity.Alert
	for _, amAlert := range amPayload.Alerts {
		// Skip resolved alerts
		if amAlert.Status == "resolved" {
			continue
		}

		alertName, ok := amAlert.Labels["alertname"]
		if !ok || alertName == "" {
			continue
		}

		// Get severity, default to warning
		severity := amAlert.Labels["severity"]
		if severity == "" {
			severity = entity.SeverityWarning
		}

		// Get title from summary annotation or fall back to alertname
		title := amAlert.Annotations["summary"]
		if title == "" {
			title = alertName
		}

		// Create unique ID from alertname and timestamp
		alertID := alertName + "-" + amAlert.StartsAt.Format(time.RFC3339)

		alert, err := entity.NewAlert(alertID, p.name, severity, title)
		if err != nil {
			continue
		}

		// Set description from annotations
		if desc, ok := amAlert.Annotations["description"]; ok {
			alert.WithDescription(desc)
		}

		// Set labels
		alert.WithLabels(amAlert.Labels)

		// Set timestamp
		alert.WithTimestamp(amAlert.StartsAt)

		// Set raw payload
		alertPayload, _ := json.Marshal(amAlert)
		alert.WithRawPayload(alertPayload)

		alerts = append(alerts, alert)
	}

	return alerts, nil
}

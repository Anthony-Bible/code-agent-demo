// Package alert provides adapters for various alert sources.
// This file implements the Google Cloud Monitoring webhook alert source.
package alert

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"encoding/json"
	"strings"
	"time"
)

// GCPMonitoringSource implements port.WebhookAlertSource for Google Cloud Monitoring.
// It parses Cloud Monitoring webhook payloads (v1.2 schema) and converts them to domain Alert entities.
type GCPMonitoringSource struct {
	name        string
	webhookPath string
	extra       map[string]string
}

// gcpMonitoringPayload represents the JSON structure of GCP Monitoring webhooks (v1.2 schema).
// See: https://cloud.google.com/monitoring/support/notification-options
type gcpMonitoringPayload struct {
	Version  string                `json:"version"`
	Incident gcpMonitoringIncident `json:"incident"`
}

// gcpMonitoringIncident represents a single incident in the GCP Monitoring webhook payload.
type gcpMonitoringIncident struct {
	IncidentID     string      `json:"incident_id"`
	State          string      `json:"state"` // "open" or "closed"
	StartedAt      int64       `json:"started_at"`
	EndedAt        int64       `json:"ended_at"`
	Summary        string      `json:"summary"`
	PolicyName     string      `json:"policy_name"`
	ConditionName  string      `json:"condition_name"`
	Severity       string      `json:"severity"` // CRITICAL, ERROR, WARNING, INFO
	URL            string      `json:"url"`
	Resource       gcpResource `json:"resource"`
	Metric         gcpMetric   `json:"metric"`
	ObservedValue  string      `json:"observed_value"`
	ThresholdValue string      `json:"threshold_value"`
	Metadata       gcpMetadata `json:"metadata"`
}

// gcpResource represents the monitored resource.
type gcpResource struct {
	Type   string            `json:"type"`
	Labels map[string]string `json:"labels"`
}

// gcpMetric represents the metric being monitored.
type gcpMetric struct {
	Type        string            `json:"type"`
	DisplayName string            `json:"displayName"`
	Labels      map[string]string `json:"labels"`
}

// gcpMetadata represents metadata associated with the resource.
type gcpMetadata struct {
	SystemLabels map[string]string `json:"system_labels"`
	UserLabels   map[string]string `json:"user_labels"`
}

// NewGCPMonitoringSource creates a new GCP Monitoring alert source from the given configuration.
// Returns an error if the name or webhook path is invalid.
func NewGCPMonitoringSource(config SourceConfig) (port.AlertSource, error) {
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

	return &GCPMonitoringSource{
		name:        config.Name,
		webhookPath: config.WebhookPath,
		extra:       config.Extra,
	}, nil
}

// Name returns the source name.
func (g *GCPMonitoringSource) Name() string {
	return g.name
}

// Type returns the source type.
func (g *GCPMonitoringSource) Type() port.SourceType {
	return port.SourceTypeWebhook
}

// Close closes the source.
func (g *GCPMonitoringSource) Close() error {
	return nil
}

// WebhookPath returns the webhook path.
func (g *GCPMonitoringSource) WebhookPath() string {
	return g.webhookPath
}

// HandleWebhook processes a GCP Monitoring webhook payload and returns parsed alerts.
// Closed incidents are skipped. Returns an error if the payload is empty or invalid JSON.
func (g *GCPMonitoringSource) HandleWebhook(_ context.Context, payload []byte) ([]*entity.Alert, error) {
	if len(payload) == 0 {
		return nil, errEmptyPayload
	}

	var gcpPayload gcpMonitoringPayload
	if err := json.Unmarshal(payload, &gcpPayload); err != nil {
		return nil, err
	}

	incident := gcpPayload.Incident

	// Skip closed incidents
	if incident.State == "closed" {
		return []*entity.Alert{}, nil
	}

	// Use incident_id as the alert ID
	alertID := incident.IncidentID
	if alertID == "" {
		// Fall back to generating ID if incident_id is missing
		alertID = "gcp-" + time.Now().Format(time.RFC3339Nano)
	}

	// Map GCP severity to our severity levels
	severity := mapGCPSeverity(incident.Severity)

	// Build title from policy_name and condition_name
	title := buildTitle(incident.PolicyName, incident.ConditionName)

	// Create the alert
	alert, err := entity.NewAlert(alertID, g.name, severity, title)
	if err != nil {
		return nil, err
	}

	// Set description from summary
	if incident.Summary != "" {
		alert.WithDescription(incident.Summary)
	}

	// Combine all labels into a single map
	labels := make(map[string]string)

	// Add resource labels
	for k, v := range incident.Resource.Labels {
		labels["resource."+k] = v
	}

	// Add metric labels
	for k, v := range incident.Metric.Labels {
		labels["metric."+k] = v
	}

	// Add metadata labels
	for k, v := range incident.Metadata.UserLabels {
		labels["user."+k] = v
	}

	// Add key incident fields as labels
	if incident.Resource.Type != "" {
		labels["resource_type"] = incident.Resource.Type
	}
	if incident.Metric.Type != "" {
		labels["metric_type"] = incident.Metric.Type
	}
	if incident.Metric.DisplayName != "" {
		labels["metric_name"] = incident.Metric.DisplayName
	}
	if incident.ObservedValue != "" {
		labels["observed_value"] = incident.ObservedValue
	}
	if incident.ThresholdValue != "" {
		labels["threshold_value"] = incident.ThresholdValue
	}
	if incident.URL != "" {
		labels["console_url"] = incident.URL
	}

	alert.WithLabels(labels)

	// Set timestamp from started_at
	if incident.StartedAt > 0 {
		alert.WithTimestamp(time.Unix(incident.StartedAt, 0))
	}

	// Store raw payload
	alert.WithRawPayload(payload)

	return []*entity.Alert{alert}, nil
}

// mapGCPSeverity maps GCP severity levels to our entity severity levels.
func mapGCPSeverity(gcpSeverity string) string {
	switch strings.ToUpper(gcpSeverity) {
	case "CRITICAL", "ERROR":
		return entity.SeverityCritical
	case "WARNING":
		return entity.SeverityWarning
	default:
		return entity.SeverityInfo
	}
}

// buildTitle creates a descriptive title from policy and condition names.
func buildTitle(policyName, conditionName string) string {
	if policyName != "" && conditionName != "" {
		return policyName + ": " + conditionName
	}
	if policyName != "" {
		return policyName
	}
	if conditionName != "" {
		return conditionName
	}
	return "GCP Monitoring Alert"
}

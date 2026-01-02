package alert

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"testing"
)

func TestNewGCPMonitoringSource_Valid(t *testing.T) {
	source, err := NewGCPMonitoringSource(SourceConfig{
		Type:        "gcp_monitoring",
		Name:        "test-gcp",
		WebhookPath: "/alerts/gcp",
	})
	if err != nil {
		t.Errorf("NewGCPMonitoringSource() error = %v, want nil", err)
	}
	if source == nil {
		t.Error("NewGCPMonitoringSource() returned nil source without error")
	}
}

func TestNewGCPMonitoringSource_EmptyName(t *testing.T) {
	_, err := NewGCPMonitoringSource(SourceConfig{
		Type:        "gcp_monitoring",
		Name:        "",
		WebhookPath: "/alerts/gcp",
	})

	if err == nil {
		t.Error("NewGCPMonitoringSource() with empty name should return error")
	}
}

func TestNewGCPMonitoringSource_EmptyWebhookPath(t *testing.T) {
	_, err := NewGCPMonitoringSource(SourceConfig{
		Type:        "gcp_monitoring",
		Name:        "test-gcp",
		WebhookPath: "",
	})

	if err == nil {
		t.Error("NewGCPMonitoringSource() with empty webhook path should return error")
	}
}

func TestNewGCPMonitoringSource_PathTraversal(t *testing.T) {
	_, err := NewGCPMonitoringSource(SourceConfig{
		Type:        "gcp_monitoring",
		Name:        "test-gcp",
		WebhookPath: "/alerts/../etc/passwd",
	})

	if err == nil {
		t.Error("NewGCPMonitoringSource() with path traversal should return error")
	}
}

func TestGCPMonitoringSource_Interface(t *testing.T) {
	config := SourceConfig{
		Type:        "gcp_monitoring",
		Name:        "test-gcp",
		WebhookPath: "/alerts/gcp",
	}

	source, err := NewGCPMonitoringSource(config)
	if err != nil {
		t.Fatalf("NewGCPMonitoringSource() error = %v", err)
	}

	t.Run("should implement AlertSource interface", func(_ *testing.T) {
		_ = source
	})

	t.Run("should implement WebhookAlertSource interface", func(t *testing.T) {
		_, ok := source.(port.WebhookAlertSource)
		if !ok {
			t.Error("source does not implement WebhookAlertSource")
		}
	})

	t.Run("Name() should return configured name", func(t *testing.T) {
		if got := source.Name(); got != "test-gcp" {
			t.Errorf("Name() = %v, want %v", got, "test-gcp")
		}
	})

	t.Run("Type() should return webhook", func(t *testing.T) {
		if got := source.Type(); got != port.SourceTypeWebhook {
			t.Errorf("Type() = %v, want %v", got, port.SourceTypeWebhook)
		}
	})

	t.Run("WebhookPath() should return configured path", func(t *testing.T) {
		webhookSource, ok := source.(port.WebhookAlertSource)
		if !ok {
			t.Fatal("source does not implement WebhookAlertSource")
		}
		if got := webhookSource.WebhookPath(); got != "/alerts/gcp" {
			t.Errorf("WebhookPath() = %v, want %v", got, "/alerts/gcp")
		}
	})

	t.Run("Close() should not error", func(t *testing.T) {
		if err := source.Close(); err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}
	})
}

func TestGCPMonitoringSource_HandleWebhook_EmptyPayload(t *testing.T) {
	source := &GCPMonitoringSource{
		name:        "test-gcp",
		webhookPath: "/alerts/gcp",
	}

	_, err := source.HandleWebhook(context.Background(), []byte{})
	if err == nil {
		t.Error("HandleWebhook() with empty payload should return error")
	}
}

func TestGCPMonitoringSource_HandleWebhook_InvalidJSON(t *testing.T) {
	source := &GCPMonitoringSource{
		name:        "test-gcp",
		webhookPath: "/alerts/gcp",
	}

	_, err := source.HandleWebhook(context.Background(), []byte("not json"))
	if err == nil {
		t.Error("HandleWebhook() with invalid JSON should return error")
	}
}

func TestGCPMonitoringSource_HandleWebhook_ClosedIncident(t *testing.T) {
	source := &GCPMonitoringSource{
		name:        "test-gcp",
		webhookPath: "/alerts/gcp",
	}

	payload := []byte(`{
		"version": "1.2",
		"incident": {
			"incident_id": "test-123",
			"state": "closed",
			"policy_name": "Test Policy",
			"condition_name": "Test Condition"
		}
	}`)

	alerts, err := source.HandleWebhook(context.Background(), payload)
	if err != nil {
		t.Errorf("HandleWebhook() error = %v, want nil", err)
	}
	if len(alerts) != 0 {
		t.Errorf("HandleWebhook() returned %d alerts for closed incident, want 0", len(alerts))
	}
}

func TestGCPMonitoringSource_HandleWebhook_FullData(t *testing.T) {
	source := &GCPMonitoringSource{
		name:        "test-gcp",
		webhookPath: "/alerts/gcp",
	}

	payload := []byte(`{
		"version": "1.2",
		"incident": {
			"incident_id": "incident-123",
			"state": "open",
			"started_at": 1609459200,
			"summary": "CPU usage is above threshold",
			"policy_name": "High CPU Policy",
			"condition_name": "CPU > 80%",
			"severity": "CRITICAL",
			"url": "https://console.cloud.google.com/...",
			"resource": {
				"type": "gce_instance",
				"labels": {
					"instance_id": "1234567890",
					"zone": "us-central1-a"
				}
			},
			"metric": {
				"type": "compute.googleapis.com/instance/cpu/utilization",
				"displayName": "CPU utilization",
				"labels": {
					"instance_name": "my-instance"
				}
			},
			"observed_value": "0.95",
			"threshold_value": "0.80",
			"metadata": {
				"system_labels": {},
				"user_labels": {
					"env": "production"
				}
			}
		}
	}`)

	alerts, err := source.HandleWebhook(context.Background(), payload)
	if err != nil {
		t.Errorf("HandleWebhook() error = %v, want nil", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("HandleWebhook() returned %d alerts, want 1", len(alerts))
	}

	alert := alerts[0]

	if alert.ID() != "incident-123" {
		t.Errorf("Alert ID = %v, want %v", alert.ID(), "incident-123")
	}
	if alert.Source() != "test-gcp" {
		t.Errorf("Alert Source = %v, want %v", alert.Source(), "test-gcp")
	}
	if alert.Severity() != entity.SeverityCritical {
		t.Errorf("Alert Severity = %v, want %v", alert.Severity(), entity.SeverityCritical)
	}
	if alert.Title() != "High CPU Policy: CPU > 80%" {
		t.Errorf("Alert Title = %v, want %v", alert.Title(), "High CPU Policy: CPU > 80%")
	}
	if alert.Description() != "CPU usage is above threshold" {
		t.Errorf("Alert Description = %v, want %v", alert.Description(), "CPU usage is above threshold")
	}

	labels := alert.Labels()
	if labels["resource.instance_id"] != "1234567890" {
		t.Errorf("Label resource.instance_id = %v, want %v", labels["resource.instance_id"], "1234567890")
	}
	if labels["metric.instance_name"] != "my-instance" {
		t.Errorf("Label metric.instance_name = %v, want %v", labels["metric.instance_name"], "my-instance")
	}
	if labels["user.env"] != "production" {
		t.Errorf("Label user.env = %v, want %v", labels["user.env"], "production")
	}
	if labels["observed_value"] != "0.95" {
		t.Errorf("Label observed_value = %v, want %v", labels["observed_value"], "0.95")
	}
	if labels["console_url"] != "https://console.cloud.google.com/..." {
		t.Errorf("Label console_url = %v, want %v", labels["console_url"], "https://console.cloud.google.com/...")
	}
}

func TestGCPMonitoringSource_HandleWebhook_MinimalData(t *testing.T) {
	source := &GCPMonitoringSource{
		name:        "test-gcp",
		webhookPath: "/alerts/gcp",
	}

	payload := []byte(`{
		"version": "1.2",
		"incident": {
			"state": "open",
			"policy_name": "Simple Policy"
		}
	}`)

	alerts, err := source.HandleWebhook(context.Background(), payload)
	if err != nil {
		t.Errorf("HandleWebhook() error = %v, want nil", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("HandleWebhook() returned %d alerts, want 1", len(alerts))
	}

	alert := alerts[0]
	if alert.Title() != "Simple Policy" {
		t.Errorf("Alert Title = %v, want %v", alert.Title(), "Simple Policy")
	}
	if alert.Severity() != entity.SeverityInfo {
		t.Errorf("Alert Severity = %v, want %v (default)", alert.Severity(), entity.SeverityInfo)
	}
}

func TestMapGCPSeverity(t *testing.T) {
	tests := []struct {
		gcpSeverity  string
		wantSeverity string
	}{
		{"CRITICAL", entity.SeverityCritical},
		{"critical", entity.SeverityCritical},
		{"ERROR", entity.SeverityCritical},
		{"error", entity.SeverityCritical},
		{"WARNING", entity.SeverityWarning},
		{"warning", entity.SeverityWarning},
		{"INFO", entity.SeverityInfo},
		{"info", entity.SeverityInfo},
		{"", entity.SeverityInfo},
		{"unknown", entity.SeverityInfo},
	}

	for _, tt := range tests {
		t.Run(tt.gcpSeverity, func(t *testing.T) {
			got := mapGCPSeverity(tt.gcpSeverity)
			if got != tt.wantSeverity {
				t.Errorf("mapGCPSeverity(%q) = %v, want %v", tt.gcpSeverity, got, tt.wantSeverity)
			}
		})
	}
}

func TestBuildTitle(t *testing.T) {
	tests := []struct {
		name          string
		policyName    string
		conditionName string
		wantTitle     string
	}{
		{
			name:          "both names present",
			policyName:    "My Policy",
			conditionName: "My Condition",
			wantTitle:     "My Policy: My Condition",
		},
		{
			name:          "only policy name",
			policyName:    "My Policy",
			conditionName: "",
			wantTitle:     "My Policy",
		},
		{
			name:          "only condition name",
			policyName:    "",
			conditionName: "My Condition",
			wantTitle:     "My Condition",
		},
		{
			name:          "both empty",
			policyName:    "",
			conditionName: "",
			wantTitle:     "GCP Monitoring Alert",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildTitle(tt.policyName, tt.conditionName)
			if got != tt.wantTitle {
				t.Errorf("buildTitle(%q, %q) = %v, want %v", tt.policyName, tt.conditionName, got, tt.wantTitle)
			}
		})
	}
}

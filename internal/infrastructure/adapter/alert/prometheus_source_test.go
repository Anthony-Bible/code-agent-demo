package alert

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"testing"
	"time"
)

// =============================================================================
// Prometheus Source Tests - RED PHASE
// These tests define the expected behavior of the PrometheusSource adapter.
// All tests should FAIL until the implementation is complete.
// =============================================================================

func TestNewPrometheusSource_Validation(t *testing.T) {
	tests := []struct {
		name        string
		config      SourceConfig
		wantErr     bool
		errContains string
	}{
		{
			name: "should create source with valid config",
			config: SourceConfig{
				Name:        "prometheus-prod",
				WebhookPath: "/alerts/prometheus",
			},
			wantErr: false,
		},
		{
			name: "should create source with full config",
			config: SourceConfig{
				Type:        "prometheus",
				Name:        "prometheus-staging",
				WebhookPath: "/webhooks/alertmanager",
				Extra: map[string]string{
					"cluster": "staging-us-east",
				},
			},
			wantErr: false,
		},
		{
			name: "should reject missing name",
			config: SourceConfig{
				WebhookPath: "/alerts/prometheus",
			},
			wantErr:     true,
			errContains: "name",
		},
		{
			name: "should reject empty name",
			config: SourceConfig{
				Name:        "",
				WebhookPath: "/alerts/prometheus",
			},
			wantErr:     true,
			errContains: "name",
		},
		{
			name: "should reject missing webhook path",
			config: SourceConfig{
				Name: "prometheus-prod",
			},
			wantErr:     true,
			errContains: "webhook",
		},
		{
			name: "should reject empty webhook path",
			config: SourceConfig{
				Name:        "prometheus-prod",
				WebhookPath: "",
			},
			wantErr:     true,
			errContains: "webhook",
		},
		{
			name: "should reject webhook path without leading slash",
			config: SourceConfig{
				Name:        "prometheus-prod",
				WebhookPath: "alerts/prometheus",
			},
			wantErr:     true,
			errContains: "slash",
		},
		{
			name: "should reject webhook path with path traversal",
			config: SourceConfig{
				Name:        "prometheus-prod",
				WebhookPath: "/alerts/../../../etc/passwd",
			},
			wantErr:     true,
			errContains: "traversal",
		},
		{
			name: "should reject webhook path with double dots",
			config: SourceConfig{
				Name:        "prometheus-prod",
				WebhookPath: "/alerts/..hidden",
			},
			wantErr:     true,
			errContains: "traversal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := NewPrometheusSource(tt.config)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewPrometheusSource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if source != nil {
					t.Errorf("NewPrometheusSource() returned non-nil source on error")
				}
				if tt.errContains != "" && err != nil {
					if !containsIgnoreCase(err.Error(), tt.errContains) {
						t.Errorf("NewPrometheusSource() error = %v, should contain %q", err, tt.errContains)
					}
				}
			} else {
				if source == nil {
					t.Error("NewPrometheusSource() returned nil source without error")
				}
			}
		})
	}
}

func TestPrometheusSource_Interface(t *testing.T) {
	config := SourceConfig{
		Name:        "test-prometheus",
		WebhookPath: "/alerts/test",
	}

	source, err := NewPrometheusSource(config)
	if err != nil {
		t.Fatalf("NewPrometheusSource() error = %v", err)
	}

	t.Run("should implement AlertSource interface", func(t *testing.T) {
		_ = source
	})

	t.Run("should implement WebhookAlertSource interface", func(t *testing.T) {
		webhookSource, ok := source.(port.WebhookAlertSource)
		if !ok {
			t.Fatal("PrometheusSource should implement WebhookAlertSource")
		}

		if webhookSource.WebhookPath() != "/alerts/test" {
			t.Errorf("WebhookPath() = %v, want /alerts/test", webhookSource.WebhookPath())
		}
	})

	t.Run("Name should return configured name", func(t *testing.T) {
		if source.Name() != "test-prometheus" {
			t.Errorf("Name() = %v, want test-prometheus", source.Name())
		}
	})

	t.Run("Type should return webhook source type", func(t *testing.T) {
		if source.Type() != port.SourceTypeWebhook {
			t.Errorf("Type() = %v, want %v", source.Type(), port.SourceTypeWebhook)
		}
	})

	t.Run("Close should return nil error", func(t *testing.T) {
		if err := source.Close(); err != nil {
			t.Errorf("Close() error = %v, want nil", err)
		}
	})
}

func TestPrometheusSource_HandleWebhook(t *testing.T) {
	config := SourceConfig{
		Name:        "test-prometheus",
		WebhookPath: "/alerts/test",
	}

	source, err := NewPrometheusSource(config)
	if err != nil {
		t.Fatalf("NewPrometheusSource() error = %v", err)
	}

	webhookSource, ok := source.(port.WebhookAlertSource)
	if !ok {
		t.Fatal("PrometheusSource should implement WebhookAlertSource")
	}

	t.Run("should parse valid firing alert", func(t *testing.T) {
		payload := []byte(`{
			"alerts": [
				{
					"status": "firing",
					"labels": {
						"alertname": "HighCPU",
						"severity": "critical",
						"instance": "web-01"
					},
					"annotations": {
						"summary": "High CPU usage detected",
						"description": "CPU usage is above 90% for more than 5 minutes"
					},
					"startsAt": "2024-01-15T10:30:00Z",
					"endsAt": "0001-01-01T00:00:00Z"
				}
			]
		}`)

		ctx := context.Background()
		alerts, err := webhookSource.HandleWebhook(ctx, payload)
		if err != nil {
			t.Fatalf("HandleWebhook() error = %v", err)
		}

		if len(alerts) != 1 {
			t.Fatalf("HandleWebhook() returned %d alerts, want 1", len(alerts))
		}

		alert := alerts[0]
		if alert.Source() != "test-prometheus" {
			t.Errorf("Alert Source() = %v, want test-prometheus", alert.Source())
		}
		if alert.Severity() != entity.SeverityCritical {
			t.Errorf("Alert Severity() = %v, want critical", alert.Severity())
		}
		if alert.Title() != "High CPU usage detected" {
			t.Errorf("Alert Title() = %v, want 'High CPU usage detected'", alert.Title())
		}
		if alert.Description() != "CPU usage is above 90% for more than 5 minutes" {
			t.Errorf("Alert Description() = %v", alert.Description())
		}
		if alert.Labels()["instance"] != "web-01" {
			t.Errorf("Alert Labels()[instance] = %v, want web-01", alert.Labels()["instance"])
		}
	})

	t.Run("should skip resolved alerts", func(t *testing.T) {
		payload := []byte(`{
			"alerts": [
				{
					"status": "resolved",
					"labels": {
						"alertname": "HighCPU",
						"severity": "critical"
					},
					"annotations": {
						"summary": "High CPU usage detected"
					},
					"startsAt": "2024-01-15T10:30:00Z",
					"endsAt": "2024-01-15T10:45:00Z"
				}
			]
		}`)

		ctx := context.Background()
		alerts, err := webhookSource.HandleWebhook(ctx, payload)
		if err != nil {
			t.Fatalf("HandleWebhook() error = %v", err)
		}

		if len(alerts) != 0 {
			t.Errorf("HandleWebhook() returned %d alerts, want 0 (resolved should be skipped)", len(alerts))
		}
	})

	t.Run("should handle multiple alerts", func(t *testing.T) {
		payload := []byte(`{
			"alerts": [
				{
					"status": "firing",
					"labels": {
						"alertname": "HighCPU",
						"severity": "critical"
					},
					"annotations": {
						"summary": "High CPU"
					},
					"startsAt": "2024-01-15T10:30:00Z"
				},
				{
					"status": "resolved",
					"labels": {
						"alertname": "LowDisk",
						"severity": "warning"
					},
					"annotations": {
						"summary": "Low Disk"
					},
					"startsAt": "2024-01-15T10:25:00Z"
				},
				{
					"status": "firing",
					"labels": {
						"alertname": "HighMemory",
						"severity": "warning"
					},
					"annotations": {
						"summary": "High Memory"
					},
					"startsAt": "2024-01-15T10:35:00Z"
				}
			]
		}`)

		ctx := context.Background()
		alerts, err := webhookSource.HandleWebhook(ctx, payload)
		if err != nil {
			t.Fatalf("HandleWebhook() error = %v", err)
		}

		// Should have 2 alerts (skipping the resolved one)
		if len(alerts) != 2 {
			t.Errorf("HandleWebhook() returned %d alerts, want 2", len(alerts))
		}
	})

	t.Run("should return error for invalid JSON", func(t *testing.T) {
		payload := []byte(`{invalid json`)

		ctx := context.Background()
		alerts, err := webhookSource.HandleWebhook(ctx, payload)

		if err == nil {
			t.Error("HandleWebhook() should return error for invalid JSON")
		}
		if alerts != nil {
			t.Errorf("HandleWebhook() should return nil alerts on error, got %v", alerts)
		}
	})

	t.Run("should return error for empty payload", func(t *testing.T) {
		payload := []byte(``)

		ctx := context.Background()
		alerts, err := webhookSource.HandleWebhook(ctx, payload)

		if err == nil {
			t.Error("HandleWebhook() should return error for empty payload")
		}
		if alerts != nil {
			t.Errorf("HandleWebhook() should return nil alerts on error, got %v", alerts)
		}
	})

	t.Run("should handle empty alerts array", func(t *testing.T) {
		payload := []byte(`{"alerts": []}`)

		ctx := context.Background()
		alerts, err := webhookSource.HandleWebhook(ctx, payload)
		if err != nil {
			t.Fatalf("HandleWebhook() error = %v", err)
		}

		if len(alerts) != 0 {
			t.Errorf("HandleWebhook() returned %d alerts, want 0", len(alerts))
		}
	})

	t.Run("should use alertname as title when summary is missing", func(t *testing.T) {
		payload := []byte(`{
			"alerts": [
				{
					"status": "firing",
					"labels": {
						"alertname": "MyAlert",
						"severity": "warning"
					},
					"annotations": {},
					"startsAt": "2024-01-15T10:30:00Z"
				}
			]
		}`)

		ctx := context.Background()
		alerts, err := webhookSource.HandleWebhook(ctx, payload)
		if err != nil {
			t.Fatalf("HandleWebhook() error = %v", err)
		}

		if len(alerts) != 1 {
			t.Fatalf("HandleWebhook() returned %d alerts, want 1", len(alerts))
		}

		if alerts[0].Title() != "MyAlert" {
			t.Errorf("Alert Title() = %v, want MyAlert (should fallback to alertname)", alerts[0].Title())
		}
	})

	t.Run("should return error when alertname label is missing", func(t *testing.T) {
		payload := []byte(`{
			"alerts": [
				{
					"status": "firing",
					"labels": {
						"severity": "warning"
					},
					"annotations": {
						"summary": "Some alert"
					},
					"startsAt": "2024-01-15T10:30:00Z"
				}
			]
		}`)

		ctx := context.Background()
		alerts, err := webhookSource.HandleWebhook(ctx, payload)

		// This could either return an error or skip the alert
		// The design doc shows alertname is required for ID generation
		if err == nil && len(alerts) > 0 {
			t.Error("HandleWebhook() should either error or skip alert when alertname is missing")
		}
	})

	t.Run("should default to warning severity when missing", func(t *testing.T) {
		payload := []byte(`{
			"alerts": [
				{
					"status": "firing",
					"labels": {
						"alertname": "NoSeverityAlert"
					},
					"annotations": {
						"summary": "Alert without severity"
					},
					"startsAt": "2024-01-15T10:30:00Z"
				}
			]
		}`)

		ctx := context.Background()
		alerts, err := webhookSource.HandleWebhook(ctx, payload)
		if err != nil {
			t.Fatalf("HandleWebhook() error = %v", err)
		}

		if len(alerts) != 1 {
			t.Fatalf("HandleWebhook() returned %d alerts, want 1", len(alerts))
		}

		if alerts[0].Severity() != entity.SeverityWarning {
			t.Errorf("Alert Severity() = %v, want warning (default)", alerts[0].Severity())
		}
	})

	t.Run("should set correct timestamp from startsAt", func(t *testing.T) {
		payload := []byte(`{
			"alerts": [
				{
					"status": "firing",
					"labels": {
						"alertname": "TimestampTest",
						"severity": "info"
					},
					"annotations": {
						"summary": "Test timestamp"
					},
					"startsAt": "2024-01-15T10:30:00Z"
				}
			]
		}`)

		ctx := context.Background()
		alerts, err := webhookSource.HandleWebhook(ctx, payload)
		if err != nil {
			t.Fatalf("HandleWebhook() error = %v", err)
		}

		if len(alerts) != 1 {
			t.Fatalf("HandleWebhook() returned %d alerts, want 1", len(alerts))
		}

		expectedTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		if !alerts[0].Timestamp().Equal(expectedTime) {
			t.Errorf("Alert Timestamp() = %v, want %v", alerts[0].Timestamp(), expectedTime)
		}
	})

	t.Run("should include raw payload in alert", func(t *testing.T) {
		payload := []byte(`{
			"alerts": [
				{
					"status": "firing",
					"labels": {
						"alertname": "RawPayloadTest",
						"severity": "info"
					},
					"annotations": {
						"summary": "Test raw payload"
					},
					"startsAt": "2024-01-15T10:30:00Z"
				}
			]
		}`)

		ctx := context.Background()
		alerts, err := webhookSource.HandleWebhook(ctx, payload)
		if err != nil {
			t.Fatalf("HandleWebhook() error = %v", err)
		}

		if len(alerts) != 1 {
			t.Fatalf("HandleWebhook() returned %d alerts, want 1", len(alerts))
		}

		if alerts[0].RawPayload() == nil {
			t.Error("Alert RawPayload() should not be nil")
		}
	})

	t.Run("should respect context cancellation", func(t *testing.T) {
		payload := []byte(`{
			"alerts": [
				{
					"status": "firing",
					"labels": {
						"alertname": "ContextTest",
						"severity": "info"
					},
					"annotations": {
						"summary": "Test context"
					},
					"startsAt": "2024-01-15T10:30:00Z"
				}
			]
		}`)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		// Should either return context error or handle gracefully
		_, err := webhookSource.HandleWebhook(ctx, payload)

		// Either error due to context or successful parse is acceptable
		// The key is it shouldn't hang
		_ = err
	})
}

// Helper function for case-insensitive string contains.
func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(substr) == 0 ||
			(len(s) > 0 && containsIgnoreCaseImpl(s, substr)))
}

func containsIgnoreCaseImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFoldAt(s, i, substr) {
			return true
		}
	}
	return false
}

func equalFoldAt(s string, start int, substr string) bool {
	for j := range len(substr) {
		c1 := s[start+j]
		c2 := substr[j]
		if c1 == c2 {
			continue
		}
		// ASCII lowercase
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

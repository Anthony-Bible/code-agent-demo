package entity

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Alert Entity Tests - RED PHASE
// These tests define the expected behavior of the Alert entity.
// All tests should FAIL until the implementation is complete.
// =============================================================================

func TestNewAlert_Validation(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		source      string
		severity    string
		title       string
		wantErr     bool
		expectedErr error
	}{
		{
			name:     "should create valid critical alert",
			id:       "alert-001",
			source:   "prometheus",
			severity: SeverityCritical,
			title:    "High CPU Usage",
			wantErr:  false,
		},
		{
			name:     "should create valid warning alert",
			id:       "alert-002",
			source:   "datadog",
			severity: SeverityWarning,
			title:    "Memory Usage Elevated",
			wantErr:  false,
		},
		{
			name:     "should create valid info alert",
			id:       "alert-003",
			source:   "nats",
			severity: SeverityInfo,
			title:    "Deployment Started",
			wantErr:  false,
		},
		{
			name:        "should reject empty ID",
			id:          "",
			source:      "prometheus",
			severity:    SeverityCritical,
			title:       "Test Alert",
			wantErr:     true,
			expectedErr: ErrEmptyAlertID,
		},
		{
			name:        "should reject whitespace-only ID",
			id:          "   ",
			source:      "prometheus",
			severity:    SeverityCritical,
			title:       "Test Alert",
			wantErr:     true,
			expectedErr: ErrEmptyAlertID,
		},
		{
			name:        "should reject empty source",
			id:          "alert-004",
			source:      "",
			severity:    SeverityWarning,
			title:       "Test Alert",
			wantErr:     true,
			expectedErr: ErrEmptyAlertSource,
		},
		{
			name:        "should reject whitespace-only source",
			id:          "alert-005",
			source:      "  \t  ",
			severity:    SeverityWarning,
			title:       "Test Alert",
			wantErr:     true,
			expectedErr: ErrEmptyAlertSource,
		},
		{
			name:        "should reject empty title",
			id:          "alert-006",
			source:      "prometheus",
			severity:    SeverityInfo,
			title:       "",
			wantErr:     true,
			expectedErr: ErrEmptyAlertTitle,
		},
		{
			name:        "should reject whitespace-only title",
			id:          "alert-007",
			source:      "prometheus",
			severity:    SeverityInfo,
			title:       "   ",
			wantErr:     true,
			expectedErr: ErrEmptyAlertTitle,
		},
		{
			name:        "should reject invalid severity",
			id:          "alert-008",
			source:      "prometheus",
			severity:    "unknown",
			title:       "Test Alert",
			wantErr:     true,
			expectedErr: ErrInvalidSeverity,
		},
		{
			name:        "should reject empty severity",
			id:          "alert-009",
			source:      "prometheus",
			severity:    "",
			title:       "Test Alert",
			wantErr:     true,
			expectedErr: ErrInvalidSeverity,
		},
		{
			name:        "should reject mixed case severity",
			id:          "alert-010",
			source:      "prometheus",
			severity:    "Critical",
			title:       "Test Alert",
			wantErr:     true,
			expectedErr: ErrInvalidSeverity,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewAlert(tt.id, tt.source, tt.severity, tt.title)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewAlert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				if got != nil {
					t.Errorf("NewAlert() returned non-nil alert on error: %+v", got)
				}
				if tt.expectedErr != nil && !errors.Is(err, tt.expectedErr) {
					t.Errorf("NewAlert() error = %v, expected %v", err, tt.expectedErr)
				}
				return
			}

			if got == nil {
				t.Error("NewAlert() returned nil alert without error")
				return
			}
			if got.ID() != tt.id {
				t.Errorf("NewAlert() ID = %v, want %v", got.ID(), tt.id)
			}
			if got.Source() != tt.source {
				t.Errorf("NewAlert() Source = %v, want %v", got.Source(), tt.source)
			}
			if got.Severity() != tt.severity {
				t.Errorf("NewAlert() Severity = %v, want %v", got.Severity(), tt.severity)
			}
			if got.Title() != tt.title {
				t.Errorf("NewAlert() Title = %v, want %v", got.Title(), tt.title)
			}
		})
	}
}

func TestAlert_Getters(t *testing.T) {
	t.Run("should return correct values for all getters", func(t *testing.T) {
		alert, err := NewAlert("alert-getter-test", "test-source", SeverityCritical, "Test Title")
		if err != nil {
			t.Fatalf("NewAlert() error = %v", err)
		}

		if alert.ID() != "alert-getter-test" {
			t.Errorf("ID() = %v, want %v", alert.ID(), "alert-getter-test")
		}

		if alert.Source() != "test-source" {
			t.Errorf("Source() = %v, want %v", alert.Source(), "test-source")
		}

		if alert.Severity() != SeverityCritical {
			t.Errorf("Severity() = %v, want %v", alert.Severity(), SeverityCritical)
		}

		if alert.Title() != "Test Title" {
			t.Errorf("Title() = %v, want %v", alert.Title(), "Test Title")
		}

		// Description should be empty by default
		if alert.Description() != "" {
			t.Errorf("Description() = %v, want empty string", alert.Description())
		}

		// Labels should be empty but not nil
		if alert.Labels() == nil {
			t.Error("Labels() should not be nil")
		}
		if len(alert.Labels()) != 0 {
			t.Errorf("Labels() should be empty, got %v", alert.Labels())
		}

		// Timestamp should be set to approximately now
		if alert.Timestamp().IsZero() {
			t.Error("Timestamp() should not be zero")
		}
		timeDiff := time.Since(alert.Timestamp())
		if timeDiff > time.Second {
			t.Errorf("Timestamp() is too old: %v ago", timeDiff)
		}

		// RawPayload should be nil by default
		if alert.RawPayload() != nil {
			t.Errorf("RawPayload() = %v, want nil", alert.RawPayload())
		}
	})
}

func TestAlert_IsCritical(t *testing.T) {
	tests := []struct {
		name     string
		severity string
		want     bool
	}{
		{
			name:     "critical severity should return true",
			severity: SeverityCritical,
			want:     true,
		},
		{
			name:     "warning severity should return false",
			severity: SeverityWarning,
			want:     false,
		},
		{
			name:     "info severity should return false",
			severity: SeverityInfo,
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			alert, err := NewAlert("test-id", "test-source", tt.severity, "Test Title")
			if err != nil {
				t.Fatalf("NewAlert() error = %v", err)
			}

			if got := alert.IsCritical(); got != tt.want {
				t.Errorf("IsCritical() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAlert_Age(t *testing.T) {
	t.Run("should return positive duration for new alert", func(t *testing.T) {
		alert, err := NewAlert("test-id", "test-source", SeverityInfo, "Test Title")
		if err != nil {
			t.Fatalf("NewAlert() error = %v", err)
		}

		age := alert.Age()
		if age < 0 {
			t.Errorf("Age() = %v, want positive duration", age)
		}
	})

	t.Run("should return approximately correct age for older alert", func(t *testing.T) {
		alert, err := NewAlert("test-id", "test-source", SeverityInfo, "Test Title")
		if err != nil {
			t.Fatalf("NewAlert() error = %v", err)
		}

		// Set timestamp to 5 minutes ago
		pastTime := time.Now().Add(-5 * time.Minute)
		alert.WithTimestamp(pastTime)

		age := alert.Age()
		expectedAge := 5 * time.Minute

		// Allow 1 second tolerance
		if age < expectedAge-time.Second || age > expectedAge+time.Second {
			t.Errorf("Age() = %v, want approximately %v", age, expectedAge)
		}
	})
}

func TestAlert_WithDescription(t *testing.T) {
	t.Run("should set description using builder pattern", func(t *testing.T) {
		alert, err := NewAlert("test-id", "test-source", SeverityWarning, "Test Title")
		if err != nil {
			t.Fatalf("NewAlert() error = %v", err)
		}

		description := "This is a detailed description of the alert"
		result := alert.WithDescription(description)

		// Should return same alert for chaining
		if result != alert {
			t.Error("WithDescription() should return the same alert instance for chaining")
		}

		if alert.Description() != description {
			t.Errorf("Description() = %v, want %v", alert.Description(), description)
		}
	})

	t.Run("should allow empty description", func(t *testing.T) {
		alert, err := NewAlert("test-id", "test-source", SeverityWarning, "Test Title")
		if err != nil {
			t.Fatalf("NewAlert() error = %v", err)
		}

		alert.WithDescription("initial description")
		alert.WithDescription("")

		if alert.Description() != "" {
			t.Errorf("Description() = %v, want empty string", alert.Description())
		}
	})
}

func TestAlert_WithLabels(t *testing.T) {
	t.Run("should set labels using builder pattern", func(t *testing.T) {
		alert, err := NewAlert("test-id", "test-source", SeverityWarning, "Test Title")
		if err != nil {
			t.Fatalf("NewAlert() error = %v", err)
		}

		labels := map[string]string{
			"env":      "production",
			"instance": "web-01",
			"team":     "platform",
		}
		result := alert.WithLabels(labels)

		// Should return same alert for chaining
		if result != alert {
			t.Error("WithLabels() should return the same alert instance for chaining")
		}

		gotLabels := alert.Labels()
		if len(gotLabels) != len(labels) {
			t.Errorf("Labels() len = %v, want %v", len(gotLabels), len(labels))
		}

		for k, v := range labels {
			if gotLabels[k] != v {
				t.Errorf("Labels()[%q] = %v, want %v", k, gotLabels[k], v)
			}
		}
	})

	t.Run("should handle nil labels", func(t *testing.T) {
		alert, err := NewAlert("test-id", "test-source", SeverityWarning, "Test Title")
		if err != nil {
			t.Fatalf("NewAlert() error = %v", err)
		}

		alert.WithLabels(nil)

		// Should not panic, labels might be nil or empty map
		labels := alert.Labels()
		if len(labels) != 0 {
			t.Errorf("Labels() should be nil or empty after setting nil, got %v", labels)
		}
	})
}

func TestAlert_WithTimestamp(t *testing.T) {
	t.Run("should set custom timestamp using builder pattern", func(t *testing.T) {
		alert, err := NewAlert("test-id", "test-source", SeverityCritical, "Test Title")
		if err != nil {
			t.Fatalf("NewAlert() error = %v", err)
		}

		customTime := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
		result := alert.WithTimestamp(customTime)

		// Should return same alert for chaining
		if result != alert {
			t.Error("WithTimestamp() should return the same alert instance for chaining")
		}

		if !alert.Timestamp().Equal(customTime) {
			t.Errorf("Timestamp() = %v, want %v", alert.Timestamp(), customTime)
		}
	})
}

func TestAlert_WithRawPayload(t *testing.T) {
	t.Run("should set raw payload using builder pattern", func(t *testing.T) {
		alert, err := NewAlert("test-id", "test-source", SeverityInfo, "Test Title")
		if err != nil {
			t.Fatalf("NewAlert() error = %v", err)
		}

		payload := []byte(`{"key": "value", "nested": {"foo": "bar"}}`)
		result := alert.WithRawPayload(payload)

		// Should return same alert for chaining
		if result != alert {
			t.Error("WithRawPayload() should return the same alert instance for chaining")
		}

		gotPayload := alert.RawPayload()
		if string(gotPayload) != string(payload) {
			t.Errorf("RawPayload() = %s, want %s", string(gotPayload), string(payload))
		}
	})

	t.Run("should handle nil payload", func(t *testing.T) {
		alert, err := NewAlert("test-id", "test-source", SeverityInfo, "Test Title")
		if err != nil {
			t.Fatalf("NewAlert() error = %v", err)
		}

		alert.WithRawPayload([]byte("initial"))
		alert.WithRawPayload(nil)

		if alert.RawPayload() != nil {
			t.Errorf("RawPayload() = %v, want nil", alert.RawPayload())
		}
	})
}

func TestAlert_Labels_DefensiveCopy(t *testing.T) {
	t.Run("modifying returned labels should not affect entity", func(t *testing.T) {
		alert, err := NewAlert("test-id", "test-source", SeverityWarning, "Test Title")
		if err != nil {
			t.Fatalf("NewAlert() error = %v", err)
		}

		originalLabels := map[string]string{
			"env": "production",
		}
		alert.WithLabels(originalLabels)

		// Get labels and modify the returned map
		returnedLabels := alert.Labels()
		returnedLabels["env"] = "staging"
		returnedLabels["new-key"] = "new-value"

		// Original entity labels should not be affected
		freshLabels := alert.Labels()
		if freshLabels["env"] != "production" {
			t.Errorf("Labels modification affected entity: env = %v, want production", freshLabels["env"])
		}
		if _, exists := freshLabels["new-key"]; exists {
			t.Error("Labels modification added key to entity")
		}
	})

	t.Run("modifying input labels should not affect entity", func(t *testing.T) {
		alert, err := NewAlert("test-id", "test-source", SeverityWarning, "Test Title")
		if err != nil {
			t.Fatalf("NewAlert() error = %v", err)
		}

		inputLabels := map[string]string{
			"env": "production",
		}
		alert.WithLabels(inputLabels)

		// Modify the input map after setting
		inputLabels["env"] = "staging"
		inputLabels["new-key"] = "new-value"

		// Entity labels should not be affected
		entityLabels := alert.Labels()
		if entityLabels["env"] != "production" {
			t.Errorf("Input modification affected entity: env = %v, want production", entityLabels["env"])
		}
		if _, exists := entityLabels["new-key"]; exists {
			t.Error("Input modification added key to entity")
		}
	})
}

func TestAlert_BuilderChaining(t *testing.T) {
	t.Run("should support full builder pattern chaining", func(t *testing.T) {
		alert, err := NewAlert("chain-test", "test-source", SeverityCritical, "Chained Alert")
		if err != nil {
			t.Fatalf("NewAlert() error = %v", err)
		}

		customTime := time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC)
		payload := []byte(`{"test": true}`)

		// Chain all builder methods
		result := alert.
			WithDescription("Detailed description").
			WithLabels(map[string]string{"key": "value"}).
			WithTimestamp(customTime).
			WithRawPayload(payload)

		if result != alert {
			t.Error("Builder chain should return the same alert instance")
		}

		if alert.Description() != "Detailed description" {
			t.Errorf("Description() = %v, want Detailed description", alert.Description())
		}
		if alert.Labels()["key"] != "value" {
			t.Errorf("Labels()[key] = %v, want value", alert.Labels()["key"])
		}
		if !alert.Timestamp().Equal(customTime) {
			t.Errorf("Timestamp() = %v, want %v", alert.Timestamp(), customTime)
		}
		if string(alert.RawPayload()) != string(payload) {
			t.Errorf("RawPayload() = %s, want %s", alert.RawPayload(), payload)
		}
	})
}

func TestAlert_Validate(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		source      string
		severity    string
		title       string
		wantErr     bool
		errContains string
	}{
		{
			name:     "valid alert should pass validation",
			id:       "valid-id",
			source:   "valid-source",
			severity: SeverityCritical,
			title:    "Valid Title",
			wantErr:  false,
		},
		{
			name:        "empty id should fail validation",
			id:          "",
			source:      "valid-source",
			severity:    SeverityCritical,
			title:       "Valid Title",
			wantErr:     true,
			errContains: "ID",
		},
		{
			name:        "empty source should fail validation",
			id:          "valid-id",
			source:      "",
			severity:    SeverityCritical,
			title:       "Valid Title",
			wantErr:     true,
			errContains: "source",
		},
		{
			name:        "empty title should fail validation",
			id:          "valid-id",
			source:      "valid-source",
			severity:    SeverityCritical,
			title:       "",
			wantErr:     true,
			errContains: "title",
		},
		{
			name:        "invalid severity should fail validation",
			id:          "valid-id",
			source:      "valid-source",
			severity:    "invalid",
			title:       "Valid Title",
			wantErr:     true,
			errContains: "severity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: We cannot create an invalid alert via NewAlert, so we need to
			// test validation by attempting to create with invalid params
			_, err := NewAlert(tt.id, tt.source, tt.severity, tt.title)

			if (err != nil) != tt.wantErr {
				t.Errorf("NewAlert() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr && tt.errContains != "" {
				if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tt.errContains)) {
					t.Errorf("NewAlert() error = %v, should contain %q", err, tt.errContains)
				}
			}
		})
	}
}

func TestSeverityConstants(t *testing.T) {
	t.Run("severity constants should have expected values", func(t *testing.T) {
		if SeverityCritical != "critical" {
			t.Errorf("SeverityCritical = %v, want critical", SeverityCritical)
		}
		if SeverityWarning != "warning" {
			t.Errorf("SeverityWarning = %v, want warning", SeverityWarning)
		}
		if SeverityInfo != "info" {
			t.Errorf("SeverityInfo = %v, want info", SeverityInfo)
		}
	})
}

func TestAlertErrors(t *testing.T) {
	t.Run("error constants should be defined", func(t *testing.T) {
		if ErrEmptyAlertID == nil {
			t.Error("ErrEmptyAlertID should not be nil")
		}
		if ErrEmptyAlertSource == nil {
			t.Error("ErrEmptyAlertSource should not be nil")
		}
		if ErrEmptyAlertTitle == nil {
			t.Error("ErrEmptyAlertTitle should not be nil")
		}
		if ErrInvalidSeverity == nil {
			t.Error("ErrInvalidSeverity should not be nil")
		}
	})

	t.Run("error messages should be descriptive", func(t *testing.T) {
		if ErrEmptyAlertID.Error() == "" {
			t.Error("ErrEmptyAlertID should have a message")
		}
		if ErrEmptyAlertSource.Error() == "" {
			t.Error("ErrEmptyAlertSource should have a message")
		}
		if ErrEmptyAlertTitle.Error() == "" {
			t.Error("ErrEmptyAlertTitle should have a message")
		}
		if ErrInvalidSeverity.Error() == "" {
			t.Error("ErrInvalidSeverity should have a message")
		}
	})
}

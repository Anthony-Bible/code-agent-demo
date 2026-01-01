// Package entity contains the core domain entities for the code-editing-agent.
// These entities represent the fundamental business objects and their behaviors.
package entity

import (
	"errors"
	"strings"
	"time"
)

// Alert severity levels define the urgency of an alert.
// Use these constants when creating new alerts to ensure valid severity values.
const (
	// SeverityCritical indicates an urgent issue requiring immediate attention.
	SeverityCritical = "critical"
	// SeverityWarning indicates a potential issue that should be investigated.
	SeverityWarning = "warning"
	// SeverityInfo indicates an informational notification.
	SeverityInfo = "info"
)

// Sentinel errors for Alert validation.
// These errors are returned by NewAlert and Validate when validation fails.
var (
	// ErrEmptyAlertID is returned when an alert ID is empty or whitespace-only.
	ErrEmptyAlertID = errors.New("alert ID cannot be empty")
	// ErrEmptyAlertSource is returned when an alert source is empty or whitespace-only.
	ErrEmptyAlertSource = errors.New("alert source cannot be empty")
	// ErrEmptyAlertTitle is returned when an alert title is empty or whitespace-only.
	ErrEmptyAlertTitle = errors.New("alert title cannot be empty")
	// ErrInvalidSeverity is returned when an alert severity is not one of the valid constants.
	ErrInvalidSeverity = errors.New("invalid severity level")
)

// Alert represents a notification from an alerting system such as Prometheus Alertmanager.
// It is an immutable entity once created, with optional fields set via builder methods.
// Alert implements defensive copying for mutable fields like labels.
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

// NewAlert creates a new Alert with the required fields.
// The id, source, and title fields are trimmed of leading/trailing whitespace.
// Returns an error if validation fails (empty required fields or invalid severity).
func NewAlert(id, source, severity, title string) (*Alert, error) {
	a := &Alert{
		id:        strings.TrimSpace(id),
		source:    strings.TrimSpace(source),
		severity:  severity,
		title:     strings.TrimSpace(title),
		timestamp: time.Now(),
		labels:    make(map[string]string),
	}
	if err := a.Validate(); err != nil {
		return nil, err
	}
	return a, nil
}

// Validate checks that all required fields are set and valid.
// Returns the first validation error encountered, or nil if the alert is valid.
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

// ID returns the alert identifier.
func (a *Alert) ID() string { return a.id }

// Source returns the alert source name.
func (a *Alert) Source() string { return a.source }

// Severity returns the alert severity level.
func (a *Alert) Severity() string { return a.severity }

// Title returns the alert title.
func (a *Alert) Title() string { return a.title }

// Description returns the alert description.
func (a *Alert) Description() string { return a.description }

// Timestamp returns the alert timestamp.
func (a *Alert) Timestamp() time.Time { return a.timestamp }

// RawPayload returns the raw payload bytes.
func (a *Alert) RawPayload() []byte { return a.rawPayload }

// Labels returns a defensive copy of the alert labels.
func (a *Alert) Labels() map[string]string {
	if a.labels == nil {
		return nil
	}
	result := make(map[string]string, len(a.labels))
	for k, v := range a.labels {
		result[k] = v
	}
	return result
}

// IsCritical returns true if the alert has critical severity.
func (a *Alert) IsCritical() bool { return a.severity == SeverityCritical }

// Age returns the duration since the alert was created.
func (a *Alert) Age() time.Duration { return time.Since(a.timestamp) }

// WithDescription sets the alert description and returns the alert for chaining.
func (a *Alert) WithDescription(desc string) *Alert {
	a.description = desc
	return a
}

// WithLabels sets the alert labels and returns the alert for chaining.
func (a *Alert) WithLabels(labels map[string]string) *Alert {
	a.labels = make(map[string]string, len(labels))
	for k, v := range labels {
		a.labels[k] = v
	}
	return a
}

// WithTimestamp sets the alert timestamp and returns the alert for chaining.
func (a *Alert) WithTimestamp(t time.Time) *Alert {
	a.timestamp = t
	return a
}

// WithRawPayload sets the raw payload and returns the alert for chaining.
func (a *Alert) WithRawPayload(payload []byte) *Alert {
	a.rawPayload = payload
	return a
}

// isValidSeverity checks if the given string is a valid severity level.
func isValidSeverity(s string) bool {
	switch s {
	case SeverityCritical, SeverityWarning, SeverityInfo:
		return true
	}
	return false
}

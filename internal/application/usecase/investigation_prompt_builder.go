// Package usecase contains application use cases.
package usecase

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors for prompt builder operations.
// These errors are returned when prompt generation fails.
var (
	// ErrNilAlert is returned when BuildPrompt is called with a nil alert.
	ErrNilAlert = errors.New("alert cannot be nil")
	// ErrUnknownAlertType is returned when an alert type has no registered builder.
	ErrUnknownAlertType = errors.New("unknown alert type")
	// ErrPromptBuilderNotFound is returned when no builder is registered for an alert type.
	ErrPromptBuilderNotFound = errors.New("prompt builder not found for alert type")
	// ErrEmptyPromptTemplate is returned when a prompt template is empty.
	ErrEmptyPromptTemplate = errors.New("prompt template cannot be empty")
	// ErrInvalidPromptVariables is returned when required template variables are missing.
	ErrInvalidPromptVariables = errors.New("missing required prompt variables")
	// ErrNilPromptBuilder is returned when Register is called with a nil builder.
	ErrNilPromptBuilder = errors.New("prompt builder cannot be nil")
)

// investigationToolsHeader is the common preamble that explains available tools to the AI.
const investigationToolsHeader = `You are an AI assistant investigating an alert. You MUST use the available tools to investigate.

## Available Tools

1. **bash** - Execute shell commands to gather information
   Example: {"command": "top -b -n 1 | head -20"}

2. **read_file** - Read file contents
   Example: {"path": "/var/log/syslog"}

3. **list_files** - List directory contents
   Example: {"path": "/var/log"}

4. **complete_investigation** - Call this when you have finished investigating
   Required fields:
   - findings: array of strings describing what you found
   - confidence: number from 0.0 to 1.0 indicating your confidence level
   Example: {"findings": ["High CPU from process X", "Caused by memory leak"], "confidence": 0.85}

5. **escalate_investigation** - Call this if you cannot resolve the issue
   Required fields:
   - reason: why you are escalating
   - partial_findings: any findings so far
   Example: {"reason": "Unable to access logs", "partial_findings": ["Detected high load"]}

## IMPORTANT RULES

- You MUST use the bash tool to execute commands (not just describe them)
- You MUST end by calling either complete_investigation or escalate_investigation
- Use read-only commands only - DO NOT modify, restart, or kill anything
- If you cannot determine the root cause, use escalate_investigation

`

// AlertStub represents a lightweight alert structure for prompt building.
// It contains only the fields needed to generate investigation prompts.
type AlertStub struct {
	id          string            // Unique alert identifier
	source      string            // Alert source system (e.g., "prometheus", "cloudwatch")
	severity    string            // Alert severity level
	title       string            // Human-readable alert title
	description string            // Detailed alert description
	labels      map[string]string // Key-value metadata labels
}

// ID returns the unique alert identifier.
func (a *AlertStub) ID() string { return a.id }

// Source returns the system that generated this alert.
func (a *AlertStub) Source() string { return a.source }

// Severity returns the alert severity level (e.g., "warning", "critical").
func (a *AlertStub) Severity() string { return a.severity }

// Title returns the human-readable alert title.
func (a *AlertStub) Title() string { return a.title }

// Description returns the detailed alert description.
func (a *AlertStub) Description() string { return a.description }

// Labels returns the metadata labels attached to this alert.
// The returned map should be treated as read-only.
func (a *AlertStub) Labels() map[string]string { return a.labels }

// IsCritical returns true if the alert severity is "critical".
func (a *AlertStub) IsCritical() bool { return a.severity == "critical" }

// LabelValue returns the value of a specific label, or empty string if not found.
func (a *AlertStub) LabelValue(key string) string { return a.labels[key] }

// InvestigationPromptBuilder generates prompts for AI-driven alert investigation.
// Each builder is specialized for a specific alert type and generates prompts
// with appropriate investigation steps and safety rules.
type InvestigationPromptBuilder interface {
	// BuildPrompt generates an investigation prompt for the given alert.
	// Returns ErrNilAlert if alert is nil.
	BuildPrompt(alert *AlertStub) (string, error)
	// AlertType returns the type of alerts this builder handles (e.g., "HighCPU", "DiskSpace").
	AlertType() string
}

// PromptBuilderRegistry manages a collection of prompt builders and routes
// alerts to the appropriate builder based on alert type or content.
type PromptBuilderRegistry interface {
	// Register adds a prompt builder to the registry. Returns ErrNilPromptBuilder if nil.
	Register(builder InvestigationPromptBuilder) error
	// Get retrieves a builder by alert type. Returns ErrPromptBuilderNotFound if not found.
	Get(alertType string) (InvestigationPromptBuilder, error)
	// BuildPromptForAlert finds the appropriate builder and generates a prompt.
	// Falls back to Generic builder if no specific builder is found.
	BuildPromptForAlert(alert *AlertStub) (string, error)
	// ListAlertTypes returns all registered alert types.
	ListAlertTypes() []string
}

// HighCPUPromptBuilder generates investigation prompts for high CPU usage alerts.
// It guides the AI to check process CPU consumption and identify runaway processes.
type HighCPUPromptBuilder struct{}

// NewHighCPUPromptBuilder creates a new HighCPUPromptBuilder instance.
func NewHighCPUPromptBuilder() *HighCPUPromptBuilder {
	return &HighCPUPromptBuilder{}
}

// AlertType returns "HighCPU" as the alert type this builder handles.
func (b *HighCPUPromptBuilder) AlertType() string {
	return "HighCPU"
}

// BuildPrompt generates an investigation prompt for high CPU alerts.
// Uses the "instance" label from the alert if available.
// Returns ErrNilAlert if alert is nil.
func (b *HighCPUPromptBuilder) BuildPrompt(alert *AlertStub) (string, error) {
	if alert == nil {
		return "", ErrNilAlert
	}

	instance := alert.LabelValue("instance")
	if instance == "" {
		instance = "unknown"
	}

	alertDetails := fmt.Sprintf(`## Alert Details
- Type: High CPU Usage
- Severity: %s
- Instance: %s
- Title: %s

## Suggested Investigation Steps
1. Run: top -b -n 1 | head -20
2. Run: ps aux --sort=-%%cpu | head -20
3. Look for runaway processes or high load

Begin your investigation now using the bash tool.`,
		alert.Severity(), instance, alert.Title())

	return investigationToolsHeader + alertDetails, nil
}

// DiskSpacePromptBuilder generates investigation prompts for low disk space alerts.
// It guides the AI to check filesystem usage and identify large files/directories.
type DiskSpacePromptBuilder struct{}

// NewDiskSpacePromptBuilder creates a new DiskSpacePromptBuilder instance.
func NewDiskSpacePromptBuilder() *DiskSpacePromptBuilder {
	return &DiskSpacePromptBuilder{}
}

// AlertType returns "DiskSpace" as the alert type this builder handles.
func (b *DiskSpacePromptBuilder) AlertType() string {
	return "DiskSpace"
}

// BuildPrompt generates an investigation prompt for disk space alerts.
// Uses the "mountpoint" label from the alert, defaulting to "/" if not present.
// Returns ErrNilAlert if alert is nil.
func (b *DiskSpacePromptBuilder) BuildPrompt(alert *AlertStub) (string, error) {
	if alert == nil {
		return "", ErrNilAlert
	}

	mountpoint := alert.LabelValue("mountpoint")
	if mountpoint == "" {
		mountpoint = "/"
	}

	alertDetails := fmt.Sprintf(`## Alert Details
- Type: Low Disk Space
- Severity: %s
- Mountpoint: %s
- Title: %s

## Suggested Investigation Steps
1. Run: df -h
2. Run: du -sh /* 2>/dev/null | sort -hr | head -10
3. Check for large log files that can be rotated

Begin your investigation now using the bash tool.`,
		alert.Severity(), mountpoint, alert.Title())

	return investigationToolsHeader + alertDetails, nil
}

// MemoryPromptBuilder generates investigation prompts for high memory usage alerts.
// It guides the AI to check memory consumption and identify memory-hungry processes.
type MemoryPromptBuilder struct{}

// NewMemoryPromptBuilder creates a new MemoryPromptBuilder instance.
func NewMemoryPromptBuilder() *MemoryPromptBuilder {
	return &MemoryPromptBuilder{}
}

// AlertType returns "HighMemory" as the alert type this builder handles.
func (b *MemoryPromptBuilder) AlertType() string {
	return "HighMemory"
}

// BuildPrompt generates an investigation prompt for high memory alerts.
// Returns ErrNilAlert if alert is nil.
func (b *MemoryPromptBuilder) BuildPrompt(alert *AlertStub) (string, error) {
	if alert == nil {
		return "", ErrNilAlert
	}

	alertDetails := fmt.Sprintf(`## Alert Details
- Type: High Memory Usage
- Severity: %s
- Title: %s

## Suggested Investigation Steps
1. Run: free -h
2. Run: ps aux --sort=-rss | head -20
3. Check for memory leaks or unusual consumption patterns

Begin your investigation now using the bash tool.`,
		alert.Severity(), alert.Title())

	return investigationToolsHeader + alertDetails, nil
}

// OOMPromptBuilder generates investigation prompts for OOM (Out of Memory) killed alerts.
// It guides the AI to check kernel logs and identify what triggered the OOM killer.
type OOMPromptBuilder struct{}

// NewOOMPromptBuilder creates a new OOMPromptBuilder instance.
func NewOOMPromptBuilder() *OOMPromptBuilder {
	return &OOMPromptBuilder{}
}

// AlertType returns "OOMKilled" as the alert type this builder handles.
func (b *OOMPromptBuilder) AlertType() string {
	return "OOMKilled"
}

// BuildPrompt generates an investigation prompt for OOM killed alerts.
// OOM events are considered critical and the prompt suggests escalation.
// Returns ErrNilAlert if alert is nil.
func (b *OOMPromptBuilder) BuildPrompt(alert *AlertStub) (string, error) {
	if alert == nil {
		return "", ErrNilAlert
	}

	alertDetails := fmt.Sprintf(`## Alert Details
- Type: OOM (Out of Memory) Killed
- Severity: %s
- Title: %s

## Suggested Investigation Steps
1. Run: dmesg | grep -i oom | tail -20
2. Run: journalctl -k | grep -i oom | tail -20
3. Run: free -h
4. Identify which process was killed and why

This is a critical event. If you cannot determine the root cause, escalate.

Begin your investigation now using the bash tool.`,
		alert.Severity(), alert.Title())

	return investigationToolsHeader + alertDetails, nil
}

// GenericPromptBuilder generates investigation prompts for alerts with no specific builder.
// It provides a general-purpose template that works with any alert type.
type GenericPromptBuilder struct{}

// NewGenericPromptBuilder creates a new GenericPromptBuilder instance.
func NewGenericPromptBuilder() *GenericPromptBuilder {
	return &GenericPromptBuilder{}
}

// AlertType returns "Generic" as the alert type this builder handles.
// This builder serves as a fallback when no specialized builder is available.
func (b *GenericPromptBuilder) AlertType() string {
	return "Generic"
}

// BuildPrompt generates a generic investigation prompt using all available alert fields.
// Returns ErrNilAlert if alert is nil.
func (b *GenericPromptBuilder) BuildPrompt(alert *AlertStub) (string, error) {
	if alert == nil {
		return "", ErrNilAlert
	}

	alertDetails := fmt.Sprintf(`## Alert Details
- ID: %s
- Source: %s
- Severity: %s
- Title: %s
- Description: %s

## Suggested Investigation Steps
1. Gather relevant system information (uptime, load, memory, disk)
2. Check logs for related errors
3. Identify potential root causes

Begin your investigation now using the bash tool.`,
		alert.ID(), alert.Source(), alert.Severity(), alert.Title(), alert.Description())

	return investigationToolsHeader + alertDetails, nil
}

// DefaultPromptBuilderRegistry is the default implementation of PromptBuilderRegistry.
// It stores builders in a map keyed by alert type and provides fallback logic
// to find appropriate builders based on alert labels and title.
type DefaultPromptBuilderRegistry struct {
	builders map[string]InvestigationPromptBuilder
}

// NewPromptBuilderRegistry creates a new empty registry.
// Use Register to add builders for specific alert types.
func NewPromptBuilderRegistry() *DefaultPromptBuilderRegistry {
	return &DefaultPromptBuilderRegistry{
		builders: make(map[string]InvestigationPromptBuilder),
	}
}

// Register adds a prompt builder to the registry.
// The builder is indexed by its AlertType() return value.
// Registering a builder with the same type replaces the existing one.
// Returns ErrNilPromptBuilder if builder is nil.
func (r *DefaultPromptBuilderRegistry) Register(builder InvestigationPromptBuilder) error {
	if builder == nil {
		return ErrNilPromptBuilder
	}
	r.builders[builder.AlertType()] = builder
	return nil
}

// Get retrieves a builder by exact alert type match.
// Returns ErrPromptBuilderNotFound if no builder is registered for the type.
func (r *DefaultPromptBuilderRegistry) Get(alertType string) (InvestigationPromptBuilder, error) {
	builder, exists := r.builders[alertType]
	if !exists {
		return nil, ErrPromptBuilderNotFound
	}
	return builder, nil
}

// BuildPromptForAlert finds the appropriate builder and generates a prompt.
// Builder selection order:
//  1. Exact match on "alertname" label
//  2. Substring match in alert title
//  3. Fallback to "Generic" builder if registered
//
// Returns ErrNilAlert if alert is nil.
// Returns ErrPromptBuilderNotFound if no suitable builder is found.
func (r *DefaultPromptBuilderRegistry) BuildPromptForAlert(alert *AlertStub) (string, error) {
	if alert == nil {
		return "", ErrNilAlert
	}

	// Try to find builder by alertname label
	alertName := alert.LabelValue("alertname")
	if builder, exists := r.builders[alertName]; exists {
		return builder.BuildPrompt(alert)
	}

	// Try to match by title
	for alertType, builder := range r.builders {
		if strings.Contains(alert.Title(), alertType) {
			return builder.BuildPrompt(alert)
		}
	}

	// Fall back to Generic if available
	if builder, exists := r.builders["Generic"]; exists {
		return builder.BuildPrompt(alert)
	}

	return "", ErrPromptBuilderNotFound
}

// ListAlertTypes returns the list of all registered alert types.
// The order of types is undefined (map iteration order).
func (r *DefaultPromptBuilderRegistry) ListAlertTypes() []string {
	types := make([]string, 0, len(r.builders))
	for t := range r.builders {
		types = append(types, t)
	}
	return types
}

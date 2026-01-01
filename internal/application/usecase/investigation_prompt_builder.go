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

	prompt := fmt.Sprintf(`Investigate high CPU usage alert.

Alert Details:
- Severity: %s
- Instance: %s
- Title: %s

Investigation Steps:
1. Use 'top -b -n 1' to check current CPU usage and identify top processes
2. Check process details with 'ps aux --sort=-%%cpu | head -20'
3. Look for any runaway processes or high load

Safety Rules:
- DO NOT restart or kill any processes without confirmation
- Only use safe, read-only commands for investigation
- If you cannot determine the root cause, escalate to human operator

Report your findings and recommendations.`,
		alert.Severity(), instance, alert.Title())

	return prompt, nil
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

	prompt := fmt.Sprintf(`Investigate low disk space alert.

Alert Details:
- Severity: %s
- Mountpoint: %s
- Title: %s

Investigation Steps:
1. Use 'df -h' to check disk usage across all filesystems
2. Use 'du -sh /*' to find large directories
3. Check for large log files that can be rotated or cleaned

Safety Rules:
- DO NOT delete any files without confirmation
- Only use safe, read-only commands for investigation
- If space is critically low, escalate immediately

Report your findings and recommendations.`,
		alert.Severity(), mountpoint, alert.Title())

	return prompt, nil
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

	prompt := fmt.Sprintf(`Investigate high memory usage alert.

Alert Details:
- Severity: %s
- Title: %s

Investigation Steps:
1. Use 'free -h' to check overall memory usage
2. Use 'ps aux --sort=-rss | head -20' to find top memory consumers
3. Check for memory leaks or unusual consumption patterns

Safety Rules:
- DO NOT kill any processes without confirmation
- Only use safe, read-only commands for investigation
- If memory is critically low, escalate immediately

Report your findings and recommendations.`,
		alert.Severity(), alert.Title())

	return prompt, nil
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

	prompt := fmt.Sprintf(`Investigate OOM (Out of Memory) killed process alert.

Alert Details:
- Severity: %s
- Title: %s

Investigation Steps:
1. Check dmesg for OOM killer messages: 'dmesg | grep -i oom'
2. Review journal logs: 'journalctl -k | grep -i oom'
3. Check current memory state with 'free -h'
4. Identify memory limit configurations

Safety Rules:
- DO NOT modify any memory limits without confirmation
- Only use safe, read-only commands for investigation
- This is a critical event - consider escalating

Report your findings and recommendations.`,
		alert.Severity(), alert.Title())

	return prompt, nil
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

	prompt := fmt.Sprintf(`Investigate the following alert.

Alert Details:
- ID: %s
- Source: %s
- Severity: %s
- Title: %s
- Description: %s

Investigation Steps:
1. Gather relevant system information
2. Check logs for related errors
3. Identify potential root causes

Safety Rules:
- DO NOT make any changes without confirmation
- Only use safe, read-only commands for investigation
- If unsure, escalate to human operator

Report your findings and recommendations.`,
		alert.ID(), alert.Source(), alert.Severity(), alert.Title(), alert.Description())

	return prompt, nil
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

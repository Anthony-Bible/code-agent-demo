// Package usecase contains application use cases.
package usecase

import (
	"errors"
	"fmt"
	"strings"
)

// Sentinel errors for prompt builders.
var (
	ErrNilAlert               = errors.New("alert cannot be nil")
	ErrUnknownAlertType       = errors.New("unknown alert type")
	ErrPromptBuilderNotFound  = errors.New("prompt builder not found for alert type")
	ErrEmptyPromptTemplate    = errors.New("prompt template cannot be empty")
	ErrInvalidPromptVariables = errors.New("missing required prompt variables")
	ErrNilPromptBuilder       = errors.New("prompt builder cannot be nil")
)

// AlertStub represents a minimal alert for prompt building.
type AlertStub struct {
	id          string
	source      string
	severity    string
	title       string
	description string
	labels      map[string]string
}

// ID returns the alert ID.
func (a *AlertStub) ID() string { return a.id }

// Source returns the source.
func (a *AlertStub) Source() string { return a.source }

// Severity returns the severity.
func (a *AlertStub) Severity() string { return a.severity }

// Title returns the title.
func (a *AlertStub) Title() string { return a.title }

// Description returns the description.
func (a *AlertStub) Description() string { return a.description }

// Labels returns the labels.
func (a *AlertStub) Labels() map[string]string { return a.labels }

// IsCritical returns true if severity is critical.
func (a *AlertStub) IsCritical() bool { return a.severity == "critical" }

// LabelValue returns a label value by key.
func (a *AlertStub) LabelValue(key string) string { return a.labels[key] }

// InvestigationPromptBuilder generates prompts for AI investigation.
type InvestigationPromptBuilder interface {
	BuildPrompt(alert *AlertStub) (string, error)
	AlertType() string
}

// PromptBuilderRegistry manages multiple prompt builders.
type PromptBuilderRegistry interface {
	Register(builder InvestigationPromptBuilder) error
	Get(alertType string) (InvestigationPromptBuilder, error)
	BuildPromptForAlert(alert *AlertStub) (string, error)
	ListAlertTypes() []string
}

// HighCPUPromptBuilder builds prompts for high CPU alerts.
type HighCPUPromptBuilder struct{}

// NewHighCPUPromptBuilder creates a new HighCPUPromptBuilder.
func NewHighCPUPromptBuilder() *HighCPUPromptBuilder {
	return &HighCPUPromptBuilder{}
}

// AlertType returns the alert type this builder handles.
func (b *HighCPUPromptBuilder) AlertType() string {
	return "HighCPU"
}

// BuildPrompt builds a prompt for high CPU investigation.
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

// DiskSpacePromptBuilder builds prompts for disk space alerts.
type DiskSpacePromptBuilder struct{}

// NewDiskSpacePromptBuilder creates a new DiskSpacePromptBuilder.
func NewDiskSpacePromptBuilder() *DiskSpacePromptBuilder {
	return &DiskSpacePromptBuilder{}
}

// AlertType returns the alert type this builder handles.
func (b *DiskSpacePromptBuilder) AlertType() string {
	return "DiskSpace"
}

// BuildPrompt builds a prompt for disk space investigation.
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

// MemoryPromptBuilder builds prompts for memory alerts.
type MemoryPromptBuilder struct{}

// NewMemoryPromptBuilder creates a new MemoryPromptBuilder.
func NewMemoryPromptBuilder() *MemoryPromptBuilder {
	return &MemoryPromptBuilder{}
}

// AlertType returns the alert type this builder handles.
func (b *MemoryPromptBuilder) AlertType() string {
	return "HighMemory"
}

// BuildPrompt builds a prompt for memory investigation.
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

// OOMPromptBuilder builds prompts for OOM alerts.
type OOMPromptBuilder struct{}

// NewOOMPromptBuilder creates a new OOMPromptBuilder.
func NewOOMPromptBuilder() *OOMPromptBuilder {
	return &OOMPromptBuilder{}
}

// AlertType returns the alert type this builder handles.
func (b *OOMPromptBuilder) AlertType() string {
	return "OOMKilled"
}

// BuildPrompt builds a prompt for OOM investigation.
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

// GenericPromptBuilder builds prompts for unknown alert types.
type GenericPromptBuilder struct{}

// NewGenericPromptBuilder creates a new GenericPromptBuilder.
func NewGenericPromptBuilder() *GenericPromptBuilder {
	return &GenericPromptBuilder{}
}

// AlertType returns the alert type this builder handles.
func (b *GenericPromptBuilder) AlertType() string {
	return "Generic"
}

// BuildPrompt builds a generic investigation prompt.
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

// DefaultPromptBuilderRegistry manages prompt builders.
type DefaultPromptBuilderRegistry struct {
	builders map[string]InvestigationPromptBuilder
}

// NewPromptBuilderRegistry creates a new registry.
func NewPromptBuilderRegistry() *DefaultPromptBuilderRegistry {
	return &DefaultPromptBuilderRegistry{
		builders: make(map[string]InvestigationPromptBuilder),
	}
}

// Register adds a builder to the registry.
func (r *DefaultPromptBuilderRegistry) Register(builder InvestigationPromptBuilder) error {
	if builder == nil {
		return ErrNilPromptBuilder
	}
	r.builders[builder.AlertType()] = builder
	return nil
}

// Get retrieves a builder by alert type.
func (r *DefaultPromptBuilderRegistry) Get(alertType string) (InvestigationPromptBuilder, error) {
	builder, exists := r.builders[alertType]
	if !exists {
		return nil, ErrPromptBuilderNotFound
	}
	return builder, nil
}

// BuildPromptForAlert builds a prompt using the appropriate builder.
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

// ListAlertTypes returns all registered alert types.
func (r *DefaultPromptBuilderRegistry) ListAlertTypes() []string {
	types := make([]string, 0, len(r.builders))
	for t := range r.builders {
		types = append(types, t)
	}
	return types
}

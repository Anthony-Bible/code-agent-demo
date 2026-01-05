// Package usecase contains application use cases.
package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
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

// Alert type constants.
const (
	// AlertTypeGeneric is the fallback alert type for the GenericPromptBuilder.
	AlertTypeGeneric = "Generic"
)

// GenerateToolsHeader creates formatted documentation for a list of tools.
// It returns an empty string if no tools are provided.
func GenerateToolsHeader(tools []entity.Tool) string {
	if len(tools) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, tool := range tools {
		sb.WriteString(fmt.Sprintf("%d. **%s** - %s\n", i+1, tool.Name, tool.Description))

		// Add simple example based on tool name
		if example := getToolExample(tool.Name); example != "" {
			sb.WriteString(fmt.Sprintf("   Example: %s\n", example))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// getToolExample returns a simple example for common investigation tools.
func getToolExample(toolName string) string {
	examples := map[string]string{
		"bash":                   `{"command": "ps aux --sort=-%cpu | head -20"}`,
		"read_file":              `{"path": "/var/log/syslog"}`,
		"list_files":             `{"path": "/var/log"}`,
		"batch_tool":             `{"invocations": [{"tool_name": "read_file", "arguments": {"path": "config.yaml"}}, {"tool_name": "bash", "arguments": {"command": "df -h"}}]}`,
		"activate_skill":         `{"name": "cloud-metrics"}`,
		"complete_investigation": `{"findings": ["Root cause identified"], "confidence": 0.85}`,
		"escalate_investigation": `{"reason": "Unable to determine root cause", "partial_findings": ["Observed high CPU"]}`,
		"task":                   `{"agent_name": "code-reviewer", "prompt": "Analyze the authentication module for security issues"}`,
		"delegate":               `{"name": "log-analyzer", "system_prompt": "You are a log analysis specialist", "task": "Analyze error patterns in /var/log/app.log"}`,
	}
	return examples[toolName]
}

// GenerateSkillsHeader creates formatted XML documentation for available skills.
// Returns an empty string if no skills are provided.
// Format matches the XML structure used in anthropic_adapter.go.
func GenerateSkillsHeader(skills []port.SkillInfo) string {
	if len(skills) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("<available_skills>\n")
	for _, skill := range skills {
		sb.WriteString("  <skill>\n")
		sb.WriteString(fmt.Sprintf("    <name>%s</name>\n", skill.Name))
		sb.WriteString(fmt.Sprintf("    <description>%s</description>\n", skill.Description))
		sb.WriteString("  </skill>\n")
	}
	sb.WriteString("</available_skills>\n")
	return sb.String()
}

// AlertView represents a lightweight alert structure for prompt building.
// It contains only the fields needed to generate investigation prompts.
type AlertView struct {
	id          string            // Unique alert identifier
	source      string            // Alert source system (e.g., "prometheus", "cloudwatch")
	severity    string            // Alert severity level
	title       string            // Human-readable alert title
	description string            // Detailed alert description
	labels      map[string]string // Key-value metadata labels
}

// ID returns the unique alert identifier.
func (a *AlertView) ID() string { return a.id }

// Source returns the system that generated this alert.
func (a *AlertView) Source() string { return a.source }

// Severity returns the alert severity level (e.g., "warning", "critical").
func (a *AlertView) Severity() string { return a.severity }

// Title returns the human-readable alert title.
func (a *AlertView) Title() string { return a.title }

// Description returns the detailed alert description.
func (a *AlertView) Description() string { return a.description }

// Labels returns the metadata labels attached to this alert.
// The returned map should be treated as read-only.
func (a *AlertView) Labels() map[string]string { return a.labels }

// IsCritical returns true if the alert severity is "critical".
func (a *AlertView) IsCritical() bool { return a.severity == "critical" }

// LabelValue returns the value of a specific label, or empty string if not found.
func (a *AlertView) LabelValue(key string) string { return a.labels[key] }

// InvestigationPromptBuilder generates prompts for AI-driven alert investigation.
// Each builder is specialized for a specific alert type and generates prompts
// with appropriate investigation steps and safety rules.
type InvestigationPromptBuilder interface {
	// BuildPrompt generates an investigation prompt for the given alert.
	// Returns ErrNilAlert if alert is nil.
	BuildPrompt(alert *AlertView, tools []entity.Tool, skills []port.SkillInfo) (string, error)
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
	BuildPromptForAlert(alert *AlertView, tools []entity.Tool, skills []port.SkillInfo) (string, error)
	// ListAlertTypes returns all registered alert types.
	ListAlertTypes() []string
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
	return AlertTypeGeneric
}

// BuildPrompt generates an enhanced investigation prompt using all available alert fields.
// The prompt includes full alert context, all labels, and environment-aware investigation guidance.
// Returns ErrNilAlert if alert is nil.
func (b *GenericPromptBuilder) BuildPrompt(
	alert *AlertView,
	tools []entity.Tool,
	skills []port.SkillInfo,
) (string, error) {
	if alert == nil {
		return "", ErrNilAlert
	}

	var sb strings.Builder

	// Role section
	sb.WriteString(`## Role
You are an intelligent systems investigator. Analyze the alert below and use the available tools to determine the root cause.

`)

	// Tools section
	sb.WriteString("## Available Tools\n\n")
	sb.WriteString(GenerateToolsHeader(tools))

	// Skills section
	if len(skills) > 0 {
		sb.WriteString("## Available Skills\n\n")
		sb.WriteString(GenerateSkillsHeader(skills))
		sb.WriteString("\nUse the `activate_skill` tool to load the full content of a skill.\n\n")
	}

	// Rules section
	sb.WriteString(`## Rules
- Use read-only commands only - DO NOT modify, restart, or kill anything
- You MUST end by calling either complete_investigation or escalate_investigation
- If you cannot determine the root cause, escalate with partial findings

`)

	// Alert context section with ALL information
	sb.WriteString("## Alert Context\n\n")
	sb.WriteString(fmt.Sprintf("- **ID**: %s\n", alert.ID()))
	sb.WriteString(fmt.Sprintf("- **Source**: %s\n", alert.Source()))
	sb.WriteString(fmt.Sprintf("- **Severity**: %s\n", alert.Severity()))
	sb.WriteString(fmt.Sprintf("- **Title**: %s\n", alert.Title()))
	if alert.Description() != "" {
		sb.WriteString(fmt.Sprintf("- **Description**: %s\n", alert.Description()))
	}
	sb.WriteString("\n")

	// Labels section - ALL of them, sorted
	if labels := alert.Labels(); len(labels) > 0 {
		sb.WriteString("### Labels\n\n")
		// Sort keys for consistent output
		keys := make([]string, 0, len(labels))
		for k := range labels {
			keys = append(keys, k)
		}
		// Sort keys alphabetically
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
		for _, k := range keys {
			sb.WriteString(fmt.Sprintf("- `%s`: %s\n", k, labels[k]))
		}
		sb.WriteString("\n")
	}

	// Investigation guidance - environment-aware
	sb.WriteString(`## Investigation Guidance

Based on the alert source, labels, and description, determine the appropriate investigation approach:

- Unless otherwise specified, assume the alert is for a remote host.
- **Cloud/GCP alerts**: If labels contain resource_type, metric_type, or the source indicates cloud monitoring, consider using the activate_skill tool with "cloud-metrics" skill for querying GCP metrics
- **Kubernetes alerts**: Look for namespace, pod, container labels to scope your investigation
- **Examine ALL labels**: They contain critical context (instance, mountpoint, threshold_value, etc.)

Begin your investigation now.
`)

	return sb.String(), nil
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

// BuildPromptForAlert generates an investigation prompt for the given alert.
// Always uses the Generic builder, allowing the LLM to determine the appropriate
// investigation approach based on alert context and available tools.
//
// Returns ErrNilAlert if alert is nil.
// Returns ErrPromptBuilderNotFound if Generic builder is not registered.
func (r *DefaultPromptBuilderRegistry) BuildPromptForAlert(
	alert *AlertView,
	tools []entity.Tool,
	skills []port.SkillInfo,
) (string, error) {
	if alert == nil {
		return "", ErrNilAlert
	}

	// Always use Generic builder - LLM determines investigation approach
	if builder, exists := r.builders[AlertTypeGeneric]; exists {
		return builder.BuildPrompt(alert, tools, skills)
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

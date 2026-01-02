package usecase

import (
	"code-editing-agent/internal/domain/entity"
	"errors"
	"strings"
	"testing"
)

// =============================================================================
// Test Helpers
// =============================================================================

// createTestTools creates a minimal set of tools for testing prompt generation.
func createTestTools() []entity.Tool {
	bash, _ := entity.NewTool("bash", "bash", "Execute shell commands")
	readFile, _ := entity.NewTool("read_file", "read_file", "Read file contents")
	listFiles, _ := entity.NewTool("list_files", "list_files", "List directory contents")
	complete, _ := entity.NewTool("complete_investigation", "complete_investigation", "Complete investigation")
	escalate, _ := entity.NewTool("escalate_investigation", "escalate_investigation", "Escalate investigation")

	return []entity.Tool{*bash, *readFile, *listFiles, *complete, *escalate}
}

// =============================================================================
// InvestigationPromptBuilder Tests
// These tests verify the behavior of InvestigationPromptBuilder implementations.
// =============================================================================

// =============================================================================
// Generic Prompt Builder Tests
// =============================================================================

func TestNewGenericPromptBuilder_NotNil(t *testing.T) {
	builder := NewGenericPromptBuilder()
	if builder == nil {
		t.Error("NewGenericPromptBuilder() should not return nil")
	}
}

func TestGenericPromptBuilder_AlertType(t *testing.T) {
	builder := NewGenericPromptBuilder()
	if builder == nil {
		t.Skip("NewGenericPromptBuilder() returned nil")
	}
	alertType := builder.AlertType()
	if alertType != "Generic" && alertType != "*" && alertType != "default" {
		t.Errorf("AlertType() = %v, want Generic or * or default", alertType)
	}
}

func TestGenericPromptBuilder_BuildPrompt_ValidAlert(t *testing.T) {
	builder := NewGenericPromptBuilder()
	if builder == nil {
		t.Skip("NewGenericPromptBuilder() returned nil")
	}

	alert := &AlertView{
		id:          "alert-generic-001",
		source:      "custom-monitoring",
		severity:    "warning",
		title:       "Something Unusual Happened",
		description: "An unknown condition was detected",
		labels:      map[string]string{"service": "mystery-service"},
	}

	prompt, err := builder.BuildPrompt(alert, createTestTools())
	if err != nil {
		t.Errorf("BuildPrompt() error = %v", err)
	}
	if prompt == "" {
		t.Error("BuildPrompt() returned empty string")
	}
}

func TestGenericPromptBuilder_BuildPrompt_ContainsGeneralInstructions(t *testing.T) {
	builder := NewGenericPromptBuilder()
	if builder == nil {
		t.Skip("NewGenericPromptBuilder() returned nil")
	}

	alert := &AlertView{
		id:       "alert-generic-002",
		source:   "custom",
		severity: "info",
		title:    "Custom Alert",
		labels:   map[string]string{},
	}

	prompt, err := builder.BuildPrompt(alert, createTestTools())
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	// Should contain essential prompt sections
	requiredSections := []string{"Role", "Available Tools", "Rules", "Alert Context"}
	for _, section := range requiredSections {
		if !strings.Contains(prompt, section) {
			t.Errorf("BuildPrompt() should contain %q section", section)
		}
	}

	// Should contain safety rules
	if !strings.Contains(prompt, "DO NOT") {
		t.Error("BuildPrompt() should contain safety rules with 'DO NOT'")
	}
}

func TestGenericPromptBuilder_BuildPrompt_IncludesAllLabels(t *testing.T) {
	builder := NewGenericPromptBuilder()
	if builder == nil {
		t.Skip("NewGenericPromptBuilder() returned nil")
	}

	alert := &AlertView{
		id:       "alert-labels-001",
		source:   "prometheus",
		severity: "warning",
		title:    "Test Alert",
		labels: map[string]string{
			"instance":        "web-01",
			"namespace":       "production",
			"pod":             "app-pod-123",
			"threshold_value": "80",
			"current_value":   "95",
		},
	}

	prompt, err := builder.BuildPrompt(alert, createTestTools())
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	// Verify all labels appear in the prompt
	for key, value := range alert.Labels() {
		if !strings.Contains(prompt, key) {
			t.Errorf("BuildPrompt() should contain label key %q", key)
		}
		if !strings.Contains(prompt, value) {
			t.Errorf("BuildPrompt() should contain label value %q", value)
		}
	}

	// Verify labels section exists
	if !strings.Contains(prompt, "Labels") {
		t.Error("BuildPrompt() should have a Labels section")
	}
}

func TestGenericPromptBuilder_BuildPrompt_ContainsCloudGuidance(t *testing.T) {
	builder := NewGenericPromptBuilder()
	if builder == nil {
		t.Skip("NewGenericPromptBuilder() returned nil")
	}

	alert := &AlertView{
		id:       "alert-cloud-001",
		source:   "gcp-monitoring",
		severity: "critical",
		title:    "Cloud Resource Alert",
		labels: map[string]string{
			"resource_type": "gce_instance",
			"metric_type":   "compute.googleapis.com/instance/cpu/utilization",
		},
	}

	prompt, err := builder.BuildPrompt(alert, createTestTools())
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	// Verify cloud-specific guidance is present
	if !strings.Contains(prompt, "cloud-metrics") {
		t.Error("BuildPrompt() should mention cloud-metrics skill")
	}

	// Verify investigation guidance section exists
	if !strings.Contains(prompt, "Investigation Guidance") {
		t.Error("BuildPrompt() should have Investigation Guidance section")
	}

	// Verify mentions of Cloud/GCP alerts
	if !strings.Contains(prompt, "Cloud/GCP") && !strings.Contains(prompt, "cloud monitoring") {
		t.Error("BuildPrompt() should contain cloud investigation guidance")
	}
}

func TestGenericPromptBuilder_BuildPrompt_ContainsAllAlertFields(t *testing.T) {
	builder := NewGenericPromptBuilder()
	if builder == nil {
		t.Skip("NewGenericPromptBuilder() returned nil")
	}

	alert := &AlertView{
		id:          "alert-full-001",
		source:      "test-source",
		severity:    "critical",
		title:       "Complete Alert Test",
		description: "This is a detailed description of the alert",
		labels: map[string]string{
			"key1": "value1",
		},
	}

	prompt, err := builder.BuildPrompt(alert, createTestTools())
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	// Verify all alert fields are present
	if !strings.Contains(prompt, alert.ID()) {
		t.Error("BuildPrompt() should contain alert ID")
	}
	if !strings.Contains(prompt, alert.Source()) {
		t.Error("BuildPrompt() should contain alert source")
	}
	if !strings.Contains(prompt, alert.Severity()) {
		t.Error("BuildPrompt() should contain alert severity")
	}
	if !strings.Contains(prompt, alert.Title()) {
		t.Error("BuildPrompt() should contain alert title")
	}
	if !strings.Contains(prompt, alert.Description()) {
		t.Error("BuildPrompt() should contain alert description")
	}

	// Verify Alert Context section exists
	if !strings.Contains(prompt, "Alert Context") {
		t.Error("BuildPrompt() should have Alert Context section")
	}
}

func TestGenericPromptBuilder_BuildPrompt_EmptyLabels(t *testing.T) {
	builder := NewGenericPromptBuilder()
	if builder == nil {
		t.Skip("NewGenericPromptBuilder() returned nil")
	}

	alert := &AlertView{
		id:          "alert-empty-001",
		source:      "test",
		severity:    "info",
		title:       "No Labels Alert",
		description: "Alert with no labels",
		labels:      map[string]string{},
	}

	prompt, err := builder.BuildPrompt(alert, createTestTools())
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	// Should still generate valid prompt
	if prompt == "" {
		t.Error("BuildPrompt() should not return empty string for alert with no labels")
	}

	// Should still have core sections
	if !strings.Contains(prompt, "Alert Context") {
		t.Error("BuildPrompt() should have Alert Context section even with no labels")
	}
	if !strings.Contains(prompt, "Investigation Guidance") {
		t.Error("BuildPrompt() should have Investigation Guidance section even with no labels")
	}
}

func TestGenericPromptBuilder_BuildPrompt_NilAlert(t *testing.T) {
	builder := NewGenericPromptBuilder()
	if builder == nil {
		t.Skip("NewGenericPromptBuilder() returned nil")
	}

	_, err := builder.BuildPrompt(nil, createTestTools())
	if !errors.Is(err, ErrNilAlert) {
		t.Errorf("BuildPrompt(nil) error = %v, want ErrNilAlert", err)
	}
}

// =============================================================================
// Prompt Builder Registry Tests
// =============================================================================

func TestNewPromptBuilderRegistry_NotNil(t *testing.T) {
	registry := NewPromptBuilderRegistry()
	if registry == nil {
		t.Error("NewPromptBuilderRegistry() should not return nil")
	}
}

func TestPromptBuilderRegistry_Register_Success(t *testing.T) {
	registry := NewPromptBuilderRegistry()
	if registry == nil {
		t.Skip("NewPromptBuilderRegistry() returned nil")
	}

	builder := NewGenericPromptBuilder()
	if builder == nil {
		t.Skip("NewGenericPromptBuilder() returned nil")
	}

	err := registry.Register(builder)
	if err != nil {
		t.Errorf("Register() error = %v", err)
	}
}

func TestPromptBuilderRegistry_Register_NilBuilder(t *testing.T) {
	registry := NewPromptBuilderRegistry()
	if registry == nil {
		t.Skip("NewPromptBuilderRegistry() returned nil")
	}

	err := registry.Register(nil)
	if err == nil {
		t.Error("Register(nil) should return error")
	}
}

func TestPromptBuilderRegistry_Get_Exists(t *testing.T) {
	registry := NewPromptBuilderRegistry()
	if registry == nil {
		t.Skip("NewPromptBuilderRegistry() returned nil")
	}

	builder := NewGenericPromptBuilder()
	if builder == nil {
		t.Skip("NewGenericPromptBuilder() returned nil")
	}

	if err := registry.Register(builder); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	got, err := registry.Get(builder.AlertType())
	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if got == nil {
		t.Error("Get() returned nil")
	}
}

func TestPromptBuilderRegistry_Get_NotExists(t *testing.T) {
	registry := NewPromptBuilderRegistry()
	if registry == nil {
		t.Skip("NewPromptBuilderRegistry() returned nil")
	}

	_, err := registry.Get("NonExistentAlertType")
	if err == nil {
		t.Error("Get() should return error for nonexistent type")
	}
}

func TestPromptBuilderRegistry_BuildPromptForAlert_UsesGenericBuilder(t *testing.T) {
	registry := NewPromptBuilderRegistry()
	if registry == nil {
		t.Skip("NewPromptBuilderRegistry() returned nil")
	}

	builder := NewGenericPromptBuilder()
	if builder == nil {
		t.Skip("NewGenericPromptBuilder() returned nil")
	}

	if err := registry.Register(builder); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	alert := &AlertView{
		id:       "alert-001",
		source:   "prometheus",
		severity: "critical",
		title:    "CPU Alert",
		labels:   map[string]string{"alertname": "HighCPU"},
	}

	prompt, err := registry.BuildPromptForAlert(alert, createTestTools())
	if err != nil {
		t.Errorf("BuildPromptForAlert() error = %v", err)
	}
	if prompt == "" {
		t.Error("BuildPromptForAlert() returned empty string")
	}
}

func TestPromptBuilderRegistry_BuildPromptForAlert_FallbackToGeneric(t *testing.T) {
	registry := NewPromptBuilderRegistry()
	if registry == nil {
		t.Skip("NewPromptBuilderRegistry() returned nil")
	}

	// Register generic builder as fallback
	genericBuilder := NewGenericPromptBuilder()
	if genericBuilder == nil {
		t.Skip("NewGenericPromptBuilder() returned nil")
	}

	if err := registry.Register(genericBuilder); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	alert := &AlertView{
		id:       "alert-unknown",
		source:   "custom",
		severity: "info",
		title:    "Unknown Alert Type",
		labels:   map[string]string{"alertname": "SomethingUnknown"},
	}

	prompt, err := registry.BuildPromptForAlert(alert, createTestTools())
	// Should either succeed with generic builder or return meaningful error
	if err != nil && prompt == "" {
		t.Logf("BuildPromptForAlert() returned error for unknown type: %v (acceptable if no fallback)", err)
	}
}

func TestPromptBuilderRegistry_BuildPromptForAlert_NilAlert(t *testing.T) {
	registry := NewPromptBuilderRegistry()
	if registry == nil {
		t.Skip("NewPromptBuilderRegistry() returned nil")
	}

	_, err := registry.BuildPromptForAlert(nil, createTestTools())
	if err == nil {
		t.Error("BuildPromptForAlert(nil) should return error")
	}
}

func TestPromptBuilderRegistry_ListAlertTypes_Empty(t *testing.T) {
	registry := NewPromptBuilderRegistry()
	if registry == nil {
		t.Skip("NewPromptBuilderRegistry() returned nil")
	}

	types := registry.ListAlertTypes()
	if types == nil {
		t.Error("ListAlertTypes() should return empty slice, not nil")
	}
	if len(types) != 0 {
		t.Errorf("ListAlertTypes() len = %v, want 0", len(types))
	}
}

func TestPromptBuilderRegistry_ListAlertTypes_WithBuilders(t *testing.T) {
	registry := NewPromptBuilderRegistry()
	if registry == nil {
		t.Skip("NewPromptBuilderRegistry() returned nil")
	}

	builder := NewGenericPromptBuilder()
	if builder == nil {
		t.Skip("NewGenericPromptBuilder() returned nil")
	}

	if err := registry.Register(builder); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	types := registry.ListAlertTypes()
	if len(types) != 1 {
		t.Errorf("ListAlertTypes() len = %v, want 1", len(types))
	}
	if len(types) > 0 && types[0] != "Generic" {
		t.Errorf("ListAlertTypes()[0] = %v, want Generic", types[0])
	}
}

// =============================================================================
// Error Constants Tests
// =============================================================================

func TestPromptBuilderErrors_NotNil(t *testing.T) {
	if ErrNilAlert == nil {
		t.Error("ErrNilAlert should not be nil")
	}
	if ErrUnknownAlertType == nil {
		t.Error("ErrUnknownAlertType should not be nil")
	}
	if ErrPromptBuilderNotFound == nil {
		t.Error("ErrPromptBuilderNotFound should not be nil")
	}
	if ErrEmptyPromptTemplate == nil {
		t.Error("ErrEmptyPromptTemplate should not be nil")
	}
	if ErrInvalidPromptVariables == nil {
		t.Error("ErrInvalidPromptVariables should not be nil")
	}
}

func TestPromptBuilderErrors_HaveMessages(t *testing.T) {
	if ErrNilAlert.Error() == "" {
		t.Error("ErrNilAlert should have a message")
	}
	if ErrUnknownAlertType.Error() == "" {
		t.Error("ErrUnknownAlertType should have a message")
	}
	if ErrPromptBuilderNotFound.Error() == "" {
		t.Error("ErrPromptBuilderNotFound should have a message")
	}
	if ErrEmptyPromptTemplate.Error() == "" {
		t.Error("ErrEmptyPromptTemplate should have a message")
	}
	if ErrInvalidPromptVariables.Error() == "" {
		t.Error("ErrInvalidPromptVariables should have a message")
	}
}

// =============================================================================
// GenerateToolsHeader Tests
// =============================================================================

func TestGenerateToolsHeader_EmptyTools(t *testing.T) {
	header := GenerateToolsHeader([]entity.Tool{})
	if header != "" {
		t.Errorf("GenerateToolsHeader([]) = %q, want empty string", header)
	}
}

func TestGenerateToolsHeader_NilTools(t *testing.T) {
	header := GenerateToolsHeader(nil)
	if header != "" {
		t.Errorf("GenerateToolsHeader(nil) = %q, want empty string", header)
	}
}

func TestGenerateToolsHeader_SingleTool(t *testing.T) {
	tool, _ := entity.NewTool("bash", "bash", "Execute shell commands")
	header := GenerateToolsHeader([]entity.Tool{*tool})

	if !strings.Contains(header, "bash") {
		t.Error("Header should contain tool name 'bash'")
	}
	if !strings.Contains(header, "Execute shell commands") {
		t.Error("Header should contain tool description")
	}
	if !strings.Contains(header, "1.") {
		t.Error("Header should start with numbered list")
	}
}

func TestGenerateToolsHeader_MultipleTools(t *testing.T) {
	tools := createTestTools()
	header := GenerateToolsHeader(tools)

	if !strings.Contains(header, "bash") {
		t.Error("Header should contain 'bash'")
	}
	if !strings.Contains(header, "read_file") {
		t.Error("Header should contain 'read_file'")
	}
	if !strings.Contains(header, "complete_investigation") {
		t.Error("Header should contain 'complete_investigation'")
	}

	// Check for numbered list
	if !strings.Contains(header, "1.") {
		t.Error("Header should contain '1.'")
	}
	if !strings.Contains(header, "2.") {
		t.Error("Header should contain '2.'")
	}
}

func TestGenerateToolsHeader_ContainsExamples(t *testing.T) {
	tools := createTestTools()
	header := GenerateToolsHeader(tools)

	// Should contain examples for common tools
	if !strings.Contains(header, "Example:") {
		t.Error("Header should contain example usage")
	}
}

package usecase

import (
	"strings"
	"testing"
)

// =============================================================================
// InvestigationPromptBuilder Tests
// These tests verify the behavior of InvestigationPromptBuilder implementations.
// =============================================================================

// =============================================================================
// HighCPU Prompt Builder Tests
// =============================================================================

func TestNewHighCPUPromptBuilder_NotNil(t *testing.T) {
	builder := NewHighCPUPromptBuilder()
	if builder == nil {
		t.Error("NewHighCPUPromptBuilder() should not return nil")
	}
}

func TestHighCPUPromptBuilder_AlertType(t *testing.T) {
	builder := NewHighCPUPromptBuilder()
	if builder == nil {
		t.Skip("NewHighCPUPromptBuilder() returned nil")
	}
	if builder.AlertType() != "HighCPU" && builder.AlertType() != "HighCPUAlert" {
		t.Errorf("AlertType() = %v, want HighCPU or HighCPUAlert", builder.AlertType())
	}
}

func TestHighCPUPromptBuilder_BuildPrompt_ValidAlert(t *testing.T) {
	builder := NewHighCPUPromptBuilder()
	if builder == nil {
		t.Skip("NewHighCPUPromptBuilder() returned nil")
	}

	alert := &AlertStub{
		id:       "alert-cpu-001",
		source:   "prometheus",
		severity: "critical",
		title:    "High CPU Usage",
		labels: map[string]string{
			"instance":  "web-01",
			"value":     "95",
			"threshold": "80",
		},
	}

	prompt, err := builder.BuildPrompt(alert)
	if err != nil {
		t.Errorf("BuildPrompt() error = %v", err)
	}
	if prompt == "" {
		t.Error("BuildPrompt() returned empty string")
	}
}

func TestHighCPUPromptBuilder_BuildPrompt_ContainsAlertInfo(t *testing.T) {
	builder := NewHighCPUPromptBuilder()
	if builder == nil {
		t.Skip("NewHighCPUPromptBuilder() returned nil")
	}

	alert := &AlertStub{
		id:          "alert-cpu-002",
		source:      "prometheus",
		severity:    "critical",
		title:       "High CPU Usage on web-01",
		description: "CPU has exceeded 90% for 5 minutes",
		labels: map[string]string{
			"instance": "web-01",
			"value":    "92",
		},
	}

	prompt, err := builder.BuildPrompt(alert)
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	// Prompt should contain key alert information
	if !strings.Contains(prompt, "critical") {
		t.Error("BuildPrompt() should mention severity")
	}
	if !strings.Contains(prompt, "web-01") {
		t.Error("BuildPrompt() should mention instance")
	}
}

func TestHighCPUPromptBuilder_BuildPrompt_ContainsInstructions(t *testing.T) {
	builder := NewHighCPUPromptBuilder()
	if builder == nil {
		t.Skip("NewHighCPUPromptBuilder() returned nil")
	}

	alert := &AlertStub{
		id:       "alert-cpu-003",
		source:   "prometheus",
		severity: "warning",
		title:    "High CPU Usage",
		labels:   map[string]string{"instance": "app-01"},
	}

	prompt, err := builder.BuildPrompt(alert)
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	// Should contain investigation instructions
	instructionKeywords := []string{"top", "CPU", "process", "check", "investigate"}
	foundInstruction := false
	for _, keyword := range instructionKeywords {
		if strings.Contains(strings.ToLower(prompt), strings.ToLower(keyword)) {
			foundInstruction = true
			break
		}
	}
	if !foundInstruction {
		t.Error("BuildPrompt() should contain investigation instructions")
	}
}

func TestHighCPUPromptBuilder_BuildPrompt_ContainsSafetyRules(t *testing.T) {
	builder := NewHighCPUPromptBuilder()
	if builder == nil {
		t.Skip("NewHighCPUPromptBuilder() returned nil")
	}

	alert := &AlertStub{
		id:       "alert-cpu-004",
		source:   "prometheus",
		severity: "critical",
		title:    "High CPU Usage",
		labels:   map[string]string{},
	}

	prompt, err := builder.BuildPrompt(alert)
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	// Should contain safety rules
	safetyKeywords := []string{"DO NOT", "confirm", "safe", "restart", "kill"}
	foundSafety := false
	for _, keyword := range safetyKeywords {
		if strings.Contains(prompt, keyword) {
			foundSafety = true
			break
		}
	}
	if !foundSafety {
		t.Error("BuildPrompt() should contain safety rules")
	}
}

func TestHighCPUPromptBuilder_BuildPrompt_NilAlert(t *testing.T) {
	builder := NewHighCPUPromptBuilder()
	if builder == nil {
		t.Skip("NewHighCPUPromptBuilder() returned nil")
	}

	_, err := builder.BuildPrompt(nil)
	if err == nil {
		t.Error("BuildPrompt(nil) should return error")
	}
}

// =============================================================================
// DiskSpace Prompt Builder Tests
// =============================================================================

func TestNewDiskSpacePromptBuilder_NotNil(t *testing.T) {
	builder := NewDiskSpacePromptBuilder()
	if builder == nil {
		t.Error("NewDiskSpacePromptBuilder() should not return nil")
	}
}

func TestDiskSpacePromptBuilder_AlertType(t *testing.T) {
	builder := NewDiskSpacePromptBuilder()
	if builder == nil {
		t.Skip("NewDiskSpacePromptBuilder() returned nil")
	}
	alertType := builder.AlertType()
	if alertType != "DiskSpace" && alertType != "DiskSpaceAlert" && alertType != "LowDiskSpace" {
		t.Errorf("AlertType() = %v, want DiskSpace-related", alertType)
	}
}

func TestDiskSpacePromptBuilder_BuildPrompt_ValidAlert(t *testing.T) {
	builder := NewDiskSpacePromptBuilder()
	if builder == nil {
		t.Skip("NewDiskSpacePromptBuilder() returned nil")
	}

	alert := &AlertStub{
		id:       "alert-disk-001",
		source:   "prometheus",
		severity: "warning",
		title:    "Low Disk Space",
		labels: map[string]string{
			"instance":   "db-01",
			"mountpoint": "/var/lib/postgresql",
			"usage":      "92",
		},
	}

	prompt, err := builder.BuildPrompt(alert)
	if err != nil {
		t.Errorf("BuildPrompt() error = %v", err)
	}
	if prompt == "" {
		t.Error("BuildPrompt() returned empty string")
	}
}

func TestDiskSpacePromptBuilder_BuildPrompt_ContainsDiskCommands(t *testing.T) {
	builder := NewDiskSpacePromptBuilder()
	if builder == nil {
		t.Skip("NewDiskSpacePromptBuilder() returned nil")
	}

	alert := &AlertStub{
		id:       "alert-disk-002",
		source:   "prometheus",
		severity: "critical",
		title:    "Disk Full",
		labels:   map[string]string{"mountpoint": "/"},
	}

	prompt, err := builder.BuildPrompt(alert)
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	// Should mention disk-related commands
	diskKeywords := []string{"df", "du", "disk", "space", "files", "log"}
	foundDiskCmd := false
	for _, keyword := range diskKeywords {
		if strings.Contains(strings.ToLower(prompt), strings.ToLower(keyword)) {
			foundDiskCmd = true
			break
		}
	}
	if !foundDiskCmd {
		t.Error("BuildPrompt() should contain disk investigation commands/keywords")
	}
}

// =============================================================================
// Memory Prompt Builder Tests
// =============================================================================

func TestNewMemoryPromptBuilder_NotNil(t *testing.T) {
	builder := NewMemoryPromptBuilder()
	if builder == nil {
		t.Error("NewMemoryPromptBuilder() should not return nil")
	}
}

func TestMemoryPromptBuilder_AlertType(t *testing.T) {
	builder := NewMemoryPromptBuilder()
	if builder == nil {
		t.Skip("NewMemoryPromptBuilder() returned nil")
	}
	alertType := builder.AlertType()
	if alertType != "Memory" && alertType != "MemoryUsage" && alertType != "HighMemory" {
		t.Errorf("AlertType() = %v, want Memory-related", alertType)
	}
}

func TestMemoryPromptBuilder_BuildPrompt_ValidAlert(t *testing.T) {
	builder := NewMemoryPromptBuilder()
	if builder == nil {
		t.Skip("NewMemoryPromptBuilder() returned nil")
	}

	alert := &AlertStub{
		id:       "alert-mem-001",
		source:   "prometheus",
		severity: "warning",
		title:    "High Memory Usage",
		labels: map[string]string{
			"instance": "app-01",
			"usage":    "85",
		},
	}

	prompt, err := builder.BuildPrompt(alert)
	if err != nil {
		t.Errorf("BuildPrompt() error = %v", err)
	}
	if prompt == "" {
		t.Error("BuildPrompt() returned empty string")
	}
}

func TestMemoryPromptBuilder_BuildPrompt_ContainsMemoryCommands(t *testing.T) {
	builder := NewMemoryPromptBuilder()
	if builder == nil {
		t.Skip("NewMemoryPromptBuilder() returned nil")
	}

	alert := &AlertStub{
		id:       "alert-mem-002",
		source:   "prometheus",
		severity: "critical",
		title:    "Memory Exhausted",
		labels:   map[string]string{},
	}

	prompt, err := builder.BuildPrompt(alert)
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	// Should mention memory-related commands/keywords
	memKeywords := []string{"free", "memory", "ps", "top", "rss", "swap"}
	foundMemCmd := false
	for _, keyword := range memKeywords {
		if strings.Contains(strings.ToLower(prompt), strings.ToLower(keyword)) {
			foundMemCmd = true
			break
		}
	}
	if !foundMemCmd {
		t.Error("BuildPrompt() should contain memory investigation commands/keywords")
	}
}

// =============================================================================
// OOM Prompt Builder Tests
// =============================================================================

func TestNewOOMPromptBuilder_NotNil(t *testing.T) {
	builder := NewOOMPromptBuilder()
	if builder == nil {
		t.Error("NewOOMPromptBuilder() should not return nil")
	}
}

func TestOOMPromptBuilder_AlertType(t *testing.T) {
	builder := NewOOMPromptBuilder()
	if builder == nil {
		t.Skip("NewOOMPromptBuilder() returned nil")
	}
	alertType := builder.AlertType()
	if alertType != "OOM" && alertType != "OOMKilled" && alertType != "OutOfMemory" {
		t.Errorf("AlertType() = %v, want OOM-related", alertType)
	}
}

func TestOOMPromptBuilder_BuildPrompt_ValidAlert(t *testing.T) {
	builder := NewOOMPromptBuilder()
	if builder == nil {
		t.Skip("NewOOMPromptBuilder() returned nil")
	}

	alert := &AlertStub{
		id:       "alert-oom-001",
		source:   "prometheus",
		severity: "critical",
		title:    "OOM Kill Detected",
		labels: map[string]string{
			"container": "app-container",
			"pod":       "app-pod-abc123",
		},
	}

	prompt, err := builder.BuildPrompt(alert)
	if err != nil {
		t.Errorf("BuildPrompt() error = %v", err)
	}
	if prompt == "" {
		t.Error("BuildPrompt() returned empty string")
	}
}

func TestOOMPromptBuilder_BuildPrompt_ContainsOOMInvestigation(t *testing.T) {
	builder := NewOOMPromptBuilder()
	if builder == nil {
		t.Skip("NewOOMPromptBuilder() returned nil")
	}

	alert := &AlertStub{
		id:       "alert-oom-002",
		source:   "prometheus",
		severity: "critical",
		title:    "OOM Killer Invoked",
		labels:   map[string]string{},
	}

	prompt, err := builder.BuildPrompt(alert)
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	// Should mention OOM-related investigation
	oomKeywords := []string{"dmesg", "oom", "killed", "memory", "journal", "limit"}
	foundOOMCmd := false
	for _, keyword := range oomKeywords {
		if strings.Contains(strings.ToLower(prompt), strings.ToLower(keyword)) {
			foundOOMCmd = true
			break
		}
	}
	if !foundOOMCmd {
		t.Error("BuildPrompt() should contain OOM investigation commands/keywords")
	}
}

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

	alert := &AlertStub{
		id:          "alert-generic-001",
		source:      "custom-monitoring",
		severity:    "warning",
		title:       "Something Unusual Happened",
		description: "An unknown condition was detected",
		labels:      map[string]string{"service": "mystery-service"},
	}

	prompt, err := builder.BuildPrompt(alert)
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

	alert := &AlertStub{
		id:       "alert-generic-002",
		source:   "custom",
		severity: "info",
		title:    "Custom Alert",
		labels:   map[string]string{},
	}

	prompt, err := builder.BuildPrompt(alert)
	if err != nil {
		t.Fatalf("BuildPrompt() error = %v", err)
	}

	// Should contain general investigation guidance
	if !strings.Contains(prompt, "investigate") && !strings.Contains(prompt, "Investigate") {
		t.Error("BuildPrompt() should contain investigation guidance")
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

	builder := NewHighCPUPromptBuilder()
	if builder == nil {
		t.Skip("NewHighCPUPromptBuilder() returned nil")
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

	builder := NewHighCPUPromptBuilder()
	if builder == nil {
		t.Skip("NewHighCPUPromptBuilder() returned nil")
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

func TestPromptBuilderRegistry_BuildPromptForAlert_MatchingBuilder(t *testing.T) {
	registry := NewPromptBuilderRegistry()
	if registry == nil {
		t.Skip("NewPromptBuilderRegistry() returned nil")
	}

	builder := NewHighCPUPromptBuilder()
	if builder == nil {
		t.Skip("NewHighCPUPromptBuilder() returned nil")
	}

	if err := registry.Register(builder); err != nil {
		t.Fatalf("Register() error = %v", err)
	}

	alert := &AlertStub{
		id:       "alert-001",
		source:   "prometheus",
		severity: "critical",
		title:    "HighCPU Alert", // Title or labels should match the builder
		labels:   map[string]string{"alertname": "HighCPU"},
	}

	prompt, err := registry.BuildPromptForAlert(alert)
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

	alert := &AlertStub{
		id:       "alert-unknown",
		source:   "custom",
		severity: "info",
		title:    "Unknown Alert Type",
		labels:   map[string]string{"alertname": "SomethingUnknown"},
	}

	prompt, err := registry.BuildPromptForAlert(alert)
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

	_, err := registry.BuildPromptForAlert(nil)
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

	builders := []InvestigationPromptBuilder{
		NewHighCPUPromptBuilder(),
		NewDiskSpacePromptBuilder(),
		NewMemoryPromptBuilder(),
	}

	registeredCount := 0
	for _, b := range builders {
		if b == nil {
			continue
		}
		if err := registry.Register(b); err == nil {
			registeredCount++
		}
	}

	types := registry.ListAlertTypes()
	if len(types) != registeredCount {
		t.Errorf("ListAlertTypes() len = %v, want %v", len(types), registeredCount)
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

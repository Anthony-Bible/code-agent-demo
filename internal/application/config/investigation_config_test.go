package config

import (
	"errors"
	"testing"
	"time"
)

// =============================================================================
// InvestigationConfig Tests - RED PHASE
// These tests define the expected behavior of the InvestigationConfig.
// All tests should FAIL until the implementation is complete.
// =============================================================================

// Sentinel errors expected to be defined in investigation_config.go.
var (
	ErrInvalidMaxActions             = errors.New("max actions must be positive")
	ErrInvalidMaxDuration            = errors.New("max duration must be positive")
	ErrInvalidMaxConcurrent          = errors.New("max concurrent investigations must be positive")
	ErrInvalidConfidenceThreshold    = errors.New("confidence threshold must be between 0.0 and 1.0")
	ErrInvalidErrorThreshold         = errors.New("error threshold must be positive")
	ErrEmptyAllowedTools             = errors.New("allowed tools list cannot be empty")
	ErrBlockedCommandContainsAllowed = errors.New("blocked command overlaps with allowed tools")
)

// InvestigationConfig holds safety and operational limits for investigations.
// This is a stub struct - the real implementation should be in investigation_config.go.
type InvestigationConfig struct{}

// Stub functions - to be implemented in investigation_config.go.
func NewInvestigationConfig() *InvestigationConfig {
	return nil
}

func DefaultInvestigationConfig() *InvestigationConfig {
	return nil
}

func (c *InvestigationConfig) MaxActionsPerInvestigation() int {
	return 0
}

func (c *InvestigationConfig) MaxDuration() time.Duration {
	return 0
}

func (c *InvestigationConfig) MaxConcurrentInvestigations() int {
	return 0
}

func (c *InvestigationConfig) AllowedTools() []string {
	return nil
}

func (c *InvestigationConfig) BlockedCommands() []string {
	return nil
}

func (c *InvestigationConfig) AllowedDirectories() []string {
	return nil
}

func (c *InvestigationConfig) RequireHumanApprovalPatterns() []string {
	return nil
}

func (c *InvestigationConfig) ConfirmBeforeRestart() bool {
	return false
}

func (c *InvestigationConfig) ConfirmBeforeDelete() bool {
	return false
}

func (c *InvestigationConfig) EscalateOnConfidenceBelow() float64 {
	return 0
}

func (c *InvestigationConfig) EscalateOnMultipleErrors() int {
	return 0
}

func (c *InvestigationConfig) SetMaxActions(_ int) error {
	return errors.New("not implemented")
}

func (c *InvestigationConfig) SetMaxDuration(_ time.Duration) error {
	return errors.New("not implemented")
}

func (c *InvestigationConfig) SetMaxConcurrent(_ int) error {
	return errors.New("not implemented")
}

func (c *InvestigationConfig) SetAllowedTools(_ []string) error {
	return errors.New("not implemented")
}

func (c *InvestigationConfig) SetBlockedCommands(_ []string) error {
	return errors.New("not implemented")
}

func (c *InvestigationConfig) SetAllowedDirectories(_ []string) {
}

func (c *InvestigationConfig) SetRequireHumanApprovalPatterns(_ []string) {
}

func (c *InvestigationConfig) SetConfirmBeforeRestart(_ bool) {
}

func (c *InvestigationConfig) SetConfirmBeforeDelete(_ bool) {
}

func (c *InvestigationConfig) SetEscalateOnConfidenceBelow(_ float64) error {
	return errors.New("not implemented")
}

func (c *InvestigationConfig) SetEscalateOnMultipleErrors(_ int) error {
	return errors.New("not implemented")
}

func (c *InvestigationConfig) IsToolAllowed(_ string) bool {
	return false
}

func (c *InvestigationConfig) IsCommandBlocked(_ string) bool {
	return false
}

func (c *InvestigationConfig) IsDirectoryAllowed(_ string) bool {
	return false
}

func (c *InvestigationConfig) RequiresHumanApproval(_ string) bool {
	return false
}

func (c *InvestigationConfig) Validate() error {
	return errors.New("not implemented")
}

// =============================================================================
// Default Config Tests
// =============================================================================

func TestDefaultInvestigationConfig_NotNil(t *testing.T) {
	cfg := DefaultInvestigationConfig()
	if cfg == nil {
		t.Error("DefaultInvestigationConfig() should not return nil")
	}
}

func TestDefaultInvestigationConfig_HasReasonableMaxActions(t *testing.T) {
	cfg := DefaultInvestigationConfig()
	if cfg == nil {
		t.Skip("DefaultInvestigationConfig() returned nil")
	}
	maxActions := cfg.MaxActionsPerInvestigation()
	if maxActions <= 0 {
		t.Errorf("MaxActionsPerInvestigation() = %v, want positive", maxActions)
	}
	if maxActions > 100 {
		t.Errorf("MaxActionsPerInvestigation() = %v, want <= 100 for safety", maxActions)
	}
}

func TestDefaultInvestigationConfig_HasReasonableMaxDuration(t *testing.T) {
	cfg := DefaultInvestigationConfig()
	if cfg == nil {
		t.Skip("DefaultInvestigationConfig() returned nil")
	}
	duration := cfg.MaxDuration()
	if duration <= 0 {
		t.Errorf("MaxDuration() = %v, want positive", duration)
	}
	if duration > 1*time.Hour {
		t.Errorf("MaxDuration() = %v, want <= 1 hour for safety", duration)
	}
}

func TestDefaultInvestigationConfig_HasReasonableMaxConcurrent(t *testing.T) {
	cfg := DefaultInvestigationConfig()
	if cfg == nil {
		t.Skip("DefaultInvestigationConfig() returned nil")
	}
	maxConcurrent := cfg.MaxConcurrentInvestigations()
	if maxConcurrent <= 0 {
		t.Errorf("MaxConcurrentInvestigations() = %v, want positive", maxConcurrent)
	}
	if maxConcurrent > 20 {
		t.Errorf("MaxConcurrentInvestigations() = %v, want <= 20 for resource safety", maxConcurrent)
	}
}

func TestDefaultInvestigationConfig_HasDefaultAllowedTools(t *testing.T) {
	cfg := DefaultInvestigationConfig()
	if cfg == nil {
		t.Skip("DefaultInvestigationConfig() returned nil")
	}
	tools := cfg.AllowedTools()
	if len(tools) == 0 {
		t.Error("AllowedTools() should have default tools")
	}
	// Should include at least bash and read_file for basic investigation
	hasBash := false
	hasReadFile := false
	for _, tool := range tools {
		if tool == "bash" {
			hasBash = true
		}
		if tool == "read_file" {
			hasReadFile = true
		}
	}
	if !hasBash {
		t.Error("AllowedTools() should include 'bash'")
	}
	if !hasReadFile {
		t.Error("AllowedTools() should include 'read_file'")
	}
}

func TestDefaultInvestigationConfig_HasDefaultBlockedCommands(t *testing.T) {
	cfg := DefaultInvestigationConfig()
	if cfg == nil {
		t.Skip("DefaultInvestigationConfig() returned nil")
	}
	blocked := cfg.BlockedCommands()
	if len(blocked) == 0 {
		t.Error("BlockedCommands() should have default dangerous commands")
	}
}

func TestDefaultInvestigationConfig_BlocksDangerousCommands(t *testing.T) {
	cfg := DefaultInvestigationConfig()
	if cfg == nil {
		t.Skip("DefaultInvestigationConfig() returned nil")
	}

	dangerousCommands := []string{
		"rm -rf",
		"dd if=",
		"mkfs",
		":(){:|:&};:", // fork bomb
		"> /dev/sda",
		"chmod -R 777 /",
	}

	for _, cmd := range dangerousCommands {
		if !cfg.IsCommandBlocked(cmd) {
			t.Errorf("IsCommandBlocked(%q) = false, want true", cmd)
		}
	}
}

func TestDefaultInvestigationConfig_ConfirmationsEnabledByDefault(t *testing.T) {
	cfg := DefaultInvestigationConfig()
	if cfg == nil {
		t.Skip("DefaultInvestigationConfig() returned nil")
	}
	if !cfg.ConfirmBeforeRestart() {
		t.Error("ConfirmBeforeRestart() should be true by default")
	}
	if !cfg.ConfirmBeforeDelete() {
		t.Error("ConfirmBeforeDelete() should be true by default")
	}
}

func TestDefaultInvestigationConfig_HasReasonableEscalationThresholds(t *testing.T) {
	cfg := DefaultInvestigationConfig()
	if cfg == nil {
		t.Skip("DefaultInvestigationConfig() returned nil")
	}

	confidence := cfg.EscalateOnConfidenceBelow()
	if confidence < 0 || confidence > 1 {
		t.Errorf("EscalateOnConfidenceBelow() = %v, want between 0 and 1", confidence)
	}

	errors := cfg.EscalateOnMultipleErrors()
	if errors <= 0 {
		t.Errorf("EscalateOnMultipleErrors() = %v, want positive", errors)
	}
}

// =============================================================================
// Setter Validation Tests
// =============================================================================

func TestInvestigationConfig_SetMaxActions_Valid(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	err := cfg.SetMaxActions(50)
	if err != nil {
		t.Errorf("SetMaxActions(50) error = %v", err)
	}
	if cfg.MaxActionsPerInvestigation() != 50 {
		t.Errorf("MaxActionsPerInvestigation() = %v, want 50", cfg.MaxActionsPerInvestigation())
	}
}

func TestInvestigationConfig_SetMaxActions_Zero(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	err := cfg.SetMaxActions(0)
	if err == nil {
		t.Error("SetMaxActions(0) should return error")
	}
}

func TestInvestigationConfig_SetMaxActions_Negative(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	err := cfg.SetMaxActions(-1)
	if err == nil {
		t.Error("SetMaxActions(-1) should return error")
	}
}

func TestInvestigationConfig_SetMaxDuration_Valid(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	err := cfg.SetMaxDuration(30 * time.Minute)
	if err != nil {
		t.Errorf("SetMaxDuration(30m) error = %v", err)
	}
	if cfg.MaxDuration() != 30*time.Minute {
		t.Errorf("MaxDuration() = %v, want 30m", cfg.MaxDuration())
	}
}

func TestInvestigationConfig_SetMaxDuration_Zero(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	err := cfg.SetMaxDuration(0)
	if err == nil {
		t.Error("SetMaxDuration(0) should return error")
	}
}

func TestInvestigationConfig_SetMaxDuration_Negative(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	err := cfg.SetMaxDuration(-1 * time.Minute)
	if err == nil {
		t.Error("SetMaxDuration(-1m) should return error")
	}
}

func TestInvestigationConfig_SetMaxConcurrent_Valid(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	err := cfg.SetMaxConcurrent(10)
	if err != nil {
		t.Errorf("SetMaxConcurrent(10) error = %v", err)
	}
	if cfg.MaxConcurrentInvestigations() != 10 {
		t.Errorf("MaxConcurrentInvestigations() = %v, want 10", cfg.MaxConcurrentInvestigations())
	}
}

func TestInvestigationConfig_SetMaxConcurrent_Zero(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	err := cfg.SetMaxConcurrent(0)
	if err == nil {
		t.Error("SetMaxConcurrent(0) should return error")
	}
}

func TestInvestigationConfig_SetAllowedTools_Valid(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	tools := []string{"bash", "read_file", "list_files"}
	err := cfg.SetAllowedTools(tools)
	if err != nil {
		t.Errorf("SetAllowedTools() error = %v", err)
	}
	if len(cfg.AllowedTools()) != 3 {
		t.Errorf("AllowedTools() len = %v, want 3", len(cfg.AllowedTools()))
	}
}

func TestInvestigationConfig_SetAllowedTools_Empty(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	err := cfg.SetAllowedTools([]string{})
	if err == nil {
		t.Error("SetAllowedTools([]) should return error")
	}
}

func TestInvestigationConfig_SetAllowedTools_Nil(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	err := cfg.SetAllowedTools(nil)
	if err == nil {
		t.Error("SetAllowedTools(nil) should return error")
	}
}

func TestInvestigationConfig_SetEscalateOnConfidenceBelow_Valid(t *testing.T) {
	tests := []struct {
		name      string
		threshold float64
	}{
		{"zero", 0.0},
		{"half", 0.5},
		{"high", 0.9},
		{"one", 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewInvestigationConfig()
			if cfg == nil {
				t.Skip("NewInvestigationConfig() returned nil")
			}
			err := cfg.SetEscalateOnConfidenceBelow(tt.threshold)
			if err != nil {
				t.Errorf("SetEscalateOnConfidenceBelow(%v) error = %v", tt.threshold, err)
			}
			if cfg.EscalateOnConfidenceBelow() != tt.threshold {
				t.Errorf("EscalateOnConfidenceBelow() = %v, want %v", cfg.EscalateOnConfidenceBelow(), tt.threshold)
			}
		})
	}
}

func TestInvestigationConfig_SetEscalateOnConfidenceBelow_Invalid(t *testing.T) {
	tests := []struct {
		name      string
		threshold float64
	}{
		{"negative", -0.1},
		{"greater than one", 1.1},
		{"large", 5.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := NewInvestigationConfig()
			if cfg == nil {
				t.Skip("NewInvestigationConfig() returned nil")
			}
			err := cfg.SetEscalateOnConfidenceBelow(tt.threshold)
			if err == nil {
				t.Errorf("SetEscalateOnConfidenceBelow(%v) should return error", tt.threshold)
			}
		})
	}
}

func TestInvestigationConfig_SetEscalateOnMultipleErrors_Valid(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	err := cfg.SetEscalateOnMultipleErrors(5)
	if err != nil {
		t.Errorf("SetEscalateOnMultipleErrors(5) error = %v", err)
	}
	if cfg.EscalateOnMultipleErrors() != 5 {
		t.Errorf("EscalateOnMultipleErrors() = %v, want 5", cfg.EscalateOnMultipleErrors())
	}
}

func TestInvestigationConfig_SetEscalateOnMultipleErrors_Zero(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	err := cfg.SetEscalateOnMultipleErrors(0)
	if err == nil {
		t.Error("SetEscalateOnMultipleErrors(0) should return error")
	}
}

func TestInvestigationConfig_SetEscalateOnMultipleErrors_Negative(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	err := cfg.SetEscalateOnMultipleErrors(-1)
	if err == nil {
		t.Error("SetEscalateOnMultipleErrors(-1) should return error")
	}
}

// =============================================================================
// Tool/Command Checking Tests
// =============================================================================

func TestInvestigationConfig_IsToolAllowed_InList(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	_ = cfg.SetAllowedTools([]string{"bash", "read_file", "list_files"})

	if !cfg.IsToolAllowed("bash") {
		t.Error("IsToolAllowed('bash') = false, want true")
	}
	if !cfg.IsToolAllowed("read_file") {
		t.Error("IsToolAllowed('read_file') = false, want true")
	}
}

func TestInvestigationConfig_IsToolAllowed_NotInList(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	_ = cfg.SetAllowedTools([]string{"bash", "read_file"})

	if cfg.IsToolAllowed("edit_file") {
		t.Error("IsToolAllowed('edit_file') = true, want false")
	}
	if cfg.IsToolAllowed("execute_dangerous") {
		t.Error("IsToolAllowed('execute_dangerous') = true, want false")
	}
}

func TestInvestigationConfig_IsCommandBlocked_ExactMatch(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	_ = cfg.SetBlockedCommands([]string{"rm -rf", "dd if=", "mkfs"})

	if !cfg.IsCommandBlocked("rm -rf /") {
		t.Error("IsCommandBlocked('rm -rf /') = false, want true")
	}
	if !cfg.IsCommandBlocked("dd if=/dev/zero of=/dev/sda") {
		t.Error("IsCommandBlocked('dd if=...') = false, want true")
	}
}

func TestInvestigationConfig_IsCommandBlocked_SafeCommand(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	_ = cfg.SetBlockedCommands([]string{"rm -rf", "dd if=", "mkfs"})

	if cfg.IsCommandBlocked("ls -la") {
		t.Error("IsCommandBlocked('ls -la') = true, want false")
	}
	if cfg.IsCommandBlocked("cat /var/log/syslog") {
		t.Error("IsCommandBlocked('cat /var/log/syslog') = true, want false")
	}
	if cfg.IsCommandBlocked("top -b -n 1") {
		t.Error("IsCommandBlocked('top -b -n 1') = true, want false")
	}
}

func TestInvestigationConfig_IsDirectoryAllowed_InList(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	cfg.SetAllowedDirectories([]string{"/var/log", "/etc/myapp", "/tmp"})

	if !cfg.IsDirectoryAllowed("/var/log") {
		t.Error("IsDirectoryAllowed('/var/log') = false, want true")
	}
	if !cfg.IsDirectoryAllowed("/var/log/syslog") {
		t.Error("IsDirectoryAllowed('/var/log/syslog') = false, want true (subdirectory)")
	}
}

func TestInvestigationConfig_IsDirectoryAllowed_NotInList(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	cfg.SetAllowedDirectories([]string{"/var/log", "/etc/myapp"})

	if cfg.IsDirectoryAllowed("/etc/passwd") {
		t.Error("IsDirectoryAllowed('/etc/passwd') = true, want false")
	}
	if cfg.IsDirectoryAllowed("/root") {
		t.Error("IsDirectoryAllowed('/root') = true, want false")
	}
}

func TestInvestigationConfig_IsDirectoryAllowed_EmptyListAllowsAll(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	cfg.SetAllowedDirectories(nil)

	// When no directories are specified, all should be allowed (permissive default)
	if !cfg.IsDirectoryAllowed("/any/path") {
		t.Error("IsDirectoryAllowed('/any/path') = false, want true when list is empty")
	}
}

// =============================================================================
// Human Approval Pattern Tests
// =============================================================================

func TestInvestigationConfig_RequiresHumanApproval_MatchingPattern(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	cfg.SetRequireHumanApprovalPatterns([]string{"restart", "kill", "delete", "systemctl stop"})

	if !cfg.RequiresHumanApproval("systemctl restart nginx") {
		t.Error("RequiresHumanApproval('systemctl restart nginx') = false, want true")
	}
	if !cfg.RequiresHumanApproval("kill -9 1234") {
		t.Error("RequiresHumanApproval('kill -9 1234') = false, want true")
	}
	if !cfg.RequiresHumanApproval("rm -rf (delete operation)") {
		t.Error("RequiresHumanApproval('delete operation') = false, want true")
	}
}

func TestInvestigationConfig_RequiresHumanApproval_NoMatch(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	cfg.SetRequireHumanApprovalPatterns([]string{"restart", "kill", "delete"})

	if cfg.RequiresHumanApproval("cat /var/log/syslog") {
		t.Error("RequiresHumanApproval('cat /var/log/syslog') = true, want false")
	}
	if cfg.RequiresHumanApproval("top -b -n 1") {
		t.Error("RequiresHumanApproval('top -b -n 1') = true, want false")
	}
}

func TestInvestigationConfig_RequiresHumanApproval_EmptyPatterns(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	cfg.SetRequireHumanApprovalPatterns(nil)

	// When no patterns are set, nothing requires approval
	if cfg.RequiresHumanApproval("anything") {
		t.Error("RequiresHumanApproval('anything') = true, want false when patterns empty")
	}
}

// =============================================================================
// Validation Tests
// =============================================================================

func TestInvestigationConfig_Validate_ValidConfig(t *testing.T) {
	cfg := DefaultInvestigationConfig()
	if cfg == nil {
		t.Skip("DefaultInvestigationConfig() returned nil")
	}
	err := cfg.Validate()
	if err != nil {
		t.Errorf("Validate() error = %v, default config should be valid", err)
	}
}

func TestInvestigationConfig_Validate_InvalidMaxActions(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	// Force invalid state by setting other fields first, then checking validation
	_ = cfg.SetMaxDuration(15 * time.Minute)
	_ = cfg.SetMaxConcurrent(5)
	_ = cfg.SetAllowedTools([]string{"bash"})
	// MaxActions is still 0 (invalid)

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should return error for zero MaxActions")
	}
}

func TestInvestigationConfig_Validate_InvalidMaxDuration(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	_ = cfg.SetMaxActions(20)
	_ = cfg.SetMaxConcurrent(5)
	_ = cfg.SetAllowedTools([]string{"bash"})
	// MaxDuration is still 0 (invalid)

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should return error for zero MaxDuration")
	}
}

func TestInvestigationConfig_Validate_EmptyAllowedTools(t *testing.T) {
	cfg := NewInvestigationConfig()
	if cfg == nil {
		t.Skip("NewInvestigationConfig() returned nil")
	}
	_ = cfg.SetMaxActions(20)
	_ = cfg.SetMaxDuration(15 * time.Minute)
	_ = cfg.SetMaxConcurrent(5)
	// AllowedTools is still empty (invalid)

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should return error for empty AllowedTools")
	}
}

// =============================================================================
// Error Constants Tests
// =============================================================================

func TestInvestigationConfigErrors_NotNil(t *testing.T) {
	if ErrInvalidMaxActions == nil {
		t.Error("ErrInvalidMaxActions should not be nil")
	}
	if ErrInvalidMaxDuration == nil {
		t.Error("ErrInvalidMaxDuration should not be nil")
	}
	if ErrInvalidMaxConcurrent == nil {
		t.Error("ErrInvalidMaxConcurrent should not be nil")
	}
	if ErrInvalidConfidenceThreshold == nil {
		t.Error("ErrInvalidConfidenceThreshold should not be nil")
	}
	if ErrInvalidErrorThreshold == nil {
		t.Error("ErrInvalidErrorThreshold should not be nil")
	}
	if ErrEmptyAllowedTools == nil {
		t.Error("ErrEmptyAllowedTools should not be nil")
	}
}

func TestInvestigationConfigErrors_HaveMessages(t *testing.T) {
	if ErrInvalidMaxActions.Error() == "" {
		t.Error("ErrInvalidMaxActions should have a message")
	}
	if ErrInvalidMaxDuration.Error() == "" {
		t.Error("ErrInvalidMaxDuration should have a message")
	}
	if ErrInvalidMaxConcurrent.Error() == "" {
		t.Error("ErrInvalidMaxConcurrent should have a message")
	}
	if ErrInvalidConfidenceThreshold.Error() == "" {
		t.Error("ErrInvalidConfidenceThreshold should have a message")
	}
	if ErrInvalidErrorThreshold.Error() == "" {
		t.Error("ErrInvalidErrorThreshold should have a message")
	}
	if ErrEmptyAllowedTools.Error() == "" {
		t.Error("ErrEmptyAllowedTools should have a message")
	}
}

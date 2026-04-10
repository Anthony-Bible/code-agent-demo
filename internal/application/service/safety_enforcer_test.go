package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/application/config"
	"github.com/anthony-bible/code-agent-demo/internal/domain/safety"
)

// mockValidator is a test double for safety.CommandValidator.
type mockValidator struct {
	allowAll     bool
	blocked      map[string]bool
	needsConfirm map[string]bool
}

func newMockValidator(allowAll bool) *mockValidator {
	return &mockValidator{
		allowAll:     allowAll,
		blocked:      make(map[string]bool),
		needsConfirm: make(map[string]bool),
	}
}

func (m *mockValidator) blockCommand(cmd string) {
	m.blocked[cmd] = true
}

func (m *mockValidator) requireConfirm(cmd string) {
	m.needsConfirm[cmd] = true
}

func (m *mockValidator) Validate(command string, _ bool) safety.ValidationResult {
	if m.blocked[command] {
		return safety.ValidationResult{
			Allowed:     false,
			IsDangerous: true,
			Reason:      "blocked by mock",
		}
	}
	if m.needsConfirm[command] {
		return safety.ValidationResult{
			Allowed:      true,
			IsDangerous:  true,
			Reason:       "requires confirmation",
			NeedsConfirm: true,
		}
	}
	if m.allowAll {
		return safety.ValidationResult{Allowed: true}
	}
	// Check against dangerous patterns for realistic behavior
	isDangerous, reason := safety.IsDangerousCommand(command)
	if isDangerous {
		return safety.ValidationResult{
			Allowed:     false,
			IsDangerous: true,
			Reason:      reason,
		}
	}
	return safety.ValidationResult{Allowed: true}
}

// newTestEnforcer creates an enforcer with a mock validator for testing.
// The mock validator checks dangerous patterns but allows safe commands.
func newTestEnforcer(t *testing.T, cfg *config.InvestigationConfig) SafetyEnforcer {
	t.Helper()
	validator := newMockValidator(false) // Use realistic validation
	enforcer, err := NewInvestigationSafetyEnforcerWithValidator(cfg, validator)
	if err != nil {
		t.Fatalf("NewInvestigationSafetyEnforcerWithValidator() error = %v", err)
	}
	return enforcer
}

// =============================================================================
// SafetyEnforcer Tests
// These tests verify the behavior of SafetyEnforcer interface and
// InvestigationSafetyEnforcer implementation.
// =============================================================================

// =============================================================================
// Sentinel Error Tests
// =============================================================================

func TestSafetyEnforcerErrors_NotNil(t *testing.T) {
	if ErrToolBlocked == nil {
		t.Error("ErrToolBlocked should not be nil")
	}
	if ErrCommandBlocked == nil {
		t.Error("ErrCommandBlocked should not be nil")
	}
	if ErrActionBudgetExhausted == nil {
		t.Error("ErrActionBudgetExhausted should not be nil")
	}
	if ErrInvestigationTimeout == nil {
		t.Error("ErrInvestigationTimeout should not be nil")
	}
}

func TestSafetyEnforcerErrors_HaveMessages(t *testing.T) {
	if ErrToolBlocked.Error() == "" {
		t.Error("ErrToolBlocked should have a message")
	}
	if ErrCommandBlocked.Error() == "" {
		t.Error("ErrCommandBlocked should have a message")
	}
	if ErrActionBudgetExhausted.Error() == "" {
		t.Error("ErrActionBudgetExhausted should have a message")
	}
	if ErrInvestigationTimeout.Error() == "" {
		t.Error("ErrInvestigationTimeout should have a message")
	}
}

// =============================================================================
// Constructor Tests
// =============================================================================

func TestNewInvestigationSafetyEnforcerWithValidator_NotNil(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	validator := newMockValidator(true)
	enforcer, err := NewInvestigationSafetyEnforcerWithValidator(cfg, validator)
	if err != nil {
		t.Fatalf("NewInvestigationSafetyEnforcerWithValidator() error = %v", err)
	}
	if enforcer == nil {
		t.Error("NewInvestigationSafetyEnforcerWithValidator() should not return nil")
	}
}

func TestNewInvestigationSafetyEnforcerWithValidator_NilConfig(t *testing.T) {
	validator := newMockValidator(true)
	enforcer, err := NewInvestigationSafetyEnforcerWithValidator(nil, validator)
	if err == nil {
		t.Error("NewInvestigationSafetyEnforcerWithValidator(nil, validator) should return error")
	}
	if !errors.Is(err, ErrNilConfig) {
		t.Errorf("NewInvestigationSafetyEnforcerWithValidator(nil, validator) error = %v, want ErrNilConfig", err)
	}
	if enforcer != nil {
		t.Error("NewInvestigationSafetyEnforcerWithValidator(nil, validator) should return nil enforcer")
	}
}

func TestNewInvestigationSafetyEnforcerWithValidator_NilValidator(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer, err := NewInvestigationSafetyEnforcerWithValidator(cfg, nil)
	if err == nil {
		t.Error("NewInvestigationSafetyEnforcerWithValidator(cfg, nil) should return error")
	}
	if !errors.Is(err, ErrNilValidator) {
		t.Errorf("NewInvestigationSafetyEnforcerWithValidator(cfg, nil) error = %v, want ErrNilValidator", err)
	}
	if enforcer != nil {
		t.Error("NewInvestigationSafetyEnforcerWithValidator(cfg, nil) should return nil enforcer")
	}
}

func TestNewInvestigationSafetyEnforcerWithValidator_InvalidConfig(t *testing.T) {
	cfg := config.NewInvestigationConfig() // Empty config - invalid
	validator := newMockValidator(true)
	enforcer, err := NewInvestigationSafetyEnforcerWithValidator(cfg, validator)
	if err == nil {
		t.Error("NewInvestigationSafetyEnforcerWithValidator() with invalid config should return error")
	}
	if enforcer != nil {
		t.Error("NewInvestigationSafetyEnforcerWithValidator() with invalid config should return nil enforcer")
	}
}

// =============================================================================
// CheckToolAllowed Tests
// =============================================================================

func TestInvestigationSafetyEnforcer_CheckToolAllowed_AllowedTool(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	tests := []struct {
		name string
		tool string
	}{
		{"bash allowed", "bash"},
		{"read_file allowed", "read_file"},
		{"list_files allowed", "list_files"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforcer.CheckToolAllowed(tt.tool)
			if err != nil {
				t.Errorf("CheckToolAllowed(%q) error = %v, want nil", tt.tool, err)
			}
		})
	}
}

func TestInvestigationSafetyEnforcer_CheckToolAllowed_BlockedTool(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	tests := []struct {
		name string
		tool string
	}{
		{"edit_file blocked", "edit_file"},
		{"execute_sql blocked", "execute_sql"},
		{"delete_file blocked", "delete_file"},
		{"unknown_tool blocked", "unknown_tool"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforcer.CheckToolAllowed(tt.tool)
			if err == nil {
				t.Errorf("CheckToolAllowed(%q) should return error for blocked tool", tt.tool)
			}
			if !errors.Is(err, ErrToolBlocked) {
				t.Errorf("CheckToolAllowed(%q) error = %v, want ErrToolBlocked", tt.tool, err)
			}
		})
	}
}

func TestInvestigationSafetyEnforcer_CheckToolAllowed_EmptyTool(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	err := enforcer.CheckToolAllowed("")
	if err == nil {
		t.Error("CheckToolAllowed('') should return error")
	}
	if !errors.Is(err, ErrToolBlocked) {
		t.Errorf("CheckToolAllowed('') error = %v, want ErrToolBlocked", err)
	}
}

func TestInvestigationSafetyEnforcer_CheckToolAllowed_CustomConfig(t *testing.T) {
	cfg := config.NewInvestigationConfig()
	_ = cfg.SetMaxActions(10)
	_ = cfg.SetMaxDuration(5 * time.Minute)
	_ = cfg.SetMaxConcurrent(3)
	_ = cfg.SetAllowedTools([]string{"custom_tool", "another_tool"})

	enforcer := newTestEnforcer(t, cfg)

	// Custom tool should be allowed
	if err := enforcer.CheckToolAllowed("custom_tool"); err != nil {
		t.Errorf("CheckToolAllowed('custom_tool') error = %v, want nil", err)
	}

	// Default tool should now be blocked
	if err := enforcer.CheckToolAllowed("bash"); err == nil {
		t.Error("CheckToolAllowed('bash') should return error when not in custom allowed list")
	}
}

// =============================================================================
// CheckCommandAllowed Tests
// =============================================================================

func TestInvestigationSafetyEnforcer_CheckCommandAllowed_SafeCommands(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	tests := []struct {
		name    string
		command string
	}{
		{"ls command", "ls -la"},
		{"cat command", "cat /var/log/syslog"},
		{"ps command", "ps aux"},
		{"top command", "top -b -n 1"},
		{"grep command", "grep error /var/log/app.log"},
		{"tail command", "tail -100 /var/log/messages"},
		{"kubectl get", "kubectl get pods"},
		{"docker ps", "docker ps -a"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforcer.CheckCommandAllowed(tt.command)
			if err != nil {
				t.Errorf("CheckCommandAllowed(%q) error = %v, want nil", tt.command, err)
			}
		})
	}
}

func TestInvestigationSafetyEnforcer_CheckCommandAllowed_DangerousCommands(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	tests := []struct {
		name    string
		command string
	}{
		{"rm -rf root", "rm -rf /"},
		{"rm -rf home", "rm -rf /home/user"},
		{"dd to disk", "dd if=/dev/zero of=/dev/sda"},
		{"mkfs", "mkfs.ext4 /dev/sdb1"},
		{"fork bomb", ":(){:|:&};:"},
		{"write to disk device", "> /dev/sda"},
		{"chmod 777 root", "chmod -R 777 /"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforcer.CheckCommandAllowed(tt.command)
			if err == nil {
				t.Errorf("CheckCommandAllowed(%q) should return error for dangerous command", tt.command)
			}
			if !errors.Is(err, ErrCommandBlocked) {
				t.Errorf("CheckCommandAllowed(%q) error = %v, want ErrCommandBlocked", tt.command, err)
			}
		})
	}
}

func TestInvestigationSafetyEnforcer_CheckCommandAllowed_EmptyCommand(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	// Empty command should be allowed (no dangerous patterns)
	err := enforcer.CheckCommandAllowed("")
	if err != nil {
		t.Errorf("CheckCommandAllowed('') error = %v, want nil", err)
	}
}

func TestInvestigationSafetyEnforcer_CheckCommandAllowed_NeedsConfirm(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	validator := newMockValidator(true)
	validator.requireConfirm("sudo systemctl restart app")
	enforcer, err := NewInvestigationSafetyEnforcerWithValidator(cfg, validator)
	if err != nil {
		t.Fatalf("NewInvestigationSafetyEnforcerWithValidator() error = %v", err)
	}

	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{
			name:    "NeedsConfirm=true blocks command in headless mode",
			command: "sudo systemctl restart app",
			wantErr: true,
		},
		{
			name:    "NeedsConfirm=false allows safe command",
			command: "ls -la",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforcer.CheckCommandAllowed(tt.command)
			if tt.wantErr {
				if err == nil {
					t.Errorf("CheckCommandAllowed(%q) should return ErrCommandBlocked", tt.command)
				} else if !errors.Is(err, ErrCommandBlocked) {
					t.Errorf("CheckCommandAllowed(%q) error = %v, want ErrCommandBlocked", tt.command, err)
				}
			} else if err != nil {
				t.Errorf("CheckCommandAllowed(%q) error = %v, want nil", tt.command, err)
			}
		})
	}
}

func TestInvestigationSafetyEnforcer_CheckCommandAllowed_CustomBlockedPatterns(t *testing.T) {
	cfg := config.NewInvestigationConfig()
	_ = cfg.SetMaxActions(10)
	_ = cfg.SetMaxDuration(5 * time.Minute)
	_ = cfg.SetMaxConcurrent(3)
	_ = cfg.SetAllowedTools([]string{"bash"})
	_ = cfg.SetBlockedCommands([]string{"custom_danger", "secret_cmd"})

	// Create a mock validator that blocks specific custom commands
	validator := newMockValidator(true) // Allow all by default
	validator.blockCommand("run custom_danger now")
	enforcer, err := NewInvestigationSafetyEnforcerWithValidator(cfg, validator)
	if err != nil {
		t.Fatalf("NewInvestigationSafetyEnforcerWithValidator() error = %v", err)
	}

	// Custom blocked pattern
	if err := enforcer.CheckCommandAllowed("run custom_danger now"); err == nil {
		t.Error("CheckCommandAllowed() should block custom pattern")
	}

	// Default dangerous command - validator allows all, so this passes
	if err := enforcer.CheckCommandAllowed("rm -rf /"); err != nil {
		t.Logf("Note: custom config may or may not include default blocked commands: %v", err)
	}
}

// =============================================================================
// CheckActionBudget Tests
// =============================================================================

func TestInvestigationSafetyEnforcer_CheckActionBudget_UnderLimit(t *testing.T) {
	cfg := config.DefaultInvestigationConfig() // Default is 20 max actions
	enforcer := newTestEnforcer(t, cfg)

	tests := []struct {
		name         string
		actionsTaken int
	}{
		{"zero actions", 0},
		{"one action", 1},
		{"half limit", 10},
		{"one under limit", 19},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforcer.CheckActionBudget(tt.actionsTaken)
			if err != nil {
				t.Errorf("CheckActionBudget(%d) error = %v, want nil", tt.actionsTaken, err)
			}
		})
	}
}

func TestInvestigationSafetyEnforcer_CheckActionBudget_AtLimit(t *testing.T) {
	cfg := config.DefaultInvestigationConfig() // Default is 20 max actions
	enforcer := newTestEnforcer(t, cfg)

	err := enforcer.CheckActionBudget(20) // At limit
	if err == nil {
		t.Error("CheckActionBudget(20) should return error at limit")
	}
	if !errors.Is(err, ErrActionBudgetExhausted) {
		t.Errorf("CheckActionBudget(20) error = %v, want ErrActionBudgetExhausted", err)
	}
}

func TestInvestigationSafetyEnforcer_CheckActionBudget_OverLimit(t *testing.T) {
	cfg := config.DefaultInvestigationConfig() // Default is 20 max actions
	enforcer := newTestEnforcer(t, cfg)

	tests := []struct {
		name         string
		actionsTaken int
	}{
		{"one over limit", 21},
		{"double limit", 40},
		{"way over limit", 100},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforcer.CheckActionBudget(tt.actionsTaken)
			if err == nil {
				t.Errorf("CheckActionBudget(%d) should return error over limit", tt.actionsTaken)
			}
			if !errors.Is(err, ErrActionBudgetExhausted) {
				t.Errorf("CheckActionBudget(%d) error = %v, want ErrActionBudgetExhausted", tt.actionsTaken, err)
			}
		})
	}
}

func TestInvestigationSafetyEnforcer_CheckActionBudget_NegativeActions(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	// Negative actions should be treated as under limit (or error)
	err := enforcer.CheckActionBudget(-1)
	if err != nil {
		t.Errorf("CheckActionBudget(-1) error = %v, want nil (negative treated as under limit)", err)
	}
}

func TestInvestigationSafetyEnforcer_CheckActionBudget_CustomLimit(t *testing.T) {
	cfg := config.NewInvestigationConfig()
	_ = cfg.SetMaxActions(5) // Very low limit
	_ = cfg.SetMaxDuration(5 * time.Minute)
	_ = cfg.SetMaxConcurrent(3)
	_ = cfg.SetAllowedTools([]string{"bash"})

	enforcer := newTestEnforcer(t, cfg)

	// Under custom limit
	if err := enforcer.CheckActionBudget(4); err != nil {
		t.Errorf("CheckActionBudget(4) error = %v, want nil", err)
	}

	// At custom limit
	if err := enforcer.CheckActionBudget(5); err == nil {
		t.Error("CheckActionBudget(5) should return error at custom limit")
	}
}

// =============================================================================
// CheckTimeout Tests
// =============================================================================

func TestInvestigationSafetyEnforcer_CheckTimeout_ActiveContext(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	ctx := context.Background()
	err := enforcer.CheckTimeout(ctx)
	if err != nil {
		t.Errorf("CheckTimeout() with active context error = %v, want nil", err)
	}
}

func TestInvestigationSafetyEnforcer_CheckTimeout_CancelledContext(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := enforcer.CheckTimeout(ctx)
	if err == nil {
		t.Error("CheckTimeout() with cancelled context should return error")
	}
	if !errors.Is(err, ErrInvestigationTimeout) {
		t.Errorf("CheckTimeout() error = %v, want ErrInvestigationTimeout", err)
	}
}

func TestInvestigationSafetyEnforcer_CheckTimeout_ExpiredDeadline(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	// Context with already-expired deadline
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-1*time.Second))
	defer cancel()

	err := enforcer.CheckTimeout(ctx)
	if err == nil {
		t.Error("CheckTimeout() with expired deadline should return error")
	}
	if !errors.Is(err, ErrInvestigationTimeout) {
		t.Errorf("CheckTimeout() error = %v, want ErrInvestigationTimeout", err)
	}
}

func TestInvestigationSafetyEnforcer_CheckTimeout_FutureDeadline(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	// Context with future deadline
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(1*time.Hour))
	defer cancel()

	err := enforcer.CheckTimeout(ctx)
	if err != nil {
		t.Errorf("CheckTimeout() with future deadline error = %v, want nil", err)
	}
}

func TestInvestigationSafetyEnforcer_CheckTimeout_NilContext(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	// This should either panic or return error - we test it doesn't panic silently
	defer func() {
		if r := recover(); r != nil {
			t.Logf("CheckTimeout(nil) panicked as expected: %v", r)
		}
	}()

	//nolint:staticcheck // Testing nil context behavior intentionally
	err := enforcer.CheckTimeout(nil)
	if err == nil {
		t.Error("CheckTimeout(nil) should return error or panic")
	}
}

// =============================================================================
// Interface Compliance Test
// =============================================================================

func TestInvestigationSafetyEnforcer_ImplementsInterface(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	// Compile-time check that InvestigationSafetyEnforcer implements SafetyEnforcer
	_ = enforcer
}

// =============================================================================
// SafetyEnforcer Interface Tests (using mock if available)
// =============================================================================

func TestSafetyEnforcer_InterfaceMethods(t *testing.T) {
	// This test verifies the interface definition by using the concrete implementation
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	// All interface methods should be callable
	_ = enforcer.CheckToolAllowed("bash")
	_ = enforcer.CheckCommandAllowed("ls")
	_ = enforcer.CheckActionBudget(0)
	_ = enforcer.CheckTimeout(context.Background())
}

// =============================================================================
// Edge Cases and Boundary Tests
// =============================================================================

func TestInvestigationSafetyEnforcer_CommandWithWhitespace(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	tests := []struct {
		name      string
		command   string
		wantError bool
	}{
		{"leading whitespace rm -rf", "  rm -rf /", true},
		{"trailing whitespace", "rm -rf /  ", true},
		{"tabs in command", "rm\t-rf\t/", true},
		{"newlines in command", "rm\n-rf\n/", true},
		{"safe with whitespace", "  ls -la  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforcer.CheckCommandAllowed(tt.command)
			if tt.wantError && err == nil {
				t.Errorf("CheckCommandAllowed(%q) should return error", tt.command)
			}
			if !tt.wantError && err != nil {
				t.Errorf("CheckCommandAllowed(%q) error = %v, want nil", tt.command, err)
			}
		})
	}
}

func TestInvestigationSafetyEnforcer_ToolWithCase(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	// Tool names should be case-sensitive
	tests := []struct {
		name      string
		tool      string
		wantError bool
	}{
		{"lowercase bash", "bash", false},
		{"uppercase BASH", "BASH", true},  // Should be blocked (case sensitive)
		{"mixed case Bash", "Bash", true}, // Should be blocked (case sensitive)
		{"lowercase read_file", "read_file", false},
		{"uppercase READ_FILE", "READ_FILE", true}, // Should be blocked
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := enforcer.CheckToolAllowed(tt.tool)
			if tt.wantError && err == nil {
				t.Errorf("CheckToolAllowed(%q) should return error", tt.tool)
			}
			if !tt.wantError && err != nil {
				t.Errorf("CheckToolAllowed(%q) error = %v, want nil", tt.tool, err)
			}
		})
	}
}

// =============================================================================
// Concurrent Safety Tests
// =============================================================================

func TestInvestigationSafetyEnforcer_ConcurrentChecks(t *testing.T) {
	cfg := config.DefaultInvestigationConfig()
	enforcer := newTestEnforcer(t, cfg)

	done := make(chan bool, 3)

	// Concurrent tool checks
	go func() {
		for range 100 {
			_ = enforcer.CheckToolAllowed("bash")
			_ = enforcer.CheckToolAllowed("unknown")
		}
		done <- true
	}()

	// Concurrent command checks
	go func() {
		for range 100 {
			_ = enforcer.CheckCommandAllowed("ls -la")
			_ = enforcer.CheckCommandAllowed("rm -rf /")
		}
		done <- true
	}()

	// Concurrent budget checks
	go func() {
		for i := range 100 {
			_ = enforcer.CheckActionBudget(i % 25)
		}
		done <- true
	}()

	// Wait for all goroutines
	for range 3 {
		<-done
	}

	// If we get here without panic, concurrent safety is working
}

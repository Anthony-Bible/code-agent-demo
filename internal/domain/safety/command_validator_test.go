package safety

import (
	"errors"
	"sync"
	"testing"
)

// mockAllowChecker is a mock implementation of CommandAllowChecker for testing.
type mockAllowChecker struct {
	allowedCommands map[string]string // command -> description
}

func newMockAllowChecker(allowed map[string]string) *mockAllowChecker {
	return &mockAllowChecker{allowedCommands: allowed}
}

func (m *mockAllowChecker) IsAllowed(cmd string) (bool, string) {
	if desc, ok := m.allowedCommands[cmd]; ok {
		return true, desc
	}
	return false, ""
}

func (m *mockAllowChecker) IsAllowedWithPipes(cmd string) (bool, string) {
	// For simplicity, just delegate to IsAllowed in tests
	return m.IsAllowed(cmd)
}

func TestNewCommandValidator(t *testing.T) {
	mock := newMockAllowChecker(map[string]string{"ls": "list files"})

	validator, err := NewCommandValidator(ModeWhitelist, mock, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if validator.Mode() != ModeWhitelist {
		t.Errorf("expected mode %v, got %v", ModeWhitelist, validator.Mode())
	}
	if !validator.AskLLMOnUnknown() {
		t.Error("expected AskLLMOnUnknown to be true")
	}
}

func TestCommandValidator_WhitelistMode_Allowed(t *testing.T) {
	mock := newMockAllowChecker(map[string]string{
		"ls":       "list files",
		"cat file": "display file",
	})
	validator, err := NewCommandValidator(ModeWhitelist, mock, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name         string
		command      string
		llmDangerous bool
		wantAllowed  bool
		wantDanger   bool
		wantConfirm  bool
	}{
		{
			name:         "whitelisted command passes",
			command:      "ls",
			llmDangerous: false,
			wantAllowed:  true,
			wantDanger:   false,
			wantConfirm:  false,
		},
		{
			name:         "whitelisted command passes even with LLM flag",
			command:      "cat file",
			llmDangerous: true, // Should be ignored for whitelisted commands
			wantAllowed:  true,
			wantDanger:   false,
			wantConfirm:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.command, tt.llmDangerous)
			if result.Allowed != tt.wantAllowed {
				t.Errorf("Allowed: got %v, want %v", result.Allowed, tt.wantAllowed)
			}
			if result.IsDangerous != tt.wantDanger {
				t.Errorf("IsDangerous: got %v, want %v", result.IsDangerous, tt.wantDanger)
			}
			if result.NeedsConfirm != tt.wantConfirm {
				t.Errorf("NeedsConfirm: got %v, want %v", result.NeedsConfirm, tt.wantConfirm)
			}
		})
	}
}

func TestCommandValidator_WhitelistMode_Blocked(t *testing.T) {
	mock := newMockAllowChecker(map[string]string{"ls": "list files"})
	validator, err := NewCommandValidator(ModeWhitelist, mock, false) // strict mode
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result := validator.Validate("rm -rf /", false)

	if result.Allowed {
		t.Error("expected non-whitelisted command to be blocked")
	}
	if result.NeedsConfirm {
		t.Error("expected no confirmation needed when blocked")
	}
	if result.Reason == "" {
		t.Error("expected reason to be set for blocked command")
	}
}

func TestCommandValidator_WhitelistMode_LLMFallback(t *testing.T) {
	mock := newMockAllowChecker(map[string]string{"ls": "list files"})
	validator, err := NewCommandValidator(ModeWhitelist, mock, true) // askLLMOnUnknown=true
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name         string
		command      string
		llmDangerous bool
		wantAllowed  bool
		wantDanger   bool
		wantConfirm  bool
	}{
		{
			name:         "non-whitelisted safe command needs confirm",
			command:      "echo hello",
			llmDangerous: false,
			wantAllowed:  true,
			wantDanger:   false,
			wantConfirm:  true,
		},
		{
			name:         "non-whitelisted dangerous command (LLM) needs confirm",
			command:      "custom-dangerous-cmd",
			llmDangerous: true,
			wantAllowed:  true,
			wantDanger:   true,
			wantConfirm:  true,
		},
		{
			name:         "non-whitelisted dangerous command (pattern) needs confirm",
			command:      "rm -rf /",
			llmDangerous: false,
			wantAllowed:  true,
			wantDanger:   true,
			wantConfirm:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.command, tt.llmDangerous)
			if result.Allowed != tt.wantAllowed {
				t.Errorf("Allowed: got %v, want %v", result.Allowed, tt.wantAllowed)
			}
			if result.IsDangerous != tt.wantDanger {
				t.Errorf("IsDangerous: got %v, want %v", result.IsDangerous, tt.wantDanger)
			}
			if result.NeedsConfirm != tt.wantConfirm {
				t.Errorf("NeedsConfirm: got %v, want %v", result.NeedsConfirm, tt.wantConfirm)
			}
		})
	}
}

func TestCommandValidator_BlacklistMode_Safe(t *testing.T) {
	validator, err := NewCommandValidator(ModeBlacklist, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name    string
		command string
	}{
		{"ls command", "ls -la"},
		{"cat command", "cat file.txt"},
		{"grep command", "grep -r 'pattern' ."},
		{"echo command", "echo hello"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.command, false)
			if !result.Allowed {
				t.Errorf("expected safe command to be allowed")
			}
			if result.IsDangerous {
				t.Errorf("expected safe command not to be flagged dangerous")
			}
			if result.NeedsConfirm {
				t.Errorf("expected safe command not to need confirmation")
			}
		})
	}
}

func TestCommandValidator_BlacklistMode_Dangerous(t *testing.T) {
	validator, err := NewCommandValidator(ModeBlacklist, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tests := []struct {
		name         string
		command      string
		llmDangerous bool
		wantDanger   bool
	}{
		{
			name:         "pattern-detected dangerous (rm -rf)",
			command:      "rm -rf /",
			llmDangerous: false,
			wantDanger:   true,
		},
		{
			name:         "pattern-detected dangerous (sudo)",
			command:      "sudo rm file",
			llmDangerous: false,
			wantDanger:   true,
		},
		{
			name:         "LLM-detected dangerous",
			command:      "some-custom-dangerous-command",
			llmDangerous: true,
			wantDanger:   true,
		},
		{
			name:         "both LLM and pattern detect",
			command:      "rm -rf /tmp/*",
			llmDangerous: true,
			wantDanger:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validator.Validate(tt.command, tt.llmDangerous)
			if result.IsDangerous != tt.wantDanger {
				t.Errorf("IsDangerous: got %v, want %v", result.IsDangerous, tt.wantDanger)
			}
			if tt.wantDanger && !result.NeedsConfirm {
				t.Error("expected dangerous command to need confirmation")
			}
			if result.IsDangerous && result.Reason == "" {
				t.Error("expected reason to be set for dangerous command")
			}
		})
	}
}

func TestCommandValidator_BlacklistMode_LLMOverride(t *testing.T) {
	validator, err := NewCommandValidator(ModeBlacklist, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Pattern detects dangerous, LLM does not
	result := validator.Validate("rm -rf /", false)
	if !result.IsDangerous {
		t.Error("expected pattern-detected command to be dangerous")
	}
	if result.Reason == "" || result.Reason == ErrMsgMarkedDangerousByAI {
		t.Error("expected pattern reason, not LLM reason")
	}

	// LLM detects dangerous, pattern does not
	result = validator.Validate("custom-safe-looking-cmd", true)
	if !result.IsDangerous {
		t.Error("expected LLM-detected command to be dangerous")
	}
	if result.Reason != ErrMsgMarkedDangerousByAI {
		t.Errorf("expected LLM reason, got: %s", result.Reason)
	}
}

func TestNewCommandValidator_WhitelistModeRequiresWhitelist(t *testing.T) {
	// Whitelist mode with nil whitelist should return an error
	_, err := NewCommandValidator(ModeWhitelist, nil, true)
	if err == nil {
		t.Fatal("expected error when whitelist mode has nil whitelist")
	}
	if !errors.Is(err, ErrWhitelistRequired) {
		t.Errorf("expected ErrWhitelistRequired, got: %v", err)
	}
}

func TestNewCommandValidator_BlacklistModeWithNilWhitelist(t *testing.T) {
	// Blacklist mode with nil whitelist should succeed
	validator, err := NewCommandValidator(ModeBlacklist, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if validator == nil {
		t.Fatal("expected non-nil validator")
	}
	if validator.Mode() != ModeBlacklist {
		t.Errorf("expected mode %v, got %v", ModeBlacklist, validator.Mode())
	}
}

func TestNewCommandValidator_WhitelistModeWithValidWhitelist(t *testing.T) {
	// Whitelist mode with valid whitelist should succeed
	mock := newMockAllowChecker(map[string]string{"ls": "list files"})
	validator, err := NewCommandValidator(ModeWhitelist, mock, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if validator == nil {
		t.Fatal("expected non-nil validator")
	}
	if validator.Mode() != ModeWhitelist {
		t.Errorf("expected mode %v, got %v", ModeWhitelist, validator.Mode())
	}
}

func TestValidationResult_Fields(t *testing.T) {
	result := ValidationResult{
		Allowed:      true,
		IsDangerous:  true,
		Reason:       "test reason",
		NeedsConfirm: true,
	}

	if !result.Allowed {
		t.Error("expected Allowed to be true")
	}
	if !result.IsDangerous {
		t.Error("expected IsDangerous to be true")
	}
	if result.Reason != "test reason" {
		t.Errorf("expected Reason 'test reason', got '%s'", result.Reason)
	}
	if !result.NeedsConfirm {
		t.Error("expected NeedsConfirm to be true")
	}
}

// Ensure CommandAllowChecker interface is properly implemented by mock.
var _ CommandAllowChecker = (*mockAllowChecker)(nil)

func TestCommandValidatorImpl_ConcurrentValidateWhitelistMode(t *testing.T) {
	mock := newMockAllowChecker(map[string]string{
		"ls":  "list files",
		"cat": "display file",
	})

	validator, err := NewCommandValidator(ModeWhitelist, mock, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const numGoroutines = 100
	const numIterations = 100

	errCh := make(chan error, numGoroutines)
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for range numGoroutines {
		go func() {
			defer wg.Done()
			for j := range numIterations {
				cmd := "ls"
				if j%2 == 0 {
					cmd = "cat"
				}
				result := validator.Validate(cmd, false)
				if !result.Allowed {
					errCh <- errors.New("whitelisted command should be allowed")
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}
}

func TestCommandValidatorImpl_ConcurrentValidateBlacklistMode(t *testing.T) {
	validator, err := NewCommandValidator(ModeBlacklist, nil, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const numGoroutines = 100
	const numIterations = 100

	errCh := make(chan error, numGoroutines)
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	// Mix of safe and dangerous commands
	commands := []struct {
		cmd        string
		wantDanger bool
	}{
		{"ls -la", false},
		{"cat file.txt", false},
		{"rm -rf /", true},
		{"sudo rm file", true},
		{"echo hello", false},
	}

	for range numGoroutines {
		go func() {
			defer wg.Done()
			for j := range numIterations {
				tc := commands[j%len(commands)]
				result := validator.Validate(tc.cmd, false)
				if result.IsDangerous != tc.wantDanger {
					errCh <- errors.New("inconsistent danger detection")
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}
}

func TestCommandValidatorImpl_ConcurrentValidate(t *testing.T) {
	mock := newMockAllowChecker(map[string]string{
		"ls":   "list files",
		"cat":  "display file",
		"grep": "search",
	})

	validator, err := NewCommandValidator(ModeWhitelist, mock, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const numGoroutines = 50
	const numIterations = 200

	errCh := make(chan error, numGoroutines)
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	commands := []string{"ls", "cat", "grep", "rm", "unknown"}

	for range numGoroutines {
		go func() {
			defer wg.Done()
			for j := range numIterations {
				cmd := commands[j%len(commands)]
				llmDangerous := j%3 == 0
				result := validator.Validate(cmd, llmDangerous)
				isWhitelisted := cmd == "ls" || cmd == "cat" || cmd == "grep"
				if isWhitelisted != result.Allowed {
					errCh <- errors.New("inconsistent result for whitelisted command")
					return
				}
			}
		}()
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Errorf("concurrent error: %v", err)
	}
}

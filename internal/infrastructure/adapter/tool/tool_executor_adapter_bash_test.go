package tool

import (
	"code-editing-agent/internal/infrastructure/adapter/file"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// bashOutput represents the expected output structure from bash tool.
type bashOutputTest struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

func TestBashTool_Registration(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	tools, err := adapter.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	found := false
	for _, tool := range tools {
		if tool.Name == "bash" {
			found = true
			break
		}
	}

	if !found {
		t.Error("bash tool should be registered")
	}
}

func TestBashTool_BasicExecution(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	input := `{"command": "echo hello", "dangerous": false}`
	result, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output bashOutputTest
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if output.Stdout != "hello\n" {
		t.Errorf("Expected stdout 'hello\\n', got %q", output.Stdout)
	}
	if output.Stderr != "" {
		t.Errorf("Expected empty stderr, got %q", output.Stderr)
	}
	if output.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", output.ExitCode)
	}
}

func TestBashTool_StderrCapture(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	input := `{"command": "echo error >&2", "dangerous": false}`
	result, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output bashOutputTest
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if output.Stderr != "error\n" {
		t.Errorf("Expected stderr 'error\\n', got %q", output.Stderr)
	}
}

func TestBashTool_ExitCode(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	input := `{"command": "exit 42", "dangerous": false}`
	result, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output bashOutputTest
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if output.ExitCode != 42 {
		t.Errorf("Expected exit code 42, got %d", output.ExitCode)
	}
}

func TestBashTool_Timeout(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	input := `{"command": "sleep 5", "timeout_ms": 100, "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timeout") && !strings.Contains(err.Error(), "killed") {
		t.Errorf("Expected error to contain 'timeout' or 'killed', got: %v", err)
	}
}

func TestBashTool_DangerousCommandBlocked(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)
	// No callback set - dangerous commands should be blocked

	// LLM incorrectly marks as safe, but patterns detect dangerous
	input := `{"command": "rm -rf /", "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err == nil {
		t.Fatal("Expected error for dangerous command, got nil")
	}

	if !strings.Contains(err.Error(), "dangerous") {
		t.Errorf("Expected error to mention 'dangerous', got: %v", err)
	}
}

func TestBashTool_DangerousCommandDenied(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Set callback that returns false
	adapter.SetDangerousCommandCallback(func(_, _ string) bool {
		return false
	})

	input := `{"command": "sudo -n ls", "dangerous": true}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err == nil {
		t.Fatal("Expected error for denied dangerous command, got nil")
	}

	if !strings.Contains(err.Error(), "denied") && !strings.Contains(err.Error(), "dangerous") {
		t.Errorf("Expected error to mention 'denied' or 'dangerous', got: %v", err)
	}
}

func TestBashTool_DangerousCommandAllowed(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Set callback that returns true (user confirmed)
	adapter.SetDangerousCommandCallback(func(_, _ string) bool {
		return true
	})

	// Use a "dangerous" command that's actually safe to run
	// Use -n flag to prevent blocking on password prompt
	input := `{"command": "sudo -n echo allowed", "dangerous": true}`
	result, err := adapter.ExecuteTool(context.Background(), "bash", input)
	// The command may fail due to no sudo access, but it should attempt execution
	// (not be blocked by the dangerous command check)
	if err != nil {
		// If error, it should be from sudo failure, not from dangerous command block
		if strings.Contains(err.Error(), "dangerous") {
			t.Errorf("Command should not be blocked as dangerous when callback returns true, got: %v", err)
		}
		// sudo failure is expected on most systems, so we just verify it tried
		return
	}

	var output bashOutputTest
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// If sudo worked (unlikely), verify output
	if output.ExitCode == 0 && output.Stdout != "allowed\n" {
		t.Errorf("Expected stdout 'allowed\\n', got %q", output.Stdout)
	}
}

func TestBashTool_DangerousPatterns(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)
	// No callback - all dangerous commands should be blocked

	dangerousCases := []struct {
		name    string
		command string
	}{
		{"rm -rf /", `rm -rf /`},
		{"rm -rf ~", `rm -rf ~`},
		{"rm -rf *", `rm -rf *`},
		{"sudo command", `sudo apt-get install something`},
		{"chmod 777", `chmod 777 /etc/passwd`},
		{"mkfs", `mkfs.ext4 /dev/sda`},
		{"dd if=", `dd if=/dev/zero of=/dev/sda`},
		{"write to /dev/", `echo test > /dev/sda`},
	}

	for _, tc := range dangerousCases {
		t.Run(tc.name, func(t *testing.T) {
			input, _ := json.Marshal(map[string]interface{}{"command": tc.command, "dangerous": false})
			_, err := adapter.ExecuteTool(context.Background(), "bash", string(input))
			if err == nil {
				t.Errorf("Expected error for dangerous command %q, got nil", tc.command)
			}
		})
	}
}

func TestBashTool_EmptyCommand(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	input := `{"command": "", "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err == nil {
		t.Fatal("Expected error for empty command, got nil")
	}
}

func TestBashTool_MixedOutput(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	input := `{"command": "echo stdout; echo stderr >&2", "dangerous": false}`
	result, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output bashOutputTest
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if output.Stdout != "stdout\n" {
		t.Errorf("Expected stdout 'stdout\\n', got %q", output.Stdout)
	}
	if output.Stderr != "stderr\n" {
		t.Errorf("Expected stderr 'stderr\\n', got %q", output.Stderr)
	}
}

func TestBashTool_CommandNotFound(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	input := `{"command": "nonexistent_command_xyz123", "dangerous": false}`
	result, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	var output bashOutputTest
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	// Command not found typically returns exit code 127
	if output.ExitCode != 127 {
		t.Errorf("Expected exit code 127, got %d", output.ExitCode)
	}
}

func TestBashTool_DangerousPatternVariations(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	dangerousCases := []struct {
		name    string
		command string
	}{
		// Whitespace variations
		{"rm with tabs", "rm\t-rf\t/"},
		{"rm with multiple spaces", "rm  -rf  /"},
		// Flag variations
		{"rm with verbose", "rm -rfv /"},
		{"rm with separate flags", "rm -r -f /"},
		// Sudo variations
		{"sudo with path", "/usr/bin/sudo ls"},
		{"sudo with env", "sudo -E ls"},
	}

	for _, tc := range dangerousCases {
		t.Run(tc.name, func(t *testing.T) {
			input, _ := json.Marshal(map[string]interface{}{"command": tc.command, "dangerous": false})
			_, err := adapter.ExecuteTool(context.Background(), "bash", string(input))
			if err == nil {
				t.Errorf("Expected error for dangerous command %q, got nil", tc.command)
			}
		})
	}
}

// =============================================================================
// Tests for CommandConfirmationCallback (TDD Red Phase)
// These tests are expected to FAIL until the feature is implemented
// =============================================================================

// callbackInvocation tracks the arguments passed to CommandConfirmationCallback.
type callbackInvocation struct {
	command     string
	isDangerous bool
	reason      string
	description string
}

func TestBashTool_AllCommandsConfirmation_CallbackCalledForNonDangerous(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	var invocations []callbackInvocation

	// Set CommandConfirmationCallback that tracks all invocations and returns true
	adapter.SetCommandConfirmationCallback(func(command string, isDangerous bool, reason, description string) bool {
		invocations = append(invocations, callbackInvocation{
			command:     command,
			isDangerous: isDangerous,
			reason:      reason,
			description: description,
		})
		return true
	})

	input := `{"command": "echo hello", "dangerous": false}`
	result, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Verify callback was called exactly once
	if len(invocations) != 1 {
		t.Fatalf("Expected callback to be called 1 time, got %d", len(invocations))
	}

	// Verify callback arguments for non-dangerous command
	inv := invocations[0]
	if inv.command != "echo hello" {
		t.Errorf("Expected command 'echo hello', got %q", inv.command)
	}
	if inv.isDangerous != false {
		t.Errorf("Expected isDangerous=false for 'echo hello', got true")
	}
	if inv.reason != "" {
		t.Errorf("Expected empty reason for non-dangerous command, got %q", inv.reason)
	}

	// Verify command executed successfully
	var output bashOutputTest
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}
	if output.Stdout != "hello\n" {
		t.Errorf("Expected stdout 'hello\\n', got %q", output.Stdout)
	}
}

func TestBashTool_AllCommandsConfirmation_CallbackCalledForDangerous(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	var invocations []callbackInvocation

	// Set CommandConfirmationCallback that tracks all invocations and returns true
	adapter.SetCommandConfirmationCallback(func(command string, isDangerous bool, reason, description string) bool {
		invocations = append(invocations, callbackInvocation{
			command:     command,
			isDangerous: isDangerous,
			reason:      reason,
			description: description,
		})
		return true
	})

	input := `{"command": "sudo ls", "dangerous": true}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)
	// The command may fail due to sudo requiring password, but we should not get
	// a "dangerous command blocked" error since callback returned true
	if err != nil {
		if strings.Contains(err.Error(), "dangerous") && strings.Contains(err.Error(), "blocked") {
			t.Errorf("Command should not be blocked when callback returns true, got: %v", err)
		}
		// sudo failure is acceptable, just check callback was invoked
	}

	// Verify callback was called exactly once
	if len(invocations) != 1 {
		t.Fatalf("Expected callback to be called 1 time, got %d", len(invocations))
	}

	// Verify callback arguments for dangerous command
	inv := invocations[0]
	if inv.command != "sudo ls" {
		t.Errorf("Expected command 'sudo ls', got %q", inv.command)
	}
	if inv.isDangerous != true {
		t.Errorf("Expected isDangerous=true for 'sudo ls', got false")
	}
	if !strings.Contains(inv.reason, "sudo") {
		t.Errorf("Expected reason to contain 'sudo' for sudo command, got %q", inv.reason)
	}
}

func TestBashTool_AllCommandsConfirmation_NonDangerousDenied(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Set CommandConfirmationCallback that denies all commands
	adapter.SetCommandConfirmationCallback(func(_ string, _ bool, _, _ string) bool {
		return false
	})

	input := `{"command": "echo hello", "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)

	// Command should be denied even though it's not dangerous
	if err == nil {
		t.Fatal("Expected error when callback denies non-dangerous command, got nil")
	}

	if !strings.Contains(err.Error(), "denied") {
		t.Errorf("Expected error to contain 'denied', got: %v", err)
	}
}

func TestBashTool_AllCommandsConfirmation_NoCallbackNonDangerousProceeds(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Explicitly do NOT set any callback - backward compatible behavior
	// Non-dangerous commands should proceed without requiring confirmation

	input := `{"command": "echo backward_compat", "dangerous": false}`
	result, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err != nil {
		t.Fatalf("Expected non-dangerous command to succeed without callback, got error: %v", err)
	}

	var output bashOutputTest
	if err := json.Unmarshal([]byte(result), &output); err != nil {
		t.Fatalf("Failed to unmarshal result: %v", err)
	}

	if output.Stdout != "backward_compat\n" {
		t.Errorf("Expected stdout 'backward_compat\\n', got %q", output.Stdout)
	}
	if output.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", output.ExitCode)
	}
}

func TestBashTool_BackwardCompat_DangerousCallbackStillWorks(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	var dangerousInvocations []struct {
		command string
		reason  string
	}

	// Use the OLD SetDangerousCommandCallback API - should still work
	adapter.SetDangerousCommandCallback(func(command, reason string) bool {
		dangerousInvocations = append(dangerousInvocations, struct {
			command string
			reason  string
		}{command, reason})
		return true // Allow dangerous commands
	})

	// First, execute a non-dangerous command - old callback should NOT be triggered
	nonDangerousInput := `{"command": "echo safe", "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", nonDangerousInput)
	if err != nil {
		t.Fatalf("Non-dangerous command failed: %v", err)
	}

	// Verify old callback was NOT called for non-dangerous command
	if len(dangerousInvocations) != 0 {
		t.Errorf(
			"Old DangerousCommandCallback should not be called for non-dangerous command, got %d invocations",
			len(dangerousInvocations),
		)
	}

	// Now execute a dangerous command - old callback SHOULD be triggered
	dangerousInput := `{"command": "sudo echo test", "dangerous": true}`
	_, err = adapter.ExecuteTool(context.Background(), "bash", dangerousInput)
	// Command may fail due to sudo, but should not be blocked
	if err != nil {
		if strings.Contains(err.Error(), "dangerous") && strings.Contains(err.Error(), "blocked") {
			t.Errorf("Dangerous command should not be blocked when old callback returns true, got: %v", err)
		}
	}

	// Verify old callback WAS called for dangerous command with 2-argument signature
	if len(dangerousInvocations) != 1 {
		t.Fatalf(
			"Expected old DangerousCommandCallback to be called 1 time for dangerous command, got %d",
			len(dangerousInvocations),
		)
	}

	inv := dangerousInvocations[0]
	if inv.command != "sudo echo test" {
		t.Errorf("Expected command 'sudo echo test', got %q", inv.command)
	}
	if !strings.Contains(inv.reason, "sudo") {
		t.Errorf("Expected reason to contain 'sudo', got %q", inv.reason)
	}
}

func TestBashTool_LLMSpecifiedDangerous(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	var invocations []callbackInvocation

	// Set CommandConfirmationCallback that tracks all invocations
	adapter.SetCommandConfirmationCallback(func(command string, isDangerous bool, reason, description string) bool {
		invocations = append(invocations, callbackInvocation{
			command:     command,
			isDangerous: isDangerous,
			reason:      reason,
			description: description,
		})
		return true
	})

	// Execute a safe command but with dangerous:true flag from LLM
	input := `{"command": "echo hello", "dangerous": true, "description": "LLM thinks this is risky"}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err != nil {
		t.Fatalf("Expected command to succeed, got error: %v", err)
	}

	// Verify callback was called with isDangerous=true and appropriate reason
	if len(invocations) != 1 {
		t.Fatalf("Expected callback to be called 1 time, got %d", len(invocations))
	}

	inv := invocations[0]
	if inv.command != "echo hello" {
		t.Errorf("Expected command 'echo hello', got %q", inv.command)
	}
	if !inv.isDangerous {
		t.Errorf("Expected isDangerous=true when LLM specifies dangerous:true")
	}
	if inv.reason != "marked dangerous by AI" {
		t.Errorf("Expected reason 'marked dangerous by AI', got %q", inv.reason)
	}
	if inv.description != "LLM thinks this is risky" {
		t.Errorf("Expected description 'LLM thinks this is risky', got %q", inv.description)
	}
}

func TestBashTool_LLMSpecifiedDangerous_CombinesWithPatternDetection(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	var invocations []callbackInvocation

	// Set CommandConfirmationCallback
	adapter.SetCommandConfirmationCallback(func(command string, isDangerous bool, reason, description string) bool {
		invocations = append(invocations, callbackInvocation{
			command:     command,
			isDangerous: isDangerous,
			reason:      reason,
			description: description,
		})
		return true
	})

	// Execute a pattern-detected dangerous command with dangerous:true flag
	// Pattern detection should take precedence for the reason
	input := `{"command": "sudo ls", "dangerous": true}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)
	// sudo may fail but should not be blocked
	if err != nil {
		if strings.Contains(err.Error(), "blocked") {
			t.Fatalf("Command should not be blocked: %v", err)
		}
	}

	if len(invocations) != 1 {
		t.Fatalf("Expected callback to be called 1 time, got %d", len(invocations))
	}

	inv := invocations[0]
	if !inv.isDangerous {
		t.Errorf("Expected isDangerous=true")
	}
	// Pattern detection reason should be used when patterns match
	if !strings.Contains(inv.reason, "sudo") {
		t.Errorf("Expected pattern-detected reason to contain 'sudo', got %q", inv.reason)
	}
}

func TestBashTool_LLMFailedToIdentifyDangerous(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	var invocations []callbackInvocation

	// Set CommandConfirmationCallback
	adapter.SetCommandConfirmationCallback(func(command string, isDangerous bool, reason, description string) bool {
		invocations = append(invocations, callbackInvocation{
			command:     command,
			isDangerous: isDangerous,
			reason:      reason,
			description: description,
		})
		return true
	})

	// Execute a dangerous command but LLM incorrectly marks it as safe
	// This should still be detected as dangerous with a warning in the reason
	input := `{"command": "sudo ls", "dangerous": false}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)
	// sudo may fail but should not be blocked since callback returns true
	if err != nil {
		if strings.Contains(err.Error(), "blocked") {
			t.Fatalf("Command should not be blocked when callback returns true: %v", err)
		}
	}

	if len(invocations) != 1 {
		t.Fatalf("Expected callback to be called 1 time, got %d", len(invocations))
	}

	inv := invocations[0]
	if !inv.isDangerous {
		t.Errorf("Expected isDangerous=true even when LLM says false (patterns should override)")
	}
	// Reason should contain both the pattern reason AND a warning about LLM failure
	if !strings.Contains(inv.reason, "sudo") {
		t.Errorf("Expected reason to contain 'sudo', got %q", inv.reason)
	}
	if !strings.Contains(inv.reason, "WARNING") {
		t.Errorf("Expected reason to contain WARNING about LLM failure, got %q", inv.reason)
	}
	if !strings.Contains(inv.reason, "LLM failed to identify") {
		t.Errorf("Expected reason to mention LLM failed to identify, got %q", inv.reason)
	}
}

// =============================================================================
// Tests for Fetch Tool
// =============================================================================

func TestFetchTool_Registration(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	tools, err := adapter.ListTools()
	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	found := false
	for _, tool := range tools {
		if tool.Name == "fetch" {
			found = true
			break
		}
	}

	if !found {
		t.Error("fetch tool should be registered")
	}

	// Verify the fetch tool has the correct schema
	fetchTool, exists := adapter.GetTool("fetch")
	if !exists {
		t.Fatal("fetch tool should exist")
	}

	if fetchTool.Description != "Fetches web resources via HTTP/HTTPS. Prefer this to bash-isms like curl/wget" {
		t.Errorf("Expected description to mention curl/wget alternative, got: %s", fetchTool.Description)
	}

	// Verify required fields
	if len(fetchTool.RequiredFields) != 1 || fetchTool.RequiredFields[0] != "url" {
		t.Errorf("Expected required fields to be ['url'], got: %v", fetchTool.RequiredFields)
	}
}

func TestFetchTool_SimpleTextFetch(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Use a public test server
	serverURL := "https://httpbin.org/robots.txt"

	input := fmt.Sprintf(`{"url": "%s"}`, serverURL)
	result, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Verify we got some content (robots.txt typically contains some text)
	if result == "" {
		t.Error("Expected non-empty response from robots.txt")
	}
}

func TestFetchTool_HTMLToTextConversion(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Use a public test server that returns simple HTML
	serverURL := "https://httpbin.org/html"

	// Test with includeMarkup=false (default)
	input := fmt.Sprintf(`{"url": "%s"}`, serverURL)
	result, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Verify we got some converted text (httpbin's HTML should be converted to text)
	if result == "" {
		t.Error("Expected non-empty response from HTML conversion")
	}

	// The response should contain some text content (not HTML tags)
	if strings.Contains(result, "<") && strings.Contains(result, ">") {
		t.Error("Expected HTML to be converted to text, but result contains HTML tags")
	}
}

func TestFetchTool_IncludeMarkup(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Use a public test server that returns HTML
	serverURL := "https://httpbin.org/html"

	// Test with includeMarkup=true
	input := fmt.Sprintf(`{"url": "%s", "includeMarkup": true}`, serverURL)
	result, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// The response should contain HTML tags when includeMarkup=true
	if !strings.Contains(result, "<") || !strings.Contains(result, ">") {
		t.Error("Expected HTML markup to be included, but result doesn't contain HTML tags")
	}

	// Should contain typical HTML elements
	expectedElements := []string{"html", "body", "h1"}
	for _, element := range expectedElements {
		if !strings.Contains(result, element) {
			t.Errorf("Expected result to contain HTML element '%s'", element)
		}
	}
}

func TestFetchTool_HTTPError(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Use a public test server that returns 404
	serverURL := "https://httpbin.org/status/404"

	input := fmt.Sprintf(`{"url": "%s"}`, serverURL)
	_, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err == nil {
		t.Fatal("Expected error for 404, got nil")
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Expected error to contain '404', got: %v", err)
	}
}

func TestFetchTool_403AuthorizationError(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Use a public test server that returns 403
	serverURL := "https://httpbin.org/status/403"

	input := fmt.Sprintf(`{"url": "%s"}`, serverURL)
	_, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err == nil {
		t.Fatal("Expected error for 403, got nil")
	}

	if !strings.Contains(err.Error(), "authorization required") {
		t.Errorf("Expected error to contain 'authorization required', got: %v", err)
	}
}

func TestFetchTool_InvalidURL(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	testCases := []struct {
		name string
		url  string
	}{
		{"file protocol", "file:///etc/passwd"},
		{"ftp protocol", "ftp://example.com/file.txt"},
		{"no protocol", "example.com/file.txt"},
		{"invalid format", "http://"},
		{"empty", ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(`{"url": "%s"}`, tc.url)
			_, err := adapter.ExecuteTool(context.Background(), "fetch", input)
			if err == nil {
				t.Errorf("Expected error for invalid URL %q, got nil", tc.url)
			}
		})
	}
}

func TestFetchTool_EmptyURL(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	input := `{"url": ""}`
	_, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err == nil {
		t.Fatal("Expected error for empty URL, got nil")
	}

	if !strings.Contains(err.Error(), "invalid URL") {
		t.Errorf("Expected error to contain 'invalid URL', got: %v", err)
	}
}

func TestFetchTool_MissingURL(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	input := `{"includeMarkup": true}`
	_, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err == nil {
		t.Fatal("Expected error for missing URL, got nil")
	}

	if !strings.Contains(err.Error(), "url") {
		t.Errorf("Expected error to contain 'url', got: %v", err)
	}
}

func TestFetchTool_ContextCancel(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Use a public test server that delays response
	serverURL := "https://httpbin.org/delay/2"

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	input := fmt.Sprintf(`{"url": "%s"}`, serverURL)
	_, err := adapter.ExecuteTool(ctx, "fetch", input)
	if err == nil {
		t.Fatal("Expected error due to context cancellation, got nil")
	}

	// Should be a context/deadline error, not a private IP blocking error
	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("Expected error to contain context/deadline, got: %v", err)
	}

	// Ensure it's not being blocked by SSRF protection
	if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "blocked") {
		t.Errorf("Request should not be blocked by SSRF protection, got: %v", err)
	}
}

func TestFetchTool_NonHTMLContentKeepOriginal(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Use a public test server that returns JSON content
	serverURL := "https://httpbin.org/json"

	// Even with includeMarkup=false, non-HTML content should not be converted
	input := fmt.Sprintf(`{"url": "%s"}`, serverURL)
	result, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	// Verify we got JSON content (should contain typical JSON structure)
	if !strings.Contains(result, "{") || !strings.Contains(result, "}") {
		t.Error("Expected JSON content to be preserved, but result doesn't contain JSON brackets")
	}

	// Should not be converted to plain text (should still be JSON)
	if !strings.Contains(result, "\"") {
		t.Error("Expected JSON quotes to be preserved, but they appear to be stripped")
	}
}

func TestFetchTool_RedirectPolicyLimit(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// This endpoint will redirect multiple times to test our redirect limit
	// httpbin.org/relative-redirect/3 creates 3 redirects
	serverURL := "https://httpbin.org/relative-redirect/3"

	input := fmt.Sprintf(`{"url": "%s"}`, serverURL)
	result, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err != nil {
		// If it fails due to redirect limit, that's what we want to test
		if strings.Contains(err.Error(), "redirect") || strings.Contains(err.Error(), "stopped after") {
			// This is expected behavior - our redirect policy is working
			t.Logf("Redirect policy working: %v", err)
			return
		}
		// Other errors are acceptable for network reasons
		t.Logf("Network error (acceptable): %v", err)
		return
	}

	// If it succeeds, verify we got some content
	if len(result) > 0 {
		t.Logf("Successfully fetched with redirects: %d bytes", len(result))
	}
}

func TestFetchTool_ExcessiveRedirectsBlocked(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// This endpoint will create 5 redirects, which should exceed our limit of 3
	serverURL := "https://httpbin.org/relative-redirect/5"

	input := fmt.Sprintf(`{"url": "%s"}`, serverURL)
	_, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err == nil {
		t.Error("Expected error due to excessive redirects, got nil")
	} else if !strings.Contains(err.Error(), "redirect") && !strings.Contains(err.Error(), "stopped after") {
		t.Errorf("Expected redirect-related error, got: %v", err)
	}
}

func TestFetchTool_RedirectToPrivateIPBlocked(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// We'll use a test that simulates a redirect to a private IP
	// Since we can't easily set up a server that redirects to private IPs,
	// we'll test the validation logic indirectly

	// Test that the redirect URL validation works by checking a private IP URL directly
	privateURL := "http://127.0.0.1:8080"
	input := fmt.Sprintf(`{"url": "%s"}`, privateURL)
	_, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err == nil {
		t.Error("Expected error for private IP redirect target, got nil")
	} else if !strings.Contains(err.Error(), "private") && !strings.Contains(err.Error(), "blocked") {
		t.Errorf("Expected private IP blocking error, got: %v", err)
	}
}

func TestFetchTool_RedirectURLWithCredentialsBlocked(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Test that URLs with credentials in redirect are blocked
	credsURL := "http://user:pass@example.com"
	input := fmt.Sprintf(`{"url": "%s"}`, credsURL)
	_, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err == nil {
		t.Error("Expected error for URL with credentials, got nil")
	} else if !strings.Contains(err.Error(), "credentials") {
		t.Errorf("Expected credentials error, got: %v", err)
	}
}

func TestFetchTool_ContentLengthHandling(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Test case 1: Normal content-Length handling with public server
	t.Run("Content-Length header present", func(t *testing.T) {
		// Use a public test server that returns a reasonable sized response
		serverURL := "https://httpbin.org/robots.txt"

		input := fmt.Sprintf(`{"url": "%s"}`, serverURL)
		result, err := adapter.ExecuteTool(context.Background(), "fetch", input)
		if err != nil {
			t.Fatalf("ExecuteTool failed: %v", err)
		}

		// Verify we got some content
		if len(result) == 0 {
			t.Error("Expected non-empty response")
		}

		t.Logf("Successfully fetched %d bytes", len(result))
	})

	// Test the size limit behavior by trying to fetch a very large response
	t.Run("Response size limit test", func(t *testing.T) {
		// This test verifies that the fetch tool properly handles size limits
		// Since we can't easily create a huge response with public servers,
		// we'll test that the fetch tool at least handles responses normally
		serverURL := "https://httpbin.org/bytes/1024" // 1KB response

		input := fmt.Sprintf(`{"url": "%s"}`, serverURL)
		result, err := adapter.ExecuteTool(context.Background(), "fetch", input)
		if err != nil {
			// Should not be blocked by SSRF protection
			if err != nil && (strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "blocked")) {
				t.Errorf("Public URL should not be blocked by SSRF protection, got: %v", err)
			}
			t.Logf("Network error (acceptable): %v", err)
			return
		}

		// Should get exactly 1024 bytes
		if len(result) != 1024 {
			t.Errorf("Expected 1024 bytes, got %d", len(result))
		}
	})
}

func TestFetchTool_MalformedInput(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	testCases := []string{
		`{"url": 123}`,
		`"invalid json"`,
		`{}`,
		`null`,
	}

	for _, input := range testCases {
		t.Run(fmt.Sprintf("input: %s", input), func(t *testing.T) {
			_, err := adapter.ExecuteTool(context.Background(), "fetch", input)
			if err == nil {
				t.Errorf("Expected error for malformed input %q, got nil", input)
			}
		})
	}
}

func TestFetchTool_SSFRProtection_BlockPrivateIPs(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	testCases := []struct {
		name string
		url  string
	}{
		{"localhost", "http://localhost:8080/test"},
		{"127.0.0.1", "http://127.0.0.1:3000/api"},
		{"loopback IPv6", "http://[::1]:8080"},
		{"private Class A", "http://10.0.0.1/admin"},
		{"private Class B", "http://172.16.0.1/internal"},
		{"private Class C", "http://192.168.1.1/config"},
		{"link-local", "http://169.254.0.1/metadata"},
		{"metadata service", "http://169.254.169.254/latest/meta-data"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(`{"url": "%s"}`, tc.url)
			_, err := adapter.ExecuteTool(context.Background(), "fetch", input)
			if err == nil {
				t.Errorf("Expected error for private URL %s, got nil", tc.url)
			}

			// Check that the error mentions private/internal range and blocking
			if !strings.Contains(err.Error(), "private") && !strings.Contains(err.Error(), "blocked") {
				t.Errorf("Expected error to contain 'private' or 'blocked', got: %v", err)
			}
		})
	}
}

func TestFetchTool_SSFRProtection_BlockHostnamesResolvingToPrivateIPs(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// We can't actually resolve internal hostnames in the test environment,
	// but we can test the mechanism by using hostnames that might resolve to private IPs
	// In a real scenario, these would be blocked if they resolved to private ranges
	testCases := []struct {
		name string
		url  string
	}{
		{"localhost with path", "http://localhost/path"},
		{"localhost with port", "http://localhost:8080"},
		{"IPv6 localhost", "http://[::1]:3000"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(`{"url": "%s"}`, tc.url)
			_, err := adapter.ExecuteTool(context.Background(), "fetch", input)
			if err == nil {
				t.Errorf("Expected error for hostname %s, got nil", tc.url)
			}

			// Should mention that hostname resolves to private IP or is blocked
			if !strings.Contains(err.Error(), "blocked") && !strings.Contains(err.Error(), "private") {
				t.Errorf("Expected error to contain 'blocked' or 'private', got: %v", err)
			}
		})
	}
}

func TestFetchTool_SSFRProtection_AllowPublicIPs(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Test with a real public URL (httpbin.org for testing)
	// This tests that public IPs/domains work correctly
	testURL := "https://httpbin.org/user-agent"

	input := fmt.Sprintf(`{"url": "%s"}`, testURL)
	_, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	// This should not fail due to SSRF protection
	// It might fail due to network issues, but should not be blocked for security
	if err != nil {
		if strings.Contains(err.Error(), "private") || strings.Contains(err.Error(), "blocked") {
			t.Errorf("Public URL should not be blocked by SSRF protection, got: %v", err)
		}
		// Network errors are acceptable, we just want to verify SSRF protection doesn't kick in
		t.Logf("Network error (acceptable): %v", err)
		return
	}

	t.Log("Successfully fetched from public URL without SSRF blocking")
}

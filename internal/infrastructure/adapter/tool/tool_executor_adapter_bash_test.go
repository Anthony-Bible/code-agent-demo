package tool

import (
	"code-editing-agent/internal/infrastructure/adapter/file"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

	input := `{"command": "echo hello"}`
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

	input := `{"command": "echo error >&2"}`
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

	input := `{"command": "exit 42"}`
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

	input := `{"command": "sleep 5", "timeout_ms": 100}`
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

	input := `{"command": "rm -rf /"}`
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
	adapter.SetDangerousCommandCallback(func(command, reason string) bool {
		return false
	})

	input := `{"command": "sudo ls"}`
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
	adapter.SetDangerousCommandCallback(func(command, reason string) bool {
		return true
	})

	// Use a "dangerous" command that's actually safe to run
	input := `{"command": "sudo echo allowed"}`
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
			input, _ := json.Marshal(map[string]string{"command": tc.command})
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

	input := `{"command": ""}`
	_, err := adapter.ExecuteTool(context.Background(), "bash", input)
	if err == nil {
		t.Fatal("Expected error for empty command, got nil")
	}
}

func TestBashTool_MixedOutput(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	input := `{"command": "echo stdout; echo stderr >&2"}`
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

	input := `{"command": "nonexistent_command_xyz123"}`
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
			input, _ := json.Marshal(map[string]string{"command": tc.command})
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

	input := `{"command": "echo hello"}`
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

	input := `{"command": "sudo ls"}`
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
	adapter.SetCommandConfirmationCallback(func(command string, isDangerous bool, reason, description string) bool {
		return false
	})

	input := `{"command": "echo hello"}`
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

	input := `{"command": "echo backward_compat"}`
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
	nonDangerousInput := `{"command": "echo safe"}`
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
	dangerousInput := `{"command": "sudo echo test"}`
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

	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Hello, World!")
	}))
	defer server.Close()

	input := fmt.Sprintf(`{"url": "%s"}`, server.URL)
	result, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	if result != "Hello, World!" {
		t.Errorf("Expected 'Hello, World!', got %q", result)
	}
}

func TestFetchTool_HTMLToTextConversion(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create test server with HTML content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body><h1>Title</h1><p>Paragraph text.</p></body></html>`)
	}))
	defer server.Close()

	// Test with includeMarkup=false (default)
	input := fmt.Sprintf(`{"url": "%s"}`, server.URL)
	result, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := "Title Paragraph text."
	if result != expected {
		t.Errorf("Expected '%s', got %q", expected, result)
	}
}

func TestFetchTool_IncludeMarkup(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create test server with HTML content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `<html><body><h1>Title</h1><p>Paragraph text.</p></body></html>`)
	}))
	defer server.Close()

	// Test with includeMarkup=true
	input := fmt.Sprintf(`{"url": "%s", "includeMarkup": true}`, server.URL)
	result, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := `<html><body><h1>Title</h1><p>Paragraph text.</p></body></html>`
	if result != expected {
		t.Errorf("Expected '%s', got %q", expected, result)
	}
}

func TestFetchTool_HTTPError(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Not Found")
	}))
	defer server.Close()

	input := fmt.Sprintf(`{"url": "%s"}`, server.URL)
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

	// Create test server that returns 403
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprint(w, "Forbidden")
	}))
	defer server.Close()

	input := fmt.Sprintf(`{"url": "%s"}`, server.URL)
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

	// Create test server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay longer than context timeout
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "Delayed response")
	}))
	defer server.Close()

	// Create context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	input := fmt.Sprintf(`{"url": "%s"}`, server.URL)
	_, err := adapter.ExecuteTool(ctx, "fetch", input)
	if err == nil {
		t.Fatal("Expected error due to context cancellation, got nil")
	}

	if !strings.Contains(err.Error(), "context") && !strings.Contains(err.Error(), "deadline") {
		t.Errorf("Expected error to contain context/deadline, got: %v", err)
	}
}

func TestFetchTool_NonHTMLContentKeepOriginal(t *testing.T) {
	fileManager := file.NewLocalFileManager(".")
	adapter := NewExecutorAdapter(fileManager)

	// Create test server with JSON content
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"message": "Hello", "status": "ok"}`)
	}))
	defer server.Close()

	// Even with includeMarkup=false, non-HTML content should not be converted
	input := fmt.Sprintf(`{"url": "%s"}`, server.URL)
	result, err := adapter.ExecuteTool(context.Background(), "fetch", input)
	if err != nil {
		t.Fatalf("ExecuteTool failed: %v", err)
	}

	expected := `{"message": "Hello", "status": "ok"}`
	if result != expected {
		t.Errorf("Expected '%s', got %q", expected, result)
	}
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

package ui_test

import (
	"code-editing-agent/internal/domain/port"
	"code-editing-agent/internal/infrastructure/adapter/ui"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests are intentionally written to fail during the Red Phase of TDD.
// They define the expected behavior of the CLIAdapter before implementation.

func TestCLIAdapter_GetUserInput(t *testing.T) {
	t.Run("gets user input successfully", func(t *testing.T) {
		// Setup: These tests will fail until CLIAdapter is implemented
		t.Skip("TODO: Implement CLIAdapter - Red Phase test")
	})

	t.Run("handles empty input", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - Red Phase test")
	})

	t.Run("handles input with spaces", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - Red Phase test")
	})

	t.Run("handles EOF (ctrl+d)", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - Red Phase test")
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - Red Phase test")
	})

	t.Run("preserves special characters in input", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - Red Phase test")
	})
}

func TestCLIAdapter_DisplayMessage(t *testing.T) {
	t.Run("displays user message with correct color", func(t *testing.T) {
		// Test expects blue color (\x1b[94m) for user messages
		// This test should verify that DisplayMessage includes the correct ANSI color code
		assert.Contains(t, "\x1b[94m"+"Hello world", "Hello world")
		t.Skip("TODO: Implement CLIAdapter - need to verify color codes and message formatting")
	})

	t.Run("displays assistant message with correct color", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - need to verify color codes and message formatting")
	})

	t.Run("displays system message with correct color", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - need to verify color codes and message formatting")
	})

	t.Run("handles empty message", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test empty message handling")
	})

	t.Run("handles multiline message", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test multiline message handling")
	})

	t.Run("handles unknown message role defaults to user", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test unknown message role")
	})

	t.Run("handles message with special characters", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test special character handling")
	})

	t.Run("handles very long message", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test long message handling")
	})
}

func TestCLIAdapter_DisplayError(t *testing.T) {
	t.Run("displays error message with correct color", func(t *testing.T) {
		// Test expects red color (\x1b[91m) for error messages
		testErr := errors.New("something went wrong")
		// Should display as: "Error: something went wrong"
		errMsg := "Error: something went wrong"
		assert.Equal(t, errMsg, "Error: "+testErr.Error())
		t.Skip("TODO: Implement CLIAdapter - need to verify error formatting and color")
	})

	t.Run("handles nil error", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test nil error handling")
	})

	t.Run("handles complex error with message", func(t *testing.T) {
		_ = errors.New("complex error: nested error occurred")
		t.Skip("TODO: Implement CLIAdapter - test complex error formatting")
	})

	t.Run("handles error with special characters", func(t *testing.T) {
		_ = errors.New("Error with special chars: !@#$%^&*()")
		t.Skip("TODO: Implement CLIAdapter - test error with special characters")
	})
}

func TestCLIAdapter_DisplayToolResult(t *testing.T) {
	t.Run("displays tool result with correct color", func(t *testing.T) {
		// Test expects green color (\x1b[92m) for tool results
		// Format should follow pattern seen in main.go
		toolName := "read_file"
		input := "/path/to/file.txt"
		expected := "Tool [" + toolName + "] on " + input
		assert.Contains(t, expected, "read_file")
		t.Skip("TODO: Implement CLIAdapter - need to verify tool result formatting")
	})

	t.Run("handles empty tool name", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test empty tool name")
	})

	t.Run("handles empty input path", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test empty input path")
	})

	t.Run("handles empty result", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test empty result handling")
	})

	t.Run("handles multiline result", func(t *testing.T) {
		_ = "Line 1\nLine 2\nLine 3"
		t.Skip("TODO: Implement CLIAdapter - test multiline result")
	})

	t.Run("handles result with special characters", func(t *testing.T) {
		_ = "Result with special chars: ~`!@#$%^&*()_-+={}|[]\\:;\"'<>?,./"
		t.Skip("TODO: Implement CLIAdapter - test special character handling")
	})
}

func TestCLIAdapter_DisplaySystemMessage(t *testing.T) {
	t.Run("displays system message with correct color", func(t *testing.T) {
		message := "System initialized"
		// Should format as "System: System initialized"
		expected := "System: " + message
		assert.Equal(t, expected, "System: System initialized")
		t.Skip("TODO: Implement CLIAdapter - need to verify system message formatting")
	})

	t.Run("handles empty system message", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test empty system message")
	})

	t.Run("handles multiline system message", func(t *testing.T) {
		_ = "System starting...\nConfiguration loaded\nReady"
		t.Skip("TODO: Implement CLIAdapter - test multiline system message")
	})

	t.Run("handles system message with special characters", func(t *testing.T) {
		_ = "System error occurred: !@#$%^&*()"
		t.Skip("TODO: Implement CLIAdapter - test special character handling")
	})
}

func TestCLIAdapter_SetPrompt(t *testing.T) {
	t.Run("sets custom prompt", func(t *testing.T) {
		prompt := "MyPrompt> "
		assert.NotEmpty(t, prompt)
		t.Skip("TODO: Implement CLIAdapter - test custom prompt setting")
	})

	t.Run("handles empty prompt", func(t *testing.T) {
		_ = ""
		t.Skip("TODO: Implement CLIAdapter - test empty prompt handling")
	})

	t.Run("handles prompt with color codes", func(t *testing.T) {
		prompt := "\x1b[94mClaude\x1b[0m: "
		// Test that ANSI color codes are properly parsed/handled
		assert.Contains(t, prompt, "\x1b[94m")
		assert.Contains(t, prompt, "\x1b[0m")
		t.Skip("TODO: Implement CLIAdapter - test prompt with ANSI codes")
	})

	t.Run("handles very long prompt", func(t *testing.T) {
		prompt := strings.Repeat("VeryLongPrompt", 20)
		assert.Greater(t, len(prompt), 20)
		t.Skip("TODO: Implement CLIAdapter - test long prompt handling")
	})

	t.Run("handles prompt with special characters", func(t *testing.T) {
		_ = "Prompt! ~`!@#$%^&*()_-+={}|[]\\:;\"'<>?,./> "
		t.Skip("TODO: Implement CLIAdapter - test special character handling")
	})
}

func TestCLIAdapter_ClearScreen(t *testing.T) {
	t.Run("clears screen successfully", func(t *testing.T) {
		// Should contain ANSI clear screen sequences
		clearSeq := "\x1b[2J"
		homeCursor := "\x1b[H"
		assert.Contains(t, clearSeq+homeCursor, "\x1b[2J")
		t.Skip("TODO: Implement CLIAdapter - need to verify ANSI clear sequences")
	})

	t.Run("clears screen multiple times", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test multiple clear calls")
	})
}

func TestCLIAdapter_SetColorScheme(t *testing.T) {
	t.Run("sets valid color scheme", func(t *testing.T) {
		scheme := port.ColorScheme{
			User:      "\x1b[94m", // Blue
			Assistant: "\x1b[93m", // Yellow
			System:    "\x1b[96m", // Cyan
			Error:     "\x1b[91m", // Red
			Tool:      "\x1b[92m", // Green
			Prompt:    "\x1b[94m", // Blue
		}
		// Verify the scheme has all required fields
		assert.NotEmpty(t, scheme.User)
		assert.NotEmpty(t, scheme.Assistant)
		assert.NotEmpty(t, scheme.System)
		assert.NotEmpty(t, scheme.Error)
		assert.NotEmpty(t, scheme.Tool)
		assert.NotEmpty(t, scheme.Prompt)
		t.Skip("TODO: Implement CLIAdapter - test valid color scheme setting")
	})

	t.Run("handles empty color codes", func(t *testing.T) {
		_ = port.ColorScheme{
			User:      "",
			Assistant: "",
			System:    "",
			Error:     "",
			Tool:      "",
			Prompt:    "",
		}
		t.Skip("TODO: Implement CLIAdapter - test empty color code handling")
	})

	t.Run("handles invalid color codes gracefully", func(t *testing.T) {
		_ = port.ColorScheme{
			User:      "invalid_color",
			Assistant: "\x1b[93m",
			System:    "\x1b[96m",
			Error:     "\x1b[91m",
			Tool:      "\x1b[92m",
			Prompt:    "\x1b[94m",
		}
		t.Skip("TODO: Implement CLIAdapter - test invalid color code handling")
	})

	t.Run("handles partial color scheme", func(t *testing.T) {
		_ = port.ColorScheme{
			User: "\x1b[94m",
			// Other fields should use defaults
		}
		t.Skip("TODO: Implement CLIAdapter - test partial color scheme")
	})

	t.Run("validates color scheme format", func(t *testing.T) {
		_ = port.ColorScheme{
			User:      "not_ansi_color",
			Assistant: "also_invalid",
			System:    "\x1b[96m",
			Error:     "\x1b[91m",
			Tool:      "\x1b[92m",
			Prompt:    "\x1b[94m",
		}
		t.Skip("TODO: Implement CLIAdapter - test color scheme validation")
	})
}

func TestCLIAdapter_IntegrationScenarios(t *testing.T) {
	t.Run("complete conversation flow", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test integration flow")
	})

	t.Run("error handling in conversation", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test error handling")
	})
}

func TestCLIAdapter_EdgeCases(t *testing.T) {
	t.Run("handles rapid successive calls", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test rapid calls")
	})

	t.Run("handles concurrent access safely", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test concurrent access")
	})

	t.Run("handles very large text efficiently", func(t *testing.T) {
		largeText := strings.Repeat("This is a large text block. ", 10000)
		assert.Greater(t, len(largeText), 1000)
		t.Skip("TODO: Implement CLIAdapter - test large text handling")
	})
}

func TestCLIAdapter_ColorSchemeDefaults(t *testing.T) {
	t.Run("uses default color scheme on creation", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter creation - test default colors")
	})

	t.Run("reset to default colors works", func(t *testing.T) {
		t.Skip("TODO: Implement CLIAdapter - test color scheme reset")
	})
}

func TestCLIAdapter_ConfirmBashCommand(t *testing.T) {
	// Red Phase TDD Tests for ConfirmBashCommand
	// These tests define the expected behavior before implementation.

	t.Run("confirms execution when user types lowercase y", func(t *testing.T) {
		input := strings.NewReader("y\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result := adapter.ConfirmBashCommand("echo hello", false, "", "")

		assert.True(t, result, "should return true when user confirms with 'y'")
	})

	t.Run("confirms execution when user types yes", func(t *testing.T) {
		input := strings.NewReader("yes\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result := adapter.ConfirmBashCommand("ls -la", false, "", "")

		assert.True(t, result, "should return true when user confirms with 'yes'")
	})

	t.Run("confirms execution when user types uppercase Y", func(t *testing.T) {
		input := strings.NewReader("Y\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result := adapter.ConfirmBashCommand("pwd", false, "", "")

		assert.True(t, result, "should return true when user confirms with 'Y'")
	})

	t.Run("confirms execution when user types uppercase YES", func(t *testing.T) {
		input := strings.NewReader("YES\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result := adapter.ConfirmBashCommand("cat file.txt", false, "", "")

		assert.True(t, result, "should return true when user confirms with 'YES'")
	})

	t.Run("confirms execution when user types mixed case Yes", func(t *testing.T) {
		input := strings.NewReader("Yes\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result := adapter.ConfirmBashCommand("grep pattern file", false, "", "")

		assert.True(t, result, "should return true when user confirms with 'Yes'")
	})

	t.Run("denies execution when user types n", func(t *testing.T) {
		input := strings.NewReader("n\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result := adapter.ConfirmBashCommand("rm file.txt", true, "destructive rm command", "")

		assert.False(t, result, "should return false when user denies with 'n'")
	})

	t.Run("denies execution when user types no", func(t *testing.T) {
		input := strings.NewReader("no\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result := adapter.ConfirmBashCommand("rm -rf /", true, "destructive rm command", "")

		assert.False(t, result, "should return false when user denies with 'no'")
	})

	t.Run("denies execution on empty input (default deny)", func(t *testing.T) {
		input := strings.NewReader("\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result := adapter.ConfirmBashCommand("echo test", false, "", "")

		assert.False(t, result, "should return false on empty input (default deny behavior)")
	})

	t.Run("denies execution on EOF", func(t *testing.T) {
		input := strings.NewReader("") // Empty reader simulates EOF
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result := adapter.ConfirmBashCommand("echo test", false, "", "")

		assert.False(t, result, "should return false on EOF")
	})

	t.Run("denies execution on unrecognized input", func(t *testing.T) {
		input := strings.NewReader("maybe\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result := adapter.ConfirmBashCommand("echo test", false, "", "")

		assert.False(t, result, "should return false on unrecognized input")
	})

	t.Run("denies execution on whitespace-only input", func(t *testing.T) {
		input := strings.NewReader("   \n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result := adapter.ConfirmBashCommand("echo test", false, "", "")

		assert.False(t, result, "should return false on whitespace-only input")
	})

	t.Run("displays dangerous warning in red for dangerous commands", func(t *testing.T) {
		input := strings.NewReader("n\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		adapter.ConfirmBashCommand("rm -rf /home", true, "destructive rm command", "")

		outputStr := output.String()
		// Check for red color code (\x1b[91m) and dangerous warning text
		assert.Contains(t, outputStr, "\x1b[91m", "should contain red color code for dangerous warning")
		assert.Contains(t, outputStr, "[DANGEROUS COMMAND]", "should display dangerous command warning label")
		assert.Contains(t, outputStr, "destructive rm command", "should display the danger reason")
	})

	t.Run("displays standard prefix in cyan for non-dangerous commands", func(t *testing.T) {
		input := strings.NewReader("n\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		adapter.ConfirmBashCommand("ls -la", false, "", "")

		outputStr := output.String()
		// Check for cyan color code (\x1b[96m) and standard prefix
		assert.Contains(t, outputStr, "\x1b[96m", "should contain cyan color code for standard commands")
		assert.Contains(t, outputStr, "[BASH COMMAND]", "should display bash command label")
	})

	t.Run("displays the command in green color", func(t *testing.T) {
		input := strings.NewReader("n\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		command := "echo 'hello world'"
		adapter.ConfirmBashCommand(command, false, "", "")

		outputStr := output.String()
		// Check for green color code (\x1b[92m) and the command itself
		assert.Contains(t, outputStr, "\x1b[92m", "should contain green color code for command display")
		assert.Contains(t, outputStr, command, "should display the actual command")
	})

	t.Run("displays confirmation prompt", func(t *testing.T) {
		input := strings.NewReader("y\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		adapter.ConfirmBashCommand("echo test", false, "", "")

		outputStr := output.String()
		assert.Contains(
			t,
			outputStr,
			"Execute? [y/N]:",
			"should display confirmation prompt with default deny indicator",
		)
	})

	t.Run("handles multiline command display", func(t *testing.T) {
		input := strings.NewReader("y\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		multilineCmd := "echo line1 && \\\necho line2"
		result := adapter.ConfirmBashCommand(multilineCmd, false, "", "")

		outputStr := output.String()
		assert.True(t, result, "should confirm multiline command")
		assert.Contains(t, outputStr, multilineCmd, "should display full multiline command")
	})

	t.Run("handles command with special characters", func(t *testing.T) {
		input := strings.NewReader("y\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		specialCmd := "echo 'test' | grep -E \"[a-z]+\" && ls $HOME"
		result := adapter.ConfirmBashCommand(specialCmd, false, "", "")

		outputStr := output.String()
		assert.True(t, result, "should confirm command with special characters")
		assert.Contains(t, outputStr, specialCmd, "should display command with special characters")
	})

	t.Run("trims whitespace from user input before checking", func(t *testing.T) {
		input := strings.NewReader("  y  \n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result := adapter.ConfirmBashCommand("echo test", false, "", "")

		assert.True(t, result, "should trim whitespace and accept 'y' with surrounding spaces")
	})

	t.Run("dangerous command with empty reason still shows warning", func(t *testing.T) {
		input := strings.NewReader("n\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		adapter.ConfirmBashCommand("sudo rm -rf /", true, "", "")

		outputStr := output.String()
		assert.Contains(
			t,
			outputStr,
			"[DANGEROUS COMMAND]",
			"should still show dangerous warning even with empty reason",
		)
	})

	t.Run("resets color after output", func(t *testing.T) {
		input := strings.NewReader("y\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		adapter.ConfirmBashCommand("echo test", false, "", "")

		outputStr := output.String()
		// Check that color reset code (\x1b[0m) is present
		assert.Contains(t, outputStr, "\x1b[0m", "should reset color after output")
	})
}

// Red Phase TDD Tests for ConfirmBashCommand description parameter.
// These tests define the expected behavior for displaying command descriptions.
// All tests will fail until the description parameter is added to the method signature
// and the implementation is updated to display the description.
// Red Phase TDD Tests for truncation integration into CLIAdapter.
// These tests define the expected behavior before implementation.
// All tests will fail until the truncation config is added to CLIAdapter
// and DisplayToolResult is modified to apply truncation.

func TestCLIAdapter_DisplayToolResult_TruncatesLargeOutput(t *testing.T) {
	// Test that output with more than 30 lines (head 20 + tail 10) gets truncated
	// This test will fail because DisplayToolResult does not yet apply truncation

	t.Run("truncates output exceeding threshold", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Generate 50 lines of output - should trigger truncation with default config (20 head + 10 tail = 30 threshold)
		var lines []string
		for i := 1; i <= 50; i++ {
			lines = append(lines, fmt.Sprintf("line %d", i))
		}
		largeResult := strings.Join(lines, "\n")

		err := adapter.DisplayToolResult("test_tool", "test_input", largeResult)

		require.NoError(t, err)
		outputStr := output.String()

		// Should contain truncation indicator showing 20 lines were removed (50 - 30 = 20)
		assert.Contains(t, outputStr, "[... 20 lines truncated ...]",
			"should show truncation indicator for 20 removed lines")

		// Should contain first 20 lines
		assert.Contains(t, outputStr, "line 1", "should preserve first line")
		assert.Contains(t, outputStr, "line 20", "should preserve line 20 (last of head)")

		// Should contain last 10 lines
		assert.Contains(t, outputStr, "line 41", "should preserve line 41 (first of tail)")
		assert.Contains(t, outputStr, "line 50", "should preserve last line")

		// Should NOT contain middle lines
		assert.NotContains(t, outputStr, "line 21\n", "should NOT contain line 21 (truncated)")
		assert.NotContains(t, outputStr, "line 30\n", "should NOT contain line 30 (truncated)")
		assert.NotContains(t, outputStr, "line 40\n", "should NOT contain line 40 (truncated)")
	})

	t.Run("truncates output at exactly threshold plus one", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Generate 31 lines - should truncate exactly 1 line
		var lines []string
		for i := 1; i <= 31; i++ {
			lines = append(lines, fmt.Sprintf("line %d", i))
		}
		result := strings.Join(lines, "\n")

		err := adapter.DisplayToolResult("test_tool", "input", result)

		require.NoError(t, err)
		outputStr := output.String()

		// Should show 1 line truncated
		assert.Contains(t, outputStr, "[... 1 lines truncated ...]",
			"should show truncation indicator for 1 removed line")
	})
}

func TestCLIAdapter_DisplayToolResult_PreservesSmallOutput(t *testing.T) {
	// Test that output with fewer lines than the threshold passes through unchanged
	// This test will fail because DisplayToolResult does not yet check truncation config

	t.Run("preserves output under threshold unchanged", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Generate 25 lines - under 30 threshold, should NOT truncate
		var lines []string
		for i := 1; i <= 25; i++ {
			lines = append(lines, fmt.Sprintf("line %d", i))
		}
		smallResult := strings.Join(lines, "\n")

		err := adapter.DisplayToolResult("test_tool", "test_input", smallResult)

		require.NoError(t, err)
		outputStr := output.String()

		// Should NOT contain any truncation indicator
		assert.NotContains(t, outputStr, "truncated",
			"should NOT show truncation indicator for small output")

		// Should contain ALL lines
		for i := 1; i <= 25; i++ {
			assert.Contains(t, outputStr, fmt.Sprintf("line %d", i),
				"should preserve all lines when under threshold")
		}
	})

	t.Run("preserves output at exactly threshold", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Generate exactly 30 lines - at threshold, should NOT truncate
		var lines []string
		for i := 1; i <= 30; i++ {
			lines = append(lines, fmt.Sprintf("line %d", i))
		}
		result := strings.Join(lines, "\n")

		err := adapter.DisplayToolResult("test_tool", "input", result)

		require.NoError(t, err)
		outputStr := output.String()

		// Should NOT contain any truncation indicator
		assert.NotContains(t, outputStr, "truncated",
			"should NOT truncate when exactly at threshold")

		// All lines should be present
		assert.Contains(t, outputStr, "line 30", "should preserve line 30")
	})

	t.Run("preserves empty output", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayToolResult("test_tool", "test_input", "")

		require.NoError(t, err)
		outputStr := output.String()

		// Should NOT contain any truncation indicator
		assert.NotContains(t, outputStr, "truncated",
			"should NOT show truncation for empty output")
	})

	t.Run("preserves single line output", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		singleLine := "just one line here"
		err := adapter.DisplayToolResult("test_tool", "input", singleLine)

		require.NoError(t, err)
		outputStr := output.String()

		assert.Contains(t, outputStr, singleLine, "should preserve single line output")
		assert.NotContains(t, outputStr, "truncated", "should NOT truncate single line")
	})
}

func TestCLIAdapter_DisplayToolResult_BashToolHandling(t *testing.T) {
	// Test that bash tool results use TruncateBashOutput which handles JSON format
	// This test will fail because DisplayToolResult does not yet detect bash tool

	t.Run("uses TruncateBashOutput for bash tool", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Create bash-style JSON with large stdout
		var stdoutLines []string
		for i := 1; i <= 50; i++ {
			stdoutLines = append(stdoutLines, fmt.Sprintf("stdout line %d", i))
		}
		bashResult := fmt.Sprintf(`{"stdout":"%s","stderr":"","exit_code":0}`,
			strings.Join(stdoutLines, "\\n"))

		err := adapter.DisplayToolResult("bash", "echo test", bashResult)

		require.NoError(t, err)
		outputStr := output.String()

		// Should contain truncation indicator within the JSON stdout field
		assert.Contains(t, outputStr, "truncated",
			"should truncate stdout within bash JSON output")

		// Should preserve structure - exit_code should still be present
		assert.Contains(t, outputStr, "exit_code",
			"should preserve bash JSON structure")
	})

	t.Run("truncates stderr in bash output independently", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Create bash-style JSON with large stderr
		var stderrLines []string
		for i := 1; i <= 50; i++ {
			stderrLines = append(stderrLines, fmt.Sprintf("error line %d", i))
		}
		bashResult := fmt.Sprintf(`{"stdout":"ok","stderr":"%s","exit_code":1}`,
			strings.Join(stderrLines, "\\n"))

		err := adapter.DisplayToolResult("bash", "failing cmd", bashResult)

		require.NoError(t, err)
		outputStr := output.String()

		// stderr should be truncated
		assert.Contains(t, outputStr, "truncated",
			"should truncate stderr within bash JSON output")
	})

	t.Run("falls back to regular truncation for invalid bash JSON", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Generate plain text output (not JSON) but use bash tool name
		var lines []string
		for i := 1; i <= 50; i++ {
			lines = append(lines, fmt.Sprintf("line %d", i))
		}
		plainResult := strings.Join(lines, "\n")

		err := adapter.DisplayToolResult("bash", "echo test", plainResult)

		require.NoError(t, err)
		outputStr := output.String()

		// Should still truncate using fallback
		assert.Contains(t, outputStr, "truncated",
			"should fall back to regular truncation for non-JSON bash output")
	})

	t.Run("does not use bash truncation for non-bash tools", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Create JSON that looks like bash output but for a different tool
		var lines []string
		for i := 1; i <= 50; i++ {
			lines = append(lines, fmt.Sprintf("line %d", i))
		}
		result := strings.Join(lines, "\n")

		err := adapter.DisplayToolResult("read_file", "/path/to/file", result)

		require.NoError(t, err)
		outputStr := output.String()

		// Should truncate with regular TruncateOutput, not bash-specific
		assert.Contains(t, outputStr, "truncated",
			"should use regular truncation for non-bash tools")
	})
}

func TestCLIAdapter_SetTruncationConfig(t *testing.T) {
	// Test that truncation config can be set and affects truncation behavior
	// This test will fail because SetTruncationConfig method does not exist

	t.Run("sets custom truncation config", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Set custom config with 5 head lines and 3 tail lines
		customConfig := ui.TruncationConfig{
			HeadLines: 5,
			TailLines: 3,
			Enabled:   true,
		}
		adapter.SetTruncationConfig(customConfig)

		// Generate 20 lines - should truncate with custom config (5 + 3 = 8 threshold)
		var lines []string
		for i := 1; i <= 20; i++ {
			lines = append(lines, fmt.Sprintf("line %d", i))
		}
		result := strings.Join(lines, "\n")

		err := adapter.DisplayToolResult("test_tool", "input", result)

		require.NoError(t, err)
		outputStr := output.String()

		// Should truncate 12 lines (20 - 8 = 12)
		assert.Contains(t, outputStr, "[... 12 lines truncated ...]",
			"should truncate based on custom config")

		// Should contain only first 5 lines
		assert.Contains(t, outputStr, "line 5", "should preserve line 5 (last of custom head)")
		assert.NotContains(t, outputStr, "\nline 6\n", "should NOT contain line 6 with custom config")

		// Should contain only last 3 lines
		assert.Contains(t, outputStr, "line 18", "should preserve line 18 (first of custom tail)")
	})

	t.Run("can disable truncation via config", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Disable truncation
		disabledConfig := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   false,
		}
		adapter.SetTruncationConfig(disabledConfig)

		// Generate 100 lines - would normally truncate, but disabled
		var lines []string
		for i := 1; i <= 100; i++ {
			lines = append(lines, fmt.Sprintf("line %d", i))
		}
		result := strings.Join(lines, "\n")

		err := adapter.DisplayToolResult("test_tool", "input", result)

		require.NoError(t, err)
		outputStr := output.String()

		// Should NOT truncate when disabled
		assert.NotContains(t, outputStr, "truncated",
			"should NOT truncate when config is disabled")

		// All lines should be present
		assert.Contains(t, outputStr, "line 50", "should preserve all lines when disabled")
		assert.Contains(t, outputStr, "line 100", "should preserve last line when disabled")
	})

	t.Run("setting config multiple times uses latest", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Set first config
		adapter.SetTruncationConfig(ui.TruncationConfig{
			HeadLines: 2,
			TailLines: 2,
			Enabled:   true,
		})

		// Override with second config
		adapter.SetTruncationConfig(ui.TruncationConfig{
			HeadLines: 10,
			TailLines: 5,
			Enabled:   true,
		})

		// Generate 20 lines
		var lines []string
		for i := 1; i <= 20; i++ {
			lines = append(lines, fmt.Sprintf("line %d", i))
		}
		result := strings.Join(lines, "\n")

		err := adapter.DisplayToolResult("test_tool", "input", result)

		require.NoError(t, err)
		outputStr := output.String()

		// Should truncate based on second config (10 + 5 = 15 threshold, 5 removed)
		assert.Contains(t, outputStr, "[... 5 lines truncated ...]",
			"should use most recent config")
	})
}

func TestCLIAdapter_GetTruncationConfig(t *testing.T) {
	// Test that truncation config can be retrieved
	// This test will fail because GetTruncationConfig method does not exist

	t.Run("returns current truncation config", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Set custom config
		customConfig := ui.TruncationConfig{
			HeadLines: 15,
			TailLines: 8,
			Enabled:   true,
		}
		adapter.SetTruncationConfig(customConfig)

		// Retrieve config
		retrievedConfig := adapter.GetTruncationConfig()

		assert.Equal(t, customConfig.HeadLines, retrievedConfig.HeadLines,
			"HeadLines should match set value")
		assert.Equal(t, customConfig.TailLines, retrievedConfig.TailLines,
			"TailLines should match set value")
		assert.Equal(t, customConfig.Enabled, retrievedConfig.Enabled,
			"Enabled should match set value")
	})

	t.Run("returns default config when not explicitly set", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Get config without setting it first
		config := adapter.GetTruncationConfig()

		// Should return default values
		defaultConfig := ui.DefaultTruncationConfig()
		assert.Equal(t, defaultConfig.HeadLines, config.HeadLines,
			"HeadLines should have default value")
		assert.Equal(t, defaultConfig.TailLines, config.TailLines,
			"TailLines should have default value")
		assert.Equal(t, defaultConfig.Enabled, config.Enabled,
			"Enabled should have default value")
	})

	t.Run("returns copy not reference to internal state", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Get config
		config1 := adapter.GetTruncationConfig()

		// Modify the returned config
		config1.HeadLines = 999
		config1.Enabled = false

		// Get config again
		config2 := adapter.GetTruncationConfig()

		// Should not be affected by modification of previous return value
		assert.NotEqual(t, 999, config2.HeadLines,
			"modifying returned config should not affect adapter internal state")
		assert.True(t, config2.Enabled,
			"modifying returned config should not affect adapter internal state")
	})
}

func TestCLIAdapter_DefaultTruncationConfig(t *testing.T) {
	// Test that new adapters have default truncation config applied
	// This test will fail because CLIAdapter does not yet initialize truncationConfig

	t.Run("new adapter with default IO has default truncation config", func(t *testing.T) {
		// Note: NewCLIAdapter uses os.Stdin/Stdout, but we can still check config
		adapter := ui.NewCLIAdapter()

		config := adapter.GetTruncationConfig()

		// Should match DefaultTruncationConfig() values
		assert.Equal(t, 20, config.HeadLines,
			"default HeadLines should be 20")
		assert.Equal(t, 10, config.TailLines,
			"default TailLines should be 10")
		assert.True(t, config.Enabled,
			"default Enabled should be true")
	})

	t.Run("new adapter with custom IO has default truncation config", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		config := adapter.GetTruncationConfig()

		// Should also have default config
		assert.Equal(t, 20, config.HeadLines,
			"default HeadLines should be 20 for custom IO adapter")
		assert.Equal(t, 10, config.TailLines,
			"default TailLines should be 10 for custom IO adapter")
		assert.True(t, config.Enabled,
			"default Enabled should be true for custom IO adapter")
	})

	t.Run("default config enables truncation on large output without explicit config", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Do NOT set any config - rely on defaults

		// Generate 50 lines - should truncate with default config
		var lines []string
		for i := 1; i <= 50; i++ {
			lines = append(lines, fmt.Sprintf("line %d", i))
		}
		result := strings.Join(lines, "\n")

		err := adapter.DisplayToolResult("test_tool", "input", result)

		require.NoError(t, err)
		outputStr := output.String()

		// Should truncate using default config (20 + 10 = 30 threshold, 20 removed)
		assert.Contains(t, outputStr, "[... 20 lines truncated ...]",
			"should apply default truncation config to large output")
	})
}

func TestCLIAdapter_ConfirmBashCommand_Description(t *testing.T) {
	t.Run("displays description when provided for non-dangerous command", func(t *testing.T) {
		// This test will fail because:
		// 1. Current signature is ConfirmBashCommand(command, isDangerous, reason string)
		// 2. New signature should be ConfirmBashCommand(command, isDangerous, reason, description string)
		input := strings.NewReader("y\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Call with the new 4-parameter signature
		adapter.ConfirmBashCommand("ls", false, "", "List files in directory")

		outputStr := output.String()

		// The description should appear on a line before the command
		assert.Contains(t, outputStr, "List files in directory",
			"should display the description text in output")

		// Verify the description appears before the command in the output
		descIndex := strings.Index(outputStr, "List files in directory")
		cmdIndex := strings.Index(outputStr, "ls")
		assert.Greater(t, cmdIndex, descIndex,
			"description should appear before the command")
	})

	t.Run("omits description when empty string provided", func(t *testing.T) {
		// This test verifies that when description is empty, no extra line is shown
		input := strings.NewReader("y\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		adapter.ConfirmBashCommand("ls", false, "", "")

		outputStr := output.String()

		// Should show [BASH COMMAND] header
		assert.Contains(t, outputStr, "[BASH COMMAND]",
			"should display bash command label")

		// Count the lines between [BASH COMMAND] and the command itself
		// With no description, the command should immediately follow the header
		lines := strings.Split(outputStr, "\n")
		var headerLineIdx, cmdLineIdx int
		for i, line := range lines {
			if strings.Contains(line, "[BASH COMMAND]") {
				headerLineIdx = i
			}
			if strings.Contains(line, "ls") && !strings.Contains(line, "[BASH COMMAND]") {
				cmdLineIdx = i
				break
			}
		}

		// When description is empty, command should be on the very next line after header
		assert.Equal(t, headerLineIdx+1, cmdLineIdx,
			"command should immediately follow header when description is empty")
	})

	t.Run("displays both danger reason and description for dangerous command", func(t *testing.T) {
		// This test verifies that both the danger reason and description are shown
		input := strings.NewReader("n\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		adapter.ConfirmBashCommand("rm -rf /", true, "destructive rm", "Clean temp files")

		outputStr := output.String()

		// Should show dangerous command warning with reason
		assert.Contains(t, outputStr, "[DANGEROUS COMMAND]",
			"should display dangerous command label")
		assert.Contains(t, outputStr, "destructive rm",
			"should display the danger reason")

		// Should also show the description
		assert.Contains(t, outputStr, "Clean temp files",
			"should display the description text")

		// Verify ordering: danger warning -> description -> command
		dangerIndex := strings.Index(outputStr, "[DANGEROUS COMMAND]")
		descIndex := strings.Index(outputStr, "Clean temp files")
		cmdIndex := strings.Index(outputStr, "rm -rf /")

		assert.Greater(t, descIndex, dangerIndex,
			"description should appear after the danger warning")
		assert.Greater(t, cmdIndex, descIndex,
			"command should appear after the description")
	})

	t.Run("description line uses appropriate styling", func(t *testing.T) {
		// This test verifies the description has proper visual styling
		input := strings.NewReader("y\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		adapter.ConfirmBashCommand("git status", false, "", "Check repository status")

		outputStr := output.String()

		// The description should be visible in the output
		assert.Contains(t, outputStr, "Check repository status",
			"description should be present in output")

		// The output should contain color reset to ensure proper formatting
		assert.Contains(t, outputStr, "\x1b[0m",
			"should contain color reset codes for proper formatting")
	})

	t.Run("handles description with special characters", func(t *testing.T) {
		input := strings.NewReader("y\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		specialDescription := "Build project with flags: -v --output=\"dist/\" && run tests"
		adapter.ConfirmBashCommand("make build", false, "", specialDescription)

		outputStr := output.String()
		assert.Contains(t, outputStr, specialDescription,
			"should display description with special characters correctly")
	})

	t.Run("handles multiline description", func(t *testing.T) {
		input := strings.NewReader("y\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		multilineDescription := "Step 1: Install dependencies\nStep 2: Build project"
		adapter.ConfirmBashCommand("npm install && npm run build", false, "", multilineDescription)

		outputStr := output.String()
		assert.Contains(t, outputStr, "Step 1: Install dependencies",
			"should display first line of multiline description")
		assert.Contains(t, outputStr, "Step 2: Build project",
			"should display second line of multiline description")
	})

	t.Run("dangerous command with description but empty reason", func(t *testing.T) {
		input := strings.NewReader("n\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		adapter.ConfirmBashCommand("sudo apt-get update", true, "", "Update package lists")

		outputStr := output.String()

		// Should still show dangerous warning
		assert.Contains(t, outputStr, "[DANGEROUS COMMAND]",
			"should show dangerous command label even with empty reason")

		// Should show the description
		assert.Contains(t, outputStr, "Update package lists",
			"should display description even when danger reason is empty")
	})

	t.Run("description does not affect confirmation result", func(t *testing.T) {
		// Test that adding description parameter does not change the confirmation logic
		inputYes := strings.NewReader("y\n")
		outputYes := &strings.Builder{}
		adapterYes := ui.NewCLIAdapterWithIO(inputYes, outputYes)

		resultYes := adapterYes.ConfirmBashCommand("echo hello", false, "", "Print greeting")
		assert.True(t, resultYes, "should return true when user confirms with description present")

		inputNo := strings.NewReader("n\n")
		outputNo := &strings.Builder{}
		adapterNo := ui.NewCLIAdapterWithIO(inputNo, outputNo)

		resultNo := adapterNo.ConfirmBashCommand("echo hello", false, "", "Print greeting")
		assert.False(t, resultNo, "should return false when user denies with description present")
	})
}

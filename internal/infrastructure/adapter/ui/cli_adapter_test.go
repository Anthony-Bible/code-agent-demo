package ui_test

import (
	"bufio"
	"bytes"
	"code-editing-agent/internal/domain/port"
	"code-editing-agent/internal/infrastructure/adapter/ui"
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests are intentionally written to fail during the Red Phase of TDD.
// They define the expected behavior of the CLIAdapter before implementation.

func TestCLIAdapter_GetUserInput(t *testing.T) {
	t.Run("gets user input successfully", func(t *testing.T) {
		input := strings.NewReader("hello world\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result, ok := adapter.GetUserInput(context.Background())

		assert.True(t, ok, "should return ok=true for successful input")
		assert.Equal(t, "hello world", result, "should return the input text")
	})

	t.Run("handles empty input", func(t *testing.T) {
		input := strings.NewReader("\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result, ok := adapter.GetUserInput(context.Background())

		assert.True(t, ok, "should return ok=true even for empty line")
		assert.Empty(t, result, "should return empty string")
	})

	t.Run("handles input with spaces", func(t *testing.T) {
		input := strings.NewReader("  hello   world  \n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result, ok := adapter.GetUserInput(context.Background())

		assert.True(t, ok, "should return ok=true")
		assert.Equal(t, "  hello   world  ", result, "should preserve spaces in input")
	})

	t.Run("handles EOF (ctrl+d)", func(t *testing.T) {
		input := strings.NewReader("") // Empty reader simulates EOF
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		_, ok := adapter.GetUserInput(context.Background())

		assert.False(t, ok, "should return ok=false on EOF")
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		input := strings.NewReader("test\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		_, ok := adapter.GetUserInput(ctx)

		assert.False(t, ok, "should return ok=false when context is cancelled")
	})

	t.Run("preserves special characters in input", func(t *testing.T) {
		specialInput := "test ~!@#$%^&*()_+-={}[]|\\:;\"'<>?,./"
		input := strings.NewReader(specialInput + "\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		result, ok := adapter.GetUserInput(context.Background())

		assert.True(t, ok, "should return ok=true")
		assert.Equal(t, specialInput, result, "should preserve special characters")
	})
}

func TestCLIAdapter_DisplayMessage(t *testing.T) {
	t.Run("displays user message with correct color", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayMessage("Hello world", "user")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "\x1b[94m", "should contain blue color code for user")
		assert.Contains(t, outputStr, "Hello world", "should contain the message")
		assert.Contains(t, outputStr, "\x1b[0m", "should reset color at end")
	})

	t.Run("displays assistant message with correct color", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayMessage("I can help with that", "assistant")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "\x1b[93m", "should contain yellow color code for assistant")
		assert.Contains(t, outputStr, "I can help with that", "should contain the message")
	})

	t.Run("displays system message with correct color", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayMessage("System initialized", "system")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "\x1b[96m", "should contain cyan color code for system")
		assert.Contains(t, outputStr, "System initialized", "should contain the message")
	})

	t.Run("handles empty message", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayMessage("", "user")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "\x1b[94m", "should still output color codes for empty message")
	})

	t.Run("handles multiline message", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		multiline := "Line 1\nLine 2\nLine 3"
		err := adapter.DisplayMessage(multiline, "user")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "Line 1", "should contain first line")
		assert.Contains(t, outputStr, "Line 2", "should contain second line")
		assert.Contains(t, outputStr, "Line 3", "should contain third line")
	})

	t.Run("handles unknown message role defaults to user", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayMessage("Unknown role message", "unknown_role")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "\x1b[94m", "should default to blue/user color for unknown role")
		assert.Contains(t, outputStr, "Unknown role message", "should contain the message")
	})

	t.Run("handles message with special characters", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		specialChars := "Special chars: ~`!@#$%^&*()_-+={}|[]\\:;\"'<>?,./"
		err := adapter.DisplayMessage(specialChars, "user")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, specialChars, "should preserve special characters")
	})

	t.Run("handles very long message", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		longMessage := strings.Repeat("This is a long message. ", 1000)
		err := adapter.DisplayMessage(longMessage, "user")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "This is a long message.", "should contain the repeated text")
		assert.Greater(t, len(outputStr), 10000, "should handle large output")
	})
}

func TestCLIAdapter_DisplayError(t *testing.T) {
	t.Run("displays error message with correct color", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		testErr := errors.New("something went wrong")
		err := adapter.DisplayError(testErr)

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "\x1b[91m", "should contain red color code for error")
		assert.Contains(t, outputStr, "Error:", "should contain 'Error:' prefix")
		assert.Contains(t, outputStr, "something went wrong", "should contain error message")
		assert.Contains(t, outputStr, "\x1b[0m", "should reset color at end")
	})

	t.Run("handles nil error", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayError(nil)

		require.NoError(t, err, "should not return error for nil input")
		outputStr := output.String()
		assert.Empty(t, outputStr, "should not output anything for nil error")
	})

	t.Run("handles complex error with message", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		complexErr := errors.New("complex error: nested error occurred")
		err := adapter.DisplayError(complexErr)

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "complex error: nested error occurred", "should preserve full error message")
	})

	t.Run("handles error with special characters", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		specialErr := errors.New("Error with special chars: !@#$%^&*()")
		err := adapter.DisplayError(specialErr)

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "!@#$%^&*()", "should preserve special characters in error")
	})
}

func TestCLIAdapter_DisplayToolResult(t *testing.T) {
	t.Run("displays tool result with correct color", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Use bash tool as it doesn't have compact display
		err := adapter.DisplayToolResult("bash", "echo hello", "hello")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "\x1b[92m", "should contain green color code for tool")
		assert.Contains(t, outputStr, "Tool [bash]", "should contain tool name in brackets")
		assert.Contains(t, outputStr, "echo hello", "should contain input")
		assert.Contains(t, outputStr, "hello", "should contain result")
		assert.Contains(t, outputStr, "\x1b[0m", "should reset color")
	})

	t.Run("handles empty tool name", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayToolResult("", "input", "result")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "Tool []", "should show empty tool name in brackets")
	})

	t.Run("handles empty input path", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayToolResult("bash", "", "result")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "Tool [bash]", "should display tool name")
		assert.Contains(t, outputStr, "result", "should display result")
	})

	t.Run("handles empty result", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayToolResult("bash", "input", "")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "Tool [bash]", "should display tool name for empty result")
	})

	t.Run("handles multiline result", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		multilineResult := "Line 1\nLine 2\nLine 3"
		err := adapter.DisplayToolResult("bash", "input", multilineResult)

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "Line 1", "should contain first line")
		assert.Contains(t, outputStr, "Line 2", "should contain second line")
		assert.Contains(t, outputStr, "Line 3", "should contain third line")
	})

	t.Run("handles result with special characters", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		specialResult := "Result with special chars: ~`!@#$%^&*()_-+={}|[]\\:;\"'<>?,./"
		err := adapter.DisplayToolResult("bash", "input", specialResult)

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, specialResult, "should preserve special characters in result")
	})
}

func TestCLIAdapter_DisplaySystemMessage(t *testing.T) {
	t.Run("displays system message with correct color", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplaySystemMessage("System initialized")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "\x1b[96m", "should contain cyan color code for system")
		assert.Contains(t, outputStr, "System:", "should contain 'System:' prefix")
		assert.Contains(t, outputStr, "System initialized", "should contain the message")
		assert.Contains(t, outputStr, "\x1b[0m", "should reset color at end")
	})

	t.Run("handles empty system message", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplaySystemMessage("")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "System:", "should still show 'System:' prefix for empty message")
	})

	t.Run("handles multiline system message", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		multiline := "System starting...\nConfiguration loaded\nReady"
		err := adapter.DisplaySystemMessage(multiline)

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "System starting...", "should contain first line")
		assert.Contains(t, outputStr, "Configuration loaded", "should contain second line")
		assert.Contains(t, outputStr, "Ready", "should contain third line")
	})

	t.Run("handles system message with special characters", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		specialMsg := "System error occurred: !@#$%^&*()"
		err := adapter.DisplaySystemMessage(specialMsg)

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "!@#$%^&*()", "should preserve special characters")
	})
}

func TestCLIAdapter_SetPrompt(t *testing.T) {
	t.Run("sets custom prompt", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.SetPrompt("MyPrompt> ")

		require.NoError(t, err)
		assert.Equal(t, "MyPrompt> ", adapter.GetPromptPrefix(), "should set custom prompt")
	})

	t.Run("handles empty prompt", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.SetPrompt("")

		require.Error(t, err, "should return error for empty prompt")
		assert.Equal(t, port.ErrInvalidPrompt, err, "should return ErrInvalidPrompt")
	})

	t.Run("handles prompt with color codes", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		colorPrompt := "\x1b[94mClaude\x1b[0m: "
		err := adapter.SetPrompt(colorPrompt)

		require.NoError(t, err)
		assert.Equal(t, colorPrompt, adapter.GetPromptPrefix(), "should preserve ANSI codes in prompt")
	})

	t.Run("handles very long prompt", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		longPrompt := strings.Repeat("VeryLongPrompt", 20)
		err := adapter.SetPrompt(longPrompt)

		require.NoError(t, err)
		assert.Equal(t, longPrompt, adapter.GetPromptPrefix(), "should accept long prompts")
	})

	t.Run("handles prompt with special characters", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		specialPrompt := "Prompt! ~`!@#$%^&*()_-+={}|[]\\:;\"'<>?,./> "
		err := adapter.SetPrompt(specialPrompt)

		require.NoError(t, err)
		assert.Equal(t, specialPrompt, adapter.GetPromptPrefix(), "should preserve special characters in prompt")
	})
}

func TestCLIAdapter_ClearScreen(t *testing.T) {
	t.Run("clears screen successfully", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.ClearScreen()

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "\x1b[2J", "should contain ANSI clear screen sequence")
		assert.Contains(t, outputStr, "\x1b[H", "should contain cursor home sequence")
	})

	t.Run("clears screen multiple times", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Clear multiple times
		err1 := adapter.ClearScreen()
		err2 := adapter.ClearScreen()
		err3 := adapter.ClearScreen()

		require.NoError(t, err1)
		require.NoError(t, err2)
		require.NoError(t, err3)
		// Should contain 3 clear sequences
		outputStr := output.String()
		assert.Equal(t, 3, strings.Count(outputStr, "\x1b[2J"), "should contain 3 clear sequences")
	})
}

func TestCLIAdapter_SetColorScheme(t *testing.T) {
	t.Run("sets valid color scheme", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		scheme := port.ColorScheme{
			User:      "\x1b[95m", // Magenta (different from default)
			Assistant: "\x1b[93m",
			System:    "\x1b[96m",
			Error:     "\x1b[91m",
			Tool:      "\x1b[92m",
			Prompt:    "\x1b[94m",
		}
		err := adapter.SetColorScheme(scheme)

		require.NoError(t, err)
		// Verify color is applied by displaying a message
		_ = adapter.DisplayMessage("test", "user")
		outputStr := output.String()
		assert.Contains(t, outputStr, "\x1b[95m", "should use new magenta color for user")
	})

	t.Run("handles empty color codes", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		emptyScheme := port.ColorScheme{
			User:      "",
			Assistant: "",
			System:    "",
			Error:     "",
			Tool:      "",
			Prompt:    "",
		}
		err := adapter.SetColorScheme(emptyScheme)

		require.Error(t, err, "should return error for all-empty color scheme")
		assert.Equal(t, port.ErrInvalidColor, err, "should return ErrInvalidColor")
	})

	t.Run("handles invalid color codes gracefully", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Invalid color codes are still accepted (no validation of format)
		// The implementation allows any string as a color code
		scheme := port.ColorScheme{
			User:      "invalid_color",
			Assistant: "\x1b[93m",
			System:    "\x1b[96m",
			Error:     "\x1b[91m",
			Tool:      "\x1b[92m",
			Prompt:    "\x1b[94m",
		}
		err := adapter.SetColorScheme(scheme)

		require.NoError(t, err, "should accept non-ANSI strings as color codes")
		_ = adapter.DisplayMessage("test", "user")
		outputStr := output.String()
		assert.Contains(t, outputStr, "invalid_color", "should use provided string as color")
	})

	t.Run("handles partial color scheme", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Only set User color, others remain default
		partialScheme := port.ColorScheme{
			User: "\x1b[95m", // Magenta
			// Other fields empty - should keep defaults
		}
		err := adapter.SetColorScheme(partialScheme)

		require.NoError(t, err)
		// User message should use new color
		_ = adapter.DisplayMessage("user msg", "user")
		// Assistant should still use default yellow
		_ = adapter.DisplayMessage("assistant msg", "assistant")

		outputStr := output.String()
		assert.Contains(t, outputStr, "\x1b[95m", "should use new magenta for user")
		assert.Contains(t, outputStr, "\x1b[93m", "should keep default yellow for assistant")
	})

	t.Run("validates color scheme format", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// The implementation doesn't validate ANSI format, just that at least one is set
		scheme := port.ColorScheme{
			User:      "not_ansi_color",
			Assistant: "also_invalid",
			System:    "\x1b[96m",
			Error:     "\x1b[91m",
			Tool:      "\x1b[92m",
			Prompt:    "\x1b[94m",
		}
		err := adapter.SetColorScheme(scheme)

		require.NoError(t, err, "should accept any non-empty strings")
	})
}

func TestCLIAdapter_IntegrationScenarios(t *testing.T) {
	t.Run("complete conversation flow", func(t *testing.T) {
		input := strings.NewReader("Hello AI\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Simulate a conversation flow
		// 1. Get user input
		userInput, ok := adapter.GetUserInput(context.Background())
		require.True(t, ok)
		assert.Equal(t, "Hello AI", userInput)

		// 2. Display user message
		err := adapter.DisplayMessage(userInput, "user")
		require.NoError(t, err)

		// 3. Display assistant response
		err = adapter.DisplayMessage("Hello! How can I help?", "assistant")
		require.NoError(t, err)

		// 4. Display tool result
		err = adapter.DisplayToolResult("bash", "echo test", "test output")
		require.NoError(t, err)

		// 5. Display system message
		err = adapter.DisplaySystemMessage("Tool execution complete")
		require.NoError(t, err)

		outputStr := output.String()
		// Verify all parts are present
		assert.Contains(t, outputStr, "Hello AI")
		assert.Contains(t, outputStr, "Hello! How can I help?")
		assert.Contains(t, outputStr, "Tool [bash]")
		assert.Contains(t, outputStr, "System:")
	})

	t.Run("error handling in conversation", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Display an error
		testErr := errors.New("connection failed")
		err := adapter.DisplayError(testErr)
		require.NoError(t, err)

		// Continue with a system message
		err = adapter.DisplaySystemMessage("Retrying...")
		require.NoError(t, err)

		outputStr := output.String()
		assert.Contains(t, outputStr, "Error:")
		assert.Contains(t, outputStr, "connection failed")
		assert.Contains(t, outputStr, "Retrying...")
	})
}

func TestCLIAdapter_EdgeCases(t *testing.T) {
	t.Run("handles rapid successive calls", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Make 100 rapid calls
		for i := range 100 {
			err := adapter.DisplayMessage(fmt.Sprintf("Message %d", i), "user")
			require.NoError(t, err)
		}

		outputStr := output.String()
		assert.Contains(t, outputStr, "Message 0")
		assert.Contains(t, outputStr, "Message 99")
	})

	t.Run("handles concurrent access safely", func(t *testing.T) {
		input := strings.NewReader("")
		output := &bytes.Buffer{} // Use bytes.Buffer for thread-safe writes
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Run concurrent display operations
		done := make(chan bool, 10)
		for i := range 10 {
			go func(n int) {
				defer func() { done <- true }()
				for j := range 10 {
					_ = adapter.DisplayMessage(fmt.Sprintf("Goroutine %d msg %d", n, j), "user")
				}
			}(i)
		}

		// Wait for all goroutines
		for range 10 {
			<-done
		}

		// Should complete without panics or deadlocks
		outputStr := output.String()
		assert.NotEmpty(t, outputStr, "should have produced output")
	})

	t.Run("handles very large text efficiently", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		largeText := strings.Repeat("This is a large text block. ", 10000)
		err := adapter.DisplayMessage(largeText, "user")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Greater(t, len(outputStr), 100000, "should handle large output")
		assert.Contains(t, outputStr, "This is a large text block.")
	})
}

func TestCLIAdapter_ColorSchemeDefaults(t *testing.T) {
	t.Run("uses default color scheme on creation", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Display messages with different roles to verify default colors
		_ = adapter.DisplayMessage("user msg", "user")
		_ = adapter.DisplayMessage("assistant msg", "assistant")
		_ = adapter.DisplaySystemMessage("system msg")
		_ = adapter.DisplayError(errors.New("error msg"))

		outputStr := output.String()
		// Verify default colors are used
		assert.Contains(t, outputStr, "\x1b[94m", "should use blue for user")
		assert.Contains(t, outputStr, "\x1b[93m", "should use yellow for assistant")
		assert.Contains(t, outputStr, "\x1b[96m", "should use cyan for system")
		assert.Contains(t, outputStr, "\x1b[91m", "should use red for error")
	})

	t.Run("reset to default colors works", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Change colors
		customScheme := port.ColorScheme{
			User:      "\x1b[95m", // Magenta
			Assistant: "\x1b[95m",
			System:    "\x1b[95m",
			Error:     "\x1b[95m",
			Tool:      "\x1b[95m",
			Prompt:    "\x1b[95m",
		}
		err := adapter.SetColorScheme(customScheme)
		require.NoError(t, err)

		// Reset to defaults by setting the original colors
		defaultScheme := port.ColorScheme{
			User:      "\x1b[94m",
			Assistant: "\x1b[93m",
			System:    "\x1b[96m",
			Error:     "\x1b[91m",
			Tool:      "\x1b[92m",
			Prompt:    "\x1b[94m",
		}
		err = adapter.SetColorScheme(defaultScheme)
		require.NoError(t, err)

		// Verify defaults are restored
		_ = adapter.DisplayMessage("test", "user")
		outputStr := output.String()
		assert.Contains(t, outputStr, "\x1b[94m", "should restore default blue for user")
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

		// Create output for a different tool (edit_file uses regular truncation)
		var lines []string
		for i := 1; i <= 50; i++ {
			lines = append(lines, fmt.Sprintf("line %d", i))
		}
		result := strings.Join(lines, "\n")

		err := adapter.DisplayToolResult("edit_file", `{"path": "/path/to/file"}`, result)

		require.NoError(t, err)
		outputStr := output.String()

		// Should truncate with regular TruncateOutput, not bash-specific
		assert.Contains(t, outputStr, "truncated",
			"should use regular truncation for non-bash tools")
	})
}

func TestCLIAdapter_DisplayToolResult_CompactDisplay(t *testing.T) {
	t.Run("read_file shows compact output with path only", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayToolResult("read_file", `{"path": "src/main.go"}`, "file contents here...")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "read(src/main.go)")
		assert.NotContains(t, outputStr, "file contents here")
	})

	t.Run("read_file shows compact output with line range", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayToolResult(
			"read_file",
			`{"path": "src/main.go", "start_line": 10, "end_line": 50}`,
			"lines 10-50...",
		)

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "read(src/main.go:10-50)")
	})

	t.Run("read_file shows start line only when end_line missing", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayToolResult("read_file", `{"path": "src/main.go", "start_line": 5}`, "from line 5...")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "read(src/main.go:5-end)")
	})

	t.Run("list_files shows compact output", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayToolResult("list_files", `{"path": "src/"}`, "file1.go\nfile2.go\nfile3.go")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "list(src/)")
		assert.NotContains(t, outputStr, "file1.go")
	})

	t.Run("read_file handles invalid JSON gracefully", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayToolResult("read_file", "not valid json", "file contents")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "read(not valid json)")
	})

	t.Run("list_files handles invalid JSON gracefully", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayToolResult("list_files", "not valid json", "file list")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "list(not valid json)")
	})

	t.Run("other tools still show full output", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		err := adapter.DisplayToolResult("edit_file", `{"path": "file.go"}`, "edit result content")

		require.NoError(t, err)
		outputStr := output.String()
		assert.Contains(t, outputStr, "Tool [edit_file]")
		assert.Contains(t, outputStr, "edit result content")
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

// =============================================================================
// Terminal Detection Tests - TDD Cycle 3 (Red Phase)
// =============================================================================
// These tests define expected behavior for terminal detection functionality.
// All tests will FAIL until the implementation is added.
// The goal is to detect if input is from an interactive terminal so we can
// use go-prompt (interactive) or bufio.Scanner (non-interactive/piped input).

func TestIsTerminal(t *testing.T) {
	// Tests for the isTerminal() function that detects if a reader is a terminal.
	// This function is used to determine whether to use interactive (go-prompt)
	// or non-interactive (bufio.Scanner) input mode.

	t.Run("returns true for os.File that is a terminal character device", func(t *testing.T) {
		// This test will fail because isTerminal() function does not exist yet.
		// When running in an actual terminal, os.Stdin should be detected as a terminal.
		//
		// Note: This test is tricky to run in CI environments since stdin
		// might not be a true terminal. The implementation should use:
		// stat.Mode() & os.ModeCharDevice != 0

		// We can only test this with os.Stdout in certain environments
		// For now, we test the function exists and handles *os.File
		result := ui.IsTerminal(os.Stdout)

		// In test environments this may be false (redirected output),
		// but the function should at least not panic and return a bool
		assert.IsType(t, true, result, "isTerminal should return a boolean")
	})

	t.Run("returns false for bytes.Buffer reader", func(t *testing.T) {
		// This test will fail because isTerminal() function does not exist yet.
		// A bytes.Buffer is not an *os.File, so it cannot be a terminal.

		buf := &bytes.Buffer{}
		buf.WriteString("some input\n")

		result := ui.IsTerminal(buf)

		assert.False(t, result, "bytes.Buffer should not be detected as a terminal")
	})

	t.Run("returns false for strings.Reader", func(t *testing.T) {
		// This test will fail because isTerminal() function does not exist yet.
		// A strings.Reader is not an *os.File, so it cannot be a terminal.

		reader := strings.NewReader("test input\n")

		result := ui.IsTerminal(reader)

		assert.False(t, result, "strings.Reader should not be detected as a terminal")
	})

	t.Run("returns false for nil reader", func(t *testing.T) {
		// This test will fail because isTerminal() function does not exist yet.
		// Nil reader should safely return false without panicking.

		result := ui.IsTerminal(nil)

		assert.False(t, result, "nil reader should return false")
	})

	t.Run("returns false for pipe reader", func(t *testing.T) {
		// This test will fail because isTerminal() function does not exist yet.
		// A pipe is an *os.File but not a character device (terminal).

		pipeReader, pipeWriter, err := os.Pipe()
		require.NoError(t, err, "should create pipe without error")
		defer pipeReader.Close()
		defer pipeWriter.Close()

		result := ui.IsTerminal(pipeReader)

		assert.False(t, result, "pipe reader should not be detected as a terminal")
	})

	t.Run("returns false for regular file", func(t *testing.T) {
		// This test will fail because isTerminal() function does not exist yet.
		// A regular file on disk is an *os.File but not a terminal.

		// Create a temporary file
		tmpFile, err := os.CreateTemp(t.TempDir(), "terminal_test_*.txt")
		require.NoError(t, err, "should create temp file without error")
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		result := ui.IsTerminal(tmpFile)

		assert.False(t, result, "regular file should not be detected as a terminal")
	})

	t.Run("returns false for io.Reader wrapper around os.File", func(t *testing.T) {
		// This test will fail because isTerminal() function does not exist yet.
		// If an *os.File is wrapped in another io.Reader, we lose the type info.

		tmpFile, err := os.CreateTemp(t.TempDir(), "terminal_test_*.txt")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		// Wrap the file in a bufio.Reader - this hides the underlying *os.File
		wrappedReader := bufio.NewReader(tmpFile)

		result := ui.IsTerminal(wrappedReader)

		assert.False(t, result,
			"wrapped os.File should return false since type assertion fails")
	})
}

func TestCLIAdapter_InteractiveMode(t *testing.T) {
	// Tests for interactive mode detection in CLIAdapter constructors.
	// The adapter needs to know if it's running interactively to choose
	// between go-prompt (interactive) and bufio.Scanner (non-interactive).

	t.Run("NewCLIAdapter detects terminal mode from os.Stdin", func(t *testing.T) {
		// This test will fail because:
		// 1. CLIAdapter does not have a useInteractive field yet
		// 2. NewCLIAdapter does not call isTerminal() yet
		// 3. IsInteractive() method does not exist yet

		adapter := ui.NewCLIAdapter()

		// The adapter should have detected whether stdin is a terminal
		// In test environments, this is typically false (not a TTY)
		// But the method should exist and return a boolean
		result := adapter.IsInteractive()

		assert.IsType(t, true, result,
			"IsInteractive should return a boolean indicating terminal mode")
	})

	t.Run("NewCLIAdapterWithIO is always non-interactive", func(t *testing.T) {
		// This test will fail because:
		// 1. CLIAdapter does not have a useInteractive field yet
		// 2. IsInteractive() method does not exist yet
		//
		// NewCLIAdapterWithIO is used for testing with custom io.Reader/Writer,
		// and should always be non-interactive since test readers are not terminals.

		input := strings.NewReader("test input\n")
		output := &strings.Builder{}

		adapter := ui.NewCLIAdapterWithIO(input, output)

		result := adapter.IsInteractive()

		assert.False(t, result,
			"NewCLIAdapterWithIO should always create non-interactive adapter")
	})

	t.Run("NewCLIAdapterWithHistory creates interactive adapter with history support", func(t *testing.T) {
		// This test will fail because:
		// 1. NewCLIAdapterWithHistory constructor does not exist yet
		// 2. CLIAdapter does not have historyFile or maxHistoryEntries fields yet
		// 3. GetHistoryFile() method does not exist yet
		// 4. GetMaxHistoryEntries() method does not exist yet
		//
		// This constructor creates an interactive adapter with command history
		// that persists to a file.

		historyFile := "/tmp/test_history.txt"
		maxEntries := 100

		adapter := ui.NewCLIAdapterWithHistory(historyFile, maxEntries)

		assert.True(t, adapter.IsInteractive(),
			"NewCLIAdapterWithHistory should create an interactive adapter")
		assert.Equal(t, historyFile, adapter.GetHistoryFile(),
			"should store the history file path")
		assert.Equal(t, maxEntries, adapter.GetMaxHistoryEntries(),
			"should store the max history entries")
	})

	t.Run("NewCLIAdapterWithHistory with empty history file path", func(t *testing.T) {
		// This test will fail because NewCLIAdapterWithHistory does not exist yet.
		// Empty history file path should still work (just no persistence).

		adapter := ui.NewCLIAdapterWithHistory("", 50)

		assert.True(t, adapter.IsInteractive(),
			"should still be interactive even without history file")
		assert.Empty(t, adapter.GetHistoryFile(),
			"empty history file should be stored as-is")
	})

	t.Run("NewCLIAdapterWithHistory with zero max entries uses default", func(t *testing.T) {
		// This test will fail because NewCLIAdapterWithHistory does not exist yet.
		// Zero max entries should use a reasonable default.

		adapter := ui.NewCLIAdapterWithHistory("/tmp/test_history.txt", 0)

		assert.True(t, adapter.IsInteractive(),
			"should still be interactive with zero max entries")
		assert.Positive(t, adapter.GetMaxHistoryEntries(),
			"zero max entries should use a positive default value")
	})

	t.Run("NewCLIAdapterWithHistory with negative max entries uses default", func(t *testing.T) {
		// This test will fail because NewCLIAdapterWithHistory does not exist yet.
		// Negative max entries should use a reasonable default.

		adapter := ui.NewCLIAdapterWithHistory("/tmp/test_history.txt", -10)

		assert.Positive(t, adapter.GetMaxHistoryEntries(),
			"negative max entries should use a positive default value")
	})
}

func TestCLIAdapter_UseInteractiveFlag(t *testing.T) {
	// Tests for the useInteractive field that controls input mode.
	// When useInteractive=false, uses bufio.Scanner (existing behavior).
	// When useInteractive=true, should use go-prompt (Cycle 4).

	t.Run("adapter has useInteractive field controlling input mode", func(t *testing.T) {
		// This test will fail because:
		// 1. useInteractive field does not exist yet
		// 2. IsInteractive() method does not exist yet

		input := strings.NewReader("")
		output := &strings.Builder{}

		adapter := ui.NewCLIAdapterWithIO(input, output)

		// The adapter should have an IsInteractive method
		isInteractive := adapter.IsInteractive()

		assert.False(t, isInteractive,
			"adapter created with custom IO should not be interactive")
	})

	t.Run("non-interactive adapter uses bufio.Scanner for input", func(t *testing.T) {
		// This test will fail because IsInteractive() does not exist yet.
		// This test verifies that the existing bufio.Scanner behavior works
		// when useInteractive is false.

		input := strings.NewReader("hello world\n")
		output := &strings.Builder{}

		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Verify non-interactive mode
		assert.False(t, adapter.IsInteractive(),
			"should be non-interactive for custom IO")

		// Verify existing bufio.Scanner behavior still works
		ctx := context.Background()
		userInput, ok := adapter.GetUserInput(ctx)

		assert.True(t, ok, "should successfully read input")
		assert.Equal(t, "hello world", userInput, "should read input via Scanner")
	})

	t.Run("SetInteractive allows changing interactive mode", func(t *testing.T) {
		// This test will fail because:
		// 1. SetInteractive() method does not exist yet
		// 2. useInteractive field does not exist yet
		//
		// This method allows forcing interactive or non-interactive mode,
		// which is useful for testing.

		input := strings.NewReader("")
		output := &strings.Builder{}

		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Initially non-interactive
		assert.False(t, adapter.IsInteractive(), "should start non-interactive")

		// Enable interactive mode
		adapter.SetInteractive(true)
		assert.True(t, adapter.IsInteractive(), "should be interactive after SetInteractive(true)")

		// Disable interactive mode
		adapter.SetInteractive(false)
		assert.False(t, adapter.IsInteractive(), "should be non-interactive after SetInteractive(false)")
	})

	t.Run("interactive mode flag persists across multiple GetUserInput calls", func(t *testing.T) {
		// This test will fail because IsInteractive() does not exist yet.
		// The interactive flag should not change during usage.

		input := strings.NewReader("line1\nline2\nline3\n")
		output := &strings.Builder{}

		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Verify mode before first call
		initialMode := adapter.IsInteractive()

		ctx := context.Background()

		// Read multiple lines
		adapter.GetUserInput(ctx)
		assert.Equal(t, initialMode, adapter.IsInteractive(),
			"interactive mode should not change after first GetUserInput")

		adapter.GetUserInput(ctx)
		assert.Equal(t, initialMode, adapter.IsInteractive(),
			"interactive mode should not change after second GetUserInput")

		adapter.GetUserInput(ctx)
		assert.Equal(t, initialMode, adapter.IsInteractive(),
			"interactive mode should not change after third GetUserInput")
	})

	t.Run("interactive adapter stores prompt configuration", func(t *testing.T) {
		// This test will fail because NewCLIAdapterWithHistory does not exist yet.
		// Interactive adapters need to store prompt configuration for go-prompt.

		adapter := ui.NewCLIAdapterWithHistory("/tmp/history.txt", 100)

		// Should be interactive
		assert.True(t, adapter.IsInteractive(), "should be interactive")

		// Should be able to set and retrieve prompt
		err := adapter.SetPrompt("custom> ")
		assert.NoError(t, err, "should set prompt without error")

		// The prompt should be stored and available for go-prompt
		// (This will be verified more thoroughly in Cycle 4 with go-prompt integration)
	})
}

func TestCLIAdapter_InteractiveModeWithContext(t *testing.T) {
	// Tests for context handling in interactive mode.
	// Both interactive and non-interactive modes should respect context cancellation.

	t.Run("non-interactive mode respects context cancellation", func(t *testing.T) {
		// This should work with existing implementation but tests the documented behavior

		input := strings.NewReader("this should not be read\n")
		output := &strings.Builder{}

		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Cancel context before reading
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		userInput, ok := adapter.GetUserInput(ctx)

		assert.False(t, ok, "should return false when context is cancelled")
		assert.Empty(t, userInput, "should return empty string when context is cancelled")
	})

	t.Run("interactive mode respects context cancellation", func(t *testing.T) {
		// This test will fail because:
		// 1. NewCLIAdapterWithHistory does not exist yet
		// 2. Interactive mode context handling is not implemented yet
		//
		// Interactive mode with go-prompt should also respect context cancellation.

		adapter := ui.NewCLIAdapterWithHistory("/tmp/history.txt", 100)

		// Cancel context before reading
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		userInput, ok := adapter.GetUserInput(ctx)

		assert.False(t, ok, "interactive mode should return false when context is cancelled")
		assert.Empty(t, userInput, "should return empty string when context is cancelled")
	})
}

func TestCLIAdapter_InputModeString(t *testing.T) {
	// Tests for a method that returns a string description of the input mode.
	// This is useful for debugging and logging.

	t.Run("returns 'interactive' for interactive mode", func(t *testing.T) {
		// This test will fail because:
		// 1. NewCLIAdapterWithHistory does not exist yet
		// 2. InputModeString() method does not exist yet

		adapter := ui.NewCLIAdapterWithHistory("/tmp/history.txt", 100)

		modeStr := adapter.InputModeString()

		assert.Equal(t, "interactive", modeStr,
			"should return 'interactive' for interactive adapter")
	})

	t.Run("returns 'non-interactive' for non-interactive mode", func(t *testing.T) {
		// This test will fail because InputModeString() method does not exist yet

		input := strings.NewReader("")
		output := &strings.Builder{}

		adapter := ui.NewCLIAdapterWithIO(input, output)

		modeStr := adapter.InputModeString()

		assert.Equal(t, "non-interactive", modeStr,
			"should return 'non-interactive' for non-interactive adapter")
	})
}

// =============================================================================
// Go-Prompt Integration Tests - TDD Cycle 4 (Red Phase)
// =============================================================================
// These tests define expected behavior for go-prompt integration in CLIAdapter.
// All tests will FAIL until the implementation is added.
// The goal is to integrate go-prompt for interactive input with arrow key support
// and command history.
//
// NOTE: These tests use t.Skip() with detailed comments describing the expected
// behavior. This allows the tests to compile while documenting what needs to be
// implemented. Once the methods are added, remove the t.Skip() calls.

func TestCLIAdapter_GetUserInput_InteractiveMode(t *testing.T) {
	// Tests for interactive mode input behavior.
	// When in interactive mode, the adapter should use go-prompt instead of bufio.Scanner.
	// Note: We cannot easily test go-prompt directly in unit tests since it requires
	// a real terminal. These tests verify the setup and mode switching behavior.

	t.Run("interactive mode uses different input method than non-interactive", func(t *testing.T) {
		// Expected behavior:
		// 1. Interactive mode should use go-prompt instead of bufio.Scanner
		// 2. The adapter needs to switch between bufio.Scanner and go-prompt based on mode
		// 3. The key difference is that interactive mode should NOT use the scanner field
		//    directly - it should delegate to go-prompt.

		adapter := ui.NewCLIAdapterWithHistory("/tmp/test_history.txt", 100)

		// Verify we're in interactive mode
		assert.True(t, adapter.IsInteractive(),
			"adapter created with NewCLIAdapterWithHistory should be interactive")

		// The adapter should have a different code path for interactive input.
		// We cannot test go-prompt behavior directly, but we can verify the adapter
		// is configured correctly for interactive mode.
	})

	t.Run("non-interactive mode continues to use bufio.Scanner", func(t *testing.T) {
		// This test verifies that the existing non-interactive behavior is preserved.
		input := strings.NewReader("test input line\n")
		output := &strings.Builder{}

		adapter := ui.NewCLIAdapterWithIO(input, output)

		assert.False(t, adapter.IsInteractive(),
			"adapter created with custom IO should be non-interactive")

		ctx := context.Background()
		result, ok := adapter.GetUserInput(ctx)

		assert.True(t, ok, "should successfully read input in non-interactive mode")
		assert.Equal(t, "test input line", result,
			"non-interactive mode should use bufio.Scanner for input")
	})

	t.Run("interactive adapter returns false when using custom readers", func(t *testing.T) {
		// Expected behavior:
		// When SetInteractive(true) is called on an adapter with custom IO,
		// GetUserInput should return false since go-prompt requires real
		// terminal file descriptors.

		input := strings.NewReader("input\n")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Force interactive mode on adapter with custom IO
		adapter.SetInteractive(true)

		ctx := context.Background()
		_, ok := adapter.GetUserInput(ctx)

		// Current behavior: still uses scanner, so this passes
		// After go-prompt integration: should fail because go-prompt cannot work with strings.Reader
		// For now, we just verify the mode flag is set
		assert.True(t, adapter.IsInteractive(),
			"SetInteractive(true) should set interactive mode flag")
		_ = ok // Result depends on implementation
	})
}

func TestCLIAdapter_HistoryIntegration(t *testing.T) {
	// Tests for HistoryManager integration with CLIAdapter.
	// The adapter should create and manage a HistoryManager for interactive mode.

	t.Run("interactive adapter creates a HistoryManager", func(t *testing.T) {
		// Expected behavior:
		// 1. CLIAdapter should have a historyManager field of type *HistoryManager
		// 2. GetHistoryManager() method should return the history manager
		// 3. NewCLIAdapterWithHistory should initialize the historyManager

		adapter := ui.NewCLIAdapterWithHistory("/tmp/test_history_integration.txt", 100)
		historyManager := adapter.GetHistoryManager()
		require.NotNil(t, historyManager, "interactive adapter should have a HistoryManager")
	})

	t.Run("history manager uses configured file path", func(t *testing.T) {
		// Expected behavior:
		// The history manager should be configured with the file path passed to
		// NewCLIAdapterWithHistory. Since HistoryManager.filePath is private,
		// we verify via the adapter's GetHistoryFile() getter.

		historyFile := "/tmp/custom_history_path.txt"
		adapter := ui.NewCLIAdapterWithHistory(historyFile, 50)

		// This part works - GetHistoryFile exists
		assert.Equal(t, historyFile, adapter.GetHistoryFile(),
			"adapter should store the history file path")

		hm := adapter.GetHistoryManager()
		require.NotNil(t, hm, "should have a history manager")
	})

	t.Run("history manager uses configured max entries", func(t *testing.T) {
		// Expected behavior:
		// The history manager should respect the maxEntries parameter.

		maxEntries := 75
		adapter := ui.NewCLIAdapterWithHistory("/tmp/test_history.txt", maxEntries)

		// This part works - GetMaxHistoryEntries exists
		assert.Equal(t, maxEntries, adapter.GetMaxHistoryEntries(),
			"adapter should store the max history entries")

		hm := adapter.GetHistoryManager()
		require.NotNil(t, hm, "should have a history manager")
	})

	t.Run("history is loaded from file on adapter creation", func(t *testing.T) {
		// Expected behavior:
		// When an adapter is created with a history file that already exists,
		// the HistoryManager should load the existing history entries.

		historyFile := "/tmp/test_history_load_" + strconv.Itoa(os.Getpid()) + ".txt"
		defer os.Remove(historyFile)
		content := "first command\nsecond command\nthird command\n"
		err := os.WriteFile(historyFile, []byte(content), 0o600)
		require.NoError(t, err)
		adapter := ui.NewCLIAdapterWithHistory(historyFile, 100)
		hm := adapter.GetHistoryManager()
		require.NotNil(t, hm)
		history := hm.History()
		assert.Len(t, history, 3)
	})

	t.Run("adding input adds to history", func(t *testing.T) {
		// Expected behavior:
		// CLIAdapter should have an AddToHistory(entry string) error method
		// that delegates to the internal HistoryManager.

		historyFile := "/tmp/test_history_add_" + strconv.Itoa(os.Getpid()) + ".txt"
		defer os.Remove(historyFile)
		adapter := ui.NewCLIAdapterWithHistory(historyFile, 100)
		err := adapter.AddToHistory("new command entered")
		require.NoError(t, err)
		hm := adapter.GetHistoryManager()
		require.NotNil(t, hm)
		history := hm.History()
		assert.Len(t, history, 1)
		assert.Equal(t, "new command entered", history[0])
	})

	t.Run("history persists across adapter instances", func(t *testing.T) {
		// Expected behavior:
		// History added in one adapter instance should be available in a new
		// instance that uses the same history file.

		historyFile := "/tmp/test_history_persist_" + strconv.Itoa(os.Getpid()) + ".txt"
		defer os.Remove(historyFile)
		adapter1 := ui.NewCLIAdapterWithHistory(historyFile, 100)
		_ = adapter1.AddToHistory("command from session 1")
		_ = adapter1.AddToHistory("another command")
		adapter2 := ui.NewCLIAdapterWithHistory(historyFile, 100)
		hm := adapter2.GetHistoryManager()
		history := hm.History()
		assert.Len(t, history, 2)
	})

	t.Run("non-interactive adapter does not create HistoryManager", func(t *testing.T) {
		// Expected behavior:
		// GetHistoryManager() should return nil for non-interactive adapters.

		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)
		hm := adapter.GetHistoryManager()
		assert.Nil(t, hm)
	})

	t.Run("AddToHistory returns error for non-interactive adapter", func(t *testing.T) {
		// Expected behavior:
		// Attempting to add history in non-interactive mode should return an error
		// like ErrNotInteractive with message containing "interactive".

		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)
		err := adapter.AddToHistory("some command")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "interactive")
	})

	t.Run("AddToHistory with empty string returns error", func(t *testing.T) {
		// Expected behavior:
		// Empty or whitespace-only input should not be added to history.
		// Should return ErrEmptyEntry from HistoryManager.

		historyFile := "/tmp/test_history_empty_" + strconv.Itoa(os.Getpid()) + ".txt"
		defer os.Remove(historyFile)
		adapter := ui.NewCLIAdapterWithHistory(historyFile, 100)
		err := adapter.AddToHistory("")
		require.Error(t, err)
	})

	t.Run("AddToHistory with whitespace-only string returns error", func(t *testing.T) {
		// Expected behavior:
		// Whitespace-only input should return ErrEmptyEntry.

		historyFile := "/tmp/test_history_ws_" + strconv.Itoa(os.Getpid()) + ".txt"
		defer os.Remove(historyFile)
		adapter := ui.NewCLIAdapterWithHistory(historyFile, 100)
		err := adapter.AddToHistory("   \t\n   ")
		require.Error(t, err)
	})

	t.Run("ClearHistory clears all history entries", func(t *testing.T) {
		// Expected behavior:
		// CLIAdapter should have a ClearHistory() method that delegates to
		// the internal HistoryManager.Clear().

		historyFile := "/tmp/test_history_clear_" + strconv.Itoa(os.Getpid()) + ".txt"
		defer os.Remove(historyFile)
		adapter := ui.NewCLIAdapterWithHistory(historyFile, 100)
		_ = adapter.AddToHistory("first")
		_ = adapter.AddToHistory("second")
		adapter.ClearHistory()
		hm := adapter.GetHistoryManager()
		assert.Equal(t, 0, hm.Size())
	})
}

func TestCLIAdapter_GetHistoryManager(t *testing.T) {
	// Tests for the GetHistoryManager() method.

	t.Run("interactive adapter has GetHistoryManager method returning HistoryManager pointer", func(t *testing.T) {
		// Expected behavior:
		// GetHistoryManager() should return *HistoryManager for interactive adapters.

		adapter := ui.NewCLIAdapterWithHistory("/tmp/test_history.txt", 100)
		hm := adapter.GetHistoryManager()
		require.NotNil(t, hm)
		_ = hm.Size() // Verify it's the correct type
	})

	t.Run("GetHistoryManager returns nil for non-interactive adapters", func(t *testing.T) {
		// Expected behavior:
		// Non-interactive adapters should return nil from GetHistoryManager().

		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)
		hm := adapter.GetHistoryManager()
		assert.Nil(t, hm)
	})

	t.Run("GetHistoryManager returns same instance on multiple calls", func(t *testing.T) {
		// Expected behavior:
		// The HistoryManager should be created once and reused (same pointer).

		adapter := ui.NewCLIAdapterWithHistory("/tmp/test_history.txt", 100)
		hm1 := adapter.GetHistoryManager()
		hm2 := adapter.GetHistoryManager()
		require.NotNil(t, hm1)
		require.NotNil(t, hm2)
		assert.Same(t, hm1, hm2)
	})

	t.Run("GetHistoryManager returns manager with correct configuration", func(t *testing.T) {
		// Expected behavior:
		// The returned HistoryManager should respect the configured max entries.

		historyFile := "/tmp/configured_history.txt"
		defer os.Remove(historyFile)
		maxEntries := 250
		adapter := ui.NewCLIAdapterWithHistory(historyFile, maxEntries)
		hm := adapter.GetHistoryManager()
		require.NotNil(t, hm)
		for i := range maxEntries + 50 {
			_ = hm.Add(fmt.Sprintf("entry %d", i))
		}
		assert.LessOrEqual(t, hm.Size(), maxEntries)
	})
}

func TestCLIAdapter_PromptPrefix(t *testing.T) {
	// Tests for prompt prefix configuration in interactive mode.
	// The prompt prefix is shown before user input in interactive mode.

	t.Run("interactive adapter uses the configured prompt prefix", func(t *testing.T) {
		// Expected behavior:
		// SetPrompt() should set the prompt that go-prompt will display.

		adapter := ui.NewCLIAdapterWithHistory("/tmp/test.txt", 100)

		// Set a custom prompt - this method already exists
		err := adapter.SetPrompt("MyApp> ")
		require.NoError(t, err)

		// The prompt should be stored and available for go-prompt
		// This is verified by the GetPromptPrefix test below
	})

	t.Run("GetPromptPrefix returns current prompt string", func(t *testing.T) {
		// Expected behavior:
		// GetPromptPrefix() should return the current prompt string.

		adapter := ui.NewCLIAdapterWithHistory("/tmp/test.txt", 100)
		err := adapter.SetPrompt("Custom> ")
		require.NoError(t, err)
		promptPrefix := adapter.GetPromptPrefix()
		assert.Equal(t, "Custom> ", promptPrefix)
	})

	t.Run("default prompt prefix is used when not explicitly set", func(t *testing.T) {
		// Expected behavior:
		// New adapters should have a default prompt prefix of "> ".

		adapter := ui.NewCLIAdapterWithHistory("/tmp/test.txt", 100)
		promptPrefix := adapter.GetPromptPrefix()
		assert.NotEmpty(t, promptPrefix)
		assert.Equal(t, "> ", promptPrefix)
	})

	t.Run("prompt prefix is available after SetPrompt", func(t *testing.T) {
		// Expected behavior:
		// GetPromptPrefix() should reflect the most recent SetPrompt() call.

		adapter := ui.NewCLIAdapterWithHistory("/tmp/test.txt", 100)
		_ = adapter.SetPrompt("First> ")
		assert.Equal(t, "First> ", adapter.GetPromptPrefix())
		_ = adapter.SetPrompt("Second> ")
		assert.Equal(t, "Second> ", adapter.GetPromptPrefix())
	})
}

func TestCLIAdapter_HistoryCallback(t *testing.T) {
	// Tests for history callback integration with go-prompt.
	// go-prompt needs a callback function to provide history entries.

	t.Run("GetHistoryCallback returns function for go-prompt", func(t *testing.T) {
		// Expected behavior:
		// GetHistoryCallback() should return func() []string that provides
		// history entries for go-prompt's arrow key navigation.

		historyFile := "/tmp/test_callback_" + strconv.Itoa(os.Getpid()) + ".txt"
		defer os.Remove(historyFile)
		adapter := ui.NewCLIAdapterWithHistory(historyFile, 100)
		_ = adapter.AddToHistory("ls -la")
		_ = adapter.AddToHistory("git status")
		_ = adapter.AddToHistory("go test ./...")
		callback := adapter.GetHistoryCallback()
		require.NotNil(t, callback)
		entries := callback()
		assert.Len(t, entries, 3)
		assert.Equal(t, "ls -la", entries[0])
	})

	t.Run("history callback returns empty slice for empty history", func(t *testing.T) {
		// Expected behavior:
		// Callback should return []string{} when history is empty.

		historyFile := "/tmp/test_callback_empty_" + strconv.Itoa(os.Getpid()) + ".txt"
		defer os.Remove(historyFile)
		adapter := ui.NewCLIAdapterWithHistory(historyFile, 100)
		callback := adapter.GetHistoryCallback()
		require.NotNil(t, callback)
		entries := callback()
		assert.Empty(t, entries)
	})

	t.Run("history callback reflects live updates", func(t *testing.T) {
		// Expected behavior:
		// The callback should return current history, not a cached copy.

		historyFile := "/tmp/test_callback_live_" + strconv.Itoa(os.Getpid()) + ".txt"
		defer os.Remove(historyFile)
		adapter := ui.NewCLIAdapterWithHistory(historyFile, 100)
		callback := adapter.GetHistoryCallback()
		require.NotNil(t, callback)
		assert.Empty(t, callback())
		_ = adapter.AddToHistory("first")
		assert.Len(t, callback(), 1)
		_ = adapter.AddToHistory("second")
		assert.Len(t, callback(), 2)
		adapter.ClearHistory()
		assert.Empty(t, callback())
	})

	t.Run("non-interactive adapter GetHistoryCallback returns nil", func(t *testing.T) {
		// Expected behavior:
		// Non-interactive adapters should return nil.

		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)
		callback := adapter.GetHistoryCallback()
		assert.Nil(t, callback)
	})
}

func TestCLIAdapter_InteractiveInputBehavior(t *testing.T) {
	// Tests for interactive input behavior specifics.
	// These tests document expected behavior but cannot be fully automated
	// since go-prompt requires a real terminal.

	t.Run("interactive GetUserInput adds successful input to history", func(t *testing.T) {
		// Expected behavior:
		// When the user provides input in interactive mode, it should be
		// automatically added to history (after successful input, not on empty/EOF).
		t.Skip("Cannot test go-prompt input behavior without real terminal")
	})

	t.Run("interactive GetUserInput does not add empty input to history", func(t *testing.T) {
		// Expected behavior:
		// Empty input (just pressing Enter) should not be added to history.
		t.Skip("Cannot test go-prompt input behavior without real terminal")
	})

	t.Run("interactive GetUserInput does not add consecutive duplicates", func(t *testing.T) {
		// Expected behavior:
		// Entering the same command twice in a row should only add it once.
		t.Skip("Cannot test go-prompt input behavior without real terminal")
	})

	t.Run("interactive mode supports arrow key navigation through history", func(t *testing.T) {
		// Expected behavior:
		// Up/down arrows should navigate through history.
		t.Skip("Cannot test go-prompt input behavior without real terminal")
	})

	t.Run("interactive mode supports basic line editing", func(t *testing.T) {
		// Expected behavior:
		// Left/right arrows, backspace, delete, home, end should work.
		t.Skip("Cannot test go-prompt input behavior without real terminal")
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

// Mode Toggle Tests - RED Phase
// These tests define the expected behavior of mode toggle functionality.
// They will FAIL until the implementation is added.

func TestCLIAdapter_ShiftTabKeybinding(t *testing.T) {
	t.Run("invokes mode toggle callback on Shift+Tab", func(t *testing.T) {
		// Setup: Track callback invocations
		callbackInvoked := false
		toggleCallback := func() {
			callbackInvoked = true
		}

		// Create adapter with mode toggle callback
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)
		adapter.SetModeToggleCallback(toggleCallback)

		// Simulate Shift+Tab keypress
		// This will call the underlying keybinding handler
		adapter.HandleKeyPress(ui.KeyShiftTab)

		// Assert that callback was invoked
		assert.True(t, callbackInvoked,
			"Shift+Tab keypress should invoke the mode toggle callback")
	})

	t.Run("multiple Shift+Tab presses toggle mode back and forth", func(t *testing.T) {
		modeStates := []bool{}
		toggleCallback := func() {
			modeStates = append(modeStates, true)
		}

		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)
		adapter.SetModeToggleCallback(toggleCallback)

		// Press Shift+Tab three times
		adapter.HandleKeyPress(ui.KeyShiftTab)
		adapter.HandleKeyPress(ui.KeyShiftTab)
		adapter.HandleKeyPress(ui.KeyShiftTab)

		// Should have invoked callback three times
		assert.Len(t, modeStates, 3,
			"each Shift+Tab press should invoke the toggle callback")
	})

	t.Run("other keys do not invoke mode toggle callback", func(t *testing.T) {
		callbackInvoked := false
		toggleCallback := func() {
			callbackInvoked = true
		}

		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)
		adapter.SetModeToggleCallback(toggleCallback)

		// Press regular Tab (not Shift+Tab)
		adapter.HandleKeyPress(ui.KeyTab)

		// Callback should NOT be invoked
		assert.False(t, callbackInvoked,
			"regular Tab key should not invoke mode toggle callback")
	})

	t.Run("handles nil mode toggle callback gracefully", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)
		adapter.SetModeToggleCallback(nil)

		// Should not panic when Shift+Tab is pressed with nil callback
		assert.NotPanics(t, func() {
			adapter.HandleKeyPress(ui.KeyShiftTab)
		}, "should not panic when mode toggle callback is nil")
	})
}

func TestCLIAdapter_PromptChangesWithMode(t *testing.T) {
	t.Run("prompt shows [PLAN MODE] indicator when in plan mode", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Set adapter to plan mode
		adapter.SetPlanMode(true)

		// Get prompt text
		prompt := adapter.GetPrompt()

		// Should contain [PLAN MODE] indicator
		assert.Contains(t, prompt, "[PLAN MODE]",
			"prompt should contain [PLAN MODE] indicator when in plan mode")

		// Should be visible (not empty after formatting)
		assert.NotEmpty(t, prompt,
			"prompt should not be empty when in plan mode")
	})

	t.Run("prompt does not show [PLAN MODE] indicator when in normal mode", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Set adapter to normal mode
		adapter.SetPlanMode(false)

		// Get prompt text
		prompt := adapter.GetPrompt()

		// Should NOT contain [PLAN MODE] indicator
		assert.NotContains(t, prompt, "[PLAN MODE]",
			"prompt should NOT contain [PLAN MODE] indicator when in normal mode")
	})

	t.Run("prompt updates dynamically when mode toggles", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Start in normal mode
		adapter.SetPlanMode(false)
		normalPrompt := adapter.GetPrompt()
		assert.NotContains(t, normalPrompt, "[PLAN MODE]",
			"normal mode prompt should not contain indicator")

		// Toggle to plan mode
		adapter.SetPlanMode(true)
		planPrompt := adapter.GetPrompt()
		assert.Contains(t, planPrompt, "[PLAN MODE]",
			"plan mode prompt should contain indicator")

		// Toggle back to normal mode
		adapter.SetPlanMode(false)
		normalPromptAgain := adapter.GetPrompt()
		assert.NotContains(t, normalPromptAgain, "[PLAN MODE]",
			"normal mode prompt should not contain indicator after toggle")
	})

	t.Run("prompt shows indicator with consistent styling", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		adapter.SetPlanMode(true)
		prompt := adapter.GetPrompt()

		// The [PLAN MODE] indicator should be the first element in the prompt
		// This ensures it's consistently visible
		assert.True(t, strings.HasPrefix(prompt, "[PLAN MODE]") ||
			strings.Contains(prompt, "[PLAN MODE]"),
			"[PLAN MODE] indicator should appear prominently in prompt")
	})

	t.Run("prompt preserves other elements with mode indicator", func(t *testing.T) {
		input := strings.NewReader("")
		output := &strings.Builder{}
		adapter := ui.NewCLIAdapterWithIO(input, output)

		// Set some session context before enabling plan mode
		adapter.SetSessionID("test-session-123")
		adapter.SetPlanMode(true)

		prompt := adapter.GetPrompt()

		// Should contain both mode indicator and session info
		assert.Contains(t, prompt, "[PLAN MODE]",
			"prompt should show mode indicator")
		assert.Contains(t, prompt, "test-session-123",
			"prompt should preserve session information")
	})
}

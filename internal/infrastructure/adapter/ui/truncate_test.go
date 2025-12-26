package ui_test

import (
	"code-editing-agent/internal/infrastructure/adapter/ui"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// TDD Red Phase Tests for TruncateOutput
// These tests define the expected behavior for output truncation functionality.
// All tests should FAIL initially until the implementation is complete.
// =============================================================================

// generateLines creates a string with the specified number of lines.
// Each line contains "Line N" where N is the 1-indexed line number.
func generateLines(count int) string {
	if count == 0 {
		return ""
	}
	lines := make([]string, count)
	for i := range count {
		lines[i] = fmt.Sprintf("Line %d", i+1)
	}
	return strings.Join(lines, "\n")
}

// =============================================================================
// Test: DefaultTruncationConfig
// =============================================================================

func TestDefaultTruncationConfig(t *testing.T) {
	t.Run("should have HeadLines default of 20", func(t *testing.T) {
		config := ui.DefaultTruncationConfig()

		assert.Equal(t, 20, config.HeadLines,
			"HeadLines should default to 20")
	})

	t.Run("should have TailLines default of 10", func(t *testing.T) {
		config := ui.DefaultTruncationConfig()

		assert.Equal(t, 10, config.TailLines,
			"TailLines should default to 10")
	})

	t.Run("should have Enabled default of true", func(t *testing.T) {
		config := ui.DefaultTruncationConfig()

		assert.True(t, config.Enabled,
			"Enabled should default to true")
	})

	t.Run("should return consistent defaults on multiple calls", func(t *testing.T) {
		config1 := ui.DefaultTruncationConfig()
		config2 := ui.DefaultTruncationConfig()

		assert.Equal(t, config1.HeadLines, config2.HeadLines,
			"HeadLines should be consistent across calls")
		assert.Equal(t, config1.TailLines, config2.TailLines,
			"TailLines should be consistent across calls")
		assert.Equal(t, config1.Enabled, config2.Enabled,
			"Enabled should be consistent across calls")
	})
}

// =============================================================================
// Test: TruncateOutput - Empty Output
// =============================================================================

func TestTruncateOutput_EmptyOutput(t *testing.T) {
	t.Run("should return empty string and zero lines removed for empty input", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}

		result, linesRemoved := ui.TruncateOutput("", config)

		assert.Empty(t, result,
			"empty input should return empty output")
		assert.Equal(t, 0, linesRemoved,
			"empty input should report zero lines removed")
	})

	t.Run("should return empty string even when truncation is disabled", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   false,
		}

		result, linesRemoved := ui.TruncateOutput("", config)

		assert.Empty(t, result,
			"empty input should return empty output when disabled")
		assert.Equal(t, 0, linesRemoved,
			"empty input should report zero lines removed when disabled")
	})
}

// =============================================================================
// Test: TruncateOutput - Single Line
// =============================================================================

func TestTruncateOutput_SingleLine(t *testing.T) {
	t.Run("should return single line unchanged", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := "This is a single line of output"

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, input, result,
			"single line should be returned unchanged")
		assert.Equal(t, 0, linesRemoved,
			"single line should report zero lines removed")
	})

	t.Run("should handle single line with trailing newline", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := "Single line with newline\n"

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, input, result,
			"single line with trailing newline should be returned unchanged")
		assert.Equal(t, 0, linesRemoved,
			"single line with trailing newline should report zero lines removed")
	})

	t.Run("should handle single empty line (just newline)", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := "\n"

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, input, result,
			"single newline should be returned unchanged")
		assert.Equal(t, 0, linesRemoved,
			"single newline should report zero lines removed")
	})
}

// =============================================================================
// Test: TruncateOutput - Within Threshold
// =============================================================================

func TestTruncateOutput_WithinThreshold(t *testing.T) {
	t.Run("should return 30 lines unchanged when exactly at threshold", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := generateLines(30) // Exactly HeadLines + TailLines

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, input, result,
			"output at exactly threshold should be returned unchanged")
		assert.Equal(t, 0, linesRemoved,
			"output at exactly threshold should report zero lines removed")
	})

	t.Run("should return 29 lines unchanged when below threshold", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := generateLines(29) // Below threshold

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, input, result,
			"output below threshold should be returned unchanged")
		assert.Equal(t, 0, linesRemoved,
			"output below threshold should report zero lines removed")
	})

	t.Run("should return 15 lines unchanged when well below threshold", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := generateLines(15)

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, input, result,
			"output well below threshold should be returned unchanged")
		assert.Equal(t, 0, linesRemoved,
			"output well below threshold should report zero lines removed")
	})

	t.Run("should handle custom threshold values", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 5,
			TailLines: 5,
			Enabled:   true,
		}
		input := generateLines(10) // Exactly at custom threshold

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, input, result,
			"output at custom threshold should be returned unchanged")
		assert.Equal(t, 0, linesRemoved,
			"output at custom threshold should report zero lines removed")
	})
}

// =============================================================================
// Test: TruncateOutput - Exceeds Threshold
// =============================================================================

func TestTruncateOutput_ExceedsThreshold(t *testing.T) {
	t.Run("should truncate 50 lines showing head and tail with indicator", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := generateLines(50) // 50 lines total, should truncate 20

		result, linesRemoved := ui.TruncateOutput(input, config)

		// Verify lines removed count
		assert.Equal(t, 20, linesRemoved,
			"should report 20 lines removed (50 - 20 - 10 = 20)")

		// Verify the truncation indicator is present
		assert.Contains(t, result, "[... 20 lines truncated ...]",
			"should contain truncation indicator with correct count")

		// Verify head lines are preserved (first 20 lines)
		for i := 1; i <= 20; i++ {
			assert.Contains(t, result, fmt.Sprintf("Line %d", i),
				"should contain head line %d", i)
		}

		// Verify tail lines are preserved (last 10 lines)
		for i := 41; i <= 50; i++ {
			assert.Contains(t, result, fmt.Sprintf("Line %d", i),
				"should contain tail line %d", i)
		}

		// Verify middle lines are NOT present
		for i := 21; i <= 40; i++ {
			assert.NotContains(t, result, fmt.Sprintf("Line %d\n", i),
				"should NOT contain truncated line %d", i)
		}
	})

	t.Run("should truncate 31 lines removing exactly 1 line", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := generateLines(31) // Just over threshold

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, 1, linesRemoved,
			"should report 1 line removed (31 - 20 - 10 = 1)")
		assert.Contains(t, result, "[... 1 lines truncated ...]",
			"should contain truncation indicator for 1 line")
	})

	t.Run("should truncate 100 lines correctly", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := generateLines(100)

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, 70, linesRemoved,
			"should report 70 lines removed (100 - 20 - 10 = 70)")
		assert.Contains(t, result, "[... 70 lines truncated ...]",
			"should contain truncation indicator with correct count")

		// Verify first line is present
		assert.Contains(t, result, "Line 1",
			"should contain first line")

		// Verify line 20 is present (last of head)
		assert.Contains(t, result, "Line 20",
			"should contain line 20 (last of head)")

		// Verify line 91 is present (first of tail)
		assert.Contains(t, result, "Line 91",
			"should contain line 91 (first of tail)")

		// Verify line 100 is present (last line)
		assert.Contains(t, result, "Line 100",
			"should contain last line")
	})

	t.Run("should handle custom head and tail values", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 5,
			TailLines: 3,
			Enabled:   true,
		}
		input := generateLines(20) // 20 lines, threshold is 8

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, 12, linesRemoved,
			"should report 12 lines removed (20 - 5 - 3 = 12)")
		assert.Contains(t, result, "[... 12 lines truncated ...]",
			"should contain truncation indicator with correct count")

		// Verify head lines (1-5)
		for i := 1; i <= 5; i++ {
			assert.Contains(t, result, fmt.Sprintf("Line %d", i),
				"should contain head line %d", i)
		}

		// Verify tail lines (18-20)
		for i := 18; i <= 20; i++ {
			assert.Contains(t, result, fmt.Sprintf("Line %d", i),
				"should contain tail line %d", i)
		}
	})
}

// =============================================================================
// Test: TruncateOutput - Disabled
// =============================================================================

func TestTruncateOutput_Disabled(t *testing.T) {
	t.Run("should return original unchanged when truncation disabled", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   false, // Disabled
		}
		input := generateLines(100) // Would normally truncate

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, input, result,
			"should return original output when truncation is disabled")
		assert.Equal(t, 0, linesRemoved,
			"should report zero lines removed when truncation is disabled")
	})

	t.Run("should return very large output unchanged when disabled", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   false,
		}
		input := generateLines(1000) // Very large output

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, input, result,
			"should return large output unchanged when disabled")
		assert.Equal(t, 0, linesRemoved,
			"should report zero lines removed for large output when disabled")
	})

	t.Run("should handle small output unchanged when disabled", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   false,
		}
		input := generateLines(5)

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, input, result,
			"should return small output unchanged when disabled")
		assert.Equal(t, 0, linesRemoved,
			"should report zero lines removed for small output when disabled")
	})
}

// =============================================================================
// Test: TruncateOutput - Edge Cases
// =============================================================================

func TestTruncateOutput_EdgeCases(t *testing.T) {
	t.Run("should handle output with only whitespace lines", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 2,
			TailLines: 2,
			Enabled:   true,
		}
		input := "   \n\t\n   \n\t\n   \n" // 5 whitespace lines

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, 1, linesRemoved,
			"should truncate whitespace lines correctly")
		assert.Contains(t, result, "[... 1 lines truncated ...]",
			"should contain truncation indicator")
	})

	t.Run("should handle output with mixed content", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 2,
			TailLines: 2,
			Enabled:   true,
		}
		lines := []string{
			"First line",
			"",
			"Third line",
			"Fourth line",
			"Fifth line",
			"Sixth line",
		}
		input := strings.Join(lines, "\n")

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, 2, linesRemoved,
			"should report correct lines removed with mixed content")
		assert.Contains(t, result, "First line",
			"should preserve first line")
		assert.Contains(t, result, "Sixth line",
			"should preserve last line")
	})

	t.Run("should handle output with special characters", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 2,
			TailLines: 2,
			Enabled:   true,
		}
		lines := []string{
			"Line with special chars: !@#$%^&*()",
			"Line with unicode: \u4e2d\u6587",
			"Line with tabs:\t\ttabbed",
			"Line with quotes: \"quoted\"",
			"Line with backslash: C:\\path\\file",
			"Final line",
		}
		input := strings.Join(lines, "\n")

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, 2, linesRemoved,
			"should report correct lines removed with special characters")
		assert.Contains(t, result, "!@#$%^&*()",
			"should preserve special characters in head")
		assert.Contains(t, result, "Final line",
			"should preserve last line")
	})

	t.Run("should handle output with ANSI color codes", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 2,
			TailLines: 2,
			Enabled:   true,
		}
		lines := []string{
			"\x1b[31mRed line\x1b[0m",
			"\x1b[32mGreen line\x1b[0m",
			"\x1b[33mYellow line\x1b[0m",
			"\x1b[34mBlue line\x1b[0m",
			"\x1b[35mMagenta line\x1b[0m",
			"Normal line",
		}
		input := strings.Join(lines, "\n")

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, 2, linesRemoved,
			"should report correct lines removed with ANSI codes")
		assert.Contains(t, result, "\x1b[31mRed line\x1b[0m",
			"should preserve ANSI codes in head")
	})

	t.Run("should handle HeadLines of zero", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 0,
			TailLines: 10,
			Enabled:   true,
		}
		input := generateLines(20)

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, 10, linesRemoved,
			"should remove correct lines with HeadLines=0")
		// Only tail should be present
		assert.Contains(t, result, "Line 11",
			"should contain first tail line")
		assert.Contains(t, result, "Line 20",
			"should contain last line")
		// Head should not be present
		assert.NotContains(t, result, "Line 1\n",
			"should not contain head lines when HeadLines=0")
	})

	t.Run("should handle TailLines of zero", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 10,
			TailLines: 0,
			Enabled:   true,
		}
		input := generateLines(20)

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, 10, linesRemoved,
			"should remove correct lines with TailLines=0")
		// Only head should be present
		assert.Contains(t, result, "Line 1",
			"should contain first line")
		assert.Contains(t, result, "Line 10",
			"should contain last head line")
	})

	t.Run("should handle very long lines", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 2,
			TailLines: 2,
			Enabled:   true,
		}
		longLine := strings.Repeat("x", 10000)
		lines := []string{
			longLine,
			"Short line 2",
			"Short line 3",
			"Short line 4",
			"Short line 5",
			longLine,
		}
		input := strings.Join(lines, "\n")

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, 2, linesRemoved,
			"should truncate correctly with very long lines")
		assert.Contains(t, result, longLine,
			"should preserve long lines in output")
	})

	t.Run("should handle trailing newline in input", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := generateLines(50) + "\n" // Add trailing newline

		result, linesRemoved := ui.TruncateOutput(input, config)

		// The trailing newline creates an extra empty line
		// Verify truncation still works correctly
		require.Positive(t, linesRemoved,
			"should truncate output with trailing newline")
		assert.Contains(t, result, "[...",
			"should contain truncation indicator")
	})

	t.Run("should handle Windows-style line endings (CRLF)", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 2,
			TailLines: 2,
			Enabled:   true,
		}
		lines := []string{"Line 1", "Line 2", "Line 3", "Line 4", "Line 5", "Line 6"}
		input := strings.Join(lines, "\r\n") // Windows line endings

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, 2, linesRemoved,
			"should handle CRLF line endings correctly")
		assert.Contains(t, result, "Line 1",
			"should preserve first line with CRLF")
		assert.Contains(t, result, "Line 6",
			"should preserve last line with CRLF")
	})
}

// =============================================================================
// Test: TruncateOutput - Table-Driven Tests
// =============================================================================

func TestTruncateOutput_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		lineCount     int
		headLines     int
		tailLines     int
		enabled       bool
		wantRemoved   int
		wantTruncated bool
	}{
		{
			name:          "exactly at threshold - no truncation",
			lineCount:     30,
			headLines:     20,
			tailLines:     10,
			enabled:       true,
			wantRemoved:   0,
			wantTruncated: false,
		},
		{
			name:          "one over threshold - truncate 1 line",
			lineCount:     31,
			headLines:     20,
			tailLines:     10,
			enabled:       true,
			wantRemoved:   1,
			wantTruncated: true,
		},
		{
			name:          "50 lines - truncate 20",
			lineCount:     50,
			headLines:     20,
			tailLines:     10,
			enabled:       true,
			wantRemoved:   20,
			wantTruncated: true,
		},
		{
			name:          "100 lines - truncate 70",
			lineCount:     100,
			headLines:     20,
			tailLines:     10,
			enabled:       true,
			wantRemoved:   70,
			wantTruncated: true,
		},
		{
			name:          "disabled - no truncation for large output",
			lineCount:     100,
			headLines:     20,
			tailLines:     10,
			enabled:       false,
			wantRemoved:   0,
			wantTruncated: false,
		},
		{
			name:          "custom small threshold",
			lineCount:     15,
			headLines:     5,
			tailLines:     5,
			enabled:       true,
			wantRemoved:   5,
			wantTruncated: true,
		},
		{
			name:          "below threshold - no truncation",
			lineCount:     10,
			headLines:     20,
			tailLines:     10,
			enabled:       true,
			wantRemoved:   0,
			wantTruncated: false,
		},
		{
			name:          "single line - no truncation",
			lineCount:     1,
			headLines:     20,
			tailLines:     10,
			enabled:       true,
			wantRemoved:   0,
			wantTruncated: false,
		},
		{
			name:          "very large output",
			lineCount:     10000,
			headLines:     20,
			tailLines:     10,
			enabled:       true,
			wantRemoved:   9970,
			wantTruncated: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ui.TruncationConfig{
				HeadLines: tt.headLines,
				TailLines: tt.tailLines,
				Enabled:   tt.enabled,
			}
			input := generateLines(tt.lineCount)

			result, linesRemoved := ui.TruncateOutput(input, config)

			assert.Equal(t, tt.wantRemoved, linesRemoved,
				"linesRemoved mismatch")

			if tt.wantTruncated {
				assert.Contains(t, result,
					fmt.Sprintf("[... %d lines truncated ...]", tt.wantRemoved),
					"should contain truncation indicator")
			} else {
				assert.NotContains(t, result, "[...",
					"should not contain truncation indicator")
				assert.Equal(t, input, result,
					"output should be unchanged when not truncated")
			}
		})
	}
}

// =============================================================================
// Test: TruncateOutput - Output Format Verification
// =============================================================================

func TestTruncateOutput_OutputFormat(t *testing.T) {
	t.Run("should have correct line order in truncated output", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 3,
			TailLines: 3,
			Enabled:   true,
		}
		input := generateLines(10)

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, 4, linesRemoved,
			"should report 4 lines removed")

		// Verify structure: head lines, indicator, tail lines
		lines := strings.Split(result, "\n")
		require.GreaterOrEqual(t, len(lines), 7,
			"should have at least 7 lines (3 head + 1 indicator + 3 tail)")

		// First 3 lines should be the head
		assert.Equal(t, "Line 1", lines[0], "first line should be Line 1")
		assert.Equal(t, "Line 2", lines[1], "second line should be Line 2")
		assert.Equal(t, "Line 3", lines[2], "third line should be Line 3")

		// Find the indicator line
		indicatorFound := false
		indicatorIndex := -1
		for i, line := range lines {
			if strings.Contains(line, "[... 4 lines truncated ...]") {
				indicatorFound = true
				indicatorIndex = i
				break
			}
		}
		assert.True(t, indicatorFound, "should find truncation indicator")
		assert.Equal(t, 3, indicatorIndex, "indicator should be at index 3")

		// Tail lines should follow the indicator
		assert.Equal(t, "Line 8", lines[indicatorIndex+1], "first tail line should be Line 8")
		assert.Equal(t, "Line 9", lines[indicatorIndex+2], "second tail line should be Line 9")
		assert.Equal(t, "Line 10", lines[indicatorIndex+3], "third tail line should be Line 10")
	})

	t.Run("truncation indicator should be on its own line", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 5,
			TailLines: 5,
			Enabled:   true,
		}
		input := generateLines(20)

		result, _ := ui.TruncateOutput(input, config)

		lines := strings.Split(result, "\n")
		indicatorLineFound := false
		for _, line := range lines {
			if line == "[... 10 lines truncated ...]" {
				indicatorLineFound = true
				break
			}
		}
		assert.True(t, indicatorLineFound,
			"truncation indicator should be on its own separate line")
	})
}

// =============================================================================
// Test: TruncateOutput - Boundary Conditions
// =============================================================================

func TestTruncateOutput_BoundaryConditions(t *testing.T) {
	t.Run("threshold minus one - no truncation", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := generateLines(29) // threshold - 1

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, input, result,
			"should not truncate at threshold - 1")
		assert.Equal(t, 0, linesRemoved,
			"should report zero lines removed at threshold - 1")
	})

	t.Run("threshold exactly - no truncation", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := generateLines(30) // exactly threshold

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, input, result,
			"should not truncate at exactly threshold")
		assert.Equal(t, 0, linesRemoved,
			"should report zero lines removed at exactly threshold")
	})

	t.Run("threshold plus one - truncate 1", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := generateLines(31) // threshold + 1

		result, linesRemoved := ui.TruncateOutput(input, config)

		assert.Equal(t, 1, linesRemoved,
			"should truncate 1 line at threshold + 1")
		assert.Contains(t, result, "[... 1 lines truncated ...]",
			"should contain truncation indicator for 1 line")
		assert.NotEqual(t, input, result,
			"output should be different from input when truncated")
	})
}

// =============================================================================
// TDD Red Phase Tests for TruncateBashOutput
// These tests define the expected behavior for bash JSON output truncation.
// All tests should FAIL initially until the implementation is complete.
// =============================================================================

// bashOutput represents the JSON structure returned by the bash tool.
type bashOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// makeBashJSON creates a valid bash JSON output string from the given fields.
func makeBashJSON(stdout, stderr string, exitCode int) string {
	output := bashOutput{
		Stdout:   stdout,
		Stderr:   stderr,
		ExitCode: exitCode,
	}
	data, _ := json.Marshal(output)
	return string(data)
}

// parseBashJSON parses a bash JSON output string into its component fields.
func parseBashJSON(input string) (bashOutput, error) {
	var output bashOutput
	err := json.Unmarshal([]byte(input), &output)
	return output, err
}

// =============================================================================
// Test: TruncateBashOutput - Truncates Stdout
// =============================================================================

func TestTruncateBashOutput_TruncatesStdout(t *testing.T) {
	t.Run("should truncate large stdout field in JSON", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		largeStdout := generateLines(50) // 50 lines, should truncate 20
		input := makeBashJSON(largeStdout, "", 0)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		// Should report lines removed from stdout
		assert.Equal(t, 20, linesRemoved,
			"should report 20 lines removed from stdout (50 - 20 - 10 = 20)")

		// Result should be valid JSON
		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		// Stdout should contain truncation indicator
		assert.Contains(t, parsed.Stdout, "[... 20 lines truncated ...]",
			"stdout should contain truncation indicator")

		// Stdout should contain head lines
		assert.Contains(t, parsed.Stdout, "Line 1",
			"stdout should contain first line")
		assert.Contains(t, parsed.Stdout, "Line 20",
			"stdout should contain last head line")

		// Stdout should contain tail lines
		assert.Contains(t, parsed.Stdout, "Line 41",
			"stdout should contain first tail line")
		assert.Contains(t, parsed.Stdout, "Line 50",
			"stdout should contain last line")

		// Stderr should remain empty
		assert.Empty(t, parsed.Stderr, "stderr should remain empty")
	})

	t.Run("should truncate 100 line stdout correctly", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		largeStdout := generateLines(100)
		input := makeBashJSON(largeStdout, "", 0)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		assert.Equal(t, 70, linesRemoved,
			"should report 70 lines removed from stdout")

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Contains(t, parsed.Stdout, "[... 70 lines truncated ...]",
			"stdout should contain correct truncation indicator")
	})
}

// =============================================================================
// Test: TruncateBashOutput - Truncates Stderr
// =============================================================================

func TestTruncateBashOutput_TruncatesStderr(t *testing.T) {
	t.Run("should truncate large stderr field in JSON", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		largeStderr := generateLines(50) // 50 lines, should truncate 20
		input := makeBashJSON("", largeStderr, 1)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		// Should report lines removed from stderr
		assert.Equal(t, 20, linesRemoved,
			"should report 20 lines removed from stderr (50 - 20 - 10 = 20)")

		// Result should be valid JSON
		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		// Stderr should contain truncation indicator
		assert.Contains(t, parsed.Stderr, "[... 20 lines truncated ...]",
			"stderr should contain truncation indicator")

		// Stderr should contain head lines
		assert.Contains(t, parsed.Stderr, "Line 1",
			"stderr should contain first line")
		assert.Contains(t, parsed.Stderr, "Line 20",
			"stderr should contain last head line")

		// Stderr should contain tail lines
		assert.Contains(t, parsed.Stderr, "Line 41",
			"stderr should contain first tail line")
		assert.Contains(t, parsed.Stderr, "Line 50",
			"stderr should contain last line")

		// Stdout should remain empty
		assert.Empty(t, parsed.Stdout, "stdout should remain empty")
	})

	t.Run("should truncate 100 line stderr correctly", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		largeStderr := generateLines(100)
		input := makeBashJSON("", largeStderr, 1)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		assert.Equal(t, 70, linesRemoved,
			"should report 70 lines removed from stderr")

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Contains(t, parsed.Stderr, "[... 70 lines truncated ...]",
			"stderr should contain correct truncation indicator")
	})
}

// =============================================================================
// Test: TruncateBashOutput - Truncates Both Fields
// =============================================================================

func TestTruncateBashOutput_TruncatesBothFields(t *testing.T) {
	t.Run("should truncate both stdout and stderr when both are large", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		largeStdout := generateLines(50) // 20 lines truncated
		largeStderr := generateLines(60) // 30 lines truncated
		input := makeBashJSON(largeStdout, largeStderr, 0)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		// Should report total lines removed from both fields
		assert.Equal(t, 50, linesRemoved,
			"should report 50 total lines removed (20 from stdout + 30 from stderr)")

		// Result should be valid JSON
		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		// Both fields should contain truncation indicators
		assert.Contains(t, parsed.Stdout, "[... 20 lines truncated ...]",
			"stdout should contain truncation indicator")
		assert.Contains(t, parsed.Stderr, "[... 30 lines truncated ...]",
			"stderr should contain truncation indicator")
	})

	t.Run("should truncate both fields with equal sizes", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 10,
			TailLines: 5,
			Enabled:   true,
		}
		stdout := generateLines(40) // 25 lines truncated
		stderr := generateLines(40) // 25 lines truncated
		input := makeBashJSON(stdout, stderr, 0)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		assert.Equal(t, 50, linesRemoved,
			"should report 50 total lines removed (25 from each)")

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Contains(t, parsed.Stdout, "[... 25 lines truncated ...]",
			"stdout should show 25 lines truncated")
		assert.Contains(t, parsed.Stderr, "[... 25 lines truncated ...]",
			"stderr should show 25 lines truncated")
	})

	t.Run("should handle one field truncated and one not", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		largeStdout := generateLines(50) // 20 lines truncated
		smallStderr := generateLines(10) // no truncation needed
		input := makeBashJSON(largeStdout, smallStderr, 0)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		assert.Equal(t, 20, linesRemoved,
			"should report 20 lines removed (only from stdout)")

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Contains(t, parsed.Stdout, "[... 20 lines truncated ...]",
			"stdout should be truncated")
		assert.NotContains(t, parsed.Stderr, "[...",
			"stderr should not be truncated")
		assert.Equal(t, smallStderr, parsed.Stderr,
			"stderr should be unchanged")
	})
}

// =============================================================================
// Test: TruncateBashOutput - Preserves Small Output
// =============================================================================

func TestTruncateBashOutput_PreservesSmallOutput(t *testing.T) {
	t.Run("should not truncate when stdout and stderr are small", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		smallStdout := generateLines(15)
		smallStderr := generateLines(10)
		input := makeBashJSON(smallStdout, smallStderr, 0)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		assert.Equal(t, 0, linesRemoved,
			"should report zero lines removed when both fields are small")

		// Result should be valid JSON with unchanged content
		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Equal(t, smallStdout, parsed.Stdout,
			"stdout should be unchanged")
		assert.Equal(t, smallStderr, parsed.Stderr,
			"stderr should be unchanged")
	})

	t.Run("should not truncate exactly at threshold", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		exactStdout := generateLines(30) // Exactly HeadLines + TailLines
		exactStderr := generateLines(30)
		input := makeBashJSON(exactStdout, exactStderr, 0)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		assert.Equal(t, 0, linesRemoved,
			"should report zero lines removed at exactly threshold")

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Equal(t, exactStdout, parsed.Stdout,
			"stdout should be unchanged at threshold")
		assert.Equal(t, exactStderr, parsed.Stderr,
			"stderr should be unchanged at threshold")
	})

	t.Run("should handle empty stdout and stderr", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := makeBashJSON("", "", 0)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		assert.Equal(t, 0, linesRemoved,
			"should report zero lines removed for empty fields")

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Empty(t, parsed.Stdout, "stdout should remain empty")
		assert.Empty(t, parsed.Stderr, "stderr should remain empty")
	})

	t.Run("should handle single line output", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		input := makeBashJSON("single line stdout", "single line stderr", 0)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		assert.Equal(t, 0, linesRemoved,
			"should report zero lines removed for single line output")

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Equal(t, "single line stdout", parsed.Stdout,
			"stdout should be unchanged")
		assert.Equal(t, "single line stderr", parsed.Stderr,
			"stderr should be unchanged")
	})
}

// =============================================================================
// Test: TruncateBashOutput - Preserves Exit Code
// =============================================================================

func TestTruncateBashOutput_PreservesExitCode(t *testing.T) {
	t.Run("should preserve exit_code 0 after truncation", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		largeStdout := generateLines(50)
		input := makeBashJSON(largeStdout, "", 0)

		result, _ := ui.TruncateBashOutput(input, config)

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Equal(t, 0, parsed.ExitCode,
			"exit_code should be preserved as 0")
	})

	t.Run("should preserve exit_code 1 after truncation", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		largeStderr := generateLines(50)
		input := makeBashJSON("", largeStderr, 1)

		result, _ := ui.TruncateBashOutput(input, config)

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Equal(t, 1, parsed.ExitCode,
			"exit_code should be preserved as 1")
	})

	t.Run("should preserve non-standard exit codes", func(t *testing.T) {
		testCases := []int{2, 127, 128, 130, 137, 255}

		for _, exitCode := range testCases {
			t.Run(fmt.Sprintf("exit_code_%d", exitCode), func(t *testing.T) {
				config := ui.TruncationConfig{
					HeadLines: 20,
					TailLines: 10,
					Enabled:   true,
				}
				largeStdout := generateLines(50)
				input := makeBashJSON(largeStdout, "", exitCode)

				result, _ := ui.TruncateBashOutput(input, config)

				parsed, err := parseBashJSON(result)
				require.NoError(t, err, "result should be valid JSON")

				assert.Equal(t, exitCode, parsed.ExitCode,
					"exit_code should be preserved as %d", exitCode)
			})
		}
	})

	t.Run("should preserve exit_code when no truncation occurs", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		smallStdout := generateLines(10)
		input := makeBashJSON(smallStdout, "", 42)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		assert.Equal(t, 0, linesRemoved,
			"should not truncate small output")

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Equal(t, 42, parsed.ExitCode,
			"exit_code should be preserved as 42")
	})
}

// =============================================================================
// Test: TruncateBashOutput - Fallback for Non-JSON
// =============================================================================

func TestTruncateBashOutput_FallbackForNonJSON(t *testing.T) {
	t.Run("should fall back to TruncateOutput for plain text", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		plainText := generateLines(50) // Not JSON, just plain text

		result, linesRemoved := ui.TruncateBashOutput(plainText, config)

		// Should behave exactly like TruncateOutput
		expectedResult, expectedRemoved := ui.TruncateOutput(plainText, config)

		assert.Equal(t, expectedRemoved, linesRemoved,
			"should report same lines removed as TruncateOutput")
		assert.Equal(t, expectedResult, result,
			"result should match TruncateOutput result")
	})

	t.Run("should fall back for invalid JSON", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		invalidJSON := `{"stdout": "missing closing brace"`

		result, linesRemoved := ui.TruncateBashOutput(invalidJSON, config)

		expectedResult, expectedRemoved := ui.TruncateOutput(invalidJSON, config)

		assert.Equal(t, expectedRemoved, linesRemoved,
			"should fall back to TruncateOutput for invalid JSON")
		assert.Equal(t, expectedResult, result,
			"result should match TruncateOutput for invalid JSON")
	})

	t.Run("should fall back for JSON without required fields", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		// Valid JSON but missing stdout/stderr/exit_code structure
		wrongStructure := `{"message": "hello", "code": 200}`

		result, linesRemoved := ui.TruncateBashOutput(wrongStructure, config)

		expectedResult, expectedRemoved := ui.TruncateOutput(wrongStructure, config)

		assert.Equal(t, expectedRemoved, linesRemoved,
			"should fall back for JSON without bash fields")
		assert.Equal(t, expectedResult, result,
			"result should match TruncateOutput for wrong JSON structure")
	})

	t.Run("should fall back for JSON array", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		jsonArray := `[1, 2, 3, 4, 5]`

		result, linesRemoved := ui.TruncateBashOutput(jsonArray, config)

		expectedResult, expectedRemoved := ui.TruncateOutput(jsonArray, config)

		assert.Equal(t, expectedRemoved, linesRemoved,
			"should fall back for JSON array")
		assert.Equal(t, expectedResult, result,
			"result should match TruncateOutput for JSON array")
	})

	t.Run("should fall back for empty string", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}

		result, linesRemoved := ui.TruncateBashOutput("", config)

		expectedResult, expectedRemoved := ui.TruncateOutput("", config)

		assert.Equal(t, expectedRemoved, linesRemoved,
			"should fall back for empty string")
		assert.Equal(t, expectedResult, result,
			"result should match TruncateOutput for empty string")
	})

	t.Run("should fall back for JSON with only stdout field", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		partialJSON := `{"stdout": "some output"}`

		result, linesRemoved := ui.TruncateBashOutput(partialJSON, config)

		expectedResult, expectedRemoved := ui.TruncateOutput(partialJSON, config)

		assert.Equal(t, expectedRemoved, linesRemoved,
			"should fall back for partial bash JSON")
		assert.Equal(t, expectedResult, result,
			"result should match TruncateOutput for partial bash JSON")
	})
}

// =============================================================================
// Test: TruncateBashOutput - Table-Driven Tests
// =============================================================================

func TestTruncateBashOutput_TableDriven(t *testing.T) {
	tests := []struct {
		name                string
		stdoutLines         int
		stderrLines         int
		exitCode            int
		headLines           int
		tailLines           int
		enabled             bool
		wantTotalRemoved    int
		wantStdoutTruncated bool
		wantStderrTruncated bool
	}{
		{
			name:                "only stdout truncated",
			stdoutLines:         50,
			stderrLines:         10,
			exitCode:            0,
			headLines:           20,
			tailLines:           10,
			enabled:             true,
			wantTotalRemoved:    20,
			wantStdoutTruncated: true,
			wantStderrTruncated: false,
		},
		{
			name:                "only stderr truncated",
			stdoutLines:         10,
			stderrLines:         50,
			exitCode:            1,
			headLines:           20,
			tailLines:           10,
			enabled:             true,
			wantTotalRemoved:    20,
			wantStdoutTruncated: false,
			wantStderrTruncated: true,
		},
		{
			name:                "both truncated",
			stdoutLines:         50,
			stderrLines:         50,
			exitCode:            0,
			headLines:           20,
			tailLines:           10,
			enabled:             true,
			wantTotalRemoved:    40,
			wantStdoutTruncated: true,
			wantStderrTruncated: true,
		},
		{
			name:                "neither truncated - below threshold",
			stdoutLines:         15,
			stderrLines:         15,
			exitCode:            0,
			headLines:           20,
			tailLines:           10,
			enabled:             true,
			wantTotalRemoved:    0,
			wantStdoutTruncated: false,
			wantStderrTruncated: false,
		},
		{
			name:                "truncation disabled",
			stdoutLines:         100,
			stderrLines:         100,
			exitCode:            0,
			headLines:           20,
			tailLines:           10,
			enabled:             false,
			wantTotalRemoved:    0,
			wantStdoutTruncated: false,
			wantStderrTruncated: false,
		},
		{
			name:                "custom small threshold",
			stdoutLines:         20,
			stderrLines:         20,
			exitCode:            127,
			headLines:           5,
			tailLines:           5,
			enabled:             true,
			wantTotalRemoved:    20,
			wantStdoutTruncated: true,
			wantStderrTruncated: true,
		},
		{
			name:                "empty output",
			stdoutLines:         0,
			stderrLines:         0,
			exitCode:            0,
			headLines:           20,
			tailLines:           10,
			enabled:             true,
			wantTotalRemoved:    0,
			wantStdoutTruncated: false,
			wantStderrTruncated: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ui.TruncationConfig{
				HeadLines: tt.headLines,
				TailLines: tt.tailLines,
				Enabled:   tt.enabled,
			}

			var stdout, stderr string
			if tt.stdoutLines > 0 {
				stdout = generateLines(tt.stdoutLines)
			}
			if tt.stderrLines > 0 {
				stderr = generateLines(tt.stderrLines)
			}

			input := makeBashJSON(stdout, stderr, tt.exitCode)

			result, linesRemoved := ui.TruncateBashOutput(input, config)

			assert.Equal(t, tt.wantTotalRemoved, linesRemoved,
				"total lines removed mismatch")

			parsed, err := parseBashJSON(result)
			require.NoError(t, err, "result should be valid JSON")

			// Verify exit code is preserved
			assert.Equal(t, tt.exitCode, parsed.ExitCode,
				"exit_code should be preserved")

			// Verify truncation indicators
			if tt.wantStdoutTruncated {
				assert.Contains(t, parsed.Stdout, "[...",
					"stdout should contain truncation indicator")
			} else {
				assert.NotContains(t, parsed.Stdout, "[...",
					"stdout should not contain truncation indicator")
			}

			if tt.wantStderrTruncated {
				assert.Contains(t, parsed.Stderr, "[...",
					"stderr should contain truncation indicator")
			} else {
				assert.NotContains(t, parsed.Stderr, "[...",
					"stderr should not contain truncation indicator")
			}
		})
	}
}

// =============================================================================
// Test: TruncateBashOutput - Edge Cases
// =============================================================================

func TestTruncateBashOutput_EdgeCases(t *testing.T) {
	t.Run("should handle stdout with special characters in JSON", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 2,
			TailLines: 2,
			Enabled:   true,
		}
		// Content with special JSON characters that need escaping
		lines := []string{
			`Line with "quotes"`,
			`Line with \backslash`,
			`Line with newline\n`,
			`Line with tab\t`,
			`Line with unicode: \u4e2d\u6587`,
			`Final line`,
		}
		stdout := strings.Join(lines, "\n")
		input := makeBashJSON(stdout, "", 0)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		assert.Equal(t, 2, linesRemoved,
			"should truncate correctly with special characters")

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON after truncation")

		assert.Contains(t, parsed.Stdout, `Line with "quotes"`,
			"should preserve special characters in head")
	})

	t.Run("should handle stderr with ANSI color codes", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 2,
			TailLines: 2,
			Enabled:   true,
		}
		lines := []string{
			"\x1b[31mError: something went wrong\x1b[0m",
			"\x1b[33mWarning: be careful\x1b[0m",
			"\x1b[31mError: another error\x1b[0m",
			"\x1b[33mWarning: again\x1b[0m",
			"\x1b[31mError: final error\x1b[0m",
			"\x1b[0mDone\x1b[0m",
		}
		stderr := strings.Join(lines, "\n")
		input := makeBashJSON("", stderr, 1)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		assert.Equal(t, 2, linesRemoved,
			"should truncate correctly with ANSI codes")

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Contains(t, parsed.Stderr, "\x1b[31m",
			"should preserve ANSI codes")
	})

	t.Run("should handle very large stdout and stderr", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		largeStdout := generateLines(5000)
		largeStderr := generateLines(5000)
		input := makeBashJSON(largeStdout, largeStderr, 0)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		// 5000 - 30 = 4970 per field, total = 9940
		assert.Equal(t, 9940, linesRemoved,
			"should handle very large output")

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Contains(t, parsed.Stdout, "[... 4970 lines truncated ...]",
			"stdout should show correct truncation count")
		assert.Contains(t, parsed.Stderr, "[... 4970 lines truncated ...]",
			"stderr should show correct truncation count")
	})

	t.Run("should handle Windows line endings in stdout", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 2,
			TailLines: 2,
			Enabled:   true,
		}
		lines := []string{"Line 1", "Line 2", "Line 3", "Line 4", "Line 5", "Line 6"}
		stdout := strings.Join(lines, "\r\n") // Windows line endings
		input := makeBashJSON(stdout, "", 0)

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		assert.Equal(t, 2, linesRemoved,
			"should handle CRLF line endings in stdout")

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Contains(t, parsed.Stdout, "Line 1",
			"should preserve first line")
		assert.Contains(t, parsed.Stdout, "Line 6",
			"should preserve last line")
	})

	t.Run("should handle null-like values in JSON", func(t *testing.T) {
		config := ui.TruncationConfig{
			HeadLines: 20,
			TailLines: 10,
			Enabled:   true,
		}
		// Manually create JSON with empty strings (null-like)
		input := `{"stdout":"","stderr":"","exit_code":0}`

		result, linesRemoved := ui.TruncateBashOutput(input, config)

		assert.Equal(t, 0, linesRemoved,
			"should handle empty string fields")

		parsed, err := parseBashJSON(result)
		require.NoError(t, err, "result should be valid JSON")

		assert.Empty(t, parsed.Stdout, "stdout should be empty")
		assert.Empty(t, parsed.Stderr, "stderr should be empty")
		assert.Equal(t, 0, parsed.ExitCode, "exit_code should be 0")
	})
}

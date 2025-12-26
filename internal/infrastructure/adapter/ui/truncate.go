// Package ui provides user interface components and utilities for the CLI adapter.
package ui

import (
	"encoding/json"
	"fmt"
	"strings"
)

// truncationIndicatorFormat is the format string for the truncation indicator line.
// It shows how many lines were omitted from the middle of the output.
const truncationIndicatorFormat = "[... %d lines truncated ...]"

// TruncationConfig controls how output truncation behaves when displaying
// large command outputs or log files. It allows showing the beginning and
// end of output while omitting the middle section.
type TruncationConfig struct {
	// HeadLines is the number of lines to preserve from the beginning of the output.
	HeadLines int
	// TailLines is the number of lines to preserve from the end of the output.
	TailLines int
	// Enabled controls whether truncation is active. When false, output is returned unchanged.
	Enabled bool
}

// DefaultTruncationConfig returns the default truncation configuration with
// sensible defaults: 20 head lines, 10 tail lines, and truncation enabled.
func DefaultTruncationConfig() TruncationConfig {
	return TruncationConfig{
		HeadLines: 20,
		TailLines: 10,
		Enabled:   true,
	}
}

// detectLineSeparator determines the line separator used in the output.
// It returns "\r\n" for Windows-style (CRLF) or "\n" for Unix-style (LF).
func detectLineSeparator(output string) string {
	if strings.Contains(output, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

// TruncateOutput truncates the output string according to the provided configuration.
// It preserves the first HeadLines lines, inserts a truncation indicator showing
// how many lines were omitted, and preserves the last TailLines lines.
//
// The function handles both Unix (LF) and Windows (CRLF) line endings, preserving
// the original format. Trailing newlines in the input are also preserved.
//
// Returns:
//   - The (possibly truncated) output string
//   - The number of lines that were removed (0 if no truncation occurred)
func TruncateOutput(output string, config TruncationConfig) (string, int) {
	// Early returns for cases that don't require truncation
	if !config.Enabled || output == "" {
		return output, 0
	}

	// Detect and preserve the original line separator format
	separator := detectLineSeparator(output)
	hasTrailingSeparator := strings.HasSuffix(output, separator)

	// Split into lines and normalize by removing empty trailing element
	lines := strings.Split(output, separator)
	lines = removeTrailingEmptyLine(lines, hasTrailingSeparator)

	// Calculate truncation threshold and check if truncation is needed
	totalLines := len(lines)
	threshold := config.HeadLines + config.TailLines
	if totalLines <= threshold {
		return output, 0
	}

	// Perform truncation: extract head and tail portions
	linesRemoved := totalLines - threshold
	headPortion := lines[:config.HeadLines]
	tailPortion := lines[totalLines-config.TailLines:]

	// Build the truncated result with indicator
	truncatedResult := buildTruncatedOutput(headPortion, tailPortion, linesRemoved, separator)

	// Restore trailing separator if the original had one
	if hasTrailingSeparator {
		truncatedResult += separator
	}

	return truncatedResult, linesRemoved
}

// removeTrailingEmptyLine removes the empty string element that results from
// splitting a string with a trailing separator. This ensures accurate line counting.
func removeTrailingEmptyLine(lines []string, hasTrailingSeparator bool) []string {
	if hasTrailingSeparator && len(lines) > 0 && lines[len(lines)-1] == "" {
		return lines[:len(lines)-1]
	}
	return lines
}

// buildTruncatedOutput assembles the final truncated output from head lines,
// a truncation indicator, and tail lines.
func buildTruncatedOutput(head, tail []string, linesRemoved int, separator string) string {
	indicator := fmt.Sprintf(truncationIndicatorFormat, linesRemoved)

	// Pre-allocate result slice: head + indicator + tail
	result := make([]string, 0, len(head)+1+len(tail))
	result = append(result, head...)
	result = append(result, indicator)
	result = append(result, tail...)

	return strings.Join(result, separator)
}

// bashOutput represents the JSON structure returned by the bash tool.
type bashOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// TruncateBashOutput handles truncation of bash tool JSON output.
// It parses the JSON structure with stdout, stderr, and exit_code fields,
// truncates stdout and stderr independently using TruncateOutput,
// then reconstructs the JSON.
//
// If the input is not valid bash JSON format, it falls back to regular
// TruncateOutput on the entire input.
//
// Returns:
//   - The (possibly truncated) output string
//   - The total number of lines removed from both stdout and stderr
func TruncateBashOutput(output string, config TruncationConfig) (string, int) {
	// Try to parse as bash JSON
	var bash bashOutput
	if err := json.Unmarshal([]byte(output), &bash); err != nil {
		// Not valid bash JSON, fall back to plain truncation
		return TruncateOutput(output, config)
	}

	// Truncate stdout and stderr independently
	truncatedStdout, stdoutRemoved := TruncateOutput(bash.Stdout, config)
	truncatedStderr, stderrRemoved := TruncateOutput(bash.Stderr, config)

	totalRemoved := stdoutRemoved + stderrRemoved

	// If nothing was truncated, return original output unchanged
	if totalRemoved == 0 {
		return output, 0
	}

	// Reconstruct JSON with truncated fields
	bash.Stdout = truncatedStdout
	bash.Stderr = truncatedStderr

	result, err := json.MarshalIndent(bash, "", "  ")
	if err != nil {
		// Should not happen, but fall back if it does
		return TruncateOutput(output, config)
	}

	return string(result), totalRemoved
}

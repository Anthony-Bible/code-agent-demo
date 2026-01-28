// Package safety provides shared safety-related functionality for command execution.
package safety

import (
	"errors"
	"strings"
)

// Sentinel errors for command validation.
// These errors can be checked using errors.Is() for programmatic error handling.
var (
	// ErrUnbalancedQuotes indicates unbalanced quotes in a command.
	ErrUnbalancedQuotes = errors.New("unbalanced quotes in command")

	// ErrUnbalancedParens indicates unbalanced $() in a command.
	ErrUnbalancedParens = errors.New("unbalanced $() in command")

	// ErrNestedQuantifiers indicates a pattern contains nested quantifiers which may cause ReDoS.
	ErrNestedQuantifiers = errors.New("pattern contains nested quantifiers which may cause ReDoS")

	// ErrPatternTooLong indicates a pattern exceeds the maximum allowed length.
	ErrPatternTooLong = errors.New("pattern too long")

	// ErrPatternRequired indicates a pattern is required but was not provided.
	ErrPatternRequired = errors.New("pattern is required")

	// ErrWhitelistRequired indicates whitelist mode was requested but no whitelist was provided.
	ErrWhitelistRequired = errors.New("whitelist required for whitelist mode")

	// ErrLargeRepetition indicates a pattern contains large repetition which may cause ReDoS.
	ErrLargeRepetition = errors.New("pattern contains large repetition which may cause ReDoS")

	// ErrAlternationQuantifier indicates a pattern contains alternation with outer quantifier which may cause ReDoS.
	ErrAlternationQuantifier = errors.New("pattern contains alternation with quantifier which may cause ReDoS")
)

// Shared regex patterns for dangerous command detection.
// These constants are used by both whitelist and blacklist patterns.
const (
	// GitDeleteFlags matches git branch/tag delete flags.
	GitDeleteFlags = `(?i)(-d\s|-D\s|--delete\s)`

	// FindDangerousFlags matches find command flags that can execute or delete.
	FindDangerousFlags = `(?i)(-exec\s|-execdir\s|-delete(\s|$)|-ok\s|-okdir\s)`

	// AwkDangerousPatterns matches awk constructs that can execute commands or write files.
	// Uses print\s*> to avoid matching comparison operators like NR > 1.
	AwkDangerousPatterns = `(?i)(system\s*\(|getline|print\s*>\s|print\s*>>\s|print\s*\|\s)`

	// SedDangerousPatterns matches sed flags that modify files or execute commands.
	// Matches -i flag and /e, /w flags at various positions including before quotes.
	SedDangerousPatterns = `(?i)(-i\s|-i$|-i['"]|/e\s|/e$|/e['"]|/e[gp]*['"\s]|/e[gp]*$|/w\s)`
)

// Quote character sets for command parsing.
const (
	// quoteCharsAll includes all quote characters (for $() extraction).
	// Inside backticks, we don't start new $() extraction, so backticks are quote chars.
	quoteCharsAll = "'\"`"

	// quoteCharsNoBacktick excludes backticks (for backtick extraction).
	// When extracting backticks, we don't treat them as quote toggling chars in the outer loop.
	quoteCharsNoBacktick = "'\""
)

// Validation bounds for investigation metrics and command processing.
const (
	// ConfidenceMin is the minimum allowed confidence value.
	ConfidenceMin = 0.0
	// ConfidenceMax is the maximum allowed confidence value.
	ConfidenceMax = 1.0
	// ProgressMin is the minimum allowed progress value.
	ProgressMin = 0
	// ProgressMax is the maximum allowed progress value.
	ProgressMax = 100
	// MaxCommandLength is the maximum length of a command that will be processed.
	// Commands exceeding this length are considered dangerous to prevent ReDoS attacks.
	MaxCommandLength = 10000

	// MaxRecursionDepth is the maximum nesting depth for command substitutions.
	// Prevents stack overflow from deeply nested $() or backtick commands.
	MaxRecursionDepth = 20

	// MaxTotalSegments is the maximum total number of command segments processed
	// across all recursion levels. This prevents DoS attacks using commands with
	// many segments at each nesting level (e.g., 100 segments × 20 levels = exponential work).
	// Set to a reasonable limit that allows legitimate complex commands while
	// preventing abuse.
	MaxTotalSegments = 500
)

// Error format templates for command validation messages.
//
// Whitelist errors use a single format parameter (command only):
//   - ErrFmtWhitelistBlocked, ErrFmtWhitelistDenied
//
// Dangerous command errors use two format parameters (reason, command):
//   - ErrFmtDangerousBlocked, ErrFmtDangerousDenied
//
// Simple command errors use a single format parameter (command only):
//   - ErrFmtCommandDenied
const (
	// ErrFmtWhitelistBlocked formats: command.
	ErrFmtWhitelistBlocked = "whitelist: command blocked: %s"

	// ErrFmtWhitelistDenied formats: command.
	ErrFmtWhitelistDenied = "whitelist: user denied: %s"

	// ErrFmtDangerousBlocked formats: reason, command.
	ErrFmtDangerousBlocked = "dangerous command blocked: %s (%s)"

	// ErrFmtDangerousDenied formats: reason, command.
	ErrFmtDangerousDenied = "dangerous command denied by user: %s (%s)"

	// ErrFmtCommandDenied formats: command.
	ErrFmtCommandDenied = "command denied by user: %s"

	// ErrMsgLLMFailedToDetect is appended when pattern detection catches what LLM missed.
	ErrMsgLLMFailedToDetect = "(WARNING: LLM failed to identify this as dangerous)"

	// ErrMsgMarkedDangerousByAI is the reason when only LLM flagged command as dangerous.
	ErrMsgMarkedDangerousByAI = "marked dangerous by AI"
)

// commandExtractAction is called for each non-escaped, non-quote character during parsing.
// Returns the number of positions to skip (0 = don't skip, >0 = skip that many chars).
type commandExtractAction func(cmd string, pos int, state quoteState, results *[]string) int

// parseCommandWithQuoteAwareness iterates through cmd, handling escapes and quotes,
// and calls action for each "significant" character.
// quoteChars defines which characters toggle quote state (e.g., `'"` + "`" or just `'"`).
func parseCommandWithQuoteAwareness(cmd string, quoteChars string, action commandExtractAction) []string {
	var results []string
	state := quoteNone
	escaped := false

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]

		if escaped {
			escaped = false
			continue
		}
		if isEscapeChar(c, state) {
			escaped = true
			continue
		}

		if strings.ContainsRune(quoteChars, rune(c)) {
			state = updateQuoteState(state, c)
			continue
		}

		if skip := action(cmd, i, state, &results); skip > 0 {
			i += skip - 1 // -1 because loop will increment
		}
	}
	return results
}

// extractDollarParenActionWithDepth handles $() extraction at a given position with depth tracking.
func extractDollarParenActionWithDepth(cmd string, pos int, state quoteState, results *[]string, depth int) int {
	if !isDollarParenStart(cmd, pos, state) {
		return 0
	}

	// Find the matching closing paren
	content, endPos := extractSingleDollarParen(cmd, pos+2, state)
	if content != "" {
		*results = append(*results, content)
		// Recursively extract from nested content (with depth limit)
		if depth < MaxRecursionDepth {
			nested := extractDollarParenCommandsWithDepth(content, depth+1)
			*results = append(*results, nested...)
		}
	}
	if endPos > pos {
		return endPos - pos + 1 // Skip past the $() we just processed
	}
	return 0
}

// extractDollarParenCommandsWithDepth extracts $() commands with depth tracking.
func extractDollarParenCommandsWithDepth(cmd string, depth int) []string {
	if depth >= MaxRecursionDepth {
		return nil // Stop recursion at max depth
	}
	action := func(cmd string, pos int, state quoteState, results *[]string) int {
		return extractDollarParenActionWithDepth(cmd, pos, state, results, depth)
	}
	return parseCommandWithQuoteAwareness(cmd, quoteCharsAll, action)
}

// ExtractDollarParenCommands extracts all commands from $() substitutions in a string.
// It handles nested $() and respects quoting (single quotes prevent extraction).
// Returns a slice of command strings found inside $() substitutions.
// This is used by both whitelist and blacklist validation to check commands inside $().
// Recursion depth is limited to MaxRecursionDepth to prevent stack overflow.
func ExtractDollarParenCommands(cmd string) []string {
	return extractDollarParenCommandsWithDepth(cmd, 0)
}

// extractImmediateDollarParenAction handles $() extraction without recursion.
// Only extracts the immediate (top-level) $() substitutions, not nested ones.
func extractImmediateDollarParenAction(cmd string, pos int, state quoteState, results *[]string) int {
	if !isDollarParenStart(cmd, pos, state) {
		return 0
	}

	// Find the matching closing paren
	content, endPos := extractSingleDollarParen(cmd, pos+2, state)
	if content != "" {
		*results = append(*results, content)
		// Do NOT recursively extract from nested content
	}
	if endPos > pos {
		return endPos - pos + 1 // Skip past the $() we just processed
	}
	return 0
}

// ExtractImmediateDollarParenCommands extracts only immediate (top-level) $() substitutions.
// Unlike ExtractDollarParenCommands, this does NOT recursively extract from nested content.
// Use this when the caller will handle recursive validation separately.
func ExtractImmediateDollarParenCommands(cmd string) []string {
	return parseCommandWithQuoteAwareness(cmd, quoteCharsAll, extractImmediateDollarParenAction)
}

// extractSingleDollarParen extracts the content of a single $() starting at pos.
// pos should point to the first character after "$(". Returns the content and the
// position of the closing paren (or -1 if unbalanced).
func extractSingleDollarParen(cmd string, pos int, outerState quoteState) (string, int) {
	depth := 1
	state := outerState
	escaped := false
	start := pos

	for i := pos; i < len(cmd); i++ {
		c := cmd[i]

		// Handle escape sequences
		if escaped {
			escaped = false
			continue
		}
		if isEscapeChar(c, state) {
			escaped = true
			continue
		}

		// Handle quote state changes
		if isQuoteChar(c) {
			state = updateQuoteState(state, c)
			continue
		}

		// Look for nested $(
		if isDollarParenStart(cmd, i, state) {
			depth++
			i++ // Skip past the $
			continue
		}

		// Look for closing )
		if c == ')' && (state == quoteNone || state == quoteDouble) {
			depth--
			if depth == 0 {
				return strings.TrimSpace(cmd[start:i]), i
			}
		}
	}

	// Unbalanced - return empty
	return "", -1
}

// extractBacktickActionWithDepth handles backtick extraction at a given position with depth tracking.
func extractBacktickActionWithDepth(cmd string, pos int, state quoteState, results *[]string, depth int) int {
	c := cmd[pos]
	// Look for backtick start - only outside single quotes
	if c != '`' || state == quoteSingle {
		return 0
	}

	content, endPos := extractSingleBacktick(cmd, pos+1)
	if content != "" {
		*results = append(*results, content)
		// Recursively extract $() from inside backticks (with depth limit)
		if depth < MaxRecursionDepth {
			nested := extractDollarParenCommandsWithDepth(content, depth+1)
			*results = append(*results, nested...)
		}
	}
	if endPos > pos {
		return endPos - pos + 1 // Skip past the backtick we just processed
	}
	return 0
}

// extractBacktickCommandsWithDepth extracts backtick commands with depth tracking.
func extractBacktickCommandsWithDepth(cmd string, depth int) []string {
	if depth >= MaxRecursionDepth {
		return nil // Stop recursion at max depth
	}
	action := func(cmd string, pos int, state quoteState, results *[]string) int {
		return extractBacktickActionWithDepth(cmd, pos, state, results, depth)
	}
	return parseCommandWithQuoteAwareness(cmd, quoteCharsNoBacktick, action)
}

// ExtractBacktickCommands extracts commands from backtick substitutions.
// Backticks don't nest like $() - inner backticks must be escaped.
// Only extracts when NOT in single quotes (where backticks are literal).
// Recursion depth is limited to MaxRecursionDepth to prevent stack overflow.
func ExtractBacktickCommands(cmd string) []string {
	return extractBacktickCommandsWithDepth(cmd, 0)
}

// extractImmediateBacktickAction handles backtick extraction without recursion.
// Only extracts the immediate backtick substitutions, not nested $() inside them.
func extractImmediateBacktickAction(cmd string, pos int, state quoteState, results *[]string) int {
	c := cmd[pos]
	// Look for backtick start - only outside single quotes
	if c != '`' || state == quoteSingle {
		return 0
	}

	content, endPos := extractSingleBacktick(cmd, pos+1)
	if content != "" {
		*results = append(*results, content)
		// Do NOT recursively extract $() from inside backticks
	}
	if endPos > pos {
		return endPos - pos + 1 // Skip past the backtick we just processed
	}
	return 0
}

// ExtractImmediateBacktickCommands extracts only immediate backtick substitutions.
// Unlike ExtractBacktickCommands, this does NOT recursively extract $() from content.
// Use this when the caller will handle recursive validation separately.
func ExtractImmediateBacktickCommands(cmd string) []string {
	return parseCommandWithQuoteAwareness(cmd, quoteCharsNoBacktick, extractImmediateBacktickAction)
}

// extractSingleBacktick extracts content between backticks.
// Handles escaped backticks inside (e.g., `echo \`pwd\“).
func extractSingleBacktick(cmd string, pos int) (string, int) {
	escaped := false
	start := pos
	var content strings.Builder

	for i := pos; i < len(cmd); i++ {
		c := cmd[i]

		if escaped {
			// Write the escaped character (without the backslash for backticks)
			if c == '`' {
				content.WriteByte(c)
			} else {
				// Keep backslash for other escaped chars
				content.WriteByte('\\')
				content.WriteByte(c)
			}
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}

		// Found closing backtick
		if c == '`' {
			// Return the processed content (with escaped backticks resolved)
			result := strings.TrimSpace(content.String())
			if result == "" {
				// Also try the raw extraction for simple cases
				result = strings.TrimSpace(cmd[start:i])
			}
			return result, i
		}

		content.WriteByte(c)
	}

	// Unbalanced - return empty (security default)
	return "", -1
}

// Package safety provides shared safety-related functionality for command execution.
// This file implements a whitelist-based approach to command validation.
package safety

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// CommandAllowChecker provides a simple allow/deny decision for commands.
// This interface enables mock implementations for testing and decouples
// validation logic from the concrete CommandWhitelist implementation.
//
// *CommandWhitelist implements this interface.
type CommandAllowChecker interface {
	// IsAllowed checks if a single command is allowed.
	// Returns (true, description) if allowed, (false, "") if not allowed.
	IsAllowed(cmd string) (bool, string)

	// IsAllowedWithPipes checks if a piped/chained command is allowed.
	// Splits command on |, &&, ||, ; and checks each segment.
	// Returns (true, description) if all parts allowed, (false, "") if any part is not allowed.
	IsAllowedWithPipes(cmd string) (bool, string)
}

// Compile-time check that *CommandWhitelist implements CommandAllowChecker.
var _ CommandAllowChecker = (*CommandWhitelist)(nil)

// CommandValidationMode determines how commands are validated.
type CommandValidationMode string

const (
	// ModeBlacklist uses the default blacklist approach (blocking dangerous commands).
	ModeBlacklist CommandValidationMode = "blacklist"
	// ModeWhitelist only allows explicitly whitelisted commands.
	ModeWhitelist CommandValidationMode = "whitelist"
)

// WhitelistPattern represents a safe command pattern.
type WhitelistPattern struct {
	Pattern        *regexp.Regexp
	Description    string
	ExcludePattern *regexp.Regexp // Optional: if matches, command is NOT allowed even if Pattern matches
}

// WhitelistPatternJSON represents a whitelist pattern in JSON format for CLI input.
// This struct is used to unmarshal JSON configuration from environment variables.
type WhitelistPatternJSON struct {
	Pattern         string `json:"pattern"`
	ExcludePattern  string `json:"exclude_pattern,omitempty"`
	Description     string `json:"description,omitempty"`
	CaseInsensitive bool   `json:"case_insensitive,omitempty"`
}

// CommandWhitelist manages allowed command patterns.
type CommandWhitelist struct {
	patterns []WhitelistPattern
}

// cmdBoundary is the regex suffix that matches end of command or whitespace.
// Used by all whitelist patterns to ensure proper command boundary matching.
const cmdBoundary = `(\s|$)`

// MustSimple creates a WhitelistPattern for a simple command.
// Uses regexp.QuoteMeta to escape cmd, ensuring safe pattern compilation.
// Panics on invalid pattern (should never happen with QuoteMeta).
func MustSimple(cmd, desc string) WhitelistPattern {
	pattern := `^` + regexp.QuoteMeta(cmd) + cmdBoundary
	return WhitelistPattern{
		Pattern:     regexp.MustCompile(pattern),
		Description: desc,
	}
}

// MustSubcmd creates a pattern for a command with subcommand (e.g., "git status").
// Both cmd and subcmd are escaped with QuoteMeta.
func MustSubcmd(cmd, subcmd, desc string) WhitelistPattern {
	pattern := `^` + regexp.QuoteMeta(cmd) + `\s+` + regexp.QuoteMeta(subcmd) + cmdBoundary
	return WhitelistPattern{
		Pattern:     regexp.MustCompile(pattern),
		Description: desc,
	}
}

// MustPattern creates a pattern from a custom regex string.
// Use for patterns requiring special regex (flags, specific syntax).
// Caller must ensure pattern is valid.
func MustPattern(pattern, desc string) WhitelistPattern {
	return WhitelistPattern{
		Pattern:     regexp.MustCompile(pattern),
		Description: desc,
	}
}

// MustExcluding creates a simple command pattern with an exclusion regex.
func MustExcluding(cmd, desc, exclude string) WhitelistPattern {
	pattern := `^` + regexp.QuoteMeta(cmd) + cmdBoundary
	return WhitelistPattern{
		Pattern:        regexp.MustCompile(pattern),
		Description:    desc,
		ExcludePattern: regexp.MustCompile(exclude),
	}
}

// MustSubcmdExcluding creates a subcommand pattern with an exclusion regex.
func MustSubcmdExcluding(cmd, subcmd, desc, exclude string) WhitelistPattern {
	pattern := `^` + regexp.QuoteMeta(cmd) + `\s+` + regexp.QuoteMeta(subcmd) + cmdBoundary
	return WhitelistPattern{
		Pattern:        regexp.MustCompile(pattern),
		Description:    desc,
		ExcludePattern: regexp.MustCompile(exclude),
	}
}

// NewCommandWhitelist creates a new CommandWhitelist with the provided patterns.
func NewCommandWhitelist(patterns []WhitelistPattern) *CommandWhitelist {
	return &CommandWhitelist{
		patterns: patterns,
	}
}

// IsAllowed checks if a command matches any whitelisted pattern.
// Returns (true, description) if allowed, (false, "") if not allowed.
// If a pattern has an ExcludePattern set, the command must not match the exclude pattern.
//
// Note: This function does not perform length validation. Callers should use
// IsAllowedWithPipes() which validates length before splitting and checking segments.
func (w *CommandWhitelist) IsAllowed(cmd string) (bool, string) {
	for _, wp := range w.patterns {
		if wp.Pattern.MatchString(cmd) {
			// Check exclusion pattern (simulates negative lookahead)
			if wp.ExcludePattern != nil && wp.ExcludePattern.MatchString(cmd) {
				continue // Excluded, try next pattern
			}
			return true, wp.Description
		}
	}
	return false, ""
}

// isAllowedWithPipesInternal is the depth and segment-tracking internal version.
// Tracks recursion depth to prevent stack overflow from deeply nested substitutions.
// Tracks total segments processed to prevent DoS from commands with many segments at each level.
func (w *CommandWhitelist) isAllowedWithPipesInternal(cmd string, depth int, totalSegments *int) (bool, string) {
	// Apply same length limit as dangerous command detection for ReDoS protection
	if len(cmd) > MaxCommandLength {
		return false, ""
	}

	// Check recursion depth
	if depth >= MaxRecursionDepth {
		return false, "" // Reject excessive nesting
	}

	// Split on pipe operators and command separators (quote-aware)
	segments, err := splitCommandSegmentsQuoteAware(cmd)
	if err != nil {
		// Unbalanced quotes are blocked for security
		return false, ""
	}

	if len(segments) == 0 {
		return false, ""
	}

	// Check total segments limit to prevent DoS (e.g., 100 segments × 20 levels)
	*totalSegments += len(segments)
	if *totalSegments > MaxTotalSegments {
		return false, "" // Reject commands with excessive total complexity
	}

	var descriptions []string
	matchedCount := 0
	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}

		allowed, desc := w.IsAllowed(segment)
		if !allowed {
			return false, ""
		}
		matchedCount++
		if desc != "" {
			descriptions = append(descriptions, desc)
		}

		// Validate commands inside substitutions ($() and backticks) with depth and segment tracking
		if !w.validateSubstitutionsInternal(segment, depth, totalSegments) {
			return false, ""
		}
	}

	if matchedCount == 0 {
		return false, ""
	}

	return true, strings.Join(descriptions, " | ")
}

// IsAllowedWithPipes checks if a piped/chained command is allowed.
// Splits command on |, &&, ||, ; and checks each segment.
// All parts must be whitelisted for the command to be allowed.
// Also recursively validates commands inside $() and backtick substitutions.
// Recursion depth is limited to MaxRecursionDepth to prevent stack overflow.
// Total segments across all levels is limited to MaxTotalSegments to prevent DoS.
// Returns (true, description) if all parts allowed, (false, "") if any part is not allowed.
func (w *CommandWhitelist) IsAllowedWithPipes(cmd string) (bool, string) {
	totalSegments := 0
	return w.isAllowedWithPipesInternal(cmd, 0, &totalSegments)
}

// validateSubstitutionsInternal checks substitutions with depth and segment tracking.
// Returns true if all substituted commands are allowed, false otherwise.
// Uses immediate extraction (non-recursive) to avoid double-counting nested substitutions,
// since the recursive validation will handle deeper levels.
func (w *CommandWhitelist) validateSubstitutionsInternal(segment string, depth int, totalSegments *int) bool {
	if depth >= MaxRecursionDepth {
		return false // Reject commands with excessive nesting
	}

	// Use immediate extraction to only get top-level substitutions.
	// The recursive call to isAllowedWithPipesInternal will handle nested ones.
	subCommands := ExtractImmediateDollarParenCommands(segment)
	subCommands = append(subCommands, ExtractImmediateBacktickCommands(segment)...)

	for _, subCmd := range subCommands {
		if allowed, _ := w.isAllowedWithPipesInternal(subCmd, depth+1, totalSegments); !allowed {
			return false
		}
	}
	return true
}

// quoteState represents the current quoting context during command parsing.
type quoteState int

const (
	quoteNone   quoteState = 0
	quoteSingle quoteState = 1
	quoteDouble quoteState = 2
	quoteBack   quoteState = 3
)

// updateQuoteState returns the new quote state after encountering a quote character.
func updateQuoteState(current quoteState, char byte) quoteState {
	switch char {
	case '\'':
		switch current {
		case quoteNone:
			return quoteSingle
		case quoteSingle:
			return quoteNone
		case quoteDouble, quoteBack:
			// Single quote inside double quotes or backticks is literal
			return current
		}
	case '"':
		switch current {
		case quoteNone:
			return quoteDouble
		case quoteDouble:
			return quoteNone
		case quoteSingle, quoteBack:
			// Double quote inside single quotes or backticks is literal
			return current
		}
	case '`':
		switch current {
		case quoteNone:
			return quoteBack
		case quoteBack:
			return quoteNone
		case quoteSingle, quoteDouble:
			// Backtick inside single or double quotes is literal
			return current
		}
	}
	return current
}

// isOperatorAt checks if there's a shell operator at the given position.
// Returns the operator length (0 if none).
func isOperatorAt(cmd string, pos int) int {
	if pos+1 < len(cmd) {
		twoChar := cmd[pos : pos+2]
		if twoChar == "&&" || twoChar == "||" {
			return 2
		}
	}
	if cmd[pos] == '|' || cmd[pos] == ';' {
		return 1
	}
	return 0
}

// isQuoteChar returns true if the character is a shell quote character.
func isQuoteChar(c byte) bool {
	return c == '\'' || c == '"' || c == '`'
}

// appendSegment adds a non-empty trimmed segment to the slice.
func appendSegment(segments []string, current *strings.Builder) []string {
	if trimmed := strings.TrimSpace(current.String()); trimmed != "" {
		segments = append(segments, trimmed)
	}
	current.Reset()
	return segments
}

// isDollarParenStart checks if position i starts a $( command substitution.
func isDollarParenStart(cmd string, i int, state quoteState) bool {
	return cmd[i] == '$' && i+1 < len(cmd) && cmd[i+1] == '(' && state != quoteSingle
}

// isDollarParenEnd checks if position i ends a $() command substitution.
// Works in both unquoted context and inside double quotes (where $() is expanded).
func isDollarParenEnd(c byte, dollarParenDepth int, state quoteState) bool {
	return c == ')' && dollarParenDepth > 0 && (state == quoteNone || state == quoteDouble)
}

// isEscapeChar checks if backslash should start an escape sequence.
func isEscapeChar(c byte, state quoteState) bool {
	return c == '\\' && state != quoteSingle
}

// splitCommandSegmentsQuoteAware splits a command string on pipe and chain operators,
// respecting shell quoting rules (single quotes, double quotes, backticks, and $()).
// Handles: | (pipe), && (and), || (or), ; (sequential)
// Returns error for unbalanced quotes or $() (secure default: block malformed commands).
func splitCommandSegmentsQuoteAware(cmd string) ([]string, error) {
	var segments []string
	var current strings.Builder

	state := quoteNone
	escaped := false
	dollarParenDepth := 0 // Track nested $() command substitutions

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]

		// Handle escape sequences (only in double quotes or unquoted)
		if escaped {
			current.WriteByte(c)
			escaped = false
			continue
		}

		if isEscapeChar(c, state) {
			escaped = true
			current.WriteByte(c)
			continue
		}

		// Detect $( start - only outside single quotes
		if isDollarParenStart(cmd, i, state) {
			dollarParenDepth++
			current.WriteByte(c)
			continue
		}

		// Detect ) end for $() - only if we're inside $() and not in quotes
		if isDollarParenEnd(c, dollarParenDepth, state) {
			dollarParenDepth--
			current.WriteByte(c)
			continue
		}

		// Handle quote characters
		if isQuoteChar(c) {
			state = updateQuoteState(state, c)
			current.WriteByte(c)
			continue
		}

		// Check for operators only when not inside quotes AND not inside $()
		if state == quoteNone && dollarParenDepth == 0 {
			if opLen := isOperatorAt(cmd, i); opLen > 0 {
				segments = appendSegment(segments, &current)
				i += opLen - 1 // Skip operator (loop will increment by 1)
				continue
			}
		}

		current.WriteByte(c)
	}

	// Check for unbalanced quotes or $() (security: block malformed commands)
	if state != quoteNone || escaped {
		return nil, ErrUnbalancedQuotes
	}
	if dollarParenDepth != 0 {
		return nil, ErrUnbalancedParens
	}

	segments = appendSegment(segments, &current)
	return segments, nil
}

// DefaultWhitelistPatterns returns the default set of safe command patterns.
// These are read-only commands that don't modify the system.
func DefaultWhitelistPatterns() []WhitelistPattern {
	var patterns []WhitelistPattern
	patterns = append(patterns, fileReadPatterns()...)
	patterns = append(patterns, searchPatterns()...)
	patterns = append(patterns, textProcessingPatterns()...)
	patterns = append(patterns, gitReadPatterns()...)
	patterns = append(patterns, devToolPatterns()...)
	patterns = append(patterns, systemInfoPatterns()...)
	patterns = append(patterns, utilityPatterns()...)
	patterns = append(patterns, containerPatterns()...)
	return patterns
}

// fileReadPatterns returns patterns for read-only file operations.
func fileReadPatterns() []WhitelistPattern {
	return []WhitelistPattern{
		MustSimple("ls", "list directory contents"),
		MustSimple("cat", "display file contents"),
		MustSimple("head", "display first lines of file"),
		MustSimple("tail", "display last lines of file"),
		MustSimple("less", "page through file"),
		MustSimple("more", "page through file"),
		MustSimple("wc", "word/line/byte count"),
		MustSimple("file", "determine file type"),
		MustSimple("stat", "display file status"),
		MustSimple("readlink", "read symbolic link"),
		MustSimple("realpath", "resolve path"),
		MustSimple("basename", "strip directory from path"),
		MustSimple("dirname", "strip last path component"),
	}
}

// searchPatterns returns patterns for search and find commands.
func searchPatterns() []WhitelistPattern {
	return []WhitelistPattern{
		MustSimple("grep", "search file contents"),
		MustSimple("egrep", "extended grep"),
		MustSimple("fgrep", "fixed string grep"),
		MustSimple("rg", "ripgrep search"),
		MustSimple("ag", "silver searcher"),
		MustExcluding("find", "find files (read-only)", FindDangerousFlags),
		MustSimple("fd", "fd file finder"),
		MustSimple("locate", "locate files"),
		MustSimple("which", "locate command"),
		MustSimple("whereis", "locate binary"),
		MustSimple("type", "describe command type"),
	}
}

// textProcessingPatterns returns patterns for text processing commands.
func textProcessingPatterns() []WhitelistPattern {
	return []WhitelistPattern{
		MustExcluding("awk", "awk text processing (read-only)", AwkDangerousPatterns),
		MustExcluding("sed", "sed text processing (read-only)", SedDangerousPatterns),
		MustSimple("sort", "sort lines"),
		MustSimple("uniq", "filter unique lines"),
		MustSimple("cut", "extract columns"),
		MustSimple("tr", "translate characters"),
		MustSimple("diff", "compare files"),
		MustSimple("comm", "compare sorted files"),
		MustSimple("cmp", "byte-by-byte compare"),
		MustSimple("md5sum", "compute MD5 checksum"),
		MustSimple("sha256sum", "compute SHA256 checksum"),
		MustSimple("sha1sum", "compute SHA1 checksum"),
		MustSimple("jq", "JSON processor"),
		MustSimple("yq", "YAML processor"),
	}
}

// gitReadPatterns returns patterns for git read-only operations.
func gitReadPatterns() []WhitelistPattern {
	return []WhitelistPattern{
		MustSubcmd("git", "status", "git status"),
		MustSubcmd("git", "log", "git log"),
		MustSubcmd("git", "diff", "git diff"),
		MustSubcmd("git", "show", "git show"),
		MustSubcmdExcluding("git", "branch", "git branch list (read-only)", GitDeleteFlags),
		MustSubcmdExcluding("git", "tag", "git tag list (read-only)", GitDeleteFlags),
		MustSubcmd("git", "remote", "git remote"),
		MustSubcmd("git", "rev-parse", "git rev-parse"),
		MustSubcmd("git", "describe", "git describe"),
		MustSubcmd("git", "ls-files", "git ls-files"),
		MustSubcmd("git", "ls-tree", "git ls-tree"),
		MustSubcmd("git", "cat-file", "git cat-file"),
		MustSubcmd("git", "blame", "git blame"),
		MustSubcmd("git", "shortlog", "git shortlog"),
		MustSubcmd("git", "reflog", "git reflog"),
		MustPattern(`^git\s+stash\s+list`+cmdBoundary, "git stash list"),
		MustPattern(`^git\s+config\s+--get`+cmdBoundary, "git config get"),
		MustPattern(`^git\s+config\s+--list`+cmdBoundary, "git config list"),
	}
}

// devToolPatterns returns patterns for development tool read operations.
func devToolPatterns() []WhitelistPattern {
	return []WhitelistPattern{
		// Go
		MustSubcmd("go", "version", "go version"),
		MustSubcmd("go", "env", "go environment"),
		MustSubcmd("go", "list", "go list packages"),
		MustSubcmd("go", "doc", "go documentation"),
		MustPattern(`^go\s+mod\s+graph`+cmdBoundary, "go mod graph"),
		MustPattern(`^go\s+mod\s+why`+cmdBoundary, "go mod why"),
		MustSubcmd("go", "vet", "go vet"),
		// Node/npm
		MustSubcmd("node", "--version", "node version"),
		MustSubcmd("npm", "version", "npm version"),
		MustSubcmd("npm", "ls", "npm list"),
		MustSubcmd("npm", "list", "npm list"),
		MustSubcmd("npm", "outdated", "npm outdated"),
		MustSubcmd("npm", "audit", "npm audit"),
		MustSubcmd("npm", "view", "npm view"),
		MustSubcmd("npm", "search", "npm search"),
		MustSubcmd("npm", "info", "npm info"),
		MustSubcmd("npm", "show", "npm show"),
		// Python
		MustSubcmd("python", "--version", "python version"),
		MustSubcmd("python3", "--version", "python3 version"),
		MustSubcmd("pip", "list", "pip list"),
		MustSubcmd("pip", "show", "pip show"),
		MustSubcmd("pip", "freeze", "pip freeze"),
		MustSubcmd("pip3", "list", "pip3 list"),
		MustSubcmd("pip3", "show", "pip3 show"),
		MustSubcmd("pip3", "freeze", "pip3 freeze"),
		// Rust
		MustSubcmd("cargo", "--version", "cargo version"),
		MustSubcmd("rustc", "--version", "rustc version"),
		MustSubcmd("cargo", "tree", "cargo tree"),
		MustSubcmd("cargo", "metadata", "cargo metadata"),
		MustSubcmd("cargo", "check", "cargo check"),
	}
}

// systemInfoPatterns returns patterns for system information commands.
func systemInfoPatterns() []WhitelistPattern {
	return []WhitelistPattern{
		MustSimple("pwd", "print working directory"),
		MustSimple("whoami", "current user"),
		MustSimple("id", "user identity"),
		MustSimple("hostname", "hostname"),
		MustSimple("uname", "system info"),
		MustSimple("date", "current date/time"),
		MustSimple("uptime", "system uptime"),
		MustSimple("env", "environment variables"),
		MustSimple("printenv", "print environment"),
		MustSimple("ps", "process status"),
		MustSimple("df", "disk free space"),
		MustSimple("du", "disk usage"),
		MustSimple("free", "memory usage"),
		MustPattern(`^top\s+-b\s+-n\s*1`+cmdBoundary, "top batch mode"),
		MustSimple("lsof", "list open files"),
		MustSimple("netstat", "network statistics"),
		MustSimple("ss", "socket statistics"),
	}
}

// utilityPatterns returns patterns for safe utility commands.
func utilityPatterns() []WhitelistPattern {
	return []WhitelistPattern{
		MustSimple("echo", "echo"),
		MustSimple("printf", "printf"),
		MustSimple("test", "test condition"),
		MustPattern(`^\[\s`, "test condition"),
		MustPattern(`^\[\[\s`, "extended test"),
		MustSimple("true", "true"),
		MustSimple("false", "false"),
		MustPattern(`^sleep\s+[0-9]+(\.[0-9]+)?`+cmdBoundary, "sleep"),
		MustSimple("seq", "sequence generator"),
		MustSimple("expr", "expression evaluator"),
		MustSimple("bc", "calculator"),
		// Archive inspection (not extraction/creation)
		MustPattern(`^tar\s+-t`, "tar list"),
		MustSubcmd("tar", "--list", "tar list"),
		MustSimple("zipinfo", "zip info"),
		MustPattern(`^unzip\s+-l`, "unzip list"),
		MustPattern(`^unzip\s+-Z`, "unzip info"),
	}
}

// containerPatterns returns patterns for container read operations.
func containerPatterns() []WhitelistPattern {
	return []WhitelistPattern{
		// Docker
		MustSubcmd("docker", "ps", "docker ps"),
		MustSubcmd("docker", "images", "docker images"),
		MustSubcmd("docker", "logs", "docker logs"),
		MustSubcmd("docker", "inspect", "docker inspect"),
		MustSubcmd("docker", "version", "docker version"),
		MustSubcmd("docker", "info", "docker info"),
		MustSubcmd("docker", "stats", "docker stats"),
		MustSubcmd("docker", "top", "docker top"),
		MustSubcmd("docker", "port", "docker port"),
		MustSubcmd("docker", "diff", "docker diff"),
		MustSubcmd("docker", "history", "docker history"),
		// Kubectl
		MustSubcmd("kubectl", "get", "kubectl get"),
		MustSubcmd("kubectl", "describe", "kubectl describe"),
		MustSubcmd("kubectl", "logs", "kubectl logs"),
		MustSubcmd("kubectl", "top", "kubectl top"),
		MustSubcmd("kubectl", "cluster-info", "kubectl cluster-info"),
		MustSubcmd("kubectl", "version", "kubectl version"),
		MustPattern(`^kubectl\s+config\s+view`+cmdBoundary, "kubectl config view"),
		MustPattern(`^kubectl\s+config\s+current-context`+cmdBoundary, "kubectl current-context"),
		MustSubcmd("kubectl", "api-resources", "kubectl api-resources"),
	}
}

// Patterns for detecting ReDoS-vulnerable regex constructs.
var (
	// Nested quantifiers: (a+)+, (.*)*,  (.+)+, etc.
	nestedQuantifierPattern = regexp.MustCompile(`\([^)]*[+*][^)]*\)[+*?]|\([^)]*[+*][^)]*\)\{`)
	// Large repetitions: {100,}, {1000}, etc.
	largeRepetitionPattern = regexp.MustCompile(`\{(\d+)(,(\d*))?\}`)
	// Alternation with outer quantifier: (a|b)+, (a|b)*, (x|y|z){n,}.
	alternationQuantifierPattern = regexp.MustCompile(`\([^)]*\|[^)]*\)[+*]|\([^)]*\|[^)]*\)\{`)
)

// validateRegexSafety checks if a regex pattern contains constructs that could cause
// catastrophic backtracking (ReDoS). Returns an error if the pattern is unsafe.
func validateRegexSafety(pattern string) error {
	// Check for nested quantifiers (e.g., (a+)+, (.*)*) which can cause exponential backtracking
	if nestedQuantifierPattern.MatchString(pattern) {
		return ErrNestedQuantifiers
	}

	// Check for alternation with outer quantifier (e.g., (a|b)+, (x|y)*) which can cause backtracking
	if alternationQuantifierPattern.MatchString(pattern) {
		return ErrAlternationQuantifier
	}

	// Check for large repetitions
	matches := largeRepetitionPattern.FindAllStringSubmatch(pattern, -1)
	for _, match := range matches {
		if len(match) >= 2 {
			var count int
			if _, err := fmt.Sscanf(match[1], "%d", &count); err == nil && count >= 100 {
				return fmt.Errorf("%w: {%d,...}", ErrLargeRepetition, count)
			}
		}
	}

	return nil
}

// parseAndValidatePattern compiles a pattern string with validation.
// Returns the compiled regex or an error.
func parseAndValidatePattern(pattern string) (*regexp.Regexp, error) {
	if len(pattern) > MaxCommandLength {
		return nil, ErrPatternTooLong
	}
	if err := validateRegexSafety(pattern); err != nil {
		return nil, err
	}
	return regexp.Compile(pattern)
}

// parseSingleWhitelistPattern parses a single JSON pattern entry into a WhitelistPattern.
func parseSingleWhitelistPattern(jp WhitelistPatternJSON) (WhitelistPattern, error) {
	if jp.Pattern == "" {
		return WhitelistPattern{}, ErrPatternRequired
	}

	// Apply case-insensitive flag by prepending (?i) if needed
	pattern := jp.Pattern
	if jp.CaseInsensitive && !strings.HasPrefix(pattern, "(?i)") {
		pattern = "(?i)" + pattern
	}

	re, err := parseAndValidatePattern(pattern)
	if err != nil {
		return WhitelistPattern{}, fmt.Errorf("invalid pattern %q: %w", jp.Pattern, err)
	}

	wp := WhitelistPattern{
		Pattern:     re,
		Description: jp.Description,
	}

	if wp.Description == "" {
		wp.Description = fmt.Sprintf("custom pattern: %s", jp.Pattern)
	}

	if jp.ExcludePattern != "" {
		excludeRe, err := parseAndValidatePattern(jp.ExcludePattern)
		if err != nil {
			return WhitelistPattern{}, fmt.Errorf("invalid exclude pattern %q: %w", jp.ExcludePattern, err)
		}
		wp.ExcludePattern = excludeRe
	}

	return wp, nil
}

// ParseWhitelistPatternsJSON parses a JSON array of whitelist patterns.
// Each entry can have: pattern (required), exclude_pattern (optional), description (optional),
// and case_insensitive (optional, defaults to false).
// Returns nil and an error if any pattern is invalid (fail-fast for security).
func ParseWhitelistPatternsJSON(jsonStr string) ([]WhitelistPattern, error) {
	if jsonStr == "" {
		return nil, nil
	}

	var jsonPatterns []WhitelistPatternJSON
	if err := json.Unmarshal([]byte(jsonStr), &jsonPatterns); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	result := make([]WhitelistPattern, 0, len(jsonPatterns))

	for i, jp := range jsonPatterns {
		wp, err := parseSingleWhitelistPattern(jp)
		if err != nil {
			// Fail completely on any invalid pattern (security: no partial results)
			return nil, fmt.Errorf("entry %d: %w", i, err)
		}
		result = append(result, wp)
	}

	return result, nil
}

// ValidateMode checks if a mode string is valid.
func ValidateMode(mode string) (CommandValidationMode, error) {
	switch strings.ToLower(mode) {
	case "blacklist", "":
		return ModeBlacklist, nil
	case "whitelist":
		return ModeWhitelist, nil
	default:
		return ModeBlacklist, fmt.Errorf(
			"invalid command validation mode: %s (must be 'blacklist' or 'whitelist')",
			mode,
		)
	}
}

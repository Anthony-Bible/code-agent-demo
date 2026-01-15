// Package safety provides shared safety-related functionality for command execution.
// It contains dangerous command patterns used by both the investigation agent
// and interactive mode to prevent execution of destructive commands.
package safety

import (
	"regexp"
	"strings"
)

// DangerousPattern represents a pattern that indicates a dangerous command.
type DangerousPattern struct {
	Pattern *regexp.Regexp
	Reason  string
}

// DangerousPatterns contains all patterns for detecting dangerous commands.
// This is the single source of truth for dangerous command detection across
// the investigation agent and interactive mode.
//
//nolint:gochecknoglobals // This is intentionally a package-level constant for dangerous command detection
var DangerousPatterns = []DangerousPattern{
	// Destructive file operations
	{regexp.MustCompile(`rm\s+(-\w+\s+)*[/~*]`), "destructive rm command"},
	{regexp.MustCompile(`rm\s+-rf\b`), "recursive force delete"},

	// Privilege escalation
	{regexp.MustCompile(`sudo\s+`), "sudo command"},
	{regexp.MustCompile(`su\s+-`), "switch user command"},
	{regexp.MustCompile(`doas\s+`), "doas privilege escalation"},

	// Insecure permissions
	{regexp.MustCompile(`chmod\s+(-R\s+)?777`), "insecure chmod"},
	{regexp.MustCompile(`chmod\s+-R\s+777\s*/`), "recursive insecure chmod on root"},

	// Filesystem operations
	{regexp.MustCompile(`mkfs\.`), "filesystem format"},
	{regexp.MustCompile(`fdisk\s+`), "disk partitioning"},
	{regexp.MustCompile(`parted\s+`), "disk partitioning"},

	// Low-level disk operations
	{regexp.MustCompile(`dd\s+if=`), "low-level disk operation"},
	{regexp.MustCompile(`>\s*/dev/sd`), "write to disk device"},
	{regexp.MustCompile(`>\s*/dev/nvme`), "write to nvme device"},
	{regexp.MustCompile(`>\s*/dev/hd`), "write to disk device"},

	// Fork bomb and resource exhaustion
	{regexp.MustCompile(`:\(\)\s*\{\s*:\s*\|\s*:\s*&\s*\}\s*;`), "fork bomb"},
	{regexp.MustCompile(`\$\(:\)\{\s*:\|:&`), "fork bomb variant"},

	// Network attacks
	{regexp.MustCompile(`curl\s+.*\|\s*(ba)?sh`), "remote code execution"},
	{regexp.MustCompile(`wget\s+.*\|\s*(ba)?sh`), "remote code execution"},
	{regexp.MustCompile(`curl\s+.*-o\s*/`), "download to system path"},

	// System modification
	{regexp.MustCompile(`>\s*/etc/passwd`), "modify passwd file"},
	{regexp.MustCompile(`>\s*/etc/shadow`), "modify shadow file"},
	{regexp.MustCompile(`>\s*/etc/sudoers`), "modify sudoers file"},

	// History manipulation (potential cover-up)
	{regexp.MustCompile(`history\s+-c`), "clear command history"},
	{regexp.MustCompile(`>\s*~/\.bash_history`), "clear bash history"},
	{regexp.MustCompile(`shred\s+.*history`), "shred history file"},

	// Process manipulation
	{regexp.MustCompile(`kill\s+-9\s+-1`), "kill all processes"},
	{regexp.MustCompile(`pkill\s+-9\s+-1`), "kill all processes"},
	{regexp.MustCompile(`killall\s+-9`), "kill all processes by name"},

	// Boot/system damage
	{regexp.MustCompile(`>\s*/boot/`), "modify boot files"},
	{regexp.MustCompile(`rm\s+.*(/boot/|/vmlinuz)`), "delete kernel files"},
}

// IsDangerousCommand checks if a command matches any dangerous patterns.
// Special case: writing to /dev/null is allowed as it's a common pattern
// for suppressing output.
// Returns (true, reason) if dangerous, (false, "") if safe.
func IsDangerousCommand(cmd string) (bool, string) {
	for _, dp := range DangerousPatterns {
		if dp.Pattern.MatchString(cmd) {
			// Allow writes to /dev/null (common pattern for suppressing output)
			if strings.Contains(dp.Reason, "write to") && strings.Contains(cmd, "/dev/null") {
				continue
			}
			return true, dp.Reason
		}
	}
	return false, ""
}

// IsCommandBlocked checks if a command contains any blocked pattern.
// This is a simpler substring-based check for backward compatibility
// with configurations that use string patterns.
func IsCommandBlocked(cmd string, blockedPatterns []string) bool {
	// Normalize whitespace
	normalized := strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, cmd)

	for _, blocked := range blockedPatterns {
		if strings.Contains(normalized, blocked) {
			return true
		}
	}
	return false
}

// DefaultBlockedCommandStrings returns the default list of blocked command substrings.
// These are used by InvestigationConfig for simple substring matching.
// For regex-based detection, use IsDangerousCommand instead.
func DefaultBlockedCommandStrings() []string {
	return []string{
		"rm -rf",
		"dd if=",
		"mkfs",
		":(){:|:&};:",
		"> /dev/sda",
		"chmod -R 777 /",
		"sudo ",
		"curl | sh",
		"wget | sh",
		"> /etc/passwd",
		"> /etc/shadow",
		"kill -9 -1",
		"history -c",
	}
}

// PatternReasons returns a map of pattern reasons for documentation/logging.
func PatternReasons() map[string]string {
	reasons := make(map[string]string)
	for _, p := range DangerousPatterns {
		reasons[p.Pattern.String()] = p.Reason
	}
	return reasons
}

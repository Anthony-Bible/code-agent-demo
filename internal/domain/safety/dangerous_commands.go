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
	Pattern      *regexp.Regexp
	Reason       string
	AllowDevNull bool // If true, writing to /dev/null is permitted for this pattern
}

// DangerousPatterns contains all patterns for detecting dangerous commands.
// This is the single source of truth for dangerous command detection across
// the investigation agent and interactive mode.
//
//nolint:gochecknoglobals // This is intentionally a package-level constant for dangerous command detection
var DangerousPatterns = []DangerousPattern{
	// Destructive file operations
	{Pattern: regexp.MustCompile(`rm\s+(-\w+\s+)*[/~*]`), Reason: "destructive rm command"},
	{Pattern: regexp.MustCompile(`rm\s+-rf\b`), Reason: "recursive force delete"},

	// Privilege escalation
	{Pattern: regexp.MustCompile(`sudo\s+`), Reason: "sudo command"},
	{Pattern: regexp.MustCompile(`su\s+-`), Reason: "switch user command"},
	{Pattern: regexp.MustCompile(`doas\s+`), Reason: "doas privilege escalation"},

	// Insecure permissions and ownership
	{Pattern: regexp.MustCompile(`chmod\s+(-R\s+)?777`), Reason: "insecure chmod"},
	{Pattern: regexp.MustCompile(`chown\s+(-R\s+)?root`), Reason: "change ownership to root"},
	{Pattern: regexp.MustCompile(`chown\s+-R\s+\S+\s+/`), Reason: "recursive ownership change on root"},

	// Filesystem operations
	{Pattern: regexp.MustCompile(`mkfs\.`), Reason: "filesystem format"},
	{Pattern: regexp.MustCompile(`fdisk\s+`), Reason: "disk partitioning"},
	{Pattern: regexp.MustCompile(`parted\s+`), Reason: "disk partitioning"},

	// Low-level disk operations
	{Pattern: regexp.MustCompile(`dd\s+if=`), Reason: "low-level disk operation"},
	{Pattern: regexp.MustCompile(`>\s*/dev/sd`), Reason: "write to disk device", AllowDevNull: true},
	{Pattern: regexp.MustCompile(`>\s*/dev/nvme`), Reason: "write to nvme device", AllowDevNull: true},
	{Pattern: regexp.MustCompile(`>\s*/dev/hd`), Reason: "write to disk device", AllowDevNull: true},

	// Fork bomb and resource exhaustion
	{Pattern: regexp.MustCompile(`:\(\)\s*\{[^}]*:\s*\|\s*:[^}]*&[^}]*\}`), Reason: "fork bomb"},
	{Pattern: regexp.MustCompile(`\$\(:\)\{\s*:\|:&`), Reason: "fork bomb variant"},

	// Network attacks
	{Pattern: regexp.MustCompile(`curl\s+.*\|\s*(/usr)?(/bin/)?(ba)?sh`), Reason: "remote code execution"},
	{Pattern: regexp.MustCompile(`wget\s+.*\|\s*(/usr)?(/bin/)?(ba)?sh`), Reason: "remote code execution"},
	{Pattern: regexp.MustCompile(`curl\s+.*-o\s*/`), Reason: "download to system path"},

	// System modification
	{Pattern: regexp.MustCompile(`>\s*/etc/passwd`), Reason: "modify passwd file"},
	{Pattern: regexp.MustCompile(`>\s*/etc/shadow`), Reason: "modify shadow file"},
	{Pattern: regexp.MustCompile(`>\s*/etc/sudoers`), Reason: "modify sudoers file"},

	// History manipulation (potential cover-up)
	{Pattern: regexp.MustCompile(`history\s+-c`), Reason: "clear command history"},
	{Pattern: regexp.MustCompile(`>\s*~/\.bash_history`), Reason: "clear bash history"},
	{Pattern: regexp.MustCompile(`shred\s+.*history`), Reason: "shred history file"},

	// Process manipulation
	{Pattern: regexp.MustCompile(`kill\s+(-9|-KILL|-SIGKILL)\s+(--\s+)?-1`), Reason: "kill all processes"},
	{Pattern: regexp.MustCompile(`pkill\s+-9\s+-1`), Reason: "kill all processes"},
	{Pattern: regexp.MustCompile(`killall\s+-9`), Reason: "kill all processes by name"},

	// Boot/system damage
	{Pattern: regexp.MustCompile(`>\s*/boot/`), Reason: "modify boot files"},
	{Pattern: regexp.MustCompile(`rm\s+.*(/boot/|/vmlinuz)`), Reason: "delete kernel files"},

	// Service manipulation
	{Pattern: regexp.MustCompile(`systemctl\s+(stop|disable|mask)\s+`), Reason: "stop/disable system service"},
	{Pattern: regexp.MustCompile(`service\s+\S+\s+stop`), Reason: "stop system service"},

	// Firewall manipulation
	{Pattern: regexp.MustCompile(`iptables\s+(-F|--flush)`), Reason: "flush firewall rules"},
	{Pattern: regexp.MustCompile(`ufw\s+disable`), Reason: "disable firewall"},
	{Pattern: regexp.MustCompile(`firewall-cmd\s+.*--remove`), Reason: "remove firewall rules"},

	// Crontab manipulation
	{Pattern: regexp.MustCompile(`crontab\s+-r`), Reason: "remove crontab"},
	{Pattern: regexp.MustCompile(`crontab\s+-e`), Reason: "edit crontab"},
	{Pattern: regexp.MustCompile(`>\s*/etc/cron`), Reason: "modify cron files"},
	{Pattern: regexp.MustCompile(`>\s*/var/spool/cron`), Reason: "modify cron spool"},

	// Environment variable manipulation (code injection / binary hijacking)
	{Pattern: regexp.MustCompile(`export\s+LD_PRELOAD=`), Reason: "LD_PRELOAD code injection"},
	{Pattern: regexp.MustCompile(`export\s+PATH=(/tmp|/var/tmp|/dev/shm)`), Reason: "PATH binary hijacking"},

	// Package manager abuse (destructive package operations)
	{
		Pattern: regexp.MustCompile(
			`apt(-get)?\s+(remove|purge)\s+.*--(purge|auto-remove).*\s+(systemd|glibc|libc6|coreutils|bash)`,
		),
		Reason: "destructive package removal",
	},
	{
		Pattern: regexp.MustCompile(`apt(-get)?\s+(remove|purge)\s+(systemd|glibc|libc6|coreutils|bash)`),
		Reason:  "critical package removal",
	},
	{
		Pattern: regexp.MustCompile(`yum\s+(erase|remove)\s+(glibc|systemd|coreutils|bash)`),
		Reason:  "critical package removal",
	},
	{
		Pattern: regexp.MustCompile(`dnf\s+(erase|remove)\s+(glibc|systemd|coreutils|bash)`),
		Reason:  "critical package removal",
	},

	// Container escapes
	{
		Pattern: regexp.MustCompile(`docker\s+run\s+.*--privileged`),
		Reason:  "privileged container (container escape risk)",
	},
	{Pattern: regexp.MustCompile(`nsenter\s+.*--target\s+1\s+`), Reason: "nsenter to init process (container escape)"},
	{Pattern: regexp.MustCompile(`nsenter\s+.*-t\s*1\s+`), Reason: "nsenter to init process (container escape)"},
}

// MaxCommandLength is the maximum length of a command that will be processed.
// Commands exceeding this length are considered dangerous to prevent ReDoS attacks.
const MaxCommandLength = 10000

// IsDangerousCommand checks if a command matches any dangerous patterns.
// Special case: writing to /dev/null is allowed for patterns with AllowDevNull set.
// Commands exceeding MaxCommandLength are rejected to prevent ReDoS attacks.
// Returns (true, reason) if dangerous, (false, "") if safe.
func IsDangerousCommand(cmd string) (bool, string) {
	// Prevent ReDoS attacks with overly long input
	if len(cmd) > MaxCommandLength {
		return true, "command exceeds maximum safe length"
	}

	for _, dp := range DangerousPatterns {
		if dp.Pattern.MatchString(cmd) {
			// Allow writes to /dev/null for patterns that permit it
			if dp.AllowDevNull && strings.Contains(cmd, "/dev/null") {
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
		"chown -R",
		"sudo ",
		"curl | sh",
		"wget | sh",
		"> /etc/passwd",
		"> /etc/shadow",
		"kill -9 -1",
		"history -c",
		"systemctl stop",
		"systemctl disable",
		"iptables -F",
		"ufw disable",
		"crontab -r",
		"export LD_PRELOAD=",
		"apt remove systemd",
		"apt-get remove systemd",
		"yum erase glibc",
		"docker run --privileged",
		"nsenter --target 1",
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

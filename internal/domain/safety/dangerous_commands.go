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

// MustDangerous creates a DangerousPattern from a regex pattern string.
// Panics if pattern is invalid (should be caught in tests).
func MustDangerous(pattern, reason string) DangerousPattern {
	return DangerousPattern{
		Pattern: regexp.MustCompile(pattern),
		Reason:  reason,
	}
}

// MustDangerousDevNull creates a DangerousPattern that allows /dev/null.
// Panics if pattern is invalid (should be caught in tests).
func MustDangerousDevNull(pattern, reason string) DangerousPattern {
	return DangerousPattern{
		Pattern:      regexp.MustCompile(pattern),
		Reason:       reason,
		AllowDevNull: true,
	}
}

// MustCmdWithFlags creates a DangerousPattern for a command with dangerous flags.
// Pattern format: `(?i){cmd}\s+.*{flagPattern}`
// Panics if pattern is invalid (should be caught in tests).
func MustCmdWithFlags(cmd, flagPattern, reason string) DangerousPattern {
	pattern := `(?i)` + regexp.QuoteMeta(cmd) + `\s+.*` + flagPattern
	return DangerousPattern{
		Pattern: regexp.MustCompile(pattern),
		Reason:  reason,
	}
}

// DangerousPatterns contains all patterns for detecting dangerous commands.
// This is the single source of truth for dangerous command detection across
// the investigation agent and interactive mode.
//
// Security Context: Each pattern is documented with its attack vector and potential impact.
// These patterns protect against common attack vectors identified in OWASP, MITRE ATT&CK,
// and real-world incident reports.
//
//nolint:gochecknoglobals // This is intentionally a package-level constant for dangerous command detection
var DangerousPatterns = []DangerousPattern{
	// === Destructive file operations ===
	// Attack: Data destruction, system sabotage
	// Impact: Permanent data loss, system unusable, potential business continuity disaster

	// rm with path wildcards can recursively delete critical system or user data
	MustDangerous(`rm\s+(-\w+\s+)*[/~*]`, "destructive rm command"),
	// rm -rf bypasses prompts and recursively deletes; common in "rm -rf /" attacks
	MustDangerous(`rm\s+-rf\b`, "recursive force delete"),

	// === Privilege escalation ===
	// Attack: Vertical privilege escalation, bypass access controls
	// Impact: Full system compromise, ability to execute any command as root

	// sudo grants temporary root access; attackers use it to escalate from user to root
	MustDangerous(`sudo\s+`, "sudo command"),
	// su - opens a root login shell; enables persistent elevated access
	MustDangerous(`su\s+-`, "switch user command"),
	// doas is BSD's sudo equivalent; same privilege escalation risks
	MustDangerous(`doas\s+`, "doas privilege escalation"),

	// === Insecure permissions and ownership ===
	// Attack: Permission weakening, ownership hijacking
	// Impact: Any user/process can read/write/execute files, enabling data theft or code injection

	// chmod 777 grants all permissions to everyone; exposes files to unauthorized modification
	MustDangerous(`chmod\s+(-R\s+)?777`, "insecure chmod"),
	// chown root can transfer ownership to root, potentially locking out legitimate owners
	MustDangerous(`chown\s+(-R\s+)?root`, "change ownership to root"),
	// Recursive chown on / can change ownership of all system files
	MustDangerous(`chown\s+-R\s+\S+\s+/`, "recursive ownership change on root"),

	// === Filesystem operations ===
	// Attack: Disk destruction, filesystem corruption
	// Impact: Complete data loss on target partition, requires full reinstall/restore

	// mkfs creates new filesystem, destroying all existing data on the device
	MustDangerous(`mkfs\.`, "filesystem format"),
	// fdisk modifies partition tables; incorrect use destroys partition layout
	MustDangerous(`fdisk\s+`, "disk partitioning"),
	// parted can resize/delete partitions; data loss on misconfiguration
	MustDangerous(`parted\s+`, "disk partitioning"),

	// === Low-level disk operations ===
	// Attack: Direct disk overwrite, data destruction
	// Impact: Bypasses filesystem protections, can overwrite boot sectors or entire drives

	// dd with if= reads raw data and can overwrite disk blocks directly
	MustDangerous(`dd\s+if=`, "low-level disk operation"),
	// Direct writes to /dev/sd* overwrites disk sectors, bypassing filesystem
	MustDangerousDevNull(`>\s*/dev/sd`, "write to disk device"),
	// NVMe devices are modern SSDs; direct writes cause same damage as SATA drives
	MustDangerousDevNull(`>\s*/dev/nvme`, "write to nvme device"),
	// Legacy /dev/hd* devices (IDE disks); same direct write risks
	MustDangerousDevNull(`>\s*/dev/hd`, "write to disk device"),

	// === Fork bomb and resource exhaustion ===
	// Attack: Denial of Service (DoS), resource exhaustion
	// Impact: System becomes unresponsive, requires hard reboot, potential data corruption

	// Classic fork bomb :(){ :|:& };: - exponentially spawns processes until system crashes
	MustDangerous(`:\(\)\s*\{[^}]*:\s*\|\s*:[^}]*&[^}]*\}`, "fork bomb"),
	// Variant fork bomb using $() syntax; same exponential process spawning
	MustDangerous(`\$\(:\)\{\s*:\|:&`, "fork bomb variant"),

	// === Network attacks ===
	// Attack: Remote Code Execution (RCE), supply chain attacks
	// Impact: Arbitrary code execution, malware installation, full system compromise

	// curl | sh downloads and executes remote code without inspection; classic RCE vector
	MustDangerous(`curl\s+.*\|\s*(/usr)?(/bin/)?(ba)?sh`, "remote code execution"),
	// wget | sh same as curl; downloads and executes untrusted remote scripts
	MustDangerous(`wget\s+.*\|\s*(/usr)?(/bin/)?(ba)?sh`, "remote code execution"),
	// curl -o to system path can overwrite binaries or config files with malicious content
	MustDangerous(`curl\s+.*-o\s*/`, "download to system path"),

	// === System modification ===
	// Attack: Authentication bypass, privilege escalation
	// Impact: Unauthorized user creation, password changes, sudo access modification

	// Writing to /etc/passwd can add users with arbitrary UIDs including root (UID 0)
	MustDangerous(`>\s*/etc/passwd`, "modify passwd file"),
	// /etc/shadow contains password hashes; modification enables authentication bypass
	MustDangerous(`>\s*/etc/shadow`, "modify shadow file"),
	// /etc/sudoers controls sudo access; modification can grant root to any user
	MustDangerous(`>\s*/etc/sudoers`, "modify sudoers file"),

	// === History manipulation ===
	// Attack: Evidence tampering, forensic evasion
	// Impact: Hides attacker activity, complicates incident response and forensics

	// history -c clears shell command history, hiding evidence of malicious commands
	MustDangerous(`history\s+-c`, "clear command history"),
	// Redirecting to .bash_history overwrites/clears persistent command history
	MustDangerous(`>\s*~/\.bash_history`, "clear bash history"),
	// shred securely overwrites history files, making forensic recovery impossible
	MustDangerous(`shred\s+.*history`, "shred history file"),

	// === Process manipulation ===
	// Attack: Denial of Service, system destabilization
	// Impact: Kills critical processes, system crash, service outage

	// kill -9 -1 sends SIGKILL to all processes; causes immediate system crash
	MustDangerous(`kill\s+(-9|-KILL|-SIGKILL)\s+(--\s+)?-1`, "kill all processes"),
	// pkill -9 -1 same effect using process name matching
	MustDangerous(`pkill\s+-9\s+-1`, "kill all processes"),
	// killall -9 kills all processes matching name; dangerous with common names
	MustDangerous(`killall\s+-9`, "kill all processes by name"),

	// === Boot/system damage ===
	// Attack: System destruction, permanent boot failure
	// Impact: System cannot boot, requires recovery media or reinstall

	// Writing to /boot/ can corrupt bootloader, initramfs, or kernel images
	MustDangerous(`>\s*/boot/`, "modify boot files"),
	// Deleting kernel (vmlinuz) or boot files renders system unbootable
	MustDangerous(`rm\s+.*(/boot/|/vmlinuz)`, "delete kernel files"),

	// === Service manipulation ===
	// Attack: Service disruption, security control bypass
	// Impact: Critical services stop, security daemons disabled, system vulnerable

	// systemctl stop/disable/mask can halt critical services (sshd, firewall, logging)
	MustDangerous(`systemctl\s+(stop|disable|mask)\s+`, "stop/disable system service"),
	// service stop halts services on SysV init systems; same risks as systemctl
	MustDangerous(`service\s+\S+\s+stop`, "stop system service"),

	// === Firewall manipulation ===
	// Attack: Security control bypass, network exposure
	// Impact: Firewall disabled, all ports exposed, network-based attacks enabled

	// iptables -F flushes all rules, leaving system with no firewall protection
	MustDangerous(`iptables\s+(-F|--flush)`, "flush firewall rules"),
	// ufw disable turns off the firewall completely on Ubuntu/Debian systems
	MustDangerous(`ufw\s+disable`, "disable firewall"),
	// firewall-cmd --remove deletes specific rules, potentially exposing services
	MustDangerous(`firewall-cmd\s+.*--remove`, "remove firewall rules"),

	// === Crontab manipulation ===
	// Attack: Persistence, scheduled malicious execution
	// Impact: Attacker maintains access via scheduled tasks, or removes legitimate jobs

	// crontab -r removes all user's cron jobs; can delete critical scheduled backups
	MustDangerous(`crontab\s+-r`, "remove crontab"),
	// crontab -e opens editor; can be used to add malicious scheduled commands
	MustDangerous(`crontab\s+-e`, "edit crontab"),
	// Writing to /etc/cron* can add system-wide scheduled malicious jobs
	MustDangerous(`>\s*/etc/cron`, "modify cron files"),
	// /var/spool/cron contains user crontabs; modification affects scheduled jobs
	MustDangerous(`>\s*/var/spool/cron`, "modify cron spool"),

	// === Environment variable manipulation ===
	// Attack: Code injection, binary hijacking
	// Impact: Malicious libraries loaded, legitimate commands replaced with trojans

	// LD_PRELOAD loads shared library before others; enables code injection into any process
	MustDangerous(`export\s+LD_PRELOAD=`, "LD_PRELOAD code injection"),
	// PATH to /tmp etc. means trojaned binaries in those dirs execute instead of system ones
	MustDangerous(`export\s+PATH=(/tmp|/var/tmp|/dev/shm)`, "PATH binary hijacking"),

	// === Package manager abuse ===
	// Attack: System destruction via package removal
	// Impact: Critical libraries removed, system cannot function, requires reinstall

	// Removing systemd/glibc/coreutils/bash with purge flags destroys the system
	MustDangerous(
		`apt(-get)?\s+(remove|purge)\s+.*--(purge|auto-remove).*\s+(systemd|glibc|libc6|coreutils|bash)`,
		"destructive package removal",
	),
	// Direct removal of critical packages breaks nearly all system functionality
	MustDangerous(`apt(-get)?\s+(remove|purge)\s+(systemd|glibc|libc6|coreutils|bash)`, "critical package removal"),
	// yum on RHEL/CentOS; removing glibc/systemd bricks the system
	MustDangerous(`yum\s+(erase|remove)\s+(glibc|systemd|coreutils|bash)`, "critical package removal"),
	// dnf on Fedora/newer RHEL; same critical package removal risks
	MustDangerous(`dnf\s+(erase|remove)\s+(glibc|systemd|coreutils|bash)`, "critical package removal"),

	// === Container escapes ===
	// Attack: Container breakout, host compromise
	// Impact: Escape container isolation, gain access to host system and other containers

	// --privileged disables container isolation; container has full host access
	MustDangerous(`docker\s+run\s+.*--privileged`, "privileged container (container escape risk)"),
	// nsenter to PID 1 enters host's init namespace from container; complete escape
	MustDangerous(`nsenter\s+.*--target\s+1\s+`, "nsenter to init process (container escape)"),
	// -t 1 is shorthand for --target 1; same container escape vector
	MustDangerous(`nsenter\s+.*-t\s*1\s+`, "nsenter to init process (container escape)"),

	// === Dangerous find command options ===
	// Attack: Arbitrary code execution, uncontrolled file deletion
	// Impact: -exec runs commands on found files; -delete removes files without confirmation

	// find with -exec/-execdir/-delete/-ok/-okdir can execute or delete files
	MustCmdWithFlags("find", FindDangerousFlags, "find with dangerous flags (-exec, -execdir, -delete, -ok, -okdir)"),
}

// isDangerousCommandWithDepth checks if a command is dangerous with depth tracking.
// This internal function tracks recursion depth to prevent stack overflow from
// deeply nested command substitutions.
func isDangerousCommandWithDepth(cmd string, depth int) (bool, string) {
	// Prevent ReDoS attacks with overly long input
	if len(cmd) > MaxCommandLength {
		return true, "command exceeds maximum safe length"
	}

	// Check recursion depth
	if depth >= MaxRecursionDepth {
		return true, "command substitution nesting exceeds maximum depth"
	}

	// Check the command itself
	if dangerous, reason := checkDangerousPatterns(cmd); dangerous {
		return dangerous, reason
	}

	// Also check commands inside $() substitutions (depth-aware)
	subCommands := ExtractDollarParenCommands(cmd)
	for _, subCmd := range subCommands {
		if dangerous, reason := isDangerousCommandWithDepth(subCmd, depth+1); dangerous {
			return dangerous, reason
		}
	}

	// Also check commands inside backtick substitutions (depth-aware)
	backtickCommands := ExtractBacktickCommands(cmd)
	for _, subCmd := range backtickCommands {
		if dangerous, reason := isDangerousCommandWithDepth(subCmd, depth+1); dangerous {
			return dangerous, reason
		}
	}

	return false, ""
}

// IsDangerousCommand checks if a command matches any dangerous patterns.
// Special case: writing to /dev/null is allowed for patterns with AllowDevNull set.
// Commands exceeding MaxCommandLength are rejected to prevent ReDoS attacks.
// Also recursively checks commands inside $() and backtick substitutions.
// Recursion depth is limited to MaxRecursionDepth to prevent stack overflow.
// Returns (true, reason) if dangerous, (false, "") if safe.
func IsDangerousCommand(cmd string) (bool, string) {
	return isDangerousCommandWithDepth(cmd, 0)
}

// checkDangerousPatterns checks if a command matches any dangerous patterns.
// This is the core pattern matching logic used by IsDangerousCommand.
func checkDangerousPatterns(cmd string) (bool, string) {
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

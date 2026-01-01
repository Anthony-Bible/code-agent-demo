// Package config provides configuration types for the application layer.
package config

import (
	"errors"
	"strings"
	"time"
)

// Sentinel errors for InvestigationConfig validation.
// These errors are returned when configuration values fail validation checks.
var (
	// ErrInvalidMaxActions is returned when max actions is zero or negative.
	ErrInvalidMaxActions = errors.New("max actions must be positive")
	// ErrInvalidMaxDuration is returned when max duration is zero or negative.
	ErrInvalidMaxDuration = errors.New("max duration must be positive")
	// ErrInvalidMaxConcurrent is returned when max concurrent is zero or negative.
	ErrInvalidMaxConcurrent = errors.New("max concurrent investigations must be positive")
	// ErrInvalidConfidenceThreshold is returned when confidence threshold is outside [0.0, 1.0].
	ErrInvalidConfidenceThreshold = errors.New("confidence threshold must be between 0.0 and 1.0")
	// ErrInvalidErrorThreshold is returned when error threshold is zero or negative.
	ErrInvalidErrorThreshold = errors.New("error threshold must be positive")
	// ErrEmptyAllowedTools is returned when the allowed tools list is nil or empty.
	ErrEmptyAllowedTools = errors.New("allowed tools list cannot be empty")
	// ErrBlockedCommandContainsAllowed is returned when a blocked command pattern overlaps with allowed tools.
	ErrBlockedCommandContainsAllowed = errors.New("blocked command overlaps with allowed tools")
)

// InvestigationConfig holds safety and operational limits for investigations.
// It defines constraints on what investigations can do, how long they can run,
// and when they should escalate to human operators. Use DefaultInvestigationConfig
// for sensible production defaults, or NewInvestigationConfig for a blank config.
type InvestigationConfig struct {
	maxActions                   int           // Maximum tool executions per investigation
	maxDuration                  time.Duration // Maximum wall-clock time for an investigation
	maxConcurrent                int           // Maximum simultaneous investigations
	allowedTools                 []string      // Tools the investigation may use
	blockedCommands              []string      // Command patterns that are never allowed
	allowedDirectories           []string      // Directories the investigation may access (nil = all)
	requireHumanApprovalPatterns []string      // Patterns requiring human confirmation
	confirmBeforeRestart         bool          // Require confirmation for restart operations
	confirmBeforeDelete          bool          // Require confirmation for delete operations
	escalateOnConfidenceBelow    float64       // Escalate if confidence drops below this [0.0-1.0]
	escalateOnMultipleErrors     int           // Escalate after this many consecutive errors
}

// NewInvestigationConfig creates a new empty InvestigationConfig.
// The returned config has zero values and will fail Validate() until configured.
// For a ready-to-use config, use DefaultInvestigationConfig instead.
func NewInvestigationConfig() *InvestigationConfig {
	return &InvestigationConfig{}
}

// DefaultInvestigationConfig returns a config with production-ready defaults.
// Default values:
//   - maxActions: 20 (prevents runaway investigations)
//   - maxDuration: 15 minutes (reasonable timeout)
//   - maxConcurrent: 5 (balances throughput and resource usage)
//   - allowedTools: bash, read_file, list_files (safe investigation tools)
//   - blockedCommands: common destructive patterns (rm -rf, dd, mkfs, etc.)
//   - escalateOnConfidenceBelow: 0.5 (escalate when uncertain)
//   - escalateOnMultipleErrors: 3 (escalate after repeated failures)
func DefaultInvestigationConfig() *InvestigationConfig {
	return &InvestigationConfig{
		maxActions:    20,
		maxDuration:   15 * time.Minute,
		maxConcurrent: 5,
		allowedTools:  []string{"bash", "read_file", "list_files"},
		blockedCommands: []string{
			"rm -rf",
			"dd if=",
			"mkfs",
			":(){:|:&};:",
			"> /dev/sda",
			"chmod -R 777 /",
		},
		allowedDirectories:           nil,
		requireHumanApprovalPatterns: []string{"restart", "kill", "delete"},
		confirmBeforeRestart:         true,
		confirmBeforeDelete:          true,
		escalateOnConfidenceBelow:    0.5,
		escalateOnMultipleErrors:     3,
	}
}

// MaxActionsPerInvestigation returns the maximum number of tool executions
// allowed in a single investigation before it is forcibly stopped.
func (c *InvestigationConfig) MaxActionsPerInvestigation() int {
	return c.maxActions
}

// MaxDuration returns the maximum wall-clock time an investigation may run.
func (c *InvestigationConfig) MaxDuration() time.Duration {
	return c.maxDuration
}

// MaxConcurrentInvestigations returns the maximum number of investigations
// that may run simultaneously.
func (c *InvestigationConfig) MaxConcurrentInvestigations() int {
	return c.maxConcurrent
}

// AllowedTools returns the list of tool names that investigations may use.
// The returned slice should be treated as read-only.
func (c *InvestigationConfig) AllowedTools() []string {
	return c.allowedTools
}

// BlockedCommands returns the list of command patterns that are blocked.
// Any command containing these substrings will be rejected.
func (c *InvestigationConfig) BlockedCommands() []string {
	return c.blockedCommands
}

// AllowedDirectories returns the list of directory prefixes that investigations may access.
// A nil or empty list means all directories are allowed.
func (c *InvestigationConfig) AllowedDirectories() []string {
	return c.allowedDirectories
}

// RequireHumanApprovalPatterns returns command patterns that require human confirmation.
// Commands containing these patterns will pause for approval before execution.
func (c *InvestigationConfig) RequireHumanApprovalPatterns() []string {
	return c.requireHumanApprovalPatterns
}

// ConfirmBeforeRestart returns true if restart operations require human confirmation.
func (c *InvestigationConfig) ConfirmBeforeRestart() bool {
	return c.confirmBeforeRestart
}

// ConfirmBeforeDelete returns true if delete operations require human confirmation.
func (c *InvestigationConfig) ConfirmBeforeDelete() bool {
	return c.confirmBeforeDelete
}

// EscalateOnConfidenceBelow returns the confidence threshold for escalation.
// Investigations with confidence below this value should escalate to humans.
func (c *InvestigationConfig) EscalateOnConfidenceBelow() float64 {
	return c.escalateOnConfidenceBelow
}

// EscalateOnMultipleErrors returns the consecutive error count threshold.
// Investigations should escalate after encountering this many errors in a row.
func (c *InvestigationConfig) EscalateOnMultipleErrors() int {
	return c.escalateOnMultipleErrors
}

// SetMaxActions sets the maximum number of actions allowed per investigation.
// Returns ErrInvalidMaxActions if the limit is zero or negative.
func (c *InvestigationConfig) SetMaxActions(limit int) error {
	if limit <= 0 {
		return ErrInvalidMaxActions
	}
	c.maxActions = limit
	return nil
}

// SetMaxDuration sets the maximum wall-clock time for investigations.
// Returns ErrInvalidMaxDuration if the duration is zero or negative.
func (c *InvestigationConfig) SetMaxDuration(d time.Duration) error {
	if d <= 0 {
		return ErrInvalidMaxDuration
	}
	c.maxDuration = d
	return nil
}

// SetMaxConcurrent sets the maximum number of concurrent investigations.
// Returns ErrInvalidMaxConcurrent if the limit is zero or negative.
func (c *InvestigationConfig) SetMaxConcurrent(limit int) error {
	if limit <= 0 {
		return ErrInvalidMaxConcurrent
	}
	c.maxConcurrent = limit
	return nil
}

// SetAllowedTools sets the list of tools that investigations are permitted to use.
// Returns ErrEmptyAllowedTools if the list is nil or empty.
func (c *InvestigationConfig) SetAllowedTools(tools []string) error {
	if len(tools) == 0 {
		return ErrEmptyAllowedTools
	}
	c.allowedTools = tools
	return nil
}

// SetBlockedCommands sets the list of command patterns to block.
// Commands containing any of these substrings will be rejected.
func (c *InvestigationConfig) SetBlockedCommands(commands []string) error {
	c.blockedCommands = commands
	return nil
}

// SetAllowedDirectories sets the list of directory prefixes that may be accessed.
// Pass nil or an empty slice to allow access to all directories.
func (c *InvestigationConfig) SetAllowedDirectories(dirs []string) {
	c.allowedDirectories = dirs
}

// SetRequireHumanApprovalPatterns sets patterns that require human confirmation.
// Commands containing these patterns will pause for approval.
func (c *InvestigationConfig) SetRequireHumanApprovalPatterns(patterns []string) {
	c.requireHumanApprovalPatterns = patterns
}

// SetConfirmBeforeRestart enables or disables confirmation for restart operations.
func (c *InvestigationConfig) SetConfirmBeforeRestart(confirm bool) {
	c.confirmBeforeRestart = confirm
}

// SetConfirmBeforeDelete enables or disables confirmation for delete operations.
func (c *InvestigationConfig) SetConfirmBeforeDelete(confirm bool) {
	c.confirmBeforeDelete = confirm
}

// SetEscalateOnConfidenceBelow sets the confidence threshold for escalation.
// Investigations with confidence below this value should escalate to humans.
// Returns ErrInvalidConfidenceThreshold if the value is outside [0.0, 1.0].
func (c *InvestigationConfig) SetEscalateOnConfidenceBelow(threshold float64) error {
	if threshold < 0.0 || threshold > 1.0 {
		return ErrInvalidConfidenceThreshold
	}
	c.escalateOnConfidenceBelow = threshold
	return nil
}

// SetEscalateOnMultipleErrors sets the consecutive error count for escalation.
// Returns ErrInvalidErrorThreshold if the count is zero or negative.
func (c *InvestigationConfig) SetEscalateOnMultipleErrors(count int) error {
	if count <= 0 {
		return ErrInvalidErrorThreshold
	}
	c.escalateOnMultipleErrors = count
	return nil
}

// IsToolAllowed checks if a tool name is in the allowed list.
// Returns false if the tool is not explicitly allowed.
func (c *InvestigationConfig) IsToolAllowed(tool string) bool {
	for _, t := range c.allowedTools {
		if t == tool {
			return true
		}
	}
	return false
}

// IsCommandBlocked checks if a command contains any blocked pattern.
// Uses substring matching - any command containing a blocked pattern is rejected.
func (c *InvestigationConfig) IsCommandBlocked(cmd string) bool {
	for _, blocked := range c.blockedCommands {
		if strings.Contains(cmd, blocked) {
			return true
		}
	}
	return false
}

// IsDirectoryAllowed checks if a directory is in the allowed list.
// An empty or nil allowedDirectories list means all directories are allowed.
func (c *InvestigationConfig) IsDirectoryAllowed(dir string) bool {
	// Empty list means all directories are allowed
	if len(c.allowedDirectories) == 0 {
		return true
	}
	for _, allowed := range c.allowedDirectories {
		if strings.HasPrefix(dir, allowed) {
			return true
		}
	}
	return false
}

// RequiresHumanApproval checks if a command contains patterns that require approval.
// Returns false if no approval patterns are configured.
func (c *InvestigationConfig) RequiresHumanApproval(cmd string) bool {
	if c.requireHumanApprovalPatterns == nil {
		return false
	}
	for _, pattern := range c.requireHumanApprovalPatterns {
		if strings.Contains(cmd, pattern) {
			return true
		}
	}
	return false
}

// Validate checks if the config has valid values for all required fields.
// Returns the first validation error encountered, or nil if valid.
func (c *InvestigationConfig) Validate() error {
	if c.maxActions <= 0 {
		return ErrInvalidMaxActions
	}
	if c.maxDuration <= 0 {
		return ErrInvalidMaxDuration
	}
	if c.maxConcurrent <= 0 {
		return ErrInvalidMaxConcurrent
	}
	if len(c.allowedTools) == 0 {
		return ErrEmptyAllowedTools
	}
	return nil
}

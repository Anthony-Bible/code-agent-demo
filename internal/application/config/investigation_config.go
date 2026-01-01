// Package config provides configuration types for the application layer.
package config

import (
	"errors"
	"strings"
	"time"
)

// Sentinel errors for InvestigationConfig validation.
var (
	ErrInvalidMaxActions             = errors.New("max actions must be positive")
	ErrInvalidMaxDuration            = errors.New("max duration must be positive")
	ErrInvalidMaxConcurrent          = errors.New("max concurrent investigations must be positive")
	ErrInvalidConfidenceThreshold    = errors.New("confidence threshold must be between 0.0 and 1.0")
	ErrInvalidErrorThreshold         = errors.New("error threshold must be positive")
	ErrEmptyAllowedTools             = errors.New("allowed tools list cannot be empty")
	ErrBlockedCommandContainsAllowed = errors.New("blocked command overlaps with allowed tools")
)

// InvestigationConfig holds safety and operational limits for investigations.
type InvestigationConfig struct {
	maxActions                   int
	maxDuration                  time.Duration
	maxConcurrent                int
	allowedTools                 []string
	blockedCommands              []string
	allowedDirectories           []string
	requireHumanApprovalPatterns []string
	confirmBeforeRestart         bool
	confirmBeforeDelete          bool
	escalateOnConfidenceBelow    float64
	escalateOnMultipleErrors     int
}

// NewInvestigationConfig creates a new empty InvestigationConfig.
func NewInvestigationConfig() *InvestigationConfig {
	return &InvestigationConfig{}
}

// DefaultInvestigationConfig returns a config with sensible defaults.
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

// MaxActionsPerInvestigation returns the max actions limit.
func (c *InvestigationConfig) MaxActionsPerInvestigation() int {
	return c.maxActions
}

// MaxDuration returns the max duration limit.
func (c *InvestigationConfig) MaxDuration() time.Duration {
	return c.maxDuration
}

// MaxConcurrentInvestigations returns the max concurrent limit.
func (c *InvestigationConfig) MaxConcurrentInvestigations() int {
	return c.maxConcurrent
}

// AllowedTools returns the list of allowed tools.
func (c *InvestigationConfig) AllowedTools() []string {
	return c.allowedTools
}

// BlockedCommands returns the list of blocked commands.
func (c *InvestigationConfig) BlockedCommands() []string {
	return c.blockedCommands
}

// AllowedDirectories returns the list of allowed directories.
func (c *InvestigationConfig) AllowedDirectories() []string {
	return c.allowedDirectories
}

// RequireHumanApprovalPatterns returns patterns requiring approval.
func (c *InvestigationConfig) RequireHumanApprovalPatterns() []string {
	return c.requireHumanApprovalPatterns
}

// ConfirmBeforeRestart returns whether restart requires confirmation.
func (c *InvestigationConfig) ConfirmBeforeRestart() bool {
	return c.confirmBeforeRestart
}

// ConfirmBeforeDelete returns whether delete requires confirmation.
func (c *InvestigationConfig) ConfirmBeforeDelete() bool {
	return c.confirmBeforeDelete
}

// EscalateOnConfidenceBelow returns the confidence threshold for escalation.
func (c *InvestigationConfig) EscalateOnConfidenceBelow() float64 {
	return c.escalateOnConfidenceBelow
}

// EscalateOnMultipleErrors returns the error count threshold for escalation.
func (c *InvestigationConfig) EscalateOnMultipleErrors() int {
	return c.escalateOnMultipleErrors
}

// SetMaxActions sets the max actions limit.
func (c *InvestigationConfig) SetMaxActions(max int) error {
	if max <= 0 {
		return ErrInvalidMaxActions
	}
	c.maxActions = max
	return nil
}

// SetMaxDuration sets the max duration limit.
func (c *InvestigationConfig) SetMaxDuration(d time.Duration) error {
	if d <= 0 {
		return ErrInvalidMaxDuration
	}
	c.maxDuration = d
	return nil
}

// SetMaxConcurrent sets the max concurrent limit.
func (c *InvestigationConfig) SetMaxConcurrent(max int) error {
	if max <= 0 {
		return ErrInvalidMaxConcurrent
	}
	c.maxConcurrent = max
	return nil
}

// SetAllowedTools sets the list of allowed tools.
func (c *InvestigationConfig) SetAllowedTools(tools []string) error {
	if tools == nil || len(tools) == 0 {
		return ErrEmptyAllowedTools
	}
	c.allowedTools = tools
	return nil
}

// SetBlockedCommands sets the list of blocked commands.
func (c *InvestigationConfig) SetBlockedCommands(commands []string) error {
	c.blockedCommands = commands
	return nil
}

// SetAllowedDirectories sets the list of allowed directories.
func (c *InvestigationConfig) SetAllowedDirectories(dirs []string) {
	c.allowedDirectories = dirs
}

// SetRequireHumanApprovalPatterns sets the patterns requiring approval.
func (c *InvestigationConfig) SetRequireHumanApprovalPatterns(patterns []string) {
	c.requireHumanApprovalPatterns = patterns
}

// SetConfirmBeforeRestart sets whether restart requires confirmation.
func (c *InvestigationConfig) SetConfirmBeforeRestart(confirm bool) {
	c.confirmBeforeRestart = confirm
}

// SetConfirmBeforeDelete sets whether delete requires confirmation.
func (c *InvestigationConfig) SetConfirmBeforeDelete(confirm bool) {
	c.confirmBeforeDelete = confirm
}

// SetEscalateOnConfidenceBelow sets the confidence threshold for escalation.
func (c *InvestigationConfig) SetEscalateOnConfidenceBelow(threshold float64) error {
	if threshold < 0.0 || threshold > 1.0 {
		return ErrInvalidConfidenceThreshold
	}
	c.escalateOnConfidenceBelow = threshold
	return nil
}

// SetEscalateOnMultipleErrors sets the error count threshold for escalation.
func (c *InvestigationConfig) SetEscalateOnMultipleErrors(count int) error {
	if count <= 0 {
		return ErrInvalidErrorThreshold
	}
	c.escalateOnMultipleErrors = count
	return nil
}

// IsToolAllowed checks if a tool is in the allowed list.
func (c *InvestigationConfig) IsToolAllowed(tool string) bool {
	for _, t := range c.allowedTools {
		if t == tool {
			return true
		}
	}
	return false
}

// IsCommandBlocked checks if a command contains a blocked pattern.
func (c *InvestigationConfig) IsCommandBlocked(cmd string) bool {
	for _, blocked := range c.blockedCommands {
		if strings.Contains(cmd, blocked) {
			return true
		}
	}
	return false
}

// IsDirectoryAllowed checks if a directory is in the allowed list.
func (c *InvestigationConfig) IsDirectoryAllowed(dir string) bool {
	// Empty list means all directories are allowed
	if c.allowedDirectories == nil || len(c.allowedDirectories) == 0 {
		return true
	}
	for _, allowed := range c.allowedDirectories {
		if strings.HasPrefix(dir, allowed) {
			return true
		}
	}
	return false
}

// RequiresHumanApproval checks if a command matches approval patterns.
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

// Validate checks if the config is valid.
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
	if c.allowedTools == nil || len(c.allowedTools) == 0 {
		return ErrEmptyAllowedTools
	}
	return nil
}

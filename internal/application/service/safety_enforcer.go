package service

import (
	"code-editing-agent/internal/application/config"
	"code-editing-agent/internal/domain/safety"
	"context"
	"errors"
)

// Sentinel errors for SafetyEnforcer operations.
var (
	// ErrToolBlocked is returned when a tool is not in the allowed list.
	ErrToolBlocked = errors.New("tool not allowed by safety policy")
	// ErrCommandBlocked is returned when a command matches a blocked pattern.
	ErrCommandBlocked = errors.New("command blocked by safety policy")
	// ErrActionBudgetExhausted is returned when the action budget is exhausted.
	ErrActionBudgetExhausted = errors.New("action budget exhausted")
	// ErrInvestigationTimeout is returned when the investigation context is cancelled or timed out.
	ErrInvestigationTimeout = errors.New("investigation timed out")
	// ErrNilConfig is returned when a nil config is passed to the constructor.
	ErrNilConfig = errors.New("config cannot be nil")
	// ErrNilValidator is returned when a nil CommandValidator is passed to the constructor.
	ErrNilValidator = errors.New("command validator cannot be nil")
)

// SafetyEnforcer defines the interface for safety checks during investigations.
type SafetyEnforcer interface {
	// CheckToolAllowed verifies that a tool is permitted.
	CheckToolAllowed(tool string) error
	// CheckCommandAllowed verifies that a command does not match blocked patterns.
	CheckCommandAllowed(cmd string) error
	// CheckActionBudget verifies that the action budget is not exhausted.
	CheckActionBudget(currentActions int) error
	// CheckTimeout verifies that the context has not been cancelled or timed out.
	CheckTimeout(ctx context.Context) error
	// GetMaxActions returns the maximum number of actions allowed per investigation.
	GetMaxActions() int
}

// InvestigationSafetyEnforcer implements SafetyEnforcer using InvestigationConfig.
type InvestigationSafetyEnforcer struct {
	cfg       *config.InvestigationConfig
	validator safety.CommandValidator
}

// NewInvestigationSafetyEnforcer creates a new SafetyEnforcer from an InvestigationConfig.
// Returns ErrNilConfig if cfg is nil.
// Returns a validation error if the config is invalid.
//
// Deprecated: Use NewInvestigationSafetyEnforcerWithValidator instead to enable
// whitelist/blacklist command validation.
func NewInvestigationSafetyEnforcer(cfg *config.InvestigationConfig) (SafetyEnforcer, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &InvestigationSafetyEnforcer{cfg: cfg, validator: nil}, nil
}

// NewInvestigationSafetyEnforcerWithValidator creates a new SafetyEnforcer with CommandValidator.
// The CommandValidator provides whitelist/blacklist-based command validation.
// Returns ErrNilConfig if cfg is nil.
// Returns ErrNilValidator if validator is nil.
// Returns a validation error if the config is invalid.
func NewInvestigationSafetyEnforcerWithValidator(
	cfg *config.InvestigationConfig,
	validator safety.CommandValidator,
) (SafetyEnforcer, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if validator == nil {
		return nil, ErrNilValidator
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &InvestigationSafetyEnforcer{cfg: cfg, validator: validator}, nil
}

// CheckToolAllowed returns ErrToolBlocked if the tool is not in the allowed list.
func (e *InvestigationSafetyEnforcer) CheckToolAllowed(tool string) error {
	if tool == "" || !e.cfg.IsToolAllowed(tool) {
		return ErrToolBlocked
	}
	return nil
}

// CheckCommandAllowed returns ErrCommandBlocked if the command matches a blocked pattern.
// If a CommandValidator is configured, uses whitelist/blacklist validation.
// Otherwise, falls back to simple substring matching against blocked commands.
func (e *InvestigationSafetyEnforcer) CheckCommandAllowed(cmd string) error {
	// If validator is configured, use it for sophisticated whitelist/blacklist validation
	if e.validator != nil {
		result := e.validator.Validate(cmd, false)
		if !result.Allowed {
			return ErrCommandBlocked
		}
		return nil
	}

	// Fallback: legacy substring-based validation for backward compatibility
	// This path is used when SafetyEnforcer is created without a CommandValidator
	for _, blocked := range e.cfg.BlockedCommands() {
		if safety.IsCommandBlocked(cmd, []string{blocked}) {
			return ErrCommandBlocked
		}
	}
	return nil
}

// CheckActionBudget returns ErrActionBudgetExhausted if currentActions >= max actions.
func (e *InvestigationSafetyEnforcer) CheckActionBudget(currentActions int) error {
	if currentActions >= e.cfg.MaxActionsPerInvestigation() {
		return ErrActionBudgetExhausted
	}
	return nil
}

// CheckTimeout returns ErrInvestigationTimeout if the context is cancelled or has expired.
func (e *InvestigationSafetyEnforcer) CheckTimeout(ctx context.Context) error {
	if ctx == nil {
		return ErrInvestigationTimeout
	}
	if err := ctx.Err(); err != nil {
		return ErrInvestigationTimeout
	}
	return nil
}

// GetMaxActions returns the maximum number of actions allowed per investigation.
func (e *InvestigationSafetyEnforcer) GetMaxActions() int {
	return e.cfg.MaxActionsPerInvestigation()
}

package service

import (
	"code-editing-agent/internal/application/config"
	"context"
	"errors"
	"strings"
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
}

// InvestigationSafetyEnforcer implements SafetyEnforcer using InvestigationConfig.
type InvestigationSafetyEnforcer struct {
	cfg *config.InvestigationConfig
}

// NewInvestigationSafetyEnforcer creates a new SafetyEnforcer from an InvestigationConfig.
// Returns ErrNilConfig if cfg is nil.
// Returns a validation error if the config is invalid.
func NewInvestigationSafetyEnforcer(cfg *config.InvestigationConfig) (SafetyEnforcer, error) {
	if cfg == nil {
		return nil, ErrNilConfig
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &InvestigationSafetyEnforcer{cfg: cfg}, nil
}

// CheckToolAllowed returns ErrToolBlocked if the tool is not in the allowed list.
func (e *InvestigationSafetyEnforcer) CheckToolAllowed(tool string) error {
	if tool == "" || !e.cfg.IsToolAllowed(tool) {
		return ErrToolBlocked
	}
	return nil
}

// CheckCommandAllowed returns ErrCommandBlocked if the command matches a blocked pattern.
func (e *InvestigationSafetyEnforcer) CheckCommandAllowed(cmd string) error {
	// Normalize whitespace (tabs, newlines -> spaces) for pattern matching
	normalized := strings.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, cmd)
	for _, blocked := range e.cfg.BlockedCommands() {
		if strings.Contains(normalized, blocked) {
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

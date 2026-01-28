// Package safety provides shared safety-related functionality for command execution.
// This file implements the CommandValidator interface for orchestrating command validation.
package safety

// ValidationResult encapsulates the outcome of command validation.
// It provides a unified result that includes whether the command can execute,
// whether it's flagged as dangerous, and any relevant context.
type ValidationResult struct {
	// Allowed indicates whether the command can execute (subject to confirmation if NeedsConfirm is true).
	Allowed bool

	// IsDangerous indicates whether the command was flagged as dangerous.
	// This is true if either pattern detection or LLM assessment flagged it.
	IsDangerous bool

	// Reason provides context for why the command was blocked or flagged as dangerous.
	// Empty string if the command is allowed and not dangerous.
	Reason string

	// NeedsConfirm indicates whether user confirmation is required before execution.
	// This is true for dangerous commands and non-whitelisted commands when askLLMOnUnknown is true.
	NeedsConfirm bool
}

// CommandValidator orchestrates command validation by combining whitelist checks
// and dangerous command pattern detection.
//
// This interface abstracts the validation logic from the ExecutorAdapter,
// enabling easier testing and clearer separation of concerns.
type CommandValidator interface {
	// Validate checks a command and returns the validation result.
	// The llmDangerous parameter indicates whether the LLM assessed the command as dangerous.
	//
	// Validation behavior depends on the configured mode:
	// - Whitelist mode: Only whitelisted commands are allowed without confirmation.
	//   Non-whitelisted commands require confirmation if askLLMOnUnknown is true.
	// - Blacklist mode: Commands are allowed unless they match dangerous patterns
	//   or the LLM flagged them as dangerous.
	Validate(command string, llmDangerous bool) ValidationResult
}

// CommandValidatorImpl is the standard implementation of CommandValidator.
// It combines whitelist checking and dangerous pattern detection.
type CommandValidatorImpl struct {
	mode            CommandValidationMode
	whitelist       CommandAllowChecker // uses interface for testability
	askLLMOnUnknown bool
}

// NewCommandValidator creates a new CommandValidatorImpl.
//
// Parameters:
//   - mode: The validation mode (ModeBlacklist or ModeWhitelist)
//   - whitelist: The whitelist checker (can be nil for blacklist mode)
//   - askLLMOnUnknown: Whether to ask for confirmation on non-whitelisted commands (whitelist mode only)
//
// Returns an error if whitelist mode is specified but no whitelist is provided.
func NewCommandValidator(
	mode CommandValidationMode,
	whitelist CommandAllowChecker,
	askLLMOnUnknown bool,
) (*CommandValidatorImpl, error) {
	if mode == ModeWhitelist && whitelist == nil {
		return nil, ErrWhitelistRequired
	}
	return &CommandValidatorImpl{
		mode:            mode,
		whitelist:       whitelist,
		askLLMOnUnknown: askLLMOnUnknown,
	}, nil
}

// Validate implements CommandValidator.Validate.
func (v *CommandValidatorImpl) Validate(command string, llmDangerous bool) ValidationResult {
	// Check whitelist mode first
	if v.mode == ModeWhitelist && v.whitelist != nil {
		return v.validateWhitelistMode(command, llmDangerous)
	}

	// Blacklist mode: check dangerous patterns and LLM assessment
	return v.validateBlacklistMode(command, llmDangerous)
}

// validateWhitelistMode handles validation when in whitelist mode.
func (v *CommandValidatorImpl) validateWhitelistMode(command string, llmDangerous bool) ValidationResult {
	// Check if command is whitelisted
	if allowed, _ := v.whitelist.IsAllowedWithPipes(command); allowed {
		// Whitelisted commands execute without confirmation
		return ValidationResult{
			Allowed:      true,
			IsDangerous:  false,
			Reason:       "",
			NeedsConfirm: false,
		}
	}

	// Command is not whitelisted
	if !v.askLLMOnUnknown {
		// Strict whitelist mode: block non-whitelisted commands
		return ValidationResult{
			Allowed:      false,
			IsDangerous:  false,
			Reason:       "not on whitelist",
			NeedsConfirm: false,
		}
	}

	// askLLMOnUnknown is true: determine danger level and require confirmation
	isDangerous, reason := v.evaluateDanger(command, llmDangerous)
	if reason == "" {
		reason = "not on whitelist"
	}

	return ValidationResult{
		Allowed:      true, // Can execute if confirmed
		IsDangerous:  isDangerous,
		Reason:       reason,
		NeedsConfirm: true,
	}
}

// validateBlacklistMode handles validation when in blacklist mode.
func (v *CommandValidatorImpl) validateBlacklistMode(command string, llmDangerous bool) ValidationResult {
	isDangerous, reason := v.evaluateDanger(command, llmDangerous)

	if isDangerous {
		// Dangerous command: require confirmation
		return ValidationResult{
			Allowed:      true, // Can execute if confirmed
			IsDangerous:  true,
			Reason:       reason,
			NeedsConfirm: true,
		}
	}

	// Safe command: execute without confirmation
	return ValidationResult{
		Allowed:      true,
		IsDangerous:  false,
		Reason:       "",
		NeedsConfirm: false,
	}
}

// evaluateDanger checks if a command is dangerous using pattern detection and LLM assessment.
// Returns (isDangerous, reason).
func (v *CommandValidatorImpl) evaluateDanger(command string, llmDangerous bool) (bool, string) {
	patternDangerous, patternReason := IsDangerousCommand(command)

	if patternDangerous && !llmDangerous {
		// Pattern caught it, LLM missed it
		return true, patternReason + " " + ErrMsgLLMFailedToDetect
	}

	if llmDangerous && !patternDangerous {
		// LLM caught it, pattern didn't
		return true, ErrMsgMarkedDangerousByAI
	}

	if patternDangerous {
		// Both caught it (or just pattern)
		return true, patternReason
	}

	// Neither flagged it as dangerous
	return false, ""
}

// Mode returns the validation mode.
func (v *CommandValidatorImpl) Mode() CommandValidationMode {
	return v.mode
}

// AskLLMOnUnknown returns whether to ask for confirmation on non-whitelisted commands.
// This is a test-only accessor for verifying constructor behavior.
func (v *CommandValidatorImpl) AskLLMOnUnknown() bool {
	return v.askLLMOnUnknown
}

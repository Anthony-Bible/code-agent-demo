package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/safety"
)

// bashInput represents the input for the bash tool.
type bashInput struct {
	Command     string `json:"command"`
	Description string `json:"description,omitempty"`
	TimeoutMs   int    `json:"timeout_ms,omitempty"`
	Dangerous   bool   `json:"dangerous,omitempty"`
}

// bashOutput represents the output from the bash tool.
type bashOutput struct {
	Stdout   string `json:"stdout"`
	Stderr   string `json:"stderr"`
	ExitCode int    `json:"exit_code"`
}

// defaultBashTimeout is the default timeout for bash command execution.
const defaultBashTimeout = 30 * time.Second

// registerBashTool registers the bash tool.
func (a *ExecutorAdapter) registerBashTool() {
	bashTool := entity.Tool{
		ID:          toolNameBash,
		Name:        toolNameBash,
		Description: "Executes shell commands and returns stdout, stderr, and exit code. You MUST assess whether each command is dangerous and set the dangerous field accordingly. Dangerous commands require user confirmation.",
		Strict:      true,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"command": map[string]interface{}{
					"type":        "string",
					"description": "The shell command to execute",
				},
				"description": map[string]interface{}{
					"type":        "string",
					"description": "A brief description of what this command does and why it's being run",
				},
				"timeout_ms": map[string]interface{}{
					"type":        "integer",
					"description": "Timeout in milliseconds (default: 30000)",
				},
				"dangerous": map[string]interface{}{
					"type":        "boolean",
					"description": "REQUIRED: You must assess if this command is potentially dangerous. Set to true for commands that: delete/modify files (rm, mv), use elevated privileges (sudo, su), modify system config, execute untrusted input, or could cause data loss. Set to false for safe read-only commands (ls, cat, grep, echo).",
				},
			},
			"required": []string{"command", "dangerous"},
		},
		RequiredFields: []string{"command", "dangerous"},
	}
	a.tools[bashTool.Name] = bashTool
}

// checkCommandConfirmation checks if a command should be allowed to execute.
// The llmDangerous parameter indicates whether the LLM assessed the command as dangerous.
// Uses the CommandValidator to determine whether to allow, block, or confirm the command.
func (a *ExecutorAdapter) checkCommandConfirmation(command string, description string, llmDangerous bool) error {
	// Initialize validator exactly once using sync.Once to prevent race conditions.
	// This ensures the validator is immutable after first use, even if SetValidationMode
	// is called concurrently (which would be a programming error, but we handle it safely).
	a.validatorOnce.Do(func() {
		a.mu.Lock()
		defer a.mu.Unlock()

		// Use pending configuration if SetValidationMode was called, otherwise use defaults
		if a.validatorConfigured {
			// Error can only occur if whitelist mode without whitelist, which SetValidationMode prevents
			a.commandValidator, _ = safety.NewCommandValidator(
				a.pendingValidatorMode,
				a.pendingWhitelist,
				a.pendingAskLLM,
			)
		} else {
			// Default to blacklist mode - error is ignored because blacklist with nil whitelist is always valid
			a.commandValidator, _ = safety.NewCommandValidator(safety.ModeBlacklist, nil, false)
		}
	})

	// Read validator and callback under lock (validator is now immutable, but callback may change)
	a.mu.RLock()
	validator := a.commandValidator
	confirmCallback := a.commandConfirmationCallback
	a.mu.RUnlock()

	// Validate outside lock (validator is immutable after creation)
	result := validator.Validate(command, llmDangerous)

	// Determine whitelist mode from validator's actual mode, not from stale nil check
	// This prevents TOCTOU race where another thread could change the validator
	validatorImpl, isImpl := validator.(*safety.CommandValidatorImpl)
	isWhitelistMode := isImpl && validatorImpl.Mode() == safety.ModeWhitelist

	// If not allowed and doesn't need confirmation, it's a hard block (whitelist strict mode)
	if !result.Allowed && !result.NeedsConfirm {
		return fmt.Errorf(safety.ErrFmtWhitelistBlocked, command)
	}

	// In whitelist mode: whitelisted commands bypass the callback entirely
	// In blacklist mode: safe commands should still go through the callback
	isWhitelisted := result.Allowed && !result.NeedsConfirm && !result.IsDangerous

	if isWhitelistMode && isWhitelisted {
		return nil
	}

	// If CommandConfirmationCallback is set, call it
	// In blacklist mode, this is called for ALL commands (even safe ones)
	// isWhitelistFallback indicates this is a non-whitelisted command in whitelist mode
	isWhitelistFallback := isWhitelistMode && !isWhitelisted
	if confirmCallback != nil {
		return a.handleWithConfirmCallback(command, description, result, confirmCallback, isWhitelistFallback)
	}

	// No general callback - block dangerous commands, allow safe ones
	if result.IsDangerous {
		return fmt.Errorf(safety.ErrFmtDangerousBlocked, result.Reason, command)
	}

	return nil
}

// handleWithConfirmCallback processes confirmation using the general confirmation callback.
func (a *ExecutorAdapter) handleWithConfirmCallback(
	command, description string,
	result safety.ValidationResult,
	callback CommandConfirmationCallback,
	isWhitelistFallback bool,
) error {
	if callback(command, result.IsDangerous, result.Reason, description) {
		return nil
	}
	return a.buildDeniedError(command, result, isWhitelistFallback)
}

// buildDeniedError constructs the appropriate denial error based on context.
func (a *ExecutorAdapter) buildDeniedError(
	command string,
	result safety.ValidationResult,
	isWhitelistFallback bool,
) error {
	if isWhitelistFallback {
		return fmt.Errorf(safety.ErrFmtWhitelistDenied, command)
	}
	if result.IsDangerous {
		return fmt.Errorf(safety.ErrFmtDangerousDenied, result.Reason, command)
	}
	return fmt.Errorf(safety.ErrFmtCommandDenied, command)
}

// executeBash executes a bash command and returns the output.
func (a *ExecutorAdapter) executeBash(ctx context.Context, input json.RawMessage) (string, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to unmarshal bash input: %w", err)
	}

	if in.Command == "" {
		return "", errors.New("command is required")
	}

	// Check command confirmation
	if err := a.checkCommandConfirmation(in.Command, in.Description, in.Dangerous); err != nil {
		return "", err
	}

	// Set timeout
	timeout := defaultBashTimeout
	if in.TimeoutMs > 0 {
		timeout = time.Duration(in.TimeoutMs) * time.Millisecond
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	//nolint:gosec // G204: This is intentionally executing user-provided commands (bash tool)
	cmd := exec.CommandContext(
		ctx,
		"bash",
		"-c",
		in.Command,
	)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := bashOutput{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: 0,
	}

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("command timeout after %v", timeout)
		}
		// Get exit code from error
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			output.ExitCode = exitErr.ExitCode()
		} else {
			return "", fmt.Errorf("failed to execute command: %w", err)
		}
	}

	result, err := json.Marshal(output)
	if err != nil {
		return "", fmt.Errorf("failed to marshal output: %w", err)
	}

	return string(result), nil
}

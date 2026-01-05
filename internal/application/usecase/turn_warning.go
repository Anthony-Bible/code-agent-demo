package usecase

import "fmt"

// TurnWarningConfig configures turn warning behavior.
type TurnWarningConfig struct {
	WarningThreshold int    // When to start warnings (default: 5)
	BatchToolHint    string // Optional tool name to suggest for efficiency
}

// DefaultTurnWarningConfig returns default configuration.
func DefaultTurnWarningConfig() TurnWarningConfig {
	return TurnWarningConfig{
		WarningThreshold: 5,
	}
}

// BuildTurnWarningMessage generates a warning message based on remaining actions.
// Returns empty string if no warning should be displayed.
//
// Warning behavior:
//   - At WarningThreshold: Detailed warning with prioritization advice
//   - At 2 to (WarningThreshold-1): Simple countdown warning
//   - At 1: Final turn warning
//   - Otherwise: No warning (empty string)
//
// The BatchToolHint, if provided, is only included in the threshold warning
// to suggest efficient multi-operation execution.
func BuildTurnWarningMessage(remaining int, cfg TurnWarningConfig) string {
	threshold := cfg.WarningThreshold
	if threshold == 0 {
		threshold = 5
	}

	if remaining == threshold {
		msg := fmt.Sprintf(
			"TURN LIMIT WARNING: You have %d turns remaining before reaching the turn limit.\n\nPlease prioritize your remaining actions carefully.",
			threshold,
		)
		if cfg.BatchToolHint != "" {
			msg += fmt.Sprintf(
				" Consider using the %s to execute multiple operations efficiently in a single turn.",
				cfg.BatchToolHint,
			)
		}
		return msg
	}
	if remaining == 1 {
		return "TURN LIMIT WARNING: You have 1 turn remaining."
	}
	if remaining >= 2 && remaining < threshold {
		return fmt.Sprintf("TURN LIMIT WARNING: You have %d turns remaining.", remaining)
	}
	return ""
}

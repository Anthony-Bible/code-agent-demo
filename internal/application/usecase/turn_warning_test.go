package usecase

import (
	"strings"
	"testing"
)

func TestBuildTurnWarningMessage(t *testing.T) {
	tests := []struct {
		name      string
		remaining int
		config    TurnWarningConfig
		want      string
		wantEmpty bool
	}{
		{
			name:      "warning at default threshold (5 turns remaining)",
			remaining: 5,
			config:    DefaultTurnWarningConfig(),
			want:      "TURN LIMIT WARNING: You have 5 turns remaining",
		},
		{
			name:      "warning at 4 turns remaining",
			remaining: 4,
			config:    DefaultTurnWarningConfig(),
			want:      "TURN LIMIT WARNING: You have 4 turns remaining.",
		},
		{
			name:      "warning at 3 turns remaining",
			remaining: 3,
			config:    DefaultTurnWarningConfig(),
			want:      "TURN LIMIT WARNING: You have 3 turns remaining.",
		},
		{
			name:      "warning at 2 turns remaining",
			remaining: 2,
			config:    DefaultTurnWarningConfig(),
			want:      "TURN LIMIT WARNING: You have 2 turns remaining.",
		},
		{
			name:      "warning at 1 turn remaining",
			remaining: 1,
			config:    DefaultTurnWarningConfig(),
			want:      "TURN LIMIT WARNING: You have 1 turn remaining.",
		},
		{
			name:      "no warning at 6 turns remaining",
			remaining: 6,
			config:    DefaultTurnWarningConfig(),
			wantEmpty: true,
		},
		{
			name:      "no warning at 10 turns remaining",
			remaining: 10,
			config:    DefaultTurnWarningConfig(),
			wantEmpty: true,
		},
		{
			name:      "no warning at 0 turns remaining",
			remaining: 0,
			config:    DefaultTurnWarningConfig(),
			wantEmpty: true,
		},
		{
			name:      "custom threshold at 3",
			remaining: 3,
			config: TurnWarningConfig{
				WarningThreshold: 3,
			},
			want: "TURN LIMIT WARNING: You have 3 turns remaining before reaching the turn limit",
		},
		{
			name:      "custom threshold no warning at 4",
			remaining: 4,
			config: TurnWarningConfig{
				WarningThreshold: 3,
			},
			wantEmpty: true,
		},
		{
			name:      "batch tool hint included at threshold",
			remaining: 5,
			config: TurnWarningConfig{
				WarningThreshold: 5,
				BatchToolHint:    "batch_tool",
			},
			want: "batch_tool",
		},
		{
			name:      "batch tool hint not included below threshold",
			remaining: 3,
			config: TurnWarningConfig{
				WarningThreshold: 5,
				BatchToolHint:    "batch_tool",
			},
			want: "TURN LIMIT WARNING: You have 3 turns remaining.",
		},
		{
			name:      "zero threshold defaults to 5",
			remaining: 5,
			config: TurnWarningConfig{
				WarningThreshold: 0,
			},
			want: "TURN LIMIT WARNING: You have 5 turns remaining",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildTurnWarningMessage(tt.remaining, tt.config)

			if tt.wantEmpty {
				if got != "" {
					t.Errorf("BuildTurnWarningMessage() = %q, want empty string", got)
				}
				return
			}

			if !strings.Contains(got, tt.want) {
				t.Errorf("BuildTurnWarningMessage() = %q, want to contain %q", got, tt.want)
			}
		})
	}
}

func TestBuildTurnWarningMessage_DetailedMessages(t *testing.T) {
	tests := []struct {
		name            string
		remaining       int
		config          TurnWarningConfig
		wantContains    []string
		wantNotContains []string
	}{
		{
			name:      "threshold warning includes prioritization message",
			remaining: 5,
			config:    DefaultTurnWarningConfig(),
			wantContains: []string{
				"TURN LIMIT WARNING",
				"5 turns remaining",
				"Please prioritize your remaining actions carefully",
			},
		},
		{
			name:      "threshold warning with batch tool hint",
			remaining: 5,
			config: TurnWarningConfig{
				WarningThreshold: 5,
				BatchToolHint:    "batch_tool",
			},
			wantContains: []string{
				"TURN LIMIT WARNING",
				"5 turns remaining",
				"batch_tool",
				"execute multiple operations efficiently",
			},
		},
		{
			name:      "threshold warning without batch tool hint",
			remaining: 5,
			config: TurnWarningConfig{
				WarningThreshold: 5,
			},
			wantContains: []string{
				"TURN LIMIT WARNING",
				"5 turns remaining",
			},
			wantNotContains: []string{
				"batch_tool",
				"execute multiple operations",
			},
		},
		{
			name:      "below threshold warning is simple",
			remaining: 3,
			config:    DefaultTurnWarningConfig(),
			wantContains: []string{
				"TURN LIMIT WARNING",
				"3 turns remaining.",
			},
			wantNotContains: []string{
				"Please prioritize",
				"batch_tool",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildTurnWarningMessage(tt.remaining, tt.config)

			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("BuildTurnWarningMessage() = %q, want to contain %q", got, want)
				}
			}

			for _, notWant := range tt.wantNotContains {
				if strings.Contains(got, notWant) {
					t.Errorf("BuildTurnWarningMessage() = %q, should NOT contain %q", got, notWant)
				}
			}
		})
	}
}

func TestDefaultTurnWarningConfig(t *testing.T) {
	cfg := DefaultTurnWarningConfig()

	if cfg.WarningThreshold != 5 {
		t.Errorf("DefaultTurnWarningConfig().WarningThreshold = %d, want 5", cfg.WarningThreshold)
	}

	if cfg.BatchToolHint != "" {
		t.Errorf("DefaultTurnWarningConfig().BatchToolHint = %q, want empty string", cfg.BatchToolHint)
	}
}

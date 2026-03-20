package port

import (
	"context"
	"testing"
)

// TestWithThinkingMode_SetAndRetrieve verifies that ThinkingModeInfo
// can be stored in context and retrieved correctly.
func TestWithThinkingMode_SetAndRetrieve(t *testing.T) {
	tests := []struct {
		name      string
		info      ThinkingModeInfo
		expectOk  bool
		expectVal ThinkingModeInfo
	}{
		{
			name: "valid thinking mode with all fields set",
			info: ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 4096,
				ShowThinking: true,
			},
			expectOk: true,
			expectVal: ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 4096,
				ShowThinking: true,
			},
		},
		{
			name: "thinking enabled but show thinking disabled",
			info: ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 2048,
				ShowThinking: false,
			},
			expectOk: true,
			expectVal: ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 2048,
				ShowThinking: false,
			},
		},
		{
			name: "thinking disabled with budget still set",
			info: ThinkingModeInfo{
				Enabled:      false,
				BudgetTokens: 8192,
				ShowThinking: false,
			},
			expectOk: true,
			expectVal: ThinkingModeInfo{
				Enabled:      false,
				BudgetTokens: 8192,
				ShowThinking: false,
			},
		},
		{
			name: "minimum budget tokens (1024)",
			info: ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 1024,
				ShowThinking: true,
			},
			expectOk: true,
			expectVal: ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 1024,
				ShowThinking: true,
			},
		},
		{
			name: "large budget tokens",
			info: ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 100000,
				ShowThinking: false,
			},
			expectOk: true,
			expectVal: ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: 100000,
				ShowThinking: false,
			},
		},
		{
			name: "zero values for all fields",
			info: ThinkingModeInfo{
				Enabled:      false,
				BudgetTokens: 0,
				ShowThinking: false,
			},
			expectOk: true,
			expectVal: ThinkingModeInfo{
				Enabled:      false,
				BudgetTokens: 0,
				ShowThinking: false,
			},
		},
		{
			name: "only show thinking enabled",
			info: ThinkingModeInfo{
				Enabled:      false,
				BudgetTokens: 0,
				ShowThinking: true,
			},
			expectOk: true,
			expectVal: ThinkingModeInfo{
				Enabled:      false,
				BudgetTokens: 0,
				ShowThinking: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Set the thinking mode in context
			ctxWithThinking := WithThinkingMode(ctx, tt.info)

			// Retrieve the thinking mode from context
			retrievedInfo, ok := ThinkingModeFromContext(ctxWithThinking)

			// Verify ok matches expectation
			if ok != tt.expectOk {
				t.Errorf("expected ok=%v, got ok=%v", tt.expectOk, ok)
			}

			// Verify retrieved info matches expected
			if ok {
				if retrievedInfo.Enabled != tt.expectVal.Enabled {
					t.Errorf("expected Enabled=%v, got Enabled=%v", tt.expectVal.Enabled, retrievedInfo.Enabled)
				}
				if retrievedInfo.BudgetTokens != tt.expectVal.BudgetTokens {
					t.Errorf(
						"expected BudgetTokens=%d, got BudgetTokens=%d",
						tt.expectVal.BudgetTokens,
						retrievedInfo.BudgetTokens,
					)
				}
				if retrievedInfo.ShowThinking != tt.expectVal.ShowThinking {
					t.Errorf(
						"expected ShowThinking=%v, got ShowThinking=%v",
						tt.expectVal.ShowThinking,
						retrievedInfo.ShowThinking,
					)
				}
			}
		})
	}
}

// TestThinkingModeFromContext_Missing verifies that retrieving thinking mode
// from a context without one returns false and zero value.
func TestThinkingModeFromContext_Missing(t *testing.T) {
	tests := []struct {
		name               string
		ctx                context.Context
		expectOk           bool
		expectEnabled      bool
		expectBudgetTokens int64
		expectShowThinking bool
	}{
		{
			name:               "plain background context",
			ctx:                context.Background(),
			expectOk:           false,
			expectEnabled:      false,
			expectBudgetTokens: 0,
			expectShowThinking: false,
		},
		{
			name:               "context with session ID but no thinking mode",
			ctx:                WithSessionID(context.Background(), "some-session"),
			expectOk:           false,
			expectEnabled:      false,
			expectBudgetTokens: 0,
			expectShowThinking: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrievedInfo, ok := ThinkingModeFromContext(tt.ctx)

			if ok != tt.expectOk {
				t.Errorf("expected ok=%v, got ok=%v", tt.expectOk, ok)
			}

			// Should return zero value when not found
			if retrievedInfo.Enabled != tt.expectEnabled {
				t.Errorf("expected zero-value Enabled=%v, got Enabled=%v", tt.expectEnabled, retrievedInfo.Enabled)
			}
			if retrievedInfo.BudgetTokens != tt.expectBudgetTokens {
				t.Errorf(
					"expected zero-value BudgetTokens=%d, got BudgetTokens=%d",
					tt.expectBudgetTokens,
					retrievedInfo.BudgetTokens,
				)
			}
			if retrievedInfo.ShowThinking != tt.expectShowThinking {
				t.Errorf(
					"expected zero-value ShowThinking=%v, got ShowThinking=%v",
					tt.expectShowThinking,
					retrievedInfo.ShowThinking,
				)
			}
		})
	}
}

// TestThinkingMode_ContextChaining verifies that thinking mode context
// works correctly when chained with other context values.
func TestThinkingMode_ContextChaining(t *testing.T) {
	tests := []struct {
		name               string
		setupContext       func() context.Context
		expectThinkingOk   bool
		expectEnabled      bool
		expectBudgetTokens int64
		expectShowThinking bool
		expectSessionOk    bool
		expectSessionID    string
	}{
		{
			name: "thinking mode and session ID",
			setupContext: func() context.Context {
				ctx := context.Background()
				ctx = WithSessionID(ctx, "thinking-session")
				ctx = WithThinkingMode(ctx, ThinkingModeInfo{
					Enabled:      true,
					BudgetTokens: 4096,
					ShowThinking: true,
				})
				return ctx
			},
			expectThinkingOk:   true,
			expectEnabled:      true,
			expectBudgetTokens: 4096,
			expectShowThinking: true,
			expectSessionOk:    true,
			expectSessionID:    "thinking-session",
		},
		{
			name: "thinking mode added first",
			setupContext: func() context.Context {
				ctx := context.Background()
				ctx = WithThinkingMode(ctx, ThinkingModeInfo{
					Enabled:      false,
					BudgetTokens: 2048,
					ShowThinking: true,
				})
				ctx = WithSessionID(ctx, "second-session")
				return ctx
			},
			expectThinkingOk:   true,
			expectEnabled:      false,
			expectBudgetTokens: 2048,
			expectShowThinking: true,
			expectSessionOk:    true,
			expectSessionID:    "second-session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupContext()
			verifyThinkingModeContext(
				t,
				ctx,
				tt.expectThinkingOk,
				tt.expectEnabled,
				tt.expectBudgetTokens,
				tt.expectShowThinking,
			)
			verifySessionIDContext(t, ctx, tt.expectSessionOk, tt.expectSessionID)
		})
	}
}

// verifyThinkingModeContext checks if thinking mode matches expectations.
//
//nolint:revive // test helper: t before ctx is conventional
func verifyThinkingModeContext(
	t *testing.T,
	ctx context.Context,
	expectOk bool,
	expectEnabled bool,
	expectBudgetTokens int64,
	expectShowThinking bool,
) {
	t.Helper()
	thinkingInfo, thinkingOk := ThinkingModeFromContext(ctx)
	if thinkingOk != expectOk {
		t.Errorf("expected thinking ok=%v, got ok=%v", expectOk, thinkingOk)
	}
	if expectOk {
		if thinkingInfo.Enabled != expectEnabled {
			t.Errorf("expected Enabled=%v, got Enabled=%v", expectEnabled, thinkingInfo.Enabled)
		}
		if thinkingInfo.BudgetTokens != expectBudgetTokens {
			t.Errorf("expected BudgetTokens=%d, got BudgetTokens=%d", expectBudgetTokens, thinkingInfo.BudgetTokens)
		}
		if thinkingInfo.ShowThinking != expectShowThinking {
			t.Errorf("expected ShowThinking=%v, got ShowThinking=%v", expectShowThinking, thinkingInfo.ShowThinking)
		}
	}
}

// verifySessionIDContext checks if session ID matches expectations.
//
//nolint:revive // test helper: t before ctx is conventional
func verifySessionIDContext(t *testing.T, ctx context.Context, expectOk bool, expectSessionID string) {
	t.Helper()
	sessionID, sessionOk := SessionIDFromContext(ctx)
	if sessionOk != expectOk {
		t.Errorf("expected session ok=%v, got ok=%v", expectOk, sessionOk)
	}
	if expectOk && sessionID != expectSessionID {
		t.Errorf("expected sessionID=%q, got sessionID=%q", expectSessionID, sessionID)
	}
}

// TestThinkingMode_Overwrite verifies that setting a new thinking mode
// overwrites the previous value.
func TestThinkingMode_Overwrite(t *testing.T) {
	ctx := context.Background()

	// Set first thinking mode
	ctx = WithThinkingMode(ctx, ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 1024,
		ShowThinking: false,
	})

	// Verify first thinking mode is set
	info, ok := ThinkingModeFromContext(ctx)
	if !ok {
		t.Fatal("expected first thinking mode to be set")
	}
	if !info.Enabled {
		t.Error("expected first thinking mode to be enabled")
	}
	if info.BudgetTokens != 1024 {
		t.Errorf("expected first BudgetTokens=%d, got %d", 1024, info.BudgetTokens)
	}
	if info.ShowThinking {
		t.Error("expected first ShowThinking to be false")
	}

	// Overwrite with second thinking mode
	ctx = WithThinkingMode(ctx, ThinkingModeInfo{
		Enabled:      false,
		BudgetTokens: 16384,
		ShowThinking: true,
	})

	// Verify second thinking mode replaced first
	info, ok = ThinkingModeFromContext(ctx)
	if !ok {
		t.Fatal("expected second thinking mode to be set")
	}
	if info.Enabled {
		t.Error("expected second thinking mode to be disabled")
	}
	if info.BudgetTokens != 16384 {
		t.Errorf("expected second BudgetTokens=%d, got %d", 16384, info.BudgetTokens)
	}
	if !info.ShowThinking {
		t.Error("expected second ShowThinking to be true")
	}
}

// TestThinkingMode_ZeroValueRetrieval verifies that an empty/zero-value
// ThinkingModeInfo can still be stored and retrieved.
func TestThinkingMode_ZeroValueRetrieval(t *testing.T) {
	ctx := context.Background()

	// Set zero-value thinking mode
	zeroInfo := ThinkingModeInfo{}
	ctx = WithThinkingMode(ctx, zeroInfo)

	// Should still be retrievable
	retrievedInfo, ok := ThinkingModeFromContext(ctx)
	if !ok {
		t.Fatal("expected zero-value info to be retrievable")
	}

	if retrievedInfo.Enabled {
		t.Errorf("expected Enabled=false, got %v", retrievedInfo.Enabled)
	}
	if retrievedInfo.BudgetTokens != 0 {
		t.Errorf("expected BudgetTokens=0, got %d", retrievedInfo.BudgetTokens)
	}
	if retrievedInfo.ShowThinking {
		t.Errorf("expected ShowThinking=false, got %v", retrievedInfo.ShowThinking)
	}
}

// TestThinkingMode_ConcurrentContexts verifies that thinking modes
// in different context branches remain independent.
func TestThinkingMode_ConcurrentContexts(t *testing.T) {
	parentCtx := context.Background()

	// Create two independent context branches
	ctx1 := WithThinkingMode(parentCtx, ThinkingModeInfo{
		Enabled:      true,
		BudgetTokens: 2048,
		ShowThinking: true,
	})

	ctx2 := WithThinkingMode(parentCtx, ThinkingModeInfo{
		Enabled:      false,
		BudgetTokens: 8192,
		ShowThinking: false,
	})

	// Verify ctx1 has its own thinking mode
	info1, ok1 := ThinkingModeFromContext(ctx1)
	if !ok1 {
		t.Fatal("expected ctx1 to have thinking mode")
	}
	if !info1.Enabled {
		t.Error("ctx1: expected Enabled=true")
	}
	if info1.BudgetTokens != 2048 {
		t.Errorf("ctx1: expected BudgetTokens=%d, got %d", 2048, info1.BudgetTokens)
	}
	if !info1.ShowThinking {
		t.Error("ctx1: expected ShowThinking=true")
	}

	// Verify ctx2 has its own thinking mode
	info2, ok2 := ThinkingModeFromContext(ctx2)
	if !ok2 {
		t.Fatal("expected ctx2 to have thinking mode")
	}
	if info2.Enabled {
		t.Error("ctx2: expected Enabled=false")
	}
	if info2.BudgetTokens != 8192 {
		t.Errorf("ctx2: expected BudgetTokens=%d, got %d", 8192, info2.BudgetTokens)
	}
	if info2.ShowThinking {
		t.Error("ctx2: expected ShowThinking=false")
	}

	// Verify parent context still has no thinking mode
	_, okParent := ThinkingModeFromContext(parentCtx)
	if okParent {
		t.Error("expected parent context to have no thinking mode")
	}
}

// TestThinkingMode_BudgetTokensEdgeCases verifies handling of various budget token values.
func TestThinkingMode_BudgetTokensEdgeCases(t *testing.T) {
	tests := []struct {
		name         string
		budgetTokens int64
	}{
		{
			name:         "minimum value 1024",
			budgetTokens: 1024,
		},
		{
			name:         "common value 4096",
			budgetTokens: 4096,
		},
		{
			name:         "large value",
			budgetTokens: 1000000,
		},
		{
			name:         "zero value",
			budgetTokens: 0,
		},
		{
			name:         "negative value (edge case)",
			budgetTokens: -100,
		},
		{
			name:         "max int64",
			budgetTokens: 9223372036854775807,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			info := ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: tt.budgetTokens,
				ShowThinking: true,
			}

			ctx = WithThinkingMode(ctx, info)
			retrievedInfo, ok := ThinkingModeFromContext(ctx)

			if !ok {
				t.Fatal("expected thinking mode to be retrievable")
			}

			if retrievedInfo.BudgetTokens != tt.budgetTokens {
				t.Errorf("expected BudgetTokens=%d, got %d", tt.budgetTokens, retrievedInfo.BudgetTokens)
			}
		})
	}
}

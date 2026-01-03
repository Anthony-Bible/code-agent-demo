package port

import (
	"context"
	"testing"
)

// TestWithCustomSystemPrompt_SetAndRetrieve verifies that CustomSystemPromptInfo
// can be stored in context and retrieved correctly.
func TestWithCustomSystemPrompt_SetAndRetrieve(t *testing.T) {
	tests := []struct {
		name      string
		info      CustomSystemPromptInfo
		expectOk  bool
		expectVal CustomSystemPromptInfo
	}{
		{
			name: "valid custom prompt with session ID",
			info: CustomSystemPromptInfo{
				SessionID: "test-session-123",
				Prompt:    "You are a specialized coding assistant for Go development.",
			},
			expectOk: true,
			expectVal: CustomSystemPromptInfo{
				SessionID: "test-session-123",
				Prompt:    "You are a specialized coding assistant for Go development.",
			},
		},
		{
			name: "empty prompt field",
			info: CustomSystemPromptInfo{
				SessionID: "test-session-456",
				Prompt:    "",
			},
			expectOk: true,
			expectVal: CustomSystemPromptInfo{
				SessionID: "test-session-456",
				Prompt:    "",
			},
		},
		{
			name: "empty session ID",
			info: CustomSystemPromptInfo{
				SessionID: "",
				Prompt:    "Custom system instructions here",
			},
			expectOk: true,
			expectVal: CustomSystemPromptInfo{
				SessionID: "",
				Prompt:    "Custom system instructions here",
			},
		},
		{
			name: "both fields empty",
			info: CustomSystemPromptInfo{
				SessionID: "",
				Prompt:    "",
			},
			expectOk: true,
			expectVal: CustomSystemPromptInfo{
				SessionID: "",
				Prompt:    "",
			},
		},
		{
			name: "multiline prompt",
			info: CustomSystemPromptInfo{
				SessionID: "multiline-session",
				Prompt: `You are a specialized assistant.

You should:
- Follow best practices
- Write comprehensive tests
- Use hexagonal architecture`,
			},
			expectOk: true,
			expectVal: CustomSystemPromptInfo{
				SessionID: "multiline-session",
				Prompt: `You are a specialized assistant.

You should:
- Follow best practices
- Write comprehensive tests
- Use hexagonal architecture`,
			},
		},
		{
			name: "prompt with special characters",
			info: CustomSystemPromptInfo{
				SessionID: "special-chars-session",
				Prompt:    "Use \"quotes\", 'apostrophes', and symbols: @#$%^&*()",
			},
			expectOk: true,
			expectVal: CustomSystemPromptInfo{
				SessionID: "special-chars-session",
				Prompt:    "Use \"quotes\", 'apostrophes', and symbols: @#$%^&*()",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Set the custom system prompt in context
			ctxWithPrompt := WithCustomSystemPrompt(ctx, tt.info)

			// Retrieve the custom system prompt from context
			retrievedInfo, ok := CustomSystemPromptFromContext(ctxWithPrompt)

			// Verify ok matches expectation
			if ok != tt.expectOk {
				t.Errorf("expected ok=%v, got ok=%v", tt.expectOk, ok)
			}

			// Verify retrieved info matches expected
			if ok {
				if retrievedInfo.Prompt != tt.expectVal.Prompt {
					t.Errorf("expected Prompt=%q, got Prompt=%q", tt.expectVal.Prompt, retrievedInfo.Prompt)
				}
				if retrievedInfo.SessionID != tt.expectVal.SessionID {
					t.Errorf("expected SessionID=%q, got SessionID=%q", tt.expectVal.SessionID, retrievedInfo.SessionID)
				}
			}
		})
	}
}

// TestCustomSystemPromptFromContext_Missing verifies that retrieving custom prompt
// from a context without one returns false and zero value.
func TestCustomSystemPromptFromContext_Missing(t *testing.T) {
	tests := []struct {
		name         string
		ctx          context.Context
		expectOk     bool
		expectPrompt string
		expectSessID string
	}{
		{
			name:         "plain background context",
			ctx:          context.Background(),
			expectOk:     false,
			expectPrompt: "",
			expectSessID: "",
		},
		{
			name:         "context with session ID but no custom prompt",
			ctx:          WithSessionID(context.Background(), "some-session"),
			expectOk:     false,
			expectPrompt: "",
			expectSessID: "",
		},
		{
			name:         "context with plan mode but no custom prompt",
			ctx:          WithPlanMode(context.Background(), PlanModeInfo{Enabled: true, SessionID: "plan-session"}),
			expectOk:     false,
			expectPrompt: "",
			expectSessID: "",
		},
		{
			name: "context with multiple values but no custom prompt",
			ctx: WithPlanMode(
				WithSessionID(context.Background(), "multi-session"),
				PlanModeInfo{Enabled: false, SessionID: "plan-session"},
			),
			expectOk:     false,
			expectPrompt: "",
			expectSessID: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			retrievedInfo, ok := CustomSystemPromptFromContext(tt.ctx)

			if ok != tt.expectOk {
				t.Errorf("expected ok=%v, got ok=%v", tt.expectOk, ok)
			}

			// Should return zero value when not found
			if retrievedInfo.Prompt != tt.expectPrompt {
				t.Errorf("expected zero-value Prompt=%q, got Prompt=%q", tt.expectPrompt, retrievedInfo.Prompt)
			}
			if retrievedInfo.SessionID != tt.expectSessID {
				t.Errorf("expected zero-value SessionID=%q, got SessionID=%q", tt.expectSessID, retrievedInfo.SessionID)
			}
		})
	}
}

// TestCustomSystemPrompt_ContextChaining verifies that custom prompt context
// works correctly when chained with other context values.
func TestCustomSystemPrompt_ContextChaining(t *testing.T) {
	tests := []struct {
		name             string
		setupContext     func() context.Context
		expectPromptOk   bool
		expectPrompt     string
		expectSessionOk  bool
		expectSessionID  string
		expectPlanModeOk bool
	}{
		{
			name: "custom prompt and session ID",
			setupContext: func() context.Context {
				ctx := context.Background()
				ctx = WithSessionID(ctx, "chained-session")
				ctx = WithCustomSystemPrompt(ctx, CustomSystemPromptInfo{
					SessionID: "prompt-session",
					Prompt:    "Chained prompt",
				})
				return ctx
			},
			expectPromptOk:  true,
			expectPrompt:    "Chained prompt",
			expectSessionOk: true,
			expectSessionID: "chained-session",
		},
		{
			name: "all three context values",
			setupContext: func() context.Context {
				ctx := context.Background()
				ctx = WithSessionID(ctx, "all-session")
				ctx = WithPlanMode(ctx, PlanModeInfo{Enabled: true, SessionID: "plan-session"})
				ctx = WithCustomSystemPrompt(ctx, CustomSystemPromptInfo{
					SessionID: "custom-session",
					Prompt:    "Full context prompt",
				})
				return ctx
			},
			expectPromptOk:   true,
			expectPrompt:     "Full context prompt",
			expectSessionOk:  true,
			expectSessionID:  "all-session",
			expectPlanModeOk: true,
		},
		{
			name: "custom prompt added first",
			setupContext: func() context.Context {
				ctx := context.Background()
				ctx = WithCustomSystemPrompt(ctx, CustomSystemPromptInfo{
					SessionID: "first-session",
					Prompt:    "First added prompt",
				})
				ctx = WithSessionID(ctx, "second-session")
				return ctx
			},
			expectPromptOk:  true,
			expectPrompt:    "First added prompt",
			expectSessionOk: true,
			expectSessionID: "second-session",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupContext()
			verifyCustomPromptContext(t, ctx, tt.expectPromptOk, tt.expectPrompt)
			verifySessionIDContext(t, ctx, tt.expectSessionOk, tt.expectSessionID)
			verifyPlanModeContext(t, ctx, tt.expectPlanModeOk)
		})
	}
}

// verifyCustomPromptContext checks if custom prompt matches expectations.
func verifyCustomPromptContext(t *testing.T, ctx context.Context, expectOk bool, expectPrompt string) {
	t.Helper()
	promptInfo, promptOk := CustomSystemPromptFromContext(ctx)
	if promptOk != expectOk {
		t.Errorf("expected prompt ok=%v, got ok=%v", expectOk, promptOk)
	}
	if expectOk && promptInfo.Prompt != expectPrompt {
		t.Errorf("expected prompt=%q, got prompt=%q", expectPrompt, promptInfo.Prompt)
	}
}

// verifySessionIDContext checks if session ID matches expectations.
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

// verifyPlanModeContext checks if plan mode is present when expected.
func verifyPlanModeContext(t *testing.T, ctx context.Context, expectOk bool) {
	t.Helper()
	if expectOk {
		_, planOk := PlanModeFromContext(ctx)
		if !planOk {
			t.Error("expected plan mode to be present in context")
		}
	}
}

// TestCustomSystemPrompt_Overwrite verifies that setting a new custom prompt
// overwrites the previous value.
func TestCustomSystemPrompt_Overwrite(t *testing.T) {
	ctx := context.Background()

	// Set first custom prompt
	ctx = WithCustomSystemPrompt(ctx, CustomSystemPromptInfo{
		SessionID: "first-session",
		Prompt:    "First prompt",
	})

	// Verify first prompt is set
	info, ok := CustomSystemPromptFromContext(ctx)
	if !ok {
		t.Fatal("expected first prompt to be set")
	}
	if info.Prompt != "First prompt" {
		t.Errorf("expected first prompt=%q, got %q", "First prompt", info.Prompt)
	}

	// Overwrite with second custom prompt
	ctx = WithCustomSystemPrompt(ctx, CustomSystemPromptInfo{
		SessionID: "second-session",
		Prompt:    "Second prompt",
	})

	// Verify second prompt replaced first
	info, ok = CustomSystemPromptFromContext(ctx)
	if !ok {
		t.Fatal("expected second prompt to be set")
	}
	if info.Prompt != "Second prompt" {
		t.Errorf("expected second prompt=%q, got %q", "Second prompt", info.Prompt)
	}
	if info.SessionID != "second-session" {
		t.Errorf("expected second session=%q, got %q", "second-session", info.SessionID)
	}
}

// TestCustomSystemPrompt_ZeroValueRetrieval verifies that an empty/zero-value
// CustomSystemPromptInfo can still be stored and retrieved.
func TestCustomSystemPrompt_ZeroValueRetrieval(t *testing.T) {
	ctx := context.Background()

	// Set zero-value custom prompt
	zeroInfo := CustomSystemPromptInfo{}
	ctx = WithCustomSystemPrompt(ctx, zeroInfo)

	// Should still be retrievable
	retrievedInfo, ok := CustomSystemPromptFromContext(ctx)
	if !ok {
		t.Fatal("expected zero-value info to be retrievable")
	}

	if retrievedInfo.Prompt != "" {
		t.Errorf("expected empty Prompt, got %q", retrievedInfo.Prompt)
	}
	if retrievedInfo.SessionID != "" {
		t.Errorf("expected empty SessionID, got %q", retrievedInfo.SessionID)
	}
}

// TestCustomSystemPrompt_ConcurrentContexts verifies that custom prompts
// in different context branches remain independent.
func TestCustomSystemPrompt_ConcurrentContexts(t *testing.T) {
	parentCtx := context.Background()

	// Create two independent context branches
	ctx1 := WithCustomSystemPrompt(parentCtx, CustomSystemPromptInfo{
		SessionID: "branch-1",
		Prompt:    "Branch 1 prompt",
	})

	ctx2 := WithCustomSystemPrompt(parentCtx, CustomSystemPromptInfo{
		SessionID: "branch-2",
		Prompt:    "Branch 2 prompt",
	})

	// Verify ctx1 has its own prompt
	info1, ok1 := CustomSystemPromptFromContext(ctx1)
	if !ok1 {
		t.Fatal("expected ctx1 to have custom prompt")
	}
	if info1.Prompt != "Branch 1 prompt" {
		t.Errorf("ctx1: expected prompt=%q, got %q", "Branch 1 prompt", info1.Prompt)
	}

	// Verify ctx2 has its own prompt
	info2, ok2 := CustomSystemPromptFromContext(ctx2)
	if !ok2 {
		t.Fatal("expected ctx2 to have custom prompt")
	}
	if info2.Prompt != "Branch 2 prompt" {
		t.Errorf("ctx2: expected prompt=%q, got %q", "Branch 2 prompt", info2.Prompt)
	}

	// Verify parent context still has no prompt
	_, okParent := CustomSystemPromptFromContext(parentCtx)
	if okParent {
		t.Error("expected parent context to have no custom prompt")
	}
}

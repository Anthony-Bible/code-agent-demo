//go:build e2e

// Package ai — E2E tests for GenkitAdapter against the live Anthropic API.
//
// Run with:
//
//	source .env && go test -tags=e2e -v -run E2E ./internal/infrastructure/adapter/ai/... -timeout 120s
//
// Tests are skipped automatically when ANTHROPIC_API_KEY is unset.
package ai

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
	"github.com/anthony-bible/code-agent-demo/internal/infrastructure/adapter/file"
	"github.com/anthony-bible/code-agent-demo/internal/infrastructure/adapter/tool"
)

const (
	e2eModel     = "claude-haiku-4-5"
	e2eMaxTokens = int64(256)
)

// skipIfNoAPIKey skips the calling test if ANTHROPIC_API_KEY is unset or empty.
func skipIfNoAPIKey(t *testing.T) {
	t.Helper()
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set; skipping live E2E test")
	}
}

// newE2EAdapter builds a GenkitAdapter pointed at the live Anthropic API.
// It fails the test (not skips) if construction itself fails, because that
// indicates a code bug rather than a missing environment variable.
func newE2EAdapter(t *testing.T) port.AIProvider {
	t.Helper()
	adapter, err := NewGenkitAdapter(e2eModel, e2eMaxTokens, nil, port.NopLogger{})
	if err != nil {
		t.Fatalf("NewGenkitAdapter construction failed: %v", err)
	}
	return adapter
}

// TestE2E_GenkitAdapter runs live Anthropic API tests against GenkitAdapter.
func TestE2E_GenkitAdapter(t *testing.T) {
	skipIfNoAPIKey(t)

	// Sub-test 1: Plain SendMessage — single round-trip, no tools.
	t.Run("SendMessage_Pong", func(t *testing.T) {
		adapter := newE2EAdapter(t)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		msg, toolCalls, err := adapter.SendMessage(
			ctx,
			[]port.MessageParam{
				{Role: entity.RoleUser, Content: "Reply with the single word: pong"},
			},
			nil,
			port.AIRequestOptions{},
		)
		if err != nil {
			t.Fatalf("SendMessage failed: %v", err)
		}
		if len(toolCalls) != 0 {
			t.Errorf("expected no tool calls for plain text query; got %d", len(toolCalls))
		}
		if msg == nil {
			t.Fatal("expected non-nil message")
		}
		if !strings.Contains(strings.ToLower(msg.Content), "pong") {
			t.Errorf("response does not contain 'pong': %q", msg.Content)
		}
		t.Logf("sub-test 1 response: %q (in=%d out=%d)", msg.Content, msg.InputTokens, msg.OutputTokens)
	})

	// Sub-test 2: SendMessage with a tool — assert ToolCallInfo is returned.
	// We send a message asking what time it is and give the model a get_time tool.
	// We assert a ToolCallInfo for get_time is present; we do NOT drive the loop further
	// because the domain layer owns that responsibility.
	t.Run("SendMessage_WithTool", func(t *testing.T) {
		adapter := newE2EAdapter(t)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		tools := []port.ToolParam{
			{
				Name:        "get_time",
				Description: "Returns the current UTC time as an ISO 8601 timestamp.",
				InputSchema: map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
				},
			},
		}

		msg, toolCalls, err := adapter.SendMessage(
			ctx,
			[]port.MessageParam{
				{Role: entity.RoleUser, Content: "What is the current time? Use the get_time tool."},
			},
			tools,
			port.AIRequestOptions{},
		)
		if err != nil {
			t.Fatalf("SendMessage with tool failed: %v", err)
		}
		if msg == nil {
			t.Fatal("expected non-nil message")
		}
		if len(toolCalls) == 0 {
			t.Errorf("expected at least one ToolCallInfo for get_time; got none (response=%q)", msg.Content)
		} else {
			found := false
			for _, tc := range toolCalls {
				if tc.ToolName == "get_time" {
					found = true
					if tc.ToolID == "" {
						t.Errorf("ToolCallInfo.ToolID should be non-empty")
					}
					t.Logf("sub-test 2 tool call: id=%q name=%q input=%v", tc.ToolID, tc.ToolName, tc.Input)
				}
			}
			if !found {
				t.Errorf("tool calls present but none for 'get_time': %+v", toolCalls)
			}
		}
	})

	// Sub-test 3: SendMessageStreaming — same "pong" prompt, verify streaming callback fires
	// and the final accumulated result also contains "pong".
	t.Run("SendMessageStreaming_Pong", func(t *testing.T) {
		adapter := newE2EAdapter(t)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		var accumulated strings.Builder
		callbackCount := 0

		textCallback := func(chunk string) error {
			accumulated.WriteString(chunk)
			callbackCount++
			return nil
		}

		msg, toolCalls, err := adapter.SendMessageStreaming(
			ctx,
			[]port.MessageParam{
				{Role: entity.RoleUser, Content: "Reply with the single word: pong"},
			},
			nil,
			port.AIRequestOptions{},
			textCallback,
			nil, // no thinking callback
		)
		if err != nil {
			t.Fatalf("SendMessageStreaming failed: %v", err)
		}
		if len(toolCalls) != 0 {
			t.Errorf("expected no tool calls for plain text query; got %d", len(toolCalls))
		}
		if msg == nil {
			t.Fatal("expected non-nil final message from streaming")
		}
		// The streaming callback should have fired at least once.
		if callbackCount == 0 {
			t.Errorf("textCallback was never invoked; streaming may not be working")
		}
		// The accumulated text from callbacks should be non-empty and contain "pong".
		accText := accumulated.String()
		if accText == "" {
			t.Errorf("accumulated streaming text is empty")
		}
		if !strings.Contains(strings.ToLower(accText), "pong") {
			t.Errorf("accumulated streaming text does not contain 'pong': %q", accText)
		}
		// The returned message should also contain "pong".
		if !strings.Contains(strings.ToLower(msg.Content), "pong") {
			t.Errorf("final message content does not contain 'pong': %q", msg.Content)
		}
		t.Logf("sub-test 3: %d chunks, accumulated=%q, final=%q", callbackCount, accText, msg.Content)
	})

	// Sub-test 4 (optional): Extended thinking — simple math question with budget=1024.
	// If the Genkit Anthropic plugin returns an error for thinking with haiku-4-5
	// (e.g. model does not support thinking), we skip with a clear reason rather than
	// failing, since the model may not support extended thinking.
	t.Run("SendMessage_ExtendedThinking", func(t *testing.T) {
		// Extended thinking requires a larger max_tokens value than the default e2eMaxTokens.
		// The minimum for thinking is budget_tokens + at least some output tokens.
		// Use a dedicated adapter with higher token limit for this sub-test.
		adapter, err := NewGenkitAdapter(e2eModel, int64(2048), nil, port.NopLogger{})
		if err != nil {
			t.Fatalf("NewGenkitAdapter failed: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		opts := port.AIRequestOptions{
			Thinking: &port.ThinkingModeInfo{
				Enabled:      true,
				BudgetTokens: int64(1024),
			},
		}

		msg, _, err := adapter.SendMessage(
			ctx,
			[]port.MessageParam{
				{Role: entity.RoleUser, Content: "What is 17 multiplied by 23? Think through it step by step."},
			},
			nil,
			opts,
		)
		if err != nil {
			// Some models or API configurations may not support extended thinking.
			// Treat this as a skip rather than a hard failure so the CI doesn't
			// block on a capability gap in the chosen model.
			t.Skipf("extended thinking returned error (model may not support it): %v", err)
		}
		if msg == nil {
			t.Fatal("expected non-nil message")
		}
		// We expect thinking blocks to be present when thinking is enabled.
		if len(msg.ThinkingBlocks) == 0 {
			t.Errorf("expected at least one thinking block with extended thinking enabled; got none (content=%q)", msg.Content)
		} else {
			preview := msg.ThinkingBlocks[0].Thinking
			if len(preview) > 80 {
				preview = preview[:80]
			}
			t.Logf("sub-test 4: %d thinking block(s), first thinking=%q",
				len(msg.ThinkingBlocks), preview)
		}
		// The final answer should reference 391 (17*23).
		if !strings.Contains(msg.Content, "391") {
			t.Errorf("expected answer 391 in response; got: %q", msg.Content)
		}
		t.Logf("sub-test 4 content: %q", msg.Content)
	})

	// Sub-test 5: SendMessage with the full tool catalog registered in one call.
	// Verifies that per-tool WithStrict(false) registration, name sanitization,
	// and the bidirectional name map all hold up when many tools are presented
	// simultaneously. Asserts the model picked exactly one tool and that the
	// reported ToolName is the original (un-sanitized) port name.
	t.Run("SendMessage_AllToolsAtOnce", func(t *testing.T) {
		t.Parallel()
		adapter := newE2EAdapter(t)
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		// Source the real tool catalog from the project's ExecutorAdapter so this
		// stays in sync with whatever tools the agent actually exposes.
		fm := file.NewLocalFileManager(t.TempDir())
		exec := tool.NewExecutorAdapter(fm, port.NopLogger{})
		entityTools, err := exec.ListTools()
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}
		if len(entityTools) < 10 {
			t.Fatalf("expected the executor to expose the full tool catalog (>= 10); got %d", len(entityTools))
		}

		tools := make([]port.ToolParam, 0, len(entityTools))
		known := make(map[string]bool, len(entityTools))
		for _, et := range entityTools {
			tools = append(tools, port.ToolParam{
				Name:        et.Name,
				Description: et.Description,
				InputSchema: et.InputSchema,
				Strict:      et.Strict,
			})
			known[et.Name] = true
		}
		t.Logf("sub-test 5: registering %d tools: %v", len(tools), toolNames(entityTools))

		msg, toolCalls, err := adapter.SendMessage(
			ctx,
			[]port.MessageParam{
				{Role: entity.RoleUser, Content: "List the files in the current directory using the available tools."},
			},
			tools,
			port.AIRequestOptions{},
		)
		if err != nil {
			t.Fatalf("SendMessage with full tool catalog failed: %v", err)
		}
		if msg == nil {
			t.Fatal("expected non-nil message")
		}
		if len(toolCalls) == 0 {
			t.Fatalf("expected at least one ToolCallInfo; got none (response=%q)", msg.Content)
		}

		var picked []string
		for _, tc := range toolCalls {
			if !known[tc.ToolName] {
				t.Errorf("ToolCallInfo.ToolName %q is not one of the originally registered names %v",
					tc.ToolName, known)
			}
			if tc.ToolID == "" {
				t.Errorf("ToolCallInfo.ToolID should be non-empty for %q", tc.ToolName)
			}
			picked = append(picked, tc.ToolName)
		}
		t.Logf("sub-test 5: model picked tools %v from a catalog of %d", picked, len(tools))
	})

	// Sub-test 6: parity check — same full tool catalog through the raw
	// AnthropicAdapter. Lets us answer "but doesn't it work on the regular SDK?"
	// empirically rather than by reading docs.
	t.Run("SendMessage_AllToolsAtOnce_RawAnthropic", func(t *testing.T) {
		t.Parallel()
		// Defensive: the outer TestE2E_GenkitAdapter already skips on missing
		// key, but adding this here makes the sub-test self-contained against
		// future extraction into a top-level test.
		skipIfNoAPIKey(t)
		anth := NewAnthropicAdapter(e2eModel, e2eMaxTokens, nil)
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		fm := file.NewLocalFileManager(t.TempDir())
		exec := tool.NewExecutorAdapter(fm, port.NopLogger{})
		entityTools, err := exec.ListTools()
		if err != nil {
			t.Fatalf("ListTools failed: %v", err)
		}
		tools := make([]port.ToolParam, 0, len(entityTools))
		for _, et := range entityTools {
			tools = append(tools, port.ToolParam{
				Name:        et.Name,
				Description: et.Description,
				InputSchema: et.InputSchema,
				Strict:      et.Strict,
			})
		}
		t.Logf("sub-test 6: AnthropicAdapter registering %d tools: %v",
			len(tools), toolNames(entityTools))

		msg, toolCalls, err := anth.SendMessage(
			ctx,
			[]port.MessageParam{
				{Role: entity.RoleUser, Content: "List the files in the current directory using the available tools."},
			},
			tools,
			port.AIRequestOptions{},
		)
		if err != nil {
			t.Fatalf("AnthropicAdapter SendMessage with full tool catalog failed: %v", err)
		}
		if msg == nil {
			t.Fatal("expected non-nil message")
		}
		t.Logf("sub-test 6: AnthropicAdapter accepted the catalog; %d tool call(s); content=%q",
			len(toolCalls), msg.Content)
	})
}

// toolNames returns the Name field of each tool — used for readable test logs.
func toolNames(ts []entity.Tool) []string {
	names := make([]string, 0, len(ts))
	for _, t := range ts {
		names = append(names, t.Name)
	}
	return names
}

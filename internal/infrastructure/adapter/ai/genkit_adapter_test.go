package ai

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
	genkitai "github.com/firebase/genkit/go/ai"
)

// newTestAdapter builds a GenkitAdapter without calling genkit.Init. It is
// suitable for tests that exercise message conversion, name sanitization, and
// other logic that does not require a live registry.
func newTestAdapter() *GenkitAdapter {
	return &GenkitAdapter{
		plugin:      anthropicGenkitPlugin{},
		model:       "claude-3-5-sonnet-20241022",
		maxTokens:   1024,
		logger:      port.NopLogger{},
		toolNameMap: map[string]string{},
		nameMapMu:   &sync.RWMutex{},
	}
}

// TestNewGenkitAdapter_RejectsHFModels covers AC #2 — hf:* prefix must be
// rejected at construction with a clear error referencing the migration path.
func TestNewGenkitAdapter_RejectsHFModels(t *testing.T) {
	t.Parallel()

	cases := []string{
		"hf:zai-org/GLM-4.7",
		"hf:meta-llama/Llama-3-70B",
	}

	for _, model := range cases {
		t.Run(model, func(t *testing.T) {
			t.Parallel()
			_, err := NewGenkitAdapter(model, 1024, nil, port.NopLogger{})
			if err == nil {
				t.Fatalf("expected error for hf:* model, got nil")
			}
			if !errors.Is(err, ErrHFModelNotSupported) {
				t.Fatalf("expected ErrHFModelNotSupported, got %v", err)
			}
			if !strings.Contains(err.Error(), "ANTHROPIC_BASE_URL") {
				t.Errorf("error must reference ANTHROPIC_BASE_URL migration path; got: %v", err)
			}
		})
	}
}

// TestSetModel covers AC #9 — empty input is rejected; hf:* is rejected;
// valid input updates the model.
func TestSetModel(t *testing.T) {
	t.Parallel()

	t.Run("empty model returns error", func(t *testing.T) {
		t.Parallel()
		a := newTestAdapter()
		if err := a.SetModel(""); err == nil {
			t.Fatalf("expected error for empty model")
		}
	})

	t.Run("hf prefix rejected", func(t *testing.T) {
		t.Parallel()
		a := newTestAdapter()
		err := a.SetModel("hf:foo/bar")
		if !errors.Is(err, ErrHFModelNotSupported) {
			t.Fatalf("expected ErrHFModelNotSupported, got %v", err)
		}
	})

	t.Run("valid model accepted", func(t *testing.T) {
		t.Parallel()
		a := newTestAdapter()
		if err := a.SetModel("claude-3-7-sonnet-latest"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if a.GetModel() != "claude-3-7-sonnet-latest" {
			t.Errorf("GetModel mismatch: %s", a.GetModel())
		}
	})
}

// TestClone_Isolation covers AC #7 — Clone() returns an independent copy
// where mutating one's model does not affect the other, but the genkit
// registry reference is shared.
func TestClone_Isolation(t *testing.T) {
	t.Parallel()

	a := newTestAdapter()
	a.toolNameMap["sanitized_name"] = "Original::Name"

	clonePort := a.Clone()
	clone, ok := clonePort.(*GenkitAdapter)
	if !ok {
		t.Fatalf("Clone returned unexpected type %T", clonePort)
	}

	// Independent model field.
	if err := clone.SetModel("claude-3-5-haiku-20241022"); err != nil {
		t.Fatalf("SetModel on clone failed: %v", err)
	}
	if a.GetModel() == clone.GetModel() {
		t.Errorf("clone's model change leaked back to original (both %q)", a.GetModel())
	}

	// Shared genkit reference.
	if a.g != clone.g {
		t.Errorf("Clone() should share *genkit.Genkit reference; original=%p clone=%p", a.g, clone.g)
	}

	// Shared tool name map (so sanitized→original lookups stay consistent).
	if got := clone.originalToolName("sanitized_name"); got != "Original::Name" {
		t.Errorf("clone lost tool name mapping; got %q", got)
	}
}

// TestSanitizeToolName covers AC #4 — invalid chars replaced, length capped,
// already-valid names pass through.
func TestSanitizeToolName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		in          string
		wantOut     string
		wantChanged bool
	}{
		{"valid name passes through", "read_file", "read_file", false},
		{"hyphen allowed", "list-files", "list-files", false},
		{"colons replaced", "skill::activate", "skill__activate", true},
		{"dots replaced", "tool.with.dots", "tool_with_dots", true},
		{"slash replaced", "ns/tool", "ns_tool", true},
		{"all invalid replaced", "$$$", "___", true},
		{"empty becomes underscore", "", "_", true},
		{
			"long name truncated to 64",
			strings.Repeat("a", 100),
			strings.Repeat("a", 64),
			true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, changed := sanitizeToolName(tc.in)
			if got != tc.wantOut {
				t.Errorf("output: got %q, want %q", got, tc.wantOut)
			}
			if changed != tc.wantChanged {
				t.Errorf("changed flag: got %v, want %v", changed, tc.wantChanged)
			}
			if !genkitToolNameRE.MatchString(got) {
				t.Errorf("output %q does not match Genkit regex", got)
			}
		})
	}
}

// TestToolNameSanitization_RoundTrip covers AC #4 — the bidirectional name
// map preserves the original port-facing name when reporting tool calls back.
func TestToolNameSanitization_RoundTrip(t *testing.T) {
	t.Parallel()

	a := newTestAdapter()
	originals := []string{
		"plain_tool",
		"skill::activate",
		"weird.name/with-mixed.chars",
		strings.Repeat("a", 80),
	}

	for _, orig := range originals {
		san, _ := sanitizeToolName(orig)
		a.toolNameMap[san] = orig
	}

	for _, orig := range originals {
		san, _ := sanitizeToolName(orig)
		got := a.originalToolName(san)
		if got != orig {
			t.Errorf("round-trip failed: sanitized %q → original %q, want %q", san, got, orig)
		}
	}

	// Unknown sanitized names fall back to the input.
	if got := a.originalToolName("never_registered"); got != "never_registered" {
		t.Errorf("expected fall-through for unmapped name; got %q", got)
	}
}

// TestConvertMessages_SimpleUserText covers user-role plain text conversion.
func TestConvertMessages_SimpleUserText(t *testing.T) {
	t.Parallel()
	a := newTestAdapter()
	out := a.convertMessagesToGenkit([]port.MessageParam{
		{Role: entity.RoleUser, Content: "hello"},
	})
	if len(out) != 1 {
		t.Fatalf("expected 1 message, got %d", len(out))
	}
	if out[0].Role != genkitai.RoleUser {
		t.Errorf("role: got %v, want user", out[0].Role)
	}
	if len(out[0].Content) != 1 || !out[0].Content[0].IsText() || out[0].Content[0].Text != "hello" {
		t.Errorf("unexpected parts: %+v", out[0].Content)
	}
}

// TestConvertMessages_AssistantTextRole verifies the assistant role maps to
// Genkit's RoleModel.
func TestConvertMessages_AssistantTextRole(t *testing.T) {
	t.Parallel()
	a := newTestAdapter()
	out := a.convertMessagesToGenkit([]port.MessageParam{
		{Role: entity.RoleAssistant, Content: "hi back"},
	})
	if out[0].Role != genkitai.RoleModel {
		t.Errorf("assistant role must map to RoleModel; got %v", out[0].Role)
	}
}

// TestConvertMessages_UserToolResults verifies tool results become Genkit
// ToolResponse parts with success/error output shapes.
func TestConvertMessages_UserToolResults(t *testing.T) {
	t.Parallel()
	a := newTestAdapter()
	out := a.convertMessagesToGenkit([]port.MessageParam{
		{
			Role: entity.RoleUser,
			ToolResults: []port.ToolResultParam{
				{ToolID: "call_1", Result: "42", IsError: false},
				{ToolID: "call_2", Result: "boom", IsError: true},
			},
		},
	})
	if len(out[0].Content) != 2 {
		t.Fatalf("expected 2 tool-response parts, got %d", len(out[0].Content))
	}
	for _, p := range out[0].Content {
		if !p.IsToolResponse() {
			t.Errorf("expected IsToolResponse, got kind=%v", p.Kind)
		}
	}
	assertToolOutputField(t, out[0].Content[0].ToolResponse.Output, "result", "42")
	assertToolOutputField(t, out[0].Content[1].ToolResponse.Output, "error", "boom")
}

// TestConvertMessages_AssistantWithThinkingTextAndToolCalls verifies the
// part ordering (reasoning → text → tool request) and that tool names emitted
// to Genkit are sanitized.
func TestConvertMessages_AssistantWithThinkingTextAndToolCalls(t *testing.T) {
	t.Parallel()
	a := newTestAdapter()
	out := a.convertMessagesToGenkit([]port.MessageParam{
		{
			Role:           entity.RoleAssistant,
			Content:        "let me check",
			ThinkingBlocks: []port.ThinkingBlockParam{{Thinking: "hmm", Signature: "sig123"}},
			ToolCalls: []port.ToolCallParam{
				{ToolID: "call_a", ToolName: "weird.tool", Input: map[string]interface{}{"x": 1}},
			},
		},
	})
	parts := out[0].Content
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts (reasoning, text, toolReq), got %d", len(parts))
	}
	if !parts[0].IsReasoning() {
		t.Errorf("part 0 must be reasoning, got kind=%v", parts[0].Kind)
	}
	if !parts[1].IsText() {
		t.Errorf("part 1 must be text, got kind=%v", parts[1].Kind)
	}
	if !parts[2].IsToolRequest() {
		t.Errorf("part 2 must be tool request, got kind=%v", parts[2].Kind)
	}
	if got := parts[2].ToolRequest.Name; got != "weird_tool" {
		t.Errorf("tool name in genkit message must be sanitized; got %q", got)
	}
}

// assertToolOutputField helper unwraps the {field: want} map produced by
// toolResultOutput and reports a t.Errorf if the field is missing or differs.
func assertToolOutputField(t *testing.T, output any, field, want string) {
	t.Helper()
	m, ok := output.(map[string]any)
	if !ok {
		t.Errorf("tool response output not a map: %+v", output)
		return
	}
	if got, _ := m[field].(string); got != want {
		t.Errorf("output[%q]: got %q, want %q (full=%+v)", field, got, want, m)
	}
}

// TestConvertResponse covers AC #3 / #10 / #11 — the model→port direction:
// Genkit ModelResponse → (*entity.Message, []ToolCallInfo). Includes token
// usage int→int64 cast and tool name reverse-mapping.
func TestConvertResponse(t *testing.T) {
	t.Parallel()

	a := newTestAdapter()
	// Pre-populate the name map as if "weird.tool" had been registered.
	a.toolNameMap["weird_tool"] = "weird.tool"

	resp := &genkitai.ModelResponse{
		Message: &genkitai.Message{
			Role: genkitai.RoleModel,
			Content: []*genkitai.Part{
				genkitai.NewReasoningPart("thinking out loud", []byte("sig-bytes")),
				genkitai.NewTextPart("Here is the result."),
				genkitai.NewToolRequestPart(&genkitai.ToolRequest{
					Ref:   "call_xyz",
					Name:  "weird_tool",
					Input: map[string]interface{}{"a": "b"},
				}),
			},
		},
		Usage: &genkitai.GenerationUsage{
			InputTokens:         123,
			OutputTokens:        456,
			CachedContentTokens: 7,
		},
	}

	msg, toolCalls, err := a.convertResponse(resp)
	if err != nil {
		t.Fatalf("convertResponse error: %v", err)
	}

	if msg.Content != "Here is the result." {
		t.Errorf("text content mismatch: %q", msg.Content)
	}
	if msg.InputTokens != int64(123) {
		t.Errorf("InputTokens int64 cast: got %d, want 123", msg.InputTokens)
	}
	if msg.OutputTokens != int64(456) {
		t.Errorf("OutputTokens int64 cast: got %d, want 456", msg.OutputTokens)
	}
	if len(msg.ThinkingBlocks) != 1 {
		t.Fatalf("expected 1 thinking block, got %d", len(msg.ThinkingBlocks))
	}
	if msg.ThinkingBlocks[0].Thinking != "thinking out loud" {
		t.Errorf("thinking text mismatch: %q", msg.ThinkingBlocks[0].Thinking)
	}
	if msg.ThinkingBlocks[0].Signature != "sig-bytes" {
		t.Errorf("thinking signature mismatch: %q", msg.ThinkingBlocks[0].Signature)
	}

	if len(toolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(toolCalls))
	}
	if toolCalls[0].ToolName != "weird.tool" {
		t.Errorf("tool name should map back to original; got %q", toolCalls[0].ToolName)
	}
	if toolCalls[0].ToolID != "call_xyz" {
		t.Errorf("tool id mismatch: %q", toolCalls[0].ToolID)
	}
	if toolCalls[0].Input["a"] != "b" {
		t.Errorf("tool input mismatch: %+v", toolCalls[0].Input)
	}
	if toolCalls[0].InputJSON == "" {
		t.Errorf("InputJSON should be populated")
	}
}

// TestConvertResponse_NilGuards covers defensive handling of malformed input.
func TestConvertResponse_NilGuards(t *testing.T) {
	t.Parallel()
	a := newTestAdapter()

	if _, _, err := a.convertResponse(nil); err == nil {
		t.Errorf("expected error for nil response")
	}
	if _, _, err := a.convertResponse(&genkitai.ModelResponse{}); err == nil {
		t.Errorf("expected error for response with nil message")
	}
}

// TestQualifiedModelName ensures the provider prefix is added when missing
// and preserved when already present.
func TestQualifiedModelName(t *testing.T) {
	t.Parallel()
	a := newTestAdapter()
	cases := map[string]string{
		"claude-3-5-sonnet-20241022":           "anthropic/claude-3-5-sonnet-20241022",
		"anthropic/claude-3-5-sonnet-20241022": "anthropic/claude-3-5-sonnet-20241022",
		"some/other/path":                      "some/other/path",
	}
	for in, want := range cases {
		if got := a.qualifiedModelName(in); got != want {
			t.Errorf("qualifiedModelName(%q): got %q, want %q", in, got, want)
		}
	}
}

// TestHealthCheck covers AC #8.
func TestHealthCheck(t *testing.T) {
	t.Parallel()
	a := newTestAdapter()
	if err := a.HealthCheck(t.Context()); err != nil {
		t.Errorf("HealthCheck with model set should succeed; got %v", err)
	}
	a.model = ""
	if err := a.HealthCheck(t.Context()); err == nil {
		t.Errorf("HealthCheck with empty model should fail")
	}
}

// TestSendMessage_GuardsBeforeNetwork covers the validation guards reachable
// without invoking genkit.Generate (no API keys / network needed).
func TestSendMessage_GuardsBeforeNetwork(t *testing.T) {
	t.Parallel()

	t.Run("empty messages rejected", func(t *testing.T) {
		t.Parallel()
		a := newTestAdapter()
		_, _, err := a.SendMessage(t.Context(), nil, nil, port.AIRequestOptions{})
		if !errors.Is(err, ErrEmptyMessages) {
			t.Errorf("expected ErrEmptyMessages, got %v", err)
		}
	})

	t.Run("missing model rejected", func(t *testing.T) {
		t.Parallel()
		a := newTestAdapter()
		a.model = ""
		_, _, err := a.SendMessage(
			t.Context(),
			[]port.MessageParam{{Role: entity.RoleUser, Content: "hi"}},
			nil,
			port.AIRequestOptions{},
		)
		if !errors.Is(err, ErrModelNotSet) {
			t.Errorf("expected ErrModelNotSet, got %v", err)
		}
	})
}

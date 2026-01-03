package ai

import (
	"code-editing-agent/internal/domain/port"
	"context"
	"testing"
)

// TestConvertTools_WithRequiredField verifies that when a tool has a required field,
// it's properly included in the output anthropic.ToolInputSchemaParam.Required field.
func TestConvertTools_WithRequiredField(t *testing.T) {
	// Setup: create an adapter
	adapter := &AnthropicAdapter{}

	// Input: a tool with a standard JSON schema including type, properties, and required
	tools := []port.ToolParam{
		{
			Name:        "read_file",
			Description: "Read the contents of a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to read",
					},
				},
				"required": []string{"path"},
			},
		},
	}

	// Execute: convert tools
	result := adapter.convertTools(tools)

	// Assert: we get back one tool
	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	tool := result[0].OfTool
	if tool == nil {
		t.Fatal("expected OfTool to be non-nil")
	}

	// Assert: the tool name and description are preserved
	if tool.Name != "read_file" {
		t.Errorf("expected tool name 'read_file', got '%s'", tool.Name)
	}
	if tool.Description.Value != "Read the contents of a file" {
		t.Errorf("expected description 'Read the contents of a file', got '%s'", tool.Description.Value)
	}

	// Assert: the input schema type is set correctly
	if tool.InputSchema.Type != "object" {
		t.Errorf("expected input schema type 'object', got '%s'", tool.InputSchema.Type)
	}

	// Assert: the required field is properly set
	if tool.InputSchema.Required == nil {
		t.Fatalf("expected Required field to be non-nil, got nil")
	}
	if len(tool.InputSchema.Required) != 1 {
		t.Errorf("expected 1 required field, got %d", len(tool.InputSchema.Required))
	}
	if tool.InputSchema.Required[0] != "path" {
		t.Errorf("expected required field 'path', got '%s'", tool.InputSchema.Required[0])
	}

	// Assert: the properties contain only the property definitions, not type or required
	properties, ok := tool.InputSchema.Properties.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Properties to be map[string]interface{}, got %T", tool.InputSchema.Properties)
	}
	if len(properties) != 1 {
		t.Errorf("expected 1 property in Properties map, got %d", len(properties))
	}
	pathProperty, exists := properties["path"]
	if !exists {
		t.Fatal("expected property 'path' to exist in Properties map")
	}
	pathPropertyMap, ok := pathProperty.(map[string]interface{})
	if !ok {
		t.Fatalf("expected path property to be a map[string]interface{}, got %T", pathProperty)
	}
	if pathPropertyMap["type"] != "string" {
		t.Errorf("expected path property type 'string', got %v", pathPropertyMap["type"])
	}
}

// TestConvertTools_WithMultipleRequiredFields verifies that when a tool has multiple
// required fields, all of them are included in the output.
func TestConvertTools_WithMultipleRequiredFields(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{
		{
			Name:        "write_file",
			Description: "Write content to a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to write",
					},
					"content": map[string]interface{}{
						"type":        "string",
						"description": "The content to write to the file",
					},
					"mode": map[string]interface{}{
						"type":        "string",
						"description": "The write mode",
					},
				},
				"required": []string{"path", "content"},
			},
		},
	}

	result := adapter.convertTools(tools)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	tool := result[0].OfTool
	if tool == nil {
		t.Fatal("expected OfTool to be non-nil")
	}

	// Assert: the required field contains all required field names
	if tool.InputSchema.Required == nil {
		t.Fatalf("expected Required field to be non-nil, got nil")
	}
	if len(tool.InputSchema.Required) != 2 {
		t.Errorf("expected 2 required fields, got %d", len(tool.InputSchema.Required))
	}

	// Check that each required field is present
	requiredSet := make(map[string]bool)
	for _, req := range tool.InputSchema.Required {
		requiredSet[req] = true
	}

	if !requiredSet["path"] {
		t.Error("expected 'path' to be in required fields")
	}
	if !requiredSet["content"] {
		t.Error("expected 'content' to be in required fields")
	}
}

// TestConvertTools_WithNoRequiredField verifies that when a tool has no required
// fields, the output Required field is either nil or empty.
func TestConvertTools_WithNoRequiredField(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{
		{
			Name:        "read_file",
			Description: "Read the contents of a file",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{
						"type":        "string",
						"description": "The path to the file to read",
					},
				},
			},
		},
	}

	result := adapter.convertTools(tools)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	tool := result[0].OfTool
	if tool == nil {
		t.Fatal("expected OfTool to be non-nil")
	}

	// Assert: the required field is either nil or empty
	if len(tool.InputSchema.Required) > 0 {
		t.Errorf("expected Required field to be nil or empty, got %v", tool.InputSchema.Required)
	}
}

// TestConvertTools_PropertiesFieldIsCorrectlySet verifies that the properties field
// is correctly set and doesn't include type or required at the top level.
func TestConvertTools_PropertiesFieldIsCorrectlySet(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{
		{
			Name:        "complex_tool",
			Description: "A tool with complex schema",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"file_path": map[string]interface{}{
						"type":        "string",
						"description": "Path to the file",
					},
					"line_number": map[string]interface{}{
						"type":        "integer",
						"description": "Line number to edit",
					},
				},
				"required": []string{"file_path"},
			},
		},
	}

	result := adapter.convertTools(tools)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	tool := result[0].OfTool
	if tool == nil {
		t.Fatal("expected OfTool to be non-nil")
	}

	properties, ok := tool.InputSchema.Properties.(map[string]interface{})
	if !ok {
		t.Fatalf("expected Properties to be map[string]interface{}, got %T", tool.InputSchema.Properties)
	}

	// Assert: type and required are NOT in the Properties map
	if _, exists := properties["type"]; exists {
		t.Error("'type' should not be in the Properties map - it should be a separate field")
	}
	if _, exists := properties["required"]; exists {
		t.Error("'required' should not be in the Properties map - it should be a separate field")
	}

	// Assert: only property definitions are in Properties
	if len(properties) != 2 {
		t.Errorf("expected 2 properties in Properties map, got %d", len(properties))
	}

	// Verify each property exists and has correct structure
	filePathProp, exists := properties["file_path"]
	if !exists {
		t.Fatal("expected 'file_path' property to exist")
	}
	filePathMap, ok := filePathProp.(map[string]interface{})
	if !ok {
		t.Fatalf("expected file_path property to be a map, got %T", filePathProp)
	}
	if filePathMap["type"] != "string" {
		t.Errorf("expected file_path type 'string', got %v", filePathMap["type"])
	}

	lineNumberProp, exists := properties["line_number"]
	if !exists {
		t.Fatal("expected 'line_number' property to exist")
	}
	lineNumberMap, ok := lineNumberProp.(map[string]interface{})
	if !ok {
		t.Fatalf("expected line_number property to be a map, got %T", lineNumberProp)
	}
	if lineNumberMap["type"] != "integer" {
		t.Errorf("expected line_number type 'integer', got %v", lineNumberMap["type"])
	}
}

// TestConvertTools_EmptyInputSchema verifies that when a tool has an empty or nil
// input schema, the tool is still converted correctly.
func TestConvertTools_EmptyInputSchema(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{
		{
			Name:        "simple_tool",
			Description: "A tool with no input schema",
			InputSchema: nil,
		},
	}

	result := adapter.convertTools(tools)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	tool := result[0].OfTool
	if tool == nil {
		t.Fatal("expected OfTool to be non-nil")
	}

	if tool.Name != "simple_tool" {
		t.Errorf("expected tool name 'simple_tool', got '%s'", tool.Name)
	}

	// Properties should be nil since InputSchema was nil
	if tool.InputSchema.Properties != nil {
		t.Errorf("expected nil Properties, got %v", tool.InputSchema.Properties)
	}

	// Required should be nil
	if tool.InputSchema.Required != nil {
		t.Errorf("expected nil Required, got %v", tool.InputSchema.Required)
	}
}

// TestConvertTools_MultipleTools verifies that multiple tools are converted correctly,
// each preserving its own required fields and properties.
func TestConvertTools_MultipleTools(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{
		{
			Name:        "read_file",
			Description: "Read file contents",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string"},
				},
				"required": []string{"path"},
			},
		},
		{
			Name:        "write_file",
			Description: "Write file contents",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path":    map[string]interface{}{"type": "string"},
					"content": map[string]interface{}{"type": "string"},
				},
				"required": []string{"path", "content"},
			},
		},
		{
			Name:        "list_files",
			Description: "List directory contents",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"path": map[string]interface{}{"type": "string"},
				},
				// No required fields
			},
		},
	}

	result := adapter.convertTools(tools)

	if len(result) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(result))
	}

	// Verify first tool
	tool0 := result[0].OfTool
	if tool0.Name != "read_file" {
		t.Errorf("tool 0: expected name 'read_file', got '%s'", tool0.Name)
	}
	if len(tool0.InputSchema.Required) != 1 || tool0.InputSchema.Required[0] != "path" {
		t.Errorf("tool 0: expected Required [path], got %v", tool0.InputSchema.Required)
	}

	// Verify second tool
	tool1 := result[1].OfTool
	if tool1.Name != "write_file" {
		t.Errorf("tool 1: expected name 'write_file', got '%s'", tool1.Name)
	}
	if len(tool1.InputSchema.Required) != 2 {
		t.Errorf("tool 1: expected 2 required fields, got %d", len(tool1.InputSchema.Required))
	}

	// Verify third tool
	tool2 := result[2].OfTool
	if tool2.Name != "list_files" {
		t.Errorf("tool 2: expected name 'list_files', got '%s'", tool2.Name)
	}
	if len(tool2.InputSchema.Required) > 0 {
		t.Errorf("tool 2: expected no required fields, got %v", tool2.InputSchema.Required)
	}
}

// TestConvertTools_EmptySlice verifies that an empty input slice returns an empty output slice.
func TestConvertTools_EmptySlice(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{}

	result := adapter.convertTools(tools)

	if len(result) != 0 {
		t.Errorf("expected 0 tools, got %d", len(result))
	}
}

// TestConvertTools_RequiredPreservesOrder verifies that the order of required fields
// is preserved in the output.
func TestConvertTools_RequiredPreservesOrder(t *testing.T) {
	adapter := &AnthropicAdapter{}

	tools := []port.ToolParam{
		{
			Name:        "test_tool",
			Description: "Test tool",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"alpha":   map[string]interface{}{"type": "string"},
					"beta":    map[string]interface{}{"type": "string"},
					"gamma":   map[string]interface{}{"type": "string"},
					"delta":   map[string]interface{}{"type": "string"},
					"epsilon": map[string]interface{}{"type": "string"},
				},
				"required": []string{"beta", "alpha", "delta", "epsilon", "gamma"},
			},
		},
	}

	result := adapter.convertTools(tools)

	if len(result) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(result))
	}

	tool := result[0].OfTool
	expectedOrder := []string{"beta", "alpha", "delta", "epsilon", "gamma"}

	if len(tool.InputSchema.Required) != len(expectedOrder) {
		t.Fatalf("expected %d required fields, got %d", len(expectedOrder), len(tool.InputSchema.Required))
	}

	for i, expected := range expectedOrder {
		if tool.InputSchema.Required[i] != expected {
			t.Errorf("required field at index %d: expected '%s', got '%s'", i, expected, tool.InputSchema.Required[i])
		}
	}
}

// ============================================================================
// Custom System Prompt Priority Tests (RED PHASE)
// ============================================================================
// These tests verify that getSystemPrompt() correctly prioritizes custom
// system prompts from context over plan mode and base prompts.
//
// PRIORITY ORDER (highest to lowest):
// 1. Custom system prompt (from CustomSystemPromptFromContext)
// 2. Plan mode prompt (from PlanModeFromContext)
// 3. Base prompt with skills (default)
// ============================================================================

// TestGetSystemPrompt_CustomPromptTakesPrecedenceOverBasePrompt verifies that
// when a custom system prompt is present in the context, it is returned instead
// of the base prompt.
//
// This test will FAIL until getSystemPrompt() is updated to check for custom
// prompts in the context via CustomSystemPromptFromContext().
func TestGetSystemPrompt_CustomPromptTakesPrecedenceOverBasePrompt(t *testing.T) {
	// Setup: create adapter with no skill manager for predictable base prompt
	adapter := &AnthropicAdapter{
		model:        "test-model",
		skillManager: nil, // No skills for simpler test
	}

	// Expected prompts
	customPrompt := "You are a specialized code review assistant. Focus on security vulnerabilities."
	expectedPrompt := customPrompt

	// Setup: create context with custom system prompt
	ctx := port.WithCustomSystemPrompt(
		context.Background(),
		port.CustomSystemPromptInfo{
			SessionID: "test-session-123",
			Prompt:    customPrompt,
		},
	)

	// Execute: get system prompt
	actualPrompt := adapter.getSystemPrompt(ctx)

	// Assert: should return custom prompt, not base prompt
	if actualPrompt != expectedPrompt {
		t.Errorf("Expected custom prompt to take precedence.\nWant: %q\nGot:  %q", expectedPrompt, actualPrompt)
	}

	// Assert: should NOT be the base prompt
	basePrompt := adapter.buildBasePromptWithSkills()
	if actualPrompt == basePrompt {
		t.Error("Custom prompt should take precedence over base prompt, but got base prompt instead")
	}
}

// TestGetSystemPrompt_CustomPromptTakesPrecedenceOverPlanMode verifies that
// when BOTH custom system prompt AND plan mode are present in the context,
// the custom system prompt takes precedence.
//
// This test will FAIL until getSystemPrompt() is updated to prioritize custom
// prompts over plan mode prompts.
func TestGetSystemPrompt_CustomPromptTakesPrecedenceOverPlanMode(t *testing.T) {
	// Setup: create adapter
	adapter := &AnthropicAdapter{
		model:        "test-model",
		skillManager: nil,
	}

	// Expected prompts
	customPrompt := "You are a debugging specialist. Analyze stack traces and find root causes."
	planPath := ".agent/plans/session-456.md"

	// Setup: create context with BOTH custom prompt AND plan mode
	ctx := context.Background()
	ctx = port.WithCustomSystemPrompt(ctx, port.CustomSystemPromptInfo{
		SessionID: "test-session-456",
		Prompt:    customPrompt,
	})
	ctx = port.WithPlanMode(ctx, port.PlanModeInfo{
		Enabled:   true,
		SessionID: "test-session-456",
		PlanPath:  planPath,
	})

	// Execute: get system prompt
	actualPrompt := adapter.getSystemPrompt(ctx)

	// Assert: should return custom prompt, not plan mode prompt
	if actualPrompt != customPrompt {
		t.Errorf(
			"Expected custom prompt to take precedence over plan mode.\nWant: %q\nGot:  %q",
			customPrompt,
			actualPrompt,
		)
	}

	// Assert: should NOT contain plan mode instructions
	if containsPlanModeInstructions(actualPrompt) {
		t.Error("Custom prompt should take precedence over plan mode, but got plan mode instructions")
	}

	// Assert: should NOT contain the plan path
	if containsString(actualPrompt, planPath) {
		t.Errorf("Custom prompt should not contain plan path %q, but it does", planPath)
	}
}

// TestGetSystemPrompt_EmptyCustomPromptFallsBackToBasePrompt verifies that
// when a custom system prompt is present in context but has an empty Prompt field,
// the system falls back to the base prompt behavior.
//
// This test will FAIL until getSystemPrompt() is updated to validate that the
// custom prompt is non-empty before using it.
func TestGetSystemPrompt_EmptyCustomPromptFallsBackToBasePrompt(t *testing.T) {
	// Setup: create adapter
	adapter := &AnthropicAdapter{
		model:        "test-model",
		skillManager: nil,
	}

	// Setup: create context with empty custom prompt
	ctx := port.WithCustomSystemPrompt(
		context.Background(),
		port.CustomSystemPromptInfo{
			SessionID: "test-session-789",
			Prompt:    "", // Empty prompt should fall back to base
		},
	)

	// Execute: get system prompt
	actualPrompt := adapter.getSystemPrompt(ctx)

	// Expected: base prompt
	expectedPrompt := adapter.buildBasePromptWithSkills()

	// Assert: should return base prompt when custom prompt is empty
	if actualPrompt != expectedPrompt {
		t.Errorf("Expected base prompt when custom prompt is empty.\nWant: %q\nGot:  %q", expectedPrompt, actualPrompt)
	}
}

// TestGetSystemPrompt_NoCustomPromptWithPlanModeReturnsPlanPrompt verifies that
// when there is NO custom prompt in context but plan mode IS enabled, the plan
// mode prompt is returned (existing behavior should continue to work).
//
// This test verifies backward compatibility - plan mode should still work when
// no custom prompt is present.
func TestGetSystemPrompt_NoCustomPromptWithPlanModeReturnsPlanPrompt(t *testing.T) {
	// Setup: create adapter
	adapter := &AnthropicAdapter{
		model:        "test-model",
		skillManager: nil,
	}

	planPath := ".agent/plans/session-abc.md"

	// Setup: create context with plan mode but NO custom prompt
	ctx := port.WithPlanMode(
		context.Background(),
		port.PlanModeInfo{
			Enabled:   true,
			SessionID: "test-session-abc",
			PlanPath:  planPath,
		},
	)

	// Execute: get system prompt
	actualPrompt := adapter.getSystemPrompt(ctx)

	// Assert: should contain plan mode instructions
	if !containsPlanModeInstructions(actualPrompt) {
		t.Error("Expected plan mode prompt when plan mode is enabled and no custom prompt exists")
	}

	// Assert: should contain the plan path
	if !containsString(actualPrompt, planPath) {
		t.Errorf("Expected plan mode prompt to contain plan path %q, but it doesn't", planPath)
	}

	// Assert: should NOT be the base prompt
	basePrompt := adapter.buildBasePromptWithSkills()
	if actualPrompt == basePrompt {
		t.Error("Expected plan mode prompt, but got base prompt instead")
	}
}

// TestGetSystemPrompt_NoCustomPromptNoPlanModeReturnsBasePrompt verifies that
// when there is NO custom prompt and NO plan mode in context, the base prompt
// is returned (existing behavior should continue to work).
//
// This test verifies backward compatibility - the default behavior should still work.
func TestGetSystemPrompt_NoCustomPromptNoPlanModeReturnsBasePrompt(t *testing.T) {
	// Setup: create adapter
	adapter := &AnthropicAdapter{
		model:        "test-model",
		skillManager: nil,
	}

	// Setup: create plain context with no custom prompt or plan mode
	ctx := context.Background()

	// Execute: get system prompt
	actualPrompt := adapter.getSystemPrompt(ctx)

	// Expected: base prompt
	expectedPrompt := adapter.buildBasePromptWithSkills()

	// Assert: should return base prompt
	if actualPrompt != expectedPrompt {
		t.Errorf(
			"Expected base prompt when no custom prompt or plan mode exists.\nWant: %q\nGot:  %q",
			expectedPrompt,
			actualPrompt,
		)
	}

	// Assert: should NOT contain plan mode instructions
	if containsPlanModeInstructions(actualPrompt) {
		t.Error("Expected base prompt, but got plan mode instructions")
	}
}

// TestGetSystemPrompt_CustomPromptWithWhitespaceIsNotEmpty verifies that
// a custom prompt containing only whitespace is still considered valid and
// takes precedence over other prompts.
//
// This is an edge case test to ensure whitespace-only prompts are treated
// as valid (even if unusual) rather than falling back to base prompt.
func TestGetSystemPrompt_CustomPromptWithWhitespaceIsNotEmpty(t *testing.T) {
	// Setup: create adapter
	adapter := &AnthropicAdapter{
		model:        "test-model",
		skillManager: nil,
	}

	// Custom prompt with only whitespace
	whitespacePrompt := "   \n\t  \n   "

	// Setup: create context with whitespace-only custom prompt
	ctx := port.WithCustomSystemPrompt(
		context.Background(),
		port.CustomSystemPromptInfo{
			SessionID: "test-session-whitespace",
			Prompt:    whitespacePrompt,
		},
	)

	// Execute: get system prompt
	actualPrompt := adapter.getSystemPrompt(ctx)

	// Assert: should return whitespace prompt (even though unusual)
	// The implementation should check for empty string, not trimmed empty
	if actualPrompt != whitespacePrompt {
		t.Errorf("Expected whitespace prompt to be used.\nWant: %q\nGot:  %q", whitespacePrompt, actualPrompt)
	}
}

// TestGetSystemPrompt_CustomPromptSessionIDNotValidated verifies that
// the session ID in CustomSystemPromptInfo is informational only and
// doesn't affect whether the custom prompt is used.
//
// The getSystemPrompt() method should use the custom prompt if present,
// regardless of the session ID value.
func TestGetSystemPrompt_CustomPromptSessionIDNotValidated(t *testing.T) {
	// Setup: create adapter
	adapter := &AnthropicAdapter{
		model:        "test-model",
		skillManager: nil,
	}

	customPrompt := "You are a refactoring specialist."

	tests := []struct {
		name      string
		sessionID string
	}{
		{
			name:      "empty session ID",
			sessionID: "",
		},
		{
			name:      "mismatched session ID",
			sessionID: "different-session-id",
		},
		{
			name:      "valid session ID",
			sessionID: "test-session-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup: create context with custom prompt and varying session IDs
			ctx := port.WithCustomSystemPrompt(
				context.Background(),
				port.CustomSystemPromptInfo{
					SessionID: tt.sessionID,
					Prompt:    customPrompt,
				},
			)

			// Execute: get system prompt
			actualPrompt := adapter.getSystemPrompt(ctx)

			// Assert: should return custom prompt regardless of session ID
			if actualPrompt != customPrompt {
				t.Errorf("Expected custom prompt to be used with session ID %q.\nWant: %q\nGot:  %q",
					tt.sessionID, customPrompt, actualPrompt)
			}
		})
	}
}

// TestGetSystemPrompt_MultipleCustomPromptsInSequence verifies that
// different custom prompts can be used in different calls with different contexts.
//
// This ensures the implementation doesn't cache or persist custom prompts
// inappropriately.
func TestGetSystemPrompt_MultipleCustomPromptsInSequence(t *testing.T) {
	// Setup: create adapter
	adapter := &AnthropicAdapter{
		model:        "test-model",
		skillManager: nil,
	}

	prompt1 := "First custom prompt for session 1"
	prompt2 := "Second custom prompt for session 2"

	// Execute: call with first custom prompt
	ctx1 := port.WithCustomSystemPrompt(
		context.Background(),
		port.CustomSystemPromptInfo{
			SessionID: "session-1",
			Prompt:    prompt1,
		},
	)
	actualPrompt1 := adapter.getSystemPrompt(ctx1)

	// Execute: call with second custom prompt
	ctx2 := port.WithCustomSystemPrompt(
		context.Background(),
		port.CustomSystemPromptInfo{
			SessionID: "session-2",
			Prompt:    prompt2,
		},
	)
	actualPrompt2 := adapter.getSystemPrompt(ctx2)

	// Execute: call with no custom prompt
	ctx3 := context.Background()
	actualPrompt3 := adapter.getSystemPrompt(ctx3)

	// Assert: each call returns the appropriate prompt
	if actualPrompt1 != prompt1 {
		t.Errorf("First call: expected %q, got %q", prompt1, actualPrompt1)
	}
	if actualPrompt2 != prompt2 {
		t.Errorf("Second call: expected %q, got %q", prompt2, actualPrompt2)
	}

	basePrompt := adapter.buildBasePromptWithSkills()
	if actualPrompt3 != basePrompt {
		t.Errorf("Third call: expected base prompt %q, got %q", basePrompt, actualPrompt3)
	}
}

// ============================================================================
// Helper Functions for Custom System Prompt Tests
// ============================================================================

// containsPlanModeInstructions checks if a prompt string contains
// characteristic plan mode instructions.
func containsPlanModeInstructions(prompt string) bool {
	// Check for key phrases that appear in plan mode prompts
	planModeIndicators := []string{
		"PLAN MODE",
		"implementation plan",
		"write your plan to",
		"edit_file tool to write your plan",
	}

	for _, indicator := range planModeIndicators {
		if containsString(prompt, indicator) {
			return true
		}
	}
	return false
}

// containsString checks if a string contains a substring (case-sensitive).
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexOfString(s, substr) >= 0)
}

// indexOfString returns the index of the first occurrence of substr in s,
// or -1 if substr is not present in s.
func indexOfString(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

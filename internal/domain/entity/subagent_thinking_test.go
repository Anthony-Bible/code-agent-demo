package entity

import (
	"testing"
)

// ========================================
// Thinking Configuration Tests - RED PHASE
// ========================================
// These tests verify that ThinkingEnabled and ThinkingBudget fields
// can be parsed from YAML frontmatter and have correct inheritance behavior.

// TestParseSubagentFromYAML_ThinkingEnabled_True verifies that thinking_enabled: true
// is parsed correctly and sets ThinkingEnabled to a non-nil pointer with value true.
func TestParseSubagentFromYAML_ThinkingEnabled_True(t *testing.T) {
	yamlContent := `---
name: thinking-agent
description: Agent with thinking enabled
thinking_enabled: true
---
System prompt with thinking enabled.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent == nil {
		t.Fatal("ParseSubagentFromYAML() returned nil subagent")
	}

	// ThinkingEnabled should be a non-nil pointer to true
	if subagent.ThinkingEnabled == nil {
		t.Fatal("ThinkingEnabled should not be nil when thinking_enabled: true is specified")
	}

	if !*subagent.ThinkingEnabled {
		t.Errorf("ThinkingEnabled = false, want true")
	}
}

// TestParseSubagentFromYAML_ThinkingEnabled_False verifies that thinking_enabled: false
// is parsed correctly and sets ThinkingEnabled to a non-nil pointer with value false.
// This is distinct from omitting the field (which results in nil).
func TestParseSubagentFromYAML_ThinkingEnabled_False(t *testing.T) {
	yamlContent := `---
name: no-thinking-agent
description: Agent with thinking explicitly disabled
thinking_enabled: false
---
System prompt with thinking disabled.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent == nil {
		t.Fatal("ParseSubagentFromYAML() returned nil subagent")
	}

	// ThinkingEnabled should be a non-nil pointer to false
	if subagent.ThinkingEnabled == nil {
		t.Fatal("ThinkingEnabled should not be nil when thinking_enabled: false is specified")
	}

	if *subagent.ThinkingEnabled {
		t.Errorf("ThinkingEnabled = true, want false")
	}
}

// TestParseSubagentFromYAML_ThinkingEnabled_Omitted verifies that when thinking_enabled
// is not specified in the YAML, ThinkingEnabled is nil (inherit behavior).
func TestParseSubagentFromYAML_ThinkingEnabled_Omitted(t *testing.T) {
	yamlContent := `---
name: inherit-thinking-agent
description: Agent that inherits thinking config from parent
model: sonnet
---
System prompt with inherited thinking config.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent == nil {
		t.Fatal("ParseSubagentFromYAML() returned nil subagent")
	}

	// ThinkingEnabled should be nil when not specified (inherit from parent)
	if subagent.ThinkingEnabled != nil {
		t.Errorf("ThinkingEnabled should be nil when omitted, got %v", *subagent.ThinkingEnabled)
	}
}

// TestParseSubagentFromYAML_ThinkingBudget_CustomValue verifies that thinking_budget
// with a specific value is parsed correctly as int64.
func TestParseSubagentFromYAML_ThinkingBudget_CustomValue(t *testing.T) {
	tests := []struct {
		name           string
		yamlContent    string
		expectedBudget int64
	}{
		{
			name: "budget 1000",
			yamlContent: `---
name: budget-1000-agent
description: Agent with 1000 thinking budget
thinking_budget: 1000
---
Content.`,
			expectedBudget: 1000,
		},
		{
			name: "budget 5000",
			yamlContent: `---
name: budget-5000-agent
description: Agent with 5000 thinking budget
thinking_budget: 5000
---
Content.`,
			expectedBudget: 5000,
		},
		{
			name: "budget 15000",
			yamlContent: `---
name: budget-15000-agent
description: Agent with 15000 thinking budget
thinking_budget: 15000
---
Content.`,
			expectedBudget: 15000,
		},
		{
			name: "budget 30000",
			yamlContent: `---
name: budget-30000-agent
description: Agent with 30000 thinking budget
thinking_budget: 30000
---
Content.`,
			expectedBudget: 30000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subagent, err := ParseSubagentFromYAML(tt.yamlContent)
			if err != nil {
				t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
			}

			if subagent == nil {
				t.Fatal("ParseSubagentFromYAML() returned nil subagent")
			}

			if subagent.ThinkingBudget != tt.expectedBudget {
				t.Errorf("ThinkingBudget = %d, want %d", subagent.ThinkingBudget, tt.expectedBudget)
			}
		})
	}
}

// TestParseSubagentFromYAML_ThinkingBudget_Zero verifies that thinking_budget: 0
// is parsed correctly as 0 (explicit zero, meaning inherit from parent).
func TestParseSubagentFromYAML_ThinkingBudget_Zero(t *testing.T) {
	yamlContent := `---
name: zero-budget-agent
description: Agent with zero thinking budget (inherit)
thinking_budget: 0
---
Content.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent == nil {
		t.Fatal("ParseSubagentFromYAML() returned nil subagent")
	}

	// ThinkingBudget should be 0 when explicitly set to 0
	if subagent.ThinkingBudget != 0 {
		t.Errorf("ThinkingBudget = %d, want 0", subagent.ThinkingBudget)
	}
}

// TestParseSubagentFromYAML_ThinkingBudget_Omitted verifies that when thinking_budget
// is not specified, it defaults to 0 (inherit behavior).
func TestParseSubagentFromYAML_ThinkingBudget_Omitted(t *testing.T) {
	yamlContent := `---
name: no-budget-agent
description: Agent without thinking_budget specified
model: sonnet
---
Content.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent == nil {
		t.Fatal("ParseSubagentFromYAML() returned nil subagent")
	}

	// ThinkingBudget should be 0 (zero value) when not specified
	if subagent.ThinkingBudget != 0 {
		t.Errorf("ThinkingBudget = %d, want 0 (default)", subagent.ThinkingBudget)
	}
}

// TestParseSubagentFromYAML_ThinkingFields_Combined verifies that both
// thinking_enabled and thinking_budget can be specified together.
func TestParseSubagentFromYAML_ThinkingFields_Combined(t *testing.T) {
	yamlContent := `---
name: full-thinking-agent
description: Agent with both thinking fields specified
thinking_enabled: true
thinking_budget: 15000
---
System prompt with full thinking configuration.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent == nil {
		t.Fatal("ParseSubagentFromYAML() returned nil subagent")
	}

	// Verify ThinkingEnabled is true
	if subagent.ThinkingEnabled == nil {
		t.Fatal("ThinkingEnabled should not be nil")
	}
	if !*subagent.ThinkingEnabled {
		t.Errorf("ThinkingEnabled = false, want true")
	}

	// Verify ThinkingBudget is 15000
	if subagent.ThinkingBudget != 15000 {
		t.Errorf("ThinkingBudget = %d, want 15000", subagent.ThinkingBudget)
	}
}

// TestParseSubagentFromYAML_CompleteWithThinking verifies a complete AGENT.md
// example with all fields including thinking configuration.
func TestParseSubagentFromYAML_CompleteWithThinking(t *testing.T) {
	yamlContent := `---
name: code-reviewer
description: Expert code reviewer for security and best practices analysis
model: sonnet
max_actions: 20
thinking_enabled: true
thinking_budget: 15000
allowed-tools: read_file list_files grep
---
You are an expert code reviewer specializing in security analysis.
Your role is to identify vulnerabilities and suggest improvements.

Focus on:
- Security issues (SQL injection, XSS, etc.)
- Best practices violations
- Performance concerns
- Code maintainability

Provide clear, actionable feedback with specific line references.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent == nil {
		t.Fatal("ParseSubagentFromYAML() returned nil subagent")
	}

	// Verify all fields are parsed correctly
	if subagent.Name != "code-reviewer" {
		t.Errorf("Name = %v, want 'code-reviewer'", subagent.Name)
	}

	if subagent.Description != "Expert code reviewer for security and best practices analysis" {
		t.Errorf(
			"Description = %v, want 'Expert code reviewer for security and best practices analysis'",
			subagent.Description,
		)
	}

	if subagent.Model != "sonnet" {
		t.Errorf("Model = %v, want 'sonnet'", subagent.Model)
	}

	if subagent.MaxActions != 20 {
		t.Errorf("MaxActions = %v, want 20", subagent.MaxActions)
	}

	// Verify thinking fields
	if subagent.ThinkingEnabled == nil {
		t.Fatal("ThinkingEnabled should not be nil")
	}
	if !*subagent.ThinkingEnabled {
		t.Errorf("ThinkingEnabled = false, want true")
	}

	if subagent.ThinkingBudget != 15000 {
		t.Errorf("ThinkingBudget = %d, want 15000", subagent.ThinkingBudget)
	}

	// Verify allowed tools
	if subagent.AllowedTools == nil || len(subagent.AllowedTools) != 3 {
		t.Errorf("AllowedTools = %v, want 3 tools", subagent.AllowedTools)
	} else {
		expectedTools := []string{"read_file", "list_files", "grep"}
		for i, expected := range expectedTools {
			if subagent.AllowedTools[i] != expected {
				t.Errorf("AllowedTools[%d] = %v, want %v", i, subagent.AllowedTools[i], expected)
			}
		}
	}

	// Verify content is preserved
	if subagent.RawContent == "" {
		t.Error("RawContent should not be empty")
	}
}

// TestParseSubagentMetadataFromYAML_WithThinkingFields verifies that thinking fields
// are parsed correctly in metadata-only mode.
func TestParseSubagentMetadataFromYAML_WithThinkingFields(t *testing.T) {
	yamlContent := `---
name: metadata-thinking-agent
description: Agent metadata with thinking config
thinking_enabled: true
thinking_budget: 10000
---
This content should be ignored in metadata-only parsing.`

	subagent, err := ParseSubagentMetadataFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentMetadataFromYAML() returned unexpected error: %v", err)
	}

	if subagent == nil {
		t.Fatal("ParseSubagentMetadataFromYAML() returned nil subagent")
	}

	// Verify thinking fields are parsed
	if subagent.ThinkingEnabled == nil {
		t.Fatal("ThinkingEnabled should not be nil")
	}
	if !*subagent.ThinkingEnabled {
		t.Errorf("ThinkingEnabled = false, want true")
	}

	if subagent.ThinkingBudget != 10000 {
		t.Errorf("ThinkingBudget = %d, want 10000", subagent.ThinkingBudget)
	}

	// Verify content is NOT loaded (metadata-only)
	if subagent.RawContent != "" {
		t.Errorf("RawContent should be empty for metadata-only parsing, got: %v", subagent.RawContent)
	}
}

// TestParseSubagentFromYAML_ThinkingEnabled_NilVsFalse verifies the critical
// distinction between nil (inherit) and false (explicitly disabled).
func TestParseSubagentFromYAML_ThinkingEnabled_NilVsFalse(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		expectNil   bool
		expectValue bool // only checked if expectNil is false
		description string
	}{
		{
			name: "explicitly true",
			yamlContent: `---
name: explicit-true-agent
description: Thinking explicitly enabled
thinking_enabled: true
---
Content.`,
			expectNil:   false,
			expectValue: true,
			description: "thinking_enabled: true should result in non-nil pointer to true",
		},
		{
			name: "explicitly false",
			yamlContent: `---
name: explicit-false-agent
description: Thinking explicitly disabled
thinking_enabled: false
---
Content.`,
			expectNil:   false,
			expectValue: false,
			description: "thinking_enabled: false should result in non-nil pointer to false",
		},
		{
			name: "omitted (inherit)",
			yamlContent: `---
name: inherit-agent
description: Thinking inherited from parent
---
Content.`,
			expectNil:   true,
			expectValue: false, // not checked
			description: "omitted thinking_enabled should result in nil (inherit)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subagent, err := ParseSubagentFromYAML(tt.yamlContent)
			if err != nil {
				t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
			}

			if tt.expectNil {
				if subagent.ThinkingEnabled != nil {
					t.Errorf("%s: ThinkingEnabled should be nil, got %v", tt.description, *subagent.ThinkingEnabled)
				}
			} else {
				if subagent.ThinkingEnabled == nil {
					t.Fatalf("%s: ThinkingEnabled should not be nil", tt.description)
				}
				if *subagent.ThinkingEnabled != tt.expectValue {
					t.Errorf("%s: ThinkingEnabled = %v, want %v", tt.description, *subagent.ThinkingEnabled, tt.expectValue)
				}
			}
		})
	}
}

// TestParseSubagentFromYAML_ThinkingBudget_VariousValues verifies that
// thinking_budget handles edge cases and various numeric values correctly.
func TestParseSubagentFromYAML_ThinkingBudget_VariousValues(t *testing.T) {
	tests := []struct {
		name           string
		yamlContent    string
		expectedBudget int64
	}{
		{
			name: "minimum value 1",
			yamlContent: `---
name: min-budget-agent
description: Agent with minimum thinking budget
thinking_budget: 1
---
Content.`,
			expectedBudget: 1,
		},
		{
			name: "small value 100",
			yamlContent: `---
name: small-budget-agent
description: Agent with small thinking budget
thinking_budget: 100
---
Content.`,
			expectedBudget: 100,
		},
		{
			name: "medium value 8000",
			yamlContent: `---
name: medium-budget-agent
description: Agent with medium thinking budget
thinking_budget: 8000
---
Content.`,
			expectedBudget: 8000,
		},
		{
			name: "large value 50000",
			yamlContent: `---
name: large-budget-agent
description: Agent with large thinking budget
thinking_budget: 50000
---
Content.`,
			expectedBudget: 50000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subagent, err := ParseSubagentFromYAML(tt.yamlContent)
			if err != nil {
				t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
			}

			if subagent.ThinkingBudget != tt.expectedBudget {
				t.Errorf("ThinkingBudget = %d, want %d", subagent.ThinkingBudget, tt.expectedBudget)
			}
		})
	}
}

// TestParseSubagentFromYAML_ThinkingFields_OnlyEnabled verifies that
// thinking_enabled can be specified without thinking_budget.
func TestParseSubagentFromYAML_ThinkingFields_OnlyEnabled(t *testing.T) {
	yamlContent := `---
name: only-enabled-agent
description: Agent with only thinking_enabled specified
thinking_enabled: true
---
Content.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent.ThinkingEnabled == nil {
		t.Fatal("ThinkingEnabled should not be nil")
	}
	if !*subagent.ThinkingEnabled {
		t.Errorf("ThinkingEnabled = false, want true")
	}

	// ThinkingBudget should be 0 (default/inherit)
	if subagent.ThinkingBudget != 0 {
		t.Errorf("ThinkingBudget = %d, want 0 (default when omitted)", subagent.ThinkingBudget)
	}
}

// TestParseSubagentFromYAML_ThinkingFields_OnlyBudget verifies that
// thinking_budget can be specified without thinking_enabled.
func TestParseSubagentFromYAML_ThinkingFields_OnlyBudget(t *testing.T) {
	yamlContent := `---
name: only-budget-agent
description: Agent with only thinking_budget specified
thinking_budget: 12000
---
Content.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	// ThinkingEnabled should be nil (inherit)
	if subagent.ThinkingEnabled != nil {
		t.Errorf("ThinkingEnabled should be nil when omitted, got %v", *subagent.ThinkingEnabled)
	}

	// ThinkingBudget should be 12000
	if subagent.ThinkingBudget != 12000 {
		t.Errorf("ThinkingBudget = %d, want 12000", subagent.ThinkingBudget)
	}
}

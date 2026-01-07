package entity

import (
	"strings"
	"testing"
)

// ========================================
// YAML Parsing Tests
// ========================================

func TestParseSubagentFromYAML_Success(t *testing.T) {
	simpleYAML := `---
name: my-subagent
description: This is a subagent that does things
---
This is the subagent system prompt content after the frontmatter.`

	subagent, err := ParseSubagentFromYAML(simpleYAML)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent == nil {
		t.Fatal("ParseSubagentFromYAML() returned nil subagent")
	}

	if subagent.Name != "my-subagent" {
		t.Errorf("ParseSubagentFromYAML() Name = %v, want 'my-subagent'", subagent.Name)
	}

	if subagent.Description != "This is a subagent that does things" {
		t.Errorf(
			"ParseSubagentFromYAML() Description = %v, want 'This is a subagent that does things'",
			subagent.Description,
		)
	}

	if subagent.RawContent != "This is the subagent system prompt content after the frontmatter." {
		t.Errorf(
			"ParseSubagentFromYAML() RawContent = %v, want 'This is the subagent system prompt content after the frontmatter.'",
			subagent.RawContent,
		)
	}
}

func TestParseSubagentFromYAML_CompleteMetadata(t *testing.T) {
	completeYAML := `---
name: complete-subagent
description: A subagent with all metadata fields
model: sonnet
max_actions: 50
allowed-tools: bash read_file write_file
---
This subagent has complete metadata including optional fields.`

	subagent, err := ParseSubagentFromYAML(completeYAML)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent == nil {
		t.Fatal("ParseSubagentFromYAML() returned nil subagent")
	}

	// Required fields
	if subagent.Name != "complete-subagent" {
		t.Errorf("ParseSubagentFromYAML() Name = %v, want 'complete-subagent'", subagent.Name)
	}

	if subagent.Description != "A subagent with all metadata fields" {
		t.Errorf(
			"ParseSubagentFromYAML() Description = %v, want 'A subagent with all metadata fields'",
			subagent.Description,
		)
	}

	// Optional fields
	if subagent.Model != "sonnet" {
		t.Errorf("ParseSubagentFromYAML() Model = %v, want 'sonnet'", subagent.Model)
	}

	if subagent.MaxActions != 50 {
		t.Errorf("ParseSubagentFromYAML() MaxActions = %v, want 50", subagent.MaxActions)
	}

	// Check allowed-tools parsing (space-delimited string)
	if subagent.AllowedTools == nil || len(subagent.AllowedTools) != 3 {
		t.Errorf("ParseSubagentFromYAML() AllowedTools = %v, want 3 tools", subagent.AllowedTools)
	} else {
		expectedTools := []string{"bash", "read_file", "write_file"}
		for i, expected := range expectedTools {
			if subagent.AllowedTools[i] != expected {
				t.Errorf("ParseSubagentFromYAML() AllowedTools[%d] = %v, want %v", i, subagent.AllowedTools[i], expected)
			}
		}
	}

	// Check content becomes SystemPrompt
	if subagent.RawContent != "This subagent has complete metadata including optional fields." {
		t.Errorf(
			"ParseSubagentFromYAML() RawContent = %v, want 'This subagent has complete metadata including optional fields.'",
			subagent.RawContent,
		)
	}
}

func TestParseSubagentFromYAML_AllowedToolsAsArray(t *testing.T) {
	yamlWithArray := `---
name: array-tools-subagent
description: Subagent with allowed-tools as array
allowed-tools:
  - bash
  - read_file
  - list_files
  - edit_file
---
Content here.`

	subagent, err := ParseSubagentFromYAML(yamlWithArray)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent == nil {
		t.Fatal("ParseSubagentFromYAML() returned nil subagent")
	}

	// Check allowed-tools parsing (array format)
	if subagent.AllowedTools == nil || len(subagent.AllowedTools) != 4 {
		t.Errorf("ParseSubagentFromYAML() AllowedTools = %v, want 4 tools", subagent.AllowedTools)
	} else {
		expectedTools := []string{"bash", "read_file", "list_files", "edit_file"}
		for i, expected := range expectedTools {
			if subagent.AllowedTools[i] != expected {
				t.Errorf("ParseSubagentFromYAML() AllowedTools[%d] = %v, want %v", i, subagent.AllowedTools[i], expected)
			}
		}
	}
}

func TestParseSubagentFromYAML_AllowedToolsAsString(t *testing.T) {
	yamlWithString := `---
name: string-tools-subagent
description: Subagent with allowed-tools as space-delimited string
allowed-tools: tool1 tool2 tool3
---
Content here.`

	subagent, err := ParseSubagentFromYAML(yamlWithString)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent == nil {
		t.Fatal("ParseSubagentFromYAML() returned nil subagent")
	}

	// Check allowed-tools parsing (string format)
	if subagent.AllowedTools == nil || len(subagent.AllowedTools) != 3 {
		t.Errorf("ParseSubagentFromYAML() AllowedTools = %v, want 3 tools", subagent.AllowedTools)
	} else {
		expectedTools := []string{"tool1", "tool2", "tool3"}
		for i, expected := range expectedTools {
			if subagent.AllowedTools[i] != expected {
				t.Errorf("ParseSubagentFromYAML() AllowedTools[%d] = %v, want %v", i, subagent.AllowedTools[i], expected)
			}
		}
	}
}

func TestParseSubagentFromYAML_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "missing name field",
			yaml: `---
description: A subagent without name
---
content`,
			wantErr: true,
		},
		{
			name: "missing description field",
			yaml: `---
name: subagent-without-description
---
content`,
			wantErr: true,
		},
		{
			name: "empty name field",
			yaml: `---
name: ""
description: A subagent with empty name
---
content`,
			wantErr: true,
		},
		{
			name: "empty description field",
			yaml: `---
name: subagent-name
description: ""
---
content`,
			wantErr: true,
		},
		{
			name:    "no frontmatter at all",
			yaml:    `just content without frontmatter`,
			wantErr: true,
		},
		{
			name: "missing closing frontmatter delimiter",
			yaml: `---
name: incomplete
description: Missing closing delimiter
content without proper frontmatter end`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSubagentFromYAML(tt.yaml)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSubagentFromYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseSubagentFromYAML_InvalidYAMLSyntax(t *testing.T) {
	invalidYAML := `---
name: valid-name
description: valid description
invalid: yaml: syntax: error
---
content here`

	_, err := ParseSubagentFromYAML(invalidYAML)

	if err == nil {
		t.Error("ParseSubagentFromYAML() should return error for malformed YAML, got nil")
	}
}

func TestParseSubagentMetadataFromYAML_OnlyMetadata(t *testing.T) {
	yamlContent := `---
name: test-subagent
description: A test subagent
model: haiku
max_actions: 30
allowed-tools: bash read_file
---
This content should be ignored and not stored in RawContent.
More content here.
`

	subagent, err := ParseSubagentMetadataFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentMetadataFromYAML() returned unexpected error: %v", err)
	}

	if subagent == nil {
		t.Fatal("ParseSubagentMetadataFromYAML() returned nil subagent")
	}

	// Verify metadata fields are parsed correctly
	if subagent.Name != "test-subagent" {
		t.Errorf("Name = %v, want 'test-subagent'", subagent.Name)
	}

	if subagent.Description != "A test subagent" {
		t.Errorf("Description = %v, want 'A test subagent'", subagent.Description)
	}

	if subagent.Model != "haiku" {
		t.Errorf("Model = %v, want 'haiku'", subagent.Model)
	}

	if subagent.MaxActions != 30 {
		t.Errorf("MaxActions = %v, want 30", subagent.MaxActions)
	}

	// Verify allowed-tools is parsed
	if subagent.AllowedTools == nil || len(subagent.AllowedTools) != 2 {
		t.Errorf("AllowedTools should have 2 entries, got %v", subagent.AllowedTools)
	}

	// Verify RawContent is empty (progressive disclosure - content not loaded)
	if subagent.RawContent != "" {
		t.Errorf("RawContent should be empty for metadata-only parsing, got: %v", subagent.RawContent)
	}

	// Verify RawFrontmatter is populated
	if subagent.RawFrontmatter == "" {
		t.Error("RawFrontmatter should be populated")
	}
}

func TestParseSubagentMetadataFromYAML_EmptyContent(t *testing.T) {
	yamlContent := `---
name: minimal-subagent
description: A minimal subagent
---
`

	subagent, err := ParseSubagentMetadataFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentMetadataFromYAML() returned unexpected error: %v", err)
	}

	if subagent.RawContent != "" {
		t.Errorf("RawContent should be empty, got: %v", subagent.RawContent)
	}
}

// ========================================
// Validation Tests - Name
// ========================================

func TestSubagent_Validation_EmptyName(t *testing.T) {
	subagent := Subagent{
		Name:        "",
		Description: "A valid subagent description",
		Model:       "sonnet",
	}

	err := subagent.Validate()

	if err == nil {
		t.Error("Subagent.Validate() should return error when name is empty, got nil")
	}

	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "name") {
		t.Errorf("Subagent.Validate() error should mention 'name', got: %v", err)
	}
}

func TestSubagent_Validation_NameTooLong(t *testing.T) {
	// Name with 65 characters (exceeds 64 char limit)
	longName := strings.Repeat("a", 65)
	subagent := Subagent{
		Name:        longName,
		Description: "A valid description",
		Model:       "sonnet",
	}

	err := subagent.Validate()

	if err == nil {
		t.Error("Subagent.Validate() should return error when name exceeds 64 characters, got nil")
	}
}

func TestSubagent_Validation_NameStartsWithHyphen(t *testing.T) {
	subagent := Subagent{
		Name:        "-invalid-name",
		Description: "A valid description",
		Model:       "sonnet",
	}

	err := subagent.Validate()

	if err == nil {
		t.Error("Subagent.Validate() should return error when name starts with hyphen, got nil")
	}
}

func TestSubagent_Validation_NameEndsWithHyphen(t *testing.T) {
	subagent := Subagent{
		Name:        "invalid-name-",
		Description: "A valid description",
		Model:       "sonnet",
	}

	err := subagent.Validate()

	if err == nil {
		t.Error("Subagent.Validate() should return error when name ends with hyphen, got nil")
	}
}

func TestSubagent_Validation_NameConsecutiveHyphens(t *testing.T) {
	subagent := Subagent{
		Name:        "invalid--name",
		Description: "A valid description",
		Model:       "sonnet",
	}

	err := subagent.Validate()

	if err == nil {
		t.Error("Subagent.Validate() should return error when name contains consecutive hyphens, got nil")
	}
}

func TestSubagent_Validation_NameInvalidCharacters(t *testing.T) {
	tests := []struct {
		name         string
		subagentName string
	}{
		{
			name:         "uppercase letters",
			subagentName: "Invalid-Name",
		},
		{
			name:         "underscores",
			subagentName: "invalid_name",
		},
		{
			name:         "spaces",
			subagentName: "invalid name",
		},
		{
			name:         "special characters",
			subagentName: "invalid@name",
		},
		{
			name:         "dots",
			subagentName: "invalid.name",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subagent := Subagent{
				Name:        tt.subagentName,
				Description: "A valid description",
				Model:       "sonnet",
			}

			err := subagent.Validate()

			if err == nil {
				t.Errorf("Subagent.Validate() should return error for name with %s, got nil", tt.name)
			}
		})
	}
}

func TestSubagent_Validation_NameValid(t *testing.T) {
	tests := []struct {
		name         string
		subagentName string
	}{
		{
			name:         "simple lowercase",
			subagentName: "simple",
		},
		{
			name:         "with single hyphen",
			subagentName: "valid-name",
		},
		{
			name:         "with multiple hyphens",
			subagentName: "valid-sub-agent-name",
		},
		{
			name:         "with numbers",
			subagentName: "agent123",
		},
		{
			name:         "mix of all valid chars",
			subagentName: "my-agent-v2-beta",
		},
		{
			name:         "single character",
			subagentName: "a",
		},
		{
			name:         "exactly 64 characters",
			subagentName: strings.Repeat("a", 64),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subagent := Subagent{
				Name:        tt.subagentName,
				Description: "A valid description",
				Model:       "sonnet",
			}

			err := subagent.Validate()
			if err != nil {
				t.Errorf(
					"Subagent.Validate() should not return error for valid name '%s', got: %v",
					tt.subagentName,
					err,
				)
			}
		})
	}
}

// ========================================
// Validation Tests - Description
// ========================================

func TestSubagent_Validation_EmptyDescription(t *testing.T) {
	subagent := Subagent{
		Name:        "valid-subagent-name",
		Description: "",
		Model:       "sonnet",
	}

	err := subagent.Validate()

	if err == nil {
		t.Error("Subagent.Validate() should return error when description is empty, got nil")
	}

	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "description") {
		t.Errorf("Subagent.Validate() error should mention 'description', got: %v", err)
	}
}

// ========================================
// Validation Tests - Model
// ========================================

func TestSubagent_Validation_ValidModels(t *testing.T) {
	tests := []struct {
		name  string
		model string
	}{
		{
			name:  "inherit model",
			model: "inherit",
		},
		{
			name:  "haiku model",
			model: "haiku",
		},
		{
			name:  "sonnet model",
			model: "sonnet",
		},
		{
			name:  "opus model",
			model: "opus",
		},
		{
			name:  "empty model (defaults to inherit)",
			model: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subagent := Subagent{
				Name:        "valid-name",
				Description: "A valid description",
				Model:       tt.model,
			}

			err := subagent.Validate()
			if err != nil {
				t.Errorf("Subagent.Validate() should not return error for model '%s', got: %v", tt.model, err)
			}
		})
	}
}

func TestSubagent_Validation_InvalidModel(t *testing.T) {
	tests := []struct {
		name  string
		model string
	}{
		{
			name:  "invalid model name",
			model: "gpt-4",
		},
		{
			name:  "random string",
			model: "invalid",
		},
		{
			name:  "uppercase valid model",
			model: "SONNET",
		},
		{
			name:  "misspelled model",
			model: "sonnett",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subagent := Subagent{
				Name:        "valid-name",
				Description: "A valid description",
				Model:       tt.model,
			}

			err := subagent.Validate()

			if err == nil {
				t.Errorf("Subagent.Validate() should return error for invalid model '%s', got nil", tt.model)
			}

			if err != nil && !strings.Contains(strings.ToLower(err.Error()), "model") {
				t.Errorf("Subagent.Validate() error should mention 'model', got: %v", err)
			}
		})
	}
}

// ========================================
// SourceType Tests
// ========================================

func TestSubagent_SourceType_AllTypes(t *testing.T) {
	tests := []struct {
		name       string
		sourceType SubagentSourceType
	}{
		{
			name:       "project source type",
			sourceType: SubagentSourceProject,
		},
		{
			name:       "project-claude source type",
			sourceType: SubagentSourceProjectClaude,
		},
		{
			name:       "user source type",
			sourceType: SubagentSourceUser,
		},
		{
			name:       "programmatic source type",
			sourceType: SubagentSourceProgrammatic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subagent := Subagent{
				Name:        "test-subagent",
				Description: "A test subagent",
				Model:       "sonnet",
				SourceType:  tt.sourceType,
			}

			// Verify source type is set correctly
			if subagent.SourceType != tt.sourceType {
				t.Errorf("SourceType = %v, want %v", subagent.SourceType, tt.sourceType)
			}

			// Validation should still pass
			err := subagent.Validate()
			if err != nil {
				t.Errorf(
					"Validate() should not fail for valid subagent with source type %v, got: %v",
					tt.sourceType,
					err,
				)
			}
		})
	}
}

func TestSubagent_SourceType_Constants(t *testing.T) {
	// Test that source type constants have expected values
	if SubagentSourceProject != "project" {
		t.Errorf("SubagentSourceProject = %v, want 'project'", SubagentSourceProject)
	}

	if SubagentSourceProjectClaude != "project-claude" {
		t.Errorf("SubagentSourceProjectClaude = %v, want 'project-claude'", SubagentSourceProjectClaude)
	}

	if SubagentSourceUser != "user" {
		t.Errorf("SubagentSourceUser = %v, want 'user'", SubagentSourceUser)
	}

	if SubagentSourceProgrammatic != "programmatic" {
		t.Errorf("SubagentSourceProgrammatic = %v, want 'programmatic'", SubagentSourceProgrammatic)
	}
}

// ========================================
// Additional Field Tests
// ========================================

func TestSubagent_MaxActions_Default(t *testing.T) {
	yamlContent := `---
name: test-subagent
description: Subagent without max_actions specified
---
Content.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	// MaxActions should be 0 (zero value) when not specified
	if subagent.MaxActions != 0 {
		t.Errorf("MaxActions = %v, want 0 (default)", subagent.MaxActions)
	}
}

func TestSubagent_MaxActions_CustomValue(t *testing.T) {
	yamlContent := `---
name: test-subagent
description: Subagent with custom max_actions
max_actions: 100
---
Content.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent.MaxActions != 100 {
		t.Errorf("MaxActions = %v, want 100", subagent.MaxActions)
	}
}

func TestSubagent_AllowedTools_Empty(t *testing.T) {
	yamlContent := `---
name: test-subagent
description: Subagent without allowed-tools specified
---
Content.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	// AllowedTools should be nil or empty when not specified
	if len(subagent.AllowedTools) > 0 {
		t.Errorf("AllowedTools should be empty when not specified, got %v", subagent.AllowedTools)
	}
}

func TestSubagent_AllowedTools_EmptyString(t *testing.T) {
	yamlContent := `---
name: test-subagent
description: Subagent with empty allowed-tools string
allowed-tools: ""
---
Content.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	// AllowedTools should be empty for empty string
	if len(subagent.AllowedTools) > 0 {
		t.Errorf("AllowedTools should be empty for empty string, got %v", subagent.AllowedTools)
	}
}

func TestSubagent_RawFrontmatter_Populated(t *testing.T) {
	yamlContent := `---
name: test-subagent
description: Test description
model: haiku
---
Content here.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	if subagent.RawFrontmatter == "" {
		t.Error("RawFrontmatter should be populated")
	}

	// Should contain the YAML content
	if !strings.Contains(subagent.RawFrontmatter, "name: test-subagent") {
		t.Errorf("RawFrontmatter should contain the YAML content, got: %v", subagent.RawFrontmatter)
	}
}

func TestSubagent_Paths_NotInYAML(t *testing.T) {
	// ScriptPath and OriginalPath should not be marshaled/unmarshaled with YAML
	yamlContent := `---
name: test-subagent
description: Test description
---
Content.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	// These should be empty by default from parsing
	if subagent.ScriptPath != "" {
		t.Errorf("ScriptPath should be empty from YAML parsing, got: %v", subagent.ScriptPath)
	}

	if subagent.OriginalPath != "" {
		t.Errorf("OriginalPath should be empty from YAML parsing, got: %v", subagent.OriginalPath)
	}
}

// ========================================
// Edge Cases
// ========================================

func TestParseSubagentFromYAML_MultilineContent(t *testing.T) {
	yamlContent := `---
name: multiline-subagent
description: Subagent with multiline content
---
This is the first line of the system prompt.
This is the second line.

This is after a blank line.`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	expectedContent := `This is the first line of the system prompt.
This is the second line.

This is after a blank line.`

	if subagent.RawContent != expectedContent {
		t.Errorf("RawContent = %q, want %q", subagent.RawContent, expectedContent)
	}
}

func TestParseSubagentFromYAML_NoContentAfterFrontmatter(t *testing.T) {
	yamlContent := `---
name: no-content-subagent
description: Subagent with no content after frontmatter
---`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	// RawContent should be empty
	if subagent.RawContent != "" {
		t.Errorf("RawContent should be empty when no content after frontmatter, got: %v", subagent.RawContent)
	}
}

func TestParseSubagentFromYAML_WhitespaceHandling(t *testing.T) {
	yamlContent := `---
name: whitespace-test
description: Test whitespace handling
---


   Content with leading/trailing whitespace.


`

	subagent, err := ParseSubagentFromYAML(yamlContent)
	if err != nil {
		t.Fatalf("ParseSubagentFromYAML() returned unexpected error: %v", err)
	}

	// Whitespace should be trimmed from content
	expectedContent := "Content with leading/trailing whitespace."
	if subagent.RawContent != expectedContent {
		t.Errorf("RawContent = %q, want %q", subagent.RawContent, expectedContent)
	}
}

// ========================================
// Edge Case Tests for YAML Parsing
// ========================================

func TestParseSubagentFromYAML_EdgeCaseValues(t *testing.T) {
	tests := []struct {
		name        string
		yaml        string
		wantErr     bool
		expectedErr string
	}{
		{
			name: "negative thinking budget",
			yaml: `---
name: test-agent
description: Test agent
thinking_budget: -1000
---
content`,
			wantErr: false, // Currently accepts negative values
		},
		{
			name: "zero thinking budget",
			yaml: `---
name: test-agent
description: Test agent
thinking_budget: 0
---
content`,
			wantErr: false, // Zero is valid
		},
		{
			name: "extremely large thinking budget",
			yaml: `---
name: test-agent
description: Test agent
thinking_budget: 9999999999999999999
---
content`,
			wantErr: false, // int64 can handle large values
		},
		{
			name: "thinking_budget as string instead of int",
			yaml: `---
name: test-agent
description: Test agent
thinking_budget: "1000"
---
content`,
			wantErr: false, // Type mismatch - string ignored
		},
		{
			name: "thinking_enabled as string instead of bool",
			yaml: `---
name: test-agent
description: Test agent
thinking_enabled: "true"
---
content`,
			wantErr: false, // Type mismatch - string ignored
		},
		{
			name: "max_actions as negative",
			yaml: `---
name: test-agent
description: Test agent
max_actions: -5
---
content`,
			wantErr: false, // Currently accepts negative values
		},
		{
			name: "max_actions as float",
			yaml: `---
name: test-agent
description: Test agent
max_actions: 5.5
---
content`,
			wantErr: false, // Type mismatch - float ignored
		},
		{
			name: "invalid characters in name",
			yaml: `---
name: "test@agent#$%^&*()"
description: Test agent
---
content`,
			wantErr: false, // Currently accepts any name
		},
		{
			name: "null values",
			yaml: `---
name: null
description: Test agent
thinking_budget: null
---
content`,
			wantErr: true, // null name is not allowed - validation error
		},
		{
			name: "empty arrays",
			yaml: `---
name: test-agent
description: Test agent
allowed-tools: []
---
content`,
			wantErr: false, // Empty arrays should be fine
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subagent, err := ParseSubagentFromYAML(tt.yaml)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSubagentFromYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && tt.expectedErr != "" && err != nil {
				if !strings.Contains(err.Error(), tt.expectedErr) {
					t.Errorf("ParseSubagentFromYAML() error = %v, expected to contain %s", err, tt.expectedErr)
				}
			}

			if !tt.wantErr && subagent == nil {
				t.Error("ParseSubagentFromYAML() returned nil subagent when error was not expected")
			}
		})
	}
}

func TestParseSubagentFromYAML_MalformedStructures(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "unterminated mapping",
			yaml: `---
name: test-agent
description: Test agent
invalid_map: {
---
content`,
			wantErr: true,
		},
		{
			name: "nested array without proper closure",
			yaml: `---
name: test-agent
description: Test agent
allowed-tools: [tool1, tool2
---
content`,
			wantErr: true,
		},
		{
			name: "invalid indentation",
			yaml: `---
name: test-agent
description: Test agent
  invalid-indent: value
---
content`,
			wantErr: true, // YAML parser does not allow inconsistent indentation
		},
		{
			name: "single quotes instead of double",
			yaml: `---
name: 'test-agent'
description: 'Test agent'
---
content`,
			wantErr: false, // Single quotes are valid YAML
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSubagentFromYAML(tt.yaml)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSubagentFromYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

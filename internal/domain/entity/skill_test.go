package entity

import (
	"strings"
	"testing"
)

func TestSkill_Validation_EmptyName(t *testing.T) {
	skill := Skill{
		Name:        "",
		Description: "A valid skill description",
	}

	err := skill.Validate()

	if err == nil {
		t.Error("Skill.Validate() should return error when name is empty, got nil")
	}

	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "name") {
		t.Errorf("Skill.Validate() error should mention 'name', got: %v", err)
	}
}

func TestSkill_Validation_EmptyDescription(t *testing.T) {
	skill := Skill{
		Name:        "valid-skill-name",
		Description: "",
	}

	err := skill.Validate()

	if err == nil {
		t.Error("Skill.Validate() should return error when description is empty, got nil")
	}

	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "description") {
		t.Errorf("Skill.Validate() error should mention 'description', got: %v", err)
	}
}

func TestSkill_Validation_InvalidYAMLFrontmatter(t *testing.T) {
	// Test invalid YAML that will fail parsing
	// This should return error when trying to parse malformed YAML
	invalidYAML := `---
name: valid-name
description: valid description
invalid: yaml: syntax: error
---
content here`

	_, err := ParseSkillFromYAML(invalidYAML)

	if err == nil {
		t.Error("ParseSkillFromYAML() should return error for malformed YAML, got nil")
	}
}

func TestSkill_ParsingYAMLFrontmatter_Success(t *testing.T) {
	simpleYAML := `---
name: my-cool-skill
description: This is a cool skill description
---
This is the skill content after the frontmatter.`

	skill, err := ParseSkillFromYAML(simpleYAML)
	if err != nil {
		t.Fatalf("ParseSkillFromYAML() returned unexpected error: %v", err)
	}

	if skill == nil {
		t.Fatal("ParseSkillFromYAML() returned nil skill")
	}

	if skill.Name != "my-cool-skill" {
		t.Errorf("ParseSkillFromYAML() Name = %v, want 'my-cool-skill'", skill.Name)
	}

	if skill.Description != "This is a cool skill description" {
		t.Errorf("ParseSkillFromYAML() Description = %v, want 'This is a cool skill description'", skill.Description)
	}

	if skill.RawContent != "This is the skill content after the frontmatter." {
		t.Errorf(
			"ParseSkillFromYAML() RawContent = %v, want 'This is the skill content after the frontmatter.'",
			skill.RawContent,
		)
	}
}

func TestSkill_ParsingYAMLFrontmatter_MissingRequiredFields(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr bool
	}{
		{
			name: "missing name field",
			yaml: `---
description: A skill without name
---
content`,
			wantErr: true,
		},
		{
			name: "missing description field",
			yaml: `---
name: skill-without-description
---
content`,
			wantErr: true,
		},
		{
			name: "empty name field",
			yaml: `---
name: ""
description: A skill with empty name
---
content`,
			wantErr: true,
		},
		{
			name: "empty description field",
			yaml: `---
name: skill-name
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseSkillFromYAML(tt.yaml)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSkillFromYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSkill_ParsingYAMLFrontmatter_CompleteMetadata(t *testing.T) {
	completeYAML := `---
name: complete-skill
description: A skill with all metadata fields
license: MIT
compatibility: claude-3.5-sonnet
metadata:
  author: "Skill Author"
  version: "1.0.0"
  category: "utility"
allowed-tools: bash read_file write_file
---
This skill has complete metadata including optional fields.`

	skill, err := ParseSkillFromYAML(completeYAML)
	if err != nil {
		t.Fatalf("ParseSkillFromYAML() returned unexpected error: %v", err)
	}

	if skill == nil {
		t.Fatal("ParseSkillFromYAML() returned nil skill")
	}

	// Required fields
	if skill.Name != "complete-skill" {
		t.Errorf("ParseSkillFromYAML() Name = %v, want 'complete-skill'", skill.Name)
	}

	if skill.Description != "A skill with all metadata fields" {
		t.Errorf("ParseSkillFromYAML() Description = %v, want 'A skill with all metadata fields'", skill.Description)
	}

	// Optional fields
	if skill.License != "MIT" {
		t.Errorf("ParseSkillFromYAML() License = %v, want 'MIT'", skill.License)
	}

	if skill.Compatibility != "claude-3.5-sonnet" {
		t.Errorf("ParseSkillFromYAML() Compatibility = %v, want 'claude-3.5-sonnet'", skill.Compatibility)
	}

	// Validate metadata map
	if skill.Metadata == nil {
		t.Error("ParseSkillFromYAML() Metadata should not be nil")
	} else {
		if skill.Metadata["author"] != "Skill Author" {
			t.Errorf("ParseSkillFromYAML() Metadata[author] = %v, want 'Skill Author'", skill.Metadata["author"])
		}
		if skill.Metadata["version"] != "1.0.0" {
			t.Errorf("ParseSkillFromYAML() Metadata[version] = %v, want '1.0.0'", skill.Metadata["version"])
		}
		if skill.Metadata["category"] != "utility" {
			t.Errorf("ParseSkillFromYAML() Metadata[category] = %v, want 'utility'", skill.Metadata["category"])
		}
	}

	// Check allowed-tools parsing
	if skill.AllowedTools == nil || len(skill.AllowedTools) != 3 {
		t.Errorf("ParseSkillFromYAML() AllowedTools = %v, want 3 tools", skill.AllowedTools)
	} else {
		expectedTools := []string{"bash", "read_file", "write_file"}
		for i, expected := range expectedTools {
			if skill.AllowedTools[i] != expected {
				t.Errorf("ParseSkillFromYAML() AllowedTools[%d] = %v, want %v", i, skill.AllowedTools[i], expected)
			}
		}
	}

	// Check content
	if skill.RawContent != "This skill has complete metadata including optional fields." {
		t.Errorf(
			"ParseSkillFromYAML() RawContent = %v, want 'This skill has complete metadata including optional fields.'",
			skill.RawContent,
		)
	}
}

func TestSkillMetadata_AllFields(t *testing.T) {
	metadata := SkillMetadata{
		License:       "Apache-2.0",
		Compatibility: "anthropic/claude-3",
		Metadata: map[string]string{
			"author":  "Test Author",
			"version": "2.1.0",
		},
	}

	// Verify all fields are set correctly
	if metadata.License != "Apache-2.0" {
		t.Errorf("SkillMetadata.License = %v, want 'Apache-2.0'", metadata.License)
	}

	if metadata.Compatibility != "anthropic/claude-3" {
		t.Errorf("SkillMetadata.Compatibility = %v, want 'anthropic/claude-3'", metadata.Compatibility)
	}

	if metadata.Metadata == nil {
		t.Error("SkillMetadata.Metadata should not be nil")
	} else {
		if metadata.Metadata["author"] != "Test Author" {
			t.Errorf("SkillMetadata.Metadata[author] = %v, want 'Test Author'", metadata.Metadata["author"])
		}
		if metadata.Metadata["version"] != "2.1.0" {
			t.Errorf("SkillMetadata.Metadata[version] = %v, want '2.1.0'", metadata.Metadata["version"])
		}
	}
}

func TestSkillMetadata_OptionalFields_Empty(t *testing.T) {
	metadata := SkillMetadata{
		// All optional fields should be empty/nil by default
	}

	// Verify optional fields are empty
	if metadata.License != "" {
		t.Errorf("SkillMetadata.License = %v, want empty string", metadata.License)
	}

	if metadata.Compatibility != "" {
		t.Errorf("SkillMetadata.Compatibility = %v, want empty string", metadata.Compatibility)
	}

	if metadata.Metadata != nil {
		t.Errorf("SkillMetadata.Metadata = %v, want nil", metadata.Metadata)
	}
}

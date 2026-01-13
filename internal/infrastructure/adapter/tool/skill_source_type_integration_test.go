package tool

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/infrastructure/adapter/file"
	"code-editing-agent/internal/infrastructure/adapter/skill"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSkillActivationShowsCorrectSourceType verifies that skills from different
// directories show the correct source_type in their activation output.
func TestSkillActivationShowsCorrectSourceType(t *testing.T) {
	// Save current directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	// Create temp directory structure for testing
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create project skill (./skills)
	projectSkillDir := filepath.Join(tempDir, "skills", "project-skill")
	if err := os.MkdirAll(projectSkillDir, 0o755); err != nil {
		t.Fatalf("Failed to create project skill directory: %v", err)
	}
	projectSkillContent := `---
name: project-skill
description: A project-level skill
---
# Project Skill

This skill is from ./skills directory.
`
	if err := os.WriteFile(filepath.Join(projectSkillDir, "SKILL.md"), []byte(projectSkillContent), 0o644); err != nil {
		t.Fatalf("Failed to write project skill: %v", err)
	}

	// Create project-claude skill (./.claude/skills)
	claudeSkillDir := filepath.Join(tempDir, ".claude", "skills", "claude-skill")
	if err := os.MkdirAll(claudeSkillDir, 0o755); err != nil {
		t.Fatalf("Failed to create claude skill directory: %v", err)
	}
	claudeSkillContent := `---
name: claude-skill
description: A project-claude-level skill
---
# Claude Skill

This skill is from ./.claude/skills directory.
`
	if err := os.WriteFile(filepath.Join(claudeSkillDir, "SKILL.md"), []byte(claudeSkillContent), 0o644); err != nil {
		t.Fatalf("Failed to write claude skill: %v", err)
	}

	// Create user skill (using temp dir as user home for testing)
	userHomeDir := filepath.Join(tempDir, "user-home")
	userSkillDir := filepath.Join(userHomeDir, ".claude", "skills", "user-skill")
	if err := os.MkdirAll(userSkillDir, 0o755); err != nil {
		t.Fatalf("Failed to create user skill directory: %v", err)
	}
	userSkillContent := `---
name: user-skill
description: A user-level skill
---
# User Skill

This skill is from ~/.claude/skills directory.
`
	if err := os.WriteFile(filepath.Join(userSkillDir, "SKILL.md"), []byte(userSkillContent), 0o644); err != nil {
		t.Fatalf("Failed to write user skill: %v", err)
	}

	// Create skill manager with custom directories
	dirs := []skill.DirConfig{
		{Path: "./skills", SourceType: entity.SkillSourceProject},
		{Path: "./.claude/skills", SourceType: entity.SkillSourceProjectClaude},
		{Path: filepath.Join(userHomeDir, ".claude", "skills"), SourceType: entity.SkillSourceUser},
	}
	skillManager := skill.NewLocalSkillManagerWithDirs(dirs)

	// Create file manager
	fileManager := file.NewLocalFileManager(".")

	// Create tool executor
	toolExecutor := NewExecutorAdapter(fileManager)
	toolExecutor.SetSkillManager(skillManager)

	// Test each skill type
	tests := []struct {
		name               string
		skillName          string
		expectedSourceType string
	}{
		{
			name:               "project skill",
			skillName:          "project-skill",
			expectedSourceType: "source_type: project",
		},
		{
			name:               "project-claude skill",
			skillName:          "claude-skill",
			expectedSourceType: "source_type: project-claude",
		},
		{
			name:               "user skill",
			skillName:          "user-skill",
			expectedSourceType: "source_type: user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := `{"skill_name": "` + tt.skillName + `"}`
			result, err := toolExecutor.ExecuteTool(context.Background(), "activate_skill", []byte(input))
			if err != nil {
				t.Fatalf("Failed to activate skill %s: %v", tt.skillName, err)
			}

			// Verify source_type is present
			if !strings.Contains(result, tt.expectedSourceType) {
				t.Errorf("Expected result to contain '%s', got:\n%s", tt.expectedSourceType, result)
			}

			// Verify directory_path is present
			if !strings.Contains(result, "directory_path:") {
				t.Errorf("Expected result to contain 'directory_path:', got:\n%s", result)
			}

			t.Logf("Activated %s:\n%s", tt.skillName, result)
		})
	}
}

// TestSkillDescriptionShowsSourceType verifies that the activate_skill tool description
// includes source type labels for each skill.
func TestSkillDescriptionShowsSourceType(t *testing.T) {
	// Save current directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	defer func() {
		if err := os.Chdir(originalWd); err != nil {
			t.Errorf("Failed to restore working directory: %v", err)
		}
	}()

	// Create temp directory structure for testing
	tempDir := t.TempDir()
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp directory: %v", err)
	}

	// Create one skill of each type
	projectSkillDir := filepath.Join(tempDir, "skills", "project-skill")
	if err := os.MkdirAll(projectSkillDir, 0o755); err != nil {
		t.Fatalf("Failed to create project skill directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(projectSkillDir, "SKILL.md"), []byte(`---
name: project-skill
description: Project skill
---
# Project Skill
`), 0o644); err != nil {
		t.Fatalf("Failed to write project skill: %v", err)
	}

	claudeSkillDir := filepath.Join(tempDir, ".claude", "skills", "claude-skill")
	if err := os.MkdirAll(claudeSkillDir, 0o755); err != nil {
		t.Fatalf("Failed to create claude skill directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(claudeSkillDir, "SKILL.md"), []byte(`---
name: claude-skill
description: Claude skill
---
# Claude Skill
`), 0o644); err != nil {
		t.Fatalf("Failed to write claude skill: %v", err)
	}

	userHomeDir := filepath.Join(tempDir, "user-home")
	userSkillDir := filepath.Join(userHomeDir, ".claude", "skills", "user-skill")
	if err := os.MkdirAll(userSkillDir, 0o755); err != nil {
		t.Fatalf("Failed to create user skill directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userSkillDir, "SKILL.md"), []byte(`---
name: user-skill
description: User skill
---
# User Skill
`), 0o644); err != nil {
		t.Fatalf("Failed to write user skill: %v", err)
	}

	// Create skill manager with custom directories
	dirs := []skill.DirConfig{
		{Path: "./skills", SourceType: entity.SkillSourceProject},
		{Path: "./.claude/skills", SourceType: entity.SkillSourceProjectClaude},
		{Path: filepath.Join(userHomeDir, ".claude", "skills"), SourceType: entity.SkillSourceUser},
	}
	skillManager := skill.NewLocalSkillManagerWithDirs(dirs)

	// Create file manager
	fileManager := file.NewLocalFileManager(".")

	// Create tool executor
	toolExecutor := NewExecutorAdapter(fileManager)
	toolExecutor.SetSkillManager(skillManager)

	// Get activate_skill tool
	activateSkillTool, found := toolExecutor.GetTool("activate_skill")
	if !found {
		t.Fatal("activate_skill tool not found")
	}

	description := activateSkillTool.Description
	t.Logf("Tool description:\n%s", description)

	// Verify each source type is shown in the description
	expectedPatterns := []string{
		"**project-skill** (project):",
		"**claude-skill** (project-claude):",
		"**user-skill** (user):",
		"Skill source types indicate where scripts are located:",
		"(project): ./skills/skill-name/",
		"(project-claude): ./.claude/skills/skill-name/",
		"(user): ~/.claude/skills/skill-name/",
	}

	for _, pattern := range expectedPatterns {
		if !strings.Contains(description, pattern) {
			t.Errorf("Expected description to contain '%s'", pattern)
		}
	}
}

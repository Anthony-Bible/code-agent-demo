package tool

import (
	"code-editing-agent/internal/infrastructure/adapter/file"
	"code-editing-agent/internal/infrastructure/adapter/skill"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSkillsInToolDescription verifies that skills are included in the activate_skill tool description.
func TestSkillsInToolDescription(t *testing.T) {
	// Get project root with proper error handling
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot, err := filepath.Abs(filepath.Join(originalWd, "../../../.."))
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}
	t.Chdir(projectRoot)

	// Create skill manager
	skillManager := skill.NewLocalSkillManager()

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

	// Check if skills are in tool description
	description := activateSkillTool.Description
	t.Logf("activate_skill description:\n%s", description)

	// Verify the Available Skills section exists
	if !strings.Contains(description, "Available Skills") {
		t.Error("Expected 'Available Skills' section in tool description")
	}

	// Verify that skills were loaded (check for skill-like patterns)
	// Skills are typically listed as "- name: description" or with bullet points
	skillsLoaded := false

	// Check for common patterns that indicate skills are listed:
	// 1. Skill list marker (lines starting with "- " or numbers)
	// 2. Skill metadata like "description:" or "license:"
	// 3. Multiple consecutive lines with skill-like content
	lines := strings.Split(description, "\n")
	listItemCount := 0
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") {
			listItemCount++
			// If we have a list item that's not just "- Available Skills", it's probably a skill
			if trimmed != "- Available Skills" && trimmed != "-" {
				skillsLoaded = true
			}
		}
	}

	// We should have some list items indicating skills were loaded
	if listItemCount < 2 {
		t.Errorf("Expected multiple list items in skill description, got %d", listItemCount)
	}

	if !skillsLoaded {
		t.Error("Expected to find skill entries in the Available Skills section")
	}

	// Verify skills format - should contain markdown bold markers for skill names
	// Skills are formatted as "- **name** (source): description"
	if !strings.Contains(description, "**") {
		t.Error("Expected skill description to contain markdown bold markers for skill names")
	}

	// Verify that source type labels are present
	if !strings.Contains(description, "(project)") && !strings.Contains(description, "(user)") &&
		!strings.Contains(description, "(project-claude)") {
		t.Error("Expected skill description to contain source type labels like (project), (user), or (project-claude)")
	}

	// Verify the source type explanation section is present
	if !strings.Contains(description, "Skill source types indicate where scripts are located:") {
		t.Error("Expected skill description to contain source type explanation section")
	}
}

// TestActivateSkillIncludesSourceType verifies that activating a skill returns source_type and directory_path.
func TestActivateSkillIncludesSourceType(t *testing.T) {
	// Get project root with proper error handling
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}
	projectRoot, err := filepath.Abs(filepath.Join(originalWd, "../../../.."))
	if err != nil {
		t.Fatalf("Failed to get project root: %v", err)
	}
	t.Chdir(projectRoot)

	// Create skill manager
	skillManager := skill.NewLocalSkillManager()

	// Create file manager
	fileManager := file.NewLocalFileManager(".")

	// Create tool executor
	toolExecutor := NewExecutorAdapter(fileManager)
	toolExecutor.SetSkillManager(skillManager)

	// Activate test-skill
	input := `{"skill_name": "test-skill"}`
	result, err := toolExecutor.ExecuteTool(context.Background(), "activate_skill", []byte(input))
	if err != nil {
		t.Fatalf("Failed to activate skill: %v", err)
	}

	t.Logf("Activated skill result:\n%s", result)

	// Verify the result contains source_type
	if !strings.Contains(result, "source_type:") {
		t.Error("Expected activated skill to contain 'source_type:' field")
	}

	// Verify the result contains directory_path
	if !strings.Contains(result, "directory_path:") {
		t.Error("Expected activated skill to contain 'directory_path:' field")
	}

	// Verify source_type value is one of the expected types
	hasValidSourceType := strings.Contains(result, "source_type: user") ||
		strings.Contains(result, "source_type: project") ||
		strings.Contains(result, "source_type: project-claude")
	if !hasValidSourceType {
		t.Error("Expected source_type to be one of: user, project, or project-claude")
	}

	// Verify directory_path contains a path
	lines := strings.Split(result, "\n")
	foundDirPath := false
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "directory_path:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[1]) != "" {
				foundDirPath = true
				t.Logf("Found directory_path: %s", strings.TrimSpace(parts[1]))
				break
			}
		}
	}
	if !foundDirPath {
		t.Error("Expected directory_path to have a non-empty value")
	}
}

package tool

import (
	"code-editing-agent/internal/infrastructure/adapter/file"
	"code-editing-agent/internal/infrastructure/adapter/skill"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSkillsInToolDescription verifies that skills are included in the activate_skill tool description.
func TestSkillsInToolDescription(t *testing.T) {
	// Change to project root for test to find skills directory
	originalWd, _ := os.Getwd()
	projectRoot := filepath.Join(originalWd, "../../../..")
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
	// Skills are formatted as "- **name**: description"
	if !strings.Contains(description, "**") {
		t.Error("Expected skill description to contain markdown bold markers for skill names")
	}

	// Verify that we have colons separating names from descriptions
	if !strings.Contains(description, "**:") {
		t.Error("Expected skill description to contain '**:' pattern separating skill names from descriptions")
	}
}

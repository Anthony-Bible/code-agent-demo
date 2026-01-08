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
// Change to project root for test
originalWd, _ := os.Getwd()
projectRoot := filepath.Join(originalWd, "../../../..")
if err := os.Chdir(projectRoot); err != nil {
t.Fatalf("Failed to change to project root: %v", err)
}
defer os.Chdir(originalWd)

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

if !strings.Contains(description, "Available Skills") {
t.Error("Expected 'Available Skills' section in tool description")
}

// Check for at least one skill (test-skill should exist)
if !strings.Contains(description, "test-skill") && !strings.Contains(description, "code-review") {
t.Error("Expected at least one skill name in tool description")
}
}

// TestSkillsNotInSystemPrompt is a documentation test showing the integration.
// The actual system prompt test is in the AI adapter package.
func TestSkillsNotInSystemPrompt(t *testing.T) {
// This test documents that skills are no longer in the system prompt
// The actual verification is done in the AI adapter tests
t.Log("Skills are now in the activate_skill tool description, not in the system prompt")
t.Log("This follows the architecture change to move skill discovery to tool definitions")
}

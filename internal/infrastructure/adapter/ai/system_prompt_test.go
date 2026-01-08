package ai

import (
"code-editing-agent/internal/infrastructure/adapter/skill"
"os"
"path/filepath"
"strings"
"testing"
)

// TestSystemPromptDoesNotContainSkills verifies that the system prompt does not contain skills.
// Skills should now be in the activate_skill tool description instead.
func TestSystemPromptDoesNotContainSkills(t *testing.T) {
// Change to project root for test
originalWd, _ := os.Getwd()
projectRoot := filepath.Join(originalWd, "../../../..")
if err := os.Chdir(projectRoot); err != nil {
t.Fatalf("Failed to change to project root: %v", err)
}
defer os.Chdir(originalWd)

// Create skill manager
skillManager := skill.NewLocalSkillManager()

// Create AI adapter with skill manager
adapter := NewAnthropicAdapter("test-model", 4096, skillManager, nil)

// Get system prompt
systemPrompt := adapter.(*AnthropicAdapter).buildBasePromptWithSkills()

t.Logf("System prompt:\n%s", systemPrompt)

// Verify skills are NOT in system prompt
if strings.Contains(systemPrompt, "<available_skills>") {
t.Error("System prompt should NOT contain <available_skills> XML block")
}

if strings.Contains(systemPrompt, "test-skill") {
t.Error("System prompt should NOT contain skill names like 'test-skill'")
}

if strings.Contains(systemPrompt, "code-review") {
t.Error("System prompt should NOT contain skill names like 'code-review'")
}

// Verify base prompt is still present
if !strings.Contains(systemPrompt, "AI assistant") {
t.Error("System prompt should still contain base prompt text")
}
}

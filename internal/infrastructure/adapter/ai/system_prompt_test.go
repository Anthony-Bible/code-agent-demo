package ai

import (
	"strings"
	"testing"
)

// TestSystemPromptDoesNotContainSkills verifies that the system prompt does not contain skills.
// Skills should now be in the activate_skill tool description instead.
func TestSystemPromptDoesNotContainSkills(t *testing.T) {
	// Create AI adapter without skill manager
	// The buildBasePromptWithSkills method doesn't actually use the skill manager,
	// as skills are now loaded in the activate_skill tool description
	adapter := &AnthropicAdapter{
		model:        "test-model",
		skillManager: nil,
	}

	// Get system prompt
	systemPrompt := adapter.buildBasePromptWithSkills()

	t.Logf("System prompt:\n%s", systemPrompt)

	// Verify skills are NOT in system prompt by checking for patterns
	// that would indicate skill information leaked in

	// 1. Should not contain XML blocks for skills
	if strings.Contains(systemPrompt, "<available_skills>") {
		t.Error("System prompt should NOT contain <available_skills> XML block")
	}

	// 2. Should not contain common skill-related patterns
	skillsPatterns := []string{
		"Available Skills",
		"Skill: ",
		"Description: ",
	}
	for _, pattern := range skillsPatterns {
		if strings.Contains(systemPrompt, pattern) {
			t.Errorf("System prompt should NOT contain skill-related pattern '%s'", pattern)
		}
	}

	// 3. Should not contain typical skill names that might exist
	// These are common skill names from typical projects
	commonSkillNames := []string{
		"test-skill",
		"code-review",
		"commit",
		"documentation",
		"test-writer",
		"code-reviewer",
	}
	for _, skillName := range commonSkillNames {
		if strings.Contains(systemPrompt, skillName) {
			t.Errorf("System prompt should NOT contain skill name '%s'", skillName)
		}
	}

	// Verify base prompt is still present
	if !strings.Contains(systemPrompt, "AI assistant") {
		t.Error("System prompt should still contain base prompt text")
	}
}

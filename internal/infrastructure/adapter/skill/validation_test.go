package skill

import (
	"code-editing-agent/internal/domain/entity"
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestDiscoverySkipsInvalidSkills verifies that skills with mismatched names are skipped.
func TestDiscoverySkipsInvalidSkills(t *testing.T) {
	// Create a temporary skills directory
	tempDir := t.TempDir()
	skillsDir := filepath.Join(tempDir, "skills")

	// Create valid skill
	validSkillDir := filepath.Join(skillsDir, "valid-skill")
	err := os.MkdirAll(validSkillDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create valid skill directory: %v", err)
	}

	validSkillContent := `---
name: valid-skill
description: A valid skill
---
Content here
`
	err = os.WriteFile(filepath.Join(validSkillDir, "SKILL.md"), []byte(validSkillContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write valid skill file: %v", err)
	}

	// Create invalid skill (directory name doesn't match skill name)
	invalidSkillDir := filepath.Join(skillsDir, "bad-directory")
	err = os.MkdirAll(invalidSkillDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create invalid skill directory: %v", err)
	}

	invalidSkillContent := `---
name: wrong-name
description: This should fail validation
---
Content here
`
	err = os.WriteFile(filepath.Join(invalidSkillDir, "SKILL.md"), []byte(invalidSkillContent), 0o644)
	if err != nil {
		t.Fatalf("Failed to write invalid skill file: %v", err)
	}

	// Create skill manager with temp directory
	sm := &LocalSkillManager{
		skillsDir: skillsDir,
		skills:    make(map[string]*entity.Skill),
		active:    make(map[string]bool),
	}

	// Discover skills
	ctx := context.Background()
	result, err := sm.DiscoverSkills(ctx)
	if err != nil {
		t.Fatalf("DiscoverSkills failed: %v", err)
	}

	// Should only find 1 valid skill
	if result.TotalCount != 1 {
		t.Errorf("Expected 1 skill, got %d", result.TotalCount)
	}

	// Should find the valid skill
	if len(result.Skills) != 1 || result.Skills[0].Name != "valid-skill" {
		t.Errorf("Expected to find 'valid-skill', got: %+v", result.Skills)
	}

	// Should NOT find the invalid skill
	for _, s := range result.Skills {
		if s.Name == "wrong-name" {
			t.Error("Invalid skill 'wrong-name' should have been skipped but was found")
		}
	}

	t.Log("✅ Validation correctly skipped skill with mismatched directory name")
}

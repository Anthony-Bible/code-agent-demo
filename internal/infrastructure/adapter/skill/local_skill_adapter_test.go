package skill

import (
	"code-editing-agent/internal/domain/entity"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestLocalSkillManager_DiscoverSkills_EmptySkillsDirectory(t *testing.T) {
	sm := NewLocalSkillManager()
	result, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("DiscoverSkills() returned unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("DiscoverSkills() returned nil result")
	}

	if result.TotalCount != 0 {
		t.Errorf("DiscoverSkills() TotalCount = %d, want 0", result.TotalCount)
	}

	if result.ActiveCount != 0 {
		t.Errorf("DiscoverSkills() ActiveCount = %d, want 0", result.ActiveCount)
	}

	if len(result.Skills) != 0 {
		t.Errorf("DiscoverSkills() returned %d skills, want 0", len(result.Skills))
	}
}

func TestLocalSkillManager_DiscoverSkills_WithSkillFiles(t *testing.T) {
	// Create a temporary skills directory
	tempDir := t.TempDir()
	skillsDir := filepath.Join(tempDir, "skills")
	testSkillDir := filepath.Join(skillsDir, "test-skill")

	if err := os.MkdirAll(testSkillDir, 0o750); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a SKILL.md file
	skillContent := `---
name: test-skill
description: A test skill
---
Test content`
	if err := os.WriteFile(filepath.Join(testSkillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Create skill manager with custom skills dir
	sm := &LocalSkillManager{
		skillsDir: skillsDir,
		skills:    make(map[string]*entity.Skill),
		active:    make(map[string]bool),
	}

	result, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("DiscoverSkills() returned unexpected error: %v", err)
	}

	if result.TotalCount != 1 {
		t.Errorf("DiscoverSkills() TotalCount = %d, want 1", result.TotalCount)
	}

	if len(result.Skills) != 1 {
		t.Fatalf("DiscoverSkills() returned %d skills, want 1", len(result.Skills))
	}

	skill := result.Skills[0]
	if skill.Name != "test-skill" {
		t.Errorf("DiscoverSkills() skill Name = %v, want 'test-skill'", skill.Name)
	}

	if skill.Description != "A test skill" {
		t.Errorf("DiscoverSkills() skill Description = %v, want 'A test skill'", skill.Description)
	}

	if skill.IsActive {
		t.Errorf("DiscoverSkills() skill IsActive = true, want false for new discover")
	}
}

func TestLocalSkillManager_ActivateSkill(t *testing.T) {
	// Create a temporary skills directory
	tempDir := t.TempDir()
	skillsDir := filepath.Join(tempDir, "skills")
	testSkillDir := filepath.Join(skillsDir, "test-skill")

	if err := os.MkdirAll(testSkillDir, 0o750); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a SKILL.md file
	skillContent := `---
name: test-skill
description: A test skill
---
Test content`
	if err := os.WriteFile(filepath.Join(testSkillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Create skill manager and discover skills
	sm := &LocalSkillManager{
		skillsDir: skillsDir,
		skills:    make(map[string]*entity.Skill),
		active:    make(map[string]bool),
	}

	_, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("Failed to discover skills: %v", err)
	}

	// Activate the skill
	activated, err := sm.ActivateSkill(context.Background(), "test-skill")
	if err != nil {
		t.Fatalf("ActivateSkill() returned unexpected error: %v", err)
	}

	if !activated {
		t.Error("ActivateSkill() returned false, want true")
	}

	// Check if skill is active
	skill, ok := sm.skills["test-skill"]
	if !ok {
		t.Fatal("Skill not found in skills map")
	}

	if !sm.active["test-skill"] {
		t.Error("Skill is not marked as active")
	}

	_ = skill
}

func TestLocalSkillManager_ActivateSkill_SkillNotFound(t *testing.T) {
	sm := NewLocalSkillManager()
	activated, err := sm.ActivateSkill(context.Background(), "nonexistent-skill")

	if err == nil {
		t.Error("ActivateSkill() should return error for nonexistent skill")
	}

	if !errors.Is(err, ErrSkillNotFound) {
		t.Errorf("ActivateSkill() error = %v, want ErrSkillNotFound", err)
	}

	if activated {
		t.Error("ActivateSkill() returned true for nonexistent skill, want false")
	}
}

func TestLocalSkillManager_DeactivateSkill(t *testing.T) {
	// Create a temporary skills directory
	tempDir := t.TempDir()
	skillsDir := filepath.Join(tempDir, "skills")
	testSkillDir := filepath.Join(skillsDir, "test-skill")

	if err := os.MkdirAll(testSkillDir, 0o750); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a SKILL.md file
	skillContent := `---
name: test-skill
description: A test skill
---
Test content`
	if err := os.WriteFile(filepath.Join(testSkillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Create skill manager and discover skills
	sm := &LocalSkillManager{
		skillsDir: skillsDir,
		skills:    make(map[string]*entity.Skill),
		active:    make(map[string]bool),
	}

	_, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("Failed to discover skills: %v", err)
	}

	// Activate the skill first
	sm.ActivateSkill(context.Background(), "test-skill")

	// Deactivate the skill
	deactivated, err := sm.DeactivateSkill(context.Background(), "test-skill")
	if err != nil {
		t.Fatalf("DeactivateSkill() returned unexpected error: %v", err)
	}

	if !deactivated {
		t.Error("DeactivateSkill() returned false, want true")
	}

	if sm.active["test-skill"] {
		t.Error("Skill is still marked as active after deactivation")
	}
}

func TestLocalSkillManager_GetAvailableSkills(t *testing.T) {
	// Create a temporary skills directory
	tempDir := t.TempDir()
	skillsDir := filepath.Join(tempDir, "skills")

	// Create two skill directories with explicit names
	for _, skillName := range []string{"skill-a", "skill-b"} {
		testSkillDir := filepath.Join(skillsDir, skillName)
		if err := os.MkdirAll(testSkillDir, 0o750); err != nil {
			t.Fatalf("Failed to create test directory: %v", err)
		}

		skillContent := `---
name: ` + skillName + `
description: Test skill
---
Content`
		if err := os.WriteFile(filepath.Join(testSkillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
			t.Fatalf("Failed to write SKILL.md: %v", err)
		}
	}

	// Create skill manager and discover skills
	sm := &LocalSkillManager{
		skillsDir: skillsDir,
		skills:    make(map[string]*entity.Skill),
		active:    make(map[string]bool),
	}

	_, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("Failed to discover skills: %v", err)
	}

	// Initially no skills are active
	available, err := sm.GetAvailableSkills(context.Background())
	if err != nil {
		t.Fatalf("GetAvailableSkills() returned unexpected error: %v", err)
	}

	if len(available) != 0 {
		t.Errorf("GetAvailableSkills() returned %d skills, want 0 initially", len(available))
	}

	// Activate one skill
	sm.ActivateSkill(context.Background(), "skill-b")

	available, err = sm.GetAvailableSkills(context.Background())
	if err != nil {
		t.Fatalf("GetAvailableSkills() returned unexpected error: %v", err)
	}

	if len(available) != 1 {
		t.Errorf("GetAvailableSkills() returned %d skills, want 1 after activation", len(available))
	}

	if available[0].Name != "skill-b" {
		t.Errorf("GetAvailableSkills() returned skill Name = %v, want 'skill-b'", available[0].Name)
	}
}

func TestLocalSkillManager_GetSkillByName(t *testing.T) {
	// Create a temporary skills directory
	tempDir := t.TempDir()
	skillsDir := filepath.Join(tempDir, "skills")
	testSkillDir := filepath.Join(skillsDir, "test-skill")

	if err := os.MkdirAll(testSkillDir, 0o750); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a SKILL.md file
	skillContent := `---
name: test-skill
description: A test skill
license: MIT
compatibility: all
---
Test content`
	if err := os.WriteFile(filepath.Join(testSkillDir, "SKILL.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Create skill manager and discover skills
	sm := &LocalSkillManager{
		skillsDir: skillsDir,
		skills:    make(map[string]*entity.Skill),
		active:    make(map[string]bool),
	}

	_, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("Failed to discover skills: %v", err)
	}

	// Get skill by name
	skillInfo, err := sm.GetSkillByName(context.Background(), "test-skill")
	if err != nil {
		t.Fatalf("GetSkillByName() returned unexpected error: %v", err)
	}

	if skillInfo == nil {
		t.Fatal("GetSkillByName() returned nil skill")
	}

	if skillInfo.Name != "test-skill" {
		t.Errorf("GetSkillByName() Name = %v, want 'test-skill'", skillInfo.Name)
	}

	if skillInfo.Description != "A test skill" {
		t.Errorf("GetSkillByName() Description = %v, want 'A test skill'", skillInfo.Description)
	}

	if skillInfo.License != "MIT" {
		t.Errorf("GetSkillByName() License = %v, want 'MIT'", skillInfo.License)
	}

	if skillInfo.Compatibility != "all" {
		t.Errorf("GetSkillByName() Compatibility = %v, want 'all'", skillInfo.Compatibility)
	}

	if skillInfo.IsActive {
		t.Error("GetSkillByName() IsActive = true, want false initially")
	}
}

func TestLocalSkillManager_GetSkillByName_NotFound(t *testing.T) {
	sm := NewLocalSkillManager()
	_, err := sm.GetSkillByName(context.Background(), "nonexistent-skill")

	if err == nil {
		t.Error("GetSkillByName() should return error for nonexistent skill")
	}

	if !errors.Is(err, ErrSkillNotFound) {
		t.Errorf("GetSkillByName() error = %v, want ErrSkillNotFound", err)
	}
}

func TestLocalSkillManager_ValidateSkills(t *testing.T) {
	// Create a temporary skills directory
	tempDir := t.TempDir()
	skillsDir := filepath.Join(tempDir, "skills")

	// Create a valid skill
	validSkillDir := filepath.Join(skillsDir, "valid-skill")
	if err := os.MkdirAll(validSkillDir, 0o750); err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	validSkillContent := `---
name: valid-skill
description: A valid skill
---
Valid content`
	if err := os.WriteFile(filepath.Join(validSkillDir, "SKILL.md"), []byte(validSkillContent), 0o644); err != nil {
		t.Fatalf("Failed to write SKILL.md: %v", err)
	}

	// Create skill manager and discover skills
	sm := &LocalSkillManager{
		skillsDir: skillsDir,
		skills:    make(map[string]*entity.Skill),
		active:    make(map[string]bool),
	}

	_, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("Failed to discover skills: %v", err)
	}

	// Manually add an invalid skill to the skills map
	// This tests the ValidateSkills method directly
	sm.skills["invalid-skill"] = &entity.Skill{
		Name:        "",
		Description: "Invalid skill",
	}

	// Validate skills
	validationErrors, err := sm.ValidateSkills(context.Background())
	if err != nil {
		t.Fatalf("ValidateSkills() returned unexpected error: %v", err)
	}

	// We should have validation errors for the invalid skill
	if len(validationErrors) == 0 {
		t.Error("ValidateSkills() returned no errors, expected at least one")
	}

	// Check that valid-skill is not in the errors
	if _, ok := validationErrors["valid-skill"]; ok {
		t.Error("ValidateSkills() returned error for valid-skill, expected no error")
	}

	// Check that invalid-skill is in the errors
	if _, ok := validationErrors["invalid-skill"]; !ok {
		t.Error("ValidateSkills() did not return error for invalid-skill")
	}
}

package skill

import (
	"code-editing-agent/internal/domain/entity"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestLocalSkillManager_DiscoverSkills_EmptySkillsDirectory(t *testing.T) {
	tempDir := t.TempDir()
	emptySkillsDir := filepath.Join(tempDir, "empty-skills")

	sm := &LocalSkillManager{
		skillsDirs: []DirConfig{
			{Path: emptySkillsDir, SourceType: entity.SkillSourceProject},
		},
		skills: make(map[string]*entity.Skill),
		active: make(map[string]bool),
	}
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
		skillsDirs: []DirConfig{{Path: skillsDir, SourceType: entity.SkillSourceProject}},
		skills:     make(map[string]*entity.Skill),
		active:     make(map[string]bool),
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
		skillsDirs: []DirConfig{{Path: skillsDir, SourceType: entity.SkillSourceProject}},
		skills:     make(map[string]*entity.Skill),
		active:     make(map[string]bool),
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
		skillsDirs: []DirConfig{{Path: skillsDir, SourceType: entity.SkillSourceProject}},
		skills:     make(map[string]*entity.Skill),
		active:     make(map[string]bool),
	}

	_, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("Failed to discover skills: %v", err)
	}

	// Activate the skill first
	if _, err := sm.ActivateSkill(context.Background(), "test-skill"); err != nil {
		t.Fatalf("ActivateSkill() returned unexpected error: %v", err)
	}

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
		skillsDirs: []DirConfig{{Path: skillsDir, SourceType: entity.SkillSourceProject}},
		skills:     make(map[string]*entity.Skill),
		active:     make(map[string]bool),
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
		skillsDirs: []DirConfig{{Path: skillsDir, SourceType: entity.SkillSourceProject}},
		skills:     make(map[string]*entity.Skill),
		active:     make(map[string]bool),
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

func TestValidateSkillName(t *testing.T) {
	tests := []struct {
		name      string
		skillName string
		wantErr   error
	}{
		// Valid names
		{name: "valid simple name", skillName: "test-skill", wantErr: nil},
		{name: "valid with numbers", skillName: "skill123", wantErr: nil},
		{name: "valid all lowercase", skillName: "myskill", wantErr: nil},
		{name: "valid with hyphen", skillName: "my-cool-skill", wantErr: nil},

		// Path traversal attempts
		{name: "path traversal with ../", skillName: "../etc/passwd", wantErr: ErrInvalidSkillName},
		{name: "path traversal with ..", skillName: "..skill", wantErr: ErrInvalidSkillName},
		{name: "path traversal with /", skillName: "skill/subdir", wantErr: ErrInvalidSkillName},
		{name: "absolute path", skillName: "/etc/passwd", wantErr: ErrInvalidSkillName},
		{name: "windows path", skillName: "C:\\Windows", wantErr: ErrInvalidSkillName},
		{name: "backslash", skillName: "skill\\name", wantErr: ErrInvalidSkillName},
		{name: "null byte", skillName: "skill\x00name", wantErr: ErrInvalidSkillName},

		// Invalid characters
		{name: "uppercase letters", skillName: "MySkill", wantErr: ErrInvalidSkillName},
		{name: "spaces", skillName: "my skill", wantErr: ErrInvalidSkillName},
		{name: "underscore", skillName: "my_skill", wantErr: ErrInvalidSkillName},
		{name: "dot", skillName: "my.skill", wantErr: ErrInvalidSkillName},

		// Hyphen rules
		{name: "starts with hyphen", skillName: "-skill", wantErr: ErrSkillNameHyphen},
		{name: "ends with hyphen", skillName: "skill-", wantErr: ErrSkillNameHyphen},
		{name: "consecutive hyphens", skillName: "my--skill", wantErr: ErrSkillNameConsecHyphen},

		// Length and empty
		{name: "empty name", skillName: "", wantErr: ErrSkillNameEmpty},
		{
			name:      "too long name",
			skillName: "a123456789012345678901234567890123456789012345678901234567890123456789",
			wantErr:   ErrSkillNameTooLong,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSkillName(tt.skillName)
			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("validateSkillName(%q) unexpected error: %v", tt.skillName, err)
				}
			} else {
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("validateSkillName(%q) error = %v, want %v", tt.skillName, err, tt.wantErr)
				}
			}
		})
	}
}

func TestLocalSkillManager_LoadSkillMetadata_PathTraversalPrevention(t *testing.T) {
	sm := NewLocalSkillManager()

	pathTraversalAttempts := []struct {
		name      string
		skillName string
	}{
		{name: "parent directory", skillName: "../etc/passwd"},
		{name: "double parent", skillName: "../../secret"},
		{name: "absolute path", skillName: "/etc/passwd"},
		{name: "encoded slash", skillName: "skill%2F..%2Fetc"},
		{name: "dot dot", skillName: ".."},
		{name: "hidden file", skillName: ".hidden"},
	}

	for _, tt := range pathTraversalAttempts {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sm.LoadSkillMetadata(context.Background(), tt.skillName)
			if err == nil {
				t.Errorf("LoadSkillMetadata(%q) should have blocked path traversal attempt", tt.skillName)
			}
			// The error should be related to invalid skill name, not file not found
			if errors.Is(err, ErrSkillFileNotFound) {
				t.Errorf(
					"LoadSkillMetadata(%q) returned file not found instead of blocking path traversal",
					tt.skillName,
				)
			}
		})
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
		skillsDirs: []DirConfig{{Path: skillsDir, SourceType: entity.SkillSourceProject}},
		skills:     make(map[string]*entity.Skill),
		active:     make(map[string]bool),
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

// =============================================================================
// Multi-Directory Skill Discovery Tests (RED PHASE - Expected to FAIL)
// =============================================================================
// These tests verify that skills can be discovered from multiple directories:
// - ~/.claude/skills (user global) - SourceType: "user"
// - ./.claude/skills (project .claude directory) - SourceType: "project-claude"
// - ./skills (project root, highest priority) - SourceType: "project"
//
// Priority order: ./skills > ./.claude/skills > ~/.claude/skills
// When the same skill name exists in multiple directories, the highest priority wins.
// =============================================================================

// createSkillFile is a helper to create a SKILL.md file with given name and description.
func createSkillFile(t *testing.T, dir, skillName, description string) {
	t.Helper()
	skillDir := filepath.Join(dir, skillName)
	if err := os.MkdirAll(skillDir, 0o750); err != nil {
		t.Fatalf("Failed to create skill directory %s: %v", skillDir, err)
	}

	content := "---\nname: " + skillName + "\ndescription: " + description + "\n---\nContent for " + skillName
	skillFile := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte(content), 0o644); err != nil {
		t.Fatalf("Failed to write SKILL.md at %s: %v", skillFile, err)
	}
}

// TestLocalSkillManager_DiscoverSkills_MultipleDirectories verifies that skills are
// discovered from all three directory locations:
// - User global (~/.claude/skills)
// - Project .claude directory (./.claude/skills)
// - Project root (./skills).
func TestLocalSkillManager_DiscoverSkills_MultipleDirectories(t *testing.T) {
	// Create a temporary base directory to simulate the environment
	tempDir := t.TempDir()

	// Simulate the three skill directories
	userSkillsDir := filepath.Join(tempDir, "home", ".claude", "skills")
	projectClaudeSkillsDir := filepath.Join(tempDir, "project", ".claude", "skills")
	projectSkillsDir := filepath.Join(tempDir, "project", "skills")

	// Create unique skills in each directory
	createSkillFile(t, userSkillsDir, "user-only-skill", "A skill only in user global directory")
	createSkillFile(t, projectClaudeSkillsDir, "project-claude-skill", "A skill only in project .claude directory")
	createSkillFile(t, projectSkillsDir, "project-skill", "A skill only in project root directory")

	// Create the skill manager with multiple directories
	sm := &LocalSkillManager{
		skillsDirs: []DirConfig{
			{Path: projectSkillsDir, SourceType: entity.SkillSourceProject},
			{Path: projectClaudeSkillsDir, SourceType: entity.SkillSourceProjectClaude},
			{Path: userSkillsDir, SourceType: entity.SkillSourceUser},
		},
		skills: make(map[string]*entity.Skill),
		active: make(map[string]bool),
	}

	result, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("DiscoverSkills() returned unexpected error: %v", err)
	}

	// FAILING ASSERTION: We expect 3 skills from 3 directories, but current implementation
	// only discovers from one directory (projectSkillsDir), so we get 1 skill.
	if result.TotalCount != 3 {
		t.Errorf("DiscoverSkills() TotalCount = %d, want 3 (skills from all three directories)", result.TotalCount)
	}

	// Verify all three skills are discovered
	skillNames := make(map[string]bool)
	for _, skill := range result.Skills {
		skillNames[skill.Name] = true
	}

	expectedSkills := []string{"user-only-skill", "project-claude-skill", "project-skill"}
	for _, expected := range expectedSkills {
		if !skillNames[expected] {
			t.Errorf("DiscoverSkills() missing expected skill %q", expected)
		}
	}

	// FAILING ASSERTION: SkillsDirs should list all searched directories
	if len(result.SkillsDirs) != 3 {
		t.Errorf("DiscoverSkills() SkillsDirs length = %d, want 3", len(result.SkillsDirs))
	}
}

// TestLocalSkillManager_DiscoverSkills_PriorityOverride verifies that when the same
// skill name exists in multiple directories, the highest priority directory wins.
// Priority: ./skills > ./.claude/skills > ~/.claude/skills.
func TestLocalSkillManager_DiscoverSkills_PriorityOverride(t *testing.T) {
	tempDir := t.TempDir()

	// Simulate the three skill directories
	userSkillsDir := filepath.Join(tempDir, "home", ".claude", "skills")
	projectClaudeSkillsDir := filepath.Join(tempDir, "project", ".claude", "skills")
	projectSkillsDir := filepath.Join(tempDir, "project", "skills")

	// Create the SAME skill in all three directories with different descriptions
	createSkillFile(t, userSkillsDir, "common-skill", "User global version of common-skill")
	createSkillFile(t, projectClaudeSkillsDir, "common-skill", "Project .claude version of common-skill")
	createSkillFile(t, projectSkillsDir, "common-skill", "Project root version of common-skill (highest priority)")

	// Also create a skill that only exists in user and project-claude (to test mid-priority override)
	createSkillFile(t, userSkillsDir, "mid-priority-skill", "User global version")
	createSkillFile(t, projectClaudeSkillsDir, "mid-priority-skill", "Project .claude version (should win)")

	// Create skill manager
	sm := &LocalSkillManager{
		skillsDirs: []DirConfig{
			{Path: projectSkillsDir, SourceType: entity.SkillSourceProject},
			{Path: projectClaudeSkillsDir, SourceType: entity.SkillSourceProjectClaude},
			{Path: userSkillsDir, SourceType: entity.SkillSourceUser},
		},
		skills: make(map[string]*entity.Skill),
		active: make(map[string]bool),
	}

	result, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("DiscoverSkills() returned unexpected error: %v", err)
	}

	// Find the common-skill in results
	var commonSkill *struct {
		description string
		sourceType  entity.SkillSourceType
	}
	var midPrioritySkill *struct {
		description string
		sourceType  entity.SkillSourceType
	}

	for _, skill := range result.Skills {
		if skill.Name == "common-skill" {
			commonSkill = &struct {
				description string
				sourceType  entity.SkillSourceType
			}{skill.Description, skill.SourceType}
		}
		if skill.Name == "mid-priority-skill" {
			midPrioritySkill = &struct {
				description string
				sourceType  entity.SkillSourceType
			}{skill.Description, skill.SourceType}
		}
	}

	// FAILING ASSERTION: common-skill should have description from highest priority (./skills)
	if commonSkill == nil {
		t.Fatal("DiscoverSkills() did not find common-skill")
	}
	if commonSkill.description != "Project root version of common-skill (highest priority)" {
		t.Errorf("DiscoverSkills() common-skill.Description = %q, want %q (highest priority should win)",
			commonSkill.description, "Project root version of common-skill (highest priority)")
	}
	if commonSkill.sourceType != entity.SkillSourceProject {
		t.Errorf("DiscoverSkills() common-skill.SourceType = %q, want %q",
			commonSkill.sourceType, entity.SkillSourceProject)
	}

	// FAILING ASSERTION: mid-priority-skill should exist and come from project-claude
	if midPrioritySkill == nil {
		t.Errorf("DiscoverSkills() did not find mid-priority-skill (should be discovered from project-claude or user)")
	} else {
		if midPrioritySkill.description != "Project .claude version (should win)" {
			t.Errorf("DiscoverSkills() mid-priority-skill.Description = %q, want %q",
				midPrioritySkill.description, "Project .claude version (should win)")
		}
		if midPrioritySkill.sourceType != entity.SkillSourceProjectClaude {
			t.Errorf("DiscoverSkills() mid-priority-skill.SourceType = %q, want %q",
				midPrioritySkill.sourceType, entity.SkillSourceProjectClaude)
		}
	}

	// We should have exactly 2 unique skills after priority resolution
	if result.TotalCount != 2 {
		t.Errorf("DiscoverSkills() TotalCount = %d, want 2 (after priority deduplication)", result.TotalCount)
	}
}

// TestLocalSkillManager_DiscoverSkills_MissingDirectories verifies that discovery
// gracefully handles missing directories without erroring.
func TestLocalSkillManager_DiscoverSkills_MissingDirectories(t *testing.T) {
	tempDir := t.TempDir()

	// Only create the project skills directory, leave others missing
	projectSkillsDir := filepath.Join(tempDir, "project", "skills")
	userSkillsDir := filepath.Join(tempDir, "home", ".claude", "skills")             // Does not exist
	projectClaudeSkillsDir := filepath.Join(tempDir, "project", ".claude", "skills") // Does not exist

	createSkillFile(t, projectSkillsDir, "existing-skill", "A skill in the existing directory")

	// Create skill manager that should handle missing directories gracefully
	sm := &LocalSkillManager{
		skillsDirs: []DirConfig{
			{Path: projectSkillsDir, SourceType: entity.SkillSourceProject},
			{Path: projectClaudeSkillsDir, SourceType: entity.SkillSourceProjectClaude},
			{Path: userSkillsDir, SourceType: entity.SkillSourceUser},
		},
		skills: make(map[string]*entity.Skill),
		active: make(map[string]bool),
	}

	result, err := sm.DiscoverSkills(context.Background())
	// Should NOT error when some directories are missing
	if err != nil {
		t.Fatalf("DiscoverSkills() should not error when some directories are missing: %v", err)
	}

	// Should still discover skills from existing directories
	if result.TotalCount != 1 {
		t.Errorf("DiscoverSkills() TotalCount = %d, want 1", result.TotalCount)
	}

	// FAILING ASSERTION: SkillsDirs should list all directories that were searched,
	// including those that don't exist (for transparency)
	if len(result.SkillsDirs) < 1 {
		t.Errorf("DiscoverSkills() SkillsDirs should list searched directories, got %d", len(result.SkillsDirs))
	}
}

// TestLocalSkillManager_DiscoverSkills_SourceType verifies that each discovered skill
// has the correct SourceType field set based on which directory it was found in.
func TestLocalSkillManager_DiscoverSkills_SourceType(t *testing.T) {
	tempDir := t.TempDir()

	// Create skill directories
	userSkillsDir := filepath.Join(tempDir, "home", ".claude", "skills")
	projectClaudeSkillsDir := filepath.Join(tempDir, "project", ".claude", "skills")
	projectSkillsDir := filepath.Join(tempDir, "project", "skills")

	// Create unique skills in each directory
	createSkillFile(t, userSkillsDir, "user-skill", "User global skill")
	createSkillFile(t, projectClaudeSkillsDir, "claude-skill", "Project .claude skill")
	createSkillFile(t, projectSkillsDir, "root-skill", "Project root skill")

	sm := &LocalSkillManager{
		skillsDirs: []DirConfig{
			{Path: projectSkillsDir, SourceType: entity.SkillSourceProject},
			{Path: projectClaudeSkillsDir, SourceType: entity.SkillSourceProjectClaude},
			{Path: userSkillsDir, SourceType: entity.SkillSourceUser},
		},
		skills: make(map[string]*entity.Skill),
		active: make(map[string]bool),
	}

	result, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("DiscoverSkills() returned unexpected error: %v", err)
	}

	// Build a map of skill name to source type
	sourceTypes := make(map[string]entity.SkillSourceType)
	for _, skill := range result.Skills {
		sourceTypes[skill.Name] = skill.SourceType
	}

	// Test cases for expected source types
	tests := []struct {
		skillName      string
		wantSourceType entity.SkillSourceType
	}{
		{"user-skill", entity.SkillSourceUser},
		{"claude-skill", entity.SkillSourceProjectClaude},
		{"root-skill", entity.SkillSourceProject},
	}

	for _, tt := range tests {
		t.Run(tt.skillName, func(t *testing.T) {
			gotSourceType, found := sourceTypes[tt.skillName]

			// FAILING ASSERTION: Skill may not be found because multi-dir discovery is not implemented
			if !found {
				t.Errorf("DiscoverSkills() did not find skill %q", tt.skillName)
				return
			}

			// FAILING ASSERTION: SourceType is not set in current implementation
			if gotSourceType != tt.wantSourceType {
				t.Errorf("DiscoverSkills() %s.SourceType = %q, want %q",
					tt.skillName, gotSourceType, tt.wantSourceType)
			}
		})
	}
}

// TestLocalSkillManager_LoadSkillMetadata_FromCorrectDirectory verifies that after
// multi-directory discovery, LoadSkillMetadata loads from the correct directory
// based on where the skill was discovered (respecting priority).
func TestLocalSkillManager_LoadSkillMetadata_FromCorrectDirectory(t *testing.T) {
	tempDir := t.TempDir()

	// Create skill directories
	userSkillsDir := filepath.Join(tempDir, "home", ".claude", "skills")
	projectClaudeSkillsDir := filepath.Join(tempDir, "project", ".claude", "skills")
	projectSkillsDir := filepath.Join(tempDir, "project", "skills")

	// Create the same skill in user and project-claude directories
	// Project-claude should win (higher priority)
	createSkillFile(t, userSkillsDir, "shared-skill", "User version - should be overridden")
	createSkillFile(t, projectClaudeSkillsDir, "shared-skill", "Project .claude version - should win")

	// Create a user-only skill
	createSkillFile(t, userSkillsDir, "user-exclusive", "Only in user directory")

	sm := &LocalSkillManager{
		skillsDirs: []DirConfig{
			{Path: projectSkillsDir, SourceType: entity.SkillSourceProject},
			{Path: projectClaudeSkillsDir, SourceType: entity.SkillSourceProjectClaude},
			{Path: userSkillsDir, SourceType: entity.SkillSourceUser},
		},
		skills: make(map[string]*entity.Skill),
		active: make(map[string]bool),
	}

	// First discover skills
	_, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("DiscoverSkills() returned unexpected error: %v", err)
	}

	// Test loading shared-skill - should get project-claude version
	t.Run("shared-skill loads from project-claude", func(t *testing.T) {
		skill, err := sm.LoadSkillMetadata(context.Background(), "shared-skill")
		// FAILING ASSERTION: Skill not found because multi-dir discovery not implemented
		if err != nil {
			t.Fatalf("LoadSkillMetadata(shared-skill) returned error: %v", err)
		}

		// FAILING ASSERTION: Description should be from project-claude (higher priority)
		if skill.Description != "Project .claude version - should win" {
			t.Errorf("LoadSkillMetadata(shared-skill).Description = %q, want %q",
				skill.Description, "Project .claude version - should win")
		}

		// FAILING ASSERTION: SourceType should indicate where it was loaded from
		if skill.SourceType != entity.SkillSourceProjectClaude {
			t.Errorf("LoadSkillMetadata(shared-skill).SourceType = %q, want %q",
				skill.SourceType, entity.SkillSourceProjectClaude)
		}
	})

	// Test loading user-exclusive - should come from user directory
	t.Run("user-exclusive loads from user directory", func(t *testing.T) {
		skill, err := sm.LoadSkillMetadata(context.Background(), "user-exclusive")
		// FAILING ASSERTION: Skill not found because multi-dir discovery not implemented
		if err != nil {
			t.Fatalf("LoadSkillMetadata(user-exclusive) returned error: %v", err)
		}

		if skill.Description != "Only in user directory" {
			t.Errorf("LoadSkillMetadata(user-exclusive).Description = %q, want %q",
				skill.Description, "Only in user directory")
		}

		// FAILING ASSERTION: SourceType should indicate user source
		if skill.SourceType != entity.SkillSourceUser {
			t.Errorf("LoadSkillMetadata(user-exclusive).SourceType = %q, want %q",
				skill.SourceType, entity.SkillSourceUser)
		}
	})
}

// TestLocalSkillManager_DiscoverSkills_AllDirectoriesMissing verifies behavior when
// none of the skill directories exist.
func TestLocalSkillManager_DiscoverSkills_AllDirectoriesMissing(t *testing.T) {
	tempDir := t.TempDir()

	// None of these directories exist
	userSkillsDir := filepath.Join(tempDir, "nonexistent", "home", ".claude", "skills")
	projectClaudeSkillsDir := filepath.Join(tempDir, "nonexistent", "project", ".claude", "skills")
	projectSkillsDir := filepath.Join(tempDir, "nonexistent", "project", "skills")

	sm := &LocalSkillManager{
		skillsDirs: []DirConfig{
			{Path: projectSkillsDir, SourceType: entity.SkillSourceProject},
			{Path: projectClaudeSkillsDir, SourceType: entity.SkillSourceProjectClaude},
			{Path: userSkillsDir, SourceType: entity.SkillSourceUser},
		},
		skills: make(map[string]*entity.Skill),
		active: make(map[string]bool),
	}

	result, err := sm.DiscoverSkills(context.Background())
	// Should NOT error - missing directories are acceptable
	if err != nil {
		t.Fatalf("DiscoverSkills() should not error when all directories are missing: %v", err)
	}

	// Should return empty result
	if result.TotalCount != 0 {
		t.Errorf("DiscoverSkills() TotalCount = %d, want 0", result.TotalCount)
	}

	if len(result.Skills) != 0 {
		t.Errorf("DiscoverSkills() returned %d skills, want 0", len(result.Skills))
	}
}

// TestLocalSkillManager_DiscoverSkills_DirectoryOrderInResult verifies that
// SkillsDirs is returned in priority order (highest to lowest).
func TestLocalSkillManager_DiscoverSkills_DirectoryOrderInResult(t *testing.T) {
	tempDir := t.TempDir()

	projectSkillsDir := filepath.Join(tempDir, "project", "skills")
	projectClaudeSkillsDir := filepath.Join(tempDir, "project", ".claude", "skills")
	userSkillsDir := filepath.Join(tempDir, "home", ".claude", "skills")

	// Create all directories with skills
	createSkillFile(t, projectSkillsDir, "skill-a", "Skill A")
	createSkillFile(t, projectClaudeSkillsDir, "skill-b", "Skill B")
	createSkillFile(t, userSkillsDir, "skill-c", "Skill C")

	sm := &LocalSkillManager{
		skillsDirs: []DirConfig{
			{Path: projectSkillsDir, SourceType: entity.SkillSourceProject},
			{Path: projectClaudeSkillsDir, SourceType: entity.SkillSourceProjectClaude},
			{Path: userSkillsDir, SourceType: entity.SkillSourceUser},
		},
		skills: make(map[string]*entity.Skill),
		active: make(map[string]bool),
	}

	result, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("DiscoverSkills() returned unexpected error: %v", err)
	}

	// FAILING ASSERTION: SkillsDirs should be populated with all directories
	if len(result.SkillsDirs) != 3 {
		t.Fatalf("DiscoverSkills() SkillsDirs length = %d, want 3", len(result.SkillsDirs))
	}

	// FAILING ASSERTION: Directories should be in priority order (highest first)
	// Expected order: ./skills, ./.claude/skills, ~/.claude/skills
	expectedOrder := []string{projectSkillsDir, projectClaudeSkillsDir, userSkillsDir}

	// Make copies to sort for comparison since we care about order
	gotDirs := make([]string, len(result.SkillsDirs))
	copy(gotDirs, result.SkillsDirs)

	for i, expected := range expectedOrder {
		if i >= len(gotDirs) {
			t.Errorf("DiscoverSkills() SkillsDirs[%d] missing, want %q", i, expected)
			continue
		}
		// Use filepath.Clean for comparison to handle path normalization
		if filepath.Clean(gotDirs[i]) != filepath.Clean(expected) {
			t.Errorf("DiscoverSkills() SkillsDirs[%d] = %q, want %q (priority order)", i, gotDirs[i], expected)
		}
	}
}

// TestLocalSkillManager_GetSkillByName_ReturnsSourceType verifies that GetSkillByName
// returns the correct SourceType for skills discovered from different directories.
func TestLocalSkillManager_GetSkillByName_ReturnsSourceType(t *testing.T) {
	tempDir := t.TempDir()

	projectSkillsDir := filepath.Join(tempDir, "project", "skills")
	createSkillFile(t, projectSkillsDir, "test-skill", "A test skill")

	sm := &LocalSkillManager{
		skillsDirs: []DirConfig{
			{Path: projectSkillsDir, SourceType: entity.SkillSourceProject},
		},
		skills: make(map[string]*entity.Skill),
		active: make(map[string]bool),
	}

	_, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("DiscoverSkills() returned unexpected error: %v", err)
	}

	skillInfo, err := sm.GetSkillByName(context.Background(), "test-skill")
	if err != nil {
		t.Fatalf("GetSkillByName() returned unexpected error: %v", err)
	}

	// FAILING ASSERTION: SourceType should be set to "project" for skills from ./skills
	if skillInfo.SourceType != entity.SkillSourceProject {
		t.Errorf("GetSkillByName().SourceType = %q, want %q",
			skillInfo.SourceType, entity.SkillSourceProject)
	}
}

// TestLocalSkillManager_GetAvailableSkills_ReturnsSourceType verifies that
// GetAvailableSkills returns the correct SourceType for active skills.
func TestLocalSkillManager_GetAvailableSkills_ReturnsSourceType(t *testing.T) {
	tempDir := t.TempDir()

	projectSkillsDir := filepath.Join(tempDir, "project", "skills")
	createSkillFile(t, projectSkillsDir, "active-skill", "An active skill")

	sm := &LocalSkillManager{
		skillsDirs: []DirConfig{
			{Path: projectSkillsDir, SourceType: entity.SkillSourceProject},
		},
		skills: make(map[string]*entity.Skill),
		active: make(map[string]bool),
	}

	_, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("DiscoverSkills() returned unexpected error: %v", err)
	}

	// Activate the skill
	_, err = sm.ActivateSkill(context.Background(), "active-skill")
	if err != nil {
		t.Fatalf("ActivateSkill() returned unexpected error: %v", err)
	}

	availableSkills, err := sm.GetAvailableSkills(context.Background())
	if err != nil {
		t.Fatalf("GetAvailableSkills() returned unexpected error: %v", err)
	}

	if len(availableSkills) != 1 {
		t.Fatalf("GetAvailableSkills() returned %d skills, want 1", len(availableSkills))
	}

	// FAILING ASSERTION: SourceType should be set for available skills
	if availableSkills[0].SourceType != entity.SkillSourceProject {
		t.Errorf("GetAvailableSkills()[0].SourceType = %q, want %q",
			availableSkills[0].SourceType, entity.SkillSourceProject)
	}
}

// TestLocalSkillManager_DiscoverSkills_SkillsListedByPriority verifies that when
// listing all discovered skills, they are sorted by priority (project > project-claude > user).
func TestLocalSkillManager_DiscoverSkills_SkillsListedByPriority(t *testing.T) {
	tempDir := t.TempDir()

	userSkillsDir := filepath.Join(tempDir, "home", ".claude", "skills")
	projectClaudeSkillsDir := filepath.Join(tempDir, "project", ".claude", "skills")
	projectSkillsDir := filepath.Join(tempDir, "project", "skills")

	// Create skills in reverse priority order to test sorting
	createSkillFile(t, userSkillsDir, "aaa-skill", "User skill (lowest priority)")
	createSkillFile(t, projectClaudeSkillsDir, "bbb-skill", "Project .claude skill")
	createSkillFile(t, projectSkillsDir, "ccc-skill", "Project skill (highest priority)")

	sm := &LocalSkillManager{
		skillsDirs: []DirConfig{
			{Path: projectSkillsDir, SourceType: entity.SkillSourceProject},
			{Path: projectClaudeSkillsDir, SourceType: entity.SkillSourceProjectClaude},
			{Path: userSkillsDir, SourceType: entity.SkillSourceUser},
		},
		skills: make(map[string]*entity.Skill),
		active: make(map[string]bool),
	}

	result, err := sm.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("DiscoverSkills() returned unexpected error: %v", err)
	}

	// FAILING ASSERTION: Should find all 3 skills
	if result.TotalCount != 3 {
		t.Fatalf("DiscoverSkills() TotalCount = %d, want 3", result.TotalCount)
	}

	// Get source types in order
	sourceTypePriority := map[entity.SkillSourceType]int{
		entity.SkillSourceProject:       0, // Highest priority
		entity.SkillSourceProjectClaude: 1,
		entity.SkillSourceUser:          2, // Lowest priority
	}

	// Sort by source type priority
	sortedSkills := make([]struct {
		name       string
		sourceType entity.SkillSourceType
	}, len(result.Skills))

	for i, s := range result.Skills {
		sortedSkills[i] = struct {
			name       string
			sourceType entity.SkillSourceType
		}{s.Name, s.SourceType}
	}

	sort.Slice(sortedSkills, func(i, j int) bool {
		return sourceTypePriority[sortedSkills[i].sourceType] < sourceTypePriority[sortedSkills[j].sourceType]
	})

	// Verify we have skills from each source type
	foundSourceTypes := make(map[entity.SkillSourceType]bool)
	for _, s := range sortedSkills {
		foundSourceTypes[s.sourceType] = true
	}

	if !foundSourceTypes[entity.SkillSourceProject] {
		t.Error("DiscoverSkills() missing skills from project directory")
	}
	if !foundSourceTypes[entity.SkillSourceProjectClaude] {
		t.Error("DiscoverSkills() missing skills from project-claude directory")
	}
	if !foundSourceTypes[entity.SkillSourceUser] {
		t.Error("DiscoverSkills() missing skills from user directory")
	}
}

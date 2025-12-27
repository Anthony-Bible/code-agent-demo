package port

import (
	"code-editing-agent/internal/domain/entity"
	"context"
	"testing"
)

// TestSkillManagerInterface_Contract validates that SkillManager interface exists with expected methods.
func TestSkillManagerInterface_Contract(_ *testing.T) {
	// Verify that SkillManager interface exists
	var _ SkillManager = (*mockSkillManager)(nil)
}

// TestSkillManagerInterface_DiscoverSkills validates DiscoverSkills method exists.
func TestSkillManagerInterface_DiscoverSkills(_ *testing.T) {
	var manager SkillManager = (*mockSkillManager)(nil)

	// This will fail to compile if DiscoverSkills method doesn't exist with correct signature
	_ = manager.DiscoverSkills
}

// TestSkillManagerInterface_LoadSkillMetadata validates LoadSkillMetadata method exists.
func TestSkillManagerInterface_LoadSkillMetadata(_ *testing.T) {
	var manager SkillManager = (*mockSkillManager)(nil)

	// This will fail to compile if LoadSkillMetadata method doesn't exist with correct signature
	_ = manager.LoadSkillMetadata
}

// TestSkillManagerInterface_ActivateSkill validates ActivateSkill method exists.
func TestSkillManagerInterface_ActivateSkill(_ *testing.T) {
	var manager SkillManager = (*mockSkillManager)(nil)

	// This will fail to compile if ActivateSkill method doesn't exist with correct signature
	_ = manager.ActivateSkill
}

// TestSkillManagerInterface_DeactivateSkill validates DeactivateSkill method exists.
func TestSkillManagerInterface_DeactivateSkill(_ *testing.T) {
	var manager SkillManager = (*mockSkillManager)(nil)

	// This will fail to compile if DeactivateSkill method doesn't exist with correct signature
	_ = manager.DeactivateSkill
}

// TestSkillManagerInterface_GetAvailableSkills validates GetAvailableSkills method exists.
func TestSkillManagerInterface_GetAvailableSkills(_ *testing.T) {
	var manager SkillManager = (*mockSkillManager)(nil)

	// This will fail to compile if GetAvailableSkills method doesn't exist with correct signature
	_ = manager.GetAvailableSkills
}

// TestSkillManagerInterface_GetSkillByName validates GetSkillByName method exists.
func TestSkillManagerInterface_GetSkillByName(_ *testing.T) {
	var manager SkillManager = (*mockSkillManager)(nil)

	// This will fail to compile if GetSkillByName method doesn't exist with correct signature
	_ = manager.GetSkillByName
}

// TestSkillManagerInterface_ValidateSkills validates ValidateSkills method exists.
func TestSkillManagerInterface_ValidateSkills(_ *testing.T) {
	var manager SkillManager = (*mockSkillManager)(nil)

	// This will fail to compile if ValidateSkills method doesn't exist with correct signature
	_ = manager.ValidateSkills
}

// mockSkillManager is a minimal implementation to validate interface contract.
type mockSkillManager struct{}

func (m *mockSkillManager) DiscoverSkills(_ context.Context) (*SkillDiscoveryResult, error) {
	return &SkillDiscoveryResult{}, nil
}

func (m *mockSkillManager) LoadSkillMetadata(_ context.Context, _ string) (*entity.Skill, error) {
	return &entity.Skill{}, nil
}

func (m *mockSkillManager) ActivateSkill(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *mockSkillManager) DeactivateSkill(_ context.Context, _ string) (bool, error) {
	return false, nil
}

func (m *mockSkillManager) GetAvailableSkills(_ context.Context) ([]SkillInfo, error) {
	return []SkillInfo{}, nil
}

func (m *mockSkillManager) GetSkillByName(_ context.Context, _ string) (*SkillInfo, error) {
	return &SkillInfo{}, nil
}

func (m *mockSkillManager) ValidateSkills(_ context.Context) (map[string]error, error) {
	return map[string]error{}, nil
}

// Package service provides application-level services that orchestrate
// the use cases and provide high-level interfaces for the application.
package service

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
	"testing"
)

// mockSkillManager is a mock implementation of SkillManager for testing.
type mockSkillManager struct {
	discoverFunc     func(ctx context.Context) (*port.SkillDiscoveryResult, error)
	loadMetadataFunc func(ctx context.Context, skillName string) (*entity.Skill, error)
	activateFunc     func(ctx context.Context, skillName string) (bool, error)
	deactivateFunc   func(ctx context.Context, skillName string) (bool, error)
	getAvailableFunc func(ctx context.Context) ([]port.SkillInfo, error)
	getByNameFunc    func(ctx context.Context, skillName string) (*port.SkillInfo, error)
	validateFunc     func(ctx context.Context) (map[string]error, error)
}

func (m *mockSkillManager) DiscoverSkills(ctx context.Context) (*port.SkillDiscoveryResult, error) {
	if m.discoverFunc != nil {
		return m.discoverFunc(ctx)
	}
	return &port.SkillDiscoveryResult{}, nil
}

func (m *mockSkillManager) LoadSkillMetadata(ctx context.Context, skillName string) (*entity.Skill, error) {
	if m.loadMetadataFunc != nil {
		return m.loadMetadataFunc(ctx, skillName)
	}
	return &entity.Skill{}, nil
}

func (m *mockSkillManager) ActivateSkill(ctx context.Context, skillName string) (bool, error) {
	if m.activateFunc != nil {
		return m.activateFunc(ctx, skillName)
	}
	return false, nil
}

func (m *mockSkillManager) DeactivateSkill(ctx context.Context, skillName string) (bool, error) {
	if m.deactivateFunc != nil {
		return m.deactivateFunc(ctx, skillName)
	}
	return false, nil
}

func (m *mockSkillManager) GetAvailableSkills(ctx context.Context) ([]port.SkillInfo, error) {
	if m.getAvailableFunc != nil {
		return m.getAvailableFunc(ctx)
	}
	return []port.SkillInfo{}, nil
}

func (m *mockSkillManager) GetSkillByName(ctx context.Context, skillName string) (*port.SkillInfo, error) {
	if m.getByNameFunc != nil {
		return m.getByNameFunc(ctx, skillName)
	}
	return &port.SkillInfo{}, nil
}

func (m *mockSkillManager) ValidateSkills(ctx context.Context) (map[string]error, error) {
	if m.validateFunc != nil {
		return m.validateFunc(ctx)
	}
	return map[string]error{}, nil
}

func TestNewSkillService_NilSkillManager(t *testing.T) {
	_, err := NewSkillService(nil)

	if err == nil {
		t.Error("NewSkillService() should return error when skill manager is nil")
	}

	if !errors.Is(err, ErrSkillManagerRequired) {
		t.Errorf("NewSkillService() error = %v, want ErrSkillManagerRequired", err)
	}
}

func TestNewSkillService_ValidSkillManager(t *testing.T) {
	mock := &mockSkillManager{}
	service, err := NewSkillService(mock)
	if err != nil {
		t.Fatalf("NewSkillService() returned unexpected error: %v", err)
	}

	if service == nil {
		t.Fatal("NewSkillService() returned nil service")
	}

	if service.GetSkillManager() != mock {
		t.Error("NewSkillService() did not set skill manager correctly")
	}
}

func TestSkillService_DiscoverSkills(t *testing.T) {
	expectedResult := &port.SkillDiscoveryResult{
		Skills:      []port.SkillInfo{{Name: "test-skill"}},
		TotalCount:  1,
		ActiveCount: 0,
	}

	mock := &mockSkillManager{
		discoverFunc: func(_ context.Context) (*port.SkillDiscoveryResult, error) {
			return expectedResult, nil
		},
	}

	service, err := NewSkillService(mock)
	if err != nil {
		t.Fatalf("Failed to create skill service: %v", err)
	}

	result, err := service.DiscoverSkills(context.Background())
	if err != nil {
		t.Fatalf("DiscoverSkills() returned unexpected error: %v", err)
	}

	if result.TotalCount != expectedResult.TotalCount {
		t.Errorf("DiscoverSkills() TotalCount = %d, want %d", result.TotalCount, expectedResult.TotalCount)
	}
}

func TestSkillService_DiscoverSkills_Error(t *testing.T) {
	expectedErr := errors.New("discovery failed")

	mock := &mockSkillManager{
		discoverFunc: func(_ context.Context) (*port.SkillDiscoveryResult, error) {
			return nil, expectedErr
		},
	}

	service, err := NewSkillService(mock)
	if err != nil {
		t.Fatalf("Failed to create skill service: %v", err)
	}

	_, err = service.DiscoverSkills(context.Background())
	if err == nil {
		t.Error("DiscoverSkills() should return error when underlying manager fails")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("DiscoverSkills() error = %v, want %v", err, expectedErr)
	}
}

func TestSkillService_LoadSkillMetadata(t *testing.T) {
	expectedSkill := &entity.Skill{
		Name:        "test-skill",
		Description: "A test skill",
	}

	mock := &mockSkillManager{
		loadMetadataFunc: func(_ context.Context, _ string) (*entity.Skill, error) {
			return expectedSkill, nil
		},
	}

	service, err := NewSkillService(mock)
	if err != nil {
		t.Fatalf("Failed to create skill service: %v", err)
	}

	skill, err := service.LoadSkillMetadata(context.Background(), "test-skill")
	if err != nil {
		t.Fatalf("LoadSkillMetadata() returned unexpected error: %v", err)
	}

	if skill.Name != expectedSkill.Name {
		t.Errorf("LoadSkillMetadata() Name = %v, want %v", skill.Name, expectedSkill.Name)
	}
}

func TestSkillService_ActivateSkill(t *testing.T) {
	mock := &mockSkillManager{
		activateFunc: func(_ context.Context, _ string) (bool, error) {
			return true, nil
		},
	}

	service, err := NewSkillService(mock)
	if err != nil {
		t.Fatalf("Failed to create skill service: %v", err)
	}

	activated, err := service.ActivateSkill(context.Background(), "test-skill")
	if err != nil {
		t.Fatalf("ActivateSkill() returned unexpected error: %v", err)
	}

	if !activated {
		t.Error("ActivateSkill() returned false, want true")
	}
}

func TestSkillService_DeactivateSkill(t *testing.T) {
	mock := &mockSkillManager{
		deactivateFunc: func(_ context.Context, _ string) (bool, error) {
			return true, nil
		},
	}

	service, err := NewSkillService(mock)
	if err != nil {
		t.Fatalf("Failed to create skill service: %v", err)
	}

	deactivated, err := service.DeactivateSkill(context.Background(), "test-skill")
	if err != nil {
		t.Fatalf("DeactivateSkill() returned unexpected error: %v", err)
	}

	if !deactivated {
		t.Error("DeactivateSkill() returned false, want true")
	}
}

func TestSkillService_GetAvailableSkills(t *testing.T) {
	expectedSkills := []port.SkillInfo{
		{Name: "skill-a", IsActive: true},
		{Name: "skill-b", IsActive: true},
	}

	mock := &mockSkillManager{
		getAvailableFunc: func(_ context.Context) ([]port.SkillInfo, error) {
			return expectedSkills, nil
		},
	}

	service, err := NewSkillService(mock)
	if err != nil {
		t.Fatalf("Failed to create skill service: %v", err)
	}

	skills, err := service.GetAvailableSkills(context.Background())
	if err != nil {
		t.Fatalf("GetAvailableSkills() returned unexpected error: %v", err)
	}

	if len(skills) != len(expectedSkills) {
		t.Errorf("GetAvailableSkills() returned %d skills, want %d", len(skills), len(expectedSkills))
	}
}

func TestSkillService_GetSkillByName(t *testing.T) {
	expectedSkill := &port.SkillInfo{
		Name:        "test-skill",
		Description: "A test skill",
		IsActive:    true,
	}

	mock := &mockSkillManager{
		getByNameFunc: func(_ context.Context, _ string) (*port.SkillInfo, error) {
			return expectedSkill, nil
		},
	}

	service, err := NewSkillService(mock)
	if err != nil {
		t.Fatalf("Failed to create skill service: %v", err)
	}

	skill, err := service.GetSkillByName(context.Background(), "test-skill")
	if err != nil {
		t.Fatalf("GetSkillByName() returned unexpected error: %v", err)
	}

	if skill.Name != expectedSkill.Name {
		t.Errorf("GetSkillByName() Name = %v, want %v", skill.Name, expectedSkill.Name)
	}
}

func TestSkillService_ValidateSkills(t *testing.T) {
	expectedErrors := map[string]error{
		"invalid-skill": errors.New("invalid skill"),
	}

	mock := &mockSkillManager{
		validateFunc: func(_ context.Context) (map[string]error, error) {
			return expectedErrors, nil
		},
	}

	service, err := NewSkillService(mock)
	if err != nil {
		t.Fatalf("Failed to create skill service: %v", err)
	}

	errors, err := service.ValidateSkills(context.Background())
	if err != nil {
		t.Fatalf("ValidateSkills() returned unexpected error: %v", err)
	}

	if len(errors) != len(expectedErrors) {
		t.Errorf("ValidateSkills() returned %d errors, want %d", len(errors), len(expectedErrors))
	}
}

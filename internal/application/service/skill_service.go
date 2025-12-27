// Package service provides application-level services that orchestrate
// the use cases and provide high-level interfaces for the application.
package service

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
)

// ErrSkillManagerRequired is returned when SkillManager is nil.
var ErrSkillManagerRequired = errors.New("skill manager is required")

// SkillService is the high-level orchestration service for skill management operations.
// It coordinates skill discovery, activation, deactivation, and validation operations
// by wrapping the underlying SkillManager port.
//
// This service serves as the main entry point for skill management in the
// application layer, following hexagonal architecture principles.
type SkillService struct {
	skillManager port.SkillManager
}

// NewSkillService creates a new SkillService with required dependencies.
//
// Parameters:
//   - sm: Skill manager port for skill operations
//
// Returns:
//   - *SkillService: A new skill service instance
//   - error: An error if the skill manager is nil
func NewSkillService(sm port.SkillManager) (*SkillService, error) {
	if sm == nil {
		return nil, ErrSkillManagerRequired
	}

	return &SkillService{
		skillManager: sm,
	}, nil
}

// DiscoverSkills scans the skills directory for available skills.
// This is a convenience wrapper around the underlying SkillManager port.
//
// Parameters:
//   - ctx: Context for the operation
//
// Returns:
//   - *port.SkillDiscoveryResult: Information about discovered skills
//   - error: An error if discovery fails
func (ss *SkillService) DiscoverSkills(ctx context.Context) (*port.SkillDiscoveryResult, error) {
	return ss.skillManager.DiscoverSkills(ctx)
}

// LoadSkillMetadata loads the metadata for a specific skill.
//
// Parameters:
//   - ctx: Context for the operation
//   - skillName: Name of the skill to load
//
// Returns:
//   - *entity.Skill: The skill entity with all metadata
//   - error: An error if loading fails
func (ss *SkillService) LoadSkillMetadata(ctx context.Context, skillName string) (*entity.Skill, error) {
	return ss.skillManager.LoadSkillMetadata(ctx, skillName)
}

// ActivateSkill activates a skill by name.
//
// Parameters:
//   - ctx: Context for the operation
//   - skillName: Name of the skill to activate
//
// Returns:
//   - bool: True if the skill was activated
//   - error: An error if activation fails
func (ss *SkillService) ActivateSkill(ctx context.Context, skillName string) (bool, error) {
	return ss.skillManager.ActivateSkill(ctx, skillName)
}

// DeactivateSkill deactivates a skill by name.
//
// Parameters:
//   - ctx: Context for the operation
//   - skillName: Name of the skill to deactivate
//
// Returns:
//   - bool: True if the skill was deactivated
//   - error: An error if deactivation fails
func (ss *SkillService) DeactivateSkill(ctx context.Context, skillName string) (bool, error) {
	return ss.skillManager.DeactivateSkill(ctx, skillName)
}

// GetAvailableSkills returns a list of all currently active skills.
//
// Parameters:
//   - ctx: Context for the operation
//
// Returns:
//   - []port.SkillInfo: List of active skills
//   - error: An error if retrieval fails
func (ss *SkillService) GetAvailableSkills(ctx context.Context) ([]port.SkillInfo, error) {
	return ss.skillManager.GetAvailableSkills(ctx)
}

// GetSkillByName returns information about a specific skill.
//
// Parameters:
//   - ctx: Context for the operation
//   - skillName: Name of the skill to retrieve
//
// Returns:
//   - *port.SkillInfo: Information about the skill
//   - error: An error if the skill is not found
func (ss *SkillService) GetSkillByName(ctx context.Context, skillName string) (*port.SkillInfo, error) {
	return ss.skillManager.GetSkillByName(ctx, skillName)
}

// ValidateSkills checks all available skills for validity.
//
// Parameters:
//   - ctx: Context for the operation
//
// Returns:
//   - map[string]error: Validation errors by skill name
//   - error: An error if validation fails
func (ss *SkillService) ValidateSkills(ctx context.Context) (map[string]error, error) {
	return ss.skillManager.ValidateSkills(ctx)
}

// GetSkillManager returns the underlying skill manager port.
// This is primarily intended for testing or scenarios where direct port access is needed.
//
// Returns:
//   - port.SkillManager: The skill manager port
func (ss *SkillService) GetSkillManager() port.SkillManager {
	return ss.skillManager
}

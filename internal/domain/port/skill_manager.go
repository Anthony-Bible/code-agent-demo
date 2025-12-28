package port

import (
	"code-editing-agent/internal/domain/entity"
	"context"
)

// SkillInfo represents information about a discovered skill.
type SkillInfo struct {
	Name          string            `json:"name"`           // Name of the skill
	Description   string            `json:"description"`    // Description of what the skill does
	License       string            `json:"license"`        // License information
	Compatibility string            `json:"compatibility"`  // Compatibility information
	Metadata      map[string]string `json:"metadata"`       // Additional metadata
	AllowedTools  []string          `json:"allowed_tools"`  // Allowed tools for this skill
	DirectoryPath string            `json:"directory_path"` // Path to skill directory
	IsActive      bool              `json:"is_active"`      // Whether the skill is currently active
}

// SkillDiscoveryResult represents the result of a skill discovery operation.
type SkillDiscoveryResult struct {
	Skills      []SkillInfo `json:"skills"`       // Discovered skills
	SkillsDir   string      `json:"skills_dir"`   // Directory where skills were discovered
	TotalCount  int         `json:"total_count"`  // Total number of skills discovered
	ActiveCount int         `json:"active_count"` // Number of active skills
}

// SkillManager defines the interface for managing agent skills.
// This port represents the outbound dependency for skill operations and follows
// hexagonal architecture principles by abstracting skill management implementations.
// Skills follow the agentskills.io specification where skills are directories
// with SKILL.md files containing YAML frontmatter.
type SkillManager interface {
	// DiscoverSkills scans the skills directory for available skills.
	// Skills are discovered from ./skills directory relative to working directory.
	// Returns information about all discovered skills including metadata.
	DiscoverSkills(ctx context.Context) (*SkillDiscoveryResult, error)

	// LoadSkillMetadata loads the metadata for a specific skill from its SKILL.md file.
	// The skillName should match the skill directory name.
	// Returns the skill entity with all parsed metadata.
	LoadSkillMetadata(ctx context.Context, skillName string) (*entity.Skill, error)

	// ActivateSkill activates a skill by name, making it available for use by the AI.
	// Activated skills can be invoked by the AI through the tool system.
	// Returns true if the skill was successfully activated.
	ActivateSkill(ctx context.Context, skillName string) (bool, error)

	// DeactivateSkill deactivates a skill by name, removing it from available tools.
	// Returns true if the skill was successfully deactivated.
	DeactivateSkill(ctx context.Context, skillName string) (bool, error)

	// GetAvailableSkills returns a list of all currently active skills.
	// Active skills are those that have been activated and are available for use.
	GetAvailableSkills(ctx context.Context) ([]SkillInfo, error)

	// GetSkillByName returns information about a specific skill by name.
	// Returns nil if the skill is not found.
	GetSkillByName(ctx context.Context, skillName string) (*SkillInfo, error)

	// ValidateSkills checks all available skills for validity.
	// Returns validation errors for any skills that fail validation.
	ValidateSkills(ctx context.Context) (map[string]error, error)
}

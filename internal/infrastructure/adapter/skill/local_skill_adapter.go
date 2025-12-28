// Package skill provides an implementation of the domain SkillManager port.
// It follows hexagonal architecture principles by providing infrastructure-level
// skill discovery and management operations for the AI coding agent.
//
// Skills are discovered from the ./skills directory relative to the current
// working directory. Each skill is represented by a directory containing a
// SKILL.md file with YAML frontmatter defining the skill's metadata.
//
// Example usage:
//
//	sm := skill.NewLocalSkillManager()
//	result, err := sm.DiscoverSkills(context.Background())
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Found %d skills\n", result.TotalCount)
package skill

import (
	"code-editing-agent/internal/domain/entity"
	"code-editing-agent/internal/domain/port"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrSkillNotFound     = errors.New("skill not found")
	ErrSkillFileNotFound = errors.New("SKILL.md file not found in skill directory")
)

// LocalSkillManager implements the SkillManager port for managing local file system skills.
// It discovers skills from the ./skills directory, loads their metadata, and manages
// their activation state.
type LocalSkillManager struct {
	mu        sync.RWMutex
	skillsDir string                   // Directory containing skill directories
	skills    map[string]*entity.Skill // Discovered skills by name
	active    map[string]bool          // Active skills by name
}

// NewLocalSkillManager creates a new LocalSkillManager instance.
// Skills are discovered from ./skills directory relative to working directory.
func NewLocalSkillManager() port.SkillManager {
	return &LocalSkillManager{
		skillsDir: "./skills",
		skills:    make(map[string]*entity.Skill),
		active:    make(map[string]bool),
	}
}

// DiscoverSkills scans the skills directory for available skills.
// Skills are discovered from ./skills directory relative to working directory.
// Returns information about all discovered skills including metadata.
func (sm *LocalSkillManager) DiscoverSkills(_ context.Context) (*port.SkillDiscoveryResult, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Clear existing skills
	sm.skills = make(map[string]*entity.Skill)

	// Check if skills directory exists
	info, err := os.Stat(sm.skillsDir)
	switch {
	case os.IsNotExist(err):
		return &port.SkillDiscoveryResult{
			Skills:      []port.SkillInfo{},
			SkillsDir:   sm.skillsDir,
			TotalCount:  0,
			ActiveCount: 0,
		}, nil
	case err != nil:
		return nil, fmt.Errorf("failed to access skills directory: %w", err)
	case !info.IsDir():
		return nil, fmt.Errorf("skills path is not a directory: %s", sm.skillsDir)
	}

	// Walk the skills directory
	var discoveredSkills []port.SkillInfo
	activeCount := 0

	err = filepath.Walk(sm.skillsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip the root skills directory
		if path == sm.skillsDir {
			return nil
		}

		// Look for SKILL.md files and process them
		if info.Name() == "SKILL.md" && !info.IsDir() {
			if skillInfo := sm.processSkillFile(path); skillInfo != nil {
				discoveredSkills = append(discoveredSkills, *skillInfo)
				if skillInfo.IsActive {
					activeCount++
				}
			}
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to walk skills directory: %w", err)
	}

	return &port.SkillDiscoveryResult{
		Skills:      discoveredSkills,
		SkillsDir:   sm.skillsDir,
		TotalCount:  len(discoveredSkills),
		ActiveCount: activeCount,
	}, nil
}

// processSkillFile processes a single SKILL.md file and returns skill info if valid.
// Returns nil if the skill is invalid or cannot be processed.
func (sm *LocalSkillManager) processSkillFile(path string) *port.SkillInfo {
	// Load and parse the skill metadata only (progressive disclosure)
	content, err := os.ReadFile(path)
	if err != nil {
		// Skip file read errors and continue discovery
		return nil
	}

	skill, parseErr := entity.ParseSkillMetadataFromYAML(string(content))
	// Intentionally skip invalid skills and continue discovering others
	if parseErr != nil {
		return nil
	}

	// Validate the skill before storing
	if err := skill.Validate(); err != nil {
		// Skip invalid skill and continue discovery
		return nil
	}

	// Validate directory name matches skill name (agentskills.io spec requirement)
	dirName := filepath.Base(filepath.Dir(path))
	if err := skill.ValidateDirectoryName(dirName); err != nil {
		// Skip invalid skill and continue discovery
		return nil
	}

	// Set the path
	absPath, _ := filepath.Abs(filepath.Dir(path))
	skill.ScriptPath = absPath
	skill.OriginalPath = filepath.Dir(path)

	// Store the skill
	sm.skills[skill.Name] = skill

	// Check if active
	isActive := sm.active[skill.Name]

	return &port.SkillInfo{
		Name:          skill.Name,
		Description:   skill.Description,
		License:       skill.License,
		Compatibility: skill.Compatibility,
		Metadata:      skill.Metadata,
		AllowedTools:  skill.AllowedTools,
		DirectoryPath: skill.OriginalPath,
		IsActive:      isActive,
	}
}

// LoadSkillMetadata loads the metadata for a specific skill from its SKILL.md file.
// The skillName should match the skill directory name.
// Returns the skill entity with all parsed metadata.
// If the skill was discovered with ParseSkillMetadataFromYAML (progressive disclosure),
// this function will load the full content on-demand.
func (sm *LocalSkillManager) LoadSkillMetadata(_ context.Context, skillName string) (*entity.Skill, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if skill is already loaded
	skill, exists := sm.skills[skillName]
	if exists && skill.RawContent != "" {
		// Skill exists and has full content loaded
		return skill, nil
	}

	// If skill exists but has no content, or doesn't exist, load full content
	skillPath := filepath.Join(sm.skillsDir, skillName, "SKILL.md")
	content, err := os.ReadFile(skillPath)
	if os.IsNotExist(err) {
		return nil, ErrSkillFileNotFound
	} else if err != nil {
		return nil, fmt.Errorf("failed to read SKILL.md: %w", err)
	}

	fullSkill, err := entity.ParseSkillFromYAML(string(content))
	if err != nil {
		return nil, fmt.Errorf("failed to parse skill: %w", err)
	}

	if exists {
		// Update the existing skill with full content while preserving path info
		skill.RawContent = fullSkill.RawContent
		skill.RawFrontmatter = fullSkill.RawFrontmatter
		return skill, nil
	}

	// Return the newly parsed skill
	return fullSkill, nil
}

// ActivateSkill activates a skill by name, making it available for use by the AI.
// Activated skills can be invoked by the AI through the tool system.
// Returns true if the skill was successfully activated.
func (sm *LocalSkillManager) ActivateSkill(_ context.Context, skillName string) (bool, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if skill exists
	if _, ok := sm.skills[skillName]; !ok {
		return false, ErrSkillNotFound
	}

	// Mark as active
	sm.active[skillName] = true
	return true, nil
}

// DeactivateSkill deactivates a skill by name, removing it from available tools.
// Returns true if the skill was successfully deactivated.
func (sm *LocalSkillManager) DeactivateSkill(_ context.Context, skillName string) (bool, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, ok := sm.skills[skillName]; !ok {
		return false, ErrSkillNotFound
	}

	// Mark as inactive
	delete(sm.active, skillName)
	return true, nil
}

// GetAvailableSkills returns a list of all currently active skills.
// Active skills are those that have been activated and are available for use.
func (sm *LocalSkillManager) GetAvailableSkills(_ context.Context) ([]port.SkillInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	var availableSkills []port.SkillInfo
	for _, skill := range sm.skills {
		if sm.active[skill.Name] {
			availableSkills = append(availableSkills, port.SkillInfo{
				Name:          skill.Name,
				Description:   skill.Description,
				License:       skill.License,
				Compatibility: skill.Compatibility,
				Metadata:      skill.Metadata,
				AllowedTools:  skill.AllowedTools,
				DirectoryPath: skill.OriginalPath,
				IsActive:      true,
			})
		}
	}

	return availableSkills, nil
}

// GetSkillByName returns information about a specific skill by name.
// Returns nil if the skill is not found.
func (sm *LocalSkillManager) GetSkillByName(_ context.Context, skillName string) (*port.SkillInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	skill, ok := sm.skills[skillName]
	if !ok {
		return nil, ErrSkillNotFound
	}

	return &port.SkillInfo{
		Name:          skill.Name,
		Description:   skill.Description,
		License:       skill.License,
		Compatibility: skill.Compatibility,
		Metadata:      skill.Metadata,
		AllowedTools:  skill.AllowedTools,
		DirectoryPath: skill.OriginalPath,
		IsActive:      sm.active[skillName],
	}, nil
}

// ValidateSkills checks all available skills for validity.
// Returns validation errors for any skills that fail validation.
func (sm *LocalSkillManager) ValidateSkills(_ context.Context) (map[string]error, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	validationErrors := make(map[string]error)
	for name, skill := range sm.skills {
		if err := skill.Validate(); err != nil {
			validationErrors[name] = err
		}
	}

	return validationErrors, nil
}

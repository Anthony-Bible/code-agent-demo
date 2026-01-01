// Package skill provides an implementation of the domain SkillManager port.
// It follows hexagonal architecture principles by providing infrastructure-level
// skill discovery and management operations for the AI coding agent.
//
// Skills are discovered from multiple directories in priority order:
//   - ./skills (project root, highest priority)
//   - ./.claude/skills (project .claude directory)
//   - ~/.claude/skills (user global, lowest priority)
//
// When the same skill name exists in multiple directories, the highest priority
// directory wins. Each skill is represented by a directory containing a SKILL.md
// file with YAML frontmatter defining the skill's metadata.
//
// Example usage:
//
//	sm := skill.NewLocalSkillManager()
//	result, err := sm.DiscoverSkills(context.Background())
//	if err != nil {
//		log.Fatal(err)
//	}
//
//	fmt.Printf("Found %d skills from %d directories\n", result.TotalCount, len(result.SkillsDirs))
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
	ErrInvalidSkillName  = errors.New(
		"invalid skill name: must contain only lowercase letters, numbers, and hyphens",
	)
	ErrSkillNameEmpty        = errors.New("skill name cannot be empty")
	ErrSkillNameTooLong      = errors.New("skill name must be 64 characters or less")
	ErrSkillNameHyphen       = errors.New("skill name cannot start or end with a hyphen")
	ErrSkillNameConsecHyphen = errors.New("skill name cannot contain consecutive hyphens")
)

// validateSkillName validates a skill name to prevent path traversal attacks.
// Skill names must match the agentskills.io spec: 1-64 lowercase alphanumeric
// characters and hyphens, cannot start/end with hyphen or have consecutive hyphens.
func validateSkillName(name string) error {
	if name == "" {
		return ErrSkillNameEmpty
	}
	if len(name) > 64 {
		return ErrSkillNameTooLong
	}
	if name[0] == '-' || name[len(name)-1] == '-' {
		return ErrSkillNameHyphen
	}

	prevChar := byte(0)
	for i := range len(name) {
		c := name[i]
		if c == '-' && prevChar == '-' {
			return ErrSkillNameConsecHyphen
		}
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return ErrInvalidSkillName
		}
		prevChar = c
	}
	return nil
}

// DirConfig represents a directory to search for skills with its source type.
type DirConfig struct {
	Path       string
	SourceType entity.SkillSourceType
}

// LocalSkillManager implements the SkillManager port for managing local file system skills.
// It discovers skills from multiple directories, loads their metadata, and manages
// their activation state.
type LocalSkillManager struct {
	mu         sync.RWMutex
	skillsDirs []DirConfig              // Directories to search for skills in priority order
	skills     map[string]*entity.Skill // Discovered skills by name
	active     map[string]bool          // Active skills by name
}

// NewLocalSkillManager creates a new LocalSkillManager instance.
// Skills are discovered from multiple directories in priority order:
// ./skills, ./.claude/skills, and ~/.claude/skills.
func NewLocalSkillManager() port.SkillManager {
	skillsDirs := []DirConfig{
		{Path: "./skills", SourceType: entity.SkillSourceProject},
		{Path: "./.claude/skills", SourceType: entity.SkillSourceProjectClaude},
	}
	if homeDir, err := os.UserHomeDir(); err == nil && homeDir != "" {
		skillsDirs = append(skillsDirs, DirConfig{
			Path:       filepath.Join(homeDir, ".claude", "skills"),
			SourceType: entity.SkillSourceUser,
		})
	}
	return &LocalSkillManager{
		skillsDirs: skillsDirs,
		skills:     make(map[string]*entity.Skill),
		active:     make(map[string]bool),
	}
}

// NewLocalSkillManagerWithDirs creates a LocalSkillManager with custom directories.
// This is primarily for testing to avoid discovering skills from user's home directory.
func NewLocalSkillManagerWithDirs(dirs []DirConfig) port.SkillManager {
	return &LocalSkillManager{
		skillsDirs: dirs,
		skills:     make(map[string]*entity.Skill),
		active:     make(map[string]bool),
	}
}

// DiscoverSkills scans all configured skill directories for available skills.
// Directories are searched in priority order, and when a skill name exists in
// multiple directories, the highest priority version is used.
//
// Active skills (those in the active map) have their entities preserved
// to avoid breaking external references. Their metadata is updated if changed.
func (sm *LocalSkillManager) DiscoverSkills(_ context.Context) (*port.SkillDiscoveryResult, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.preserveActiveSkills()

	dirsToSearch := sm.getDirsToSearch()

	var discoveredSkills []port.SkillInfo
	var skillsDirs []string
	seenSkills := make(map[string]bool)
	activeCount := 0

	for _, dirConfig := range dirsToSearch {
		skillsDirs = append(skillsDirs, dirConfig.Path)
		skills := sm.discoverFromDirectory(dirConfig, seenSkills)
		for _, s := range skills {
			discoveredSkills = append(discoveredSkills, s)
			if s.IsActive {
				activeCount++
			}
		}
	}

	return &port.SkillDiscoveryResult{
		Skills:      discoveredSkills,
		SkillsDirs:  skillsDirs,
		TotalCount:  len(discoveredSkills),
		ActiveCount: activeCount,
	}, nil
}

// preserveActiveSkills retains only active skills in the skills map.
// This is called before re-discovery to ensure active skill references remain valid.
func (sm *LocalSkillManager) preserveActiveSkills() {
	preservedSkills := make(map[string]*entity.Skill)
	for name, skill := range sm.skills {
		if sm.active[name] {
			preservedSkills[name] = skill
		}
	}
	sm.skills = preservedSkills
}

// getDirsToSearch returns the list of directories to search for skills.
func (sm *LocalSkillManager) getDirsToSearch() []DirConfig {
	return sm.skillsDirs
}

// discoverFromDirectory scans a single directory for SKILL.md files.
// The seenSkills map tracks already-discovered skill names for deduplication.
// Returns skill info for each valid skill found that has not already been seen.
func (sm *LocalSkillManager) discoverFromDirectory(
	dirConfig DirConfig,
	seenSkills map[string]bool,
) []port.SkillInfo {
	var skills []port.SkillInfo

	info, err := os.Stat(dirConfig.Path)
	if err != nil || !info.IsDir() {
		return skills
	}

	_ = filepath.Walk(dirConfig.Path, func(path string, info os.FileInfo, _ error) error {
		if path == dirConfig.Path || info == nil {
			return nil
		}
		if info.Name() == "SKILL.md" && !info.IsDir() {
			if skillInfo := sm.processSkillFileWithSource(path, dirConfig.SourceType, seenSkills); skillInfo != nil {
				skills = append(skills, *skillInfo)
			}
		}
		return nil
	})

	return skills
}

// findSkillPath searches all configured directories for a skill's SKILL.md file.
// Returns the path to the first matching file found, or empty string if not found.
func (sm *LocalSkillManager) findSkillPath(skillName string) string {
	for _, dirConfig := range sm.getDirsToSearch() {
		skillPath := filepath.Join(dirConfig.Path, skillName, "SKILL.md")
		if _, err := os.Stat(skillPath); err == nil {
			return skillPath
		}
	}
	return ""
}

// processSkillFileWithSource processes a SKILL.md file with source type and deduplication.
func (sm *LocalSkillManager) processSkillFileWithSource(
	path string,
	sourceType entity.SkillSourceType,
	seenSkills map[string]bool,
) *port.SkillInfo {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}

	skill, parseErr := entity.ParseSkillMetadataFromYAML(string(content))
	if parseErr != nil {
		return nil
	}

	if err := skill.Validate(); err != nil {
		return nil
	}

	dirName := filepath.Base(filepath.Dir(path))
	if err := skill.ValidateDirectoryName(dirName); err != nil {
		return nil
	}

	// Skip if already seen (higher priority directory already discovered this skill)
	if seenSkills[skill.Name] {
		return nil
	}
	seenSkills[skill.Name] = true

	dirPath := filepath.Dir(path)
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		absPath = dirPath
	}
	skill.ScriptPath = absPath
	skill.OriginalPath = dirPath
	skill.SourceType = sourceType

	sm.skills[skill.Name] = skill

	info := sm.skillToInfo(skill)
	return &info
}

// LoadSkillMetadata loads the metadata for a specific skill from its SKILL.md file.
// The skillName should match the skill directory name.
// Returns the skill entity with all parsed metadata.
// If the skill was discovered with ParseSkillMetadataFromYAML (progressive disclosure),
// this function will load the full content on-demand.
func (sm *LocalSkillManager) LoadSkillMetadata(_ context.Context, skillName string) (*entity.Skill, error) {
	// Validate skillName to prevent path traversal attacks
	// Skill names must match the agentskills.io spec: lowercase alphanumeric and hyphens only
	if err := validateSkillName(skillName); err != nil {
		return nil, err
	}

	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Check if skill is already loaded
	skill, exists := sm.skills[skillName]
	if exists && skill.RawContent != "" {
		// Skill exists and has full content loaded
		return skill, nil
	}

	// If skill exists but has no content, use its OriginalPath
	// Otherwise, search through directories
	var skillPath string
	if exists && skill.OriginalPath != "" {
		skillPath = filepath.Join(skill.OriginalPath, "SKILL.md")
	} else {
		skillPath = sm.findSkillPath(skillName)
	}

	if skillPath == "" {
		return nil, ErrSkillFileNotFound
	}

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
			availableSkills = append(availableSkills, sm.skillToInfo(skill))
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

	info := sm.skillToInfo(skill)
	return &info, nil
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

// skillToInfo converts an entity.Skill to a port.SkillInfo, including the active state.
func (sm *LocalSkillManager) skillToInfo(skill *entity.Skill) port.SkillInfo {
	return port.SkillInfo{
		Name:          skill.Name,
		Description:   skill.Description,
		License:       skill.License,
		Compatibility: skill.Compatibility,
		Metadata:      skill.Metadata,
		AllowedTools:  skill.AllowedTools,
		DirectoryPath: skill.OriginalPath,
		IsActive:      sm.active[skill.Name],
		SourceType:    skill.SourceType,
	}
}

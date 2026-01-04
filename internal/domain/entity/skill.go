package entity

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// SkillMetadata contains optional metadata for skills as defined in the agentskills.io specification.
type SkillMetadata struct {
	License       string            `yaml:"license,omitempty"`       // License for the skill
	Compatibility string            `yaml:"compatibility,omitempty"` // Compatibility information
	Metadata      map[string]string `yaml:"metadata,omitempty"`      // Additional metadata as key-value pairs
}

// SkillSourceType indicates where a skill was discovered from.
type SkillSourceType string

const (
	// SkillSourceUser indicates a skill from ~/.claude/skills (user global).
	SkillSourceUser SkillSourceType = "user"
	// SkillSourceProjectClaude indicates a skill from ./.claude/skills (project .claude directory).
	SkillSourceProjectClaude SkillSourceType = "project-claude"
	// SkillSourceProject indicates a skill from ./skills (project root, highest priority).
	SkillSourceProject SkillSourceType = "project"
)

// Skill represents an agent skill from the agentskills.io specification.
// Skills are directories with SKILL.md files containing YAML frontmatter.
type Skill struct {
	Name           string            `yaml:"name"`                    // Required: human-readable skill name
	Description    string            `yaml:"description"`             // Required: what the skill does
	License        string            `yaml:"license,omitempty"`       // Optional: license
	Compatibility  string            `yaml:"compatibility,omitempty"` // Optional: compatibility info
	Metadata       map[string]string `yaml:"metadata,omitempty"`      // Optional: additional metadata
	AllowedTools   []string          `yaml:"allowed-tools,omitempty"` // Optional: space-delimited list of tools
	ScriptPath     string            `yaml:"-"`                       // Absolute path to skill directory
	OriginalPath   string            `yaml:"-"`                       // Original path (relative or absolute)
	RawFrontmatter string            `yaml:"-"`                       // Raw YAML frontmatter
	RawContent     string            `yaml:"-"`                       // Content after frontmatter
	SourceType     SkillSourceType   `yaml:"-"`                       // Where the skill was discovered from
}

// UnmarshalYAML implements custom YAML unmarshaling to handle allowed-tools as either a string or slice.
func (s *Skill) UnmarshalYAML(value *yaml.Node) error {
	// First try to unmarshal into a map to handle special cases
	var raw map[string]interface{}
	if err := value.Decode(&raw); err != nil {
		return err
	}

	s.parseStringFields(raw)
	s.parseMetadata(raw)
	s.parseAllowedTools(raw)

	return nil
}

func (s *Skill) parseStringFields(raw map[string]interface{}) {
	if v, ok := raw["name"].(string); ok {
		s.Name = v
	}
	if v, ok := raw["description"].(string); ok {
		s.Description = v
	}
	if v, ok := raw["license"].(string); ok {
		s.License = v
	}
	if v, ok := raw["compatibility"].(string); ok {
		s.Compatibility = v
	}
}

func (s *Skill) parseMetadata(raw map[string]interface{}) {
	if v, ok := raw["metadata"].(map[string]interface{}); ok {
		s.Metadata = make(map[string]string)
		for key, val := range v {
			if str, ok := val.(string); ok {
				s.Metadata[key] = str
			}
		}
	}
}

func (s *Skill) parseAllowedTools(raw map[string]interface{}) {
	v, ok := raw["allowed-tools"]
	if !ok {
		return
	}

	switch tools := v.(type) {
	case string:
		if tools != "" {
			s.AllowedTools = strings.Fields(tools)
		}
	case []interface{}:
		s.AllowedTools = make([]string, 0, len(tools))
		for _, tool := range tools {
			if str, ok := tool.(string); ok {
				s.AllowedTools = append(s.AllowedTools, str)
			}
		}
	case []string:
		s.AllowedTools = tools
	}
}

// SkillMetadataEntity represents the complete metadata for a skill.
type SkillMetadataEntity struct {
	Name          string
	Description   string
	License       string
	Compatibility string
	Metadata      map[string]string
	AllowedTools  []string
}

// Validate checks if the skill has valid required fields.
// Returns error if name or description is empty or if name doesn't match the spec.
// Per agentskills.io spec: name must be 1-64 lowercase alphanumeric characters
// and hyphens, cannot start/end with hyphen or contain consecutive hyphens.
func (s *Skill) Validate() error {
	if s.Name == "" {
		return errors.New("skill name cannot be empty")
	}
	if len(s.Name) > 64 {
		return errors.New("skill name must be 64 characters or less")
	}

	// Validate name format: lowercase alphanumeric and hyphens only
	// Cannot start/end with hyphen, cannot have consecutive hyphens
	if s.Name[0] == '-' || s.Name[len(s.Name)-1] == '-' {
		return errors.New("skill name cannot start or end with a hyphen")
	}

	prevChar := byte(0)
	for i := range len(s.Name) {
		c := s.Name[i]
		// Check for consecutive hyphens
		if c == '-' && prevChar == '-' {
			return errors.New("skill name cannot contain consecutive hyphens")
		}
		// Check character is lowercase letter, digit, or hyphen
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return errors.New("skill name must contain only lowercase letters, numbers, and hyphens")
		}
		prevChar = c
	}

	if s.Description == "" {
		return errors.New("skill description cannot be empty")
	}
	if len(s.Description) > 1024 {
		return errors.New("skill description must be 1024 characters or less")
	}

	return nil
}

// ValidateDirectoryName checks if the skill name matches the directory name.
// Per agentskills.io spec, the skill name must match the parent directory name.
// This is called during skill discovery to ensure spec compliance.
func (s *Skill) ValidateDirectoryName(dirName string) error {
	if s.Name != dirName {
		return fmt.Errorf("skill name '%s' must match directory name '%s'", s.Name, dirName)
	}
	return nil
}

// ParseSkillFromYAML parses a skill from YAML frontmatter format.
// The input should be a string with YAML frontmatter between --- markers.
// Example:
// ---
// name: skill-name
// description: A description
// ---
// Content here.
func ParseSkillFromYAML(content string) (*Skill, error) {
	// Extract frontmatter using shared helper
	frontmatter, rawContent, err := extractFrontmatter(content)
	if err != nil {
		return nil, err
	}

	// Parse YAML
	var skill Skill
	if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
		return nil, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	// Store raw values
	skill.RawFrontmatter = frontmatter
	skill.RawContent = rawContent

	// Validate required fields
	if skill.Name == "" {
		return nil, errors.New("skill name is required")
	}
	if skill.Description == "" {
		return nil, errors.New("skill description is required")
	}

	return &skill, nil
}

// ParseSkillMetadataFromYAML parses only the metadata from YAML frontmatter format.
// This is used during skill discovery to efficiently load only metadata without the full content.
// The input should be a string with YAML frontmatter between --- markers.
// Unlike ParseSkillFromYAML, this function does NOT extract or store RawContent.
// Example:
// ---
// name: skill-name
// description: A description
// ---
// Content here (ignored).
func ParseSkillMetadataFromYAML(content string) (*Skill, error) {
	// Extract frontmatter using shared helper (ignoring remaining content)
	frontmatter, _, err := extractFrontmatter(content)
	if err != nil {
		return nil, err
	}

	// Parse YAML
	var skill Skill
	if err := yaml.Unmarshal([]byte(frontmatter), &skill); err != nil {
		return nil, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	// Store raw frontmatter but NOT raw content (progressive disclosure)
	skill.RawFrontmatter = frontmatter
	// RawContent is intentionally left empty

	// Validate required fields
	if skill.Name == "" {
		return nil, errors.New("skill name is required")
	}
	if skill.Description == "" {
		return nil, errors.New("skill description is required")
	}

	return &skill, nil
}

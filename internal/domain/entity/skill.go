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
}

// UnmarshalYAML implements custom YAML unmarshaling to handle allowed-tools as either a string or slice.
func (s *Skill) UnmarshalYAML(value *yaml.Node) error {
	// First try to unmarshal into a map to handle special cases
	var raw map[string]interface{}
	if err := value.Decode(&raw); err != nil {
		return err
	}

	// Handle basic string fields
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

	// Handle metadata map
	if v, ok := raw["metadata"].(map[string]interface{}); ok {
		s.Metadata = make(map[string]string)
		for key, val := range v {
			if str, ok := val.(string); ok {
				s.Metadata[key] = str
			}
		}
	}

	// Handle allowed-tools - can be string or slice
	if v, ok := raw["allowed-tools"]; ok {
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

	return nil
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
// Returns error if name or description is empty.
func (s *Skill) Validate() error {
	if s.Name == "" {
		return errors.New("skill name cannot be empty")
	}
	if s.Description == "" {
		return errors.New("skill description cannot be empty")
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
	// Find the frontmatter boundaries
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return nil, errors.New("invalid YAML frontmatter: missing opening ---")
	}

	// Find the closing ---
	firstLineEnd := strings.Index(content[3:], "\n---")
	if firstLineEnd == -1 {
		// Try to find it at the start of a line without the preceding newline
		firstLineEnd = strings.Index(content, "\n---")
		if firstLineEnd == -1 {
			return nil, errors.New("invalid YAML frontmatter: missing closing ---")
		}
	}

	// Get the frontmatter part
	frontmatterEnd := firstLineEnd + 4
	frontmatter := content[:frontmatterEnd]

	// Get the content after frontmatter
	rawContent := strings.TrimSpace(content[frontmatterEnd+3:])

	// Remove the opening and closing --- from frontmatter
	frontmatter = strings.TrimPrefix(frontmatter, "---")
	frontmatter = strings.TrimSuffix(frontmatter, "\n---")
	frontmatter = strings.TrimSpace(frontmatter)

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

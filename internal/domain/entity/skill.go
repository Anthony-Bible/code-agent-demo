package entity

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

// SkillMetadataEntity represents the complete metadata for a skill.
type SkillMetadataEntity struct {
	Name          string
	Description   string
	License       string
	Compatibility string
	Metadata      map[string]string
	AllowedTools  []string
}

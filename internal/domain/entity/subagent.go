package entity

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// SubagentSourceType indicates where a subagent was discovered from.
type SubagentSourceType string

const (
	// SubagentSourceProject indicates a subagent from ./subagents (project root, highest priority).
	SubagentSourceProject SubagentSourceType = "project"
	// SubagentSourceProjectClaude indicates a subagent from ./.claude/subagents (project .claude directory).
	SubagentSourceProjectClaude SubagentSourceType = "project-claude"
	// SubagentSourceUser indicates a subagent from ~/.claude/subagents (user global).
	SubagentSourceUser SubagentSourceType = "user"
	// SubagentSourceProgrammatic indicates a subagent created programmatically.
	SubagentSourceProgrammatic SubagentSourceType = "programmatic"
)

// SubagentModel represents the AI model to use for a subagent.
type SubagentModel string

const (
	// ModelInherit indicates the subagent should inherit the parent model.
	ModelInherit SubagentModel = "inherit"
	// ModelHaiku indicates the Claude Haiku model.
	ModelHaiku SubagentModel = "haiku"
	// ModelSonnet indicates the Claude Sonnet model.
	ModelSonnet SubagentModel = "sonnet"
	// ModelOpus indicates the Claude Opus model.
	ModelOpus SubagentModel = "opus"
)

// Subagent represents an agent with a specialized system prompt.
type Subagent struct {
	Name           string             `yaml:"name"`                    // Required: subagent name
	Description    string             `yaml:"description"`             // Required: what the subagent does
	Model          string             `yaml:"model,omitempty"`         // Optional: model to use
	MaxActions     int                `yaml:"max_actions,omitempty"`   // Optional: maximum actions
	AllowedTools   []string           `yaml:"allowed-tools,omitempty"` // Optional: allowed tools
	ScriptPath     string             `yaml:"-"`                       // Absolute path to subagent directory
	OriginalPath   string             `yaml:"-"`                       // Original path (relative or absolute)
	RawFrontmatter string             `yaml:"-"`                       // Raw YAML frontmatter
	RawContent     string             `yaml:"-"`                       // Content after frontmatter (system prompt)
	SourceType     SubagentSourceType `yaml:"-"`                       // Where the subagent was discovered from
}

// UnmarshalYAML implements custom YAML unmarshaling to handle allowed-tools as either a string or slice.
func (s *Subagent) UnmarshalYAML(value *yaml.Node) error {
	// First try to unmarshal into a map to handle special cases
	var raw map[string]interface{}
	if err := value.Decode(&raw); err != nil {
		return err
	}

	s.parseStringFields(raw)
	s.parseIntFields(raw)
	s.parseAllowedTools(raw)

	return nil
}

func (s *Subagent) parseStringFields(raw map[string]interface{}) {
	if v, ok := raw["name"].(string); ok {
		s.Name = v
	}
	if v, ok := raw["description"].(string); ok {
		s.Description = v
	}
	if v, ok := raw["model"].(string); ok {
		s.Model = v
	}
}

func (s *Subagent) parseIntFields(raw map[string]interface{}) {
	if v, ok := raw["max_actions"].(int); ok {
		s.MaxActions = v
	}
}

func (s *Subagent) parseAllowedTools(raw map[string]interface{}) {
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

// Validate checks if the subagent has valid required fields.
func (s *Subagent) Validate() error {
	if s.Name == "" {
		return errors.New("subagent name cannot be empty")
	}
	if len(s.Name) > 64 {
		return errors.New("subagent name must be 64 characters or less")
	}

	// Validate name format: lowercase alphanumeric and hyphens only
	// Cannot start/end with hyphen, cannot have consecutive hyphens
	if s.Name[0] == '-' || s.Name[len(s.Name)-1] == '-' {
		return errors.New("subagent name cannot start or end with a hyphen")
	}

	prevChar := byte(0)
	for i := range len(s.Name) {
		c := s.Name[i]
		// Check for consecutive hyphens
		if c == '-' && prevChar == '-' {
			return errors.New("subagent name cannot contain consecutive hyphens")
		}
		// Check character is lowercase letter, digit, or hyphen
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '-' {
			return errors.New("subagent name must contain only lowercase letters, numbers, and hyphens")
		}
		prevChar = c
	}

	if s.Description == "" {
		return errors.New("subagent description cannot be empty")
	}

	// Validate model if specified
	if s.Model != "" {
		validModels := map[string]bool{
			"inherit": true,
			"haiku":   true,
			"sonnet":  true,
			"opus":    true,
		}
		if !validModels[s.Model] {
			return errors.New("subagent model must be one of: inherit, haiku, sonnet, opus")
		}
	}

	return nil
}

// ParseSubagentFromYAML parses a subagent from YAML frontmatter format.
func ParseSubagentFromYAML(content string) (*Subagent, error) {
	// Extract frontmatter using shared helper
	frontmatter, rawContent, err := extractFrontmatter(content)
	if err != nil {
		return nil, err
	}

	// Parse YAML
	var subagent Subagent
	if err := yaml.Unmarshal([]byte(frontmatter), &subagent); err != nil {
		return nil, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	// Store raw values
	subagent.RawFrontmatter = frontmatter
	subagent.RawContent = rawContent

	// Validate required fields
	if subagent.Name == "" {
		return nil, errors.New("subagent name is required")
	}
	if subagent.Description == "" {
		return nil, errors.New("subagent description is required")
	}

	return &subagent, nil
}

// ParseSubagentMetadataFromYAML parses only the metadata from YAML frontmatter format.
func ParseSubagentMetadataFromYAML(content string) (*Subagent, error) {
	// Extract frontmatter using shared helper (ignoring remaining content)
	frontmatter, _, err := extractFrontmatter(content)
	if err != nil {
		return nil, err
	}

	// Parse YAML
	var subagent Subagent
	if err := yaml.Unmarshal([]byte(frontmatter), &subagent); err != nil {
		return nil, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
	}

	// Store raw frontmatter but NOT raw content (progressive disclosure)
	subagent.RawFrontmatter = frontmatter
	// RawContent is intentionally left empty

	// Validate required fields
	if subagent.Name == "" {
		return nil, errors.New("subagent name is required")
	}
	if subagent.Description == "" {
		return nil, errors.New("subagent description is required")
	}

	return &subagent, nil
}

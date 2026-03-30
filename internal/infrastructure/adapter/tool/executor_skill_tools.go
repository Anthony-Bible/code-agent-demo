package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/anthony-bible/code-agent-demo/internal/domain/entity"
	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// activateSkillInput represents the input for the activate_skill tool.
type activateSkillInput struct {
	SkillName string `json:"skill_name"`
}

// deactivateSkillInput represents the input for the deactivate_skill tool.
type deactivateSkillInput struct {
	SkillName string `json:"skill_name"`
}

// registerSkillTools registers skill-related tools.
func (a *ExecutorAdapter) registerSkillTools() {
	// Register activate_skill tool (will be rebuilt with dynamic description if SetSkillManager is called)
	activateSkillTool := entity.Tool{
		ID:          "activate_skill",
		Name:        "activate_skill",
		Description: "Activates a skill by name and returns its full content. Use this to load detailed instructions for specific capabilities.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"skill_name": map[string]interface{}{
					"type":        "string",
					"description": "The name of the skill to activate",
				},
			},
			"required": []string{"skill_name"},
		},
		RequiredFields: []string{"skill_name"},
	}
	a.tools[activateSkillTool.Name] = activateSkillTool

	// Register deactivate_skill tool
	deactivateSkillTool := entity.Tool{
		ID:          "deactivate_skill",
		Name:        "deactivate_skill",
		Description: "Deactivates a previously activated skill, removing its tool restrictions. Use this when you no longer need a skill's capabilities.",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"skill_name": map[string]interface{}{
					"type":        "string",
					"description": "Name of the skill to deactivate",
				},
			},
			"required": []string{"skill_name"},
		},
		RequiredFields: []string{"skill_name"},
	}
	a.tools[deactivateSkillTool.Name] = deactivateSkillTool
}

// rebuildActivateSkillToolLocked updates the activate_skill tool definition.
// REQUIRES: a.mu must be held by the caller.
func (a *ExecutorAdapter) rebuildActivateSkillToolLocked() {
	// Build description with available skills
	description := a.buildActivateSkillDescription()

	// Update the activate_skill tool with new description
	activateSkillTool := entity.Tool{
		ID:          "activate_skill",
		Name:        "activate_skill",
		Description: description,
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"skill_name": map[string]interface{}{
					"type":        "string",
					"description": "The name of the skill to activate",
				},
			},
			"required": []string{"skill_name"},
		},
		RequiredFields: []string{"skill_name"},
	}
	a.tools[activateSkillTool.Name] = activateSkillTool
}

// buildActivateSkillDescription builds the description for the activate_skill tool.
// If a skill manager is available, it includes available skills in the description.
func (a *ExecutorAdapter) buildActivateSkillDescription() string {
	baseDescription := "Execute a skill within the main conversation\n\n" +
		"When users ask you to perform tasks, check if any of the available skills below can help complete the task more effectively. " +
		"Skills provide specialized capabilities and domain knowledge.\n\n" +
		"Use this tool to load the full content of a skill when its capabilities are needed for the task at hand."

	// If no skill manager, return base description
	if a.skillManager == nil {
		return baseDescription
	}

	// Try to discover skills with timeout to prevent blocking indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	skills, err := a.skillManager.DiscoverSkills(ctx)
	if err != nil {
		a.logger.Warn("failed to discover skills for tool description", "error", err)
		return baseDescription
	}
	if len(skills.Skills) == 0 {
		return baseDescription
	}

	// Build skills section following the example format
	var sb strings.Builder
	sb.WriteString(baseDescription)
	sb.WriteString("\n\n## Available Skills\n\n")

	for _, skill := range skills.Skills {
		// Include source type to help AI understand where skill scripts are located
		sourceLabel := ""
		switch skill.SourceType {
		case entity.SkillSourceUser:
			sourceLabel = " (user)"
		case entity.SkillSourceProject:
			sourceLabel = " (project)"
		case entity.SkillSourceProjectClaude:
			sourceLabel = " (project-claude)"
		}
		fmt.Fprintf(&sb, "- **%s**%s: %s\n", skill.Name, sourceLabel, skill.Description)
	}

	sb.WriteString("\nActivate a skill by providing its name to load detailed instructions and capabilities.")
	sb.WriteString("\n\nSkill source types indicate where scripts are located:")
	sb.WriteString("\n- (project): ./skills/skill-name/ - highest priority")
	sb.WriteString("\n- (project-claude): ./.claude/skills/skill-name/")
	sb.WriteString("\n- (user): ~/.claude/skills/skill-name/ - user global skills")

	return sb.String()
}

// executeActivateSkill activates a skill by name and returns its full content.
// This allows the AI to load detailed instructions for specific capabilities.
// If no skill manager is set, returns an error.
func (a *ExecutorAdapter) executeActivateSkill(ctx context.Context, input json.RawMessage) (string, error) {
	// Check if skill manager is available
	if a.skillManager == nil {
		return "", errors.New("skill manager not available")
	}

	var in activateSkillInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to unmarshal activate_skill input: %w", err)
	}

	if in.SkillName == "" {
		return "", errors.New("skill_name parameter is required but was empty")
	}

	// Try to load the skill metadata first (avoids redundant filesystem scans).
	// If the skill is not found, refresh the discovered skills once and retry.
	skill, err := a.skillManager.LoadSkillMetadata(ctx, in.SkillName)
	if err != nil {
		// Attempt to refresh the skills list once
		if _, discoverErr := a.skillManager.DiscoverSkills(ctx); discoverErr != nil {
			return "", fmt.Errorf("failed to discover skills: %w", discoverErr)
		}

		// Retry loading the skill metadata after refreshing
		skill, err = a.skillManager.LoadSkillMetadata(ctx, in.SkillName)
		if err != nil {
			return "", fmt.Errorf("failed to load skill '%s': %w", in.SkillName, err)
		}
	}

	// Verify we have the full content (safety check for progressive disclosure)
	if skill.RawContent == "" {
		return "", fmt.Errorf("skill '%s' content not loaded", in.SkillName)
	}

	// Build result with frontmatter and content
	var result strings.Builder
	fmt.Fprintf(&result, "---\nname: %s\ndescription: %s", skill.Name, skill.Description)
	if skill.License != "" {
		fmt.Fprintf(&result, "\nlicense: %s", skill.License)
	}
	if skill.Compatibility != "" {
		fmt.Fprintf(&result, "\ncompatibility: %s", skill.Compatibility)
	}
	if len(skill.AllowedTools) > 0 {
		fmt.Fprintf(&result, "\nallowed-tools: %s", strings.Join(skill.AllowedTools, " "))
	}
	// Include source_type to indicate if skill is user, project, or project-claude
	fmt.Fprintf(&result, "\nsource_type: %s", skill.SourceType)
	// Include directory_path for script execution context
	fmt.Fprintf(&result, "\ndirectory_path: %s", skill.OriginalPath)
	if len(skill.Metadata) > 0 {
		result.WriteString("\nmetadata:")
		for key, value := range skill.Metadata {
			fmt.Fprintf(&result, "\n  %s: %s", key, value)
		}
	}
	result.WriteString("\n---\n")
	result.WriteString(skill.RawContent)

	// Track active skill for allowed-tools enforcement if callback is set
	if a.skillActivationCallback != nil {
		if sessionID, ok := port.SessionIDFromContext(ctx); ok {
			_ = a.skillActivationCallback(sessionID, *skill)
		}
	}

	return result.String(), nil
}

// executeDeactivateSkill deactivates a skill by name, removing its tool restrictions.
// This allows the AI to remove skills when they're no longer needed.
// If no skill manager is set, returns an error.
func (a *ExecutorAdapter) executeDeactivateSkill(ctx context.Context, input json.RawMessage) (string, error) {
	var in deactivateSkillInput
	if err := json.Unmarshal(input, &in); err != nil {
		return "", fmt.Errorf("failed to unmarshal deactivate_skill input: %w", err)
	}

	if in.SkillName == "" {
		return "", errors.New("skill_name parameter is required but was empty")
	}

	// Get sessionID from context
	sessionID, ok := port.SessionIDFromContext(ctx)
	if !ok {
		return "", errors.New("session ID not found in context")
	}

	// Call deactivation callback if set.
	// When no callback is registered (e.g., in subagent contexts), deactivation
	// reports success intentionally — this is symmetric with the idempotent behavior
	// when the skill is not active.
	if a.skillDeactivationCallback != nil {
		if err := a.skillDeactivationCallback(sessionID, in.SkillName); err != nil {
			return "", fmt.Errorf("failed to deactivate skill '%s': %w", in.SkillName, err)
		}
	}

	return fmt.Sprintf("Skill '%s' deactivated successfully", in.SkillName), nil
}

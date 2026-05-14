// Package ai — system_prompt.go holds the shared system-prompt construction
// logic used by every AIProvider adapter in this package. Adapters differ in
// SDK and wire shape but must produce IDENTICAL prompt text so behavior is
// stable across provider selection.
package ai

import (
	"context"
	"fmt"
	"strings"

	"github.com/anthony-bible/code-agent-demo/internal/domain/port"
)

// basePrompt is the default assistant prompt when no SystemPrompt override
// and no PlanMode are set.
const basePrompt = "You are an AI assistant that helps users with code editing and explanations. Use the available tools when necessary to provide accurate and helpful responses."

// ResolveSystemPrompt applies the priority order shared by all adapters:
//  1. opts.SystemPrompt — explicit override
//  2. opts.PlanMode — when plan mode is enabled
//  3. base prompt, optionally augmented with the subagent catalog
//
// Exported so adapters in sibling packages (e.g. a future Genkit adapter)
// can reuse the same construction logic and stay prompt-identical with the
// Anthropic adapter.
func ResolveSystemPrompt(ctx context.Context, opts port.AIRequestOptions, subagentManager port.SubagentManager) (string, error) {
	if opts.SystemPrompt != "" {
		return opts.SystemPrompt, nil
	}
	if opts.PlanMode != nil && opts.PlanMode.Enabled {
		return buildPlanModePrompt(*opts.PlanMode), nil
	}
	return buildBasePromptWithAgents(ctx, subagentManager)
}

// buildPlanModePrompt constructs the specialized plan mode system prompt.
func buildPlanModePrompt(planInfo port.PlanModeInfo) string {
	return fmt.Sprintf(
		`You are an AI assistant in PLAN MODE. Your job is to explore the codebase and write an implementation plan before making changes.

## Your Role in Plan Mode

You should:
1. Use read_file and list_files to understand the existing code
2. Use read-only bash commands (e.g., git status, ls, find) to explore
3. Write your implementation plan to: %s

## How to Write Your Plan

Use the edit_file tool to write your plan to %s. Structure your plan as:

### Summary
Brief overview of what you're implementing

### Files to Modify
- path/to/file1.go - what changes are needed
- path/to/file2.go - what changes are needed

### Implementation Steps
1. First step
2. Second step
...

### Considerations
- Any trade-offs or decisions to highlight

## Important Rules

- You CAN use edit_file to write to %s - this is your plan file
- Other mutating tools (edit_file for other paths, destructive bash commands) will be blocked
- If you try to use a blocked tool, you'll receive a reminder to write to your plan file instead
- Focus on thorough exploration and detailed planning before implementation

## When You're Done

When your plan is complete, tell the user to exit plan mode with :mode normal to begin implementation.
`,
		planInfo.PlanPath, planInfo.PlanPath, planInfo.PlanPath,
	)
}

// buildBasePromptWithAgents returns the base prompt, appending a list of
// available specialized agents when the SubagentManager is non-nil and
// returns at least one agent.
func buildBasePromptWithAgents(ctx context.Context, subagentManager port.SubagentManager) (string, error) {
	if subagentManager == nil {
		return basePrompt, nil
	}
	agents, err := subagentManager.ListAgents(ctx)
	if err != nil {
		// Best-effort: the agent catalog is optional enrichment for the
		// system prompt, not required for correctness. A transient FS error
		// or missing agents/ directory must not fail the user's AI request.
		return basePrompt, nil //nolint:nilerr // intentional graceful degradation
	}
	if len(agents) == 0 {
		return basePrompt, nil
	}
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\nYou have access to the following specialized agents:\n")
	for _, agent := range agents {
		fmt.Fprintf(&sb, "- %s: %s\n", agent.Name, agent.Description)
	}
	return sb.String(), nil
}

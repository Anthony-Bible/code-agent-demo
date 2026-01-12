# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go-based AI coding agent using hexagonal (clean) architecture. Provides an interactive CLI chat with file manipulation tools, interfacing with the Anthropic API.

## Development Commands

```bash
# Build and run
go build -o code-editing-agent ./cmd/cli
./code-editing-agent chat

# Run directly
go run ./cmd/cli/main.go chat

# Testing
go test ./...                                    # All tests
go test ./internal/domain/entity -v              # Single package
go test ./internal/infrastructure/adapter/file -v -run TestLocalFileManager_DeleteFile  # Single test

# Code quality
go fmt ./...
go vet ./...
```

## Architecture

### Hexagonal Architecture (Ports & Adapters)

```
Presentation (cmd/cli/) → Application (internal/application/) → Domain (internal/domain/) ← Infrastructure (internal/infrastructure/)
```

**Domain Layer** (`internal/domain/`) - No external dependencies
- `entity/` - Core objects: `Conversation`, `Message`, `Tool`
- `port/` - Interface contracts: `AIProvider`, `ToolExecutor`, `FileManager`, `UserInterface`
- `service/` - Business logic: `ConversationService`, `ToolService`

**Application Layer** (`internal/application/`)
- `service/ChatService` - High-level orchestration
- `usecase/` - `MessageProcessUseCase`, `ToolExecutionUseCase`
- `dto/` - Data transfer objects between layers

**Infrastructure Layer** (`internal/infrastructure/`)
- `adapter/ai/anthropic_adapter.go` - Implements `AIProvider`
- `adapter/file/local_file_adapter.go` - Implements `FileManager` (with path traversal protection)
- `adapter/tool/tool_executor_adapter.go` - Implements `ToolExecutor` (bash, read_file, list_files, edit_file)
- `adapter/tool/planning_executor_adapter.go` - Decorator that wraps `ToolExecutor` for plan mode
- `adapter/ui/cli_adapter.go` - Implements `UserInterface`
- `config/container.go` - Dependency injection wiring
- `signal/interrupt_handler.go` - Double Ctrl+C exit handling

### Key Data Flows

**Chat Flow**: User Input → `ChatService.SendMessage()` → `ConversationService.ProcessAssistantResponse()` → `AIProvider.SendMessage()` → Tool execution if needed → Response

**Tool Execution**: AI requests tool → `ToolExecutionUseCase.ExecuteToolsInSession()` → `PlanningExecutorAdapter` (if plan mode enabled) → `ExecutorAdapter.ExecuteTool()` → Results fed back to AI

### Ports (Interfaces)

| Port | Purpose | Adapter |
|------|---------|---------|
| `AIProvider` | AI model communication | `AnthropicAdapter` |
| `FileManager` | Sandboxed file operations | `LocalFileManager` |
| `ToolExecutor` | Tool registry & execution | `ExecutorAdapter` (decorated by `PlanningExecutorAdapter`) |
| `UserInterface` | Terminal I/O | `CLIAdapter` |

## Adding New Tools

1. Register in `ExecutorAdapter.registerDefaultTools()` (`internal/infrastructure/adapter/tool/tool_executor_adapter.go`)
2. Implement in `ExecuteTool()` switch statement
3. Add tests

## Configuration

Environment variables with `AGENT_` prefix:
- `AGENT_MODEL` - AI model (default: `hf:zai-org/GLM-4.6`)
- `AGENT_MAX_TOKENS` - Response limit
- `AGENT_WORKING_DIR` - Base directory for file operations

## Testing Patterns

Table-driven tests throughout. Example pattern:
```go
tests := []struct {
    name    string
    input   Type
    want    Expected
    wantErr bool
}{...}
for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) { ... })
}
```

Mock implementations of ports for isolated testing - see `conversation_service_test.go`.

## Security Features

- **Path traversal prevention** in `LocalFileManager` - validates paths stay within baseDir
- **Dangerous command detection** in `ExecutorAdapter` - patterns like `rm -rf`, `dd`, etc. require confirmation
- **Input validation** at entity and DTO levels

## Agent Skills

This agent supports skills following the [agentskills.io](https://agentskills.io) specification.

### Skill Discovery Locations

Skills are discovered from three directories in priority order:

1. `./skills` (project root, **highest priority**)
2. `./.claude/skills` (project .claude directory)
3. `~/.claude/skills` (user global, **lowest priority**)

When the same skill name exists in multiple directories, the highest priority version is used.
Each discovered skill includes a `source_type` field indicating its origin ("project", "project-claude", or "user").

### Skill Directory Structure

Each skill directory follows the [agentskills.io](https://agentskills.io) specification:

```
skills/                    # or ~/.claude/skills/ or ./.claude/skills/
├── skill-name/
│   └── SKILL.md          # Required
├── other-skill/
│   ├── SKILL.md          # Required
│   ├── scripts/          # Optional - executable code
│   ├── references/       # Optional - documentation
│   └── assets/           # Optional - static resources
└── README.md             # Optional
```

### SKILL.md Format

Each skill must contain a `SKILL.md` file with YAML frontmatter:

```yaml
---
name: skill-name
description: A description of what this skill does and when to use it.
license: MIT              # Optional
compatibility: Go 1.22+   # Optional
metadata:
  key: value              # Optional map
allowed-tools: read_file list_files  # Optional space-delimited list
---

# Skill Content

Detailed instructions, patterns, and examples for using the skill.
```

### Required Frontmatter Fields

- `name`: Skill name (lowercase alphanumeric and hyphens, max 64 chars)
- `description`: What the skill does and when to use it (max 1024 chars)

### Optional Frontmatter Fields

- `license`: License name or reference
- `compatibility`: Environment requirements
- `metadata`: Additional key-value pairs
- `allowed-tools`: Pre-approved tools for this skill

### How Skills Work

1. **Discovery**: Skills are automatically discovered from `./skills` at startup
2. **Metadata**: Skill name, description, and source type are shown in the `activate_skill` tool description
3. **Activation**: Use the `activate_skill` tool to load full skill content on demand
4. **Scripts**: Skills can reference scripts in a `scripts/` subdirectory (executed via bash tool)

### Skill Activation

When a skill is activated via the `activate_skill` tool, it returns the skill content with additional metadata:

- `source_type`: Indicates where the skill is located ("project", "project-claude", or "user")
- `directory_path`: The path to the skill directory for script execution

Example activation output:
```yaml
---
name: my-skill
description: A skill description
source_type: project
directory_path: skills/my-skill
---
# Skill Content

To run a script: `bash {directory_path}/scripts/setup.sh`
```

The `source_type` helps the AI understand the correct context:
- `project` - Skills from `./skills/` (highest priority)
- `project-claude` - Skills from `./.claude/skills/`
- `user` - Skills from `~/.claude/skills/` (user global)

This is crucial when skills reference scripts, as the AI needs to know the full path to execute them correctly.

### Adding a New Skill

1. Create a directory under `./skills/skill-name/`
2. Create `SKILL.md` following the format above
3. (Optional) Add `scripts/`, `references/`, `assets/` subdirectories
4. The skill will be automatically discovered at next startup

### Example Skill

See `skills/test-skill/SKILL.md` and `skills/code-review/SKILL.md` for examples.

## Subagent System

This agent supports spawning specialized AI subagents to handle delegated tasks in isolated conversation contexts. Subagents are useful for complex workflows requiring specialized expertise or parallel execution.

### What are Subagents?

Subagents are isolated AI agents that:
- Run in their own conversation session (separate from the main agent)
- Have custom system prompts defining their specialized role
- Can have restricted tool access for safety
- Can use different AI models (haiku for speed, sonnet for quality)
- Execute independently and return results to the main agent

### Subagent Discovery Locations

Subagents are discovered from three directories in priority order:

1. `./agents` (project root, **highest priority**)
2. `./.claude/agents` (project .claude directory)
3. `~/.claude/agents` (user global, **lowest priority**)

When the same agent name exists in multiple directories, the highest priority version is used.

### Agent Directory Structure

Each agent directory contains an `AGENT.md` file:

```
agents/                    # or ~/.claude/agents/ or ./.claude/agents/
├── code-reviewer/
│   └── AGENT.md          # Required
├── test-writer/
│   └── AGENT.md          # Required
└── documentation-writer/
    └── AGENT.md          # Required
```

### AGENT.md Format

Each agent must contain an `AGENT.md` file with YAML frontmatter:

```yaml
---
name: code-reviewer
description: Expert code reviewer for security and best practices analysis
allowed_tools:
  - read_file
  - list_files
  - grep
model: sonnet
max_actions: 15
---

# Agent System Prompt

Detailed instructions for the agent's role, responsibilities, and behavior.
```

### Required Frontmatter Fields

- `name`: Agent name (lowercase alphanumeric and hyphens, max 64 chars)
- `description`: What the agent does and when to use it (max 1024 chars)

### Optional Frontmatter Fields

- `allowed_tools`: List of tools this agent can use (default: all tools)
  - Use to restrict agent capabilities for safety
  - Empty list `[]` blocks all tools
  - Omit field or use `null` to allow all tools
- `model`: AI model to use (`haiku`, `sonnet`, `opus`, `inherit`)
  - `haiku` - Fast, cost-effective for simple tasks
  - `sonnet` - Balanced quality and speed (recommended)
  - `opus` - Highest quality for complex reasoning
  - `inherit` - Use same model as parent agent (default)
- `max_actions`: Maximum tool calls before stopping (default: 20)
  - Prevents infinite loops in runaway agents
  - Recommended: 10-20 for most tasks

### Using Subagents

#### Method 1: Task Tool (Synchronous)

Use the `task` tool to spawn a subagent and wait for results:

```json
{
  "tool": "task",
  "input": {
    "agent_name": "code-reviewer",
    "prompt": "Review the authentication module for security issues"
  }
}
```

The task tool:
- Spawns the subagent in an isolated session
- Waits for completion
- Returns the subagent's output
- Cannot be called from within a subagent (prevents recursion)

#### Method 2: Programmatic (Advanced)

For parallel execution or async workflows, use the SubagentUseCase directly:

```go
// Synchronous spawn
result, err := subagentUseCase.SpawnSubagent(ctx, "code-reviewer", "Review auth.go")

// Asynchronous spawn (returns immediately)
handle, err := subagentUseCase.SpawnSubagentAsync(ctx, "test-writer", "Write tests for payment.go")
select {
case result := <-handle.Result:
    // Handle success
case err := <-handle.Error:
    // Handle error
}

// Parallel spawn (multiple agents concurrently)
requests := []*SubagentRequest{
    {AgentName: "code-reviewer", Prompt: "Review file1.go"},
    {AgentName: "test-writer", Prompt: "Write tests for file2.go"},
}
batchResult, _ := subagentUseCase.SpawnMultiple(ctx, requests)
```

### Example Agents

This repository includes three example agents in `./agents/`:

**code-reviewer** - Security and quality analysis
- Identifies vulnerabilities (SQL injection, XSS, etc.)
- Reviews code quality and best practices
- Provides actionable, prioritized feedback
- Tools: read_file, list_files, grep

**test-writer** - Comprehensive test creation
- Writes tests for happy paths and edge cases
- Uses table-driven test patterns
- Focuses on maintainability and coverage
- Tools: read_file, list_files, grep, write_file, edit_file

**documentation-writer** - Technical documentation
- Creates godoc comments and README files
- Documents APIs, configuration, and architecture
- Provides clear examples and explanations
- Tools: read_file, list_files, grep, write_file, edit_file

### Creating Custom Agents

1. Create a directory under `./agents/your-agent-name/`
2. Create `AGENT.md` with proper frontmatter and system prompt
3. Define the agent's role, responsibilities, and behavior
4. Specify allowed tools if restricting access
5. Test by spawning the agent with the task tool

### Best Practices

**Agent Design:**
- Give agents clear, focused responsibilities
- Write detailed system prompts explaining the agent's role
- Provide examples and guidelines in the agent's prompt
- Use tool restrictions to prevent dangerous operations

**Tool Selection:**
- Use `haiku` for simple, fast tasks (linting, formatting)
- Use `sonnet` for balanced quality (code review, testing)
- Use `opus` for complex reasoning (architecture design)
- Use `inherit` when agent should match parent capabilities

**Safety:**
- Set `max_actions` to prevent runaway agents (recommended: 10-20)
- Restrict tools with `allowed_tools` for sensitive operations
- Test agents in safe environments before production use
- Monitor agent behavior and adjust system prompts as needed

**Recursion Prevention:**
- Subagents cannot spawn other subagents (blocked by design)
- Task tool will error if called from within a subagent context
- This prevents infinite recursion and resource exhaustion

### Architecture

The subagent system uses clean architecture with clear separation:

**Domain Layer:**
- `entity.Subagent` - Agent metadata and configuration
- `port.SubagentManager` - Discovery and loading interface

**Infrastructure Layer:**
- `adapter/subagent.LocalSubagentManager` - File-based discovery

**Application Layer:**
- `usecase.SubagentRunner` - Isolated execution orchestration
- `usecase.SubagentUseCase` - High-level spawn operations

**Tool Integration:**
- `adapter/tool.ExecutorAdapter` - Task tool implementation
- Context-based recursion prevention

### Configuration

Subagent behavior is configured in the container:

```go
SubagentConfig{
    MaxActions:    20,              // Max tool calls per agent
    MaxDuration:   5 * time.Minute, // Timeout for execution
    MaxConcurrent: 5,               // Parallel agent limit
    AllowedTools:  nil,             // nil = allow all (can be overridden per agent)
}
```

## Mode Toggle Feature (Plan Mode)

The agent supports a "plan mode" where tool executions are written to `.agent/plans/` instead of being executed directly. This allows reviewing proposed changes before applying them.

### Enabling Plan Mode

In the CLI, use the `:mode` command:
- `:mode` or `:mode toggle` - Toggle between plan and normal mode
- `:mode plan` - Enable plan mode
- `:mode normal` - Disable plan mode

### Visual Indicators

When in plan mode:
- Assistant responses are prefixed with `[PLAN MODE]`
- Tools write JSON plans to `{workingDir}/.agent/plans/{sessionID}_{timestamp}.json`
- System message confirms mode status when toggled

### Plan File Format

Plans are written as JSON with the following structure:
```json
{
  "session_id": "...",
  "tool_name": "bash",
  "input": {"command": "ls -la"},
  "timestamp": "2024-01-20T15:30:01Z"
}
```

### Architecture

The `PlanningExecutorAdapter` decorates the base `ExecutorAdapter` using the decorator pattern:
- Checks mode state per session via `ConversationService.SetPlanMode()`
- In plan mode: writes tool execution plans to files instead of executing
- In normal mode: delegates execution to the wrapped executor
- Uses thread-safe `sessionModes` map for concurrent access

### Implementation Details

Key files:
- `internal/infrastructure/adapter/tool/planning_executor_adapter.go` - Decorator implementation
- `internal/domain/service/conversation_service.go` - Session mode state management
- `internal/application/service/chat_service.go` - Mode command handling
- `cmd/cli/cmd/chat.go` - CLI integration of `:mode` command
- `internal/infrastructure/config/container.go` - Decorator wiring

The decorator is wired in the container:
```go
baseExecutor := tool.NewExecutorAdapter(fileManager)
toolExecutor := tool.NewPlanningExecutorAdapter(baseExecutor, fileManager, cfg.WorkingDir)
```

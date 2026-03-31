# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

A Go-based AI coding agent using hexagonal (clean) architecture. Provides a webhook server for automated alert investigation with root cause analysis, subagent orchestration, and interfaces with the Anthropic API.

## Development Commands

```bash
# Build and run
go build -o code-agent-demo ./cmd/cli
./code-agent-demo serve                          # Webhook server for alerts

# Run directly
go run ./cmd/cli/main.go serve

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
- `entity/` - Core objects: `Conversation`, `Message`, `Tool`, `Investigation`, `Alert`, `RCAFinding`
- `port/` - Interface contracts (see Ports table below)
- `service/` - Business logic: `ConversationService`, `ToolService`, `RCAService`
- `safety/` - Command validation: whitelist/blacklist patterns, dangerous command detection

**Application Layer** (`internal/application/`)
- `service/InvestigationStore` - File-based investigation persistence
- `service/SafetyEnforcer` - Command safety validation
- `usecase/` - `BaseRunner`, `ToolExecutionUseCase`, `AlertHandler`, `AlertInvestigation`, `InvestigationRunner`, `EscalationHandler`, `SubagentRunner`, `SubagentUseCase`
- `dto/` - Data transfer objects between layers
- `config/` - Application-level config (e.g., `InvestigationConfig`)

**Infrastructure Layer** (`internal/infrastructure/`)
- `adapter/ai/` - `AnthropicAdapter` implements `AIProvider`
- `adapter/file/` - `LocalFileManager` implements `FileManager` (with path traversal protection)
- `adapter/tool/` - `ExecutorAdapter` implements `ToolExecutor` (split into per-domain files: bash, file, fetch, investigation, skill, subagent, plan/batch); `PlanningExecutorAdapter` decorator for plan mode
- `adapter/ui/` - `CLIAdapter` implements `UserInterface`
- `adapter/alert/` - Alert source adapters: `PrometheusSource`, `GCPMonitoringSource`, `SourceManager`, `SourceRegistry`
- `adapter/webhook/` - HTTP server for receiving alert webhooks
- `adapter/investigation/` - File-based investigation storage
- `adapter/subagent/` - `LocalSubagentManager` for file-based agent discovery
- `config/container.go` - Dependency injection wiring
- `logger/` - Structured logging implementation (wraps slog)
- `signal/` - Signal handling (Ctrl+C exit, SIGHUP reload)

### Logging Best Practices

**Contextual Fields with `With()`**:
- Use `logger.With()` at the start of an operation to attach scoped context (e.g., `investigation_id`, `alert_id`, `subagent_id`, `session_id`)
- Log all messages through this contextual logger - don't repeat the same fields in each call
- This makes logs more queryable and consistent for filtering/analysis

**Example patterns**:
- Investigations: `log := logger.With("investigation_id", id)` 
- Alerts: `log.With("alert_id", alert.ID(), "severity", alert.Severity())`
- Subagents: `log.With("subagent_id", id, "agent_name", agent.Name(), "session_id", sessionID)`

**Message content**:
- Keep messages human-readable and concise
- Put structured data in key-value pairs, not formatted strings
- Error messages: Always include the error object as a key-value pair (`"error", err`)

### Key Data Flows

**Tool Execution**: AI requests tool → `ToolExecutionUseCase.ExecuteToolsInSession()` → `PlanningExecutorAdapter` (if plan mode) → `ExecutorAdapter.ExecuteTool()` → Results fed back to AI

**Alert Investigation Flow**: Webhook received → `AlertHandler` → `AlertInvestigation.Investigate()` → `InvestigationRunner` (AI-driven investigation with tools) → `RCAService.Correlate()` → `EscalationHandler` (if needed)

### Ports (Interfaces)

| Port | Purpose | Adapter |
|------|---------|---------|
| `AIProvider` | AI model communication | `AnthropicAdapter` |
| `FileManager` | Sandboxed file operations | `LocalFileManager` |
| `Logger` | Structured logging | `logger.New()` (slog implementation) |
| `ToolExecutor` | Tool registry & execution | `ExecutorAdapter` (decorated by `PlanningExecutorAdapter`) |
| `UserInterface` | Terminal I/O | `CLIAdapter` |
| `AlertSourceManager` | Alert source lifecycle & dispatch | `SourceManager` |
| `WebhookAlertSource` | HTTP webhook alert ingestion | `PrometheusSource`, `GCPMonitoringSource` |
| `SkillManager` | Skill discovery & loading | `LocalSkillManager` |
| `SubagentManager` | Subagent discovery & loading | `LocalSubagentManager` |

## Adding New Tools

1. Register in `ExecutorAdapter.registerDefaultTools()` (`internal/infrastructure/adapter/tool/tool_executor_adapter.go`)
2. Implement in `ExecuteTool()` switch statement
3. Add tests

## Configuration

Environment variables with `AGENT_` prefix:
- `AGENT_MODEL` - AI model (default: `hf:zai-org/GLM-4.7`)
- `AGENT_MAX_TOKENS` - Response limit
- `AGENT_WORKING_DIR` - Base directory for file operations
- `AGENT_LOG_LEVEL` - Log level: `debug`, `info` (default), `warn`, `error`
- `AGENT_LOG_FORMAT` - Log format: `text` (default) or `json`
- `AGENT_SAFETY_COMMAND_VALIDATION_MODE` - Command validation mode: `blacklist` (default) or `whitelist`
- `AGENT_SAFETY_COMMAND_WHITELIST_JSON` - JSON array of whitelist patterns with optional excludes (whitelist mode only)
- `AGENT_SAFETY_COMMAND_WHITELIST_OVERRIDE` - Replace default whitelist patterns with custom ones (default: `false`)
- `AGENT_SAFETY_ASK_LLM_ON_UNKNOWN` - Ask LLM before blocking non-whitelisted commands (default: `true`)
- `AGENT_SAFETY_AUTO_APPROVE_SAFE` - Auto-approve non-dangerous bash commands without confirmation (default: `false`)
- `AGENT_COMPACTION_THRESHOLD` - Token threshold for auto-compaction of conversation history (default: `160000`, minimum: `10000`)

## Logging

### Logger Package (`internal/infrastructure/logger`)

The project uses a structured logging abstraction following hexagonal architecture principles:

**Port Layer** (`internal/domain/port/logger.go`):
- `port.Logger` - Core interface for structured logging with levels (Debug, Info, Warn, Error, Log)
- `NopLogger` - No-op implementation for testing
- `SafeLogger(l Logger) Logger` - Helper that returns the provided logger or NopLogger if nil

**Infrastructure Layer** (`internal/infrastructure/logger/`):
- `logger.New(Options)` - Creates a configured logger from Options struct
- `logger.NewDefault()` - Creates a logger with default settings (info level, text format, stderr)
- `logger.FromSlog(l *slog.Logger) Logger` - Wraps an existing slog.Logger

### Usage Patterns

Production code:
```go
// Accept Logger as a dependency in constructors
func NewSomeService(logger port.Logger) *SomeService {
    logger = port.SafeLogger(logger) // Default nil to NopLogger
    // ...
}

// Use With() to attach contextual fields ONCE at operation scope
func (s *SomeHandler) Handle(ctx context.Context, operationID string) error {
    // Create contextual logger for this operation - fields apply to all logs within
    log := s.logger.With("operation_id", operationID, "handler", s.name)
    
    // Now use 'log' instead of 's.logger' for all logs
    log.Info("Starting operation")
    log.Error("Failed step", "error", err)
    
    return nil
}

// For alert/investigation handlers:
func (h *Handler) ProcessAlert(ctx context.Context, alert *Alert) error {
    log := h.logger.With(
        "alert_id", alert.ID(),
        "alert_title", alert.Title(),
        "severity", alert.Severity(),
    )
    log.Info("Starting investigation")
    // ...
}
```

❌ **Anti-pattern** - Repeating the same context in every call:
```go
h.logger.Info("Starting investigation", "alert_id", alert.ID(), "severity", alert.Severity())
h.logger.Error("Failed step", "alert_id", alert.ID(), "error", err) // Repeats alert_id
```

✅ **Better** - Use `With()` once at scope entry:
```go
log := h.logger.With("alert_id", alert.ID(), "severity", alert.Severity())
log.Info("Starting investigation")
log.Error("Failed step", "error", err)
```

Tests:
```go
// Use NopLogger for tests where logs aren't relevant
svc := NewSomeService(port.NopLogger{})
```

## Testing Patterns

Table-driven tests throughout. Mock implementations of ports for isolated testing — see `conversation_service_test.go`.

## Alert Investigation & RCA System

The agent can receive alerts via webhooks and automatically investigate them using AI.

**Entities:**
- `Alert` - Incoming alert with source, severity, labels
- `Investigation` - Tracks investigation state, findings, and RCA results
- `RCAFinding` - Root cause analysis output with `Cause` (confidence-scored) and `Remedy` (with actionable steps and impact level)

**Webhook Server** (`serve` command):
- `GET /health` - Health check
- `GET /ready` - Readiness check
- `POST /alerts/{source-path}` - Receive webhooks from registered alert sources

**Alert Sources** implement `WebhookAlertSource` port:
- `PrometheusSource` - Parses Prometheus Alertmanager webhook payloads
- `GCPMonitoringSource` - Parses GCP Monitoring notification payloads
- New sources: implement `WebhookAlertSource` interface and register in `SourceRegistry`

**Investigation Workflow:**
1. Alert received via webhook → parsed by source adapter
2. `AlertHandler` creates investigation, runs async
3. `InvestigationRunner` orchestrates AI-driven investigation (uses tools to gather evidence)
4. `RCAService` correlates findings into structured root causes
5. `EscalationHandler` determines if escalation is needed based on severity/confidence

## Command Validation

Two modes for bash command safety. See `internal/domain/safety/` for implementation.

**Blacklist mode** (default): Blocks known dangerous patterns (`rm -rf /`, `sudo`, `curl | bash`, etc.). Matching commands require user confirmation.

**Whitelist mode** (`AGENT_SAFETY_COMMAND_VALIDATION_MODE=whitelist`): Only allows explicitly whitelisted commands (read-only operations by default). Custom patterns via `AGENT_SAFETY_COMMAND_WHITELIST_JSON`. Piped commands require all segments to be whitelisted.

Key files: `command_whitelist.go`, `dangerous_commands.go`, `constants.go`

## Skills System

Skills follow the [agentskills.io](https://agentskills.io) spec. Discovered from `./skills`, `./.claude/skills`, `~/.claude/skills` (priority order). Each skill has a `SKILL.md` with YAML frontmatter (`name`, `description`, optional `allowed-tools`). Activated on demand via `activate_skill` tool. See `skills/` for examples.

## Subagent System

Subagents are isolated AI agents for delegated tasks. Discovered from `./agents`, `./.claude/agents`, `~/.claude/agents`. Each has an `AGENT.md` with frontmatter (`name`, `description`, optional `allowed_tools`, `model`, `max_actions`). Spawned via `task` tool (synchronous) or `SubagentUseCase` (async/parallel). Subagents cannot spawn other subagents (recursion prevention). See `agents/` for examples.

## Plan Mode

In plan mode, tool executions are written as JSON to `.agent/plans/` instead of being executed. `PlanningExecutorAdapter` decorates the base executor using the decorator pattern. Activated via the `enter_plan_mode` tool.

## CI/CD

GitHub Actions workflows in `.github/workflows/`:
- `claude-code-review.yml` - Automated PR review using Claude Code
- `claude.yml` - Interactive Claude assistance triggered by PR comments

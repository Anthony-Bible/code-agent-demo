# Contributing

Contributions are welcome! Please ensure:
- All tests pass (`go test ./...`)
- Code is formatted (`go fmt ./...`)
- New features include tests
- Commits follow conventional commit format

## Development

### Running Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/domain/entity -v
go test ./internal/infrastructure/adapter/file -v
go test ./internal/application/usecase -v

# Run tests for alert handling
go test ./internal/infrastructure/adapter/alert -v

# Run tests for investigation system
go test ./internal/application/usecase -run TestAlertInvestigation -v
```

### Building

```bash
# Standard build
go build -o code-agent-demo ./cmd/cli

# Optimized build (smaller binary)
go build -ldflags="-s -w" -o code-agent-demo ./cmd/cli
```

### Code Quality

```bash
# Format code
go fmt ./...

# Run static analysis
go vet ./...
```

## Onboarding Guide for Contributors

### Layer Responsibilities

| Layer | Responsibility | Located In |
|-------|---------------|------------|
| **Presentation** | CLI commands (serve), HTTP webhook handling | `cmd/cli/` |
| **Application** | Use cases, orchestration (Tool, Alert, Investigation, Subagent) | `internal/application/` |
| **Domain** | Business entities, services, port interfaces | `internal/domain/` |
| **Infrastructure** | Port implementations, external integrations | `internal/infrastructure/` |

### Key Domain Entities

| Entity | Description |
|--------|-------------|
| `Conversation` | Manages conversation state and message collection |
| `Message` | Represents individual messages with role, content, and validation |
| `Tool` | Represents executable tools with schema validation |
| `Alert` | Represents alerts from monitoring systems with severity and labels |
| `Investigation` | Tracks alert investigations with findings, actions, and status |
| `Skill` | Represents discoverable AI capabilities from SKILL.md files |
| `Subagent` | Represents subagent configurations from AGENT.md files |

### Project Structure

```
github.com/anthony-bible/code-agent-demo/
├── cmd/
│   └── cli/
│       ├── main.go              # CLI entry point
│       └── cmd/
│           ├── root.go          # Root command setup
│           └── serve.go         # Webhook server command
├── internal/
│   ├── application/
│   │   ├── config/              # Configuration types (investigation config, etc.)
│   │   ├── dto/                 # Data transfer objects
│   │   ├── service/             # Application services (Investigation, Safety)
│   │   └── usecase/             # Use case implementations (Tool, Alert, Investigation, Subagent, BaseRunner)
│   ├── domain/
│   │   ├── entity/              # Domain entities (Conversation, Message, Tool, Alert, Investigation, Skill, Subagent)
│   │   ├── port/                # Port interfaces (AI, File, Tool, UI, Skill, Subagent, Alert, Context)
│   │   ├── safety/              # Safety rules (dangerous commands, etc.)
│   │   └── service/             # Domain services (Conversation, Tool)
│   └── infrastructure/
│       ├── adapter/             # Port implementations
│       │   ├── ai/              # AI provider adapters (Anthropic)
│       │   ├── alert/           # Alert source adapters (Prometheus, GCP Monitoring)
│       │   ├── file/            # File manager adapters (Local)
│       │   ├── investigation/   # Investigation storage adapters (File)
│       │   ├── skill/           # Skill manager adapters (Local)
│       │   ├── subagent/        # Subagent manager adapters (Local)
│       │   ├── tool/            # Tool executor adapters (Built-in tools)
│       │   ├── ui/              # User interface adapters (CLI)
│       │   └── webhook/         # Webhook server adapters (HTTP)
│       ├── config/              # Configuration & DI container
│       └── signal/              # Signal handlers (Interrupt, Reload)
├── agents/                      # Pre-defined subagents (code-reviewer, test-writer, documentation-writer)
├── skills/                      # Project-specific skills (agentskills.io spec)
├── config/                      # Configuration files (alert-sources.yaml, etc.)
├── CLAUDE.md                    # Project development guide
├── go.mod                       # Go module definition
└── go.sum                       # Dependency checksums
```

### Understanding the Codebase

#### 1. Start with the Domain Layer
The domain layer is the heart of the application and contains no external dependencies:

- **Entities** (`internal/domain/entity/`)
  - `Conversation` - Manages chat state and message collection
  - `Message` - Represents individual messages with role, content, and validation
  - `Tool` - Represents executable tools with schema validation
  - `Alert` - Represents alerts from monitoring systems
  - `Investigation` - Tracks investigation lifecycle with state machine
  - `Skill` - Represents discoverable AI capabilities
  - `Subagent` - Represents subagent configurations

- **Domain Services** (`internal/domain/service/`)
  - `ConversationService` - Core business logic for managing conversations
  - `ToolService` - Tool-related domain logic

- **Ports** (`internal/domain/port/`)
  - `AIProvider` - Interface for AI service integration
  - `FileManager` - Interface for file operations
  - `ToolExecutor` - Interface for tool execution
  - `UserInterface` - Interface for CLI interactions
  - `SkillManager` - Interface for skill discovery
  - `SubagentManager` - Interface for subagent management
  - `AlertSource` - Interface for alert ingestion
  - `Context` - Interface for AI context management

- **Safety** (`internal/domain/safety/`)
  - `DangerousCommands` - Definitions and validation of dangerous commands

#### 2. Review the Application Layer
This layer orchestrates use cases using domain services:

- **Use Cases** (`internal/application/usecase/`)
  - `BaseRunner` - Shared run context, tool execution, permissions & cleanup (extracted base for runners)
  - `ToolExecutionUseCase` - Handles tool execution and safety
  - `AlertHandler` - Receives and processes incoming alerts
  - `AlertInvestigation` - Runs investigation workflows
  - `InvestigationRunner` - Executes investigations with AI
  - `EscalationHandler` - Handles escalation logic
  - `SubagentRunner` - Manages subagent spawning and communication (concurrency-safe)
  - `SubagentUseCase` - Orchestrates async/parallel subagent execution

- **Services** (`internal/application/service/`)
  - `InvestigationStore` - Persistence for investigations
  - `SafetyEnforcer` - Command safety validation

- **Configuration** (`internal/application/config/`)
  - `InvestigationConfig` - Investigation-specific configuration

#### 3. Examine the Infrastructure Layer
Implementations of the ports defined in the domain:

- **Adapters** (`internal/infrastructure/adapter/`)
  - `ai/anthropic_adapter.go` - Anthropic API implementation
  - `file/local_file_adapter.go` - Local file system operations
  - `tool/tool_executor_adapter.go` - Tool registry & routing (split into per-domain files: bash, file, fetch, investigation, skill, subagent, plan/batch)
  - `ui/cli_adapter.go` - Terminal interface
  - `skill/local_skill_adapter.go` - Skill discovery from filesystem
  - `subagent/local_subagent_adapter.go` - Subagent discovery from filesystem
  - `alert/prometheus_source.go` - Prometheus Alertmanager webhook handler
  - `alert/gcp_monitoring_source.go` - GCP Monitoring webhook handler
  - `investigation/file_store.go` - Investigation persistence
  - `webhook/http_adapter.go` - HTTP webhook server

- **Config** (`internal/infrastructure/config/`)
  - `container.go` - Dependency injection container
  - `config.go` - Configuration management
  - `webhook_config.go` - Webhook server configuration

- **Signal** (`internal/infrastructure/signal/`)
  - `interrupt_handler.go` - Graceful shutdown handling
  - `reload_handler.go` - SIGHUP handler for skill hot-reload

#### 4. CLI Entry Point
- `cmd/cli/main.go` - Application entry point
- `cmd/cli/cmd/` - CLI command definitions using cobra
  - `root.go` - Root command and global flags
  - `serve.go` - Webhook server command

### Adding an Alert Source

1. **Implement the new source** in `internal/infrastructure/adapter/alert/`:

```go
// my_source.go
package alert

import (
    "github.com/anthony-bible/code-agent-demo/internal/domain/entity"
    "github.com/anthony-bible/code-agent-demo/internal/domain/port"
    "context"
)

type MySource struct {
    name        string
    webhookPath string
}

func NewMySource(config SourceConfig) (port.WebhookAlertSource, error) {
    // Validate and create source
    return &MySource{
        name:        config.Name,
        webhookPath: config.WebhookPath,
    }, nil
}

func (m *MySource) Name() string { return m.name }
func (m *MySource) Type() port.SourceType { return port.SourceTypeWebhook }
func (m *MySource) WebhookPath() string { return m.webhookPath }
func (m *MySource) Close() error { return nil }

func (m *MySource) HandleWebhook(ctx context.Context, payload []byte) ([]*entity.Alert, error) {
    // Parse webhook payload and return alerts
    alert, _ := entity.NewAlert("id", m.name, entity.SeverityCritical, "title")
    return []*entity.Alert{alert}, nil
}
```

2. **Register the source** in `internal/infrastructure/adapter/alert/source_registry.go`:

```go
func (r *SourceRegistry) RegisterBuiltinFactories() {
    r.RegisterFactory("prometheus", NewPrometheusSource)
    r.RegisterFactory("gcp_monitoring", NewGCPMonitoringSource)
    r.RegisterFactory("my_source", NewMySource)  // Add this line
}
```

3. **Add configuration** to `config/alert-sources.yaml`:

```yaml
sources:
  - type: my_source
    name: my-alerts
    webhook_path: /alerts/my
```

4. **Tests are required** - Add corresponding tests in `*_test.go`

### Adding an Investigation Skill

Create investigation-focused skills to enhance alert analysis:

**skills/alert-investigation/SKILL.md**:
```yaml
---
name: alert-investigation
description: Specialized in investigating alerts from monitoring systems
allowed-tools:
  - read_file
  - list_files
  - bash
  - activate_skill
---

# Alert Investigation Skill

You are specialized in investigating alerts from monitoring systems.

## Investigation Process

1. **Understand the Alert**: Review alert metadata, labels, and description
2. **Check Logs**: Look at relevant log files using bash and read_file
3. **Analyze Metrics**: Query monitoring systems if available
4. **Examine Code**: Review recent changes that might be related
5. **Provide Findings**: Complete investigation with confidence and recommended actions

## Output Format

Complete investigation with:
- Confidence level (0.0-1.0)
- List of findings with severity
- Root cause analysis when possible
- Recommended actions
```

### Adding a New Tool

1. **Define the tool in the adapter** (`internal/infrastructure/adapter/tool/tool_executor_adapter.go`)

```go
// In NewExecutorAdapter(), register your tool:
tool := entity.NewTool("search_content", "Search for content in files")
// Set up input schema, then...
exec.RegisterTool(*tool)

// And implement the logic in the ExecuteTool method:
case "search_content":
    return s.searchInFile(input)
```

2. **Tests are required** - Add corresponding tests in `*_test.go`

### Adding a New AI Provider

1. **Implement the AIProvider port** in `internal/infrastructure/adapter/ai/`
2. **Register in the container** - Update `internal/infrastructure/config/container.go`

### Testing Philosophy

The project uses table-driven tests throughout. Look at existing tests for patterns:

- `internal/domain/entity/*_test.go` - Examples of entity testing
- `internal/domain/service/*_test.go` - Service testing with mocks
- `internal/infrastructure/adapter/*/_test.go` - Adapter testing
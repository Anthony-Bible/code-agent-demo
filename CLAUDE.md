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
- `adapter/ui/cli_adapter.go` - Implements `UserInterface`
- `config/container.go` - Dependency injection wiring
- `signal/interrupt_handler.go` - Double Ctrl+C exit handling

### Key Data Flows

**Chat Flow**: User Input → `ChatService.SendMessage()` → `ConversationService.ProcessAssistantResponse()` → `AIProvider.SendMessage()` → Tool execution if needed → Response

**Tool Execution**: AI requests tool → `ToolExecutionUseCase.ExecuteToolsInSession()` → `ExecutorAdapter.ExecuteTool()` → Results fed back to AI

### Ports (Interfaces)

| Port | Purpose | Adapter |
|------|---------|---------|
| `AIProvider` | AI model communication | `AnthropicAdapter` |
| `FileManager` | Sandboxed file operations | `LocalFileManager` |
| `ToolExecutor` | Tool registry & execution | `ExecutorAdapter` |
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

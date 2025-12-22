# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a Go-based AI coding agent that creates an interactive console chat application with file manipulation capabilities. It interfaces with the Anthropic Claude API and supports custom models.

## Development Commands

### Building and Running
```bash
# Build the application
go build -o code-editing-agent

# Run directly (for development)
go run main.go

# Build optimized binary
go build -ldflags="-s -w" -o code-editing-agent
```

### Dependencies
```bash
# Install/update dependencies
go mod tidy

# Download dependencies
go mod download

# Verify dependencies
go mod verify
```

### Testing
```bash
# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with verbose output
go test -v ./...
```

### Code Quality
```bash
# Format code
go fmt ./...

# Run static analysis
go vet ./...
```

## Architecture

### Core Components
- **Agent**: Main struct managing conversation state and tool execution
- **Tool System**: Modular tool architecture with JSON schema validation
  - `read_file`: Read file contents
  - `list_files`: Directory exploration with recursive walk
  - `edit_file`: String replacement-based editing with file creation support
- **Chat Interface**: Command-line interactive chat with colored output
- **Message Handling**: Structured conversation management with message history

### Key Patterns
- **Tool Definition Pattern**: Tools are defined as structured objects with name, description, and input schema
- **Schema Generation**: Uses `github.com/invopop/jsonschema` for automatic JSON schema generation
- **Error Propagation**: Errors are gracefully handled and propagated to the CLI with user-friendly messages
- **Single Binary Deployment**: Application compiles to a single executable

### Model Configuration
- Default model: `hf:zai-org/GLM-4.6`
- Can be changed to Claude models by modifying the model variable in main.go
- Uses Anthropic SDK for API communication

## File Structure

The application is primarily contained in `main.go` with the following key sections:
- Agent struct and methods
- Tool definitions and implementations
- Chat interface loop
- Color definitions for terminal output

## Extension Points

### Adding New Tools
1. Define the tool input struct with JSON tags
2. Create a ToolDefinition with name, description, and input schema
3. Implement the tool logic and add it to the tool map
4. Register the tool in the switch statement

### Modifying Behavior
- Change model: Update the `model` variable in main()
- Add colors: Extend the color constants map
- Modify output: Update the print functions with new color schemes
# Domain Port Tests - Red Phase

This package contains comprehensive failing tests for the domain ports based on hexagonal architecture principles. All tests are designed to fail initially and define the expected behavior of each port interface.

## Port Interfaces Tested

### 1. AIProvider (`ai_provider_test.go`)
**Purpose**: External AI service integration port
**Expected Methods**:
- `SendMessage(ctx context.Context, messages []MessageParam, tools []ToolParam) (*Message, error)`
- `GenerateToolSchema[T any]() ToolInputSchemaParam`
- `HealthCheck(ctx context.Context) error`
- `SetModel(model string) error`
- `GetModel() string`

**Tests**: 5 tests covering contract validation and individual method existence

### 2. FileManager (`file_manager_test.go`)
**Purpose**: File system operations port
**Expected Methods**:
- `ReadFile(path string) (string, error)`
- `WriteFile(path string, content string) error`
- `ListFiles(path string, recursive bool) ([]string, error)`
- `FileExists(path string) (bool, error)`
- `CreateDirectory(path string) error`
- `DeleteFile(path string) error`
- `GetFileInfo(path string) (FileInfo, error)`

**Tests**: 8 tests covering contract validation and individual method existence

### 3. UserInterface (`user_interface_test.go`)
**Purpose**: User interaction port (CLI)
**Expected Methods**:
- `GetUserInput(ctx context.Context) (string, bool)` // returns input and continuation flag
- `DisplayMessage(message string, messageRole string) error`
- `DisplayError(err error) error`
- `DisplayToolResult(toolName string, input string, result string) error`
- `DisplaySystemMessage(message string) error`
- `SetPrompt(prompt string) error`
- `ClearScreen() error`
- `SetColorScheme(scheme ColorScheme) error`

**Tests**: 9 tests covering contract validation and individual method existence

### 4. ToolExecutor (`tool_executor_test.go`)
**Purpose**: Tool execution and management port
**Expected Methods**:
- `RegisterTool(tool Tool) error`
- `UnregisterTool(name string) error`
- `ExecuteTool(ctx context.Context, name string, input interface{}) (string, error)`
- `ListTools() ([]Tool, error)`
- `GetTool(name string) (Tool, bool)`
- `ValidateToolInput(name string, input interface{}) error`

**Tests**: 7 tests covering contract validation and individual method existence

## Test Results

All tests currently fail as expected (Red Phase):
```
FAIL       29 tests total
- 5 AIProvider tests
- 8 FileManager tests
- 9 UserInterface tests
- 7 ToolExecutor tests
```

## Implementation Guidance

When implementing these interfaces in Phase 2 (Green Phase):

1. Create the interface definitions in their respective files:
   - `domain/port/ai_provider.go`
   - `domain/port/file_manager.go`
   - `domain/port/user_interface.go`
   - `domain/port/tool_executor.go`

2. Follow the exact method signatures documented in the test comments

3. Ensure implementations handle all the scenarios implied by the original monolithic code

4. Maintain hexagonal architecture principles - ports define contracts, implementations are adapters

## Design Decisions

- Tests focus on interface contracts rather than complex behavior scenarios
- Simplified to avoid external dependencies (removed anthropic SDK imports)
- Each method has its own test to ensure granular validation
- Tests use `t.Fatal()` to ensure they fail until implementations exist

This red-phase setup provides clear guidance for implementing the domain ports while maintaining the test-driven development approach.
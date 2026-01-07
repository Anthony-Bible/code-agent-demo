# RED PHASE - Step 11: Add Thinking Mode Methods to ConversationServiceInterface

## Summary

Created comprehensive failing tests that verify the `ConversationServiceInterface` includes thinking mode methods (`SetThinkingMode` and `GetThinkingMode`).

## Test File Created

**File**: `/home/anthony/GolandProjects/code-editing-agent/internal/application/usecase/alert_investigation_test.go`

Added 8 new test functions (approximately 375 lines of test code):

### Test Functions

1. **TestConversationServiceInterfaceHasThinkingMethods**
   - Verifies interface has `SetThinkingMode` method
   - Verifies interface has `GetThinkingMode` method
   - Uses type assertion to ensure mock implements interface

2. **TestConversationServiceInterfaceThinkingMethodSignatures**
   - Tests `SetThinkingMode(sessionID string, info port.ThinkingModeInfo) error` signature
   - Tests `GetThinkingMode(sessionID string) (port.ThinkingModeInfo, error)` signature
   - Verifies methods are accessible through the interface
   - Confirms methods do NOT require context (unlike `SetCustomSystemPrompt`)

3. **TestThinkingModeMethodsBehavior**
   - Table-driven test with 4 scenarios:
     - Enabled with budget
     - Disabled with zero budget
     - Enabled without showing thinking
     - Enabled with showing thinking but low budget
   - Verifies all fields (Enabled, BudgetTokens, ShowThinking) persist correctly

4. **TestThinkingModeIsolation**
   - Verifies thinking mode settings are isolated per session
   - Tests two sessions with different settings
   - Ensures no cross-session contamination

5. **TestThinkingModeUpdateBehavior**
   - Verifies thinking mode can be updated multiple times
   - Tests that updates overwrite previous settings

6. **TestGetThinkingModeDefaultBehavior**
   - Tests behavior when getting thinking mode for a session where it was never set
   - Expects zero-value `ThinkingModeInfo` (matches ConversationService behavior)

### Mock Implementation

Created `mockConversationServiceWithThinking` type that implements:
- All existing `ConversationServiceInterface` methods
- **NEW**: `SetThinkingMode(sessionID string, info port.ThinkingModeInfo) error`
- **NEW**: `GetThinkingMode(sessionID string) (port.ThinkingModeInfo, error)`

## Expected Failures

### Compilation Errors

```
internal/application/usecase/alert_investigation_test.go:1555:16:
  iface.SetThinkingMode undefined (type ConversationServiceInterface has no field or method SetThinkingMode)

internal/application/usecase/alert_investigation_test.go:1560:27:
  iface.GetThinkingMode undefined (type ConversationServiceInterface has no field or method GetThinkingMode)
```

### Why Tests Fail

The `ConversationServiceInterface` in `/home/anthony/GolandProjects/code-editing-agent/internal/application/usecase/alert_investigation.go` currently has these methods:

```go
type ConversationServiceInterface interface {
    StartConversation(ctx context.Context) (string, error)
    AddUserMessage(ctx context.Context, sessionID, content string) (*entity.Message, error)
    ProcessAssistantResponse(ctx context.Context, sessionID string) (*entity.Message, []port.ToolCallInfo, error)
    AddToolResultMessage(ctx context.Context, sessionID string, toolResults []entity.ToolResult) error
    EndConversation(ctx context.Context, sessionID string) error
    SetCustomSystemPrompt(ctx context.Context, sessionID, prompt string) error
    // MISSING: SetThinkingMode(sessionID string, info port.ThinkingModeInfo) error
    // MISSING: GetThinkingMode(sessionID string) (port.ThinkingModeInfo, error)
}
```

## Context: Why This Interface Needs These Methods

The **real implementation** (`ConversationService` in `internal/domain/service/conversation_service.go`) already has these methods:

```go
func (cs *ConversationService) SetThinkingMode(sessionID string, info port.ThinkingModeInfo) error
func (cs *ConversationService) GetThinkingMode(sessionID string) (port.ThinkingModeInfo, error)
```

However, the **interface** used by `SubagentRunner` and other application-layer components does NOT include them, causing a mismatch.

## What Needs to Be Fixed (Green Phase)

Add the following methods to `ConversationServiceInterface` in `/home/anthony/GolandProjects/code-editing-agent/internal/application/usecase/alert_investigation.go`:

```go
type ConversationServiceInterface interface {
    StartConversation(ctx context.Context) (string, error)
    AddUserMessage(ctx context.Context, sessionID, content string) (*entity.Message, error)
    ProcessAssistantResponse(ctx context.Context, sessionID string) (*entity.Message, []port.ToolCallInfo, error)
    AddToolResultMessage(ctx context.Context, sessionID string, toolResults []entity.ToolResult) error
    EndConversation(ctx context.Context, sessionID string) error
    SetCustomSystemPrompt(ctx context.Context, sessionID, prompt string) error

    // ADD THESE:
    SetThinkingMode(sessionID string, info port.ThinkingModeInfo) error
    GetThinkingMode(sessionID string) (port.ThinkingModeInfo, error)
}
```

## Test Coverage

The tests verify:

✅ **Interface completeness**: Methods exist in interface
✅ **Method signatures**: Correct parameter and return types
✅ **Behavior**: Set/Get operations work correctly
✅ **Isolation**: Per-session state management
✅ **Updates**: Multiple updates to same session
✅ **Defaults**: Zero-value behavior for unset sessions
✅ **No context requirement**: Unlike SetCustomSystemPrompt, these methods don't need context

## Running the Tests

```bash
# Run all thinking mode tests
go test ./internal/application/usecase -v -run TestConversationServiceInterface

# Run specific test
go test ./internal/application/usecase -v -run TestConversationServiceInterfaceHasThinkingMethods

# Run all usecase tests (will fail to compile until interface is updated)
go test ./internal/application/usecase -v
```

## Test Design Rationale

### Why These Tests Follow TDD Best Practices

1. **Tests the interface, not the implementation**
   - Uses type assertions to verify interface satisfaction
   - Tests behavior through the interface, not concrete type

2. **Comprehensive coverage**
   - Happy path (enabled/disabled modes)
   - Edge cases (zero budget, unset sessions)
   - State management (isolation, updates)

3. **Clear failure messages**
   - Each test has descriptive names
   - Assertions include expected vs actual values
   - Comments explain what should happen

4. **Matches existing patterns**
   - Follows same structure as `SetCustomSystemPrompt`
   - Uses same error handling approach
   - Maintains consistency with `ConversationService` implementation

## Next Steps (GREEN PHASE)

1. Add `SetThinkingMode` and `GetThinkingMode` to `ConversationServiceInterface`
2. Run tests - they should now pass
3. Verify `SubagentRunner` can now use these methods
4. No changes needed to `ConversationService` (already implements them)

## Files Modified

- `/home/anthony/GolandProjects/code-editing-agent/internal/application/usecase/alert_investigation_test.go` (375 lines added)

## Files That Will Need Changes (Green Phase)

- `/home/anthony/GolandProjects/code-editing-agent/internal/application/usecase/alert_investigation.go` (2 lines added to interface)

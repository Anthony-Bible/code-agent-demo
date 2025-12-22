# Domain Entity Tests - Red Phase

This directory contains comprehensive failing tests for the domain entities identified from the monolithic code in `main.go:25-59`.

## Domain Entities

### 1. Conversation Entity (`conversation_test.go`)
Represents the chat conversation state and manages message flow.

**Tests Coverage:**
- ✅ Conversation creation (`NewConversation`)
- ✅ Adding messages (`AddMessage`) with validation
- ✅ Retrieving messages (`GetMessages`)
- ✅ Getting the last message (`GetLastMessage`)
- ✅ Clearing conversation (`Clear`)
- ✅ Message counting (`MessageCount`)

**Business Rules Tested:**
- Message role validation (user, assistant, system)
- Message content validation (non-empty, non-whitespace)
- Ordered message collection
- Empty conversation handling

### 2. Message Entity (`message_test.go`)
Represents individual messages in the conversation.

**Tests Coverage:**
- ✅ Message creation (`NewMessage`) with validation
- ✅ Role checking methods (`IsUser`, `IsAssistant`, `IsSystem`)
- ✅ Message validation (`Validate`)
- ✅ Content updates (`UpdateContent`)
- ✅ Age tracking (`GetAge`)

**Business Rules Tested:**
- Valid roles: user, assistant, system
- Content must be non-empty
- Timestamp tracking
- Role-specific operations

### 3. Tool Entity (`tool_test.go`)
Represents tools that can be executed by the agent.

**Tests Coverage:**
- ✅ Tool creation (`NewTool`) with validation
- ✅ Tool validation (`Validate`)
- ✅ Tool equality (`Equals`)
- ✅ Input schema management (`AddInputSchema`)
- ✅ Required field checking (`HasRequired`)
- ✅ Input validation (`ValidateInput`)
- ✅ Description access (`GetDescription`)

**Business Rules Tested:**
- Tool ID uniqueness
- Schema validation
- Required field enforcement
- JSON input validation

## Test Structure

All tests follow these patterns:
1. **Table-driven tests** for comprehensive scenario coverage
2. **Happy path** and **error path** testing
3. **Edge case** validation
4. **Golang testing conventions** with descriptive test names
5. **Clear assertion failure messages**

## Current State: 🔴 RED PHASE

All tests FAIL TO COMPILE as expected because:
- Domain entity packages do not exist
- Domain entity types are not yet implemented
- Domain entity methods are not yet defined

This is the intentional starting point for Test-Driven Development.

## Next Steps

To proceed with the hexagonal architecture refactoring:

1. **Green Phase**: Implement the domain entities to make tests pass
2. **Create domain packages:**
   - `internal/domain/entity/conversation.go`
   - `internal/domain/entity/message.go`
   - `internal/domain/entity/tool.go`
3. **Implement all required methods** as specified by the failing tests
4. **Run tests** to verify all pass
5. **Refactor Phase**: Optimize implementation while maintaining test coverage

## Directory Structure

```
test/domain/entity/
├── conversation_test.go  # Tests for Conversation entity
├── message_test.go       # Tests for Message entity
├── tool_test.go          # Tests for Tool entity
└── README.md             # This documentation
```
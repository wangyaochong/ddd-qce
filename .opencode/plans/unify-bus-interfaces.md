# Plan: Unify CQRS Interface Naming — Remove Executor, Keep Only Bus

## Goal
Remove `CommandExecutor` and `QueryExecutor` interfaces, inline their methods into `CommandBus` and `QueryBus`, update all references across the codebase.

## Changes

### 1. Core Interface Files

**`cqrs/command/command.go`**
- Delete `CommandExecutor` interface
- Inline `Execute` method into `CommandBus` (alongside `RegisterHandler`)
- Change `Dispatch` parameter from `CommandExecutor` → `CommandBus`

**`cqrs/query/query.go`**
- Delete `QueryExecutor` interface
- Inline `Execute` method into `QueryBus` (alongside `RegisterHandler`)
- Change `Dispatch` parameter from `QueryExecutor` → `QueryBus`

### 2. Compile Assertions

**`cqrs/command/memory/command_bus.go`**
- `var _ command.CommandExecutor = (*CommandBus)(nil)` → `var _ command.CommandBus = (*CommandBus)(nil)`

### 3. Application Layer

**`exampleapp/application/event_handlers.go`**
- `cmdBus command.CommandExecutor` → `cmdBus command.CommandBus` (lines 13, 31)
- `func NewOrderPlacedInventoryHandler(cmdBus command.CommandExecutor)` → `cmdBus command.CommandBus` (line 16)
- `func NewOrderCancelledInventoryHandler(cmdBus command.CommandExecutor)` → `cmdBus command.CommandBus` (line 34)

**`exampleapp/application/application_test.go`**
- `var executor command.CommandExecutor = cmdBus` → `var executor command.CommandBus = cmdBus` (line 306)

### 4. Job Manager

**`job/memory/job_manager.go`**
- `executor command.CommandExecutor` → `executor command.CommandBus` (line 20)
- `func NewJobManager(store jobcore.JobStore, executor command.CommandExecutor, ...)` → `executor command.CommandBus` (line 29)

### 5. Integration Tests

**`cqrs/command/command_integration_test.go`**
- Rename `TestCommandExecutor_InterfaceSatisfaction` → `TestCommandBus_InterfaceSatisfaction`
- `var _ command.CommandExecutor = ...` → `var _ command.CommandBus = ...` (lines 42, 50, 69, 82, 126)
- Rename `TestCommandExecutor_Execute` → `TestCommandBus_Execute`
- Rename `TestCommandExecutor_Execute_NoHandler` → `TestCommandBus_Execute_NoHandler`
- Rename `TestCommandExecutor_Execute_Error` → `TestCommandBus_Execute_Error`
- Rename `TestCommandExecutor_Execute_WithAspects` → `TestCommandBus_Execute_WithAspects`
- `var executor command.CommandExecutor = bus` → `var executor command.CommandBus = bus`

### 6. Documentation

**`docs/architecture.md`**
- Remove CommandExecutor/QueryExecutor rows from interface table (lines 280-281, 285-286)
- Update CommandBus/QueryBus descriptions: no longer "嵌入 CommandExecutor", now direct methods
- Update Dispatch description: "接受 CommandBus" instead of "接受 CommandExecutor"
- Update ISP section (line 374): remove CommandExecutor/QueryExecutor mention, explain simplified approach
- Update code examples (lines 166, 401, 420)

**`docs/guide.md`**
- Update JobManager description (line 821): "需要 CommandBus" instead of "需要 CommandExecutor"

**`README.md`**
- Update directory descriptions (lines 179, 181): remove CommandExecutor/QueryExecutor

## Verification
- `go build ./...` — compile check
- `go test ./...` — full test suite

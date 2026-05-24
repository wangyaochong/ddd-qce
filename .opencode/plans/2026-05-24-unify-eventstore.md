# Unify Dual EventStore Abstraction Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move `EventStore[T]` interface from `cqrs/event` to `domain/event`, delete `DomainEventAppendOnlyStore`, and unify the two overlapping abstractions into one.

**Architecture:** The generic `EventStore[T]` interface is a domain concept (not CQRS-specific), so it belongs in `domain/event`. The non-generic `DomainEventAppendOnlyStore` is semantically equivalent to `EventStore[DomainEvent]` and will be deleted. The `DomainEventStore` concrete implementation (which handles the interface-typed `T=DomainEvent` case that the generic `EventStore[T]` cannot due to its pointer-type constraint) is kept but switched to satisfy `EventStore[DomainEvent]` instead. `cqrs/event` re-exports via type alias for backward compatibility.

**Tech Stack:** Go 1.26, no new dependencies.

---

## Key Constraints

1. **`EventStore[T]` requires `T` to be a pointer type** — both `memory.NewEventStore` and `pg.NewEventStore` check `reflect.TypeOf(zero).Kind() == reflect.Ptr` at construction. When `T=event.DomainEvent` (interface), `reflect.TypeOf(zero)` is `nil`, causing a panic. Therefore `DomainEventStore` **cannot be replaced by** `NewEventStore[event.DomainEvent]()` — it must remain a separate concrete struct that implements the `EventStore[event.DomainEvent]` interface without going through `NewEventStore`.

2. **No circular dependency** — `domain/event` currently imports only stdlib. Moving `EventStore[T]` there adds no new imports (it only uses `context.Context` and `DomainEvent`, both already in scope).

3. **Type alias preserves backward compat** — `type EventStore[T] = domainevent.EventStore[T]` in `cqrs/event` means existing `cqevent.EventStore[T]` references continue to compile.

---

## File Change Map

| File | Action | Responsibility |
|------|--------|----------------|
| `domain/event/event.go` | Modify | Add `EventStore[T]`, delete `DomainEventAppendOnlyStore` |
| `cqrs/event/event_store.go` | Modify | Replace interface definition with type alias |
| `cqrs/event/memory/domain_event_store.go` | Modify | Switch interface check from `DomainEventAppendOnlyStore` to `EventStore[DomainEvent]` |
| `cqrs/event/memory/event_store_test.go` | Modify | Update compile-time check import |
| `it/cqrs_event_pg/event_store_test.go` | Modify | Update compile-time check import |
| `docs/architecture.md` | Modify | Update EventStore location |
| `README.md` | Modify | Update directory description |

---

### Task 1: Add `EventStore[T]` to `domain/event/event.go`

**Files:**
- Modify: `domain/event/event.go:38-41`

- [ ] **Step 1: Replace `DomainEventAppendOnlyStore` with generic `EventStore[T]`**

In `domain/event/event.go`, replace lines 38-41:

```go
type DomainEventAppendOnlyStore interface {
	Append(ctx context.Context, aggregateID string, expectedVersion int, events []DomainEvent) error
	Load(ctx context.Context, aggregateID string, afterVersion int) ([]DomainEvent, error)
}
```

with:

```go
type EventStore[T DomainEvent] interface {
	Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error
	Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error)
}
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./domain/event/...`
Expected: SUCCESS (no external consumers of `DomainEventAppendOnlyStore` outside `cqrs/event/memory`)

- [ ] **Step 3: Commit**

```bash
git add domain/event/event.go
git commit -m "refactor: move EventStore[T] interface to domain/event, delete DomainEventAppendOnlyStore"
```

---

### Task 2: Replace `cqrs/event/event_store.go` with type alias

**Files:**
- Modify: `cqrs/event/event_store.go`

- [ ] **Step 1: Replace interface definition with type alias**

Replace the entire content of `cqrs/event/event_store.go`:

```go
package event

import (
	"context"

	"github.com/ddd-qce/core/domain/event"
)

type EventStore[T event.DomainEvent] interface {
	Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error
	Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error)
}
```

with:

```go
package event

import (
	domainevent "github.com/ddd-qce/core/domain/event"
)

type EventStore[T domainevent.DomainEvent] = domainevent.EventStore[T]
```

- [ ] **Step 2: Verify compilation**

Run: `go build ./cqrs/event/...`
Expected: SUCCESS — the type alias means `cqevent.EventStore[T]` is identical to `domainevent.EventStore[T]`

- [ ] **Step 3: Commit**

```bash
git add cqrs/event/event_store.go
git commit -m "refactor: cqrs/event.EventStore[T] becomes type alias for domain/event.EventStore[T]"
```

---

### Task 3: Update `DomainEventStore` interface check

**Files:**
- Modify: `cqrs/event/memory/domain_event_store.go:51`

- [ ] **Step 1: Change the compile-time interface check**

In `cqrs/event/memory/domain_event_store.go`, replace line 51:

```go
var _ event.DomainEventAppendOnlyStore = (*DomainEventStore)(nil)
```

with:

```go
var _ event.EventStore[event.DomainEvent] = (*DomainEventStore)(nil)
```

This is valid because `DomainEventStore`'s method signatures are:
- `Append(ctx context.Context, aggregateID string, expectedVersion int, events []event.DomainEvent) error`
- `Load(ctx context.Context, aggregateID string, afterVersion int) ([]event.DomainEvent, error)`

When `T = event.DomainEvent`, the `EventStore[T]` interface requires exactly these signatures.

- [ ] **Step 2: Verify compilation**

Run: `go build ./cqrs/event/memory/...`
Expected: SUCCESS

- [ ] **Step 3: Commit**

```bash
git add cqrs/event/memory/domain_event_store.go
git commit -m "refactor: DomainEventStore now satisfies EventStore[DomainEvent] instead of DomainEventAppendOnlyStore"
```

---

### Task 4: Update compile-time checks in test files

**Files:**
- Modify: `cqrs/event/memory/event_store_test.go:8,12`
- Modify: `it/cqrs_event_pg/event_store_test.go:9,16`

- [ ] **Step 1: Update `cqrs/event/memory/event_store_test.go`**

Replace lines 8-12:

```go
	cqevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/domain/event"
)

var _ cqevent.EventStore[*testStoreEvent] = (*EventStore[*testStoreEvent])(nil)
```

with:

```go
	"github.com/ddd-qce/core/domain/event"
)

var _ event.EventStore[*testStoreEvent] = (*EventStore[*testStoreEvent])(nil)
```

Note: remove the `cqevent` import since it's no longer used.

- [ ] **Step 2: Update `it/cqrs_event_pg/event_store_test.go`**

Replace lines 9-16:

```go
	cqevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/domain/event"
	pgevent "github.com/ddd-qce/core/cqrs/event/pg"
	"github.com/ddd-qce/it/testutil"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var _ cqevent.EventStore[*testDomainEvent] = (*pgevent.EventStore[*testDomainEvent])(nil)
```

with:

```go
	"github.com/ddd-qce/core/domain/event"
	pgevent "github.com/ddd-qce/core/cqrs/event/pg"
	"github.com/ddd-qce/it/testutil"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var _ event.EventStore[*testDomainEvent] = (*pgevent.EventStore[*testDomainEvent])(nil)
```

Note: remove the `cqevent` import since it's no longer used.

- [ ] **Step 3: Verify compilation**

Run: `go build ./cqrs/event/memory/... ./it/cqrs_event_pg/...`
Expected: SUCCESS

- [ ] **Step 4: Commit**

```bash
git add cqrs/event/memory/event_store_test.go it/cqrs_event_pg/event_store_test.go
git commit -m "refactor: use domain/event.EventStore[T] in compile-time checks"
```

---

### Task 5: Run full test suite

**Files:** None (verification only)

- [ ] **Step 1: Run all unit tests**

Run: `go test ./domain/event/... ./cqrs/event/... ./infra/...`
Expected: All PASS

- [ ] **Step 2: Run exampleapp tests**

Run: `go test ./exampleapp/application/... ./exampleapp/infrastructure/... ./exampleapp/integration/...`
Expected: All PASS

- [ ] **Step 3: Run example tests**

Run: `go test ./example/...`
Expected: All PASS

- [ ] **Step 4: Run integration tests (if PG available)**

Run: `go test ./it/...`
Expected: All PASS (or skip if no PG connection)

---

### Task 6: Update documentation

**Files:**
- Modify: `docs/architecture.md:270,285-289`
- Modify: `README.md:167,181-183`

- [ ] **Step 1: Update `docs/architecture.md`**

On line 270, the row already shows `EventStore[T]` at `domain/event` — this is correct, no change needed.

On lines 285-289, update the CQRS layer table:

Replace:
```
| AppendOnlyStore[T] | `cqrs/event` | 追加存储接口，Append(ctx, aggregateID, expectedVersion, events) / Load(...) |
```

with:
```
| EventStore[T] | `cqrs/event` | 事件存储接口（re-export from domain/event），Append(ctx, aggregateID, expectedVersion, events) / Load(...) |
```

On line 288, replace:
```
| EventStore[T] | `cqrs/event/memory` | 内存事件存储，实现 AppendOnlyStore[T] |
```

with:
```
| EventStore[T] | `cqrs/event/memory` | 内存事件存储，实现 domain/event.EventStore[T] |
```

On line 289, replace:
```
| EventStore[T] | `cqrs/event/pg` | PostgreSQL 事件存储，实现 AppendOnlyStore[T] |
```

with:
```
| EventStore[T] | `cqrs/event/pg` | PostgreSQL 事件存储，实现 domain/event.EventStore[T] |
```

- [ ] **Step 2: Update `README.md`**

On line 167, it already shows `EventStore[T]` at `domain/event` — correct.

On line 181, replace:
```
│   └── /event                   # EventBus / TypedEventBus[T] / AppendOnlyStore[T] 接口
```

with:
```
│   └── /event                   # EventBus 接口 / EventStore[T] re-export from domain/event
```

On line 182, replace:
```
│       ├── /memory              # 内存 EventBus / RegisterHandler[T] / Dispatch[T] / DomainEventStore / EventStore[T]
```

with:
```
│       ├── /memory              # 内存 EventBus / RegisterHandler[T] / Dispatch[T] / DomainEventStore(impl EventStore[DomainEvent]) / EventStore[T]
```

- [ ] **Step 3: Commit**

```bash
git add docs/architecture.md README.md
git commit -m "docs: update EventStore[T] location and DomainEventAppendOnlyStore removal"
```

---

## Self-Review Checklist

1. **Spec coverage:** The goal was to move `EventStore[T]` to `domain/event` and delete `DomainEventAppendOnlyStore`. Tasks 1-3 accomplish this. No gaps.

2. **Placeholder scan:** No TBDs, TODOs, or "implement later" patterns. All code is shown inline.

3. **Type consistency:**
   - `domain/event.EventStore[T]` defined in Task 1 → used by type alias in Task 2 → used by `DomainEventStore` in Task 3 → used by test compile-checks in Task 4. All consistent.
   - `*DomainEventStore` methods `Append(ctx, aggID, ver, []event.DomainEvent)` and `Load(ctx, aggID, ver) ([]event.DomainEvent, error)` satisfy `event.EventStore[event.DomainEvent]` because when `T=event.DomainEvent`, the interface requires exactly those signatures. ✅
   - `*memory.EventStore[T]` and `*pg.EventStore[T]` methods automatically satisfy `domain/event.EventStore[T]` since method signatures are identical. ✅
   - `*cqevent.EventStore[event.DomainEvent]` in `infra/repository/pg/repository.go` — this is the concrete PG struct type, not the interface. The import `cqevent "github.com/ddd-qce/core/cqrs/event/pg"` uses the pg package's struct, not the interface. The type alias at `cqrs/event` package level does not affect `cqrs/event/pg`'s own `EventStore[T]` struct definition. ✅

4. **No breaking changes for consumers:**
   - `cqevent.EventStore[T]` still works (type alias) ✅
   - `DomainEventAppendOnlyStore` is deleted — but only 2 references existed (`domain/event` definition + `DomainEventStore` check), both updated ✅
   - `DomainEventStore` concrete type is preserved, only its interface check changes ✅
   - exampleapp uses `*eventmemory.DomainEventStore` directly — no change needed ✅
   - `infra/repository/pg` uses `*cqevent.EventStore[event.DomainEvent]` (concrete PG struct) — no change needed ✅

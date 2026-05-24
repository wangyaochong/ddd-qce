# Entity ID/Version Immutability Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `Entity.ID` and `AggregateRoot.Version` unexported (private) fields, accessible only via getters, to enforce DDD identity-immutability invariant.

**Architecture:** Change `Entity.ID string` → `Entity.id string` with existing `GetID()` getter. Change `AggregateRoot.Version int` → `AggregateRoot.version int` with new `GetVersion()` getter. Add `SetSnapshotVersion(int)` exported method for pg repository snapshot loading. Update all ~50 external access sites across 4 modules.

**Tech Stack:** Go 1.26, no new dependencies

---

## Impact Summary

### Fields to privatize
| Struct | Field | New name | Getter | Setter |
|--------|-------|----------|--------|--------|
| `Entity` | `ID string` | `id string` | `GetID()` (exists) | — (constructor only) |
| `AggregateRoot` | `Version int` | `version int` | `GetVersion()` (new) | `SetSnapshotVersion(int)` (new, for infra) |

### Files requiring changes (by module)

**core module (domain/entity, domain/aggregate):**
- `domain/entity/entity.go` — field rename, update internal refs
- `domain/entity/id.go` — `Entity{ID: ...}` → `Entity{id: ...}`
- `domain/entity/auditable.go` — `Entity{ID: ...}` → `Entity{id: ...}`
- `domain/entity/entity_test.go` — `.ID` → `.GetID()`
- `domain/entity/id_test.go` — `.ID` → `.GetID()`
- `domain/entity/auditable_test.go` — `.ID` → `.GetID()`
- `domain/entity/soft_deletable_test.go` — `.ID` → `.GetID()`
- `domain/aggregate/aggregate.go` — field rename, add `GetVersion()`/`SetSnapshotVersion()`, update internal refs
- `domain/aggregate/aggregate_test.go` — `.ID` → `.GetID()`, `.Version` → `.GetVersion()`, `agg.Version = -1` → use helper

**core module (infra):**
- `infra/repository/pg/repository.go` — `root.ID` → `root.GetID()`, `root.Version` → `root.GetVersion()`, `root.Version = version` → `root.SetSnapshotVersion(version)`

**exampleapp module:**
- `exampleapp/domain/model.go` — `entity.Entity{ID: id}` → constructor; `o.ID` → `o.GetID()` (8 places)
- `exampleapp/domain/domain_test.go` — `item.ID` → `item.GetID()`, `.AggregateRoot.Version` → `.AggregateRoot.GetVersion()`, `entity.Entity{ID: ""}` → use `entity.NewEntity("")`
- `exampleapp/application/command_handlers.go` — `order.ID` → `order.GetID()` (3 places)
- `exampleapp/application/repositories.go` — `order.ID` → `order.GetID()`, `order.Version` → `order.GetVersion()`
- `exampleapp/application/application_test.go` — `found.ID` → `found.GetID()`, `.AggregateRoot.Version` → `.AggregateRoot.GetVersion()`
- `exampleapp/integration/integration_test.go` — `loaded.ID` → `loaded.GetID()` (2 places)

**NOT modified (separate types, not Entity.ID):**
- `job/core/job.go` — `Job.ID` is its own field
- `trace/span.go` — `Span.ID` is its own field
- `domain/repository/repository_test.go` — `TestAggregate.ID` is a shadowing field on test struct
- `exampleapp/infrastructure/infrastructure_test.go` — `testDomainEvent.ID` and `snap.ID` are test-local
- `it/job_pg/job_store_test.go` — `got.ID` is `Job.ID`

---

### Task 1: Privatize Entity.ID and update entity package

**Files:**
- Modify: `domain/entity/entity.go`
- Modify: `domain/entity/id.go`
- Modify: `domain/entity/auditable.go`
- Modify: `domain/entity/entity_test.go`
- Modify: `domain/entity/id_test.go`
- Modify: `domain/entity/auditable_test.go`
- Modify: `domain/entity/soft_deletable_test.go`

- [ ] **Step 1: Rename `ID` to `id` in Entity struct**

In `domain/entity/entity.go`:
```go
type Entity struct {
	id string
}

func NewEntity(id string) *Entity {
	return &Entity{id: id}
}

func (e *Entity) GetID() string {
	return e.id
}

func (e *Entity) Equals(other *Entity) bool {
	if e == nil || other == nil {
		return e == other
	}
	return e.id == other.id
}

func (e *Entity) IsEmpty() bool {
	return e.id == ""
}

func (e *Entity) Validate() error {
	if e.id == "" {
		return fmt.Errorf("entity ID cannot be empty")
	}
	return nil
}
```

- [ ] **Step 2: Update `id.go`**

Change `&Entity{ID: gen()}` → `&Entity{id: gen()}`

- [ ] **Step 3: Update `auditable.go`**

Change `Entity: Entity{ID: id}` → `Entity: Entity{id: id}`

- [ ] **Step 4: Update all test files**

In `entity_test.go`: `e.ID` → `e.GetID()`
In `id_test.go`: all `e.ID` → `e.GetID()`
In `auditable_test.go`: `e.ID` → `e.GetID()`
In `soft_deletable_test.go`: `e.ID` → `e.GetID()`

- [ ] **Step 5: Run entity package tests**

Run: `go test ./domain/entity/... -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add domain/entity/
git commit -m "refactor: make Entity.ID private, use GetID() accessor"
```

---

### Task 2: Privatize AggregateRoot.Version and update aggregate package

**Files:**
- Modify: `domain/aggregate/aggregate.go`
- Modify: `domain/aggregate/aggregate_test.go`

- [ ] **Step 1: Rename `Version` to `version` in AggregateRoot struct and add accessors**

In `domain/aggregate/aggregate.go`:
```go
type AggregateRoot struct {
	entity.Entity
	version           int
	uncommittedEvents []event.DomainEvent
	applier           EventApplier
	skipApplierCheck  bool
}

func (a *AggregateRoot) GetVersion() int {
	return a.version
}

// SetSnapshotVersion sets the aggregate version from a loaded snapshot.
// This should only be called by infrastructure (repository) when restoring from persistence.
func (a *AggregateRoot) SetSnapshotVersion(v int) {
	a.version = v
}

func (a *AggregateRoot) forceSetVersion(v int) {
	a.version = v
}
```

Update all internal references:
- Constructors: `Version: 0` → remove (zero value default)
- `a.Version++` → `a.version++`
- `a.Version < 0` → `a.version < 0`

- [ ] **Step 2: Update `aggregate_test.go`**

- All `agg.ID` → `agg.GetID()`
- All `agg.Version` / `order.Version` / `rebuilt.Version` / `o.Version` / `ar.Version` → `.GetVersion()`
- `agg.Version = -1` → `agg.forceSetVersion(-1)` (line 172)

- [ ] **Step 3: Run aggregate package tests**

Run: `go test ./domain/aggregate/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add domain/aggregate/
git commit -m "refactor: make AggregateRoot.Version private, use GetVersion() accessor"
```

---

### Task 3: Update infra/repository/pg to use accessors

**Files:**
- Modify: `infra/repository/pg/repository.go`

- [ ] **Step 1: Replace all `root.ID` → `root.GetID()`, `root.Version` → `root.GetVersion()`**

In `PgRepository.Save` (lines 72, 82):
```go
root.GetID(), r.typeName, data, root.GetVersion(), time.Now(),
return &OptimisticLockError{AggregateID: root.GetID(), ExpectedVersion: root.GetVersion()}
```

In `PgEventSourcedRepository.Save` (lines 183, 186):
```go
if err := r.eventStore.Append(ctx, root.GetID(), root.GetVersion()-len(events), typedEvents); err != nil {
if r.snapshotEvery > 0 && root.GetVersion()%r.snapshotEvery == 0 {
```

In `saveSnapshot` (lines 237, 247):
```go
root.GetID(), r.typeName, data, root.GetVersion(), time.Now(),
return &OptimisticLockError{AggregateID: root.GetID(), ExpectedVersion: root.GetVersion()}
```

- [ ] **Step 2: Replace `root.Version = version` → `root.SetSnapshotVersion(version)`**

In `PgEventSourcedRepository.Load` (line 208):
```go
root.SetSnapshotVersion(version)
```

- [ ] **Step 3: Run infra tests**

Run: `go test ./infra/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add infra/repository/pg/
git commit -m "refactor: use GetID()/GetVersion()/SetSnapshotVersion() in pg repository"
```

---

### Task 4: Update exampleapp domain layer

**Files:**
- Modify: `exampleapp/domain/model.go`
- Modify: `exampleapp/domain/domain_test.go`

- [ ] **Step 1: Update `model.go`**

Change `entity.Entity{ID: id}` → `*entity.NewEntity(id)` in `NewOrderItem`:
```go
func NewOrderItem(id, productName string, price float64, quantity int) *OrderItem {
	return &OrderItem{
		Entity:      *entity.NewEntity(id),
		ProductName: productName,
		Price:       price,
		Quantity:    quantity,
	}
}
```

Change all `o.ID` → `o.GetID()` (8 places: lines 67, 109, 112, 120, 123, 131, 134, 137)

- [ ] **Step 2: Update `domain_test.go`**

- `item.ID != "p1"` → `item.GetID() != "p1"` (line 150)
- `order.AggregateRoot.Version` → `order.AggregateRoot.GetVersion()` (lines 76-77, 89-90)
- `ar.Version` → `ar.GetVersion()` (lines 100-101)
- `&OrderItem{Entity: entity.Entity{ID: ""}}` → `NewOrderItem("", "", 0, 0)` (line 171)

- [ ] **Step 3: Run domain tests**

Run: `go test ./exampleapp/domain/... -v`
Expected: PASS

- [ ] **Step 4: Commit**

```bash
git add exampleapp/domain/
git commit -m "refactor: use GetID()/GetVersion() accessors in exampleapp domain"
```

---

### Task 5: Update exampleapp application layer

**Files:**
- Modify: `exampleapp/application/command_handlers.go`
- Modify: `exampleapp/application/repositories.go`
- Modify: `exampleapp/application/application_test.go`

- [ ] **Step 1: Update `command_handlers.go`**

Change `order.ID` → `order.GetID()` (3 places: lines 41, 48, 119)

- [ ] **Step 2: Update `repositories.go`**

- `r.orders[order.ID]` → `r.orders[order.GetID()]` (line 25)
- `order.ID` → `order.GetID()` (2 on line 79)
- `order.Version` → `order.GetVersion()` (1 on line 79)

- [ ] **Step 3: Update `application_test.go`**

- `found.ID` → `found.GetID()` (line 201)
- `loaded.ID` → `loaded.GetID()` (line 231)
- `loaded.AggregateRoot.Version` → `loaded.AggregateRoot.GetVersion()` (lines 255-256, 282-283)

- [ ] **Step 4: Run application tests**

Run: `go test ./exampleapp/application/... -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add exampleapp/application/
git commit -m "refactor: use GetID()/GetVersion() accessors in exampleapp application"
```

---

### Task 6: Update exampleapp integration tests

**Files:**
- Modify: `exampleapp/integration/integration_test.go`

- [ ] **Step 1: Update `integration_test.go`**

- `loaded.ID != "ORD-ES-FULL"` → `loaded.GetID() != "ORD-ES-FULL"` (lines 83-84)
- `found.ID != "ORD-DEL"` → `found.GetID() != "ORD-DEL"` (line 216)

Note: `job.ID` references are `Job.ID` — no change.

- [ ] **Step 2: Run integration tests**

Run: `go test ./exampleapp/integration/... -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add exampleapp/integration/
git commit -m "refactor: use GetID() accessor in exampleapp integration tests"
```

---

### Task 7: Full build and test verification

- [ ] **Step 1: Run full core module tests**

Run: `go test ./... -count=1`
Expected: All PASS

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: No issues

---

## Summary of API Changes

| Before | After | Context |
|--------|-------|---------|
| `entity.ID` | `entity.GetID()` | Read entity identity |
| `aggregateRoot.Version` | `aggregateRoot.GetVersion()` | Read aggregate version |
| `entity.Entity{ID: x}` | `entity.NewEntity(x)` / `*entity.NewEntity(x)` | Construct entity |
| `root.Version = v` | `root.SetSnapshotVersion(v)` | Restore from DB snapshot |

## Key Design Decisions

1. **`SetSnapshotVersion` is exported** — needed by `infra/repository/pg` (different package) to restore version from DB snapshots. Name makes intent clear: infrastructure-only, not business mutation.

2. **`forceSetVersion` unexported** — used only within `domain/aggregate` package tests to set negative version for validation testing.

3. **`TestAggregate.ID` in `repository_test.go` left as-is** — shadowing field on test struct, unrelated to `Entity.id`.

4. **`Job.ID`, `Span.ID`, etc. NOT changed** — independent structs, not part of Entity hierarchy.

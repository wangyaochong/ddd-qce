# Eliminate unsafe.Pointer Deep Copy — JSON Serializer Unification

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `unsafe.Pointer`-based deep copy in the in-memory repository with JSON-based serialization, unifying the deep copy mechanism with the PG repository's `SnapshotSerializer` pattern.

**Architecture:** Move `SnapshotSerializer`/`JSONSerializer` from `infra/repository/pg` to `domain/repository` (shared layer). Add `ToJSON`/`FromJSON` helper methods on `Entity` and `AggregateRoot` so business aggregates can implement `MarshalJSON`/`UnmarshalJSON` with minimal boilerplate. Replace the 5 unsafe functions in the memory repository with a single `deepCopy` using the shared serializer. Both Save and FindByID/Load in the memory repository will use serializer round-trips for isolation.

**Tech Stack:** Go 1.22+, `encoding/json`, existing project conventions

---

## File Structure

| File | Action | Responsibility |
|------|--------|----------------|
| `core/domain/entity/entity.go` | Modify | Add `EntityJSON`, `ToJSON()`, `FromJSON()` |
| `core/domain/aggregate/aggregate.go` | Modify | Add `AggregateRootJSON`, `ToJSON()`, `FromJSON()`, `SetApplier()` |
| `core/domain/repository/repository.go` | Modify | Add `SnapshotSerializer[T]` interface, `JSONSerializer[T]` struct |
| `core/infra/repository/memory/repository.go` | Modify | Delete 5 unsafe functions, add `serializer` field, rewrite Save/FindByID/Load |
| `core/infra/repository/pg/repository.go` | Modify | Remove `SnapshotSerializer`/`JSONSerializer` definitions, import from `repository` |
| `exampleapp/domain/model.go` | Modify | Add `MarshalJSON`/`UnmarshalJSON` on `Order` and `OrderItem` |
| `core/infra/repository/memory/repository_test.go` | Modify | Add `MarshalJSON`/`UnmarshalJSON` on `testAggregate` |
| `core/domain/repository/repositorytest/repository_contract.go` | Modify | Add `MarshalJSON`/`UnmarshalJSON` on `TestAggregate` |
| `it/infra_pg/repository_test.go` | Modify | Refactor `testOrder` JSON methods to use `ToJSON`/`FromJSON` pattern |
| `cmd/ddd/generator/generator.go` | Modify | Update `domainModelTmpl` to include `MarshalJSON`/`UnmarshalJSON` |

---

### Task 1: Add `EntityJSON`/`ToJSON`/`FromJSON` to Entity

**Files:**
- Modify: `core/domain/entity/entity.go`

- [ ] **Step 1: Add EntityJSON, ToJSON, FromJSON after the UnmarshalJSON method (after line 65)**

```go
type EntityJSON struct {
	ID string `json:"id"`
}

func (e *Entity) ToJSON() EntityJSON {
	return EntityJSON{ID: e.id}
}

func (e *Entity) FromJSON(j EntityJSON) {
	e.id = j.ID
}
```

- [ ] **Step 2: Run entity tests**

Run: `go test ./core/domain/entity/... -v`
Expected: PASS (existing tests unaffected, new methods not yet called)

- [ ] **Step 3: Commit**

```bash
git add core/domain/entity/entity.go
git commit -m "feat(entity): add EntityJSON/ToJSON/FromJSON for JSON serialization helpers"
```

---

### Task 2: Add `AggregateRootJSON`/`ToJSON`/`FromJSON`/`SetApplier` to AggregateRoot

**Files:**
- Modify: `core/domain/aggregate/aggregate.go`

- [ ] **Step 1: Add AggregateRootJSON, ToJSON, FromJSON, SetApplier after the Clone method (after line 141)**

```go
type AggregateRootJSON struct {
	ID               string `json:"id"`
	Version          int    `json:"version"`
	SkipApplierCheck bool   `json:"skipApplierCheck,omitempty"`
}

func (a *AggregateRoot) ToJSON() AggregateRootJSON {
	return AggregateRootJSON{
		ID:               a.ID(),
		Version:          a.version,
		SkipApplierCheck: a.skipApplierCheck,
	}
}

func (a *AggregateRoot) FromJSON(j AggregateRootJSON) {
	a.Entity = *entity.NewEntity(j.ID)
	a.version = j.Version
	a.skipApplierCheck = j.SkipApplierCheck
}

func (a *AggregateRoot) SetApplier(applier EventApplier) {
	a.applier = applier
}
```

- [ ] **Step 2: Run aggregate tests**

Run: `go test ./core/domain/aggregate/... -v`
Expected: PASS (existing tests unaffected, SetApplier already used internally)

- [ ] **Step 3: Commit**

```bash
git add core/domain/aggregate/aggregate.go
git commit -m "feat(aggregate): add AggregateRootJSON/ToJSON/FromJSON/SetApplier for serialization"
```

---

### Task 3: Move `SnapshotSerializer`/`JSONSerializer` to shared `domain/repository` package

**Files:**
- Modify: `core/domain/repository/repository.go`
- Modify: `core/infra/repository/pg/repository.go`

- [ ] **Step 1: Add SnapshotSerializer and JSONSerializer to repository.go**

Replace the entire file with:

```go
package repository

import (
	"context"
	"encoding/json"

	"github.com/ddd-qce/core/domain/aggregate"
)

type Repository[T any] interface {
	Save(ctx context.Context, aggregate T) error
	FindByID(ctx context.Context, id string) (T, error)
	Delete(ctx context.Context, id string) error
}

type EventSourcingRepository[T any] interface {
	Save(ctx context.Context, aggregate T) error
	Load(ctx context.Context, id string) (T, error)
}

type SnapshotSerializer[T aggregate.AggregateRef] interface {
	Serialize(agg T) ([]byte, error)
	Deserialize(data []byte) (T, error)
}

type JSONSerializer[T aggregate.AggregateRef] struct{}

func (JSONSerializer[T]) Serialize(agg T) ([]byte, error) { return json.Marshal(agg) }
func (JSONSerializer[T]) Deserialize(data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	return v, err
}
```

- [ ] **Step 2: Update pg/repository.go — remove local SnapshotSerializer/JSONSerializer, use repository.SnapshotSerializer**

Remove the local `SnapshotSerializer` interface definition (lines 18-21) and `JSONSerializer` struct (lines 23-30).

Add `"github.com/ddd-qce/core/domain/repository"` to imports.

Remove `"encoding/json"` from pg imports (no longer used directly).

Replace all type references:
- `SnapshotSerializer[T]` → `repository.SnapshotSerializer[T]`
- `JSONSerializer[T]{}` → `repository.JSONSerializer[T]{}`

Update struct fields:
```go
type PgRepository[T aggregate.AggregateRef] struct {
	db         *sql.DB
	serializer repository.SnapshotSerializer[T]
	typeName   string
}

type PgEventSourcedRepository[T aggregate.AggregateRef] struct {
	db            *sql.DB
	eventStore    event.EventSourceStore[event.Event]
	reconstructor AggregateReconstructor[T]
	serializer    repository.SnapshotSerializer[T]
	typeName      string
	snapshotEvery int
}
```

Update option functions:
```go
func WithRepoSerializer[T aggregate.AggregateRef](s repository.SnapshotSerializer[T]) RepoOption[T] {
	return func(r *PgRepository[T]) { r.serializer = s }
}

func WithSerializer[T aggregate.AggregateRef](s repository.SnapshotSerializer[T]) EventSourcedRepoOption[T] {
	return func(r *PgEventSourcedRepository[T]) { r.serializer = s }
}
```

In constructors, replace `JSONSerializer[T]{}` with `repository.JSONSerializer[T]{}`.

- [ ] **Step 3: Run PG repository unit test compilation check**

Run: `go build ./core/infra/repository/pg/...`
Expected: No compilation errors

- [ ] **Step 4: Commit**

```bash
git add core/domain/repository/repository.go core/infra/repository/pg/repository.go
git commit -m "refactor: move SnapshotSerializer/JSONSerializer to shared domain/repository package"
```

---

### Task 4: Add MarshalJSON/UnmarshalJSON to test aggregates

All test aggregates that go through repositories (memory or PG) must implement JSON serialization. This must be done before rewriting the memory repository (Task 5).

**Files:**
- Modify: `core/infra/repository/memory/repository_test.go`
- Modify: `core/domain/repository/repositorytest/repository_contract.go`
- Modify: `it/infra_pg/repository_test.go`

- [ ] **Step 1: Add MarshalJSON/UnmarshalJSON to memory testAggregate**

Add `"encoding/json"` to imports.

Add after the `When` method (after line 29):

```go
func (a *testAggregate) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		aggregate.AggregateRootJSON
		Name  string `json:"name"`
		Count int    `json:"count"`
	}{
		AggregateRootJSON: a.AggregateRoot.ToJSON(),
		Name:              a.Name,
		Count:             a.Count,
	})
}

func (a *testAggregate) UnmarshalJSON(data []byte) error {
	var aux struct {
		aggregate.AggregateRootJSON
		Name  string `json:"name"`
		Count int    `json:"count"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	a.AggregateRoot.FromJSON(aux.AggregateRootJSON)
	a.AggregateRoot.SetApplier(a)
	a.Name = aux.Name
	a.Count = aux.Count
	return nil
}
```

- [ ] **Step 2: Add MarshalJSON/UnmarshalJSON to contract TestAggregate**

Add `"encoding/json"` to imports.

Add after the `When` method (after line 26):

```go
func (a *TestAggregate) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		aggregate.AggregateRootJSON
		Name   string `json:"name"`
		Amount int    `json:"amount"`
	}{
		AggregateRootJSON: a.AggregateRoot.ToJSON(),
		Name:              a.Name,
		Amount:            a.Amount,
	})
}

func (a *TestAggregate) UnmarshalJSON(data []byte) error {
	var aux struct {
		aggregate.AggregateRootJSON
		Name   string `json:"name"`
		Amount int    `json:"amount"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	a.AggregateRoot.FromJSON(aux.AggregateRootJSON)
	a.AggregateRoot.SetApplier(a)
	a.Name = aux.Name
	a.Amount = aux.Amount
	return nil
}
```

- [ ] **Step 3: Refactor PG integration test testOrder**

Replace the existing `testOrderJSON` struct and `MarshalJSON`/`UnmarshalJSON` methods (lines 37-60) with:

```go
func (o *testOrder) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		aggregate.AggregateRootJSON
		Name   string  `json:"name"`
		Amount float64 `json:"amount"`
	}{
		AggregateRootJSON: o.AggregateRoot.ToJSON(),
		Name:              o.Name,
		Amount:            o.Amount,
	})
}

func (o *testOrder) UnmarshalJSON(data []byte) error {
	var aux struct {
		aggregate.AggregateRootJSON
		Name   string  `json:"name"`
		Amount float64 `json:"amount"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	o.AggregateRoot.FromJSON(aux.AggregateRootJSON)
	o.AggregateRoot.SetApplier(o)
	o.Name = aux.Name
	o.Amount = aux.Amount
	return nil
}
```

Delete the `testOrderJSON` struct (lines 37-41) as it is no longer needed.

- [ ] **Step 4: Verify all test files compile**

Run: `go build ./core/infra/repository/memory/... ./core/domain/repository/repositorytest/... ./it/...`
Expected: No compilation errors

- [ ] **Step 5: Commit**

```bash
git add core/infra/repository/memory/repository_test.go core/domain/repository/repositorytest/repository_contract.go it/infra_pg/repository_test.go
git commit -m "feat: add MarshalJSON/UnmarshalJSON to test aggregates using ToJSON/FromJSON pattern"
```

---

### Task 5: Rewrite memory repository — replace unsafe with JSON serializer

**Files:**
- Modify: `core/infra/repository/memory/repository.go`

This is the core change. Delete all 5 unsafe functions and rewrite Save/FindByID/Load to use `repository.SnapshotSerializer`.

- [ ] **Step 1: Rewrite repository.go completely**

The new file content:

```go
package memory

import (
	"context"
	"fmt"
	"sync"

	ddderror "github.com/ddd-qce/core/error"
	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/domain/repository"
	rep "github.com/ddd-qce/core/infra/repository"
)

type aggregateRecord[T aggregate.AggregateRef] struct {
	agg     T
	version int
}

type InMemoryRepository[T aggregate.AggregateRef] struct {
	mu         sync.RWMutex
	store      map[string]*aggregateRecord[T]
	serializer repository.SnapshotSerializer[T]
}

var _ repository.Repository[aggregate.AggregateRef] = (*InMemoryRepository[aggregate.AggregateRef])(nil)

func NewRepository[T aggregate.AggregateRef]() *InMemoryRepository[T] {
	return &InMemoryRepository[T]{
		store:      make(map[string]*aggregateRecord[T]),
		serializer: repository.JSONSerializer[T]{},
	}
}

func (r *InMemoryRepository[T]) Save(_ context.Context, agg T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	root := agg.GetAggregateRoot()

	if existing, ok := r.store[root.ID()]; ok {
		if root.Version() <= existing.version {
			return &rep.OptimisticLockError{AggregateID: root.ID(), ExpectedVersion: root.Version()}
		}
	}

	copied, err := deepCopy(r.serializer, agg)
	if err != nil {
		return fmt.Errorf("deep copy on save: %w", err)
	}

	r.store[root.ID()] = &aggregateRecord[T]{
		agg:     copied,
		version: root.Version(),
	}
	return nil
}

func (r *InMemoryRepository[T]) FindByID(_ context.Context, id string) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rec, ok := r.store[id]
	if !ok {
		var zero T
		return zero, fmt.Errorf("aggregate %s: %w", id, ddderror.ErrNotFound)
	}

	copied, err := deepCopy(r.serializer, rec.agg)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("deep copy on find: %w", err)
	}

	copied.GetAggregateRoot().SetSnapshotVersion(rec.version)
	return copied, nil
}

func (r *InMemoryRepository[T]) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.store[id]; !ok {
		return fmt.Errorf("aggregate %s: %w", id, ddderror.ErrNotFound)
	}
	delete(r.store, id)
	return nil
}

type InMemoryEventSourcedRepository[T aggregate.AggregateRef] struct {
	mu         sync.RWMutex
	store      map[string]*aggregateRecord[T]
	serializer repository.SnapshotSerializer[T]
}

var _ repository.EventSourcingRepository[aggregate.AggregateRef] = (*InMemoryEventSourcedRepository[aggregate.AggregateRef])(nil)

func NewEventSourcedRepository[T aggregate.AggregateRef]() *InMemoryEventSourcedRepository[T] {
	return &InMemoryEventSourcedRepository[T]{
		store:      make(map[string]*aggregateRecord[T]),
		serializer: repository.JSONSerializer[T]{},
	}
}

func (r *InMemoryEventSourcedRepository[T]) Save(_ context.Context, agg T) error {
	root := agg.GetAggregateRoot()
	events := root.UncommittedEvents()
	if len(events) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	copied, err := deepCopy(r.serializer, agg)
	if err != nil {
		return fmt.Errorf("deep copy on save: %w", err)
	}

	r.store[root.ID()] = &aggregateRecord[T]{
		agg:     copied,
		version: root.Version(),
	}
	root.MarkEventsAsCommitted()
	return nil
}

func (r *InMemoryEventSourcedRepository[T]) Load(_ context.Context, id string) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	rec, ok := r.store[id]
	if !ok {
		var zero T
		return zero, fmt.Errorf("aggregate %s: %w", id, ddderror.ErrNotFound)
	}

	copied, err := deepCopy(r.serializer, rec.agg)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("deep copy on load: %w", err)
	}

	copied.GetAggregateRoot().SetSnapshotVersion(rec.version)
	return copied, nil
}

func deepCopy[T aggregate.AggregateRef](serializer repository.SnapshotSerializer[T], src T) (T, error) {
	data, err := serializer.Serialize(src)
	if err != nil {
		var zero T
		return zero, err
	}
	copied, err := serializer.Deserialize(data)
	if err != nil {
		var zero T
		return zero, err
	}
	return copied, nil
}
```

Key changes:
- **Deleted**: `deepCopyByReflection`, `applyClonedRoot`, `deepCopyByUnsafe`, `copyAllFields`, `unsafeReflectValue` (5 functions)
- **Deleted**: imports `"reflect"`, `"unsafe"`
- **Added**: `serializer` field on both repo structs, initialized to `repository.JSONSerializer[T]{}`
- **Added**: `deepCopy` generic function using `SnapshotSerializer`
- **Changed**: `Save` now deep-copies before storing (was storing raw reference)
- **Changed**: `FindByID`/`Load` now deep-copy via serializer + `SetSnapshotVersion`

- [ ] **Step 2: Run memory repository tests**

Run: `go test ./core/infra/repository/memory/... -v`
Expected: ALL PASS

- [ ] **Step 3: Commit**

```bash
git add core/infra/repository/memory/repository.go
git commit -m "refactor(memory): replace unsafe.Pointer deep copy with JSON serializer

Eliminates all unsafe.Pointer usage in the in-memory repository.
Both Save and FindByID/Load now use SnapshotSerializer for isolation,
matching the PG repository's serialization-based approach."
```

---

### Task 6: Add MarshalJSON/UnmarshalJSON to exampleapp Order and OrderItem

**Files:**
- Modify: `exampleapp/domain/model.go`

- [ ] **Step 1: Add `"encoding/json"` to imports**

- [ ] **Step 2: Add MarshalJSON/UnmarshalJSON to OrderItem (after Subtotal method)**

```go
func (i *OrderItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		entity.EntityJSON
		ProductName string  `json:"productName"`
		Price       float64 `json:"price"`
		Quantity    int     `json:"quantity"`
	}{
		EntityJSON:  i.Entity.ToJSON(),
		ProductName: i.ProductName,
		Price:       i.Price,
		Quantity:    i.Quantity,
	})
}

func (i *OrderItem) UnmarshalJSON(data []byte) error {
	var aux struct {
		entity.EntityJSON
		ProductName string  `json:"productName"`
		Price       float64 `json:"price"`
		Quantity    int     `json:"quantity"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	i.Entity.FromJSON(aux.EntityJSON)
	i.ProductName = aux.ProductName
	i.Price = aux.Price
	i.Quantity = aux.Quantity
	return nil
}
```

- [ ] **Step 3: Add MarshalJSON/UnmarshalJSON to Order (after ItemNames method)**

```go
func (o *Order) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		aggregate.AggregateRootJSON
		UserID       string       `json:"userId"`
		Items        []*OrderItem `json:"items"`
		Status       OrderStatus  `json:"status"`
		TotalAmount  float64      `json:"totalAmount"`
		CreatedAt    time.Time    `json:"createdAt"`
		PaidAt       time.Time    `json:"paidAt"`
		ShippedAt    time.Time    `json:"shippedAt"`
		CancelledAt  time.Time    `json:"cancelledAt"`
		CancelReason string       `json:"cancelReason"`
	}{
		AggregateRootJSON: o.AggregateRoot.ToJSON(),
		UserID:            o.UserID,
		Items:             o.Items,
		Status:            o.Status,
		TotalAmount:       o.TotalAmount,
		CreatedAt:         o.CreatedAt,
		PaidAt:            o.PaidAt,
		ShippedAt:         o.ShippedAt,
		CancelledAt:       o.CancelledAt,
		CancelReason:      o.CancelReason,
	})
}

func (o *Order) UnmarshalJSON(data []byte) error {
	var aux struct {
		aggregate.AggregateRootJSON
		UserID       string       `json:"userId"`
		Items        []*OrderItem `json:"items"`
		Status       OrderStatus  `json:"status"`
		TotalAmount  float64      `json:"totalAmount"`
		CreatedAt    time.Time    `json:"createdAt"`
		PaidAt       time.Time    `json:"paidAt"`
		ShippedAt    time.Time    `json:"shippedAt"`
		CancelledAt  time.Time    `json:"cancelledAt"`
		CancelReason string       `json:"cancelReason"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	o.AggregateRoot.FromJSON(aux.AggregateRootJSON)
	o.AggregateRoot.SetApplier(o)
	o.UserID = aux.UserID
	o.Items = aux.Items
	o.Status = aux.Status
	o.TotalAmount = aux.TotalAmount
	o.CreatedAt = aux.CreatedAt
	o.PaidAt = aux.PaidAt
	o.ShippedAt = aux.ShippedAt
	o.CancelledAt = aux.CancelledAt
	o.CancelReason = aux.CancelReason
	return nil
}
```

- [ ] **Step 4: Run exampleapp build check**

Run: `go build ./exampleapp/...`
Expected: No compilation errors

- [ ] **Step 5: Commit**

```bash
git add exampleapp/domain/model.go
git commit -m "feat(exampleapp): add MarshalJSON/UnmarshalJSON to Order and OrderItem"
```

---

### Task 7: Update code generator template

**Files:**
- Modify: `cmd/ddd/generator/generator.go`

- [ ] **Step 1: Update domainModelTmpl**

In the `domainModelTmpl` string, add `"encoding/json"` to the import list.

After the `Subtotal()` method of `{{.Name}}Item`, add MarshalJSON/UnmarshalJSON for `{{.Name}}Item` using `entity.EntityJSON` pattern.

After the `calculateTotal()` method of `{{.Name}}`, add MarshalJSON/UnmarshalJSON for `{{.Name}}` using `aggregate.AggregateRootJSON` pattern, with `SetApplier(o)` in UnmarshalJSON.

The template code follows the exact same pattern as Task 6, with `{{.Name}}` and `{{.Name}}Item` substituted.

- [ ] **Step 2: Run generator build check**

Run: `go build ./cmd/ddd/...`
Expected: No compilation errors

- [ ] **Step 3: Commit**

```bash
git add cmd/ddd/generator/generator.go
git commit -m "feat(generator): add MarshalJSON/UnmarshalJSON to scaffolded aggregates"
```

---

### Task 8: Add deep copy isolation tests

**Files:**
- Modify: `core/infra/repository/memory/repository_test.go`

- [ ] **Step 1: Add test for applier self-reference after deep copy**

```go
func TestInMemoryRepository_DeepCopyApplierSelfReference(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("agg-applier")
	agg.Name = "original"
	if err := agg.Apply(event.NewBaseEvent("agg-applier", time.Now())); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, err := repo.FindByID(ctx, "agg-applier")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if err := found.Apply(event.NewBaseEvent("agg-applier", time.Now())); err != nil {
		t.Fatalf("Apply on found: %v", err)
	}

	original, err := repo.FindByID(ctx, "agg-applier")
	if err != nil {
		t.Fatalf("FindByID original: %v", err)
	}

	if original.Version() != 1 {
		t.Errorf("original version should still be 1 (from Save), got %d — applier is mutating the store copy", original.Version())
	}
}

func TestInMemoryRepository_DeepCopySaveIsolation(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("agg-isolate")
	agg.Name = "original"
	if err := agg.Apply(event.NewBaseEvent("agg-isolate", time.Now())); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	agg.Name = "modified-after-save"

	found, err := repo.FindByID(ctx, "agg-isolate")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if found.Name != "original" {
		t.Errorf("FindByID should return 'original', got %q — Save is not isolating the stored copy", found.Name)
	}
}
```

- [ ] **Step 2: Run memory repository tests**

Run: `go test ./core/infra/repository/memory/... -v -run TestInMemoryRepository_DeepCopy`
Expected: ALL PASS

- [ ] **Step 3: Commit**

```bash
git add core/infra/repository/memory/repository_test.go
git commit -m "test(memory): add deep copy isolation tests for applier and save"
```

---

### Task 9: Run full test suite and verify no unsafe remains

- [ ] **Step 1: Run all core tests**

Run: `go test ./core/... -v`
Expected: ALL PASS

- [ ] **Step 2: Run exampleapp tests**

Run: `go test ./exampleapp/... -v`
Expected: ALL PASS

- [ ] **Step 3: Verify no unsafe usage remains in core**

Run: `grep -r "unsafe\." core/ --include="*.go"`
Expected: No matches

- [ ] **Step 4: Final commit if any fixes needed**

```bash
git add -A
git commit -m "chore: fix any remaining test issues after unsafe elimination"
```

# Contract Tests & Memory Implementations Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add shared interface contract tests for all dual-implementation interfaces, implement missing memory implementations (InMemoryMessageStore for `builtin.MessageStore`, InMemoryRepository), and replace `NopMessageStore` as the memory backend default.

**Architecture:** Create `*test` sub-packages under each interface package (e.g., `job/core/jobtest`, `trace/tracetest`, `aspect/builtin/builtintest`, `cqrs/event/eventtest`, `domain/repository/repositorytest`, `infra/infratest`) that export `TestXxxContract(t, impl)` functions. Both memory and PG test files call the same contract function, ensuring behavioral parity. New memory implementations (InMemoryMessageStore in `aspect/builtin` package, InMemoryRepository in `infra/repository/memory`) are created before their contract tests.

**Tech Stack:** Go stdlib `testing`, no external test frameworks. Follows existing project conventions (pure `testing.T`, `t.Fatalf`/`t.Errorf`, `t.Helper()`, `t.Cleanup()`, `t.Run()`).

---

## File Structure

### New Files to Create

| File | Responsibility |
|------|---------------|
| `aspect/builtin/in_memory_message_store.go` | Full `InMemoryMessageStore` implementing `builtin.MessageStore` with `RecordEventHandler` support |
| `infra/repository/memory/repository.go` | `InMemoryRepository[T]` and `InMemoryEventSourcedRepository[T]` |
| `job/core/jobtest/job_store_contract.go` | Shared `TestJobStoreContract(t, store)` |
| `trace/tracetest/trace_store_contract.go` | Shared `TestTraceStoreContract(t, store)` |
| `aspect/builtin/builtintest/message_store_contract.go` | Shared `TestMessageStoreContract(t, store)` |
| `aspect/builtin/builtintest/transaction_manager_contract.go` | Shared `TestTransactionManagerContract(t, tm, newCtx)` |
| `cqrs/event/eventtest/event_store_contract.go` | Shared `TestEventStoreContract(t, newStore)` |
| `domain/repository/repositorytest/repository_contract.go` | Shared `TestRepositoryContract(t, repo)` and `TestEventSourcingRepositoryContract(t, repo, eventStore, reconstructor)` |
| `infra/infratest/backend_contract.go` | Shared `TestBackendContract(t, backend)` verifying wiring correctness |

### Files to Modify

| File | Change |
|------|--------|
| `infra/memory_backend.go` | Replace `NopMessageStore` with `InMemoryMessageStore` in `NewMemoryBackend()` defaults |
| `job/memory/job_store_test.go` | Add `TestJobStore_Contract` calling `jobtest.TestJobStoreContract` |
| `it/job_pg/job_store_test.go` | Add `TestPgJobStore_Contract` calling `jobtest.TestJobStoreContract` |
| `trace/` (appropriate test file) | Add `TestInMemoryTraceStore_Contract` calling `tracetest.TestTraceStoreContract` |
| `it/trace_pg/trace_store_test.go` | Add `TestPgTraceStore_Contract` calling `tracetest.TestTraceStoreContract` |
| `infra/backend_test.go` | Add contract test calls for TransactionManager and Backend |
| `it/pg/transaction_test.go` | Add `TestPgTransactionManager_Contract` |
| `it/aspect_builtin_pg/message_store_test.go` | Add `TestPgMessageStore_Contract` |
| `cqrs/event/memory/event_store_test.go` | Add `TestEventStore_Contract` |
| `it/cqrs_event_pg/event_store_test.go` | Add `TestPgEventStore_Contract` |
| `infra/repository/memory/repository_test.go` | Add contract test calls |
| `it/infra_pg/repository_test.go` | Add contract test calls |

---

## Task 1: Implement InMemoryMessageStore in `aspect/builtin`

**Why:** The existing `InMemoryMessageStore` in `observability` package has an incomplete `RecordEventHandler` (returns nil, discards data). The `aspect/builtin` package owns the `MessageStore` interface and should have its own proper memory implementation. This also replaces `NopMessageStore` as the default.

**Files:**
- Create: `aspect/builtin/in_memory_message_store.go`
- Create: `aspect/builtin/in_memory_message_store_test.go`
- Modify: `infra/memory_backend.go:79-87`

- [ ] **Step 1: Write failing test for InMemoryMessageStore**

Create `aspect/builtin/in_memory_message_store_test.go`:

```go
package builtin

import (
	"context"
	"testing"
	"time"
)

func TestInMemoryMessageStore_RecordCommand(t *testing.T) {
	s := NewInMemoryMessageStore()
	entry := &CommandEntry{
		TraceID:     "trace-1",
		SpanID:      "span-1",
		CommandType: "CreateUser",
		Duration:    100 * time.Millisecond,
		CreatedAt:   time.Now(),
	}
	if err := s.RecordCommand(context.Background(), entry); err != nil {
		t.Fatalf("RecordCommand failed: %v", err)
	}
	if len(s.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(s.commands))
	}
	if s.commands[0].CommandType != "CreateUser" {
		t.Errorf("expected CommandType 'CreateUser', got %s", s.commands[0].CommandType)
	}
	if s.commands[0].TraceID != "trace-1" {
		t.Errorf("expected TraceID 'trace-1', got %s", s.commands[0].TraceID)
	}
}

func TestInMemoryMessageStore_RecordQuery(t *testing.T) {
	s := NewInMemoryMessageStore()
	entry := &QueryEntry{
		TraceID:   "trace-1",
		QueryType: "GetUser",
		Duration:  50 * time.Millisecond,
		CreatedAt: time.Now(),
	}
	if err := s.RecordQuery(context.Background(), entry); err != nil {
		t.Fatalf("RecordQuery failed: %v", err)
	}
	if len(s.queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(s.queries))
	}
	if s.queries[0].QueryType != "GetUser" {
		t.Errorf("expected QueryType 'GetUser', got %s", s.queries[0].QueryType)
	}
}

func TestInMemoryMessageStore_RecordEvent(t *testing.T) {
	s := NewInMemoryMessageStore()
	entry := &EventEntry{
		TraceID:      "trace-1",
		AggregateID:  "agg-1",
		EventType:    "UserCreated",
		HandlerCount: 2,
		Duration:     30 * time.Millisecond,
		CreatedAt:    time.Now(),
	}
	if err := s.RecordEvent(context.Background(), entry); err != nil {
		t.Fatalf("RecordEvent failed: %v", err)
	}
	if len(s.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(s.events))
	}
	if s.events[0].EventType != "UserCreated" {
		t.Errorf("expected EventType 'UserCreated', got %s", s.events[0].EventType)
	}
	if s.events[0].HandlerCount != 2 {
		t.Errorf("expected HandlerCount 2, got %d", s.events[0].HandlerCount)
	}
}

func TestInMemoryMessageStore_RecordEventHandler(t *testing.T) {
	s := NewInMemoryMessageStore()
	entry := &EventHandlerEntry{
		TraceID:     "trace-1",
		AggregateID: "agg-1",
		EventType:   "UserCreated",
		HandlerType: "SendWelcomeEmail",
		Status:      "success",
		Duration:    10 * time.Millisecond,
		CreatedAt:   time.Now(),
	}
	if err := s.RecordEventHandler(context.Background(), entry); err != nil {
		t.Fatalf("RecordEventHandler failed: %v", err)
	}
	if len(s.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(s.handlers))
	}
	if s.handlers[0].HandlerType != "SendWelcomeEmail" {
		t.Errorf("expected HandlerType 'SendWelcomeEmail', got %s", s.handlers[0].HandlerType)
	}
	if s.handlers[0].Status != "success" {
		t.Errorf("expected Status 'success', got %s", s.handlers[0].Status)
	}
}

func TestInMemoryMessageStore_InterfaceConformance(t *testing.T) {
	var _ MessageStore = (*InMemoryMessageStore)(nil)
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./aspect/builtin/ -run TestInMemoryMessageStore -v`
Expected: FAIL - `NewInMemoryMessageStore` undefined

- [ ] **Step 3: Implement InMemoryMessageStore**

Create `aspect/builtin/in_memory_message_store.go`:

```go
package builtin

import (
	"context"
	"sync"
)

type InMemoryMessageStore struct {
	mu       sync.RWMutex
	commands []CommandEntry
	queries  []QueryEntry
	events   []EventEntry
	handlers []EventHandlerEntry
	maxSize  int
}

type InMemoryMessageStoreOption func(*InMemoryMessageStore)

func WithInMemoryMaxSize(n int) InMemoryMessageStoreOption {
	return func(s *InMemoryMessageStore) { s.maxSize = n }
}

func NewInMemoryMessageStore(opts ...InMemoryMessageStoreOption) *InMemoryMessageStore {
	s := &InMemoryMessageStore{
		maxSize: 1000,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *InMemoryMessageStore) RecordCommand(_ context.Context, entry *CommandEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.commands = append(s.commands, *entry)
	if len(s.commands) > s.maxSize {
		s.commands = s.commands[len(s.commands)-s.maxSize:]
	}
	return nil
}

func (s *InMemoryMessageStore) RecordQuery(_ context.Context, entry *QueryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queries = append(s.queries, *entry)
	if len(s.queries) > s.maxSize {
		s.queries = s.queries[len(s.queries)-s.maxSize:]
	}
	return nil
}

func (s *InMemoryMessageStore) RecordEvent(_ context.Context, entry *EventEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, *entry)
	if len(s.events) > s.maxSize {
		s.events = s.events[len(s.events)-s.maxSize:]
	}
	return nil
}

func (s *InMemoryMessageStore) RecordEventHandler(_ context.Context, entry *EventHandlerEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.handlers = append(s.handlers, *entry)
	if len(s.handlers) > s.maxSize {
		s.handlers = s.handlers[len(s.handlers)-s.maxSize:]
	}
	return nil
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./aspect/builtin/ -run TestInMemoryMessageStore -v`
Expected: PASS

- [ ] **Step 5: Replace NopMessageStore with InMemoryMessageStore in NewMemoryBackend**

Edit `infra/memory_backend.go`, change line 84 from:
```go
WithMessageStore(builtin.NewNopMessageStore()),
```
to:
```go
WithMessageStore(builtin.NewInMemoryMessageStore()),
```

- [ ] **Step 6: Run existing tests to verify no regression**

Run: `go test ./infra/ -v`
Expected: PASS

- [ ] **Step 7: Commit**

```bash
git add aspect/builtin/in_memory_message_store.go aspect/builtin/in_memory_message_store_test.go infra/memory_backend.go
git commit -m "feat: add InMemoryMessageStore with RecordEventHandler support, replace NopMessageStore default"
```

---

## Task 2: Implement InMemoryRepository

**Why:** Currently only PG has repository implementations. Adding memory implementations enables contract testing and provides test doubles for framework consumers.

**Files:**
- Create: `infra/repository/memory/repository.go`
- Create: `infra/repository/memory/repository_test.go`

- [ ] **Step 1: Write failing test for InMemoryRepository**

Create `infra/repository/memory/repository_test.go`:

```go
package memory

import (
	"context"
	"testing"

	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/domain/repository"
)

type testAggregate struct {
	aggregate.AggregateRoot
	Name   string
	Amount int
}

func newTestAggregate(id string) *testAggregate {
	a := &testAggregate{}
	a.AggregateRoot = *aggregate.NewAggregateRootWithApplier(id, a)
	return a
}

func (a *testAggregate) When(_ event.DomainEvent) {}

func TestInMemoryRepository_InterfaceConformance(t *testing.T) {
	var _ repository.Repository[*testAggregate] = (*InMemoryRepository[*testAggregate])(nil)
}

func TestInMemoryRepository_SaveAndFindByID(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("agg-1")
	agg.Name = "test"
	agg.Amount = 42

	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	found, err := repo.FindByID(ctx, "agg-1")
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if found.GetID() != "agg-1" {
		t.Errorf("expected ID 'agg-1', got %s", found.GetID())
	}
	if found.Name != "test" {
		t.Errorf("expected Name 'test', got %s", found.Name)
	}
	if found.Amount != 42 {
		t.Errorf("expected Amount 42, got %d", found.Amount)
	}
}

func TestInMemoryRepository_FindByID_NotFound(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent aggregate")
	}
}

func TestInMemoryRepository_Delete(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("agg-1")
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if err := repo.Delete(ctx, "agg-1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	_, err := repo.FindByID(ctx, "agg-1")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestInMemoryRepository_Delete_NotFound(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for deleting nonexistent aggregate")
	}
}

func TestInMemoryRepository_OptimisticLock(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("agg-1")
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	found, err := repo.FindByID(ctx, "agg-1")
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	stale := newTestAggregate("agg-1")
	stale.AggregateRoot = *aggregate.NewAggregateRootWithApplier("agg-1", stale)

	err = repo.Save(ctx, stale)
	if err == nil {
		t.Fatal("expected optimistic lock error for stale save")
	}

	err = repo.Save(ctx, found)
	if err != nil {
		t.Fatalf("Save with fresh aggregate should succeed: %v", err)
	}
}

func TestInMemoryRepository_UpdateExistingAggregate(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("agg-1")
	agg.Name = "original"
	agg.Amount = 10
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	found, err := repo.FindByID(ctx, "agg-1")
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	found.Name = "updated"
	found.Amount = 20
	if err := repo.Save(ctx, found); err != nil {
		t.Fatalf("Save update failed: %v", err)
	}

	updated, err := repo.FindByID(ctx, "agg-1")
	if err != nil {
		t.Fatalf("FindByID after update failed: %v", err)
	}
	if updated.Name != "updated" {
		t.Errorf("expected Name 'updated', got %s", updated.Name)
	}
	if updated.Amount != 20 {
		t.Errorf("expected Amount 20, got %d", updated.Amount)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./infra/repository/memory/ -v`
Expected: FAIL - package does not exist

- [ ] **Step 3: Implement InMemoryRepository**

Create `infra/repository/memory/repository.go`:

```go
package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/domain/event"
	ddderror "github.com/ddd-qce/core/error"
	"github.com/ddd-qce/core/domain/repository"
)

type aggregateRecord struct {
	data    []byte
	version int
}

type InMemoryRepository[T aggregate.AggregateRef] struct {
	mu    sync.RWMutex
	store map[string]*aggregateRecord
}

func NewRepository[T aggregate.AggregateRef]() *InMemoryRepository[T] {
	return &InMemoryRepository[T]{
		store: make(map[string]*aggregateRecord),
	}
}

func (r *InMemoryRepository[T]) Save(ctx context.Context, agg T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	root := agg.GetAggregateRoot()
	id := root.GetID()
	version := root.Version()

	data, err := json.Marshal(agg)
	if err != nil {
		return fmt.Errorf("serialize aggregate: %w", err)
	}

	if existing, ok := r.store[id]; ok {
		if version <= existing.version {
			return &OptimisticLockError{AggregateID: id, ExpectedVersion: version}
		}
	}

	r.store[id] = &aggregateRecord{data: data, version: version}
	return nil
}

func (r *InMemoryRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, ok := r.store[id]
	if !ok {
		var zero T
		return zero, fmt.Errorf("aggregate %s: %w", id, ddderror.ErrNotFound)
	}

	var agg T
	if err := json.Unmarshal(record.data, &agg); err != nil {
		return agg, fmt.Errorf("deserialize aggregate: %w", err)
	}
	agg.GetAggregateRoot().SetSnapshotVersion(record.version)
	return agg, nil
}

func (r *InMemoryRepository[T]) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.store[id]; !ok {
		return fmt.Errorf("aggregate %s: %w", id, ddderror.ErrNotFound)
	}
	delete(r.store, id)
	return nil
}

type InMemoryEventSourcedRepository[T aggregate.AggregateRef] struct {
	mu    sync.RWMutex
	store map[string]*aggregateRecord
}

func NewEventSourcedRepository[T aggregate.AggregateRef]() *InMemoryEventSourcedRepository[T] {
	return &InMemoryEventSourcedRepository[T]{
		store: make(map[string]*aggregateRecord),
	}
}

func (r *InMemoryEventSourcedRepository[T]) Save(ctx context.Context, agg T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	root := agg.GetAggregateRoot()
	events := root.UncommittedEvents()
	if len(events) == 0 {
		return nil
	}

	id := root.GetID()
	version := root.Version()

	data, err := json.Marshal(agg)
	if err != nil {
		return fmt.Errorf("serialize aggregate: %w", err)
	}

	r.store[id] = &aggregateRecord{data: data, version: version}
	root.MarkEventsAsCommitted()
	return nil
}

func (r *InMemoryEventSourcedRepository[T]) Load(ctx context.Context, id string) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	record, ok := r.store[id]
	if !ok {
		var zero T
		return zero, fmt.Errorf("aggregate %s: %w", id, ddderror.ErrNotFound)
	}

	var agg T
	if err := json.Unmarshal(record.data, &agg); err != nil {
		return agg, fmt.Errorf("deserialize aggregate: %w", err)
	}
	agg.GetAggregateRoot().SetSnapshotVersion(record.version)
	return agg, nil
}

type OptimisticLockError struct {
	AggregateID     string
	ExpectedVersion int
}

func (e *OptimisticLockError) Error() string {
	return fmt.Sprintf("optimistic lock error: aggregate %s version %d was already updated by another transaction", e.AggregateID, e.ExpectedVersion)
}

func (e *OptimisticLockError) Unwrap() error {
	return ddderror.ErrConcurrency
}

var _ repository.Repository[*aggregate.AggregateRoot] = (*InMemoryRepository[*aggregate.AggregateRoot])(nil)
var _ repository.EventSourcingRepository[*aggregate.AggregateRoot] = (*InMemoryEventSourcedRepository[*aggregate.AggregateRoot])(nil)
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./infra/repository/memory/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add infra/repository/memory/repository.go infra/repository/memory/repository_test.go
git commit -m "feat: add InMemoryRepository and InMemoryEventSourcedRepository"
```

---

## Task 3: JobStore Contract Tests

**Why:** `JobStore` has both memory and PG implementations with divergent test coverage. PG is missing: duplicate-create, update-not-found, empty-list. Contract tests unify coverage.

**Files:**
- Create: `job/core/jobtest/job_store_contract.go`
- Modify: `job/memory/job_store_test.go`
- Modify: `it/job_pg/job_store_test.go`

- [ ] **Step 1: Create contract test package**

Create `job/core/jobtest/job_store_contract.go`:

```go
package jobtest

import (
	"context"
	"testing"
	"time"

	jobcore "github.com/ddd-qce/core/job/core"
)

func TestJobStoreContract(t *testing.T, store jobcore.JobStore) {
	t.Helper()

	t.Run("CreateAndGet", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		job := jobcore.NewJob("contract-create-get", map[string]any{"action": "test"})
		if err := store.Create(ctx, job); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		got, err := store.Get(ctx, "contract-create-get")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got.ID != "contract-create-get" {
			t.Errorf("expected ID 'contract-create-get', got %s", got.ID)
		}
		if got.GetStatus() != jobcore.JobStatusPending {
			t.Errorf("expected status pending, got %s", got.GetStatus())
		}
	})

	t.Run("CreateDuplicate", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		job := jobcore.NewJob("contract-dup", nil)
		if err := store.Create(ctx, job); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		job2 := jobcore.NewJob("contract-dup", nil)
		if err := store.Create(ctx, job2); err == nil {
			t.Fatal("expected error for duplicate create")
		}
	})

	t.Run("GetNotFound", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		_, err := store.Get(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent job")
		}
	})

	t.Run("Update", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		job := jobcore.NewJob("contract-update", map[string]any{"action": "test"})
		if err := store.Create(ctx, job); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		got, err := store.Get(ctx, "contract-update")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		got.MarkRunning()
		if err := store.Update(ctx, got); err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		updated, err := store.Get(ctx, "contract-update")
		if err != nil {
			t.Fatalf("Get after update failed: %v", err)
		}
		if updated.GetStatus() != jobcore.JobStatusRunning {
			t.Errorf("expected status running, got %s", updated.GetStatus())
		}
	})

	t.Run("UpdateNotFound", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		job := jobcore.NewJob("contract-update-nf", nil)
		if err := store.Update(ctx, job); err == nil {
			t.Fatal("expected error for updating nonexistent job")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		job := jobcore.NewJob("contract-delete", nil)
		if err := store.Create(ctx, job); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		if err := store.Delete(ctx, "contract-delete"); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		_, err := store.Get(ctx, "contract-delete")
		if err == nil {
			t.Fatal("expected error after delete")
		}
	})

	t.Run("DeleteNotFound", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		if err := store.Delete(ctx, "nonexistent"); err == nil {
			t.Fatal("expected error for deleting nonexistent job")
		}
	})

	t.Run("ListByStatus", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()

		pending1 := jobcore.NewJob("contract-list-p1", nil)
		pending2 := jobcore.NewJob("contract-list-p2", nil)
		running1 := jobcore.NewJob("contract-list-r1", nil)
		running1.MarkRunning()

		if err := store.Create(ctx, pending1); err != nil {
			t.Fatalf("Create pending1 failed: %v", err)
		}
		if err := store.Create(ctx, pending2); err != nil {
			t.Fatalf("Create pending2 failed: %v", err)
		}
		if err := store.Create(ctx, running1); err != nil {
			t.Fatalf("Create running1 failed: %v", err)
		}

		pending, err := store.List(ctx, jobcore.JobStatusPending)
		if err != nil {
			t.Fatalf("List pending failed: %v", err)
		}
		if len(pending) != 2 {
			t.Errorf("expected 2 pending, got %d", len(pending))
		}

		running, err := store.List(ctx, jobcore.JobStatusRunning)
		if err != nil {
			t.Fatalf("List running failed: %v", err)
		}
		if len(running) != 1 {
			t.Errorf("expected 1 running, got %d", len(running))
		}

		completed, err := store.List(ctx, jobcore.JobStatusCompleted)
		if err != nil {
			t.Fatalf("List completed failed: %v", err)
		}
		if len(completed) != 0 {
			t.Errorf("expected 0 completed, got %d", len(completed))
		}
	})

	t.Run("UpdatePreservesResultAndError", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		job := jobcore.NewJob("contract-result", nil)
		if err := store.Create(ctx, job); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		got, err := store.Get(ctx, "contract-result")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		got.MarkRunning()
		got.TryComplete(map[string]any{"status": "ok"})
		if err := store.Update(ctx, got); err != nil {
			t.Fatalf("Update failed: %v", err)
		}
		updated, err := store.Get(ctx, "contract-result")
		if err != nil {
			t.Fatalf("Get after update failed: %v", err)
		}
		if updated.GetStatus() != jobcore.JobStatusCompleted {
			t.Errorf("expected status completed, got %s", updated.GetStatus())
		}
	})
}
```

- [ ] **Step 2: Add contract test call to memory JobStore test**

Add to `job/memory/job_store_test.go`:

```go
import jobtest "github.com/ddd-qce/core/job/core/jobtest"

func TestJobStore_Contract(t *testing.T) {
	store := NewJobStore()
	jobtest.TestJobStoreContract(t, store)
}
```

- [ ] **Step 3: Add contract test call to PG JobStore test**

Add to `it/job_pg/job_store_test.go`:

```go
import jobtest "github.com/ddd-qce/core/job/core/jobtest"

func TestPgJobStore_Contract(t *testing.T) {
	db := openTestDBForJob(t)
	store := pgjob.NewJobStore(db)
	jobtest.TestJobStoreContract(t, store)
}
```

- [ ] **Step 4: Run memory tests**

Run: `go test ./job/memory/ -run TestJobStore_Contract -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add job/core/jobtest/job_store_contract.go job/memory/job_store_test.go it/job_pg/job_store_test.go
git commit -m "feat: add JobStore contract tests shared between memory and PG"
```

---

## Task 4: TraceStore Contract Tests

**Why:** Memory has no direct store tests (only via TracingAspect). PG is missing time-range and status filter tests. Contract tests unify coverage.

**Files:**
- Create: `trace/tracetest/trace_store_contract.go`
- Modify: appropriate test file in `trace/` package
- Modify: `it/trace_pg/trace_store_test.go`

- [ ] **Step 1: Create contract test package**

Create `trace/tracetest/trace_store_contract.go`:

```go
package tracetest

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/trace"
)

func TestTraceStoreContract(t *testing.T, store trace.TraceStore) {
	t.Helper()

	t.Run("RecordAndGetTrace", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		span := &trace.Span{
			ID:        "span-1",
			TraceID:   "trace-1",
			ParentID:  "",
			Type:      "command",
			Name:      "CreateUser",
			Status:    "success",
			StartedAt: time.Now().Truncate(time.Microsecond),
			Duration:  100 * time.Millisecond,
		}
		if err := store.RecordSpan(ctx, span); err != nil {
			t.Fatalf("RecordSpan failed: %v", err)
		}
		spans, err := store.GetTrace(ctx, "trace-1")
		if err != nil {
			t.Fatalf("GetTrace failed: %v", err)
		}
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		if spans[0].Name != "CreateUser" {
			t.Errorf("expected Name 'CreateUser', got %s", spans[0].Name)
		}
		if spans[0].TraceID != "trace-1" {
			t.Errorf("expected TraceID 'trace-1', got %s", spans[0].TraceID)
		}
	})

	t.Run("GetTraceNotFound", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		_, err := store.GetTrace(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent trace")
		}
	})

	t.Run("RecordSpanWithError", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		span := &trace.Span{
			ID:        "span-err",
			TraceID:   "trace-err",
			Type:      "command",
			Name:      "FailCommand",
			Status:    "error",
			Error:     "something went wrong",
			StartedAt: time.Now().Truncate(time.Microsecond),
			Duration:  50 * time.Millisecond,
		}
		if err := store.RecordSpan(ctx, span); err != nil {
			t.Fatalf("RecordSpan failed: %v", err)
		}
		spans, err := store.GetTrace(ctx, "trace-err")
		if err != nil {
			t.Fatalf("GetTrace failed: %v", err)
		}
		if spans[0].Status != "error" {
			t.Errorf("expected Status 'error', got %s", spans[0].Status)
		}
		if spans[0].Error != "something went wrong" {
			t.Errorf("expected Error 'something went wrong', got %s", spans[0].Error)
		}
	})

	t.Run("ListTracesByType", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()

		store.RecordSpan(ctx, &trace.Span{ID: "s1", TraceID: "t-type-1", Type: "command", Name: "Cmd1", Status: "success", StartedAt: time.Now(), Duration: time.Millisecond})
		store.RecordSpan(ctx, &trace.Span{ID: "s2", TraceID: "t-type-2", Type: "query", Name: "Qry1", Status: "success", StartedAt: time.Now(), Duration: time.Millisecond})
		store.RecordSpan(ctx, &trace.Span{ID: "s3", TraceID: "t-type-3", Type: "command", Name: "Cmd2", Status: "success", StartedAt: time.Now(), Duration: time.Millisecond})

		traces, err := store.ListTraces(ctx, trace.TraceFilter{Type: "command"})
		if err != nil {
			t.Fatalf("ListTraces failed: %v", err)
		}
		if len(traces) != 2 {
			t.Errorf("expected 2 command traces, got %d", len(traces))
		}
	})

	t.Run("ListTracesByNameContains", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()

		store.RecordSpan(ctx, &trace.Span{ID: "s4", TraceID: "t-name-1", Type: "command", Name: "CreateUser", Status: "success", StartedAt: time.Now(), Duration: time.Millisecond})
		store.RecordSpan(ctx, &trace.Span{ID: "s5", TraceID: "t-name-2", Type: "command", Name: "DeleteOrder", Status: "success", StartedAt: time.Now(), Duration: time.Millisecond})

		traces, err := store.ListTraces(ctx, trace.TraceFilter{NameContains: "Create"})
		if err != nil {
			t.Fatalf("ListTraces failed: %v", err)
		}
		if len(traces) != 1 {
			t.Errorf("expected 1 trace with 'Create', got %d", len(traces))
		}
	})

	t.Run("ListTracesByStatus", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()

		store.RecordSpan(ctx, &trace.Span{ID: "s6", TraceID: "t-status-1", Type: "command", Name: "CmdOk", Status: "success", StartedAt: time.Now(), Duration: time.Millisecond})
		store.RecordSpan(ctx, &trace.Span{ID: "s7", TraceID: "t-status-2", Type: "command", Name: "CmdFail", Status: "error", Error: "fail", StartedAt: time.Now(), Duration: time.Millisecond})

		traces, err := store.ListTraces(ctx, trace.TraceFilter{Status: "error"})
		if err != nil {
			t.Fatalf("ListTraces failed: %v", err)
		}
		if len(traces) != 1 {
			t.Errorf("expected 1 error trace, got %d", len(traces))
		}
	})

	t.Run("ListTracesByTimeRange", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()

		oldTime := time.Now().Add(-2 * time.Hour)
		recentTime := time.Now()

		store.RecordSpan(ctx, &trace.Span{ID: "s8", TraceID: "t-time-1", Type: "command", Name: "Old", Status: "success", StartedAt: oldTime, Duration: time.Millisecond})
		store.RecordSpan(ctx, &trace.Span{ID: "s9", TraceID: "t-time-2", Type: "command", Name: "Recent", Status: "success", StartedAt: recentTime, Duration: time.Millisecond})

		traces, err := store.ListTraces(ctx, trace.TraceFilter{StartTime: time.Now().Add(-1 * time.Hour)})
		if err != nil {
			t.Fatalf("ListTraces failed: %v", err)
		}
		if len(traces) != 1 {
			t.Errorf("expected 1 recent trace, got %d", len(traces))
		}
	})

	t.Run("ListTracesEmptyResult", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()

		traces, err := store.ListTraces(ctx, trace.TraceFilter{Type: "nonexistent_type"})
		if err != nil {
			t.Fatalf("ListTraces failed: %v", err)
		}
		if len(traces) != 0 {
			t.Errorf("expected 0 traces, got %d", len(traces))
		}
	})

	t.Run("MultipleSpansPerTrace", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()

		store.RecordSpan(ctx, &trace.Span{ID: "s10", TraceID: "t-multi", ParentID: "", Type: "command", Name: "Root", Status: "success", StartedAt: time.Now(), Duration: time.Millisecond})
		store.RecordSpan(ctx, &trace.Span{ID: "s11", TraceID: "t-multi", ParentID: "s10", Type: "event", Name: "Child", Status: "success", StartedAt: time.Now(), Duration: time.Millisecond})

		spans, err := store.GetTrace(ctx, "t-multi")
		if err != nil {
			t.Fatalf("GetTrace failed: %v", err)
		}
		if len(spans) != 2 {
			t.Fatalf("expected 2 spans, got %d", len(spans))
		}
	})
}
```

- [ ] **Step 2: Add contract test call to memory TraceStore test**

Add to appropriate existing test file in `trace/` package:

```go
import tracetest "github.com/ddd-qce/core/trace/tracetest"

func TestInMemoryTraceStore_Contract(t *testing.T) {
	store := NewInMemoryTraceStore()
	tracetest.TestTraceStoreContract(t, store)
}
```

- [ ] **Step 3: Add contract test call to PG TraceStore test**

Add to `it/trace_pg/trace_store_test.go`:

```go
import tracetest "github.com/ddd-qce/core/trace/tracetest"

func TestPgTraceStore_Contract(t *testing.T) {
	db := openTestDBForTrace(t)
	store := pgtrace.NewTraceStore(db)
	tracetest.TestTraceStoreContract(t, store)
}
```

- [ ] **Step 4: Run memory tests**

Run: `go test ./trace/ -run TestInMemoryTraceStore_Contract -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add trace/tracetest/ trace/ it/trace_pg/trace_store_test.go
git commit -m "feat: add TraceStore contract tests shared between memory and PG"
```

---

## Task 5: MessageStore Contract Tests

**Why:** MessageStore has a new InMemoryMessageStore (Task 1) and PG implementation. Contract tests verify behavioral parity of all 4 Record methods.

**Files:**
- Create: `aspect/builtin/builtintest/message_store_contract.go`
- Modify: `aspect/builtin/in_memory_message_store_test.go`
- Modify: `it/aspect_builtin_pg/message_store_test.go`

- [ ] **Step 1: Create contract test package**

Create `aspect/builtin/builtintest/message_store_contract.go`:

```go
package builtintest

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
)

func TestMessageStoreContract(t *testing.T, store builtin.MessageStore) {
	t.Helper()

	t.Run("RecordCommand", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		entry := &builtin.CommandEntry{
			TraceID:     "trace-cmd",
			SpanID:      "span-cmd",
			CommandType: "CreateUser",
			CommandData: json.RawMessage(`{"name":"Alice"}`),
			ResultType:  "string",
			ResultData:  json.RawMessage(`"user-123"`),
			Duration:    100 * time.Millisecond,
			CreatedAt:   time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordCommand(ctx, entry); err != nil {
			t.Fatalf("RecordCommand failed: %v", err)
		}
	})

	t.Run("RecordCommandWithError", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		entry := &builtin.CommandEntry{
			TraceID:     "trace-cmd-err",
			CommandType: "FailCommand",
			Error:       "something went wrong",
			Duration:    50 * time.Millisecond,
			CreatedAt:   time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordCommand(ctx, entry); err != nil {
			t.Fatalf("RecordCommand with error failed: %v", err)
		}
	})

	t.Run("RecordQuery", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		entry := &builtin.QueryEntry{
			TraceID:   "trace-qry",
			SpanID:    "span-qry",
			QueryType: "GetUser",
			QueryData: json.RawMessage(`{"id":"user-1"}`),
			Duration:  30 * time.Millisecond,
			CreatedAt: time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordQuery(ctx, entry); err != nil {
			t.Fatalf("RecordQuery failed: %v", err)
		}
	})

	t.Run("RecordQueryWithError", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		entry := &builtin.QueryEntry{
			TraceID:   "trace-qry-err",
			QueryType: "FailQuery",
			Error:     "query error",
			Duration:  10 * time.Millisecond,
			CreatedAt: time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordQuery(ctx, entry); err != nil {
			t.Fatalf("RecordQuery with error failed: %v", err)
		}
	})

	t.Run("RecordEvent", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		entry := &builtin.EventEntry{
			TraceID:      "trace-evt",
			SpanID:       "span-evt",
			AggregateID:  "agg-1",
			EventType:    "UserCreated",
			EventData:    json.RawMessage(`{"userId":"user-1"}`),
			HandlerCount: 2,
			Duration:     20 * time.Millisecond,
			CreatedAt:    time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordEvent(ctx, entry); err != nil {
			t.Fatalf("RecordEvent failed: %v", err)
		}
	})

	t.Run("RecordEventWithError", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		entry := &builtin.EventEntry{
			TraceID:     "trace-evt-err",
			AggregateID: "agg-2",
			EventType:   "ProcessFailed",
			Error:       "handler error",
			Duration:    5 * time.Millisecond,
			CreatedAt:   time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordEvent(ctx, entry); err != nil {
			t.Fatalf("RecordEvent with error failed: %v", err)
		}
	})

	t.Run("RecordEventHandler", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		entry := &builtin.EventHandlerEntry{
			TraceID:     "trace-handler",
			SpanID:      "span-handler",
			AggregateID: "agg-1",
			EventType:   "UserCreated",
			HandlerType: "SendWelcomeEmail",
			Status:      "success",
			Duration:    15 * time.Millisecond,
			CreatedAt:   time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordEventHandler(ctx, entry); err != nil {
			t.Fatalf("RecordEventHandler failed: %v", err)
		}
	})

	t.Run("RecordEventHandlerWithErrorStatus", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		entry := &builtin.EventHandlerEntry{
			TraceID:     "trace-handler-err",
			AggregateID: "agg-1",
			EventType:   "UserCreated",
			HandlerType: "NotifyAdmin",
			Status:      "error",
			Error:       "notification failed",
			Duration:    8 * time.Millisecond,
			CreatedAt:   time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordEventHandler(ctx, entry); err != nil {
			t.Fatalf("RecordEventHandler with error status failed: %v", err)
		}
	})

	t.Run("AllEntryTypesTogether", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()

		if err := store.RecordCommand(ctx, &builtin.CommandEntry{CommandType: "C1", Duration: time.Millisecond, CreatedAt: time.Now()}); err != nil {
			t.Fatalf("RecordCommand: %v", err)
		}
		if err := store.RecordQuery(ctx, &builtin.QueryEntry{QueryType: "Q1", Duration: time.Millisecond, CreatedAt: time.Now()}); err != nil {
			t.Fatalf("RecordQuery: %v", err)
		}
		if err := store.RecordEvent(ctx, &builtin.EventEntry{EventType: "E1", AggregateID: "a1", Duration: time.Millisecond, CreatedAt: time.Now()}); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
		if err := store.RecordEventHandler(ctx, &builtin.EventHandlerEntry{EventType: "E1", HandlerType: "H1", Status: "success", Duration: time.Millisecond, CreatedAt: time.Now()}); err != nil {
			t.Fatalf("RecordEventHandler: %v", err)
		}
	})
}
```

- [ ] **Step 2: Add contract test call to InMemoryMessageStore test**

Add to `aspect/builtin/in_memory_message_store_test.go`:

```go
import builtintest "github.com/ddd-qce/core/aspect/builtin/builtintest"

func TestInMemoryMessageStore_Contract(t *testing.T) {
	store := NewInMemoryMessageStore()
	builtintest.TestMessageStoreContract(t, store)
}
```

- [ ] **Step 3: Add contract test call to PG MessageStore test**

Add to `it/aspect_builtin_pg/message_store_test.go`:

```go
import builtintest "github.com/ddd-qce/core/aspect/builtin/builtintest"

func TestPgMessageStore_Contract(t *testing.T) {
	db := openTestDBForAspect(t)
	store := pgmsg.NewMessageStore(db)
	builtintest.TestMessageStoreContract(t, store)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./aspect/builtin/ -run TestInMemoryMessageStore_Contract -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add aspect/builtin/builtintest/message_store_contract.go aspect/builtin/in_memory_message_store_test.go it/aspect_builtin_pg/message_store_test.go
git commit -m "feat: add MessageStore contract tests shared between memory and PG"
```

---

## Task 6: TransactionManager Contract Tests

**Why:** Memory and PG both test transaction semantics but with different coverage. PG is missing triple-nesting test. Memory is missing explicit rollback verification and re-begin-after-rollback test.

**Files:**
- Create: `aspect/builtin/builtintest/transaction_manager_contract.go`
- Modify: `infra/backend_test.go`
- Modify: `it/pg/transaction_test.go`

- [ ] **Step 1: Create contract test package**

Create `aspect/builtin/builtintest/transaction_manager_contract.go`:

```go
package builtintest

import (
	"context"
	"testing"

	"github.com/ddd-qce/core/aspect/builtin"
)

func TestTransactionManagerContract(t *testing.T, tm builtin.TransactionManager, newCtx func() context.Context) {
	t.Helper()

	t.Run("BeginAndCommit", func(t *testing.T) {
		t.Helper()
		ctx := newCtx()
		ctx, err := tm.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin failed: %v", err)
		}
		if err := tm.Commit(ctx); err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
	})

	t.Run("BeginAndRollback", func(t *testing.T) {
		t.Helper()
		ctx := newCtx()
		ctx, err := tm.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin failed: %v", err)
		}
		if err := tm.Rollback(ctx); err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}
	})

	t.Run("NoTransactionCommit", func(t *testing.T) {
		t.Helper()
		ctx := newCtx()
		if err := tm.Commit(ctx); err == nil {
			t.Fatal("expected error for commit without transaction")
		}
	})

	t.Run("NoTransactionRollback", func(t *testing.T) {
		t.Helper()
		ctx := newCtx()
		if err := tm.Rollback(ctx); err == nil {
			t.Fatal("expected error for rollback without transaction")
		}
	})

	t.Run("NestedBeginCommit", func(t *testing.T) {
		t.Helper()
		ctx := newCtx()
		ctx, err := tm.Begin(ctx)
		if err != nil {
			t.Fatalf("outer Begin failed: %v", err)
		}
		ctx, err = tm.Begin(ctx)
		if err != nil {
			t.Fatalf("inner Begin failed: %v", err)
		}
		if err := tm.Commit(ctx); err != nil {
			t.Fatalf("inner Commit failed: %v", err)
		}
		if err := tm.Commit(ctx); err != nil {
			t.Fatalf("outer Commit failed: %v", err)
		}
	})

	t.Run("NestedRollbackAbortsOuter", func(t *testing.T) {
		t.Helper()
		ctx := newCtx()
		ctx, err := tm.Begin(ctx)
		if err != nil {
			t.Fatalf("outer Begin failed: %v", err)
		}
		ctx, err = tm.Begin(ctx)
		if err != nil {
			t.Fatalf("inner Begin failed: %v", err)
		}
		if err := tm.Rollback(ctx); err != nil {
			t.Fatalf("inner Rollback failed: %v", err)
		}
		if err := tm.Commit(ctx); err == nil {
			t.Fatal("expected error for outer commit after inner rollback")
		}
	})

	t.Run("TripleNesting", func(t *testing.T) {
		t.Helper()
		ctx := newCtx()
		ctx, err := tm.Begin(ctx)
		if err != nil {
			t.Fatalf("level 1 Begin failed: %v", err)
		}
		ctx, err = tm.Begin(ctx)
		if err != nil {
			t.Fatalf("level 2 Begin failed: %v", err)
		}
		ctx, err = tm.Begin(ctx)
		if err != nil {
			t.Fatalf("level 3 Begin failed: %v", err)
		}
		if err := tm.Commit(ctx); err != nil {
			t.Fatalf("level 3 Commit failed: %v", err)
		}
		if err := tm.Commit(ctx); err != nil {
			t.Fatalf("level 2 Commit failed: %v", err)
		}
		if err := tm.Commit(ctx); err != nil {
			t.Fatalf("level 1 Commit failed: %v", err)
		}
	})

	t.Run("RollbackThenBeginAgain", func(t *testing.T) {
		t.Helper()
		ctx := newCtx()
		ctx, err := tm.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin failed: %v", err)
		}
		if err := tm.Rollback(ctx); err != nil {
			t.Fatalf("Rollback failed: %v", err)
		}
		ctx, err = tm.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin again failed: %v", err)
		}
		if err := tm.Commit(ctx); err != nil {
			t.Fatalf("Commit after re-begin failed: %v", err)
		}
	})
}
```

- [ ] **Step 2: Add contract test call to memory TransactionManager test**

Add to `infra/backend_test.go`:

```go
import builtintest "github.com/ddd-qce/core/aspect/builtin/builtintest"

func TestMemoryTransactionManager_Contract(t *testing.T) {
	tm := NewMemoryTransactionManager()
	builtintest.TestTransactionManagerContract(t, tm, func() context.Context {
		return context.Background()
	})
}
```

- [ ] **Step 3: Add contract test call to PG TransactionManager test**

Add to `it/pg/transaction_test.go`:

```go
import builtintest "github.com/ddd-qce/core/aspect/builtin/builtintest"

func TestPgTransactionManager_Contract(t *testing.T) {
	db := openTestDBForTx(t)
	tm := corepg.NewTransactionManager(db)
	builtintest.TestTransactionManagerContract(t, tm, func() context.Context {
		return context.Background()
	})
}
```

- [ ] **Step 4: Run memory tests**

Run: `go test ./infra/ -run TestMemoryTransactionManager_Contract -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add aspect/builtin/builtintest/transaction_manager_contract.go infra/backend_test.go it/pg/transaction_test.go
git commit -m "feat: add TransactionManager contract tests shared between memory and PG"
```

---

## Task 7: EventStore Contract Tests

**Why:** PG is missing concurrency conflict, multi-batch append, mutation isolation, and construction validation tests that memory already covers. Contract tests unify coverage.

**Files:**
- Create: `cqrs/event/eventtest/event_store_contract.go`
- Modify: `cqrs/event/memory/event_store_test.go`
- Modify: `it/cqrs_event_pg/event_store_test.go`

- [ ] **Step 1: Create contract test package**

Create `cqrs/event/eventtest/event_store_contract.go`:

```go
package eventtest

import (
	"context"
	"testing"

	"github.com/ddd-qce/core/domain/event"
	cqevent "github.com/ddd-qce/core/cqrs/event"
)

type TestEvent struct {
	event.BaseDomainEvent
	Data string
}

func NewTestEvent(aggID, data string) *TestEvent {
	return &TestEvent{
		BaseDomainEvent: *event.NewBaseDomainEvent(aggID, "TestEvent"),
		Data:            data,
	}
}

func TestEventStoreContract(t *testing.T, newStore func() cqevent.EventStore[*TestEvent]) {
	t.Helper()

	t.Run("AppendAndLoad", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		store := newStore()

		events := []*TestEvent{
			NewTestEvent("agg-1", "e1"),
			NewTestEvent("agg-1", "e2"),
			NewTestEvent("agg-1", "e3"),
		}
		if err := store.Append(ctx, "agg-1", 0, events); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
		loaded, err := store.Load(ctx, "agg-1", 0)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if len(loaded) != 3 {
			t.Fatalf("expected 3 events, got %d", len(loaded))
		}
	})

	t.Run("LoadAfterVersion", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		store := newStore()

		events := []*TestEvent{
			NewTestEvent("agg-2", "e1"),
			NewTestEvent("agg-2", "e2"),
			NewTestEvent("agg-2", "e3"),
			NewTestEvent("agg-2", "e4"),
		}
		if err := store.Append(ctx, "agg-2", 0, events); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
		loaded, err := store.Load(ctx, "agg-2", 2)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if len(loaded) != 2 {
			t.Fatalf("expected 2 events after version 2, got %d", len(loaded))
		}
	})

	t.Run("LoadNonExistentAggregate", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		store := newStore()

		loaded, err := store.Load(ctx, "nonexistent", 0)
		if err != nil {
			t.Fatalf("Load nonexistent should not error, got: %v", err)
		}
		if len(loaded) != 0 {
			t.Fatalf("expected 0 events, got %d", len(loaded))
		}
	})

	t.Run("AppendMultipleBatches", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		store := newStore()

		batch1 := []*TestEvent{NewTestEvent("agg-3", "e1"), NewTestEvent("agg-3", "e2")}
		if err := store.Append(ctx, "agg-3", 0, batch1); err != nil {
			t.Fatalf("Append batch1 failed: %v", err)
		}
		batch2 := []*TestEvent{NewTestEvent("agg-3", "e3")}
		if err := store.Append(ctx, "agg-3", 2, batch2); err != nil {
			t.Fatalf("Append batch2 failed: %v", err)
		}
		loaded, err := store.Load(ctx, "agg-3", 0)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if len(loaded) != 3 {
			t.Fatalf("expected 3 events, got %d", len(loaded))
		}
	})

	t.Run("ConcurrencyConflict", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		store := newStore()

		events1 := []*TestEvent{NewTestEvent("agg-4", "e1")}
		if err := store.Append(ctx, "agg-4", 0, events1); err != nil {
			t.Fatalf("Append 1 failed: %v", err)
		}
		events2 := []*TestEvent{NewTestEvent("agg-4", "e2")}
		if err := store.Append(ctx, "agg-4", 0, events2); err == nil {
			t.Fatal("expected concurrency conflict error for append at version 0 after already appended")
		}
	})

	t.Run("MultipleAggregates", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		store := newStore()

		if err := store.Append(ctx, "agg-a", 0, []*TestEvent{NewTestEvent("agg-a", "a1"), NewTestEvent("agg-a", "a2")}); err != nil {
			t.Fatalf("Append agg-a failed: %v", err)
		}
		if err := store.Append(ctx, "agg-b", 0, []*TestEvent{NewTestEvent("agg-b", "b1"), NewTestEvent("agg-b", "b2")}); err != nil {
			t.Fatalf("Append agg-b failed: %v", err)
		}

		loadedA, err := store.Load(ctx, "agg-a", 0)
		if err != nil {
			t.Fatalf("Load agg-a failed: %v", err)
		}
		if len(loadedA) != 2 {
			t.Errorf("expected 2 events for agg-a, got %d", len(loadedA))
		}

		loadedB, err := store.Load(ctx, "agg-b", 0)
		if err != nil {
			t.Fatalf("Load agg-b failed: %v", err)
		}
		if len(loadedB) != 2 {
			t.Errorf("expected 2 events for agg-b, got %d", len(loadedB))
		}
	})

	t.Run("LoadAfterVersionBeyondRange", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		store := newStore()

		if err := store.Append(ctx, "agg-5", 0, []*TestEvent{NewTestEvent("agg-5", "e1")}); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
		loaded, err := store.Load(ctx, "agg-5", 5)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if len(loaded) != 0 {
			t.Errorf("expected 0 events after version 5, got %d", len(loaded))
		}
	})

	t.Run("EventDataRoundTrip", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		store := newStore()

		if err := store.Append(ctx, "agg-data", 0, []*TestEvent{NewTestEvent("agg-data", "hello world")}); err != nil {
			t.Fatalf("Append failed: %v", err)
		}
		loaded, err := store.Load(ctx, "agg-data", 0)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if loaded[0].Data != "hello world" {
			t.Errorf("expected Data 'hello world', got %s", loaded[0].Data)
		}
		if loaded[0].AggregateID() != "agg-data" {
			t.Errorf("expected AggregateID 'agg-data', got %s", loaded[0].AggregateID())
		}
	})
}
```

- [ ] **Step 2: Add contract test call to memory EventStore test**

Add to `cqrs/event/memory/event_store_test.go`:

```go
import eventtest "github.com/ddd-qce/core/cqrs/event/eventtest"

func TestEventStore_Contract(t *testing.T) {
	eventtest.TestEventStoreContract(t, func() cqevent.EventStore[*eventtest.TestEvent] {
		store, err := NewEventStore[*eventtest.TestEvent]()
		if err != nil {
			t.Fatalf("NewEventStore failed: %v", err)
		}
		return store
	})
}
```

- [ ] **Step 3: Add contract test call to PG EventStore test**

Add to `it/cqrs_event_pg/event_store_test.go`:

```go
import eventtest "github.com/ddd-qce/core/cqrs/event/eventtest"

func TestPgEventStore_Contract(t *testing.T) {
	db := openTestDBForEvent(t)
	eventtest.TestEventStoreContract(t, func() cqevent.EventStore[*eventtest.TestEvent] {
		s, err := pgevent.NewEventStore[*eventtest.TestEvent](db)
		if err != nil {
			t.Fatalf("NewEventStore failed: %v", err)
		}
		return s
	})
}
```

- [ ] **Step 4: Run memory tests**

Run: `go test ./cqrs/event/memory/ -run TestEventStore_Contract -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add cqrs/event/eventtest/event_store_contract.go cqrs/event/memory/event_store_test.go it/cqrs_event_pg/event_store_test.go
git commit -m "feat: add EventStore contract tests shared between memory and PG"
```

---

## Task 8: Repository Contract Tests

**Why:** Repository now has both memory and PG implementations. Contract tests verify Save/FindByID/Delete, optimistic locking, and not-found errors across both.

**Files:**
- Create: `domain/repository/repositorytest/repository_contract.go`
- Modify: `infra/repository/memory/repository_test.go`
- Modify: `it/infra_pg/repository_test.go`

- [ ] **Step 1: Create contract test package**

Create `domain/repository/repositorytest/repository_contract.go`:

```go
package repositorytest

import (
	"context"
	"testing"

	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/domain/repository"
)

type TestAggregate struct {
	aggregate.AggregateRoot
	Name   string
	Amount int
}

func NewTestAggregate(id string) *TestAggregate {
	a := &TestAggregate{}
	a.AggregateRoot = *aggregate.NewAggregateRootWithApplier(id, a)
	return a
}

func (a *TestAggregate) When(_ event.DomainEvent) {}

func TestRepositoryContract(t *testing.T, repo repository.Repository[*TestAggregate]) {
	t.Helper()

	t.Run("SaveAndFindByID", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		agg := NewTestAggregate("repo-1")
		agg.Name = "test"
		agg.Amount = 42

		if err := repo.Save(ctx, agg); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
		found, err := repo.FindByID(ctx, "repo-1")
		if err != nil {
			t.Fatalf("FindByID failed: %v", err)
		}
		if found.GetID() != "repo-1" {
			t.Errorf("expected ID 'repo-1', got %s", found.GetID())
		}
		if found.Name != "test" {
			t.Errorf("expected Name 'test', got %s", found.Name)
		}
		if found.Amount != 42 {
			t.Errorf("expected Amount 42, got %d", found.Amount)
		}
	})

	t.Run("FindByID_NotFound", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		_, err := repo.FindByID(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent aggregate")
		}
	})

	t.Run("Delete", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		agg := NewTestAggregate("repo-del")
		if err := repo.Save(ctx, agg); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
		if err := repo.Delete(ctx, "repo-del"); err != nil {
			t.Fatalf("Delete failed: %v", err)
		}
		_, err := repo.FindByID(ctx, "repo-del")
		if err == nil {
			t.Fatal("expected error after delete")
		}
	})

	t.Run("Delete_NotFound", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		err := repo.Delete(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error for deleting nonexistent aggregate")
		}
	})

	t.Run("OptimisticLock", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		agg := NewTestAggregate("repo-lock")
		if err := repo.Save(ctx, agg); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
		stale := NewTestAggregate("repo-lock")
		err := repo.Save(ctx, stale)
		if err == nil {
			t.Fatal("expected optimistic lock error for stale save")
		}
	})

	t.Run("UpdateExistingAggregate", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		agg := NewTestAggregate("repo-update")
		agg.Name = "original"
		agg.Amount = 10
		if err := repo.Save(ctx, agg); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
		found, err := repo.FindByID(ctx, "repo-update")
		if err != nil {
			t.Fatalf("FindByID failed: %v", err)
		}
		found.Name = "updated"
		found.Amount = 20
		if err := repo.Save(ctx, found); err != nil {
			t.Fatalf("Update Save failed: %v", err)
		}
		updated, err := repo.FindByID(ctx, "repo-update")
		if err != nil {
			t.Fatalf("FindByID after update failed: %v", err)
		}
		if updated.Name != "updated" {
			t.Errorf("expected Name 'updated', got %s", updated.Name)
		}
	})
}

func TestEventSourcingRepositoryContract(t *testing.T, repo repository.EventSourcingRepository[*TestAggregate]) {
	t.Helper()

	t.Run("SaveAndLoad", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		agg := NewTestAggregate("es-1")
		agg.Apply(event.NewBaseDomainEvent("es-1", "TestEvent"))

		if err := repo.Save(ctx, agg); err != nil {
			t.Fatalf("Save failed: %v", err)
		}
		loaded, err := repo.Load(ctx, "es-1")
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}
		if loaded.GetID() != "es-1" {
			t.Errorf("expected ID 'es-1', got %s", loaded.GetID())
		}
		if loaded.Version() != 1 {
			t.Errorf("expected Version 1, got %d", loaded.Version())
		}
	})

	t.Run("Load_NotFound", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		_, err := repo.Load(ctx, "nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent aggregate")
		}
	})
}
```

- [ ] **Step 2: Add contract test call to InMemoryRepository test**

Add to `infra/repository/memory/repository_test.go`:

```go
import repositorytest "github.com/ddd-qce/core/domain/repository/repositorytest"

func TestInMemoryRepository_Contract(t *testing.T) {
	repo := NewRepository[*repositorytest.TestAggregate]()
	repositorytest.TestRepositoryContract(t, repo)
}

func TestInMemoryEventSourcedRepository_Contract(t *testing.T) {
	repo := NewEventSourcedRepository[*repositorytest.TestAggregate]()
	repositorytest.TestEventSourcingRepositoryContract(t, repo)
}
```

- [ ] **Step 3: Add contract test call to PG Repository test**

Add to `it/infra_pg/repository_test.go`:

```go
import repositorytest "github.com/ddd-qce/core/domain/repository/repositorytest"

func TestPgRepository_Contract(t *testing.T) {
	db := openTestDBForRepo(t)
	repo := pgrepo.NewRepository[*repositorytest.TestAggregate](db)
	repositorytest.TestRepositoryContract(t, repo)
}

func TestPgEventSourcedRepository_Contract(t *testing.T) {
	db := openTestDBForRepo(t)
	eventStore := pgevent.NewEventStore[event.DomainEvent](db)
	repo := pgrepo.NewEventSourcedRepository[*repositorytest.TestAggregate](
		db,
		eventStore,
		func(id string) *repositorytest.TestAggregate {
			return repositorytest.NewTestAggregate(id)
		},
	)
	repositorytest.TestEventSourcingRepositoryContract(t, repo)
}
```

- [ ] **Step 4: Run memory tests**

Run: `go test ./infra/repository/memory/ -run TestInMemoryRepository_Contract -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add domain/repository/repositorytest/repository_contract.go infra/repository/memory/repository_test.go it/infra_pg/repository_test.go
git commit -m "feat: add Repository contract tests shared between memory and PG"
```

---

## Task 9: Backend Contract Test & PG Backend Test

**Why:** `pgx.NewPGBackend()` is completely untested. A backend contract test verifies that a Backend is correctly wired and its components work together.

**Files:**
- Create: `infra/infratest/backend_contract.go`
- Modify: `infra/backend_test.go`
- Create: `it/infra_pg/backend_test.go`

- [ ] **Step 1: Create contract test package**

Create `infra/infratest/backend_contract.go`:

```go
package infratest

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/infra"
	"github.com/ddd-qce/core/trace"
)

func TestBackendContract(t *testing.T, backend *infra.Backend) {
	t.Helper()

	t.Run("InterfaceConformance", func(t *testing.T) {
		t.Helper()
		var _ builtin.TransactionManager = backend.TransactionManager
		var _ jobcore.JobStore = backend.JobStore
		var _ trace.TraceStore = backend.TraceStore
		var _ builtin.MessageStore = backend.MessageStore
		var _ infra.Migrator = backend.Migrator
	})

	t.Run("TransactionManagerBeginCommit", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		ctx, err := backend.TransactionManager.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin failed: %v", err)
		}
		if err := backend.TransactionManager.Commit(ctx); err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
	})

	t.Run("JobStoreCreateAndGet", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		job := jobcore.NewJob("backend-test-job", nil)
		if err := backend.JobStore.Create(ctx, job); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		got, err := backend.JobStore.Get(ctx, "backend-test-job")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got.ID != "backend-test-job" {
			t.Errorf("expected ID 'backend-test-job', got %s", got.ID)
		}
	})

	t.Run("TraceStoreRecordAndGet", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		span := &trace.Span{
			ID:        "backend-span",
			TraceID:   "backend-trace",
			Type:      "command",
			Name:      "Test",
			Status:    "success",
			StartedAt: time.Now(),
			Duration:  time.Millisecond,
		}
		if err := backend.TraceStore.RecordSpan(ctx, span); err != nil {
			t.Fatalf("RecordSpan failed: %v", err)
		}
		spans, err := backend.TraceStore.GetTrace(ctx, "backend-trace")
		if err != nil {
			t.Fatalf("GetTrace failed: %v", err)
		}
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
	})

	t.Run("MessageStoreRecordCommand", func(t *testing.T) {
		t.Helper()
		ctx := context.Background()
		entry := &builtin.CommandEntry{
			CommandType: "TestCommand",
			Duration:    time.Millisecond,
			CreatedAt:   time.Now(),
		}
		if err := backend.MessageStore.RecordCommand(ctx, entry); err != nil {
			t.Fatalf("RecordCommand failed: %v", err)
		}
	})

	t.Run("Migrate", func(t *testing.T) {
		t.Helper()
		if err := backend.Migrator.Migrate(context.Background()); err != nil {
			t.Fatalf("Migrate failed: %v", err)
		}
	})
}
```

- [ ] **Step 2: Add contract test call to memory backend test**

Add to `infra/backend_test.go`:

```go
import infratest "github.com/ddd-qce/core/infra/infratest"

func TestMemoryBackend_Contract(t *testing.T) {
	backend := NewMemoryBackend()
	infratest.TestBackendContract(t, backend)
}
```

- [ ] **Step 3: Create PG backend integration test**

Create `it/infra_pg/backend_test.go`:

```go
package infra_pg

import (
	"database/sql"
	"testing"

	infratest "github.com/ddd-qce/core/infra/infratest"
	corepgx "github.com/ddd-qce/core/pgx"
	"github.com/ddd-qce/it/testutil"
)

func openTestDBForBackend(t *testing.T) *sql.DB {
	return testutil.OpenTestDB(t, "ddd_qce_backend_test")
}

func TestPGBackend_Contract(t *testing.T) {
	db := openTestDBForBackend(t)
	backend := corepgx.NewPGBackend(db)
	infratest.TestBackendContract(t, backend)
}
```

- [ ] **Step 4: Run memory backend tests**

Run: `go test ./infra/ -run TestMemoryBackend_Contract -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add infra/infratest/backend_contract.go infra/backend_test.go it/infra_pg/backend_test.go
git commit -m "feat: add Backend contract tests and PG Backend integration test"
```

---

## Task 10: PersistenceAspect Unit Tests

**Why:** `PersistenceAspect` is completely untested - it marshals CQRS messages and routes them to the correct `MessageStore` method, but this logic has no direct tests.

**Files:**
- Create: `aspect/builtin/persistence_test.go`

- [ ] **Step 1: Write PersistenceAspect tests**

Create `aspect/builtin/persistence_test.go`:

```go
package builtin

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/trace"
)

type testPersistenceEvent struct {
	event.BaseDomainEvent
}

func TestPersistenceAspect_AfterCommand(t *testing.T) {
	store := NewInMemoryMessageStore()
	pa := NewPersistenceAspect(store)
	ctx := context.Background()

	cmd := struct{ Name string }{Name: "test"}
	err := pa.AfterCommand(ctx, cmd, "result", nil, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("AfterCommand failed: %v", err)
	}
	if len(store.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(store.commands))
	}
	if store.commands[0].Duration != 100*time.Millisecond {
		t.Errorf("expected Duration 100ms, got %v", store.commands[0].Duration)
	}
	if store.commands[0].Error != "" {
		t.Errorf("expected no error, got %s", store.commands[0].Error)
	}
}

func TestPersistenceAspect_AfterCommandWithError(t *testing.T) {
	store := NewInMemoryMessageStore()
	pa := NewPersistenceAspect(store)
	ctx := context.Background()

	err := pa.AfterCommand(ctx, struct{}{}, nil, fmt.Errorf("command failed"), 50*time.Millisecond)
	if err != nil {
		t.Fatalf("AfterCommand failed: %v", err)
	}
	if store.commands[0].Error != "command failed" {
		t.Errorf("expected Error 'command failed', got %s", store.commands[0].Error)
	}
}

func TestPersistenceAspect_AfterCommandWithTraceContext(t *testing.T) {
	store := NewInMemoryMessageStore()
	pa := NewPersistenceAspect(store)
	ctx := trace.WithTraceID(context.Background(), "trace-1")
	ctx = trace.WithSpanID(ctx, "span-1")

	err := pa.AfterCommand(ctx, struct{}{}, nil, nil, time.Millisecond)
	if err != nil {
		t.Fatalf("AfterCommand failed: %v", err)
	}
	if store.commands[0].TraceID != "trace-1" {
		t.Errorf("expected TraceID 'trace-1', got %s", store.commands[0].TraceID)
	}
	if store.commands[0].SpanID != "span-1" {
		t.Errorf("expected SpanID 'span-1', got %s", store.commands[0].SpanID)
	}
}

func TestPersistenceAspect_AfterQuery(t *testing.T) {
	store := NewInMemoryMessageStore()
	pa := NewPersistenceAspect(store)
	ctx := context.Background()

	err := pa.AfterQuery(ctx, struct{ ID string }{ID: "q1"}, "result", nil, 30*time.Millisecond)
	if err != nil {
		t.Fatalf("AfterQuery failed: %v", err)
	}
	if len(store.queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(store.queries))
	}
	if store.queries[0].Duration != 30*time.Millisecond {
		t.Errorf("expected Duration 30ms, got %v", store.queries[0].Duration)
	}
}

func TestPersistenceAspect_AfterPublish_Event(t *testing.T) {
	store := NewInMemoryMessageStore()
	pa := NewPersistenceAspect(store)
	ctx := context.Background()

	evt := &testPersistenceEvent{
		BaseDomainEvent: *event.NewBaseDomainEvent("agg-1", "UserCreated"),
	}

	err := pa.AfterPublish(ctx, evt, nil, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("AfterPublish failed: %v", err)
	}
	if len(store.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(store.events))
	}
	if store.events[0].EventType != "UserCreated" {
		t.Errorf("expected EventType 'UserCreated', got %s", store.events[0].EventType)
	}
	if store.events[0].AggregateID != "agg-1" {
		t.Errorf("expected AggregateID 'agg-1', got %s", store.events[0].AggregateID)
	}
}

func TestPersistenceAspect_AfterPublish_EventHandler(t *testing.T) {
	store := NewInMemoryMessageStore()
	pa := NewPersistenceAspect(store)
	ctx := ContextWithHandlerType(context.Background(), "SendWelcomeEmail")

	evt := &testPersistenceEvent{
		BaseDomainEvent: *event.NewBaseDomainEvent("agg-1", "UserCreated"),
	}

	err := pa.AfterPublish(ctx, evt, nil, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("AfterPublish failed: %v", err)
	}
	if len(store.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(store.handlers))
	}
	if store.handlers[0].HandlerType != "SendWelcomeEmail" {
		t.Errorf("expected HandlerType 'SendWelcomeEmail', got %s", store.handlers[0].HandlerType)
	}
	if store.handlers[0].Status != "success" {
		t.Errorf("expected Status 'success', got %s", store.handlers[0].Status)
	}
}

func TestPersistenceAspect_AfterPublish_EventHandlerWithError(t *testing.T) {
	store := NewInMemoryMessageStore()
	pa := NewPersistenceAspect(store)
	ctx := ContextWithHandlerType(context.Background(), "FailHandler")

	evt := &testPersistenceEvent{
		BaseDomainEvent: *event.NewBaseDomainEvent("agg-1", "UserCreated"),
	}

	err := pa.AfterPublish(ctx, evt, fmt.Errorf("handler error"), 5*time.Millisecond)
	if err != nil {
		t.Fatalf("AfterPublish failed: %v", err)
	}
	if store.handlers[0].Status != "error" {
		t.Errorf("expected Status 'error', got %s", store.handlers[0].Status)
	}
	if store.handlers[0].Error != "handler error" {
		t.Errorf("expected Error 'handler error', got %s", store.handlers[0].Error)
	}
}

func TestPersistenceAspect_AfterPublish_EventWithError(t *testing.T) {
	store := NewInMemoryMessageStore()
	pa := NewPersistenceAspect(store)
	ctx := context.Background()

	evt := &testPersistenceEvent{
		BaseDomainEvent: *event.NewBaseDomainEvent("agg-1", "UserCreated"),
	}

	err := pa.AfterPublish(ctx, evt, fmt.Errorf("event error"), 5*time.Millisecond)
	if err != nil {
		t.Fatalf("AfterPublish failed: %v", err)
	}
	if store.events[0].Error != "event error" {
		t.Errorf("expected Error 'event error', got %s", store.events[0].Error)
	}
}

func TestPersistenceAspect_NameAndOrder(t *testing.T) {
	a := NewPersistenceAspect(NewInMemoryMessageStore())
	if a.Name() != "persistence" {
		t.Errorf("expected Name 'persistence', got %s", a.Name())
	}
	if a.Order() != 200 {
		t.Errorf("expected Order 200, got %d", a.Order())
	}
}
```

- [ ] **Step 2: Run tests**

Run: `go test ./aspect/builtin/ -run TestPersistenceAspect -v`
Expected: PASS

- [ ] **Step 3: Commit**

```bash
git add aspect/builtin/persistence_test.go
git commit -m "feat: add PersistenceAspect unit tests"
```

---

## Task 11: Full Test Suite Verification

**Why:** Ensure all new and existing tests pass together, verify no regressions.

- [ ] **Step 1: Run all core module tests**

Run: `go test ./... -v -count=1`
Expected: All PASS

- [ ] **Step 2: Run all integration tests** (requires PG)

Run: `cd it && go test ./... -v -count=1`
Expected: All PASS

- [ ] **Step 3: Run with race detector**

Run: `go test -race ./... -count=1`
Expected: All PASS, no race conditions

- [ ] **Step 4: Commit any fixes if needed**

```bash
git add -A
git commit -m "fix: address any test issues found during full verification"
```

---

## Self-Review Checklist

### Spec Coverage
- [x] InMemoryMessageStore with RecordEventHandler -> Task 1
- [x] Replace NopMessageStore default -> Task 1
- [x] InMemoryRepository -> Task 2
- [x] JobStore contract tests -> Task 3
- [x] TraceStore contract tests -> Task 4
- [x] MessageStore contract tests -> Task 5
- [x] TransactionManager contract tests -> Task 6
- [x] EventStore contract tests -> Task 7
- [x] Repository contract tests -> Task 8
- [x] Backend contract tests + PG Backend test -> Task 9
- [x] PersistenceAspect unit tests -> Task 10
- [x] Full verification -> Task 11

### Placeholder Scan
- No TBD, TODO, or "implement later" found
- All test code is concrete and complete
- No "similar to Task N" shortcuts

### Type Consistency
- `TestEvent` in `eventtest` package matches `DomainEvent` interface via `BaseDomainEvent`
- `TestAggregate` in `repositorytest` package matches `AggregateRef` interface via `AggregateRoot`
- All contract test functions take interface types matching the implementations
- `newStore` function type in EventStore contract matches constructor signatures

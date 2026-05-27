package memory

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	ddderror "github.com/ddd-qce/core/error"
	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/domain/repository"
	"github.com/ddd-qce/core/domain/repository/repositorytest"
	rep "github.com/ddd-qce/core/infra/repository"
)

type testAggregate struct {
	aggregate.AggregateRoot
	Name  string
	Count int
}

func newTestAggregate(id string) *testAggregate {
	a := &testAggregate{}
	ar, err := aggregate.NewAggregateRoot(id)
	if err != nil {
		panic(err)
	}
	a.AggregateRoot = *ar
	return a
}

func (a *testAggregate) When(_ event.Event) error { return nil }

func (a *testAggregate) Apply(ctx context.Context, evt event.Event) error {
	return aggregate.ApplyChange(a, ctx, evt)
}

func (a *testAggregate) LoadFromHistory(events []event.Event) error {
	return aggregate.LoadFromHistory(a, events)
}

type testAggregateJSON struct {
	aggregate.AggregateRootJSON
	Name  string `json:"name"`
	Count int   `json:"count"`
}

func (a *testAggregate) MarshalJSON() ([]byte, error) {
	return json.Marshal(testAggregateJSON{
		AggregateRootJSON: a.AggregateRoot.ToJSON(),
		Name:              a.Name,
		Count:             a.Count,
	})
}

func (a *testAggregate) UnmarshalJSON(data []byte) error {
	var aux testAggregateJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	a.AggregateRoot.FromJSON(aux.AggregateRootJSON)
	a.Name = aux.Name
	a.Count = aux.Count
	return nil
}

var _ repository.Repository[*testAggregate] = (*InMemoryRepository[*testAggregate])(nil)
var _ repository.EventSourcingRepository[*testAggregate] = (*InMemoryEventSourcedRepository[*testAggregate])(nil)

func TestInMemoryRepository_SaveAndFindByID(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("agg-1")
	agg.Name = "test-name"
	agg.Count = 42

	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, err := repo.FindByID(ctx, "agg-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if found.ID() != "agg-1" {
		t.Errorf("GetID() = %q, want %q", found.ID(), "agg-1")
	}
	if found.Name != "test-name" {
		t.Errorf("Name = %q, want %q", found.Name, "test-name")
	}
	if found.Count != 42 {
		t.Errorf("Count = %d, want %d", found.Count, 42)
	}
	if found.Version() != 0 {
		t.Errorf("Version() = %d, want %d", found.Version(), 0)
	}
}

func TestInMemoryRepository_FindByID_NotFound(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ddderror.ErrNotFound) {
		t.Errorf("error should wrap ErrNotFound, got: %v", err)
	}
}

func TestInMemoryRepository_Delete(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("agg-1")
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := repo.Delete(ctx, "agg-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	_, err := repo.FindByID(ctx, "agg-1")
	if !errors.Is(err, ddderror.ErrNotFound) {
		t.Errorf("after delete, FindByID should wrap ErrNotFound, got: %v", err)
	}
}

func TestInMemoryRepository_Delete_NotFound(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ddderror.ErrNotFound) {
		t.Errorf("error should wrap ErrNotFound, got: %v", err)
	}
}

func TestInMemoryRepository_OptimisticLock(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("agg-1")
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	duplicate := newTestAggregate("agg-1")
	err := repo.Save(ctx, duplicate)
	if err == nil {
		t.Fatal("expected optimistic lock error, got nil")
	}

	var ole *rep.OptimisticLockError
	if !errors.As(err, &ole) {
		t.Fatalf("expected *OptimisticLockError, got %T: %v", err, err)
	}
	if ole.AggregateID != "agg-1" {
		t.Errorf("AggregateID = %q, want %q", ole.AggregateID, "agg-1")
	}
	if !errors.Is(err, ddderror.ErrConcurrency) {
		t.Errorf("OptimisticLockError should unwrap to ErrConcurrency, got: %v", err)
	}
}

func TestInMemoryRepository_UpdateExistingAggregate(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("agg-1")
	agg.Name = "initial"
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	loaded, err := repo.FindByID(ctx, "agg-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	loaded.Name = "updated"
	loaded.GetAggregateRoot().SetSnapshotVersion(loaded.Version() + 1)
	if err := repo.Save(ctx, loaded); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	again, err := repo.FindByID(ctx, "agg-1")
	if err != nil {
		t.Fatalf("FindByID after update: %v", err)
	}
	if again.Name != "updated" {
		t.Errorf("Name = %q, want %q", again.Name, "updated")
	}
}

func TestInMemoryEventSourcedRepository_SaveAndLoad(t *testing.T) {
	repo := NewEventSourcedRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("es-1")
	agg.Name = "event-sourced"
	agg.Count = 7
	if err := agg.Apply(context.Background(), event.NewBaseEvent("es-1", time.Now())); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if len(agg.UncommittedEvents()) != 0 {
		t.Error("expected uncommitted events to be cleared after Save")
	}

	loaded, err := repo.Load(ctx, "es-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.ID() != "es-1" {
		t.Errorf("GetID() = %q, want %q", loaded.ID(), "es-1")
	}
	if loaded.Name != "event-sourced" {
		t.Errorf("Name = %q, want %q", loaded.Name, "event-sourced")
	}
	if loaded.Count != 7 {
		t.Errorf("Count = %d, want %d", loaded.Count, 7)
	}
}

func TestInMemoryEventSourcedRepository_Load_NotFound(t *testing.T) {
	repo := NewEventSourcedRepository[*testAggregate]()
	ctx := context.Background()

	_, err := repo.Load(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ddderror.ErrNotFound) {
		t.Errorf("error should wrap ErrNotFound, got: %v", err)
	}
}

func TestInMemoryRepository_Contract(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	repositorytest.TestRepositoryContract(t, repo,
		func(id string) *testAggregate { return newTestAggregate(id) },
		func(agg *testAggregate) { agg.Name = "test-name"; agg.Count = 42 },
	)
}

func TestInMemoryEventSourcedRepository_Contract(t *testing.T) {
	repo := NewEventSourcedRepository[*testAggregate]()
	repositorytest.TestEventSourcingRepositoryContract(t, repo,
		func(id string) *testAggregate { return newTestAggregate(id) },
		func(agg *testAggregate) {
			agg.Apply(context.Background(), event.NewBaseEvent(agg.ID(), time.Now()))
		},
	)
}

func TestInMemoryEventSourcedRepository_Save_NoEvents(t *testing.T) {
	repo := NewEventSourcedRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("es-1")
	if err := repo.Save(ctx, agg); err != nil {
		t.Errorf("Save with no uncommitted events should be no-op, got: %v", err)
	}
}

func TestInMemoryRepository_DeepCopyIsolation_FindByID(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("iso-1")
	agg.Name = "original"
	agg.Count = 10
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, err := repo.FindByID(ctx, "iso-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	found.Name = "mutated"
	found.Count = 999

	again, err := repo.FindByID(ctx, "iso-1")
	if err != nil {
		t.Fatalf("FindByID second: %v", err)
	}
	if again.Name != "original" {
		t.Errorf("mutation leaked: Name = %q, want %q", again.Name, "original")
	}
	if again.Count != 10 {
		t.Errorf("mutation leaked: Count = %d, want %d", again.Count, 10)
	}
}

func TestInMemoryRepository_DeepCopyIsolation_Save(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("iso-2")
	agg.Name = "original"
	agg.Count = 10
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	agg.Name = "mutated-after-save"
	agg.Count = 999

	found, err := repo.FindByID(ctx, "iso-2")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}
	if found.Name != "original" {
		t.Errorf("mutation after Save leaked: Name = %q, want %q", found.Name, "original")
	}
	if found.Count != 10 {
		t.Errorf("mutation after Save leaked: Count = %d, want %d", found.Count, 10)
	}
}

func TestInMemoryRepository_DeepCopyIsolation_ApplierSelfReference(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("iso-3")
	agg.Name = "with-applier"
	if err := agg.Apply(context.Background(), event.NewBaseEvent("iso-3", time.Now())); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	found, err := repo.FindByID(ctx, "iso-3")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	if err := found.Apply(context.Background(), event.NewBaseEvent("iso-3", time.Now())); err != nil {
		t.Fatalf("Apply on loaded aggregate should work, got: %v", err)
	}
}

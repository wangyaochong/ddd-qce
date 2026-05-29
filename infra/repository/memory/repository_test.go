package memory

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	ddderror "github.com/ddd-qce/core/error"
	"github.com/ddd-qce/core/domain/aggregate"
	domainevent "github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/domain/repository"
	"github.com/ddd-qce/core/domain/repository/repositorytest"
	rep "github.com/ddd-qce/core/infra/repository"
)

type testAggregate struct {
	aggregate.AggregateRoot
	Name  string `json:"name"`
	Count int    `json:"count"`
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

func (a *testAggregate) When(_ domainevent.Event) error { return nil }

func (a *testAggregate) Apply(ctx context.Context, evt domainevent.Event) error {
	return aggregate.ApplyChange(a, ctx, evt)
}

func (a *testAggregate) LoadFromHistory(events []domainevent.Event) error {
	return aggregate.LoadFromHistory(a, events)
}

func (a *testAggregate) MarshalJSON() ([]byte, error) {
	return aggregate.MarshalAggregate(a)
}

func (a *testAggregate) UnmarshalJSON(data []byte) error {
	return aggregate.UnmarshalAggregate(data, a)
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

	loaded, err := repo.FindByID(ctx, "agg-1")
	if err != nil {
		t.Fatalf("FindByID: %v", err)
	}

	duplicate := newTestAggregate("agg-1")
	err = repo.Save(ctx, duplicate)
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

	if err := loaded.Apply(ctx, event.NewBaseEvent("agg-1", time.Now())); err != nil {
		t.Fatalf("Apply on loaded: %v", err)
	}
	if err := repo.Save(ctx, loaded); err != nil {
		t.Fatalf("update after failed duplicate Save: %v", err)
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
	if err := loaded.Apply(ctx, event.NewBaseEvent("agg-1", time.Now())); err != nil {
		t.Fatalf("Apply: %v", err)
	}

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

func TestInMemoryRepository_ConcurrentOptimisticLock(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("concurrent-1")
	agg.Name = "initial"
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("initial Save: %v", err)
	}

	loadedA, err := repo.FindByID(ctx, "concurrent-1")
	if err != nil {
		t.Fatalf("FindByID A: %v", err)
	}
	loadedB, err := repo.FindByID(ctx, "concurrent-1")
	if err != nil {
		t.Fatalf("FindByID B: %v", err)
	}

	loadedA.Name = "updated-by-A"
	if err := loadedA.Apply(ctx, event.NewBaseEvent("concurrent-1", time.Now())); err != nil {
		t.Fatalf("Apply A: %v", err)
	}

	loadedB.Name = "updated-by-B"
	if err := loadedB.Apply(ctx, event.NewBaseEvent("concurrent-1", time.Now())); err != nil {
		t.Fatalf("Apply B: %v", err)
	}

	errA := repo.Save(ctx, loadedA)
	errB := repo.Save(ctx, loadedB)

	if errA != nil {
		t.Fatalf("first save should succeed, got: %v", errA)
	}

	if errB == nil {
		t.Fatal("second save should fail with optimistic lock error, got nil")
	}
	var ole *rep.OptimisticLockError
	if !errors.As(errB, &ole) {
		t.Fatalf("expected *OptimisticLockError, got %T: %v", errB, errB)
	}
}

func TestInMemoryEventSourcedRepository_OptimisticLock(t *testing.T) {
	repo := NewEventSourcedRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("es-lock-1")
	if err := agg.Apply(ctx, event.NewBaseEvent("es-lock-1", time.Now())); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	loaded, err := repo.Load(ctx, "es-lock-1")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := loaded.Apply(ctx, event.NewBaseEvent("es-lock-1", time.Now())); err != nil {
		t.Fatalf("Apply on loaded: %v", err)
	}
	if err := repo.Save(ctx, loaded); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	stale := newTestAggregate("es-lock-1")
	stale.GetAggregateRoot().SetSnapshotVersion(0)
	if err := stale.Apply(ctx, event.NewBaseEvent("es-lock-1", time.Now())); err != nil {
		t.Fatalf("Apply on stale: %v", err)
	}
	err = repo.Save(ctx, stale)
	if err == nil {
		t.Fatal("expected optimistic lock error for stale aggregate, got nil")
	}
	var ole *rep.OptimisticLockError
	if !errors.As(err, &ole) {
		t.Fatalf("expected *OptimisticLockError, got %T: %v", err, err)
	}
}

type failingSerializer[T aggregate.AggregateRef] struct {
	serializeErr   error
	deserializeErr error
}

func (f *failingSerializer[T]) Serialize(_ T) ([]byte, error) {
	if f.serializeErr != nil {
		return nil, f.serializeErr
	}
	return json.Marshal(newTestAggregate("fake"))
}

func (f *failingSerializer[T]) Deserialize(_ []byte) (T, error) {
	var zero T
	if f.deserializeErr != nil {
		return zero, f.deserializeErr
	}
	return zero, nil
}

func TestWithSerializer_SetsCustomSerializer(t *testing.T) {
	custom := &failingSerializer[*testAggregate]{}
	repo := NewRepository[*testAggregate](WithSerializer[*testAggregate](custom))
	if repo.serializer != custom {
		t.Error("WithSerializer did not set the custom serializer")
	}
}

func TestWithSerializer_DefaultSerializer(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	if _, ok := repo.serializer.(repository.JSONSerializer[*testAggregate]); !ok {
		t.Errorf("expected default JSONSerializer, got %T", repo.serializer)
	}
}

func TestWithSerializer_DeepCopySerializeError(t *testing.T) {
	serializeErr := errors.New("serialize failed")
	repo := NewRepository[*testAggregate](WithSerializer[*testAggregate](
		&failingSerializer[*testAggregate]{serializeErr: serializeErr},
	))
	ctx := context.Background()

	agg := newTestAggregate("ser-err")
	err := repo.Save(ctx, agg)
	if err == nil {
		t.Fatal("expected error from Save with failing serializer, got nil")
	}
	if !errors.Is(err, serializeErr) {
		t.Errorf("expected error wrapping serializeErr, got: %v", err)
	}
}

func TestWithSerializer_DeepCopyDeserializeError(t *testing.T) {
	deserializeErr := errors.New("deserialize failed")
	repo := NewRepository[*testAggregate](WithSerializer[*testAggregate](
		&failingSerializer[*testAggregate]{deserializeErr: deserializeErr},
	))
	ctx := context.Background()

	agg := newTestAggregate("deser-err")
	err := repo.Save(ctx, agg)
	if err == nil {
		t.Fatal("expected error from Save with failing deserializer, got nil")
	}
	if !errors.Is(err, deserializeErr) {
		t.Errorf("expected error wrapping deserializeErr, got: %v", err)
	}
}

func TestWithSerializer_FindByIDDeserializeError(t *testing.T) {
	repo := NewRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("find-deser")
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	deserializeErr := errors.New("deserialize failed")
	repo.serializer = &failingSerializer[*testAggregate]{deserializeErr: deserializeErr}

	_, err := repo.FindByID(ctx, "find-deser")
	if err == nil {
		t.Fatal("expected error from FindByID with failing deserializer, got nil")
	}
	if !errors.Is(err, deserializeErr) {
		t.Errorf("expected error wrapping deserializeErr, got: %v", err)
	}
}

func TestWithEventSourcedSerializer_SetsCustomSerializer(t *testing.T) {
	custom := &failingSerializer[*testAggregate]{}
	repo := NewEventSourcedRepository[*testAggregate](WithEventSourcedSerializer[*testAggregate](custom))
	if repo.serializer != custom {
		t.Error("WithEventSourcedSerializer did not set the custom serializer")
	}
}

func TestWithEventSourcedSerializer_DefaultSerializer(t *testing.T) {
	repo := NewEventSourcedRepository[*testAggregate]()
	if _, ok := repo.serializer.(repository.JSONSerializer[*testAggregate]); !ok {
		t.Errorf("expected default JSONSerializer, got %T", repo.serializer)
	}
}

func TestWithEventSourcedSerializer_DeepCopySerializeError(t *testing.T) {
	serializeErr := errors.New("serialize failed")
	repo := NewEventSourcedRepository[*testAggregate](WithEventSourcedSerializer[*testAggregate](
		&failingSerializer[*testAggregate]{serializeErr: serializeErr},
	))
	ctx := context.Background()

	agg := newTestAggregate("es-ser-err")
	if err := agg.Apply(ctx, event.NewBaseEvent("es-ser-err", time.Now())); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	err := repo.Save(ctx, agg)
	if err == nil {
		t.Fatal("expected error from Save with failing serializer, got nil")
	}
	if !errors.Is(err, serializeErr) {
		t.Errorf("expected error wrapping serializeErr, got: %v", err)
	}
}

func TestWithEventSourcedSerializer_DeepCopyDeserializeError(t *testing.T) {
	deserializeErr := errors.New("deserialize failed")
	repo := NewEventSourcedRepository[*testAggregate](WithEventSourcedSerializer[*testAggregate](
		&failingSerializer[*testAggregate]{deserializeErr: deserializeErr},
	))
	ctx := context.Background()

	agg := newTestAggregate("es-deser-err")
	if err := agg.Apply(ctx, event.NewBaseEvent("es-deser-err", time.Now())); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	err := repo.Save(ctx, agg)
	if err == nil {
		t.Fatal("expected error from Save with failing deserializer, got nil")
	}
	if !errors.Is(err, deserializeErr) {
		t.Errorf("expected error wrapping deserializeErr, got: %v", err)
	}
}

func TestWithEventSourcedSerializer_LoadDeserializeError(t *testing.T) {
	repo := NewEventSourcedRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("es-load-deser")
	if err := agg.Apply(ctx, event.NewBaseEvent("es-load-deser", time.Now())); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	deserializeErr := errors.New("deserialize failed")
	repo.serializer = &failingSerializer[*testAggregate]{deserializeErr: deserializeErr}

	_, err := repo.Load(ctx, "es-load-deser")
	if err == nil {
		t.Fatal("expected error from Load with failing deserializer, got nil")
	}
	if !errors.Is(err, deserializeErr) {
		t.Errorf("expected error wrapping deserializeErr, got: %v", err)
	}
}

func TestInMemoryEventSourcedRepository_ConcurrentOptimisticLock(t *testing.T) {
	repo := NewEventSourcedRepository[*testAggregate]()
	ctx := context.Background()

	agg := newTestAggregate("es-concurrent-1")
	if err := agg.Apply(ctx, event.NewBaseEvent("es-concurrent-1", time.Now())); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := repo.Save(ctx, agg); err != nil {
		t.Fatalf("first Save: %v", err)
	}

	loadedA, err := repo.Load(ctx, "es-concurrent-1")
	if err != nil {
		t.Fatalf("Load A: %v", err)
	}
	loadedB, err := repo.Load(ctx, "es-concurrent-1")
	if err != nil {
		t.Fatalf("Load B: %v", err)
	}

	if err := loadedA.Apply(ctx, event.NewBaseEvent("es-concurrent-1", time.Now())); err != nil {
		t.Fatalf("Apply A: %v", err)
	}
	if err := loadedB.Apply(ctx, event.NewBaseEvent("es-concurrent-1", time.Now())); err != nil {
		t.Fatalf("Apply B: %v", err)
	}

	errA := repo.Save(ctx, loadedA)
	errB := repo.Save(ctx, loadedB)

	if errA != nil {
		t.Fatalf("first save should succeed, got: %v", errA)
	}

	if errB == nil {
		t.Fatal("second save should fail with optimistic lock error, got nil")
	}
	var ole *rep.OptimisticLockError
	if !errors.As(errB, &ole) {
		t.Fatalf("expected *OptimisticLockError, got %T: %v", errB, errB)
	}
}

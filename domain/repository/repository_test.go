package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/domain/aggregate"
	domainevent "github.com/ddd-qce/core/domain/event"
)

type TestAggregate struct {
	aggregate.AggregateRoot
	Name    string
	Version int
}

func NewTestAggregate(id, name string) *TestAggregate {
	ta := &TestAggregate{
		Name:    name,
		Version: 0,
	}
	ar, err := aggregate.NewAggregateRoot(id)
	if err != nil {
		panic(err)
	}
	ta.AggregateRoot = *ar
	return ta
}

func (a *TestAggregate) When(_ domainevent.Event) error { return nil }

func (a *TestAggregate) Apply(ctx context.Context, evt domainevent.Event) error {
	return aggregate.ApplyChange(a, ctx, evt)
}

func (a *TestAggregate) LoadFromHistory(events []domainevent.Event) error {
	return aggregate.LoadFromHistory(a, events)
}

type TestEvent struct {
	cqrsevent.BaseEvent
}

type InMemoryRepository struct {
	mu         sync.Mutex
	aggregates map[string]*TestAggregate
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		aggregates: make(map[string]*TestAggregate),
	}
}

func (r *InMemoryRepository) Save(ctx context.Context, agg *TestAggregate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.aggregates[agg.ID()] = agg
	return nil
}

func (r *InMemoryRepository) FindByID(ctx context.Context, id string) (*TestAggregate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	agg, exists := r.aggregates[id]
	if !exists {
		return nil, fmt.Errorf("aggregate %s not found", id)
	}
	return agg, nil
}

func (r *InMemoryRepository) Delete(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.aggregates, id)
	return nil
}

type InMemoryEventSourcingRepository struct {
	mu     sync.Mutex
	events map[string][]domainevent.Event
}

func NewInMemoryEventSourcingRepository() *InMemoryEventSourcingRepository {
	return &InMemoryEventSourcingRepository{
		events: make(map[string][]domainevent.Event),
	}
}

func (r *InMemoryEventSourcingRepository) Save(ctx context.Context, agg *TestAggregate) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	events := agg.UncommittedEvents()
	r.events[agg.ID()] = append(r.events[agg.ID()], events...)
	agg.MarkEventsAsCommitted()
	return nil
}

func (r *InMemoryEventSourcingRepository) Load(ctx context.Context, id string) (*TestAggregate, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	events := r.events[id]
	if len(events) == 0 {
		return nil, fmt.Errorf("aggregate %s not found", id)
	}
	agg := NewTestAggregate(id, "")
	agg.LoadFromHistory(events)
	return agg, nil
}

func TestRepository_SaveAndFindByID(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	agg := NewTestAggregate("agg-001", "Test Aggregate")

	err := repo.Save(ctx, agg)
	if err != nil {
		t.Fatalf("unexpected error on save: %v", err)
	}

	loaded, err := repo.FindByID(ctx, "agg-001")
	if err != nil {
		t.Fatalf("unexpected error on find: %v", err)
	}
	if loaded.ID() != "agg-001" {
		t.Errorf("expected ID 'agg-001', got %s", loaded.ID())
	}
	if loaded.Name != "Test Aggregate" {
		t.Errorf("expected name 'Test Aggregate', got %s", loaded.Name)
	}
}

func TestRepository_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	_, err := repo.FindByID(ctx, "agg-999")
	if err == nil {
		t.Fatal("expected error for non-existent aggregate")
	}
}

func TestRepository_Delete(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	agg := NewTestAggregate("agg-002", "To Delete")
	repo.Save(ctx, agg)

	err := repo.Delete(ctx, "agg-002")
	if err != nil {
		t.Fatalf("unexpected error on delete: %v", err)
	}

	_, err = repo.FindByID(ctx, "agg-002")
	if err == nil {
		t.Fatal("expected error after delete")
	}
}

func TestEventSourcingRepository_SaveAndLoad(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryEventSourcingRepository()

	agg := NewTestAggregate("agg-003", "Event Sourced")
	agg.Apply(context.Background(), &TestEvent{
		BaseEvent: cqrsevent.NewBaseEvent("agg-003", time.Now()),
	})

	err := repo.Save(ctx, agg)
	if err != nil {
		t.Fatalf("unexpected error on save: %v", err)
	}

	loaded, err := repo.Load(ctx, "agg-003")
	if err != nil {
		t.Fatalf("unexpected error on load: %v", err)
	}
	if loaded.ID() != "agg-003" {
		t.Errorf("expected ID 'agg-003', got %s", loaded.ID())
	}
}

func TestEventSourcingRepository_Load_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryEventSourcingRepository()

	_, err := repo.Load(ctx, "agg-999")
	if err == nil {
		t.Fatal("expected error for non-existent aggregate")
	}
}

var _ Repository[*TestAggregate] = (*InMemoryRepository)(nil)
var _ EventSourcingRepository[*TestAggregate] = (*InMemoryEventSourcingRepository)(nil)

type JSONTestAggregate struct {
	aggregate.AggregateRoot
	Name    string `json:"name"`
	Version int    `json:"version"`
}

func NewJSONTestAggregate(id, name string) *JSONTestAggregate {
	a := &JSONTestAggregate{Name: name, Version: 0}
	ar, err := aggregate.NewAggregateRoot(id)
	if err != nil {
		panic(err)
	}
	a.AggregateRoot = *ar
	return a
}

func (a *JSONTestAggregate) When(_ domainevent.Event) error { return nil }

func (a *JSONTestAggregate) Apply(ctx context.Context, evt domainevent.Event) error {
	return aggregate.ApplyChange(a, ctx, evt)
}

func (a *JSONTestAggregate) LoadFromHistory(events []domainevent.Event) error {
	return aggregate.LoadFromHistory(a, events)
}

func (a *JSONTestAggregate) MarshalJSON() ([]byte, error) {
	return aggregate.MarshalAggregate(a)
}

func (a *JSONTestAggregate) UnmarshalJSON(data []byte) error {
	return aggregate.UnmarshalAggregate(data, a)
}

func TestJSONSerializer_SerializeAndDeserialize(t *testing.T) {
	serializer := JSONSerializer[*JSONTestAggregate]{}
	agg := NewJSONTestAggregate("agg-100", "Serialize Test")
	agg.Version = 3

	data, err := serializer.Serialize(agg)
	if err != nil {
		t.Fatalf("unexpected error on serialize: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty serialized data")
	}

	result, err := serializer.Deserialize(data)
	if err != nil {
		t.Fatalf("unexpected error on deserialize: %v", err)
	}
	if result.Name != "Serialize Test" {
		t.Errorf("expected Name 'Serialize Test', got %s", result.Name)
	}
	if result.Version != 3 {
		t.Errorf("expected Version 3, got %d", result.Version)
	}
}

func TestJSONSerializer_Serialize_NilAggregate(t *testing.T) {
	serializer := JSONSerializer[*JSONTestAggregate]{}

	data, err := serializer.Serialize(nil)
	if err != nil {
		t.Fatalf("unexpected error on serialize nil: %v", err)
	}
	if string(data) != "null" {
		t.Errorf("expected 'null', got %s", string(data))
	}
}

func TestJSONSerializer_Deserialize_InvalidJSON(t *testing.T) {
	serializer := JSONSerializer[*JSONTestAggregate]{}

	_, err := serializer.Deserialize([]byte("not valid json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestJSONSerializer_ImplementsSnapshotSerializer(t *testing.T) {
	var _ SnapshotSerializer[*JSONTestAggregate] = JSONSerializer[*JSONTestAggregate]{}
}

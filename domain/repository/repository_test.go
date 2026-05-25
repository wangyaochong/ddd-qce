package repository

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/cqrs/event"
)

type TestAggregate struct {
	*aggregate.AggregateRoot
	Name    string
	Version int
}

func NewTestAggregate(id, name string) *TestAggregate {
	ta := &TestAggregate{
		Name:    name,
		Version: 0,
	}
	ta.AggregateRoot = aggregate.NewEventCollector(id)
	return ta
}

type TestEvent struct {
	event.BaseEvent
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
	events map[string][]event.DomainEvent
}

func NewInMemoryEventSourcingRepository() *InMemoryEventSourcingRepository {
	return &InMemoryEventSourcingRepository{
		events: make(map[string][]event.DomainEvent),
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
	agg.Apply(&TestEvent{
		BaseEvent: event.NewBaseEvent("agg-003", time.Now()),
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

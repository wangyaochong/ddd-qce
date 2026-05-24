package memory

import (
	"context"
	"fmt"
	"sync"

	ddderror "github.com/ddd-qce/core/error"
	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/domain/repository"
)

type aggregateRecord[T aggregate.AggregateRef] struct {
	agg     T
	version int
}

type InMemoryRepository[T aggregate.AggregateRef] struct {
	mu    sync.RWMutex
	store map[string]*aggregateRecord[T]
}

var _ repository.Repository[aggregate.AggregateRef] = (*InMemoryRepository[aggregate.AggregateRef])(nil)

func NewRepository[T aggregate.AggregateRef]() *InMemoryRepository[T] {
	return &InMemoryRepository[T]{
		store: make(map[string]*aggregateRecord[T]),
	}
}

func (r *InMemoryRepository[T]) Save(_ context.Context, agg T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	root := agg.GetAggregateRoot()

	if existing, ok := r.store[root.GetID()]; ok {
		if root.Version() <= existing.version {
			return &OptimisticLockError{AggregateID: root.GetID(), ExpectedVersion: root.Version()}
		}
	}

	r.store[root.GetID()] = &aggregateRecord[T]{
		agg:     agg,
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

	rec.agg.GetAggregateRoot().SetSnapshotVersion(rec.version)
	return rec.agg, nil
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
	mu    sync.RWMutex
	store map[string]*aggregateRecord[T]
}

var _ repository.EventSourcingRepository[aggregate.AggregateRef] = (*InMemoryEventSourcedRepository[aggregate.AggregateRef])(nil)

func NewEventSourcedRepository[T aggregate.AggregateRef]() *InMemoryEventSourcedRepository[T] {
	return &InMemoryEventSourcedRepository[T]{
		store: make(map[string]*aggregateRecord[T]),
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

	r.store[root.GetID()] = &aggregateRecord[T]{
		agg:     agg,
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

	rec.agg.GetAggregateRoot().SetSnapshotVersion(rec.version)
	return rec.agg, nil
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

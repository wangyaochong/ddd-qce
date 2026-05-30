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

type RepoOption[T aggregate.AggregateRef] func(*InMemoryRepository[T])

func WithSerializer[T aggregate.AggregateRef](s repository.SnapshotSerializer[T]) RepoOption[T] {
	return func(r *InMemoryRepository[T]) { r.serializer = s }
}

func NewRepository[T aggregate.AggregateRef](opts ...RepoOption[T]) *InMemoryRepository[T] {
	r := &InMemoryRepository[T]{
		store:      make(map[string]*aggregateRecord[T]),
		serializer: repository.JSONSerializer[T]{},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func deepCopy[T aggregate.AggregateRef](serializer repository.SnapshotSerializer[T], agg T) (T, error) {
	data, err := serializer.Serialize(agg)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("serialize aggregate: %w", err)
	}
	copied, err := serializer.Deserialize(data)
	if err != nil {
		var zero T
		return zero, fmt.Errorf("deserialize aggregate: %w", err)
	}
	return copied, nil
}

func (r *InMemoryRepository[T]) Save(_ context.Context, agg T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	root := agg.GetAggregateRoot()
	expectedVersion := root.ExpectedVersion()

	if existing, ok := r.store[root.ID()]; ok {
		if expectedVersion != existing.version {
			return &rep.OptimisticLockError{AggregateID: root.ID(), ExpectedVersion: expectedVersion}
		}
		if root.Version() == existing.version && len(root.UncommittedEvents()) == 0 {
			return &rep.OptimisticLockError{
				AggregateID:     root.ID(),
				ExpectedVersion: expectedVersion,
				VersionMismatch: true,
			}
		}
	}

	copied, err := deepCopy(r.serializer, agg)
	if err != nil {
		return err
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

	return deepCopy(r.serializer, rec.agg)
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

type EventSourcedRepoOption[T aggregate.AggregateRef] func(*InMemoryEventSourcedRepository[T])

func WithEventSourcedSerializer[T aggregate.AggregateRef](s repository.SnapshotSerializer[T]) EventSourcedRepoOption[T] {
	return func(r *InMemoryEventSourcedRepository[T]) { r.serializer = s }
}

func NewEventSourcedRepository[T aggregate.AggregateRef](opts ...EventSourcedRepoOption[T]) *InMemoryEventSourcedRepository[T] {
	r := &InMemoryEventSourcedRepository[T]{
		store:      make(map[string]*aggregateRecord[T]),
		serializer: repository.JSONSerializer[T]{},
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *InMemoryEventSourcedRepository[T]) Save(_ context.Context, agg T) error {
	root := agg.GetAggregateRoot()
	events := root.UncommittedEvents()
	if len(events) == 0 {
		return nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	expectedVersion := root.ExpectedVersion()
	if existing, ok := r.store[root.ID()]; ok {
		if expectedVersion != existing.version {
			return &rep.OptimisticLockError{AggregateID: root.ID(), ExpectedVersion: expectedVersion}
		}
		if root.Version() == existing.version {
			return &rep.OptimisticLockError{
				AggregateID:     root.ID(),
				ExpectedVersion: expectedVersion,
				VersionMismatch: true,
			}
		}
	}

	copied, err := deepCopy(r.serializer, agg)
	if err != nil {
		return err
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

	return deepCopy(r.serializer, rec.agg)
}

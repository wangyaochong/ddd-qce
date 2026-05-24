package memory

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/ddd-qce/core/domain/event"
)

type EventStore[T event.DomainEvent] struct {
	mu      sync.RWMutex
	events  map[string][]T
	once    sync.Once
	pool    sync.Pool
	newFunc func() T
}

type EventStoreOption[T event.DomainEvent] func(*EventStore[T])

func WithFactory[T event.DomainEvent](factory func() T) EventStoreOption[T] {
	return func(s *EventStore[T]) { s.newFunc = factory }
}

func NewEventStore[T event.DomainEvent](opts ...EventStoreOption[T]) *EventStore[T] {
	s := &EventStore[T]{
		events: make(map[string][]T),
		pool: sync.Pool{
			New: func() any {
				var zero T
				v := reflect.New(reflect.TypeOf(zero).Elem()).Interface()
				typed, ok := v.(T)
				if !ok {
					panic(fmt.Sprintf("EventStore[T]: pool New returned unexpected type %T, expected %T", v, zero))
				}
				return typed
			},
		},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *EventStore[T]) assertPointerType() {
	s.once.Do(func() {
		var zero T
		if reflect.TypeOf(zero).Kind() != reflect.Ptr {
			panic(fmt.Sprintf("EventStore[T]: T must be a pointer type, got %v", reflect.TypeOf(zero)))
		}
	})
}

func (s *EventStore[T]) alloc() T {
	if s.newFunc != nil {
		return s.newFunc()
	}
	v, ok := s.pool.Get().(T)
	if !ok {
		var zero T
		panic(fmt.Sprintf("EventStore[T]: pool returned unexpected type, expected %T", zero))
	}
	return v
}

func (s *EventStore[T]) copyEvent(src T) T {
	dst := s.alloc()
	reflect.ValueOf(dst).Elem().Set(reflect.ValueOf(src).Elem())
	return dst
}

func (s *EventStore[T]) Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error {
	s.assertPointerType()
	s.mu.Lock()
	defer s.mu.Unlock()

	current := len(s.events[aggregateID])
	if current != expectedVersion {
		return fmt.Errorf("concurrency conflict: expected version %d but got %d for aggregate %s", expectedVersion, current, aggregateID)
	}

	for _, evt := range events {
		aggID := evt.AggregateID()
		s.events[aggID] = append(s.events[aggID], evt)
	}
	return nil
}

func (s *EventStore[T]) Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error) {
	s.assertPointerType()
	s.mu.RLock()
	defer s.mu.RUnlock()

	events, exists := s.events[aggregateID]
	if !exists {
		return nil, fmt.Errorf("no events found for aggregate: %s", aggregateID)
	}

	if afterVersion >= len(events) {
		return []T{}, nil
	}

	result := make([]T, len(events[afterVersion:]))
	for i, e := range events[afterVersion:] {
		result[i] = s.copyEvent(e)
	}

	return result, nil
}

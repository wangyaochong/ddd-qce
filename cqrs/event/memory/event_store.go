package memory

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/ddd-qce/core/domain/event"
)

type EventStore[T event.DomainEvent] struct {
	mu     sync.RWMutex
	events map[string][]T
	once   sync.Once
}

func NewEventStore[T event.DomainEvent]() *EventStore[T] {
	return &EventStore[T]{
		events: make(map[string][]T),
	}
}

func (s *EventStore[T]) assertPointerType() {
	s.once.Do(func() {
		var zero T
		if reflect.TypeOf(zero).Kind() != reflect.Ptr {
			panic(fmt.Sprintf("EventStore[T]: T must be a pointer type, got %v", reflect.TypeOf(zero)))
		}
	})
}

func (s *EventStore[T]) Append(ctx context.Context, events []T) error {
	s.assertPointerType()
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, evt := range events {
		aggregateID := evt.AggregateID()
		s.events[aggregateID] = append(s.events[aggregateID], evt)
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
		val := reflect.New(reflect.TypeOf(e).Elem()).Interface().(T)
		reflect.ValueOf(val).Elem().Set(reflect.ValueOf(e).Elem())
		result[i] = val
	}

	return result, nil
}

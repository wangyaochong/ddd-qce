package memory

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	ddderror "github.com/ddd-qce/core/error"
	"github.com/ddd-qce/core/cqrs/event"
)

type globalEntry[T event.Event] struct {
	position int64
	event    T
}

type EventSourceStore[T event.Event] struct {
	mu           sync.RWMutex
	events       map[string][]T
	globalEvents []globalEntry[T]
	nextPosition int64
	pool         sync.Pool
	newFunc      func() T
	shallowCopy  bool
}

type EventSourceStoreOption[T event.Event] func(*EventSourceStore[T])

func WithFactory[T event.Event](factory func() T) EventSourceStoreOption[T] {
	return func(s *EventSourceStore[T]) { s.newFunc = factory }
}

func NewEventSourceStore[T event.Event](opts ...EventSourceStoreOption[T]) (*EventSourceStore[T], error) {
	var zero T
	t := reflect.TypeOf(zero)

	s := &EventSourceStore[T]{
		events: make(map[string][]T),
	}

	if t == nil {
		s.shallowCopy = true
		for _, opt := range opts {
			opt(s)
		}
		return s, nil
	}

	if t.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("EventSourceStore[T]: T must be a pointer type, got %v", t)
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.newFunc != nil {
		return s, nil
	}

	s.pool = sync.Pool{
		New: func() any {
			return reflect.New(t.Elem()).Interface()
		},
	}

	if v := s.pool.Get(); v != nil {
		if _, ok := v.(T); !ok {
			return nil, fmt.Errorf("EventSourceStore[T]: pool New returned unexpected type %T, expected %T", v, zero)
		}
		s.pool.Put(v)
	}

	return s, nil
}

func (s *EventSourceStore[T]) alloc() (T, error) {
	if s.newFunc != nil {
		return s.newFunc(), nil
	}
	v, ok := s.pool.Get().(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("EventSourceStore[T]: pool returned unexpected type, expected %T", zero)
	}
	return v, nil
}

func (s *EventSourceStore[T]) copyEvent(src T) (T, error) {
	dst, err := s.alloc()
	if err != nil {
		var zero T
		return zero, err
	}
	reflect.ValueOf(dst).Elem().Set(reflect.ValueOf(src).Elem())
	return dst, nil
}

func (s *EventSourceStore[T]) Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := len(s.events[aggregateID])
	if current != expectedVersion {
		return fmt.Errorf("concurrency conflict: expected version %d but got %d for aggregate %s: %w", expectedVersion, current, aggregateID, ddderror.ErrConcurrency)
	}

	for _, evt := range events {
		aggID := evt.AggregateID()
		var eventToStore T
		if s.shallowCopy {
			eventToStore = evt
		} else {
			copied, err := s.copyEvent(evt)
			if err != nil {
				return fmt.Errorf("copy event: %w", err)
			}
			eventToStore = copied
		}
		s.events[aggID] = append(s.events[aggID], eventToStore)
		s.nextPosition++
		s.globalEvents = append(s.globalEvents, globalEntry[T]{
			position: s.nextPosition,
			event:    eventToStore,
		})
	}
	return nil
}

func (s *EventSourceStore[T]) Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events, exists := s.events[aggregateID]
	if !exists || afterVersion >= len(events) {
		return []T{}, nil
	}

	slice := events[afterVersion:]
	if s.shallowCopy {
		result := make([]T, len(slice))
		copy(result, slice)
		return result, nil
	}

	result := make([]T, len(slice))
	for i, e := range slice {
		copied, err := s.copyEvent(e)
		if err != nil {
			return nil, fmt.Errorf("copy event at index %d: %w", i, err)
		}
		result[i] = copied
	}

	return result, nil
}

func (s *EventSourceStore[T]) LoadAll(ctx context.Context, afterPosition int64, limit int) ([]event.GlobalEvent[T], error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	startIdx := len(s.globalEvents)
	for i, entry := range s.globalEvents {
		if entry.position > afterPosition {
			startIdx = i
			break
		}
	}

	if startIdx >= len(s.globalEvents) {
		return []event.GlobalEvent[T]{}, nil
	}

	endIdx := len(s.globalEvents)
	if limit > 0 && startIdx+limit < endIdx {
		endIdx = startIdx + limit
	}

	result := make([]event.GlobalEvent[T], endIdx-startIdx)
	for i := startIdx; i < endIdx; i++ {
		entry := s.globalEvents[i]
		var evt T
		if s.shallowCopy {
			evt = entry.event
		} else {
			copied, err := s.copyEvent(entry.event)
			if err != nil {
				return nil, fmt.Errorf("copy event at position %d: %w", entry.position, err)
			}
			evt = copied
		}
		result[i-startIdx] = event.GlobalEvent[T]{
			Position: entry.position,
			Event:    evt,
		}
	}
	return result, nil
}
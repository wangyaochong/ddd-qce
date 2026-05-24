package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/ddd-qce/core/domain/event"
)

type DomainEventStore struct {
	mu     sync.RWMutex
	events map[string][]event.DomainEvent
}

func NewDomainEventStore() *DomainEventStore {
	return &DomainEventStore{
		events: make(map[string][]event.DomainEvent),
	}
}

func (s *DomainEventStore) Append(ctx context.Context, aggregateID string, expectedVersion int, events []event.DomainEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	current := len(s.events[aggregateID])
	if current != expectedVersion {
		return fmt.Errorf("concurrency conflict: expected version %d but got %d for aggregate %s", expectedVersion, current, aggregateID)
	}

	for _, evt := range events {
		s.events[evt.AggregateID()] = append(s.events[evt.AggregateID()], evt)
	}
	return nil
}

func (s *DomainEventStore) Load(ctx context.Context, aggregateID string, afterVersion int) ([]event.DomainEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	events, exists := s.events[aggregateID]
	if !exists {
		return nil, fmt.Errorf("no events found for aggregate: %s", aggregateID)
	}

	if afterVersion >= len(events) {
		return []event.DomainEvent{}, nil
	}

	result := make([]event.DomainEvent, len(events[afterVersion:]))
	copy(result, events[afterVersion:])
	return result, nil
}

var _ event.DomainEventAppendOnlyStore = (*DomainEventStore)(nil)

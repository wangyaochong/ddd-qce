package event

import (
	"context"
	"sync"
	"testing"
	"time"

	domainevent "github.com/ddd-qce/core/domain/event"
)

type OrderCreatedEvent struct {
	OrderID string
	UserID  string
	Amount  float64
}

func (e *OrderCreatedEvent) AggregateID() string   { return e.OrderID }
func (e *OrderCreatedEvent) EventType() string     { return domainevent.EventTypeOf(e) }
func (e *OrderCreatedEvent) OccurredAt() time.Time { return time.Now() }

type OrderCreatedEventHandler struct {
	mu     sync.Mutex
	events []*OrderCreatedEvent
}

func (h *OrderCreatedEventHandler) Handle(ctx context.Context, event *OrderCreatedEvent) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.events = append(h.events, event)
	return nil
}

type InMemoryEventStore struct {
	mu     sync.Mutex
	events map[string][]domainevent.DomainEvent
}

func NewInMemoryEventStore() *InMemoryEventStore {
	return &InMemoryEventStore{
		events: make(map[string][]domainevent.DomainEvent),
	}
}

func (s *InMemoryEventStore) Append(ctx context.Context, events []domainevent.DomainEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, e := range events {
		aggID := e.AggregateID()
		s.events[aggID] = append(s.events[aggID], e)
	}
	return nil
}

func (s *InMemoryEventStore) Load(ctx context.Context, aggregateID string, afterVersion int) ([]domainevent.DomainEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	events := s.events[aggregateID]
	if afterVersion >= len(events) {
		return nil, nil
	}
	return events[afterVersion:], nil
}

func TestHandler_Handle(t *testing.T) {
	ctx := context.Background()
	handler := &OrderCreatedEventHandler{}

	event := &OrderCreatedEvent{
		OrderID: "ORD-001",
		UserID:  "user-001",
		Amount:  99.99,
	}

	err := handler.Handle(ctx, event)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	handler.mu.Lock()
	defer handler.mu.Unlock()
	if len(handler.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(handler.events))
	}
	if handler.events[0].OrderID != "ORD-001" {
		t.Errorf("expected OrderID 'ORD-001', got %s", handler.events[0].OrderID)
	}
}

func TestStore_AppendAndLoad(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryEventStore()

	events := []domainevent.DomainEvent{
		&OrderCreatedEvent{OrderID: "ORD-001", UserID: "user-001", Amount: 99.99},
		&OrderCreatedEvent{OrderID: "ORD-001", UserID: "user-001", Amount: 199.99},
	}

	err := store.Append(ctx, events)
	if err != nil {
		t.Fatalf("unexpected error on append: %v", err)
	}

	loaded, err := store.Load(ctx, "ORD-001", 0)
	if err != nil {
		t.Fatalf("unexpected error on load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 events, got %d", len(loaded))
	}

	loadedAfterV1, err := store.Load(ctx, "ORD-001", 1)
	if err != nil {
		t.Fatalf("unexpected error on load after version: %v", err)
	}
	if len(loadedAfterV1) != 1 {
		t.Fatalf("expected 1 event after version 1, got %d", len(loadedAfterV1))
	}
}

func TestStore_Load_NonExistent(t *testing.T) {
	ctx := context.Background()
	store := NewInMemoryEventStore()

	loaded, err := store.Load(ctx, "ORD-999", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if loaded != nil {
		t.Errorf("expected nil for non-existent aggregate, got %v", loaded)
	}
}

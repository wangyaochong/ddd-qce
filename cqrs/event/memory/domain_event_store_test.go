package memory

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/domain/event"
)

type placedEvent struct {
	OrderID string
}

func (e *placedEvent) AggregateID() string   { return e.OrderID }
func (e *placedEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *placedEvent) OccurredAt() time.Time { return time.Now() }

type confirmedEvent struct {
	OrderID string
}

func (e *confirmedEvent) AggregateID() string   { return e.OrderID }
func (e *confirmedEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *confirmedEvent) OccurredAt() time.Time { return time.Now() }

type shippedEvent struct {
	OrderID string
}

func (e *shippedEvent) AggregateID() string   { return e.OrderID }
func (e *shippedEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *shippedEvent) OccurredAt() time.Time { return time.Now() }

func TestDomainEventStore_AppendAndLoad(t *testing.T) {
	store := NewDomainEventStore()
	ctx := context.Background()

	err := store.Append(ctx, "O1", 0, []event.DomainEvent{
		&placedEvent{OrderID: "O1"},
	})
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	events, err := store.Load(ctx, "O1", 0)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if _, ok := events[0].(*placedEvent); !ok {
		t.Errorf("expected *placedEvent, got %T", events[0])
	}
}

func TestDomainEventStore_MultipleEventTypes(t *testing.T) {
	store := NewDomainEventStore()
	ctx := context.Background()

	err := store.Append(ctx, "O1", 0, []event.DomainEvent{
		&placedEvent{OrderID: "O1"},
		&confirmedEvent{OrderID: "O1"},
		&shippedEvent{OrderID: "O1"},
	})
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	events, err := store.Load(ctx, "O1", 0)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if _, ok := events[0].(*placedEvent); !ok {
		t.Errorf("event 0: expected *placedEvent, got %T", events[0])
	}
	if _, ok := events[1].(*confirmedEvent); !ok {
		t.Errorf("event 1: expected *confirmedEvent, got %T", events[1])
	}
	if _, ok := events[2].(*shippedEvent); !ok {
		t.Errorf("event 2: expected *shippedEvent, got %T", events[2])
	}
}

func TestDomainEventStore_VersionConflict(t *testing.T) {
	store := NewDomainEventStore()
	ctx := context.Background()

	store.Append(ctx, "O1", 0, []event.DomainEvent{&placedEvent{OrderID: "O1"}})

	err := store.Append(ctx, "O1", 0, []event.DomainEvent{&confirmedEvent{OrderID: "O1"}})
	if err == nil {
		t.Fatal("expected version conflict error")
	}
}

func TestDomainEventStore_AfterVersion(t *testing.T) {
	store := NewDomainEventStore()
	ctx := context.Background()

	store.Append(ctx, "O1", 0, []event.DomainEvent{
		&placedEvent{OrderID: "O1"},
		&confirmedEvent{OrderID: "O1"},
		&shippedEvent{OrderID: "O1"},
	})

	events, err := store.Load(ctx, "O1", 1)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events after version 1, got %d", len(events))
	}
	if _, ok := events[0].(*confirmedEvent); !ok {
		t.Errorf("expected *confirmedEvent, got %T", events[0])
	}
}

func TestDomainEventStore_NotFound(t *testing.T) {
	store := NewDomainEventStore()
	ctx := context.Background()

	_, err := store.Load(ctx, "O999", 0)
	if err == nil {
		t.Fatal("expected error for missing aggregate")
	}
}

func TestDomainEventStore_AppendIncrementsVersion(t *testing.T) {
	store := NewDomainEventStore()
	ctx := context.Background()

	store.Append(ctx, "O1", 0, []event.DomainEvent{&placedEvent{OrderID: "O1"}})
	store.Append(ctx, "O1", 1, []event.DomainEvent{&confirmedEvent{OrderID: "O1"}})
	store.Append(ctx, "O1", 2, []event.DomainEvent{&shippedEvent{OrderID: "O1"}})

	events, _ := store.Load(ctx, "O1", 0)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
}

func TestDomainEventStore_AfterVersionExceeds(t *testing.T) {
	store := NewDomainEventStore()
	ctx := context.Background()

	store.Append(ctx, "O1", 0, []event.DomainEvent{&placedEvent{OrderID: "O1"}})

	events, err := store.Load(ctx, "O1", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

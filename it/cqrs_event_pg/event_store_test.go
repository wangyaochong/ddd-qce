package pg

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ddd-qce/core/cqrs/event"
	pgevent "github.com/ddd-qce/core/cqrs/impl/pg"
	"github.com/ddd-qce/core/cqrs/event/eventtest"
	"github.com/ddd-qce/it/testutil"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var _ event.EventSourceStore[*testDomainEvent] = (*pgevent.EventSourceStore[*testDomainEvent])(nil)

type testDomainEvent struct {
	event.BaseEvent
	EData string
}

func (e *testDomainEvent) AggregateID() string   { return e.BaseEvent.AggregateID() }
func (e *testDomainEvent) OccurredAt() time.Time { return e.BaseEvent.OccurredAt() }

func openTestDBForEventStore(t *testing.T) *sql.DB {
	return testutil.OpenTestDB(t, "ddd_qce_event_test")
}

func TestPgEventStore_AppendAndLoad(t *testing.T) {
	db := openTestDBForEventStore(t)
	store, err := pgevent.NewEventSourceStore[*testDomainEvent](db)
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	events := []*testDomainEvent{
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now())},
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now())},
	}

	if err := store.Append(ctx, "agg-1", 0, events); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 events, got %d", len(loaded))
	}
	if event.EventTypeOf(loaded[0]) != "testDomainEvent" {
		t.Errorf("expected first event type 'testDomainEvent', got %s", event.EventTypeOf(loaded[0]))
	}
	if event.EventTypeOf(loaded[1]) != "testDomainEvent" {
		t.Errorf("expected second event type 'testDomainEvent', got %s", event.EventTypeOf(loaded[1]))
	}
}

func TestPgEventStore_LoadAfterVersion(t *testing.T) {
	db := openTestDBForEventStore(t)
	store, err := pgevent.NewEventSourceStore[*testDomainEvent](db)
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	events := []*testDomainEvent{
		{BaseEvent: event.NewBaseEvent("agg-2", time.Now())},
		{BaseEvent: event.NewBaseEvent("agg-2", time.Now())},
		{BaseEvent: event.NewBaseEvent("agg-2", time.Now())},
	}
	if err := store.Append(ctx, "agg-2", 0, events); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-2", 1)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 events after version 1, got %d", len(loaded))
	}
}

func TestPgEventStore_LoadNotFound(t *testing.T) {
	db := openTestDBForEventStore(t)
	store, err := pgevent.NewEventSourceStore[*testDomainEvent](db)
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	loaded, err := store.Load(ctx, "nonexistent", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(loaded) != 0 {
		t.Errorf("expected empty slice for nonexistent aggregate, got %d events", len(loaded))
	}
}

func TestPgEventStore_WithFactory(t *testing.T) {
	db := openTestDBForEventStore(t)
	store, err := pgevent.NewEventSourceStore[*testDomainEvent](db, pgevent.WithFactory[*testDomainEvent](func() *testDomainEvent {
		return &testDomainEvent{}
	}))
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	events := []*testDomainEvent{
		{BaseEvent: event.NewBaseEvent("agg-3", time.Now())},
	}
	if err := store.Append(ctx, "agg-3", 0, events); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-3", 0)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 event, got %d", len(loaded))
	}
}

var _ event.Event = (*testDomainEvent)(nil)

func TestPgEventStore_Contract(t *testing.T) {
	db := openTestDBForEventStore(t)
	eventtest.TestEventStoreContract(t, func() event.EventSourceStore[*eventtest.TestEvent] {
		store, err := pgevent.NewEventSourceStore[*eventtest.TestEvent](db)
		if err != nil {
			t.Fatalf("create event store: %v", err)
		}
		return store
	})
}

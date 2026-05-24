package pg

import (
	"context"
	"database/sql"
	"testing"
	"time"

	cqevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/domain/event"
	pgevent "github.com/ddd-qce/core/cqrs/event/pg"
	"github.com/ddd-qce/it/testutil"
	_ "github.com/jackc/pgx/v5/stdlib"
)

var _ cqevent.EventStore[*testDomainEvent] = (*pgevent.EventStore[*testDomainEvent])(nil)

type testDomainEvent struct {
	AggID string
	EType string
	EAt   time.Time
	EData string
}

func (e *testDomainEvent) AggregateID() string   { return e.AggID }
func (e *testDomainEvent) EventType() string     { return e.EType }
func (e *testDomainEvent) OccurredAt() time.Time { return e.EAt }

func openTestDBForEventStore(t *testing.T) *sql.DB {
	return testutil.OpenTestDB(t, "ddd_qce_event_test")
}

func TestPgEventStore_AppendAndLoad(t *testing.T) {
	db := openTestDBForEventStore(t)
	store, err := pgevent.NewEventStore[*testDomainEvent](db)
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	events := []*testDomainEvent{
		{AggID: "agg-1", EType: "Created", EAt: time.Now()},
		{AggID: "agg-1", EType: "Updated", EAt: time.Now()},
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
	if loaded[0].EType != "Created" {
		t.Errorf("expected first event type 'Created', got %s", loaded[0].EType)
	}
	if loaded[1].EType != "Updated" {
		t.Errorf("expected second event type 'Updated', got %s", loaded[1].EType)
	}
}

func TestPgEventStore_LoadAfterVersion(t *testing.T) {
	db := openTestDBForEventStore(t)
	store, err := pgevent.NewEventStore[*testDomainEvent](db)
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	events := []*testDomainEvent{
		{AggID: "agg-2", EType: "Created", EAt: time.Now()},
		{AggID: "agg-2", EType: "Updated", EAt: time.Now()},
		{AggID: "agg-2", EType: "Deleted", EAt: time.Now()},
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
	store, err := pgevent.NewEventStore[*testDomainEvent](db)
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
	store, err := pgevent.NewEventStore[*testDomainEvent](db, pgevent.WithFactory[*testDomainEvent](func() *testDomainEvent {
		return &testDomainEvent{}
	}))
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	events := []*testDomainEvent{
		{AggID: "agg-3", EType: "Created", EAt: time.Now()},
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

var _ event.DomainEvent = (*testDomainEvent)(nil)

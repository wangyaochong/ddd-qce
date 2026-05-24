package memory

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/domain/event"
)

var _ event.EventStore[*testStoreEvent] = (*EventStore[*testStoreEvent])(nil)

type testStoreEvent struct {
	event.BaseEvent
	Data string
}

func TestEventStore_AppendAndLoad(t *testing.T) {
	store, err := NewEventStore[*testStoreEvent]()
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	events := []*testStoreEvent{
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "event1"},
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "event2"},
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "event3"},
	}

	err = store.Append(ctx, "agg-1", 0, events)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded) != 3 {
		t.Errorf("expected 3 events, got %d", len(loaded))
	}
}

func TestEventStore_LoadAfterVersion(t *testing.T) {
	store, err := NewEventStore[*testStoreEvent]()
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	events := []*testStoreEvent{
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "v0"},
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "v1"},
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "v2"},
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "v3"},
	}

	err = store.Append(ctx, "agg-1", 0, events)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-1", 2)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Errorf("expected 2 events after version 2, got %d", len(loaded))
	}
	if loaded[0].Data != "v2" {
		t.Errorf("expected first event data 'v2', got '%s'", loaded[0].Data)
	}
	if loaded[1].Data != "v3" {
		t.Errorf("expected second event data 'v3', got '%s'", loaded[1].Data)
	}
}

func TestEventStore_LoadNonExistentAggregate(t *testing.T) {
	store, err := NewEventStore[*testStoreEvent]()
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

func TestEventStore_AppendMultipleEvents(t *testing.T) {
	store, err := NewEventStore[*testStoreEvent]()
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	events1 := []*testStoreEvent{
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "e1"},
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "e2"},
	}
	events2 := []*testStoreEvent{
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "e3"},
	}

	err = store.Append(ctx, "agg-1", 0, events1)
	if err != nil {
		t.Fatalf("first append failed: %v", err)
	}

	err = store.Append(ctx, "agg-1", 2, events2)
	if err != nil {
		t.Fatalf("second append failed: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded) != 3 {
		t.Errorf("expected 3 events, got %d", len(loaded))
	}
}

func TestEventStore_ConcurrencyConflict(t *testing.T) {
	store, err := NewEventStore[*testStoreEvent]()
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	err = store.Append(ctx, "agg-1", 0, []*testStoreEvent{{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "e1"}})
	if err != nil {
		t.Fatalf("first append failed: %v", err)
	}

	err = store.Append(ctx, "agg-1", 0, []*testStoreEvent{{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "e2"}})
	if err == nil {
		t.Fatal("expected concurrency conflict error")
	}
}

func TestEventStore_MultipleAggregates(t *testing.T) {
	store, err := NewEventStore[*testStoreEvent]()
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	err = store.Append(ctx, "agg-1", 0, []*testStoreEvent{
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "a1-e1"},
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "a1-e2"},
	})
	if err != nil {
		t.Fatalf("append agg-1 failed: %v", err)
	}

	err = store.Append(ctx, "agg-2", 0, []*testStoreEvent{
		{BaseEvent: event.NewBaseEvent("agg-2", time.Now()), Data: "a2-e1"},
		{BaseEvent: event.NewBaseEvent("agg-2", time.Now()), Data: "a2-e2"},
	})
	if err != nil {
		t.Fatalf("append agg-2 failed: %v", err)
	}

	agg1, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("load agg-1 failed: %v", err)
	}
	if len(agg1) != 2 {
		t.Errorf("expected 2 events for agg-1, got %d", len(agg1))
	}

	agg2, err := store.Load(ctx, "agg-2", 0)
	if err != nil {
		t.Fatalf("load agg-2 failed: %v", err)
	}
	if len(agg2) != 2 {
		t.Errorf("expected 2 events for agg-2, got %d", len(agg2))
	}
}

func TestEventStore_ConcurrentAppend(t *testing.T) {
	store, err := NewEventStore[*testStoreEvent]()
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	for i := 0; i < 100; i++ {
		evt := &testStoreEvent{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: string(rune(i))}
		if err := store.Append(ctx, "agg-1", i, []*testStoreEvent{evt}); err != nil {
			t.Fatalf("append at version %d failed: %v", i, err)
		}
	}

	loaded, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded) != 100 {
		t.Errorf("expected 100 events, got %d", len(loaded))
	}
}

func TestEventStore_LoadReturnsCopy(t *testing.T) {
	store, err := NewEventStore[*testStoreEvent]()
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	events := []*testStoreEvent{
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "original"},
	}

	err = store.Append(ctx, "agg-1", 0, events)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	loaded[0].Data = "modified"

	loadedAgain, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("second load failed: %v", err)
	}

	if loadedAgain[0].Data != "original" {
		t.Errorf("expected data 'original', got '%s' (store was modified)", loadedAgain[0].Data)
	}
}

func TestEventStore_LoadAfterVersionBeyondRange(t *testing.T) {
	store, err := NewEventStore[*testStoreEvent]()
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	events := []*testStoreEvent{
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "e1"},
	}

	err = store.Append(ctx, "agg-1", 0, events)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-1", 5)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded) != 0 {
		t.Errorf("expected 0 events, got %d", len(loaded))
	}
}

type testValueEvent struct {
	event.BaseEvent
}

func TestEventStore_NonPointerTypeReturnsError(t *testing.T) {
	_, err := NewEventStore[testValueEvent]()
	if err == nil {
		t.Fatal("expected error for non-pointer type T")
	}
}

func TestEventStore_WithFactory(t *testing.T) {
	store, err := NewEventStore[*testStoreEvent](WithFactory[*testStoreEvent](func() *testStoreEvent {
		return &testStoreEvent{}
	}))
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	events := []*testStoreEvent{
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "event1"},
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "event2"},
	}

	err = store.Append(ctx, "agg-1", 0, events)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 events, got %d", len(loaded))
	}
	if loaded[0].Data != "event1" {
		t.Errorf("expected 'event1', got '%s'", loaded[0].Data)
	}
	if loaded[1].Data != "event2" {
		t.Errorf("expected 'event2', got '%s'", loaded[1].Data)
	}
}

func TestEventStore_WithFactory_LoadReturnsCopy(t *testing.T) {
	store, err := NewEventStore[*testStoreEvent](WithFactory[*testStoreEvent](func() *testStoreEvent {
		return &testStoreEvent{}
	}))
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()

	events := []*testStoreEvent{
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "original"},
	}

	err = store.Append(ctx, "agg-1", 0, events)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	loaded, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	loaded[0].Data = "modified"

	loadedAgain, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("second load failed: %v", err)
	}

	if loadedAgain[0].Data != "original" {
		t.Errorf("expected data 'original', got '%s' (store was modified)", loadedAgain[0].Data)
	}
}

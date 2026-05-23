package memory

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ddd-qce/core/domain/event"
)

type testStoreEvent struct {
	AggID string
	Data  string
}

func (e *testStoreEvent) AggregateID() string   { return e.AggID }
func (e *testStoreEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *testStoreEvent) OccurredAt() time.Time { return time.Now() }

func TestEventStore_AppendAndLoad(t *testing.T) {
	store := NewEventStore[*testStoreEvent]()
	ctx := context.Background()

	events := []*testStoreEvent{
		{AggID: "agg-1", Data: "event1"},
		{AggID: "agg-1", Data: "event2"},
		{AggID: "agg-1", Data: "event3"},
	}

	err := store.Append(ctx, events)
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
	store := NewEventStore[*testStoreEvent]()
	ctx := context.Background()

	events := []*testStoreEvent{
		{AggID: "agg-1", Data: "v0"},
		{AggID: "agg-1", Data: "v1"},
		{AggID: "agg-1", Data: "v2"},
		{AggID: "agg-1", Data: "v3"},
	}

	err := store.Append(ctx, events)
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
	store := NewEventStore[*testStoreEvent]()
	ctx := context.Background()

	_, err := store.Load(ctx, "nonexistent", 0)
	if err == nil {
		t.Fatal("expected error for non-existent aggregate")
	}
}

func TestEventStore_AppendMultipleEvents(t *testing.T) {
	store := NewEventStore[*testStoreEvent]()
	ctx := context.Background()

	events1 := []*testStoreEvent{
		{AggID: "agg-1", Data: "e1"},
		{AggID: "agg-1", Data: "e2"},
	}
	events2 := []*testStoreEvent{
		{AggID: "agg-1", Data: "e3"},
	}

	err := store.Append(ctx, events1)
	if err != nil {
		t.Fatalf("first append failed: %v", err)
	}

	err = store.Append(ctx, events2)
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

func TestEventStore_MultipleAggregates(t *testing.T) {
	store := NewEventStore[*testStoreEvent]()
	ctx := context.Background()

	events := []*testStoreEvent{
		{AggID: "agg-1", Data: "a1-e1"},
		{AggID: "agg-2", Data: "a2-e1"},
		{AggID: "agg-1", Data: "a1-e2"},
		{AggID: "agg-2", Data: "a2-e2"},
	}

	err := store.Append(ctx, events)
	if err != nil {
		t.Fatalf("append failed: %v", err)
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
	store := NewEventStore[*testStoreEvent]()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			evt := &testStoreEvent{AggID: "agg-1", Data: string(rune(id))}
			store.Append(ctx, []*testStoreEvent{evt})
		}(i)
	}

	wg.Wait()

	loaded, err := store.Load(ctx, "agg-1", 0)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if len(loaded) != 100 {
		t.Errorf("expected 100 events, got %d", len(loaded))
	}
}

func TestEventStore_LoadReturnsCopy(t *testing.T) {
	store := NewEventStore[*testStoreEvent]()
	ctx := context.Background()

	events := []*testStoreEvent{
		{AggID: "agg-1", Data: "original"},
	}

	err := store.Append(ctx, events)
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
	store := NewEventStore[*testStoreEvent]()
	ctx := context.Background()

	events := []*testStoreEvent{
		{AggID: "agg-1", Data: "e1"},
	}

	err := store.Append(ctx, events)
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
	AggID string
}

func (e testValueEvent) AggregateID() string   { return e.AggID }
func (e testValueEvent) EventType() string     { return event.EventTypeOf(e) }
func (e testValueEvent) OccurredAt() time.Time { return time.Now() }

func TestEventStore_NonPointerTypePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for non-pointer type T")
		}
	}()

	store := NewEventStore[testValueEvent]()
	store.Append(context.Background(), []testValueEvent{})
}

package impl

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ddd-qce/core/cqrs/event"
	ddderror "github.com/ddd-qce/core/error"
)

type TestEvent struct {
	event.BaseEvent
	Data string
}

func NewTestEvent(aggID, data string) *TestEvent {
	return &TestEvent{
		BaseEvent: event.NewBaseEvent(aggID, time.Now()),
		Data:      data,
	}
}

func TestEventStoreContract(t *testing.T, newStore func() event.EventSourceStore[*TestEvent]) {
	t.Helper()

	t.Run("AppendAndLoad", func(t *testing.T) {
		store := newStore()
		ctx := context.Background()

		events := []*TestEvent{
			NewTestEvent("contract-1", "e1"),
			NewTestEvent("contract-1", "e2"),
			NewTestEvent("contract-1", "e3"),
		}

		if err := store.Append(ctx, "contract-1", 0, events); err != nil {
			t.Fatalf("Append failed: %v", err)
		}

		loaded, err := store.Load(ctx, "contract-1", 0)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if len(loaded) != 3 {
			t.Errorf("expected 3 events, got %d", len(loaded))
		}
	})

	t.Run("LoadAfterVersion", func(t *testing.T) {
		store := newStore()
		ctx := context.Background()

		events := []*TestEvent{
			NewTestEvent("contract-2", "v0"),
			NewTestEvent("contract-2", "v1"),
			NewTestEvent("contract-2", "v2"),
			NewTestEvent("contract-2", "v3"),
		}

		if err := store.Append(ctx, "contract-2", 0, events); err != nil {
			t.Fatalf("Append failed: %v", err)
		}

		loaded, err := store.Load(ctx, "contract-2", 2)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if len(loaded) != 2 {
			t.Fatalf("expected 2 events after version 2, got %d", len(loaded))
		}
		if loaded[0].Data != "v2" {
			t.Errorf("expected first event data 'v2', got '%s'", loaded[0].Data)
		}
		if loaded[1].Data != "v3" {
			t.Errorf("expected second event data 'v3', got '%s'", loaded[1].Data)
		}
	})

	t.Run("LoadNonExistentAggregate", func(t *testing.T) {
		store := newStore()
		ctx := context.Background()

		loaded, err := store.Load(ctx, "contract-nonexistent", 0)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(loaded) != 0 {
			t.Errorf("expected empty slice for nonexistent aggregate, got %d events", len(loaded))
		}
	})

	t.Run("AppendMultipleBatches", func(t *testing.T) {
		store := newStore()
		ctx := context.Background()

		batch1 := []*TestEvent{
			NewTestEvent("contract-3", "e0"),
			NewTestEvent("contract-3", "e1"),
		}
		batch2 := []*TestEvent{
			NewTestEvent("contract-3", "e2"),
		}

		if err := store.Append(ctx, "contract-3", 0, batch1); err != nil {
			t.Fatalf("first Append failed: %v", err)
		}
		if err := store.Append(ctx, "contract-3", 2, batch2); err != nil {
			t.Fatalf("second Append failed: %v", err)
		}

		loaded, err := store.Load(ctx, "contract-3", 0)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if len(loaded) != 3 {
			t.Errorf("expected 3 events, got %d", len(loaded))
		}
	})

	t.Run("ConcurrencyConflict", func(t *testing.T) {
		store := newStore()
		ctx := context.Background()

		if err := store.Append(ctx, "contract-4", 0, []*TestEvent{NewTestEvent("contract-4", "e1")}); err != nil {
			t.Fatalf("first Append failed: %v", err)
		}

		err := store.Append(ctx, "contract-4", 0, []*TestEvent{NewTestEvent("contract-4", "e2")})
		if err == nil {
			t.Fatal("expected concurrency conflict error")
		}
		if !errors.Is(err, ddderror.ErrConcurrency) {
			t.Errorf("expected ErrConcurrency, got: %v", err)
		}
	})

	t.Run("MultipleAggregates", func(t *testing.T) {
		store := newStore()
		ctx := context.Background()

		if err := store.Append(ctx, "contract-5a", 0, []*TestEvent{
			NewTestEvent("contract-5a", "a1-e1"),
			NewTestEvent("contract-5a", "a1-e2"),
		}); err != nil {
			t.Fatalf("Append agg1 failed: %v", err)
		}

		if err := store.Append(ctx, "contract-5b", 0, []*TestEvent{
			NewTestEvent("contract-5b", "a2-e1"),
			NewTestEvent("contract-5b", "a2-e2"),
		}); err != nil {
			t.Fatalf("Append agg2 failed: %v", err)
		}

		loaded1, err := store.Load(ctx, "contract-5a", 0)
		if err != nil {
			t.Fatalf("Load agg1 failed: %v", err)
		}
		if len(loaded1) != 2 {
			t.Errorf("expected 2 events for agg1, got %d", len(loaded1))
		}

		loaded2, err := store.Load(ctx, "contract-5b", 0)
		if err != nil {
			t.Fatalf("Load agg2 failed: %v", err)
		}
		if len(loaded2) != 2 {
			t.Errorf("expected 2 events for agg2, got %d", len(loaded2))
		}
	})

	t.Run("LoadAfterVersionBeyondRange", func(t *testing.T) {
		store := newStore()
		ctx := context.Background()

		if err := store.Append(ctx, "contract-6", 0, []*TestEvent{NewTestEvent("contract-6", "e1")}); err != nil {
			t.Fatalf("Append failed: %v", err)
		}

		loaded, err := store.Load(ctx, "contract-6", 5)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if len(loaded) != 0 {
			t.Errorf("expected 0 events, got %d", len(loaded))
		}
	})

	t.Run("EventDataRoundTrip", func(t *testing.T) {
		store := newStore()
		ctx := context.Background()

		if err := store.Append(ctx, "contract-7", 0, []*TestEvent{NewTestEvent("contract-7", "hello")}); err != nil {
			t.Fatalf("Append failed: %v", err)
		}

		loaded, err := store.Load(ctx, "contract-7", 0)
		if err != nil {
			t.Fatalf("Load failed: %v", err)
		}

		if len(loaded) != 1 {
			t.Fatalf("expected 1 event, got %d", len(loaded))
		}

		if loaded[0].Data != "hello" {
			t.Errorf("expected Data 'hello', got '%s'", loaded[0].Data)
		}
		if loaded[0].AggregateID() != "contract-7" {
			t.Errorf("expected AggregateID 'contract-7', got '%s'", loaded[0].AggregateID())
		}
	})
}
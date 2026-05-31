package integration

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/cqrs/event"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
)

type orderCreated struct {
	event.BaseEvent
	UserID string
	Amount float64
}

type orderConfirmed struct {
	event.BaseEvent
}

type orderCancelled struct {
	event.BaseEvent
	Reason string
}

type OrderState struct {
	ID     string
	UserID string
	Amount float64
	Status string
}

func (s *OrderState) ApplyCreated(event *orderCreated) {
	s.ID = event.AggregateID()
	s.UserID = event.UserID
	s.Amount = event.Amount
	s.Status = "created"
}

func (s *OrderState) ApplyConfirmed(event *orderConfirmed) {
	s.Status = "confirmed"
}

func (s *OrderState) ApplyCancelled(event *orderCancelled) {
	s.Status = "cancelled"
}

func TestEventSourcing_CreateApplySaveLoadReplay(t *testing.T) {
	ctx := context.Background()

	createdStore, err := eventmemory.NewEventSourceStore[*orderCreated]()
	if err != nil {
		t.Fatalf("create created store: %v", err)
	}
	confirmedStore, err := eventmemory.NewEventSourceStore[*orderConfirmed]()
	if err != nil {
		t.Fatalf("create confirmed store: %v", err)
	}

	createdEvents := []*orderCreated{
		{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), UserID: "user-001", Amount: 99.99},
	}
	err = createdStore.Append(ctx, "ORD-001", 0, createdEvents)
	if err != nil {
		t.Fatalf("append created failed: %v", err)
	}

	confirmedEvents := []*orderConfirmed{
		{BaseEvent: event.NewBaseEvent("ORD-001", time.Now())},
	}
	err = confirmedStore.Append(ctx, "ORD-001", 0, confirmedEvents)
	if err != nil {
		t.Fatalf("append confirmed failed: %v", err)
	}

	state := &OrderState{}

	loadedCreated, err := createdStore.Load(ctx, "ORD-001", 0)
	if err != nil {
		t.Fatalf("load created failed: %v", err)
	}
	for _, e := range loadedCreated {
		state.ApplyCreated(e)
	}

	loadedConfirmed, err := confirmedStore.Load(ctx, "ORD-001", 0)
	if err != nil {
		t.Fatalf("load confirmed failed: %v", err)
	}
	for _, e := range loadedConfirmed {
		state.ApplyConfirmed(e)
	}

	if state.ID != "ORD-001" {
		t.Errorf("expected ID 'ORD-001', got '%s'", state.ID)
	}
	if state.UserID != "user-001" {
		t.Errorf("expected UserID 'user-001', got '%s'", state.UserID)
	}
	if state.Amount != 99.99 {
		t.Errorf("expected Amount 99.99, got %.2f", state.Amount)
	}
	if state.Status != "confirmed" {
		t.Errorf("expected Status 'confirmed', got '%s'", state.Status)
	}
}

func TestEventSourcing_MultipleAggregates(t *testing.T) {
	ctx := context.Background()
	store, err := eventmemory.NewEventSourceStore[*orderCreated]()
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}

	events := []*orderCreated{
		{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), UserID: "user-001", Amount: 100.00},
		{BaseEvent: event.NewBaseEvent("ORD-002", time.Now()), UserID: "user-002", Amount: 200.00},
		{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), UserID: "user-001", Amount: 150.00},
		{BaseEvent: event.NewBaseEvent("ORD-002", time.Now()), UserID: "user-002", Amount: 250.00},
	}

	err = store.Append(ctx, "ORD-001", 0, events)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	agg1, err := store.Load(ctx, "ORD-001", 0)
	if err != nil {
		t.Fatalf("load ORD-001 failed: %v", err)
	}
	if len(agg1) != 2 {
		t.Errorf("expected 2 events for ORD-001, got %d", len(agg1))
	}

	agg2, err := store.Load(ctx, "ORD-002", 0)
	if err != nil {
		t.Fatalf("load ORD-002 failed: %v", err)
	}
	if len(agg2) != 2 {
		t.Errorf("expected 2 events for ORD-002, got %d", len(agg2))
	}
}

func TestEventSourcing_Versioning(t *testing.T) {
	ctx := context.Background()
	store, err := eventmemory.NewEventSourceStore[*orderCreated]()
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}

	events := []*orderCreated{
		{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), UserID: "user-001", Amount: 100.00},
		{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), UserID: "user-001", Amount: 200.00},
		{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), UserID: "user-001", Amount: 300.00},
	}

	err = store.Append(ctx, "ORD-001", 0, events)
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}

	all, err := store.Load(ctx, "ORD-001", 0)
	if err != nil {
		t.Fatalf("load all failed: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 events from version 0, got %d", len(all))
	}

	afterV1, err := store.Load(ctx, "ORD-001", 1)
	if err != nil {
		t.Fatalf("load after v1 failed: %v", err)
	}
	if len(afterV1) != 2 {
		t.Errorf("expected 2 events after version 1, got %d", len(afterV1))
	}

	afterV2, err := store.Load(ctx, "ORD-001", 2)
	if err != nil {
		t.Fatalf("load after v2 failed: %v", err)
	}
	if len(afterV2) != 1 {
		t.Errorf("expected 1 event after version 2, got %d", len(afterV2))
	}
}

func TestEventSourcing_FullReplay(t *testing.T) {
	ctx := context.Background()
	createdStore, err := eventmemory.NewEventSourceStore[*orderCreated]()
	if err != nil {
		t.Fatalf("create created store: %v", err)
	}
	confirmedStore, err := eventmemory.NewEventSourceStore[*orderConfirmed]()
	if err != nil {
		t.Fatalf("create confirmed store: %v", err)
	}
	cancelledStore, err := eventmemory.NewEventSourceStore[*orderCancelled]()
	if err != nil {
		t.Fatalf("create cancelled store: %v", err)
	}

	createdStore.Append(ctx, "ORD-001", 0, []*orderCreated{
		{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), UserID: "user-001", Amount: 500.00},
	})
	confirmedStore.Append(ctx, "ORD-001", 0, []*orderConfirmed{
		{BaseEvent: event.NewBaseEvent("ORD-001", time.Now())},
	})
	cancelledStore.Append(ctx, "ORD-001", 0, []*orderCancelled{
		{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), Reason: "customer request"},
	})

	state := &OrderState{}

	created, _ := createdStore.Load(ctx, "ORD-001", 0)
	for _, e := range created {
		state.ApplyCreated(e)
	}

	confirmed, _ := confirmedStore.Load(ctx, "ORD-001", 0)
	for _, e := range confirmed {
		state.ApplyConfirmed(e)
	}

	cancelled, _ := cancelledStore.Load(ctx, "ORD-001", 0)
	for _, e := range cancelled {
		state.ApplyCancelled(e)
	}

	if state.Status != "cancelled" {
		t.Errorf("expected final status 'cancelled', got '%s'", state.Status)
	}
	if state.Amount != 500.00 {
		t.Errorf("expected amount 500.00, got %.2f", state.Amount)
	}
}

package integration

import (
	"context"
	"testing"
	"time"

	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
	"github.com/ddd-qce/core/domain/event"
)

type orderCreated struct {
	OrderID string
	UserID  string
	Amount  float64
}

func (e *orderCreated) AggregateID() string   { return e.OrderID }
func (e *orderCreated) EventType() string     { return event.EventTypeOf(e) }
func (e *orderCreated) OccurredAt() time.Time { return time.Now() }

type orderConfirmed struct {
	OrderID string
}

func (e *orderConfirmed) AggregateID() string   { return e.OrderID }
func (e *orderConfirmed) EventType() string     { return event.EventTypeOf(e) }
func (e *orderConfirmed) OccurredAt() time.Time { return time.Now() }

type orderCancelled struct {
	OrderID string
	Reason  string
}

func (e *orderCancelled) AggregateID() string   { return e.OrderID }
func (e *orderCancelled) EventType() string     { return event.EventTypeOf(e) }
func (e *orderCancelled) OccurredAt() time.Time { return time.Now() }

type OrderState struct {
	ID     string
	UserID string
	Amount float64
	Status string
}

func (s *OrderState) ApplyCreated(event *orderCreated) {
	s.ID = event.OrderID
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

	createdStore := eventmemory.NewEventStore[*orderCreated]()
	confirmedStore := eventmemory.NewEventStore[*orderConfirmed]()

	createdEvents := []*orderCreated{
		{OrderID: "ORD-001", UserID: "user-001", Amount: 99.99},
	}
	err := createdStore.Append(ctx, createdEvents)
	if err != nil {
		t.Fatalf("append created failed: %v", err)
	}

	confirmedEvents := []*orderConfirmed{
		{OrderID: "ORD-001"},
	}
	err = confirmedStore.Append(ctx, confirmedEvents)
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
	store := eventmemory.NewEventStore[*orderCreated]()

	events := []*orderCreated{
		{OrderID: "ORD-001", UserID: "user-001", Amount: 100.00},
		{OrderID: "ORD-002", UserID: "user-002", Amount: 200.00},
		{OrderID: "ORD-001", UserID: "user-001", Amount: 150.00},
		{OrderID: "ORD-002", UserID: "user-002", Amount: 250.00},
	}

	err := store.Append(ctx, events)
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
	store := eventmemory.NewEventStore[*orderCreated]()

	events := []*orderCreated{
		{OrderID: "ORD-001", UserID: "user-001", Amount: 100.00},
		{OrderID: "ORD-001", UserID: "user-001", Amount: 200.00},
		{OrderID: "ORD-001", UserID: "user-001", Amount: 300.00},
	}

	err := store.Append(ctx, events)
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
	createdStore := eventmemory.NewEventStore[*orderCreated]()
	confirmedStore := eventmemory.NewEventStore[*orderConfirmed]()
	cancelledStore := eventmemory.NewEventStore[*orderCancelled]()

	createdStore.Append(ctx, []*orderCreated{
		{OrderID: "ORD-001", UserID: "user-001", Amount: 500.00},
	})
	confirmedStore.Append(ctx, []*orderConfirmed{
		{OrderID: "ORD-001"},
	})
	cancelledStore.Append(ctx, []*orderCancelled{
		{OrderID: "ORD-001", Reason: "customer request"},
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

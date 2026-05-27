package aggregate

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ddd-qce/core/cqrs/event"
)

type testDomainEvent struct {
	event.BaseEvent
}

type orderCreatedEvent struct {
	event.BaseEvent
	amount float64
}

type orderConfirmedEvent struct {
	event.BaseEvent
}

type orderCancelledEvent struct {
	event.BaseEvent
	reason string
}

type testOrder struct {
	AggregateRoot
	Status string
	Amount float64
	Reason string
}

func newTestOrder(id string) *testOrder {
	o := &testOrder{}
	ar, err := NewAggregateRoot(id)
	if err != nil {
		panic(err)
	}
	o.AggregateRoot = *ar
	return o
}

func (o *testOrder) When(evt event.Event) error {
	switch e := evt.(type) {
	case *orderCreatedEvent:
		o.Status = "created"
		o.Amount = e.amount
	case *orderConfirmedEvent:
		o.Status = "confirmed"
	case *orderCancelledEvent:
		o.Status = "cancelled"
		o.Reason = e.reason
	default:
		return fmt.Errorf("testOrder: unhandled event type %T", evt)
	}
	return nil
}

func (o *testOrder) Apply(ctx context.Context, evt event.Event) error {
	return ApplyChange(o, ctx, evt)
}

func (o *testOrder) LoadFromHistory(events []event.Event) error {
	return LoadFromHistory(o, events)
}

type testEventCollector struct {
	AggregateRoot
}

func newTestEventCollector(id string) *testEventCollector {
	c := &testEventCollector{}
	ar, err := NewAggregateRoot(id)
	if err != nil {
		panic(err)
	}
	c.AggregateRoot = *ar
	return c
}

func (c *testEventCollector) When(_ event.Event) error { return nil }

func (c *testEventCollector) Apply(ctx context.Context, evt event.Event) error {
	return ApplyChange(c, ctx, evt)
}

func (c *testEventCollector) LoadFromHistory(events []event.Event) error {
	return LoadFromHistory(c, events)
}

func TestNewAggregateRoot_Initial(t *testing.T) {
	agg, err := NewAggregateRoot("order-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if agg.ID() != "order-1" {
		t.Errorf("expected ID 'order-1', got '%s'", agg.ID())
	}
	if agg.Version() != 0 {
		t.Errorf("expected version 0, got %d", agg.Version())
	}
}

func TestApply_SingleEvent(t *testing.T) {
	agg := newTestEventCollector("order-1")
	evt := &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())}

	_ = agg.Apply(context.Background(), evt)

	if agg.Version() != 1 {
		t.Errorf("expected version 1 after apply, got %d", agg.Version())
	}

	events := agg.UncommittedEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 uncommitted event, got %d", len(events))
	}
	if events[0] != evt {
		t.Error("expected uncommitted event to match applied event")
	}
}

func TestApply_MultipleEvents(t *testing.T) {
	agg := newTestEventCollector("order-1")

	evt1 := &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())}
	evt2 := &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())}
	evt3 := &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())}

	_ = agg.Apply(context.Background(), evt1)
	_ = agg.Apply(context.Background(), evt2)
	_ = agg.Apply(context.Background(), evt3)

	if agg.Version() != 3 {
		t.Errorf("expected version 3 after 3 applies, got %d", agg.Version())
	}

	events := agg.UncommittedEvents()
	if len(events) != 3 {
		t.Fatalf("expected 3 uncommitted events, got %d", len(events))
	}
}

func TestUncommittedEvents_ReturnsCopy(t *testing.T) {
	agg := newTestEventCollector("order-1")
	evt := &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())}
	_ = agg.Apply(context.Background(), evt)

	events1 := agg.UncommittedEvents()
	events1[0] = nil

	events2 := agg.UncommittedEvents()
	if events2[0] == nil {
		t.Error("expected UncommittedEvents to return a copy, mutation affected original")
	}
}

func TestUncommittedEvents_Empty(t *testing.T) {
	agg := newTestEventCollector("order-1")

	events := agg.UncommittedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 uncommitted events, got %d", len(events))
	}
}

func TestMarkEventsAsCommitted(t *testing.T) {
	agg := newTestEventCollector("order-1")
	_ = agg.Apply(context.Background(), &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())})
	_ = agg.Apply(context.Background(), &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())})

	if len(agg.UncommittedEvents()) != 2 {
		t.Fatal("expected 2 uncommitted events before marking")
	}

	agg.MarkEventsAsCommitted()

	events := agg.UncommittedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 uncommitted events after marking, got %d", len(events))
	}
}

func TestMarkEventsAsCommitted_Empty(t *testing.T) {
	agg := newTestEventCollector("order-1")
	agg.MarkEventsAsCommitted()

	events := agg.UncommittedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 uncommitted events after marking, got %d", len(events))
	}
}

func TestLoadFromHistory_Empty(t *testing.T) {
	agg := newTestEventCollector("order-1")
	_ = agg.LoadFromHistory([]event.Event{})

	if agg.Version() != 0 {
		t.Errorf("expected version 0 after loading empty history, got %d", agg.Version())
	}
}

func TestLoadFromHistory_MultipleEvents(t *testing.T) {
	agg := newTestEventCollector("order-1")

	events := []event.Event{
		&testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())},
		&testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())},
		&testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())},
	}

	_ = agg.LoadFromHistory(events)

	if agg.Version() != 3 {
		t.Errorf("expected version 3 after loading 3 events, got %d", agg.Version())
	}

	eventsAfter := agg.UncommittedEvents()
	if len(eventsAfter) != 0 {
		t.Errorf("expected 0 uncommitted events after loading history, got %d", len(eventsAfter))
	}
}

func TestValidate_Valid(t *testing.T) {
	agg := newTestEventCollector("order-1")
	_ = agg.Apply(context.Background(), &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())})

	err := agg.Validate()
	if err != nil {
		t.Errorf("expected no error for valid aggregate, got: %v", err)
	}
}

func TestValidate_EmptyID(t *testing.T) {
	_, err := NewAggregateRoot("")
	if err == nil {
		t.Error("expected error for empty aggregate ID")
	}
}

func TestValidate_NegativeVersion(t *testing.T) {
	agg := newTestEventCollector("order-1")
	agg.SetSnapshotVersion(-1)
	err := agg.Validate()
	if err == nil {
		t.Fatal("expected error for negative version")
	}
	if err.Error() != "aggregate version cannot be negative" {
		t.Errorf("expected 'aggregate version cannot be negative', got '%v'", err)
	}
}

func TestApply_NewEventMutatesState(t *testing.T) {
	order := newTestOrder("ORD-001")

	_ = order.Apply(context.Background(), &orderCreatedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), amount: 99.99})

	if order.Status != "created" {
		t.Errorf("expected status 'created', got '%s'", order.Status)
	}
	if order.Amount != 99.99 {
		t.Errorf("expected amount 99.99, got %.2f", order.Amount)
	}
	if order.Version() != 1 {
		t.Errorf("expected version 1, got %d", order.Version())
	}

	_ = order.Apply(context.Background(), &orderConfirmedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now())})

	if order.Status != "confirmed" {
		t.Errorf("expected status 'confirmed', got '%s'", order.Status)
	}
	if order.Version() != 2 {
		t.Errorf("expected version 2, got %d", order.Version())
	}
}

func TestLoadFromHistory_StateRebuild(t *testing.T) {
	order := newTestOrder("ORD-001")

	history := []event.Event{
		&orderCreatedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), amount: 500.00},
		&orderConfirmedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now())},
		&orderCancelledEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), reason: "customer request"},
	}

	_ = order.LoadFromHistory(history)

	if order.Status != "cancelled" {
		t.Errorf("expected status 'cancelled', got '%s'", order.Status)
	}
	if order.Amount != 500.00 {
		t.Errorf("expected amount 500.00, got %.2f", order.Amount)
	}
	if order.Reason != "customer request" {
		t.Errorf("expected reason 'customer request', got '%s'", order.Reason)
	}
	if order.Version() != 3 {
		t.Errorf("expected version 3, got %d", order.Version())
	}
	if len(order.UncommittedEvents()) != 0 {
		t.Errorf("expected 0 uncommitted events after LoadFromHistory, got %d", len(order.UncommittedEvents()))
	}
}

func TestApply_ThenLoadFromHistory(t *testing.T) {
	order := newTestOrder("ORD-001")

	_ = order.Apply(context.Background(), &orderCreatedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), amount: 100.00})

	if order.Status != "created" {
		t.Errorf("expected status 'created', got '%s'", order.Status)
	}

	uncommitted := order.UncommittedEvents()
	order.MarkEventsAsCommitted()

	rebuilt := newTestOrder("ORD-001")
	_ = rebuilt.LoadFromHistory(uncommitted)

	if rebuilt.Status != "created" {
		t.Errorf("expected rebuilt status 'created', got '%s'", rebuilt.Status)
	}
	if rebuilt.Amount != 100.00 {
		t.Errorf("expected rebuilt amount 100.00, got %.2f", rebuilt.Amount)
	}
}

func TestNewAggregateRoot(t *testing.T) {
	agg, err := NewAggregateRoot("order-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	evt := &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())}

	collector := newTestEventCollector(agg.ID())
	_ = collector.Apply(context.Background(), evt)

	if collector.Version() != 1 {
		t.Errorf("expected version 1, got %d", collector.Version())
	}

	history := []event.Event{
		&testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())},
		&testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())},
	}
	_ = collector.LoadFromHistory(history)

	if collector.Version() != 3 {
		t.Errorf("expected version 3, got %d", collector.Version())
	}
}

func TestAggregateRoot_OrderLifecycle(t *testing.T) {
	order := newTestOrder("order-123")

	_ = order.Apply(context.Background(), &orderCreatedEvent{BaseEvent: event.NewBaseEvent("order-123", time.Now()), amount: 100})
	_ = order.Apply(context.Background(), &orderConfirmedEvent{BaseEvent: event.NewBaseEvent("order-123", time.Now())})
	_ = order.Apply(context.Background(), &orderCancelledEvent{BaseEvent: event.NewBaseEvent("order-123", time.Now()), reason: "test"})

	if order.Version() != 3 {
		t.Errorf("expected version 3, got %d", order.Version())
	}

	uncommitted := order.UncommittedEvents()
	if len(uncommitted) != 3 {
		t.Fatalf("expected 3 uncommitted events, got %d", len(uncommitted))
	}

	order.MarkEventsAsCommitted()
	if len(order.UncommittedEvents()) != 0 {
		t.Error("expected no uncommitted events after commit")
	}

	rebuilt := newTestOrder("order-123")
	_ = rebuilt.LoadFromHistory(uncommitted)
	if rebuilt.Version() != 3 {
		t.Errorf("expected rebuilt version 3, got %d", rebuilt.Version())
	}

	err := rebuilt.Validate()
	if err != nil {
		t.Errorf("expected valid rebuilt aggregate, got: %v", err)
	}
}

func TestAggregateRoot_Equals_SameID(t *testing.T) {
	agg1 := newTestEventCollector("order-1")
	agg2 := newTestEventCollector("order-1")

	if !agg1.GetAggregateRoot().Equals(agg2.GetAggregateRoot()) {
		t.Error("expected aggregates with same ID to be equal")
	}
}

func TestAggregateRoot_Equals_DifferentID(t *testing.T) {
	agg1 := newTestEventCollector("order-1")
	agg2 := newTestEventCollector("order-2")

	if agg1.GetAggregateRoot().Equals(agg2.GetAggregateRoot()) {
		t.Error("expected aggregates with different IDs to not be equal")
	}
}

func TestAggregateRoot_Equals_NilReceiver(t *testing.T) {
	var agg1 *AggregateRoot
	agg2 := newTestEventCollector("order-1")

	if agg1.Equals(agg2.GetAggregateRoot()) {
		t.Error("expected nil receiver to not equal non-nil")
	}
}

func TestAggregateRoot_Equals_NilOther(t *testing.T) {
	agg1 := newTestEventCollector("order-1")
	var agg2 *AggregateRoot

	if agg1.GetAggregateRoot().Equals(agg2) {
		t.Error("expected non-nil to not equal nil")
	}
}

func TestAggregateRoot_Equals_BothNil(t *testing.T) {
	var agg1 *AggregateRoot
	var agg2 *AggregateRoot

	if !agg1.Equals(agg2) {
		t.Error("expected both nil to be equal")
	}
}

func TestCloneAggregate(t *testing.T) {
	order := newTestOrder("ORD-001")
	order.Apply(context.Background(), &orderCreatedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), amount: 100})

	clone := CloneAggregate(order)
	if clone == nil {
		t.Error("CloneAggregate should not return nil")
	}
	if clone.ID() != "ORD-001" {
		t.Errorf("clone ID = %s, want ORD-001", clone.ID())
	}
	if clone.Version() != 1 {
		t.Errorf("clone version = %d, want 1", clone.Version())
	}
}

func TestAggregateRoot_Validate_EmptyID(t *testing.T) {
	_, err := NewAggregateRoot("")
	if err == nil {
		t.Error("expected error for empty aggregate ID")
	}
}

func TestAggregateRoot_Validate_NegativeVersion(t *testing.T) {
	agg := newTestEventCollector("order-1")
	agg.SetSnapshotVersion(-1)
	err := agg.Validate()
	if err == nil {
		t.Error("Validate() should error for negative version")
	}
}

func TestAggregateRoot_Clone(t *testing.T) {
	agg := newTestEventCollector("order-1")
	agg.Apply(context.Background(), &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())})

	clone := agg.GetAggregateRoot().Clone()
	if clone == nil {
		t.Error("Clone() should not return nil")
	}
	if clone.Version() != 1 {
		t.Errorf("clone version = %d, want 1", clone.Version())
	}
}

func TestApply_EventHandlerError(t *testing.T) {
	order := newTestOrder("ORD-001")
	unknownEvent := &testDomainEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now())}

	err := order.Apply(context.Background(), unknownEvent)
	if err == nil {
		t.Error("Apply() should return error for unhandled event type")
	}
}

func TestLoadFromHistory_Error(t *testing.T) {
	order := newTestOrder("ORD-001")
	unknownEvent := &testDomainEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now())}

	err := order.LoadFromHistory([]event.Event{unknownEvent})
	if err == nil {
		t.Error("LoadFromHistory() should return error for unhandled event type")
	}
}

func TestApply_CollectsUncommittedEvents(t *testing.T) {
	order := newTestOrder("ORD-001")
	evt1 := &orderCreatedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), amount: 100}
	evt2 := &orderConfirmedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now())}

	_ = order.Apply(context.Background(), evt1)
	_ = order.Apply(context.Background(), evt2)

	events := order.UncommittedEvents()
	if len(events) != 2 {
		t.Fatalf("expected 2 uncommitted events, got %d", len(events))
	}
}

func TestMarkEventsAsCommitted_ClearsEvents(t *testing.T) {
	order := newTestOrder("ORD-001")
	_ = order.Apply(context.Background(), &orderCreatedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), amount: 100})

	order.MarkEventsAsCommitted()

	events := order.UncommittedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 uncommitted events after MarkEventsAsCommitted, got %d", len(events))
	}
}

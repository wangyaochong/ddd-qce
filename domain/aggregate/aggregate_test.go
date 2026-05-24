package aggregate

import (
	"testing"
	"time"

	"github.com/ddd-qce/core/domain/event"
)

type testDomainEvent struct {
	event.BaseEvent
}

func TestNewEventCollector_Initial(t *testing.T) {
	agg := NewEventCollector("order-1")
	if agg.GetID() != "order-1" {
		t.Errorf("expected ID 'order-1', got '%s'", agg.GetID())
	}
	if agg.Version() != 0 {
		t.Errorf("expected version 0, got %d", agg.Version())
	}
}

func TestApply_SingleEvent(t *testing.T) {
	agg := NewEventCollector("order-1")
	evt := &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())}

	_ = agg.Apply(evt)

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
	agg := NewEventCollector("order-1")

	evt1 := &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())}
	evt2 := &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())}
	evt3 := &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())}

	_ = agg.Apply(evt1)
	_ = agg.Apply(evt2)
	_ = agg.Apply(evt3)

	if agg.Version() != 3 {
		t.Errorf("expected version 3 after 3 applies, got %d", agg.Version())
	}

	events := agg.UncommittedEvents()
	if len(events) != 3 {
		t.Fatalf("expected 3 uncommitted events, got %d", len(events))
	}
}

func TestUncommittedEvents_ReturnsCopy(t *testing.T) {
	agg := NewEventCollector("order-1")
	evt := &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())}
	_ = agg.Apply(evt)

	events1 := agg.UncommittedEvents()
	events1[0] = nil

	events2 := agg.UncommittedEvents()
	if events2[0] == nil {
		t.Error("expected UncommittedEvents to return a copy, mutation affected original")
	}
}

func TestUncommittedEvents_Empty(t *testing.T) {
	agg := NewEventCollector("order-1")

	events := agg.UncommittedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 uncommitted events, got %d", len(events))
	}
}

func TestMarkEventsAsCommitted(t *testing.T) {
	agg := NewEventCollector("order-1")
	_ = agg.Apply(&testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())})
	_ = agg.Apply(&testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())})

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
	agg := NewEventCollector("order-1")
	agg.MarkEventsAsCommitted()

	events := agg.UncommittedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 uncommitted events, got %d", len(events))
	}
}

func TestLoadFromHistory_Empty(t *testing.T) {
	agg := NewEventCollector("order-1")
	_ = agg.LoadFromHistory([]event.DomainEvent{})

	if agg.Version() != 0 {
		t.Errorf("expected version 0 after loading empty history, got %d", agg.Version())
	}
}

func TestLoadFromHistory_MultipleEvents(t *testing.T) {
	agg := NewEventCollector("order-1")

	events := []event.DomainEvent{
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
	agg := NewEventCollector("order-1")
	_ = agg.Apply(&testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())})

	err := agg.Validate()
	if err != nil {
		t.Errorf("expected no error for valid aggregate, got: %v", err)
	}
}

func TestValidate_EmptyID(t *testing.T) {
	agg := NewEventCollector("")
	err := agg.Validate()
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if err.Error() != "aggregate: entity ID cannot be empty" {
		t.Errorf("expected 'aggregate: entity ID cannot be empty', got '%v'", err)
	}
}

func TestValidate_NegativeVersion(t *testing.T) {
	agg := NewEventCollector("order-1")
	agg.SetSnapshotVersion(-1)
	err := agg.Validate()
	if err == nil {
		t.Fatal("expected error for negative version")
	}
	if err.Error() != "aggregate version cannot be negative" {
		t.Errorf("expected 'aggregate version cannot be negative', got '%v'", err)
	}
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
	o.AggregateRoot = *NewAggregateRootWithApplier(id, o)
	return o
}

func (o *testOrder) When(evt event.DomainEvent) {
	switch e := evt.(type) {
	case *orderCreatedEvent:
		o.Status = "created"
		o.Amount = e.amount
	case *orderConfirmedEvent:
		o.Status = "confirmed"
	case *orderCancelledEvent:
		o.Status = "cancelled"
		o.Reason = e.reason
	}
}

func TestApply_NewEventMutatesState(t *testing.T) {
	order := newTestOrder("ORD-001")

	_ = order.Apply(&orderCreatedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), amount: 99.99})

	if order.Status != "created" {
		t.Errorf("expected status 'created', got '%s'", order.Status)
	}
	if order.Amount != 99.99 {
		t.Errorf("expected amount 99.99, got %.2f", order.Amount)
	}
	if order.Version() != 1 {
		t.Errorf("expected version 1, got %d", order.Version())
	}

	_ = order.Apply(&orderConfirmedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now())})

	if order.Status != "confirmed" {
		t.Errorf("expected status 'confirmed', got '%s'", order.Status)
	}
	if order.Version() != 2 {
		t.Errorf("expected version 2, got %d", order.Version())
	}
}

func TestLoadFromHistory_StateRebuild(t *testing.T) {
	order := newTestOrder("ORD-001")

	history := []event.DomainEvent{
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

	_ = order.Apply(&orderCreatedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), amount: 100.00})

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

func TestNewEventCollector(t *testing.T) {
	agg := NewEventCollector("order-1")
	evt := &testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())}

	_ = agg.Apply(evt)

	if agg.Version() != 1 {
		t.Errorf("expected version 1, got %d", agg.Version())
	}

	history := []event.DomainEvent{
		&testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())},
		&testDomainEvent{BaseEvent: event.NewBaseEvent("order-1", time.Now())},
	}
	_ = agg.LoadFromHistory(history)

	if agg.Version() != 3 {
		t.Errorf("expected version 3, got %d", agg.Version())
	}
}

func TestAggregateRoot_OrderLifecycle(t *testing.T) {
	order := NewEventCollector("order-123")

	_ = order.Apply(&testDomainEvent{BaseEvent: event.NewBaseEvent("order-123", time.Now())})
	_ = order.Apply(&testDomainEvent{BaseEvent: event.NewBaseEvent("order-123", time.Now())})
	_ = order.Apply(&testDomainEvent{BaseEvent: event.NewBaseEvent("order-123", time.Now())})
	_ = order.Apply(&testDomainEvent{BaseEvent: event.NewBaseEvent("order-123", time.Now())})

	if order.Version() != 4 {
		t.Errorf("expected version 4, got %d", order.Version())
	}

	uncommitted := order.UncommittedEvents()
	if len(uncommitted) != 4 {
		t.Fatalf("expected 4 uncommitted events, got %d", len(uncommitted))
	}

	order.MarkEventsAsCommitted()
	if len(order.UncommittedEvents()) != 0 {
		t.Error("expected no uncommitted events after commit")
	}

	rebuilt := NewEventCollector("order-123")
	_ = rebuilt.LoadFromHistory(uncommitted)
	if rebuilt.Version() != 4 {
		t.Errorf("expected rebuilt version 4, got %d", rebuilt.Version())
	}

	err := rebuilt.Validate()
	if err != nil {
		t.Errorf("expected valid rebuilt aggregate, got: %v", err)
	}
}

func TestNewAggregateRootWithApplier(t *testing.T) {
	o := &testOrder{}
	o.AggregateRoot = *NewAggregateRootWithApplier("ORD-001", o)

	_ = o.Apply(&orderCreatedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), amount: 99.99})

	if o.Status != "created" {
		t.Errorf("expected status 'created', got '%s'", o.Status)
	}
	if o.Version() != 1 {
		t.Errorf("expected version 1, got %d", o.Version())
	}
}

func TestSetApplier_StillWorks(t *testing.T) {
	o := &testOrder{}
	o.AggregateRoot = *NewAggregateRootWithApplier("ORD-001", o)

	_ = o.Apply(&orderCreatedEvent{BaseEvent: event.NewBaseEvent("ORD-001", time.Now()), amount: 50.00})

	if o.Status != "created" {
		t.Errorf("expected status 'created', got '%s'", o.Status)
	}
}

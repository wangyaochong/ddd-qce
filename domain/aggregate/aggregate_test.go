package aggregate

import (
	"testing"
	"time"

	"github.com/ddd-qce/core/domain/event"
)

type testDomainEvent struct {
	aggregateID string
	occurredAt  time.Time
}

func (e *testDomainEvent) AggregateID() string   { return e.aggregateID }
func (e *testDomainEvent) EventType() string     { return "TestEvent" }
func (e *testDomainEvent) OccurredAt() time.Time { return e.occurredAt }

func TestNewAggregateRoot(t *testing.T) {
	agg := NewAggregateRoot("order-1")
	if agg.ID != "order-1" {
		t.Errorf("expected ID 'order-1', got '%s'", agg.ID)
	}
	if agg.Version != 0 {
		t.Errorf("expected version 0, got %d", agg.Version)
	}
}

func TestApply_SingleEvent(t *testing.T) {
	agg := NewEventCollector("order-1")
	evt := &testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()}

	agg.Apply(evt)

	if agg.Version != 1 {
		t.Errorf("expected version 1 after apply, got %d", agg.Version)
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

	evt1 := &testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()}
	evt2 := &testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()}
	evt3 := &testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()}

	agg.Apply(evt1)
	agg.Apply(evt2)
	agg.Apply(evt3)

	if agg.Version != 3 {
		t.Errorf("expected version 3 after 3 applies, got %d", agg.Version)
	}

	events := agg.UncommittedEvents()
	if len(events) != 3 {
		t.Fatalf("expected 3 uncommitted events, got %d", len(events))
	}
}

func TestUncommittedEvents_ReturnsCopy(t *testing.T) {
	agg := NewEventCollector("order-1")
	evt := &testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()}
	agg.Apply(evt)

	events1 := agg.UncommittedEvents()
	events1[0] = nil

	events2 := agg.UncommittedEvents()
	if events2[0] == nil {
		t.Error("expected UncommittedEvents to return a copy, mutation affected original")
	}
}

func TestUncommittedEvents_Empty(t *testing.T) {
	agg := NewAggregateRoot("order-1")

	events := agg.UncommittedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 uncommitted events, got %d", len(events))
	}
}

func TestMarkEventsAsCommitted(t *testing.T) {
	agg := NewEventCollector("order-1")
	agg.Apply(&testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()})
	agg.Apply(&testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()})

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
	agg := NewAggregateRoot("order-1")
	agg.MarkEventsAsCommitted()

	events := agg.UncommittedEvents()
	if len(events) != 0 {
		t.Errorf("expected 0 uncommitted events, got %d", len(events))
	}
}

func TestLoadFromHistory_Empty(t *testing.T) {
	agg := NewAggregateRoot("order-1")
	agg.LoadFromHistory([]event.DomainEvent{})

	if agg.Version != 0 {
		t.Errorf("expected version 0 after loading empty history, got %d", agg.Version)
	}
}

func TestLoadFromHistory_MultipleEvents(t *testing.T) {
	agg := NewEventCollector("order-1")

	events := []event.DomainEvent{
		&testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()},
		&testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()},
		&testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()},
	}

	agg.LoadFromHistory(events)

	if agg.Version != 3 {
		t.Errorf("expected version 3 after loading 3 events, got %d", agg.Version)
	}

	eventsAfter := agg.UncommittedEvents()
	if len(eventsAfter) != 0 {
		t.Errorf("expected 0 uncommitted events after loading history, got %d", len(eventsAfter))
	}
}

func TestValidate_Valid(t *testing.T) {
	agg := NewEventCollector("order-1")
	agg.Apply(&testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()})

	err := agg.Validate()
	if err != nil {
		t.Errorf("expected no error for valid aggregate, got: %v", err)
	}
}

func TestValidate_EmptyID(t *testing.T) {
	agg := NewAggregateRoot("")
	err := agg.Validate()
	if err == nil {
		t.Fatal("expected error for empty ID")
	}
	if err.Error() != "aggregate ID cannot be empty" {
		t.Errorf("expected 'aggregate ID cannot be empty', got '%v'", err)
	}
}

func TestValidate_NegativeVersion(t *testing.T) {
	agg := NewAggregateRoot("order-1")
	agg.Version = -1
	err := agg.Validate()
	if err == nil {
		t.Fatal("expected error for negative version")
	}
	if err.Error() != "aggregate version cannot be negative" {
		t.Errorf("expected 'aggregate version cannot be negative', got '%v'", err)
	}
}

type orderCreatedEvent struct {
	aggregateID string
	amount      float64
	occurredAt  time.Time
}

func (e *orderCreatedEvent) AggregateID() string   { return e.aggregateID }
func (e *orderCreatedEvent) EventType() string     { return "OrderCreated" }
func (e *orderCreatedEvent) OccurredAt() time.Time { return e.occurredAt }

type orderConfirmedEvent struct {
	aggregateID string
	occurredAt  time.Time
}

func (e *orderConfirmedEvent) AggregateID() string   { return e.aggregateID }
func (e *orderConfirmedEvent) EventType() string     { return "OrderConfirmed" }
func (e *orderConfirmedEvent) OccurredAt() time.Time { return e.occurredAt }

type orderCancelledEvent struct {
	aggregateID string
	reason      string
	occurredAt  time.Time
}

func (e *orderCancelledEvent) AggregateID() string   { return e.aggregateID }
func (e *orderCancelledEvent) EventType() string     { return "OrderCancelled" }
func (e *orderCancelledEvent) OccurredAt() time.Time { return e.occurredAt }

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

	order.Apply(&orderCreatedEvent{aggregateID: "ORD-001", amount: 99.99, occurredAt: time.Now()})

	if order.Status != "created" {
		t.Errorf("expected status 'created', got '%s'", order.Status)
	}
	if order.Amount != 99.99 {
		t.Errorf("expected amount 99.99, got %.2f", order.Amount)
	}
	if order.Version != 1 {
		t.Errorf("expected version 1, got %d", order.Version)
	}

	order.Apply(&orderConfirmedEvent{aggregateID: "ORD-001", occurredAt: time.Now()})

	if order.Status != "confirmed" {
		t.Errorf("expected status 'confirmed', got '%s'", order.Status)
	}
	if order.Version != 2 {
		t.Errorf("expected version 2, got %d", order.Version)
	}
}

func TestLoadFromHistory_StateRebuild(t *testing.T) {
	order := newTestOrder("ORD-001")

	history := []event.DomainEvent{
		&orderCreatedEvent{aggregateID: "ORD-001", amount: 500.00, occurredAt: time.Now()},
		&orderConfirmedEvent{aggregateID: "ORD-001", occurredAt: time.Now()},
		&orderCancelledEvent{aggregateID: "ORD-001", reason: "customer request", occurredAt: time.Now()},
	}

	order.LoadFromHistory(history)

	if order.Status != "cancelled" {
		t.Errorf("expected status 'cancelled', got '%s'", order.Status)
	}
	if order.Amount != 500.00 {
		t.Errorf("expected amount 500.00, got %.2f", order.Amount)
	}
	if order.Reason != "customer request" {
		t.Errorf("expected reason 'customer request', got '%s'", order.Reason)
	}
	if order.Version != 3 {
		t.Errorf("expected version 3, got %d", order.Version)
	}
	if len(order.UncommittedEvents()) != 0 {
		t.Errorf("expected 0 uncommitted events after LoadFromHistory, got %d", len(order.UncommittedEvents()))
	}
}

func TestApply_ThenLoadFromHistory(t *testing.T) {
	order := newTestOrder("ORD-001")

	order.Apply(&orderCreatedEvent{aggregateID: "ORD-001", amount: 100.00, occurredAt: time.Now()})

	if order.Status != "created" {
		t.Errorf("expected status 'created', got '%s'", order.Status)
	}

	uncommitted := order.UncommittedEvents()
	order.MarkEventsAsCommitted()

	rebuilt := newTestOrder("ORD-001")
	rebuilt.LoadFromHistory(uncommitted)

	if rebuilt.Status != "created" {
		t.Errorf("expected rebuilt status 'created', got '%s'", rebuilt.Status)
	}
	if rebuilt.Amount != 100.00 {
		t.Errorf("expected rebuilt amount 100.00, got %.2f", rebuilt.Amount)
	}
}

func TestNewEventCollector(t *testing.T) {
	agg := NewEventCollector("order-1")
	evt := &testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()}

	agg.Apply(evt)

	if agg.Version != 1 {
		t.Errorf("expected version 1, got %d", agg.Version)
	}

	history := []event.DomainEvent{
		&testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()},
		&testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()},
	}
	agg.LoadFromHistory(history)

	if agg.Version != 3 {
		t.Errorf("expected version 3, got %d", agg.Version)
	}
}

func TestAggregateRoot_OrderLifecycle(t *testing.T) {
	order := NewEventCollector("order-123")

	order.Apply(&testDomainEvent{aggregateID: "order-123", occurredAt: time.Now()})
	order.Apply(&testDomainEvent{aggregateID: "order-123", occurredAt: time.Now()})
	order.Apply(&testDomainEvent{aggregateID: "order-123", occurredAt: time.Now()})
	order.Apply(&testDomainEvent{aggregateID: "order-123", occurredAt: time.Now()})

	if order.Version != 4 {
		t.Errorf("expected version 4, got %d", order.Version)
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
	rebuilt.LoadFromHistory(uncommitted)
	if rebuilt.Version != 4 {
		t.Errorf("expected rebuilt version 4, got %d", rebuilt.Version)
	}

	err := rebuilt.Validate()
	if err != nil {
		t.Errorf("expected valid rebuilt aggregate, got: %v", err)
	}
}

func TestApply_WithoutApplier_Panics(t *testing.T) {
	agg := NewAggregateRoot("order-1")
	evt := &testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()}

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when Apply is called without applier")
		}
		msg, ok := r.(string)
		if !ok {
			t.Fatalf("expected string panic, got %T: %v", r, r)
		}
		if msg != "AggregateRoot: applier not set, use NewAggregateRootWithApplier(id, self) or NewEventCollector(id)" {
			t.Errorf("unexpected panic message: %s", msg)
		}
	}()

	agg.Apply(evt)
}

func TestLoadFromHistory_WithoutApplier_Panics(t *testing.T) {
	agg := NewAggregateRoot("order-1")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic when LoadFromHistory is called without applier")
		}
	}()

	agg.LoadFromHistory([]event.DomainEvent{
		&testDomainEvent{aggregateID: "order-1", occurredAt: time.Now()},
	})
}

func TestNewAggregateRootWithApplier(t *testing.T) {
	o := &testOrder{}
	o.AggregateRoot = *NewAggregateRootWithApplier("ORD-001", o)

	o.Apply(&orderCreatedEvent{aggregateID: "ORD-001", amount: 99.99, occurredAt: time.Now()})

	if o.Status != "created" {
		t.Errorf("expected status 'created', got '%s'", o.Status)
	}
	if o.Version != 1 {
		t.Errorf("expected version 1, got %d", o.Version)
	}
}

func TestSetApplier_StillWorks(t *testing.T) {
	o := &testOrder{}
	o.AggregateRoot = *NewAggregateRoot("ORD-001")
	o.SetApplier(o)

	o.Apply(&orderCreatedEvent{aggregateID: "ORD-001", amount: 50.00, occurredAt: time.Now()})

	if o.Status != "created" {
		t.Errorf("expected status 'created', got '%s'", o.Status)
	}
}

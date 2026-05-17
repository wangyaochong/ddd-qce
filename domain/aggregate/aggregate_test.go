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
	agg := NewAggregateRoot("order-1")
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
	agg := NewAggregateRoot("order-1")

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
	agg := NewAggregateRoot("order-1")
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
	agg := NewAggregateRoot("order-1")
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
	agg := NewAggregateRoot("order-1")

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
	agg := NewAggregateRoot("order-1")
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

func TestAggregateRoot_OrderLifecycle(t *testing.T) {
	order := NewAggregateRoot("order-123")

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

	rebuilt := NewAggregateRoot("order-123")
	rebuilt.LoadFromHistory(uncommitted)
	if rebuilt.Version != 4 {
		t.Errorf("expected rebuilt version 4, got %d", rebuilt.Version)
	}

	err := rebuilt.Validate()
	if err != nil {
		t.Errorf("expected valid rebuilt aggregate, got: %v", err)
	}
}

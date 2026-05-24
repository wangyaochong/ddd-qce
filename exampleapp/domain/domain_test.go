package domain

import (
	"testing"
	"time"

	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/domain/event"
)

func TestOrderAggregate_Place(t *testing.T) {
	order := mustCreateOrder("ORD-001", "user-001")
	if order.Status != OrderStatusPending {
		t.Errorf("expected pending, got %s", order.Status)
	}
	if order.TotalAmount != 1029.98 {
		t.Errorf("expected 1029.98, got %.2f", order.TotalAmount)
	}
	events := order.UncommittedEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 uncommitted event, got %d", len(events))
	}
	if event.EventTypeOf(events[0]) != "OrderPlacedEvent" {
		t.Errorf("expected OrderPlacedEvent, got %s", event.EventTypeOf(events[0]))
	}
}

func TestOrderAggregate_ConfirmPayment(t *testing.T) {
	order := mustCreateOrder("ORD-001", "user-001")
	order.MarkEventsAsCommitted()
	if err := order.ConfirmPayment(); err != nil {
		t.Fatalf("confirm payment failed: %v", err)
	}
	if order.Status != OrderStatusPaid {
		t.Errorf("expected paid, got %s", order.Status)
	}
	events := order.UncommittedEvents()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if event.EventTypeOf(events[0]) != "PaymentConfirmedEvent" {
		t.Errorf("expected PaymentConfirmedEvent, got %s", event.EventTypeOf(events[0]))
	}
}

func TestOrderAggregate_Ship(t *testing.T) {
	order := mustCreateOrder("ORD-001", "user-001")
	order.MarkEventsAsCommitted()
	_ = order.ConfirmPayment()
	order.MarkEventsAsCommitted()
	if err := order.Ship(); err != nil {
		t.Fatalf("ship failed: %v", err)
	}
	if order.Status != OrderStatusShipped {
		t.Errorf("expected shipped, got %s", order.Status)
	}
}

func TestOrderAggregate_Cancel(t *testing.T) {
	order := mustCreateOrder("ORD-001", "user-001")
	order.MarkEventsAsCommitted()
	if err := order.Cancel("changed mind"); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	if order.Status != OrderStatusCancelled {
		t.Errorf("expected cancelled, got %s", order.Status)
	}
	if order.CancelReason != "changed mind" {
		t.Errorf("expected 'changed mind', got %s", order.CancelReason)
	}
}

func TestOrderAggregate_WithApplier(t *testing.T) {
	order := mustCreateOrder("ORD-001", "user-001")
	if order.AggregateRoot.Version() != 1 {
		t.Errorf("expected version 1 after Place, got %d", order.AggregateRoot.Version())
	}
}

func TestOrderAggregate_SetApplier(t *testing.T) {
	o := &Order{UserID: "u1", Status: OrderStatusPending}
	o.AggregateRoot = *aggregate.NewAggregateRootWithApplier("ORD-SET", o)
	_ = o.Apply(&OrderPlacedEvent{BaseEvent: event.NewBaseEvent("ORD-SET", time.Now()), UserID: "u1", TotalAmount: 100})
	if o.UserID != "u1" {
		t.Errorf("When not applied via SetApplier, got UserID=%s", o.UserID)
	}
	if o.AggregateRoot.Version() != 1 {
		t.Errorf("expected version 1, got %d", o.AggregateRoot.Version())
	}
}

func TestEventCollector_EventsOnly(t *testing.T) {
	ar := NewOrderEventCollector("COLLECT-001")
	_ = ar.Apply(&OrderPlacedEvent{BaseEvent: event.NewBaseEvent("COLLECT-001", time.Now()), UserID: "u1", TotalAmount: 50})
	if len(ar.UncommittedEvents()) != 1 {
		t.Errorf("expected 1 event, got %d", len(ar.UncommittedEvents()))
	}
	if ar.Version() != 1 {
		t.Errorf("expected version 1, got %d", ar.Version())
	}
}

func TestOrderAggregate_Validate_EmptyID(t *testing.T) {
	items := []*OrderItem{NewOrderItem("p1", "Laptop", 999, 1)}
	_, err := NewOrder("", "user-001", items)
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestOrderAggregate_Validate_EmptyItems(t *testing.T) {
	_, err := NewOrder("ORD-001", "user-001", nil)
	if err == nil {
		t.Error("expected error for empty items")
	}
}

func TestOrderAggregate_InvalidTransition(t *testing.T) {
	order := mustCreateOrder("ORD-001", "user-001")
	if err := order.Ship(); err == nil {
		t.Error("expected error: cannot ship pending order")
	}
	_ = order.ConfirmPayment()
	_ = order.Cancel("test")
	if err := order.Ship(); err == nil {
		t.Error("expected error: cannot ship cancelled order")
	}
}

func TestOrderAggregate_When(t *testing.T) {
	o := &Order{}
	o.AggregateRoot = *aggregate.NewAggregateRootWithApplier("ORD-WHEN", o)
	_ = o.LoadFromHistory([]event.DomainEvent{
		&OrderPlacedEvent{BaseEvent: event.NewBaseEvent("ORD-WHEN", time.Now()), UserID: "u1", TotalAmount: 200},
		&PaymentConfirmedEvent{BaseEvent: event.NewBaseEvent("ORD-WHEN", time.Now())},
	})
	if o.Status != OrderStatusPaid {
		t.Errorf("expected paid after When replay, got %s", o.Status)
	}
	if o.UserID != "u1" {
		t.Errorf("expected u1, got %s", o.UserID)
	}
}

func TestOrderItem_EntityBasics(t *testing.T) {
	item := NewOrderItem("p1", "Laptop", 999, 2)
	if item.GetID() != "p1" {
		t.Errorf("expected p1, got %s", item.GetID())
	}
	if item.Subtotal() != 1998 {
		t.Errorf("expected 1998, got %.2f", item.Subtotal())
	}
}

func TestOrderItem_Equals(t *testing.T) {
	a := NewOrderItem("p1", "Laptop", 999, 1)
	b := NewOrderItem("p1", "Mouse", 29, 1)
	c := NewOrderItem("p2", "Laptop", 999, 1)
	if !a.Equals(&b.Entity) {
		t.Error("same ID should be equal")
	}
	if a.Equals(&c.Entity) {
		t.Error("different ID should not be equal")
	}
}

func TestOrderItem_IsEmpty(t *testing.T) {
	empty := NewOrderItem("", "", 0, 0)
	notEmpty := NewOrderItem("p1", "Laptop", 999, 1)
	if !empty.IsEmpty() {
		t.Error("empty ID should be IsEmpty")
	}
	if notEmpty.IsEmpty() {
		t.Error("non-empty ID should not be IsEmpty")
	}
}

func TestDomainEvent_Interface(t *testing.T) {
	now := time.Now()
	events := []event.DomainEvent{
		&OrderPlacedEvent{BaseEvent: event.NewBaseEvent("O1", now)},
		&PaymentConfirmedEvent{BaseEvent: event.NewBaseEvent("O1", now)},
		&OrderShippedEvent{BaseEvent: event.NewBaseEvent("O1", now)},
		&OrderCancelledEvent{BaseEvent: event.NewBaseEvent("O1", now)},
		&InventoryReservedEvent{BaseEvent: event.NewBaseEvent("O1", now)},
		&InventoryReleasedEvent{BaseEvent: event.NewBaseEvent("O1", now)},
	}
	for _, e := range events {
		if e.AggregateID() == "" {
			t.Errorf("%T: AggregateID() is empty", e)
		}
		if event.EventTypeOf(e) == "" {
			t.Errorf("%T: EventTypeOf() is empty", e)
		}
		if e.OccurredAt().IsZero() {
			t.Errorf("%T: OccurredAt() is zero", e)
		}
	}
}

func TestDomainEvent_EventTypeOf(t *testing.T) {
	e := &OrderPlacedEvent{BaseEvent: event.NewBaseEvent("O1", time.Now())}
	if event.EventTypeOf(e) != "OrderPlacedEvent" {
		t.Errorf("expected OrderPlacedEvent, got %s", event.EventTypeOf(e))
	}
	if event.EventTypeOf(e) != "OrderPlacedEvent" {
		t.Errorf("EventTypeOf mismatch, got %s", event.EventTypeOf(e))
	}
}

func mustCreateOrder(id, userID string) *Order {
	items := []*OrderItem{
		NewOrderItem("laptop", "Laptop", 999.99, 1),
		NewOrderItem("mouse", "Mouse", 29.99, 1),
	}
	order, err := NewOrder(id, userID, items)
	if err != nil {
		panic(err)
	}
	return order
}

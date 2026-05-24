package domain

import (
	"time"

	"github.com/ddd-qce/core/domain/event"
)

type OrderPlacedEvent struct {
	OrderID     string
	UserID      string
	TotalAmount float64
	Items       []string
	Time        time.Time
}

func (e *OrderPlacedEvent) AggregateID() string   { return e.OrderID }
func (e *OrderPlacedEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *OrderPlacedEvent) OccurredAt() time.Time { return e.Time }

type PaymentConfirmedEvent struct {
	OrderID string
	Time    time.Time
}

func (e *PaymentConfirmedEvent) AggregateID() string   { return e.OrderID }
func (e *PaymentConfirmedEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *PaymentConfirmedEvent) OccurredAt() time.Time { return e.Time }

type OrderShippedEvent struct {
	OrderID string
	Time    time.Time
}

func (e *OrderShippedEvent) AggregateID() string   { return e.OrderID }
func (e *OrderShippedEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *OrderShippedEvent) OccurredAt() time.Time { return e.Time }

type OrderCancelledEvent struct {
	OrderID string
	Reason  string
	Time    time.Time
}

func (e *OrderCancelledEvent) AggregateID() string   { return e.OrderID }
func (e *OrderCancelledEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *OrderCancelledEvent) OccurredAt() time.Time { return e.Time }

type InventoryReservedEvent struct {
	OrderID   string
	ProductID string
	Quantity  int
	Time      time.Time
}

func (e *InventoryReservedEvent) AggregateID() string   { return e.OrderID }
func (e *InventoryReservedEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *InventoryReservedEvent) OccurredAt() time.Time { return e.Time }

type InventoryReleasedEvent struct {
	OrderID   string
	ProductID string
	Quantity  int
	Time      time.Time
}

func (e *InventoryReleasedEvent) AggregateID() string   { return e.OrderID }
func (e *InventoryReleasedEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *InventoryReleasedEvent) OccurredAt() time.Time { return e.Time }

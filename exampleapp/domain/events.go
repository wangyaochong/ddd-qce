package domain

import (
	"github.com/ddd-qce/core/cqrs/event"
)

type OrderPlacedEvent struct {
	event.BaseEvent
	UserID      string
	TotalAmount float64
	Items       []string
}

type PaymentConfirmedEvent struct {
	event.BaseEvent
}

type OrderShippedEvent struct {
	event.BaseEvent
}

type OrderCancelledEvent struct {
	event.BaseEvent
	Reason string
}

type InventoryReservedEvent struct {
	event.BaseEvent
	ProductID string
	Quantity  int
}

type InventoryReleasedEvent struct {
	event.BaseEvent
	ProductID string
	Quantity  int
}

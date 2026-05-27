package event

import (
	"github.com/ddd-qce/core/cqrs/event"
)

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

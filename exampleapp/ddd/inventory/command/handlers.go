package command

import (
	"context"

	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	domainevent "github.com/ddd-qce/core/cqrs/event"
	inventorydomain "github.com/ddd-qce/exampleapp/ddd/inventory/domain"
	inventoryevent "github.com/ddd-qce/exampleapp/ddd/inventory/event"
)

type ReserveInventoryHandler struct {
	Inventory *inventorydomain.Inventory
	EventBus  cqrsevent.EventBus
}

func NewReserveInventoryHandler(inventory *inventorydomain.Inventory, eventBus cqrsevent.EventBus) *ReserveInventoryHandler {
	return &ReserveInventoryHandler{Inventory: inventory, EventBus: eventBus}
}

func (h *ReserveInventoryHandler) Handle(ctx context.Context, cmd *ReserveInventoryCommand) (*ReserveInventoryResult, error) {
	if err := h.Inventory.Reserve(cmd.ProductID, cmd.Quantity); err != nil {
		return nil, err
	}

	cqrsevent.Dispatch[*inventoryevent.InventoryReservedEvent](ctx, h.EventBus, &inventoryevent.InventoryReservedEvent{
		BaseEvent: domainevent.WithCorrelation(ctx, cmd.OrderID),
		ProductID: cmd.ProductID,
		Quantity:  cmd.Quantity,
	})

	return &ReserveInventoryResult{Success: true}, nil
}

type ReleaseInventoryHandler struct {
	Inventory *inventorydomain.Inventory
	EventBus  cqrsevent.EventBus
}

func NewReleaseInventoryHandler(inventory *inventorydomain.Inventory, eventBus cqrsevent.EventBus) *ReleaseInventoryHandler {
	return &ReleaseInventoryHandler{Inventory: inventory, EventBus: eventBus}
}

func (h *ReleaseInventoryHandler) Handle(ctx context.Context, cmd *ReleaseInventoryCommand) (*ReleaseInventoryResult, error) {
	if err := h.Inventory.Release(cmd.ProductID, cmd.Quantity); err != nil {
		return nil, err
	}

	cqrsevent.Dispatch[*inventoryevent.InventoryReleasedEvent](ctx, h.EventBus, &inventoryevent.InventoryReleasedEvent{
		BaseEvent: domainevent.WithCorrelation(ctx, cmd.OrderID),
		ProductID: cmd.ProductID,
		Quantity:  cmd.Quantity,
	})

	return &ReleaseInventoryResult{Success: true}, nil
}

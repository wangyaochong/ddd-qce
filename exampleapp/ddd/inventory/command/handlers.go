package command

import (
	"context"

	"github.com/ddd-qce/core/cqrs/event"
	inventorydomain "github.com/ddd-qce/exampleapp/ddd/inventory/domain"
	inventoryevent "github.com/ddd-qce/exampleapp/ddd/inventory/event"
)

type ReserveInventoryHandler struct {
	Inventory *inventorydomain.Inventory
	EventBus  event.EventBus
}

func NewReserveInventoryHandler(inventory *inventorydomain.Inventory, eventBus event.EventBus) *ReserveInventoryHandler {
	return &ReserveInventoryHandler{Inventory: inventory, EventBus: eventBus}
}

func (h *ReserveInventoryHandler) Handle(ctx context.Context, cmd *ReserveInventoryCommand) (*ReserveInventoryResult, error) {
	if err := h.Inventory.Reserve(cmd.ProductID, cmd.Quantity); err != nil {
		return nil, err
	}

	h.EventBus.Publish(ctx, &inventoryevent.InventoryReservedEvent{
		BaseEvent: event.WithCorrelation(ctx, cmd.OrderID.String()),
		ProductID: cmd.ProductID.String(),
		Quantity:  cmd.Quantity,
	})

	return &ReserveInventoryResult{Success: true}, nil
}

type ReleaseInventoryHandler struct {
	Inventory *inventorydomain.Inventory
	EventBus  event.EventBus
}

func NewReleaseInventoryHandler(inventory *inventorydomain.Inventory, eventBus event.EventBus) *ReleaseInventoryHandler {
	return &ReleaseInventoryHandler{Inventory: inventory, EventBus: eventBus}
}

func (h *ReleaseInventoryHandler) Handle(ctx context.Context, cmd *ReleaseInventoryCommand) (*ReleaseInventoryResult, error) {
	if err := h.Inventory.Release(cmd.ProductID, cmd.Quantity); err != nil {
		return nil, err
	}

	h.EventBus.Publish(ctx, &inventoryevent.InventoryReleasedEvent{
		BaseEvent: event.WithCorrelation(ctx, cmd.OrderID.String()),
		ProductID: cmd.ProductID.String(),
		Quantity:  cmd.Quantity,
	})

	return &ReleaseInventoryResult{Success: true}, nil
}

type ResetInventoryHandler struct{}

func NewResetInventoryHandler() *ResetInventoryHandler {
	return &ResetInventoryHandler{}
}

func (h *ResetInventoryHandler) Handle(ctx context.Context, cmd *ResetInventoryCommand) (*ResetInventoryResult, error) {
	return &ResetInventoryResult{}, nil
}

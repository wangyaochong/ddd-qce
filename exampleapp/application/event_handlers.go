package application

import (
	"context"
	"fmt"
	"log"

	commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
	"github.com/ddd-qce/exampleapp/domain"
)

type OrderPlacedInventoryHandler struct {
	cmdBus *commandmemory.CommandBus
}

func NewOrderPlacedInventoryHandler(cmdBus *commandmemory.CommandBus) *OrderPlacedInventoryHandler {
	return &OrderPlacedInventoryHandler{cmdBus: cmdBus}
}

func (h *OrderPlacedInventoryHandler) Handle(ctx context.Context, evt *domain.OrderPlacedEvent) error {
	log.Printf("[EventHandler] OrderPlaced: reserving inventory for order %s", evt.OrderID)
	_, err := commandmemory.Dispatch[*ReserveInventoryCommand, *ReserveInventoryResult](ctx, h.cmdBus, &ReserveInventoryCommand{
		OrderID:   evt.OrderID,
		ProductID: "laptop",
		Quantity:  1,
	})
	return err
}

type OrderCancelledInventoryHandler struct {
	cmdBus *commandmemory.CommandBus
}

func NewOrderCancelledInventoryHandler(cmdBus *commandmemory.CommandBus) *OrderCancelledInventoryHandler {
	return &OrderCancelledInventoryHandler{cmdBus: cmdBus}
}

func (h *OrderCancelledInventoryHandler) Handle(ctx context.Context, evt *domain.OrderCancelledEvent) error {
	log.Printf("[EventHandler] OrderCancelled: releasing inventory for order %s", evt.OrderID)
	_, err := commandmemory.Dispatch[*ReleaseInventoryCommand, *ReleaseInventoryResult](ctx, h.cmdBus, &ReleaseInventoryCommand{
		OrderID:   evt.OrderID,
		ProductID: "laptop",
		Quantity:  1,
	})
	return err
}

type OrderPlacedNotificationHandler struct{}

func NewOrderPlacedNotificationHandler() *OrderPlacedNotificationHandler {
	return &OrderPlacedNotificationHandler{}
}

func (h *OrderPlacedNotificationHandler) Handle(ctx context.Context, evt *domain.OrderPlacedEvent) error {
	fmt.Printf("[Notification] New order %s placed by user %s, total: $%.2f\n",
		evt.OrderID, evt.UserID, evt.TotalAmount)
	return nil
}

package application

import (
	"context"
	"fmt"
	"log"

	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/exampleapp/domain"
)

type OrderPlacedInventoryHandler struct {
	cmdBus command.CommandBus
}

func NewOrderPlacedInventoryHandler(cmdBus command.CommandBus) *OrderPlacedInventoryHandler {
	return &OrderPlacedInventoryHandler{cmdBus: cmdBus}
}

func (h *OrderPlacedInventoryHandler) Handle(ctx context.Context, evt *domain.OrderPlacedEvent) error {
	log.Printf("[EventHandler] OrderPlaced: reserving inventory for order %s", evt.AggregateID())
	_, err := command.Dispatch[*ReserveInventoryCommand, *ReserveInventoryResult](ctx, h.cmdBus, &ReserveInventoryCommand{
		OrderID:   evt.AggregateID(),
		ProductID: "laptop",
		Quantity:  1,
	})
	return err
}

type OrderCancelledInventoryHandler struct {
	cmdBus command.CommandBus
}

func NewOrderCancelledInventoryHandler(cmdBus command.CommandBus) *OrderCancelledInventoryHandler {
	return &OrderCancelledInventoryHandler{cmdBus: cmdBus}
}

func (h *OrderCancelledInventoryHandler) Handle(ctx context.Context, evt *domain.OrderCancelledEvent) error {
	log.Printf("[EventHandler] OrderCancelled: releasing inventory for order %s", evt.AggregateID())
	_, err := command.Dispatch[*ReleaseInventoryCommand, *ReleaseInventoryResult](ctx, h.cmdBus, &ReleaseInventoryCommand{
		OrderID:   evt.AggregateID(),
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
		evt.AggregateID(), evt.UserID, evt.TotalAmount)
	return nil
}

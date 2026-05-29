package wire

import (
	"context"
	"fmt"
	"log"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	inventorycommand "github.com/ddd-qce/exampleapp/ddd/inventory/command"
	inventorydomain "github.com/ddd-qce/exampleapp/ddd/inventory/domain"
	inventoryquery "github.com/ddd-qce/exampleapp/ddd/inventory/query"
	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
	orderevent "github.com/ddd-qce/exampleapp/ddd/order/event"
)

func WireInventory(
	chain *aspect.AspectChain,
	cmdBus command.CommandBus,
	queryBus query.QueryBus,
	eventBus cqrsevent.EventBus,
	inventory *inventorydomain.Inventory,
) error {
	if err := cmdBus.RegisterHandler(inventorycommand.NewReserveInventoryHandler(inventory, eventBus)); err != nil {
		return fmt.Errorf("register ReserveInventoryHandler: %w", err)
	}
	if err := cmdBus.RegisterHandler(inventorycommand.NewReleaseInventoryHandler(inventory, eventBus)); err != nil {
		return fmt.Errorf("register ReleaseInventoryHandler: %w", err)
	}

	if err := queryBus.RegisterHandler(inventoryquery.NewGetInventoryHandler(inventory)); err != nil {
		return fmt.Errorf("register GetInventoryHandler: %w", err)
	}

	if err := eventBus.SubscribeHandler(NewOrderPlacedInventoryHandler(cmdBus)); err != nil {
		return fmt.Errorf("register OrderPlacedInventoryHandler: %w", err)
	}
	if err := eventBus.SubscribeHandler(NewOrderCancelledInventoryHandler(cmdBus)); err != nil {
		return fmt.Errorf("register OrderCancelledInventoryHandler: %w", err)
	}

	return nil
}

type OrderPlacedInventoryHandler struct {
	CmdBus command.CommandBus
}

func NewOrderPlacedInventoryHandler(cmdBus command.CommandBus) *OrderPlacedInventoryHandler {
	return &OrderPlacedInventoryHandler{CmdBus: cmdBus}
}

func (h *OrderPlacedInventoryHandler) Handle(ctx context.Context, evt *orderevent.OrderPlacedEvent) error {
	log.Printf("[EventHandler] OrderPlaced: reserving inventory for order %s", evt.AggregateID())
	_, err := command.Dispatch[*inventorycommand.ReserveInventoryCommand, *inventorycommand.ReserveInventoryResult](ctx, h.CmdBus, &inventorycommand.ReserveInventoryCommand{
		OrderID:   orderdomain.OrderID(evt.AggregateID()),
		ProductID: orderdomain.ProductID("laptop"),
		Quantity:  1,
	})
	return err
}

type OrderCancelledInventoryHandler struct {
	CmdBus command.CommandBus
}

func NewOrderCancelledInventoryHandler(cmdBus command.CommandBus) *OrderCancelledInventoryHandler {
	return &OrderCancelledInventoryHandler{CmdBus: cmdBus}
}

func (h *OrderCancelledInventoryHandler) Handle(ctx context.Context, evt *orderevent.OrderCancelledEvent) error {
	log.Printf("[EventHandler] OrderCancelled: releasing inventory for order %s", evt.AggregateID())
	_, err := command.Dispatch[*inventorycommand.ReleaseInventoryCommand, *inventorycommand.ReleaseInventoryResult](ctx, h.CmdBus, &inventorycommand.ReleaseInventoryCommand{
		OrderID:   orderdomain.OrderID(evt.AggregateID()),
		ProductID: orderdomain.ProductID("laptop"),
		Quantity:  1,
	})
	return err
}

package infrastructure

import (
	"github.com/ddd-qce/core/observability"
	inventorycommand "github.com/ddd-qce/exampleapp/ddd/inventory/command"
	inventoryevent "github.com/ddd-qce/exampleapp/ddd/inventory/event"
	inventoryquery "github.com/ddd-qce/exampleapp/ddd/inventory/query"
	ordercommand "github.com/ddd-qce/exampleapp/ddd/order/command"
	orderevent "github.com/ddd-qce/exampleapp/ddd/order/event"
	orderquery "github.com/ddd-qce/exampleapp/ddd/order/query"
)

func RegisterAppTypes(registry *observability.TypePrototypeRegistry) {
	registry.RegisterFromSample("command", "PlaceOrderCommand",
		ordercommand.PlaceOrderCommand{}, ordercommand.PlaceOrderResult{})
	registry.RegisterFromSample("command", "ConfirmPaymentCommand",
		ordercommand.ConfirmPaymentCommand{}, ordercommand.ConfirmPaymentResult{})
	registry.RegisterFromSample("command", "ShipOrderCommand",
		ordercommand.ShipOrderCommand{}, ordercommand.ShipOrderResult{})
	registry.RegisterFromSample("command", "CancelOrderCommand",
		ordercommand.CancelOrderCommand{}, ordercommand.CancelOrderResult{})
	registry.RegisterFromSample("command", "GenerateReportCommand",
		ordercommand.GenerateReportCommand{}, ordercommand.GenerateReportResult{})

	registry.RegisterFromSample("command", "ReserveInventoryCommand",
		inventorycommand.ReserveInventoryCommand{}, inventorycommand.ReserveInventoryResult{})
	registry.RegisterFromSample("command", "ReleaseInventoryCommand",
		inventorycommand.ReleaseInventoryCommand{}, inventorycommand.ReleaseInventoryResult{})

	registry.RegisterFromSample("query", "GetOrderQuery",
		orderquery.GetOrderQuery{}, orderquery.GetOrderResult{})
	registry.RegisterFromSample("query", "ListOrdersQuery",
		orderquery.ListOrdersQuery{}, orderquery.ListOrdersResult{})

	registry.RegisterFromSample("query", "GetInventoryQuery",
		inventoryquery.GetInventoryQuery{}, inventoryquery.GetInventoryResult{})

	registry.RegisterFromSample("event", "OrderPlacedEvent",
		orderevent.OrderPlacedEvent{}, nil)
	registry.RegisterFromSample("event", "PaymentConfirmedEvent",
		orderevent.PaymentConfirmedEvent{}, nil)
	registry.RegisterFromSample("event", "OrderShippedEvent",
		orderevent.OrderShippedEvent{}, nil)
	registry.RegisterFromSample("event", "OrderCancelledEvent",
		orderevent.OrderCancelledEvent{}, nil)

	registry.RegisterFromSample("event", "InventoryReservedEvent",
		inventoryevent.InventoryReservedEvent{}, nil)
	registry.RegisterFromSample("event", "InventoryReleasedEvent",
		inventoryevent.InventoryReleasedEvent{}, nil)
}
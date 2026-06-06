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
	registry.RegisterFromSample("command", "ResetInventoryCommand",
		inventorycommand.ResetInventoryCommand{}, inventorycommand.ResetInventoryResult{})

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

// RegisterAppTypesToProvider populates a BusTypeSampleProvider with all
// exampleapp command/query/event type samples. This enables the DDDViewer
// to auto-discover types from the buses with full field detail via reflection.
func RegisterAppTypesToProvider(provider *observability.ReflectionSampleProvider) {
	provider.RegisterCommand("PlaceOrderCommand", ordercommand.PlaceOrderCommand{}, ordercommand.PlaceOrderResult{})
	provider.RegisterCommand("ConfirmPaymentCommand", ordercommand.ConfirmPaymentCommand{}, ordercommand.ConfirmPaymentResult{})
	provider.RegisterCommand("ShipOrderCommand", ordercommand.ShipOrderCommand{}, ordercommand.ShipOrderResult{})
	provider.RegisterCommand("CancelOrderCommand", ordercommand.CancelOrderCommand{}, ordercommand.CancelOrderResult{})
	provider.RegisterCommand("GenerateReportCommand", ordercommand.GenerateReportCommand{}, ordercommand.GenerateReportResult{})
	provider.RegisterCommand("ReserveInventoryCommand", inventorycommand.ReserveInventoryCommand{}, inventorycommand.ReserveInventoryResult{})
	provider.RegisterCommand("ReleaseInventoryCommand", inventorycommand.ReleaseInventoryCommand{}, inventorycommand.ReleaseInventoryResult{})
	provider.RegisterCommand("ResetInventoryCommand", inventorycommand.ResetInventoryCommand{}, inventorycommand.ResetInventoryResult{})

	provider.RegisterQuery("GetOrderQuery", orderquery.GetOrderQuery{}, orderquery.GetOrderResult{})
	provider.RegisterQuery("ListOrdersQuery", orderquery.ListOrdersQuery{}, orderquery.ListOrdersResult{})
	provider.RegisterQuery("GetInventoryQuery", inventoryquery.GetInventoryQuery{}, inventoryquery.GetInventoryResult{})

	provider.RegisterEvent("OrderPlacedEvent", orderevent.OrderPlacedEvent{})
	provider.RegisterEvent("PaymentConfirmedEvent", orderevent.PaymentConfirmedEvent{})
	provider.RegisterEvent("OrderShippedEvent", orderevent.OrderShippedEvent{})
	provider.RegisterEvent("OrderCancelledEvent", orderevent.OrderCancelledEvent{})
	provider.RegisterEvent("InventoryReservedEvent", inventoryevent.InventoryReservedEvent{})
	provider.RegisterEvent("InventoryReleasedEvent", inventoryevent.InventoryReleasedEvent{})
}

// NewAppTypeProvider convenience constructor. Returns a configured BusTypeSampleProvider.
func NewAppTypeProvider() *observability.ReflectionSampleProvider {
	p := observability.NewReflectionSampleProvider()
	RegisterAppTypesToProvider(p)
	return p
}

package wire

import (
	"fmt"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	ordercommand "github.com/ddd-qce/exampleapp/ddd/order/command"
	orderevent "github.com/ddd-qce/exampleapp/ddd/order/event"
	orderquery "github.com/ddd-qce/exampleapp/ddd/order/query"
	orderrepo "github.com/ddd-qce/exampleapp/ddd/order/repository"
)

func WireOrder(
	chain *aspect.AspectChain,
	cmdBus command.CommandBus,
	queryBus query.QueryBus,
	eventBus cqrsevent.EventBus,
	repo orderrepo.OrderRepositoryAdapter,
) error {
	if err := cmdBus.RegisterHandler(ordercommand.NewPlaceOrderHandler(repo, eventBus)); err != nil {
		return fmt.Errorf("register PlaceOrderHandler: %w", err)
	}
	if err := cmdBus.RegisterHandler(ordercommand.NewConfirmPaymentHandler(repo, eventBus)); err != nil {
		return fmt.Errorf("register ConfirmPaymentHandler: %w", err)
	}
	if err := cmdBus.RegisterHandler(ordercommand.NewShipOrderHandler(repo, eventBus)); err != nil {
		return fmt.Errorf("register ShipOrderHandler: %w", err)
	}
	if err := cmdBus.RegisterHandler(ordercommand.NewCancelOrderHandler(repo, eventBus)); err != nil {
		return fmt.Errorf("register CancelOrderHandler: %w", err)
	}
	if err := cmdBus.RegisterHandler(ordercommand.NewGenerateReportHandler()); err != nil {
		return fmt.Errorf("register GenerateReportHandler: %w", err)
	}

	if err := queryBus.RegisterHandler(orderquery.NewGetOrderHandler(repo)); err != nil {
		return fmt.Errorf("register GetOrderHandler: %w", err)
	}
	if err := queryBus.RegisterHandler(orderquery.NewListOrdersHandler(repo)); err != nil {
		return fmt.Errorf("register ListOrdersHandler: %w", err)
	}

	if err := eventBus.SubscribeHandler(orderevent.NewOrderPlacedNotificationHandler()); err != nil {
		return fmt.Errorf("register OrderPlacedNotificationHandler: %w", err)
	}

	return nil
}

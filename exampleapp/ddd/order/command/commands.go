package command

import (
	"github.com/ddd-qce/core/cqrs/command"
	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
)

type ItemInput struct {
	ProductID   orderdomain.ProductID
	ProductName string
	Price       float64
	Quantity    int
}

type PlaceOrderCommand struct {
	command.BaseCommand
	UserID orderdomain.UserID
	Items  []ItemInput
}

type PlaceOrderResult struct {
	OrderID     orderdomain.OrderID
	TotalAmount float64
}

type ConfirmPaymentCommand struct {
	command.BaseCommand
	OrderID orderdomain.OrderID
}

type ConfirmPaymentResult struct {
	Success bool
}

type ShipOrderCommand struct {
	command.BaseCommand
	OrderID orderdomain.OrderID
}

type ShipOrderResult struct {
	Success bool
}

type CancelOrderCommand struct {
	command.BaseCommand
	OrderID orderdomain.OrderID
	Reason  string
}

type CancelOrderResult struct {
	Success bool
}

type GenerateReportCommand struct {
	command.BaseCommand
	OrderID orderdomain.OrderID
}

type GenerateReportResult struct {
	ReportID  string
	OrderID   orderdomain.OrderID
	Content   string
	Generated bool
}

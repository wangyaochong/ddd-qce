package command

import (
	"github.com/ddd-qce/core/cqrs/command"
)

type ItemInput struct {
	ProductID   string
	ProductName string
	Price       float64
	Quantity    int
}

type PlaceOrderCommand struct {
	command.BaseCommand
	UserID string
	Items  []ItemInput
}

type PlaceOrderResult struct {
	OrderID     string
	TotalAmount float64
}

type ConfirmPaymentCommand struct {
	command.BaseCommand
	OrderID string
}

type ConfirmPaymentResult struct {
	Success bool
}

type ShipOrderCommand struct {
	command.BaseCommand
	OrderID string
}

type ShipOrderResult struct {
	Success bool
}

type CancelOrderCommand struct {
	command.BaseCommand
	OrderID string
	Reason  string
}

type CancelOrderResult struct {
	Success bool
}

type GenerateReportCommand struct {
	command.BaseCommand
	OrderID string
}

type GenerateReportResult struct {
	ReportID  string
	OrderID   string
	Content   string
	Generated bool
}

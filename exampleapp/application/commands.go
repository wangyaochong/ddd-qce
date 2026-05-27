package application

import (
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/query"
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

type ReserveInventoryCommand struct {
	command.BaseCommand
	OrderID   string
	ProductID string
	Quantity  int
}

type ReserveInventoryResult struct {
	Success bool
}

type ReleaseInventoryCommand struct {
	command.BaseCommand
	OrderID   string
	ProductID string
	Quantity  int
}

type ReleaseInventoryResult struct {
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

type GetOrderQuery struct {
	query.BaseQuery
	OrderID string
}

type OrderViewItem struct {
	ProductID   string
	ProductName string
	Price       float64
	Quantity    int
	Subtotal    float64
}

type GetOrderResult struct {
	OrderID      string
	UserID       string
	Status       string
	TotalAmount  float64
	Items        []OrderViewItem
	CreatedAt    string
	PaidAt       string
	ShippedAt    string
	CancelledAt  string
	CancelReason string
}

type ListOrdersQuery struct {
	query.BaseQuery
}

type ListOrdersResult struct {
	Orders []GetOrderResult
}

type GetInventoryQuery struct {
	query.BaseQuery
}

type InventoryItem struct {
	ID    string
	Name  string
	Price float64
	Stock int
}

type GetInventoryResult struct {
	Products []InventoryItem
}

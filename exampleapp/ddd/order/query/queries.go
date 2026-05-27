package query

import (
	"github.com/ddd-qce/core/cqrs/query"
)

type OrderViewItem struct {
	ProductID   string
	ProductName string
	Price       float64
	Quantity    int
	Subtotal    float64
}

type GetOrderQuery struct {
	query.BaseQuery
	OrderID string
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

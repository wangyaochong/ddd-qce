package query

import (
	"github.com/ddd-qce/core/cqrs/query"
	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
)

type OrderViewItem struct {
	ProductID   orderdomain.ProductID
	ProductName string
	Price       float64
	Quantity    int
	Subtotal    float64
}

type GetOrderQuery struct {
	query.BaseQuery
	OrderID orderdomain.OrderID
}

type GetOrderResult struct {
	OrderID      orderdomain.OrderID
	UserID       orderdomain.UserID
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

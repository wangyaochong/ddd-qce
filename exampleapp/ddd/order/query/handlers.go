package query

import (
	"context"
	"time"

	"github.com/ddd-qce/core/cqrs/query"
	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
	"github.com/ddd-qce/exampleapp/ddd/order/repository"
)

type GetOrderHandler struct {
	Repo repository.OrderRepositoryAdapter
}

func NewGetOrderHandler(repo repository.OrderRepositoryAdapter) *GetOrderHandler {
	return &GetOrderHandler{Repo: repo}
}

func (h *GetOrderHandler) Handle(ctx context.Context, q *GetOrderQuery) (*GetOrderResult, error) {
	order, err := h.Repo.FindByID(ctx, q.OrderID.String())
	if err != nil {
		return nil, err
	}
	return toOrderView(order), nil
}

type ListOrdersHandler struct {
	Repo repository.OrderRepositoryAdapter
}

func NewListOrdersHandler(repo repository.OrderRepositoryAdapter) *ListOrdersHandler {
	return &ListOrdersHandler{Repo: repo}
}

func (h *ListOrdersHandler) Handle(ctx context.Context, q *ListOrdersQuery) (*ListOrdersResult, error) {
	orders, err := h.Repo.FindAll(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]GetOrderResult, len(orders))
	for i, o := range orders {
		result[i] = *toOrderView(o)
	}
	return &ListOrdersResult{Orders: result}, nil
}

var _ query.QueryHandler[*GetOrderQuery, *GetOrderResult] = (*GetOrderHandler)(nil)

func toOrderView(o *orderdomain.Order) *GetOrderResult {
	items := make([]OrderViewItem, len(o.Items))
	for i, item := range o.Items {
		items[i] = OrderViewItem{
			ProductID:   orderdomain.ProductID(item.ID()),
			ProductName: item.ProductName,
			Price:       item.Price,
			Quantity:    item.Quantity,
			Subtotal:    item.Subtotal(),
		}
	}
	result := &GetOrderResult{
		OrderID:     orderdomain.OrderID(o.ID()),
		UserID:      o.UserID,
		Status:      string(o.Status),
		TotalAmount: o.TotalAmount,
		Items:       items,
	}
	if !o.CreatedAt.IsZero() {
		result.CreatedAt = o.CreatedAt.Format(time.RFC3339)
	}
	if !o.PaidAt.IsZero() {
		result.PaidAt = o.PaidAt.Format(time.RFC3339)
	}
	if !o.ShippedAt.IsZero() {
		result.ShippedAt = o.ShippedAt.Format(time.RFC3339)
	}
	if !o.CancelledAt.IsZero() {
		result.CancelledAt = o.CancelledAt.Format(time.RFC3339)
	}
	result.CancelReason = o.CancelReason
	return result
}

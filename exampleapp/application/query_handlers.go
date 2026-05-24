package application

import (
	"context"
	"time"

	"github.com/ddd-qce/core/cqrs/query"
	"github.com/ddd-qce/exampleapp/domain"
)

type GetOrderHandler struct {
	repo OrderRepositoryAdapter
}

func NewGetOrderHandler(repo OrderRepositoryAdapter) *GetOrderHandler {
	return &GetOrderHandler{repo: repo}
}

func (h *GetOrderHandler) Handle(ctx context.Context, q *GetOrderQuery) (*GetOrderResult, error) {
	order, err := h.repo.FindByID(ctx, q.OrderID)
	if err != nil {
		return nil, err
	}
	return toOrderView(order), nil
}

type ListOrdersHandler struct {
	repo OrderRepositoryAdapter
}

func NewListOrdersHandler(repo OrderRepositoryAdapter) *ListOrdersHandler {
	return &ListOrdersHandler{repo: repo}
}

func (h *ListOrdersHandler) Handle(ctx context.Context, q *ListOrdersQuery) (*ListOrdersResult, error) {
	orders := h.repo.FindAll()
	result := make([]GetOrderResult, len(orders))
	for i, o := range orders {
		result[i] = *toOrderView(o)
	}
	return &ListOrdersResult{Orders: result}, nil
}

type GetInventoryHandler struct {
	inventory *domain.Inventory
}

func NewGetInventoryHandler(inventory *domain.Inventory) *GetInventoryHandler {
	return &GetInventoryHandler{inventory: inventory}
}

func (h *GetInventoryHandler) Handle(ctx context.Context, q *GetInventoryQuery) (*GetInventoryResult, error) {
	products := h.inventory.GetAll()
	items := make([]InventoryItem, len(products))
	for i, p := range products {
		items[i] = InventoryItem{
			ID:    p.ID,
			Name:  p.Name,
			Price: p.Price,
			Stock: p.Stock,
		}
	}
	return &GetInventoryResult{Products: items}, nil
}

var _ query.QueryHandler[*GetOrderQuery, *GetOrderResult] = (*GetOrderHandler)(nil)

func toOrderView(o *domain.Order) *GetOrderResult {
	items := make([]OrderViewItem, len(o.Items))
	for i, item := range o.Items {
		items[i] = OrderViewItem{
			ProductID:   item.GetID(),
			ProductName: item.ProductName,
			Price:       item.Price,
			Quantity:    item.Quantity,
			Subtotal:    item.Subtotal(),
		}
	}
	result := &GetOrderResult{
		OrderID:     o.GetID(),
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

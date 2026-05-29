package query

import (
	"context"

	inventorydomain "github.com/ddd-qce/exampleapp/ddd/inventory/domain"
	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
)

type GetInventoryHandler struct {
	Inventory *inventorydomain.Inventory
}

func NewGetInventoryHandler(inventory *inventorydomain.Inventory) *GetInventoryHandler {
	return &GetInventoryHandler{Inventory: inventory}
}

func (h *GetInventoryHandler) Handle(ctx context.Context, q *GetInventoryQuery) (*GetInventoryResult, error) {
	products := h.Inventory.GetAll()
	items := make([]InventoryItem, len(products))
	for i, p := range products {
		items[i] = InventoryItem{
			ID:    orderdomain.ProductID(p.ID.String()),
			Name:  p.Name,
			Price: p.Price,
			Stock: p.Stock,
		}
	}
	return &GetInventoryResult{Products: items}, nil
}

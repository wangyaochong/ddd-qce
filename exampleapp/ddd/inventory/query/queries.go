package query

import (
	"github.com/ddd-qce/core/cqrs/query"
	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
)

type InventoryItem struct {
	ID    orderdomain.ProductID
	Name  string
	Price float64
	Stock int
}

type GetInventoryQuery struct {
	query.BaseQuery
}

type GetInventoryResult struct {
	Products []InventoryItem
}

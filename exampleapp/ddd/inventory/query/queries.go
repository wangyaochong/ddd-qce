package query

import (
	"github.com/ddd-qce/core/cqrs/query"
)

type InventoryItem struct {
	ID    string
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

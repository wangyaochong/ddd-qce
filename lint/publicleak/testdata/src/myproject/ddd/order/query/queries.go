package query

import "myproject/ddd/inventory/domain"

type StockResult struct {
	StockMap map[string]*domain.Inventory // want "dddpublicleak"
	StockCh  chan *domain.Inventory       // want "dddpublicleak"
	StockArr [3]*domain.Inventory         // want "dddpublicleak"
}

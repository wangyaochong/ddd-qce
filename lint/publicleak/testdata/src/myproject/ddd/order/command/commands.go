package command

import (
	inventorydomain "myproject/ddd/inventory/domain"
	orderdomain "myproject/ddd/order/domain"
)

type PlaceOrderResult struct {
	Order *orderdomain.Order
}

type GetInventoryResult struct {
	Inv inventorydomain.Inventory // want "dddpublicleak"
}

func CreateOrder(o *orderdomain.Order) error {
	return nil
}

func GetInventory(id string) *inventorydomain.Inventory { // want "dddpublicleak"
	return nil
}

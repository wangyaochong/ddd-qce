package command

import (
	"github.com/ddd-qce/core/cqrs/command"
	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
)

type ReserveInventoryCommand struct {
	command.BaseCommand
	OrderID   orderdomain.OrderID
	ProductID orderdomain.ProductID
	Quantity  int
}

type ReserveInventoryResult struct {
	Success bool
}

type ReleaseInventoryCommand struct {
	command.BaseCommand
	OrderID   orderdomain.OrderID
	ProductID orderdomain.ProductID
	Quantity  int
}

type ReleaseInventoryResult struct {
	Success bool
}

type ResetInventoryCommand struct {
	command.BaseCommand
	ProductID orderdomain.ProductID
}

type ResetInventoryResult struct{}

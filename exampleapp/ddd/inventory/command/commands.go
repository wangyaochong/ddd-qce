package command

import (
	"github.com/ddd-qce/core/cqrs/command"
)

type ReserveInventoryCommand struct {
	command.BaseCommand
	OrderID   string
	ProductID string
	Quantity  int
}

type ReserveInventoryResult struct {
	Success bool
}

type ReleaseInventoryCommand struct {
	command.BaseCommand
	OrderID   string
	ProductID string
	Quantity  int
}

type ReleaseInventoryResult struct {
	Success bool
}

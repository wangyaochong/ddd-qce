package command

import (
	"context"
	"fmt"
	"time"

	corecommand "github.com/ddd-qce/core/cqrs/command"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
)

type CreateUserCommand struct {
	corecommand.BaseCommand
	Name  string
	Email string
}

type CreateUserResult struct {
	ID string
}

type CreateUserHandler struct{}

func (h *CreateUserHandler) Handle(ctx context.Context, cmd *CreateUserCommand) (*CreateUserResult, error) {
	return &CreateUserResult{ID: "user-" + time.Now().Format("20060102150405")}, nil
}

type UpdateUserCommand struct {
	corecommand.BaseCommand
	UserID string
	Name   string
	Email  string
}

type UpdateUserResult struct {
	Success bool
}

type UpdateUserHandler struct{}

func (h *UpdateUserHandler) Handle(ctx context.Context, cmd *UpdateUserCommand) (*UpdateUserResult, error) {
	return &UpdateUserResult{Success: true}, nil
}

type CancelOrderCommand struct {
	corecommand.BaseCommand
	OrderID string
	Reason  string
}

type CancelOrderResult struct {
	Success bool
}

type CancelOrderHandler struct{}

func (h *CancelOrderHandler) Handle(ctx context.Context, cmd *CancelOrderCommand) (*CancelOrderResult, error) {
	return &CancelOrderResult{Success: true}, nil
}

func RegisterHandlers(bus *commandmemory.CommandBus) {
	commandmemory.RegisterCommand(bus, &CreateUserHandler{})
	commandmemory.RegisterCommand(bus, &UpdateUserHandler{})
	commandmemory.RegisterCommand(bus, &CancelOrderHandler{})
}

func RunExample(ctx context.Context, bus *commandmemory.CommandBus) {
	fmt.Println("=== Command: CreateUser ===")
	cmdResult, err := corecommand.Dispatch(ctx, bus, &CreateUserCommand{Name: "李四", Email: "lisi@example.com"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Created user with ID: %s\n", cmdResult.ID)

	fmt.Println("\n=== Command: UpdateUser ===")
	updateResult, err := corecommand.Dispatch(ctx, bus, &UpdateUserCommand{UserID: "1", Name: "张三更新", Email: "zhangsan_new@example.com"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Update success: %v\n", updateResult.Success)

	fmt.Println("\n=== Command: CancelOrder ===")
	cancelResult, err := corecommand.Dispatch(ctx, bus, &CancelOrderCommand{OrderID: "ORD-001", Reason: "用户取消"})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Cancel success: %v\n", cancelResult.Success)
}

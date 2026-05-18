package command

import (
	"context"
	"testing"
	"time"
)

type CreateOrderCommand struct {
	UserID string
	Amount float64
}

type CreateOrderResult struct {
	OrderID string
}

type CreateOrderHandler struct{}

func (h *CreateOrderHandler) Handle(ctx context.Context, cmd *CreateOrderCommand) (*CreateOrderResult, error) {
	return &CreateOrderResult{OrderID: "ORD-" + time.Now().Format("20060102150405")}, nil
}

type InMemoryCommandBus struct {
	handlers map[string]func(ctx context.Context, cmd any) (any, error)
}

func NewInMemoryCommandBus() *InMemoryCommandBus {
	return &InMemoryCommandBus{
		handlers: make(map[string]func(ctx context.Context, cmd any) (any, error)),
	}
}

func (b *InMemoryCommandBus) Register(handler *CreateOrderHandler) {
	b.handlers["CreateOrderCommand"] = func(ctx context.Context, cmd any) (any, error) {
		return handler.Handle(ctx, cmd.(*CreateOrderCommand))
	}
}

func (b *InMemoryCommandBus) Execute(ctx context.Context, cmd any) (any, error) {
	handler, exists := b.handlers["CreateOrderCommand"]
	if !exists {
		return nil, nil
	}
	return handler(ctx, cmd)
}

func TestCommandHandler_Handle(t *testing.T) {
	ctx := context.Background()
	handler := &CreateOrderHandler{}

	cmd := &CreateOrderCommand{
		UserID: "user-001",
		Amount: 99.99,
	}

	result, err := handler.Handle(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OrderID == "" {
		t.Fatal("expected non-empty OrderID")
	}
}

func TestCommandBus_Execute(t *testing.T) {
	ctx := context.Background()
	bus := NewInMemoryCommandBus()

	handler := &CreateOrderHandler{}
	bus.Register(handler)

	cmd := &CreateOrderCommand{
		UserID: "user-001",
		Amount: 99.99,
	}

	result, err := bus.Execute(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.(*CreateOrderResult)
	if r.OrderID == "" {
		t.Errorf("expected non-empty OrderID")
	}
}

func TestCommandBus_Execute_Unregistered(t *testing.T) {
	ctx := context.Background()
	bus := NewInMemoryCommandBus()

	cmd := &CreateOrderCommand{
		UserID: "user-001",
		Amount: 99.99,
	}

	result, err := bus.Execute(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for unregistered command, got %v", result)
	}
}

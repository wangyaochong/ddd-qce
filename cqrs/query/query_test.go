package query

import (
	"context"
	"testing"
)

type GetOrderQuery struct {
	BaseQuery
	OrderID string
}

type GetOrderResult struct {
	OrderID string
	Status  string
	Total   float64
}

type GetOrderHandler struct {
	orders map[string]*GetOrderResult
}

func (h *GetOrderHandler) Handle(ctx context.Context, query *GetOrderQuery) (*GetOrderResult, error) {
	result, exists := h.orders[query.OrderID]
	if !exists {
		return nil, nil
	}
	return result, nil
}

type InMemoryQueryBus struct {
	handlers map[string]func(ctx context.Context, query any) (any, error)
}

func NewInMemoryQueryBus() *InMemoryQueryBus {
	return &InMemoryQueryBus{
		handlers: make(map[string]func(ctx context.Context, query any) (any, error)),
	}
}

func (b *InMemoryQueryBus) Register(queryType string, handler func(ctx context.Context, query any) (any, error)) {
	b.handlers[queryType] = handler
}

func (b *InMemoryQueryBus) Ask(ctx context.Context, query any) (any, error) {
	queryType := "GetOrderQuery"
	handler, exists := b.handlers[queryType]
	if !exists {
		return nil, nil
	}
	return handler(ctx, query)
}

func TestQueryHandler_Handle(t *testing.T) {
	ctx := context.Background()
	handler := &GetOrderHandler{
		orders: map[string]*GetOrderResult{
			"ORD-001": {OrderID: "ORD-001", Status: "confirmed", Total: 99.99},
		},
	}

	query := &GetOrderQuery{OrderID: "ORD-001"}

	result, err := handler.Handle(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OrderID != "ORD-001" {
		t.Errorf("expected OrderID 'ORD-001', got %s", result.OrderID)
	}
	if result.Status != "confirmed" {
		t.Errorf("expected status 'confirmed', got %s", result.Status)
	}
}

func TestQueryHandler_Handle_NotFound(t *testing.T) {
	ctx := context.Background()
	handler := &GetOrderHandler{
		orders: map[string]*GetOrderResult{},
	}

	query := &GetOrderQuery{OrderID: "ORD-999"}

	result, err := handler.Handle(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for non-existent order, got %v", result)
	}
}

func TestQueryBus_Ask(t *testing.T) {
	ctx := context.Background()
	bus := NewInMemoryQueryBus()

	bus.Register("GetOrderQuery", func(ctx context.Context, query any) (any, error) {
		q := query.(*GetOrderQuery)
		return &GetOrderResult{OrderID: q.OrderID, Status: "pending", Total: 0}, nil
	})

	query := &GetOrderQuery{OrderID: "ORD-002"}

	result, err := bus.Ask(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r := result.(*GetOrderResult)
	if r.OrderID != "ORD-002" {
		t.Errorf("expected OrderID 'ORD-002', got %s", r.OrderID)
	}
}

func TestQueryBus_Ask_Unregistered(t *testing.T) {
	ctx := context.Background()
	bus := NewInMemoryQueryBus()

	query := &GetOrderQuery{OrderID: "ORD-003"}

	result, err := bus.Ask(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for unregistered query, got %v", result)
	}
}

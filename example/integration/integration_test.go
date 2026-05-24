package integration

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
	"github.com/ddd-qce/core/cqrs/query"
	querymemory "github.com/ddd-qce/core/cqrs/query/memory"
	"github.com/ddd-qce/core/domain/event"
)

type testOrder struct {
	ID     string
	UserID string
	Amount float64
	Status string
}

type testCreateOrderCommand struct {
	command.BaseCommand
	UserID string
	Amount float64
}

type testCreateOrderResult struct {
	OrderID string
}

type testCreateOrderHandler struct{}

func (h *testCreateOrderHandler) Handle(ctx context.Context, cmd *testCreateOrderCommand) (*testCreateOrderResult, error) {
	return &testCreateOrderResult{OrderID: "ORD-" + time.Now().Format("20060102150405")}, nil
}

type testOrderCreatedEvent struct {
	OrderID string
	UserID  string
	Amount  float64
}

func (e *testOrderCreatedEvent) AggregateID() string   { return e.OrderID }
func (e *testOrderCreatedEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *testOrderCreatedEvent) OccurredAt() time.Time { return time.Now() }

type testOrderCreatedEventHandler struct {
	called bool
}

func (h *testOrderCreatedEventHandler) Handle(ctx context.Context, event *testOrderCreatedEvent) error {
	h.called = true
	return nil
}

type testGetOrderQuery struct {
	query.BaseQuery
	OrderID string
}

type testGetOrderResult struct {
	OrderID string
	Status  string
}

type testGetOrderHandler struct {
	orders map[string]*testOrder
}

func (h *testGetOrderHandler) Handle(ctx context.Context, query *testGetOrderQuery) (*testGetOrderResult, error) {
	order, exists := h.orders[query.OrderID]
	if !exists {
		return nil, nil
	}
	return &testGetOrderResult{OrderID: order.ID, Status: order.Status}, nil
}

func TestIntegration_CommandEventQueryFlow(t *testing.T) {
	ctx := context.Background()
	chain := aspect.NewAspectChain()

	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	eventBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))
	qBus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))

	orders := make(map[string]*testOrder)
	eventHandler := &testOrderCreatedEventHandler{}
	queryHandler := &testGetOrderHandler{orders: orders}

	commandmemory.RegisterCommand(cmdBus, &testCreateOrderHandler{})
	eventmemory.RegisterHandler[*testOrderCreatedEvent](eventBus, eventHandler)
	querymemory.RegisterQuery(qBus, queryHandler)

	result, err := commandmemory.Dispatch[*testCreateOrderCommand, *testCreateOrderResult](ctx, cmdBus, &testCreateOrderCommand{
		UserID: "user-001",
		Amount: 99.99,
	})
	if err != nil {
		t.Fatalf("command dispatch failed: %v", err)
	}
	if result.OrderID == "" {
		t.Fatal("expected non-empty order ID")
	}

	orders[result.OrderID] = &testOrder{
		ID:     result.OrderID,
		UserID: "user-001",
		Amount: 99.99,
		Status: "created",
	}

	err = eventmemory.Dispatch[*testOrderCreatedEvent](ctx, eventBus, &testOrderCreatedEvent{
		OrderID: result.OrderID,
		UserID:  "user-001",
		Amount:  99.99,
	})
	if err != nil {
		t.Fatalf("event publish failed: %v", err)
	}
	if !eventHandler.called {
		t.Error("event handler was not called")
	}

	qResult, err := querymemory.Dispatch[*testGetOrderQuery, *testGetOrderResult](ctx, qBus, &testGetOrderQuery{
		OrderID: result.OrderID,
	})
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if qResult.OrderID != result.OrderID {
		t.Errorf("expected order ID %s, got %s", result.OrderID, qResult.OrderID)
	}
	if qResult.Status != "created" {
		t.Errorf("expected status 'created', got %s", qResult.Status)
	}
}

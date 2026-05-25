package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	eventbus "github.com/ddd-qce/core/cqrs/event"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	"github.com/ddd-qce/core/cqrs/query"
	querymemory "github.com/ddd-qce/core/cqrs/impl/memory"
	"github.com/ddd-qce/core/cqrs/event"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusConfirmed OrderStatus = "confirmed"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type OrderItem struct {
	Product string
	Price   float64
}

type Order struct {
	ID        string
	UserID    string
	Items     []OrderItem
	Status    OrderStatus
	CreatedAt time.Time
}

func NewOrder(id, userID string, items []OrderItem) *Order {
	return &Order{
		ID:        id,
		UserID:    userID,
		Items:     items,
		Status:    OrderStatusPending,
		CreatedAt: time.Now(),
	}
}

func (o *Order) Total() float64 {
	var total float64
	for _, item := range o.Items {
		total += item.Price
	}
	return total
}

func (o *Order) Confirm() error {
	if o.Status != OrderStatusPending {
		return fmt.Errorf("order %s cannot be confirmed (status: %s)", o.ID, o.Status)
	}
	o.Status = OrderStatusConfirmed
	return nil
}

func (o *Order) Cancel() error {
	if o.Status == OrderStatusCancelled {
		return fmt.Errorf("order %s already cancelled", o.ID)
	}
	o.Status = OrderStatusCancelled
	return nil
}

type OrderRepository struct {
	orders map[string]*Order
}

func NewOrderRepository() *OrderRepository {
	return &OrderRepository{
		orders: make(map[string]*Order),
	}
}

func (r *OrderRepository) Save(order *Order) error {
	r.orders[order.ID] = order
	return nil
}

func (r *OrderRepository) FindByID(id string) (*Order, error) {
	order, exists := r.orders[id]
	if !exists {
		return nil, fmt.Errorf("order %s not found", id)
	}
	return order, nil
}

type testPlaceOrderCommand struct {
	command.BaseCommand
	OrderID string
	UserID  string
	Items   []OrderItem
}

type testPlaceOrderResult struct {
	OrderID string
	Total   float64
}

type testPlaceOrderHandler struct {
	repo *OrderRepository
}

func (h *testPlaceOrderHandler) Handle(ctx context.Context, cmd *testPlaceOrderCommand) (*testPlaceOrderResult, error) {
	order := NewOrder(cmd.OrderID, cmd.UserID, cmd.Items)
	if err := h.repo.Save(order); err != nil {
		return nil, err
	}
	return &testPlaceOrderResult{OrderID: order.ID, Total: order.Total()}, nil
}

type testConfirmOrderCommand struct {
	command.BaseCommand
	OrderID string
}

type testConfirmOrderResult struct {
	Success bool
}

type testConfirmOrderHandler struct {
	repo       *OrderRepository
	eventBus *eventmemory.EventBus
}

func (h *testConfirmOrderHandler) Handle(ctx context.Context, cmd *testConfirmOrderCommand) (*testConfirmOrderResult, error) {
	order, err := h.repo.FindByID(cmd.OrderID)
	if err != nil {
		return nil, err
	}
	if err := order.Confirm(); err != nil {
		return nil, err
	}
	eventbus.Dispatch(ctx, h.eventBus, &testOrderConfirmedEvent{
		BaseEvent: event.NewBaseEvent(order.ID, time.Now()),
	})
	return &testConfirmOrderResult{Success: true}, nil
}

type testOrderConfirmedEvent struct {
	event.BaseEvent
}

type testOrderConfirmedEventHandler struct {
	called bool
}

func (h *testOrderConfirmedEventHandler) Handle(ctx context.Context, event *testOrderConfirmedEvent) error {
	h.called = true
	return nil
}

type testGetOrderStatusQuery struct {
	query.BaseQuery
	OrderID string
}

type testGetOrderStatusResult struct {
	OrderID string
	Status  string
	Total   float64
}

type testGetOrderStatusHandler struct {
	repo *OrderRepository
}

func (h *testGetOrderStatusHandler) Handle(ctx context.Context, query *testGetOrderStatusQuery) (*testGetOrderStatusResult, error) {
	order, err := h.repo.FindByID(query.OrderID)
	if err != nil {
		return nil, err
	}
	return &testGetOrderStatusResult{
		OrderID: order.ID,
		Status:  string(order.Status),
		Total:   order.Total(),
	}, nil
}

func TestOrderAggregate_ConfirmFlow(t *testing.T) {
	ctx := context.Background()
	chain := aspect.NewAspectChain()

	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	eventBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))
	qBus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
	repo := NewOrderRepository()

	eventHandler := &testOrderConfirmedEventHandler{}
	placeHandler := &testPlaceOrderHandler{repo: repo}
	confirmHandler := &testConfirmOrderHandler{repo: repo, eventBus: eventBus}
	statusHandler := &testGetOrderStatusHandler{repo: repo}

	commandmemory.RegisterCommand(cmdBus, placeHandler)
	commandmemory.RegisterCommand(cmdBus, confirmHandler)
	eventmemory.RegisterEvent[*testOrderConfirmedEvent](eventBus, eventHandler)
	querymemory.RegisterQuery(qBus, statusHandler)

	_, err := 	command.Dispatch(ctx, cmdBus, &testPlaceOrderCommand{
		OrderID: "ORD-001",
		UserID:  "user-001",
		Items: []OrderItem{
			{Product: "Laptop", Price: 999.99},
			{Product: "Mouse", Price: 29.99},
		},
	})
	if err != nil {
		t.Fatalf("place order failed: %v", err)
	}

	_, err = 	command.Dispatch(ctx, cmdBus, &testConfirmOrderCommand{
		OrderID: "ORD-001",
	})
	if err != nil {
		t.Fatalf("confirm order failed: %v", err)
	}
	if !eventHandler.called {
		t.Error("order confirmed event handler was not called")
	}

	result, err := query.Dispatch(ctx, qBus, &testGetOrderStatusQuery{
		OrderID: "ORD-001",
	})
	if err != nil {
		t.Fatalf("get order status failed: %v", err)
	}
	if result.Status != "confirmed" {
		t.Errorf("expected status 'confirmed', got %s", result.Status)
	}
	if result.Total != 1029.98 {
		t.Errorf("expected total 1029.98, got %.2f", result.Total)
	}
}

func TestOrderAggregate_CancelFlow(t *testing.T) {
	repo := NewOrderRepository()

	order := NewOrder("ORD-002", "user-002", []OrderItem{{Product: "Phone", Price: 599.99}})
	repo.Save(order)

	err := order.Cancel()
	if err != nil {
		t.Fatalf("cancel order failed: %v", err)
	}
	if order.Status != OrderStatusCancelled {
		t.Errorf("expected status 'cancelled', got %s", order.Status)
	}

	err = order.Cancel()
	if err == nil {
		t.Fatal("expected error when cancelling already cancelled order")
	}
}

func TestOrderAggregate_ConfirmInvalidStatus(t *testing.T) {
	order := NewOrder("ORD-003", "user-003", []OrderItem{{Product: "Tablet", Price: 399.99}})
	order.Status = OrderStatusCancelled

	err := order.Confirm()
	if err == nil {
		t.Fatal("expected error when confirming cancelled order")
	}
}

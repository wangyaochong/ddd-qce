package application

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
	"github.com/ddd-qce/exampleapp/domain"
)

func setupTestApp() (*commandmemory.CommandBus, *querymemory.QueryBus, *eventmemory.EventBus, *OrderRepository, *domain.Inventory) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	queryBus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
	eventBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))
	repo := NewOrderRepository()
	inventory := domain.NewInventory()

	commandmemory.RegisterCommand(cmdBus, NewPlaceOrderHandler(repo, eventBus))
	commandmemory.RegisterCommand(cmdBus, NewConfirmPaymentHandler(repo, eventBus))
	commandmemory.RegisterCommand(cmdBus, NewShipOrderHandler(repo, eventBus))
	commandmemory.RegisterCommand(cmdBus, NewCancelOrderHandler(repo, eventBus))
	commandmemory.RegisterCommand(cmdBus, NewReserveInventoryHandler(inventory, eventBus))
	commandmemory.RegisterCommand(cmdBus, NewReleaseInventoryHandler(inventory, eventBus))

	querymemory.RegisterQuery(queryBus, NewGetOrderHandler(repo))
	querymemory.RegisterQuery(queryBus, NewListOrdersHandler(repo))
	querymemory.RegisterQuery(queryBus, NewGetInventoryHandler(inventory))

	return cmdBus, queryBus, eventBus, repo, inventory
}

func TestPlaceOrderCommand(t *testing.T) {
	cmdBus, _, _, _, _ := setupTestApp()
	ctx := context.Background()
	result, err := commandmemory.Dispatch[*PlaceOrderCommand, *PlaceOrderResult](ctx, cmdBus, &PlaceOrderCommand{
		UserID: "user-001",
		Items: []ItemInput{
			{ProductID: "laptop", ProductName: "Laptop", Price: 999.99, Quantity: 1},
		},
	})
	if err != nil {
		t.Fatalf("place order failed: %v", err)
	}
	if result.OrderID == "" {
		t.Error("expected non-empty order ID")
	}
	if result.TotalAmount != 999.99 {
		t.Errorf("expected 999.99, got %.2f", result.TotalAmount)
	}
}

func TestConfirmPaymentCommand(t *testing.T) {
	cmdBus, _, _, _, _ := setupTestApp()
	ctx := context.Background()
	placed, _ := commandmemory.Dispatch[*PlaceOrderCommand, *PlaceOrderResult](ctx, cmdBus, &PlaceOrderCommand{
		UserID: "user-001",
		Items:  []ItemInput{{ProductID: "laptop", ProductName: "Laptop", Price: 999, Quantity: 1}},
	})
	result, err := commandmemory.Dispatch[*ConfirmPaymentCommand, *ConfirmPaymentResult](ctx, cmdBus, &ConfirmPaymentCommand{OrderID: placed.OrderID})
	if err != nil {
		t.Fatalf("confirm payment failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestShipOrderCommand(t *testing.T) {
	cmdBus, _, _, _, _ := setupTestApp()
	ctx := context.Background()
	placed, _ := commandmemory.Dispatch[*PlaceOrderCommand, *PlaceOrderResult](ctx, cmdBus, &PlaceOrderCommand{
		UserID: "user-001",
		Items:  []ItemInput{{ProductID: "laptop", ProductName: "Laptop", Price: 999, Quantity: 1}},
	})
	commandmemory.Dispatch[*ConfirmPaymentCommand, *ConfirmPaymentResult](ctx, cmdBus, &ConfirmPaymentCommand{OrderID: placed.OrderID})
	result, err := commandmemory.Dispatch[*ShipOrderCommand, *ShipOrderResult](ctx, cmdBus, &ShipOrderCommand{OrderID: placed.OrderID})
	if err != nil {
		t.Fatalf("ship failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestCancelOrderCommand(t *testing.T) {
	cmdBus, _, _, _, _ := setupTestApp()
	ctx := context.Background()
	placed, _ := commandmemory.Dispatch[*PlaceOrderCommand, *PlaceOrderResult](ctx, cmdBus, &PlaceOrderCommand{
		UserID: "user-001",
		Items:  []ItemInput{{ProductID: "laptop", ProductName: "Laptop", Price: 999, Quantity: 1}},
	})
	result, err := commandmemory.Dispatch[*CancelOrderCommand, *CancelOrderResult](ctx, cmdBus, &CancelOrderCommand{OrderID: placed.OrderID, Reason: "test"})
	if err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
	if !result.Success {
		t.Error("expected success")
	}
}

func TestReserveInventoryCommand(t *testing.T) {
	cmdBus, _, _, _, inv := setupTestApp()
	ctx := context.Background()
	_, err := commandmemory.Dispatch[*ReserveInventoryCommand, *ReserveInventoryResult](ctx, cmdBus, &ReserveInventoryCommand{
		OrderID: "O1", ProductID: "laptop", Quantity: 1,
	})
	if err != nil {
		t.Fatalf("reserve failed: %v", err)
	}
	p, ok := inv.GetByID("laptop")
	if !ok {
		t.Fatal("product not found")
	}
	if p.Stock != 9 {
		t.Errorf("expected stock 9, got %d", p.Stock)
	}
}

func TestReleaseInventoryCommand(t *testing.T) {
	cmdBus, _, _, _, inv := setupTestApp()
	ctx := context.Background()
	commandmemory.Dispatch[*ReserveInventoryCommand, *ReserveInventoryResult](ctx, cmdBus, &ReserveInventoryCommand{
		OrderID: "O1", ProductID: "laptop", Quantity: 1,
	})
	_, err := commandmemory.Dispatch[*ReleaseInventoryCommand, *ReleaseInventoryResult](ctx, cmdBus, &ReleaseInventoryCommand{
		OrderID: "O1", ProductID: "laptop", Quantity: 1,
	})
	if err != nil {
		t.Fatalf("release failed: %v", err)
	}
	p, _ := inv.GetByID("laptop")
	if p.Stock != 10 {
		t.Errorf("expected stock 10, got %d", p.Stock)
	}
}

func TestGetOrderQuery(t *testing.T) {
	_, qBus, _, repo, _ := setupTestApp()
	ctx := context.Background()
	order, _ := domain.NewOrder("ORD-Q1", "user-001", []*domain.OrderItem{
		domain.NewOrderItem("laptop", "Laptop", 999, 1),
	})
	repo.Save(ctx, order)
	result, err := querymemory.Dispatch[*GetOrderQuery, *GetOrderResult](ctx, qBus, &GetOrderQuery{OrderID: "ORD-Q1"})
	if err != nil {
		t.Fatalf("get order failed: %v", err)
	}
	if result.OrderID != "ORD-Q1" {
		t.Errorf("expected ORD-Q1, got %s", result.OrderID)
	}
}

func TestListOrdersQuery(t *testing.T) {
	_, qBus, _, repo, _ := setupTestApp()
	ctx := context.Background()
	o1, _ := domain.NewOrder("ORD-L1", "u1", []*domain.OrderItem{domain.NewOrderItem("laptop", "Laptop", 999, 1)})
	o2, _ := domain.NewOrder("ORD-L2", "u2", []*domain.OrderItem{domain.NewOrderItem("mouse", "Mouse", 29, 1)})
	repo.Save(ctx, o1)
	repo.Save(ctx, o2)
	result, err := querymemory.Dispatch[*ListOrdersQuery, *ListOrdersResult](ctx, qBus, &ListOrdersQuery{})
	if err != nil {
		t.Fatalf("list orders failed: %v", err)
	}
	if len(result.Orders) != 2 {
		t.Errorf("expected 2 orders, got %d", len(result.Orders))
	}
}

func TestGetInventoryQuery(t *testing.T) {
	_, qBus, _, _, _ := setupTestApp()
	ctx := context.Background()
	result, err := querymemory.Dispatch[*GetInventoryQuery, *GetInventoryResult](ctx, qBus, &GetInventoryQuery{})
	if err != nil {
		t.Fatalf("get inventory failed: %v", err)
	}
	if len(result.Products) != 5 {
		t.Errorf("expected 5 products, got %d", len(result.Products))
	}
}

func TestOrderRepository_CRUD(t *testing.T) {
	repo := NewOrderRepository()
	ctx := context.Background()
	order, _ := domain.NewOrder("ORD-CRUD", "user-001", []*domain.OrderItem{
		domain.NewOrderItem("laptop", "Laptop", 999, 1),
	})
	if err := repo.Save(ctx, order); err != nil {
		t.Fatalf("save failed: %v", err)
	}
	found, err := repo.FindByID(ctx, "ORD-CRUD")
	if err != nil {
		t.Fatalf("find failed: %v", err)
	}
	if found.GetID() != "ORD-CRUD" {
		t.Errorf("expected ORD-CRUD, got %s", found.GetID())
	}
	if err := repo.Delete(ctx, "ORD-CRUD"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	_, err = repo.FindByID(ctx, "ORD-CRUD")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestEventSourcedRepo_SaveAndLoad(t *testing.T) {
	_ = aspect.NewAspectChain()
	repo := NewOrderRepository()
	eventStore := eventmemory.NewDomainEventStore()
	esRepo := NewOrderEventSourcedRepository(eventStore, repo)
	ctx := context.Background()

	order, _ := domain.NewOrder("ORD-ES1", "user-001", []*domain.OrderItem{
		domain.NewOrderItem("laptop", "Laptop", 999, 1),
	})
	if err := esRepo.Save(ctx, order); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := esRepo.Load(ctx, "ORD-ES1")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.GetID() != "ORD-ES1" {
		t.Errorf("expected ORD-ES1, got %s", loaded.GetID())
	}
	if loaded.UserID != "user-001" {
		t.Errorf("expected user-001, got %s", loaded.UserID)
	}
}

func TestEventSourcedRepo_LoadFromHistory(t *testing.T) {
	_ = aspect.NewAspectChain()
	repo := NewOrderRepository()
	eventStore := eventmemory.NewDomainEventStore()
	esRepo := NewOrderEventSourcedRepository(eventStore, repo)
	ctx := context.Background()

	order, _ := domain.NewOrder("ORD-ES2", "user-001", []*domain.OrderItem{
		domain.NewOrderItem("laptop", "Laptop", 999, 1),
	})
	esRepo.Save(ctx, order)

	loaded, err := esRepo.Load(ctx, "ORD-ES2")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.AggregateRoot.GetVersion() != 1 {
		t.Errorf("expected version 1, got %d", loaded.AggregateRoot.GetVersion())
	}
}

func TestEventSourcedRepo_MultipleEventTypes(t *testing.T) {
	repo := NewOrderRepository()
	eventStore := eventmemory.NewDomainEventStore()
	esRepo := NewOrderEventSourcedRepository(eventStore, repo)
	ctx := context.Background()

	order, _ := domain.NewOrder("ORD-MT", "user-001", []*domain.OrderItem{
		domain.NewOrderItem("laptop", "Laptop", 999, 1),
	})
	order.ConfirmPayment()
	order.Ship()
	if err := esRepo.Save(ctx, order); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := esRepo.Load(ctx, "ORD-MT")
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.Status != domain.OrderStatusShipped {
		t.Errorf("expected shipped, got %s", loaded.Status)
	}
	if loaded.AggregateRoot.GetVersion() != 3 {
		t.Errorf("expected version 3, got %d", loaded.AggregateRoot.GetVersion())
	}
}

func TestCommandBus_Execute_TypeErased(t *testing.T) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	repo := NewOrderRepository()
	eventBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))
	commandmemory.RegisterCommand(cmdBus, NewPlaceOrderHandler(repo, eventBus))
	ctx := context.Background()

	var executor command.CommandExecutor = cmdBus
	result, err := executor.Execute(ctx, &PlaceOrderCommand{
		UserID: "user-001",
		Items:  []ItemInput{{ProductID: "laptop", ProductName: "Laptop", Price: 999, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	placed, ok := result.(*PlaceOrderResult)
	if !ok {
		t.Fatal("expected *PlaceOrderResult")
	}
	if placed.OrderID == "" {
		t.Error("expected non-empty order ID")
	}
}

func TestCommandNameOf(t *testing.T) {
	name := command.CommandNameOf(&PlaceOrderCommand{})
	if name != "PlaceOrderCommand" {
		t.Errorf("expected PlaceOrderCommand, got %s", name)
	}
}

func TestQueryNameOf(t *testing.T) {
	name := query.QueryNameOf(&GetOrderQuery{})
	if name != "GetOrderQuery" {
		t.Errorf("expected GetOrderQuery, got %s", name)
	}
}

func TestOrderPlacedEventHandler(t *testing.T) {
	cmdBus, _, eventBus, _, _ := setupTestApp()
	ctx := context.Background()
	eventmemory.RegisterHandler[*domain.OrderPlacedEvent](eventBus, NewOrderPlacedInventoryHandler(cmdBus))

	eventmemory.Dispatch[*domain.OrderPlacedEvent](ctx, eventBus, &domain.OrderPlacedEvent{
		OrderID: "O1", UserID: "u1", TotalAmount: 100, Time: time.Now(),
	})
	time.Sleep(100 * time.Millisecond)
}

func TestOrderCancelledEventHandler(t *testing.T) {
	cmdBus, _, eventBus, _, _ := setupTestApp()
	ctx := context.Background()
	eventmemory.RegisterHandler[*domain.OrderCancelledEvent](eventBus, NewOrderCancelledInventoryHandler(cmdBus))

	eventmemory.Dispatch[*domain.OrderCancelledEvent](ctx, eventBus, &domain.OrderCancelledEvent{
		OrderID: "O1", Reason: "test", Time: time.Now(),
	})
	time.Sleep(100 * time.Millisecond)
}

func TestMultiEventHandler_ConcurrentPublish(t *testing.T) {
	chain := aspect.NewAspectChain()
	freshBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))
	eventmemory.RegisterHandler[*domain.OrderPlacedEvent](freshBus, NewOrderPlacedNotificationHandler())
	eventmemory.RegisterHandler[*domain.OrderPlacedEvent](freshBus, &loggingNotificationHandler{})
	ctx := context.Background()
	eventmemory.Dispatch[*domain.OrderPlacedEvent](ctx, freshBus, &domain.OrderPlacedEvent{
		OrderID: "O1", UserID: "u1", TotalAmount: 100, Time: time.Now(),
	})
	time.Sleep(100 * time.Millisecond)
}

type loggingNotificationHandler struct {
	called bool
}

func (h *loggingNotificationHandler) Handle(ctx context.Context, evt *domain.OrderPlacedEvent) error {
	h.called = true
	return nil
}

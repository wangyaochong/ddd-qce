package application

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ddd-qce/core/cqrs/command"
	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
	"github.com/ddd-qce/exampleapp/domain"
)

type PlaceOrderHandler struct {
	repo     *OrderRepository
	eventBus *eventmemory.EventBus
}

func NewPlaceOrderHandler(repo *OrderRepository, eventBus *eventmemory.EventBus) *PlaceOrderHandler {
	return &PlaceOrderHandler{repo: repo, eventBus: eventBus}
}

func (h *PlaceOrderHandler) Handle(ctx context.Context, cmd *PlaceOrderCommand) (*PlaceOrderResult, error) {
	items := make([]*domain.OrderItem, len(cmd.Items))
	for i, input := range cmd.Items {
		items[i] = domain.NewOrderItem(input.ProductID, input.ProductName, input.Price, input.Quantity)
	}

	orderID := uuid.New().String()
	order, err := domain.NewOrder(orderID, cmd.UserID, items)
	if err != nil {
		return nil, err
	}

	if err := h.repo.Save(ctx, order); err != nil {
		return nil, err
	}

	eventmemory.Dispatch[*domain.OrderPlacedEvent](ctx, h.eventBus, &domain.OrderPlacedEvent{
		OrderID:     order.ID(),
		UserID:      order.UserID,
		TotalAmount: order.TotalAmount,
		Items:       order.ItemNames(),
		Time:        time.Now(),
	})

	return &PlaceOrderResult{OrderID: order.ID(), TotalAmount: order.TotalAmount}, nil
}

type ConfirmPaymentHandler struct {
	repo     *OrderRepository
	eventBus *eventmemory.EventBus
}

func NewConfirmPaymentHandler(repo *OrderRepository, eventBus *eventmemory.EventBus) *ConfirmPaymentHandler {
	return &ConfirmPaymentHandler{repo: repo, eventBus: eventBus}
}

func (h *ConfirmPaymentHandler) Handle(ctx context.Context, cmd *ConfirmPaymentCommand) (*ConfirmPaymentResult, error) {
	order, err := h.repo.FindByID(ctx, cmd.OrderID)
	if err != nil {
		return nil, err
	}
	if err := order.ConfirmPayment(); err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, order); err != nil {
		return nil, err
	}
	return &ConfirmPaymentResult{Success: true}, nil
}

type ShipOrderHandler struct {
	repo     *OrderRepository
	eventBus *eventmemory.EventBus
}

func NewShipOrderHandler(repo *OrderRepository, eventBus *eventmemory.EventBus) *ShipOrderHandler {
	return &ShipOrderHandler{repo: repo, eventBus: eventBus}
}

func (h *ShipOrderHandler) Handle(ctx context.Context, cmd *ShipOrderCommand) (*ShipOrderResult, error) {
	order, err := h.repo.FindByID(ctx, cmd.OrderID)
	if err != nil {
		return nil, err
	}
	if err := order.Ship(); err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, order); err != nil {
		return nil, err
	}
	return &ShipOrderResult{Success: true}, nil
}

type CancelOrderHandler struct {
	repo     *OrderRepository
	eventBus *eventmemory.EventBus
}

func NewCancelOrderHandler(repo *OrderRepository, eventBus *eventmemory.EventBus) *CancelOrderHandler {
	return &CancelOrderHandler{repo: repo, eventBus: eventBus}
}

func (h *CancelOrderHandler) Handle(ctx context.Context, cmd *CancelOrderCommand) (*CancelOrderResult, error) {
	order, err := h.repo.FindByID(ctx, cmd.OrderID)
	if err != nil {
		return nil, err
	}
	if err := order.Cancel(cmd.Reason); err != nil {
		return nil, err
	}
	if err := h.repo.Save(ctx, order); err != nil {
		return nil, err
	}

	eventmemory.Dispatch[*domain.OrderCancelledEvent](ctx, h.eventBus, &domain.OrderCancelledEvent{
		OrderID: order.ID(),
		Reason:  cmd.Reason,
		Time:    time.Now(),
	})

	return &CancelOrderResult{Success: true}, nil
}

type ReserveInventoryHandler struct {
	inventory *domain.Inventory
	eventBus  *eventmemory.EventBus
}

func NewReserveInventoryHandler(inventory *domain.Inventory, eventBus *eventmemory.EventBus) *ReserveInventoryHandler {
	return &ReserveInventoryHandler{inventory: inventory, eventBus: eventBus}
}

func (h *ReserveInventoryHandler) Handle(ctx context.Context, cmd *ReserveInventoryCommand) (*ReserveInventoryResult, error) {
	if err := h.inventory.Reserve(cmd.ProductID, cmd.Quantity); err != nil {
		return nil, err
	}

	eventmemory.Dispatch[*domain.InventoryReservedEvent](ctx, h.eventBus, &domain.InventoryReservedEvent{
		OrderID:   cmd.OrderID,
		ProductID: cmd.ProductID,
		Quantity:  cmd.Quantity,
		Time:      time.Now(),
	})

	return &ReserveInventoryResult{Success: true}, nil
}

type ReleaseInventoryHandler struct {
	inventory *domain.Inventory
	eventBus  *eventmemory.EventBus
}

func NewReleaseInventoryHandler(inventory *domain.Inventory, eventBus *eventmemory.EventBus) *ReleaseInventoryHandler {
	return &ReleaseInventoryHandler{inventory: inventory, eventBus: eventBus}
}

func (h *ReleaseInventoryHandler) Handle(ctx context.Context, cmd *ReleaseInventoryCommand) (*ReleaseInventoryResult, error) {
	if err := h.inventory.Release(cmd.ProductID, cmd.Quantity); err != nil {
		return nil, err
	}

	eventmemory.Dispatch[*domain.InventoryReleasedEvent](ctx, h.eventBus, &domain.InventoryReleasedEvent{
		OrderID:   cmd.OrderID,
		ProductID: cmd.ProductID,
		Quantity:  cmd.Quantity,
		Time:      time.Now(),
	})

	return &ReleaseInventoryResult{Success: true}, nil
}

type GenerateReportHandler struct{}

func NewGenerateReportHandler() *GenerateReportHandler {
	return &GenerateReportHandler{}
}

func (h *GenerateReportHandler) Handle(ctx context.Context, cmd *GenerateReportCommand) (*GenerateReportResult, error) {
	select {
	case <-time.After(200 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	reportID := fmt.Sprintf("RPT-%d", time.Now().UnixNano())
	return &GenerateReportResult{
		ReportID:  reportID,
		OrderID:   cmd.OrderID,
		Content:   fmt.Sprintf("Report for order %s generated at %s", cmd.OrderID, time.Now().Format(time.RFC3339)),
		Generated: true,
	}, nil
}

var _ command.CommandHandler[*GenerateReportCommand, *GenerateReportResult] = (*GenerateReportHandler)(nil)

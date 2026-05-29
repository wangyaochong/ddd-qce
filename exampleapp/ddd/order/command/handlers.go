package command

import (
	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/ddd-qce/core/cqrs/command"
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
	"github.com/ddd-qce/exampleapp/ddd/order/repository"
)

type PlaceOrderHandler struct {
	Repo     repository.OrderRepositoryAdapter
	EventBus cqrsevent.EventBus
}

func NewPlaceOrderHandler(repo repository.OrderRepositoryAdapter, eventBus cqrsevent.EventBus) *PlaceOrderHandler {
	return &PlaceOrderHandler{Repo: repo, EventBus: eventBus}
}

func (h *PlaceOrderHandler) Handle(ctx context.Context, cmd *PlaceOrderCommand) (*PlaceOrderResult, error) {
	items := make([]*orderdomain.OrderItem, len(cmd.Items))
	for i, input := range cmd.Items {
		items[i] = orderdomain.NewOrderItem(input.ProductID, input.ProductName, input.Price, input.Quantity)
	}

	uid := uuid.New()
	orderID := orderdomain.NewOrderID(hex.EncodeToString(uid[:]))
	order, err := orderdomain.NewOrder(ctx, orderID, cmd.UserID, items)
	if err != nil {
		return nil, err
	}

	if err := h.Repo.Save(ctx, order); err != nil {
		return nil, err
	}

	return &PlaceOrderResult{OrderID: orderdomain.OrderID(order.ID()), TotalAmount: order.TotalAmount}, nil
}

type ConfirmPaymentHandler struct {
	Repo     repository.OrderRepositoryAdapter
	EventBus cqrsevent.EventBus
}

func NewConfirmPaymentHandler(repo repository.OrderRepositoryAdapter, eventBus cqrsevent.EventBus) *ConfirmPaymentHandler {
	return &ConfirmPaymentHandler{Repo: repo, EventBus: eventBus}
}

func (h *ConfirmPaymentHandler) Handle(ctx context.Context, cmd *ConfirmPaymentCommand) (*ConfirmPaymentResult, error) {
	order, err := h.Repo.FindByID(ctx, cmd.OrderID.String())
	if err != nil {
		return nil, err
	}
	if err := order.ConfirmPayment(ctx); err != nil {
		return nil, err
	}
	if err := h.Repo.Save(ctx, order); err != nil {
		return nil, err
	}
	return &ConfirmPaymentResult{Success: true}, nil
}

type ShipOrderHandler struct {
	Repo     repository.OrderRepositoryAdapter
	EventBus cqrsevent.EventBus
}

func NewShipOrderHandler(repo repository.OrderRepositoryAdapter, eventBus cqrsevent.EventBus) *ShipOrderHandler {
	return &ShipOrderHandler{Repo: repo, EventBus: eventBus}
}

func (h *ShipOrderHandler) Handle(ctx context.Context, cmd *ShipOrderCommand) (*ShipOrderResult, error) {
	order, err := h.Repo.FindByID(ctx, cmd.OrderID.String())
	if err != nil {
		return nil, err
	}
	if err := order.Ship(ctx); err != nil {
		return nil, err
	}
	if err := h.Repo.Save(ctx, order); err != nil {
		return nil, err
	}
	return &ShipOrderResult{Success: true}, nil
}

type CancelOrderHandler struct {
	Repo     repository.OrderRepositoryAdapter
	EventBus cqrsevent.EventBus
}

func NewCancelOrderHandler(repo repository.OrderRepositoryAdapter, eventBus cqrsevent.EventBus) *CancelOrderHandler {
	return &CancelOrderHandler{Repo: repo, EventBus: eventBus}
}

func (h *CancelOrderHandler) Handle(ctx context.Context, cmd *CancelOrderCommand) (*CancelOrderResult, error) {
	order, err := h.Repo.FindByID(ctx, cmd.OrderID.String())
	if err != nil {
		return nil, err
	}
	if err := order.Cancel(ctx, cmd.Reason); err != nil {
		return nil, err
	}
	if err := h.Repo.Save(ctx, order); err != nil {
		return nil, err
	}
	return &CancelOrderResult{Success: true}, nil
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

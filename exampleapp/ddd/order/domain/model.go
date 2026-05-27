package domain

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/domain/entity"
	"github.com/ddd-qce/core/cqrs/event"
	orderevent "github.com/ddd-qce/exampleapp/ddd/order/event"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type OrderItem struct {
	entity.Entity
	ProductName string
	Price       float64
	Quantity    int
}

func NewOrderItem(id, productName string, price float64, quantity int) *OrderItem {
	e, err := entity.NewEntity(id)
	if err != nil {
		panic(err)
	}
	return &OrderItem{
		Entity:      *e,
		ProductName: productName,
		Price:       price,
		Quantity:    quantity,
	}
}

func (i *OrderItem) Subtotal() float64 {
	return i.Price * float64(i.Quantity)
}

type orderItemJSON struct {
	entity.EntityJSON
	ProductName string  `json:"productName"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
}

func (i *OrderItem) MarshalJSON() ([]byte, error) {
	return json.Marshal(orderItemJSON{
		EntityJSON:  i.Entity.ToJSON(),
		ProductName: i.ProductName,
		Price:       i.Price,
		Quantity:    i.Quantity,
	})
}

func (i *OrderItem) UnmarshalJSON(data []byte) error {
	var aux orderItemJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	i.Entity.FromJSON(aux.EntityJSON)
	i.ProductName = aux.ProductName
	i.Price = aux.Price
	i.Quantity = aux.Quantity
	return nil
}

type Order struct {
	aggregate.AggregateRoot
	UserID       string
	Items        []*OrderItem
	Status       OrderStatus
	TotalAmount  float64
	CreatedAt    time.Time
	PaidAt       time.Time
	ShippedAt    time.Time
	CancelledAt  time.Time
	CancelReason string
}

func NewOrder(ctx context.Context, id, userID string, items []*OrderItem) (*Order, error) {
	o := &Order{
		UserID:    userID,
		Items:     items,
		Status:    OrderStatusPending,
		CreatedAt: time.Now(),
	}
	ar, err := aggregate.NewAggregateRoot(id)
	if err != nil {
		return nil, err
	}
	o.AggregateRoot = *ar
	if err := o.validate(); err != nil {
		return nil, err
	}
	o.TotalAmount = o.calculateTotal()
	if err := o.Apply(ctx, &orderevent.OrderPlacedEvent{
		BaseEvent:   event.WithCorrelation(ctx, o.ID()),
		UserID:      o.UserID,
		TotalAmount: o.TotalAmount,
		Items:       o.ItemNames(),
	}); err != nil {
		return nil, err
	}
	return o, nil
}

func NewOrderForReplay(id string) *Order {
	o := &Order{}
	ar, err := aggregate.NewAggregateRoot(id)
	if err != nil {
		panic(err)
	}
	o.AggregateRoot = *ar
	return o
}

func (o *Order) When(evt event.Event) error {
	switch e := evt.(type) {
	case *orderevent.OrderPlacedEvent:
		o.UserID = e.UserID
		o.TotalAmount = e.TotalAmount
		o.Status = OrderStatusPending
		o.CreatedAt = e.OccurredAt()
	case *orderevent.PaymentConfirmedEvent:
		o.Status = OrderStatusPaid
		o.PaidAt = e.OccurredAt()
	case *orderevent.OrderShippedEvent:
		o.Status = OrderStatusShipped
		o.ShippedAt = e.OccurredAt()
	case *orderevent.OrderCancelledEvent:
		o.Status = OrderStatusCancelled
		o.CancelledAt = e.OccurredAt()
		o.CancelReason = e.Reason
	default:
		return fmt.Errorf("order: unhandled event type %T", evt)
	}
	return nil
}

func (o *Order) Apply(ctx context.Context, evt event.Event) error {
	return aggregate.ApplyChange(o, ctx, evt)
}

func (o *Order) LoadFromHistory(events []event.Event) error {
	return aggregate.LoadFromHistory(o, events)
}

type orderJSON struct {
	aggregate.AggregateRootJSON
	UserID       string       `json:"userId"`
	Items        []*OrderItem `json:"items"`
	Status       OrderStatus  `json:"status"`
	TotalAmount  float64      `json:"totalAmount"`
	CreatedAt    time.Time    `json:"createdAt"`
	PaidAt       time.Time    `json:"paidAt"`
	ShippedAt    time.Time    `json:"shippedAt"`
	CancelledAt  time.Time    `json:"cancelledAt"`
	CancelReason string       `json:"cancelReason"`
}

func (o *Order) MarshalJSON() ([]byte, error) {
	return json.Marshal(orderJSON{
		AggregateRootJSON: o.AggregateRoot.ToJSON(),
		UserID:            o.UserID,
		Items:             o.Items,
		Status:            o.Status,
		TotalAmount:       o.TotalAmount,
		CreatedAt:         o.CreatedAt,
		PaidAt:            o.PaidAt,
		ShippedAt:         o.ShippedAt,
		CancelledAt:       o.CancelledAt,
		CancelReason:      o.CancelReason,
	})
}

func (o *Order) UnmarshalJSON(data []byte) error {
	var aux orderJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	o.AggregateRoot.FromJSON(aux.AggregateRootJSON)
	o.UserID = aux.UserID
	o.Items = aux.Items
	o.Status = aux.Status
	o.TotalAmount = aux.TotalAmount
	o.CreatedAt = aux.CreatedAt
	o.PaidAt = aux.PaidAt
	o.ShippedAt = aux.ShippedAt
	o.CancelledAt = aux.CancelledAt
	o.CancelReason = aux.CancelReason
	return nil
}

func (o *Order) ConfirmPayment(ctx context.Context) error {
	if o.Status != OrderStatusPending {
		return fmt.Errorf("order %s cannot be confirmed payment (status: %s)", o.ID(), o.Status)
	}
	if err := o.Apply(ctx, &orderevent.PaymentConfirmedEvent{
		BaseEvent: event.WithCorrelation(ctx, o.ID()),
	}); err != nil {
		return err
	}
	return nil
}

func (o *Order) Ship(ctx context.Context) error {
	if o.Status != OrderStatusPaid {
		return fmt.Errorf("order %s cannot be shipped (status: %s)", o.ID(), o.Status)
	}
	if err := o.Apply(ctx, &orderevent.OrderShippedEvent{
		BaseEvent: event.WithCorrelation(ctx, o.ID()),
	}); err != nil {
		return err
	}
	return nil
}

func (o *Order) Cancel(ctx context.Context, reason string) error {
	if o.Status == OrderStatusCancelled {
		return fmt.Errorf("order %s already cancelled", o.ID())
	}
	if o.Status == OrderStatusShipped {
		return fmt.Errorf("order %s already shipped, cannot cancel", o.ID())
	}
	if err := o.Apply(ctx, &orderevent.OrderCancelledEvent{
		BaseEvent: event.WithCorrelation(ctx, o.ID()),
		Reason:    reason,
	}); err != nil {
		return err
	}
	return nil
}

func (o *Order) validate() error {
	if err := o.AggregateRoot.Validate(); err != nil {
		return err
	}
	if o.UserID == "" {
		return fmt.Errorf("order must have a user ID")
	}
	if len(o.Items) == 0 {
		return fmt.Errorf("order must have at least one item")
	}
	for _, item := range o.Items {
		if item.IsEmpty() {
			return fmt.Errorf("order item has empty product ID")
		}
	}
	return nil
}

func (o *Order) calculateTotal() float64 {
	var total float64
	for _, item := range o.Items {
		total += item.Subtotal()
	}
	return total
}

func (o *Order) ItemNames() []string {
	names := make([]string, len(o.Items))
	for i, item := range o.Items {
		names[i] = item.ProductName
	}
	return names
}

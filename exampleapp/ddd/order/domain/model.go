package domain

import (
	"context"
	"fmt"
	"time"

	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/domain/entity"
	domainevent "github.com/ddd-qce/core/domain/event"
	orderevent "github.com/ddd-qce/exampleapp/ddd/order/event"
)

type OrderID string

func (id OrderID) String() string { return string(id) }
func NewOrderID(s string) OrderID { return OrderID(s) }

type UserID string

func (id UserID) String() string { return string(id) }
func NewUserID(s string) UserID  { return UserID(s) }

type ProductID string

func (id ProductID) String() string   { return string(id) }
func NewProductID(s string) ProductID { return ProductID(s) }

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusShipped   OrderStatus = "shipped"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type OrderItem struct {
	entity.Entity
	ProductName string  `json:"productName"`
	Price       float64 `json:"price"`
	Quantity    int     `json:"quantity"`
}

func NewOrderItem(id ProductID, productName string, price float64, quantity int) *OrderItem {
	e, err := entity.NewEntity(id.String())
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

func (i *OrderItem) MarshalJSON() ([]byte, error) {
	return entity.MarshalEntity(i)
}

func (i *OrderItem) UnmarshalJSON(data []byte) error {
	return entity.UnmarshalEntity(data, i)
}

type Order struct {
	aggregate.AggregateRoot
	UserID       UserID       `json:"userId"`
	Items        []*OrderItem `json:"items"`
	Status       OrderStatus  `json:"status"`
	TotalAmount  float64      `json:"totalAmount"`
	CreatedAt    time.Time    `json:"createdAt"`
	PaidAt       time.Time    `json:"paidAt"`
	ShippedAt    time.Time    `json:"shippedAt"`
	CancelledAt  time.Time    `json:"cancelledAt"`
	CancelReason string       `json:"cancelReason"`
}

func NewOrder(ctx context.Context, id OrderID, userID UserID, items []*OrderItem) (*Order, error) {
	o := &Order{
		UserID:    userID,
		Items:     items,
		Status:    OrderStatusPending,
		CreatedAt: time.Now(),
	}
	ar, err := aggregate.NewAggregateRoot(id.String())
	if err != nil {
		return nil, err
	}
	o.AggregateRoot = *ar
	if err := o.validate(); err != nil {
		return nil, err
	}
	o.TotalAmount = o.calculateTotal()
	if err := o.Apply(ctx, &orderevent.OrderPlacedEvent{
		BaseEvent:   cqrsevent.WithCorrelation(ctx, o.ID()),
		UserID:      string(o.UserID),
		TotalAmount: o.TotalAmount,
		Items:       o.ItemNames(),
	}); err != nil {
		return nil, err
	}
	return o, nil
}

func NewOrderForReplay(id OrderID) *Order {
	o := &Order{}
	ar, err := aggregate.NewAggregateRoot(id.String())
	if err != nil {
		panic(err)
	}
	o.AggregateRoot = *ar
	return o
}

func (o *Order) When(evt domainevent.Event) error {
	switch e := evt.(type) {
	case *orderevent.OrderPlacedEvent:
		o.UserID = UserID(e.UserID)
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

func (o *Order) Apply(ctx context.Context, evt domainevent.Event) error {
	return aggregate.ApplyChange(o, ctx, evt)
}

func (o *Order) LoadFromHistory(events []domainevent.Event) error {
	return aggregate.LoadFromHistory(o, events)
}

func (o *Order) MarshalJSON() ([]byte, error) {
	return aggregate.MarshalAggregate(o)
}

func (o *Order) UnmarshalJSON(data []byte) error {
	return aggregate.UnmarshalAggregate(data, o)
}

func (o *Order) ConfirmPayment(ctx context.Context) error {
	if o.Status != OrderStatusPending {
		return fmt.Errorf("order %s cannot be confirmed payment (status: %s)", o.ID(), o.Status)
	}
	if err := o.Apply(ctx, &orderevent.PaymentConfirmedEvent{
		BaseEvent: cqrsevent.WithCorrelation(ctx, o.ID()),
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
		BaseEvent: cqrsevent.WithCorrelation(ctx, o.ID()),
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
		BaseEvent: cqrsevent.WithCorrelation(ctx, o.ID()),
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

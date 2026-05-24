package domain

import (
	"fmt"
	"time"

	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/domain/entity"
	"github.com/ddd-qce/core/domain/event"
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
	return &OrderItem{
		Entity:      *entity.NewEntity(id),
		ProductName: productName,
		Price:       price,
		Quantity:    quantity,
	}
}

func (i *OrderItem) Subtotal() float64 {
	return i.Price * float64(i.Quantity)
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

func NewOrder(id, userID string, items []*OrderItem) (*Order, error) {
	o := &Order{
		UserID:    userID,
		Items:     items,
		Status:    OrderStatusPending,
		CreatedAt: time.Now(),
	}
	o.AggregateRoot = *aggregate.NewAggregateRootWithApplier(id, o)
	if err := o.validate(); err != nil {
		return nil, err
	}
	o.TotalAmount = o.calculateTotal()
	o.Apply(&OrderPlacedEvent{
		OrderID:     o.ID,
		UserID:      o.UserID,
		TotalAmount: o.TotalAmount,
		Items:       o.ItemNames(),
		Time:        time.Now(),
	})
	return o, nil
}

func NewOrderForReplay(id string) *Order {
	o := &Order{}
	o.AggregateRoot = *aggregate.NewAggregateRoot(id)
	o.SetApplier(o)
	return o
}

func NewOrderEventCollector(id string) *aggregate.AggregateRoot {
	return aggregate.NewEventCollector(id)
}

func (o *Order) When(evt event.DomainEvent) {
	switch e := evt.(type) {
	case *OrderPlacedEvent:
		o.UserID = e.UserID
		o.TotalAmount = e.TotalAmount
		o.Status = OrderStatusPending
		o.CreatedAt = e.Time
	case *PaymentConfirmedEvent:
		o.Status = OrderStatusPaid
		o.PaidAt = e.Time
	case *OrderShippedEvent:
		o.Status = OrderStatusShipped
		o.ShippedAt = e.Time
	case *OrderCancelledEvent:
		o.Status = OrderStatusCancelled
		o.CancelledAt = e.Time
		o.CancelReason = e.Reason
	}
}

func (o *Order) ConfirmPayment() error {
	if o.Status != OrderStatusPending {
		return fmt.Errorf("order %s cannot be confirmed payment (status: %s)", o.ID, o.Status)
	}
	o.Apply(&PaymentConfirmedEvent{
		OrderID: o.ID,
		Time:    time.Now(),
	})
	return nil
}

func (o *Order) Ship() error {
	if o.Status != OrderStatusPaid {
		return fmt.Errorf("order %s cannot be shipped (status: %s)", o.ID, o.Status)
	}
	o.Apply(&OrderShippedEvent{
		OrderID: o.ID,
		Time:    time.Now(),
	})
	return nil
}

func (o *Order) Cancel(reason string) error {
	if o.Status == OrderStatusCancelled {
		return fmt.Errorf("order %s already cancelled", o.ID)
	}
	if o.Status == OrderStatusShipped {
		return fmt.Errorf("order %s already shipped, cannot cancel", o.ID)
	}
	o.Apply(&OrderCancelledEvent{
		OrderID: o.ID,
		Reason:  reason,
		Time:    time.Now(),
	})
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

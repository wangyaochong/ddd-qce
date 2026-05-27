package event

import "myproject/ddd/order/domain"

type OrderPlacedEvent struct {
	Order domain.Order
}

type PaymentConfirmedEvent struct {
	OrderID string
}

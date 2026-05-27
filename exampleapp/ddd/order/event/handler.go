package event

import (
	"context"
	"fmt"
)

type OrderPlacedNotificationHandler struct{}

func NewOrderPlacedNotificationHandler() *OrderPlacedNotificationHandler {
	return &OrderPlacedNotificationHandler{}
}

func (h *OrderPlacedNotificationHandler) Handle(ctx context.Context, evt *OrderPlacedEvent) error {
	fmt.Printf("[Notification] New order %s placed by user %s, total: $%.2f\n",
		evt.AggregateID(), evt.UserID, evt.TotalAmount)
	return nil
}

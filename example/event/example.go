package event

import (
	"context"
	"fmt"
	"time"

	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
	"github.com/ddd-qce/core/domain/event"
)

type UserCreatedEvent struct {
	UserID string
	Name   string
	Time   time.Time
}

func (e *UserCreatedEvent) AggregateID() string   { return e.UserID }
func (e *UserCreatedEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *UserCreatedEvent) OccurredAt() time.Time { return e.Time }

type UserUpdatedEvent struct {
	UserID string
	Name   string
	Time   time.Time
}

func (e *UserUpdatedEvent) AggregateID() string   { return e.UserID }
func (e *UserUpdatedEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *UserUpdatedEvent) OccurredAt() time.Time { return e.Time }

type OrderCancelledEvent struct {
	OrderID string
	Reason  string
	Time    time.Time
}

func (e *OrderCancelledEvent) AggregateID() string   { return e.OrderID }
func (e *OrderCancelledEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *OrderCancelledEvent) OccurredAt() time.Time { return e.Time }

type UserCreatedEventHandler struct{}

func (h *UserCreatedEventHandler) Handle(ctx context.Context, event *UserCreatedEvent) error {
	fmt.Printf("Event handled: User %s created\n", event.Name)
	return nil
}

type UserUpdatedEventHandler struct{}

func (h *UserUpdatedEventHandler) Handle(ctx context.Context, event *UserUpdatedEvent) error {
	fmt.Printf("Event handled: User %s updated\n", event.Name)
	return nil
}

type OrderCancelledEventHandler struct{}

func (h *OrderCancelledEventHandler) Handle(ctx context.Context, event *OrderCancelledEvent) error {
	fmt.Printf("Event handled: Order %s cancelled, reason: %s\n", event.OrderID, event.Reason)
	return nil
}

func RegisterHandlers(bus *eventmemory.EventBus) {
	eventmemory.RegisterHandler[*UserCreatedEvent](bus, &UserCreatedEventHandler{})
	eventmemory.RegisterHandler[*UserUpdatedEvent](bus, &UserUpdatedEventHandler{})
	eventmemory.RegisterHandler[*OrderCancelledEvent](bus, &OrderCancelledEventHandler{})
}

func RunExample(ctx context.Context, bus *eventmemory.EventBus) {
	fmt.Println("=== Event: UserCreated ===")
	err := eventmemory.Dispatch[*UserCreatedEvent](ctx, bus, &UserCreatedEvent{
		UserID: "user-001",
		Name:   "李四",
		Time:   time.Now(),
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("\n=== Event: UserUpdated ===")
	err = eventmemory.Dispatch[*UserUpdatedEvent](ctx, bus, &UserUpdatedEvent{
		UserID: "1",
		Name:   "张三更新",
		Time:   time.Now(),
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("\n=== Event: OrderCancelled ===")
	err = eventmemory.Dispatch[*OrderCancelledEvent](ctx, bus, &OrderCancelledEvent{
		OrderID: "ORD-001",
		Reason:  "用户取消",
		Time:    time.Now(),
	})
	if err != nil {
		panic(err)
	}
}

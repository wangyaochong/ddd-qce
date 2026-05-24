package event

import (
	"context"
	"fmt"
	"time"

	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
	"github.com/ddd-qce/core/domain/event"
)

type UserCreatedEvent struct {
	event.BaseEvent
	Name string
}

type UserUpdatedEvent struct {
	event.BaseEvent
	Name string
}

type OrderCancelledEvent struct {
	event.BaseEvent
	Reason string
}

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
	fmt.Printf("Event handled: Order %s cancelled, reason: %s\n", event.AggregateID(), event.Reason)
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
		BaseEvent: event.NewBaseEvent("user-001", time.Now()),
		Name:            "李四",
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("\n=== Event: UserUpdated ===")
	err = eventmemory.Dispatch[*UserUpdatedEvent](ctx, bus, &UserUpdatedEvent{
		BaseEvent: event.NewBaseEvent("1", time.Now()),
		Name:            "张三更新",
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("\n=== Event: OrderCancelled ===")
	err = eventmemory.Dispatch[*OrderCancelledEvent](ctx, bus, &OrderCancelledEvent{
		BaseEvent: event.NewBaseEvent("ORD-001", time.Now()),
		Reason:          "用户取消",
	})
	if err != nil {
		panic(err)
	}
}

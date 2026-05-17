package event

import (
	"context"
	"fmt"
	"time"

	"github.com/ddd-qce/core/domain/event"
	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
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

func RegisterHandlers(group *eventmemory.EventBusGroup) {
	eventmemory.EventGroupBus[*UserCreatedEvent](group).Subscribe(&UserCreatedEventHandler{})
	eventmemory.EventGroupBus[*UserUpdatedEvent](group).Subscribe(&UserUpdatedEventHandler{})
	eventmemory.EventGroupBus[*OrderCancelledEvent](group).Subscribe(&OrderCancelledEventHandler{})
}

func RunExample(ctx context.Context, group *eventmemory.EventBusGroup) {
	fmt.Println("=== Event: UserCreated ===")
	err := eventmemory.EventGroupPublish[*UserCreatedEvent](group, ctx, &UserCreatedEvent{
		UserID: "user-001",
		Name:   "李四",
		Time:   time.Now(),
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("\n=== Event: UserUpdated ===")
	err = eventmemory.EventGroupPublish[*UserUpdatedEvent](group, ctx, &UserUpdatedEvent{
		UserID: "1",
		Name:   "张三更新",
		Time:   time.Now(),
	})
	if err != nil {
		panic(err)
	}

	fmt.Println("\n=== Event: OrderCancelled ===")
	err = eventmemory.EventGroupPublish[*OrderCancelledEvent](group, ctx, &OrderCancelledEvent{
		OrderID: "ORD-001",
		Reason:  "用户取消",
		Time:    time.Now(),
	})
	if err != nil {
		panic(err)
	}
}

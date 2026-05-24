package event

import (
	"context"

	"github.com/ddd-qce/core/domain/event"
)

type EventBus interface {
	SubscribeHandler(handler any) error
	Publish(ctx context.Context, evt event.DomainEvent) error
}

func Dispatch[T event.DomainEvent](ctx context.Context, bus EventBus, evt T) error {
	return bus.Publish(ctx, evt)
}

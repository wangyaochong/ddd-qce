package event

import "context"

type EventBus interface {
	SubscribeHandler(handler any) error
	Publish(ctx context.Context, evt DomainEvent) error
}

func Dispatch[T DomainEvent](ctx context.Context, bus EventBus, evt T) error {
	return bus.Publish(ctx, evt)
}
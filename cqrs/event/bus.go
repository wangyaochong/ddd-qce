package event

import "context"

type EventBus interface {
	SubscribeHandler(handler any) error
	Publish(ctx context.Context, evt Event) error
}

func Dispatch[T Event](ctx context.Context, bus EventBus, evt T) error {
	return bus.Publish(ctx, evt)
}
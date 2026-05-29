package event

import (
	"context"

	domainevent "github.com/ddd-qce/core/domain/event"
)

type EventBus interface {
	SubscribeHandler(handler any) error
	Publish(ctx context.Context, evt domainevent.Event) error
	SubscribedTypes() []string
}

type Shutdownable interface {
	Shutdown(ctx context.Context) error
}

func Dispatch[T domainevent.Event](ctx context.Context, bus EventBus, evt T) error {
	return bus.Publish(ctx, evt)
}
package event

import (
	"context"

	domainevent "github.com/ddd-qce/core/domain/event"
)

// EventBus dispatches domain events to registered handlers.
// Implementations should invoke all matching handlers for a published event.
type EventBus interface {
	SubscribeHandler(handler any) error
	Publish(ctx context.Context, evt domainevent.Event) error
	SubscribedTypes() []string
	Shutdown(ctx context.Context) error
}

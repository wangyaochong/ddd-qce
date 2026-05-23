package event

import (
	"context"

	"github.com/ddd-qce/core/domain/event"
)

type EventBus[T event.DomainEvent] interface {
	Subscribe(handler event.EventHandler[T])
	Publish(ctx context.Context, evt T) error
}

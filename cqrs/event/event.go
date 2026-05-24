package event

import (
	"context"

	"github.com/ddd-qce/core/domain/event"
)

type EventBus interface {
	SubscribeHandler(handler any) error
	Publish(ctx context.Context, evt event.DomainEvent) error
}

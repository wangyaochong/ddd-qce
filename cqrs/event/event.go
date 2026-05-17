package event

import (
	"context"

	"github.com/ddd-qce/core/domain/event"
)

type Handler[T event.DomainEvent] interface {
	Handle(ctx context.Context, event T) error
}

type Store[T event.DomainEvent] interface {
	Append(ctx context.Context, events []T) error
	Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error)
}

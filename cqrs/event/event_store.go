package event

import (
	"context"

	"github.com/ddd-qce/core/domain/event"
)

type EventStore[T event.DomainEvent] interface {
	Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error
	Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error)
}

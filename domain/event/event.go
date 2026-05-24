package event

import (
	"context"
	"reflect"
	"time"
)

func EventTypeOf(event DomainEvent) string {
	t := reflect.TypeOf(event)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

type DomainEvent interface {
	AggregateID() string
	EventType() string
	OccurredAt() time.Time
}

type EventHandler[T DomainEvent] interface {
	Handle(ctx context.Context, event T) error
}

type DomainEventAppendOnlyStore interface {
	Append(ctx context.Context, aggregateID string, expectedVersion int, events []DomainEvent) error
	Load(ctx context.Context, aggregateID string, afterVersion int) ([]DomainEvent, error)
}

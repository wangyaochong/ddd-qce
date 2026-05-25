package event

import (
	"context"
	"reflect"
	"time"
)

type BaseEvent struct {
	aggregateID string
	occurredAt  time.Time
}

func NewBaseEvent(aggregateID string, occurredAt time.Time) BaseEvent {
	return BaseEvent{aggregateID: aggregateID, occurredAt: occurredAt}
}

func (e BaseEvent) AggregateID() string   { return e.aggregateID }
func (e BaseEvent) OccurredAt() time.Time { return e.occurredAt }

func (e *BaseEvent) SetBaseEvent(aggregateID string, occurredAt time.Time) {
	e.aggregateID = aggregateID
	e.occurredAt = occurredAt
}

func EventTypeOf(event any) string {
	t := reflect.TypeOf(event)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

type DomainEvent interface {
	AggregateID() string
	OccurredAt() time.Time
}

type EventHandler[T DomainEvent] interface {
	Handle(ctx context.Context, event T) error
}

type EventSourceStore[T DomainEvent] interface {
	Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error
	Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error)
}
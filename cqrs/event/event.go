package event

import (
	"context"
	"reflect"
	"time"

	domainevent "github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/trace"
)

type BaseEvent struct {
	aggregateID   string
	occurredAt    time.Time
	correlationID string
	causationID   string
}

func NewBaseEvent(aggregateID string, occurredAt time.Time) BaseEvent {
	return BaseEvent{aggregateID: aggregateID, occurredAt: occurredAt}
}

func NewDomainEvent(aggregateID string) BaseEvent {
	return BaseEvent{aggregateID: aggregateID, occurredAt: time.Now()}
}

func NewDomainEventWithCorrelation(aggregateID, correlationID, causationID string) BaseEvent {
	return BaseEvent{
		aggregateID:   aggregateID,
		occurredAt:    time.Now(),
		correlationID: correlationID,
		causationID:   causationID,
	}
}

func WithCorrelation(ctx context.Context, aggregateID string) BaseEvent {
	correlationID := trace.GetTraceID(ctx)
	causationID := trace.GetSpanID(ctx)
	return BaseEvent{
		aggregateID:   aggregateID,
		occurredAt:    time.Now(),
		correlationID: correlationID,
		causationID:   causationID,
	}
}

func (e BaseEvent) AggregateID() string     { return e.aggregateID }
func (e BaseEvent) OccurredAt() time.Time   { return e.occurredAt }
func (e BaseEvent) CorrelationID() string   { return e.correlationID }
func (e BaseEvent) CausationID() string     { return e.causationID }

func (e *BaseEvent) SetCorrelation(correlationID, causationID string) {
	e.correlationID = correlationID
	e.causationID = causationID
}

func (e *BaseEvent) Restore(aggregateID string, occurredAt time.Time, correlationID, causationID string) {
	e.aggregateID = aggregateID
	e.occurredAt = occurredAt
	e.correlationID = correlationID
	e.causationID = causationID
}

func ApplyCorrelation(evt domainevent.Event, correlationID, causationID string) {
	if setter, ok := evt.(domainevent.CorrelationSetter); ok {
		setter.SetCorrelation(correlationID, causationID)
	}
}

func RestoreBaseEvent(evt domainevent.Event, aggregateID string, occurredAt time.Time, correlationID, causationID string) {
	if restorer, ok := evt.(domainevent.Restorer); ok {
		restorer.Restore(aggregateID, occurredAt, correlationID, causationID)
	}
}

// EventTypeOf returns the short type name of the given event.
// Panics if event is nil — nil is a programming error that should be
// caught early rather than silently producing an empty string.
// This is consistent with CommandNameOf and QueryNameOf: all three
// treat nil as misuse, not a valid input.
// Note: a typed nil pointer like (*MyEvent)(nil) is NOT nil (the
// interface has a type), so EventTypeOf((*MyEvent)(nil)) == "MyEvent".
func EventTypeOf(event any) string {
	if event == nil {
		panic("event: EventTypeOf called with nil event")
	}
	t := reflect.TypeOf(event)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

type EventHandler[T domainevent.Event] interface {
	Handle(ctx context.Context, event T) error
}

type GlobalEvent[T domainevent.Event] struct {
	Position int64
	Event    T
}

type EventSourceStore[T domainevent.Event] interface {
	Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error
	Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error)
	LoadAll(ctx context.Context, afterPosition int64, limit int) ([]GlobalEvent[T], error)
}

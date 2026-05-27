package event

import (
	"context"
	"reflect"
	"time"

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

func (e *BaseEvent) restore(aggregateID string, occurredAt time.Time, correlationID, causationID string) {
	e.aggregateID = aggregateID
	e.occurredAt = occurredAt
	e.correlationID = correlationID
	e.causationID = causationID
}

func (e *BaseEvent) setCorrelation(correlationID, causationID string) {
	e.correlationID = correlationID
	e.causationID = causationID
}

func ApplyCorrelation(evt Event, correlationID, causationID string) {
	v := reflect.ValueOf(evt)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	field := v.FieldByName("BaseEvent")
	if !field.IsValid() || field.Kind() != reflect.Struct {
		return
	}
	base, ok := field.Addr().Interface().(*BaseEvent)
	if ok {
		base.setCorrelation(correlationID, causationID)
	}
}

func RestoreBaseEvent(evt Event, aggregateID string, occurredAt time.Time, correlationID, causationID string) {
	v := reflect.ValueOf(evt)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return
	}
	field := v.FieldByName("BaseEvent")
	if !field.IsValid() || field.Kind() != reflect.Struct {
		return
	}
	base, ok := field.Addr().Interface().(*BaseEvent)
	if ok {
		base.restore(aggregateID, occurredAt, correlationID, causationID)
	}
}

func EventTypeOf(event any) string {
	if event == nil {
		return ""
	}
	t := reflect.TypeOf(event)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

type Event interface {
	AggregateID() string
	OccurredAt() time.Time
	CorrelationID() string
	CausationID() string
}

type EventHandler[T Event] interface {
	Handle(ctx context.Context, event T) error
}

type GlobalEvent[T Event] struct {
	Position int64
	Event    T
}

type EventSourceStore[T Event] interface {
	Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error
	Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error)
	LoadAll(ctx context.Context, afterPosition int64, limit int) ([]GlobalEvent[T], error)
}
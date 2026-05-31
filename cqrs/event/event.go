package event

import (
	"context"
	"reflect"
	"time"
	"unsafe"

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

func (e BaseEvent) AggregateID() string   { return e.aggregateID }
func (e BaseEvent) OccurredAt() time.Time { return e.occurredAt }
func (e BaseEvent) CorrelationID() string { return e.correlationID }
func (e BaseEvent) CausationID() string   { return e.causationID }

func ApplyCorrelation(evt domainevent.Event, correlationID, causationID string) {
	setBaseEventField(evt, "correlationID", correlationID)
	setBaseEventField(evt, "causationID", causationID)
}

func RestoreBaseEvent(evt domainevent.Event, aggregateID string, occurredAt time.Time, correlationID, causationID string) {
	setBaseEventField(evt, "aggregateID", aggregateID)
	setBaseEventField(evt, "occurredAt", occurredAt)
	setBaseEventField(evt, "correlationID", correlationID)
	setBaseEventField(evt, "causationID", causationID)
}

func setBaseEventField(evt any, fieldName string, value any) {
	v := reflect.ValueOf(evt)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return
	}
	v = v.Elem()
	f := v.FieldByName("BaseEvent")
	if !f.IsValid() {
		return
	}
	if f.Kind() == reflect.Ptr {
		if f.IsNil() {
			return
		}
		f = f.Elem()
	}
	field := f.FieldByName(fieldName)
	if !field.IsValid() {
		return
	}
	fieldPtr := unsafe.Pointer(field.UnsafeAddr())
	writableField := reflect.NewAt(field.Type(), fieldPtr).Elem()
	writableField.Set(reflect.ValueOf(value))
}

func EventNameOf(event any) string {
	if event == nil {
		panic("event: EventNameOf called with nil event")
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

type AggregateEventStore[T domainevent.Event] interface {
	Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error
	Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error)
}

type GlobalEventStore[T domainevent.Event] interface {
	LoadAll(ctx context.Context, afterPosition int64, limit int) ([]GlobalEvent[T], error)
}

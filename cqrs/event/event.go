package event

import (
	"context"
	"reflect"
	"time"

	domainevent "github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/trace"
)

// BaseEvent provides common metadata fields for all domain events.
// Embed this in concrete event types to carry aggregate identity,
// timestamps, and distributed tracing correlation.
type BaseEvent struct {
	AggregateID   string    `json:"aggregateId"`
	OccurredAt    time.Time `json:"occurredAt"`
	CorrelationID string    `json:"correlationId,omitempty"`
	CausationID   string    `json:"causationId,omitempty"`
}

func (e BaseEvent) Metadata() any { return e }

// NewBaseEvent creates a BaseEvent with the given aggregate ID and timestamp.
func NewBaseEvent(aggregateID string, occurredAt time.Time) BaseEvent {
	return BaseEvent{AggregateID: aggregateID, OccurredAt: occurredAt}
}

// NewDomainEvent creates a BaseEvent with the given aggregate ID and the current time.
func NewDomainEvent(aggregateID string) BaseEvent {
	return BaseEvent{AggregateID: aggregateID, OccurredAt: time.Now()}
}

// NewDomainEventWithCorrelation creates a BaseEvent with explicit correlation
// and causation IDs for distributed tracing.
func NewDomainEventWithCorrelation(aggregateID, correlationID, causationID string) BaseEvent {
	return BaseEvent{
		AggregateID:   aggregateID,
		OccurredAt:    time.Now(),
		CorrelationID: correlationID,
		CausationID:   causationID,
	}
}

// WithCorrelation creates a BaseEvent with correlation and causation IDs
// extracted from the current request's trace context.
func WithCorrelation(ctx context.Context, aggregateID string) BaseEvent {
	correlationID := trace.GetTraceID(ctx)
	causationID := trace.GetSpanID(ctx)
	return BaseEvent{
		AggregateID:   aggregateID,
		OccurredAt:    time.Now(),
		CorrelationID: correlationID,
		CausationID:   causationID,
	}
}

// EventNameOf returns the short type name of the given event.
// Panics if event is nil — see QueryNameOf in the query package for details.
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

// MetadataOf returns the BaseEvent metadata for an event value.
// Generic framework code uses this to read AggregateID/OccurredAt/etc.
// without depending on user-side method names.
//
// If the value does not implement the metadata interface (e.g., a bare
// custom event type that does not embed BaseEvent), the returned BaseEvent
// is the zero value.
func MetadataOf(value any) BaseEvent {
	if m, ok := value.(domainevent.Event); ok {
		if be, ok := m.Metadata().(BaseEvent); ok {
			return be
		}
	}
	return BaseEvent{}
}

// EventHandler processes an event of type T.
type EventHandler[T any] interface {
	Handle(ctx context.Context, event T) error
}

// GlobalEvent wraps an event with its position in the global event stream,
// enabling sequential replay and projection building.
type GlobalEvent[T any] struct {
	Position int64
	Event    T
}

// AggregateEventStore persists and loads events for a specific aggregate type T,
// supporting optimistic concurrency via expectedVersion.
type AggregateEventStore[T any] interface {
	Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error
	Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error)
}

// GlobalEventStore provides sequential access to all persisted events across
// aggregates, used for projections and event replay.
type GlobalEventStore[T any] interface {
	LoadAll(ctx context.Context, afterPosition int64, limit int) ([]GlobalEvent[T], error)
}

var _ domainevent.Event = BaseEvent{}

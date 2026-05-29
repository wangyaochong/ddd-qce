package event

import "time"

type Event interface {
	AggregateID() string
	OccurredAt() time.Time
	CorrelationID() string
	CausationID() string
}

type CorrelationSetter interface {
	SetCorrelation(correlationID, causationID string)
}

type Restorer interface {
	Restore(aggregateID string, occurredAt time.Time, correlationID, causationID string)
}

func FromSlice[E Event](events []E) []Event {
	result := make([]Event, len(events))
	for i, e := range events {
		result[i] = e
	}
	return result
}

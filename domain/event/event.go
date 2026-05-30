package event

import "time"

type Event interface {
	AggregateID() string
	OccurredAt() time.Time
	CorrelationID() string
	CausationID() string
}

func FromSlice[E Event](events []E) []Event {
	result := make([]Event, len(events))
	for i, e := range events {
		result[i] = e
	}
	return result
}

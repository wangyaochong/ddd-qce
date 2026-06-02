// Package event is a minimal stub of github.com/ddd-qce/core/cqrs/event
// used only by the eventimmutable analyzer's test data. The real package
// depends on github.com/ddd-qce/core/trace and github.com/ddd-qce/core/domain/event
// which are intentionally not duplicated here to keep the testdata isolated.
package event

type BaseEvent struct {
	AggregateID   string
	OccurredAt    any
	CorrelationID string
	CausationID   string
}

func (e BaseEvent) Metadata() any { return e }

func NewDomainEvent(aggregateID string) BaseEvent {
	return BaseEvent{AggregateID: aggregateID}
}

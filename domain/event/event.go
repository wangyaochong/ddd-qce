package event

// Event is a marker interface for domain events.
//
// All domain event types should embed event.BaseEvent from
// github.com/ddd-qce/core/cqrs/event, which provides the metadata fields
// (AggregateID, OccurredAt, CorrelationID, CausationID) via field access:
//
//	evt := OrderPlacedEvent{BaseEvent: event.NewDomainEvent("order-1")}
//	id := evt.AggregateID   // field access
//	t  := evt.OccurredAt    // field access
//
// Generic code (event store, bus) accesses metadata through the Metadata()
// method, which is promoted from the embedded BaseEvent. User event types
// automatically satisfy this interface by embedding BaseEvent.
//
// Metadata() returns the embedded BaseEvent value. Framework code uses
// event.MetadataOf(evt) for safe access with type assertion.
type Event interface {
	Metadata() any
}

// FromSlice converts a typed slice of events to a slice of the Event interface.
// Useful when passing domain-specific event slices to framework code.
func FromSlice[E Event](events []E) []Event {
	result := make([]Event, len(events))
	for i, e := range events {
		result[i] = e
	}
	return result
}

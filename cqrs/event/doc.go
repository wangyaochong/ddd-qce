// Package event provides CQRS event infrastructure: BaseEvent for embedding
// in domain events, EventBus for fan-out dispatch, and AggregateEventStore/
// GlobalEventStore for event persistence.
//
// Domain events implement the Event interface (domain/event package) by
// embedding BaseEvent. Events are published to zero or more subscribers.
package event

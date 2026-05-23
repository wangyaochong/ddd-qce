package aggregate

import (
	"fmt"

	"github.com/ddd-qce/core/domain/event"
)

type EventApplier interface {
	When(evt event.DomainEvent)
}

type AggregateRoot struct {
	ID                string
	Version           int
	uncommittedEvents []event.DomainEvent
	applier           EventApplier
	skipApplierCheck  bool
}

// NewAggregateRoot creates an AggregateRoot without an applier.
// Deprecated: use NewAggregateRootWithApplier(id, self) for aggregates that apply events,
// or NewEventCollector(id) for pure event collection without state mutation.
func NewAggregateRoot(id string) *AggregateRoot {
	return &AggregateRoot{
		ID:      id,
		Version: 0,
	}
}

// NewAggregateRootWithApplier creates an AggregateRoot with an EventApplier.
// This is the recommended constructor for aggregates that need to mutate state
// when events are applied. Typically called as:
//
//	type Order struct { AggregateRoot }
//	func NewOrder(id string) *Order {
//	    o := &Order{}
//	    o.AggregateRoot = *NewAggregateRootWithApplier(id, o)
//	    return o
//	}
func NewAggregateRootWithApplier(id string, applier EventApplier) *AggregateRoot {
	return &AggregateRoot{
		ID:      id,
		Version: 0,
		applier: applier,
	}
}

// NewEventCollector creates an AggregateRoot that only collects events without
// applying state mutations. Use this when you only need event sourcing metadata
// (ID, Version, uncommitted events) but no When callback.
func NewEventCollector(id string) *AggregateRoot {
	return &AggregateRoot{
		ID:               id,
		Version:          0,
		skipApplierCheck: true,
	}
}

func (a *AggregateRoot) SetApplier(applier EventApplier) {
	a.applier = applier
}

func (a *AggregateRoot) Apply(evt event.DomainEvent) {
	a.uncommittedEvents = append(a.uncommittedEvents, evt)
	a.applyEvent(evt)
}

func (a *AggregateRoot) UncommittedEvents() []event.DomainEvent {
	evts := make([]event.DomainEvent, len(a.uncommittedEvents))
	copy(evts, a.uncommittedEvents)
	return evts
}

func (a *AggregateRoot) MarkEventsAsCommitted() {
	a.uncommittedEvents = nil
}

func (a *AggregateRoot) LoadFromHistory(events []event.DomainEvent) {
	for _, evt := range events {
		a.applyEvent(evt)
	}
}

func (a *AggregateRoot) applyEvent(evt event.DomainEvent) {
	a.Version++
	if a.applier != nil {
		a.applier.When(evt)
	} else if !a.skipApplierCheck {
		panic("AggregateRoot: applier not set, use NewAggregateRootWithApplier(id, self) or NewEventCollector(id)")
	}
}

type AggregateRootValidator interface {
	Validate() error
}

func (a *AggregateRoot) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("aggregate ID cannot be empty")
	}
	if a.Version < 0 {
		return fmt.Errorf("aggregate version cannot be negative")
	}
	return nil
}

package aggregate

import (
	"fmt"

	"github.com/ddd-qce/core/domain/entity"
	"github.com/ddd-qce/core/domain/event"
)

type EventApplier interface {
	When(evt event.DomainEvent)
}

type AggregateRef interface {
	GetAggregateRoot() *AggregateRoot
}

type AggregateRoot struct {
	entity.Entity
	version           int
	uncommittedEvents []event.DomainEvent
	applier           EventApplier
	skipApplierCheck  bool
}

// NewAggregateRoot creates an AggregateRoot without an applier.
// Deprecated: use NewAggregateRootWithApplier(id, self) for aggregates that apply events,
// or NewEventCollector(id) for pure event collection without state mutation.
func NewAggregateRoot(id string) *AggregateRoot {
	return &AggregateRoot{
		Entity: *entity.NewEntity(id),
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
		Entity:  *entity.NewEntity(id),
		applier: applier,
	}
}

// NewEventCollector creates an AggregateRoot that only collects events without
// applying state mutations. Use this when you only need event sourcing metadata
// (ID, Version, uncommitted events) but no When callback.
func NewEventCollector(id string) *AggregateRoot {
	return &AggregateRoot{
		Entity:           *entity.NewEntity(id),
		skipApplierCheck: true,
	}
}

func (a *AggregateRoot) GetAggregateRoot() *AggregateRoot {
	return a
}

func (a *AggregateRoot) GetVersion() int {
	return a.version
}

// SetSnapshotVersion sets the aggregate version from a loaded snapshot.
// This should only be called by infrastructure (repository) when restoring from persistence.
func (a *AggregateRoot) SetSnapshotVersion(v int) {
	a.version = v
}

func (a *AggregateRoot) forceSetVersion(v int) {
	a.version = v
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
	a.version++
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
	if err := a.Entity.Validate(); err != nil {
		return fmt.Errorf("aggregate: %w", err)
	}
	if a.version < 0 {
		return fmt.Errorf("aggregate version cannot be negative")
	}
	return nil
}

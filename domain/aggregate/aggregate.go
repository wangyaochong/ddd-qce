package aggregate

import (
	"fmt"

	"github.com/ddd-qce/core/domain/entity"
	"github.com/ddd-qce/core/cqrs/event"
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

func (a *AggregateRoot) Equals(other *AggregateRoot) bool {
	if a == nil || other == nil {
		return a == other
	}
	return a.ID() == other.ID()
}

func (a *AggregateRoot) Version() int {
	return a.version
}

// SetSnapshotVersion sets the aggregate version from a loaded snapshot.
// This should only be called by infrastructure (repository) when restoring from persistence.
func (a *AggregateRoot) SetSnapshotVersion(v int) {
	a.version = v
}



func (a *AggregateRoot) Apply(evt event.DomainEvent) error {
	if err := a.applyEvent(evt); err != nil {
		return err
	}
	a.uncommittedEvents = append(a.uncommittedEvents, evt)
	return nil
}

func (a *AggregateRoot) UncommittedEvents() []event.DomainEvent {
	evts := make([]event.DomainEvent, len(a.uncommittedEvents))
	copy(evts, a.uncommittedEvents)
	return evts
}

func (a *AggregateRoot) MarkEventsAsCommitted() {
	a.uncommittedEvents = nil
}

func (a *AggregateRoot) LoadFromHistory(events []event.DomainEvent) error {
	for _, evt := range events {
		if err := a.applyEvent(evt); err != nil {
			return err
		}
	}
	return nil
}

func (a *AggregateRoot) applyEvent(evt event.DomainEvent) error {
	if a.applier != nil {
		a.applier.When(evt)
	} else if !a.skipApplierCheck {
		return fmt.Errorf("AggregateRoot: applier not set, use NewAggregateRootWithApplier(id, self) or NewEventCollector(id)")
	}
	a.version++
	return nil
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

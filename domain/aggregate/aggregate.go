package aggregate

import (
	"context"
	"fmt"

	"github.com/ddd-qce/core/domain/entity"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/trace"
)

type EventApplier interface {
	When(evt event.Event) error
}

type AggregateRef interface {
	GetAggregateRoot() *AggregateRoot
	Clone() *AggregateRoot
}

type AggregateRoot struct {
	entity.Entity
	version           int
	uncommittedEvents []event.Event
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



func (a *AggregateRoot) Apply(ctx context.Context, evt event.Event) error {
	if evt.CorrelationID() == "" {
		event.ApplyCorrelation(evt, trace.GetTraceID(ctx), trace.GetSpanID(ctx))
	}
	if err := a.applyEvent(evt); err != nil {
		return err
	}
	a.uncommittedEvents = append(a.uncommittedEvents, evt)
	return nil
}

func (a *AggregateRoot) UncommittedEvents() []event.Event {
	evts := make([]event.Event, len(a.uncommittedEvents))
	copy(evts, a.uncommittedEvents)
	return evts
}

func (a *AggregateRoot) MarkEventsAsCommitted() {
	a.uncommittedEvents = nil
}

func (a *AggregateRoot) LoadFromHistory(events []event.Event) error {
	for _, evt := range events {
		if err := a.applyEvent(evt); err != nil {
			return err
		}
	}
	return nil
}

func (a *AggregateRoot) applyEvent(evt event.Event) error {
	if a.applier != nil {
		if err := a.applier.When(evt); err != nil {
			return fmt.Errorf("AggregateRoot: apply event %T: %w", evt, err)
		}
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

func (a *AggregateRoot) Clone() *AggregateRoot {
	if a == nil {
		return nil
	}
	clone := &AggregateRoot{
		Entity:           *a.Entity.Clone(),
		version:          a.version,
		skipApplierCheck: a.skipApplierCheck,
	}
	if a.applier != nil {
		clone.applier = a.applier
	}
	clone.uncommittedEvents = make([]event.Event, len(a.uncommittedEvents))
	copy(clone.uncommittedEvents, a.uncommittedEvents)
	return clone
}

type AggregateRootJSON struct {
	entity.EntityJSON
	Version          int  `json:"version"`
	SkipApplierCheck bool `json:"skipApplierCheck"`
}

func (a *AggregateRoot) ToJSON() AggregateRootJSON {
	return AggregateRootJSON{
		EntityJSON:       a.Entity.ToJSON(),
		Version:          a.version,
		SkipApplierCheck: a.skipApplierCheck,
	}
}

func (a *AggregateRoot) FromJSON(j AggregateRootJSON) {
	a.Entity.FromJSON(j.EntityJSON)
	a.version = j.Version
	a.skipApplierCheck = j.SkipApplierCheck
}

func (a *AggregateRoot) SetApplier(applier EventApplier) {
	a.applier = applier
}

func CloneAggregate[T AggregateRef](agg T) *AggregateRoot {
	if any(agg) == nil {
		return nil
	}
	return agg.Clone()
}

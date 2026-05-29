package aggregate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ddd-qce/core/domain/entity"
	"github.com/ddd-qce/core/domain/event"
)

type AggregateRef interface {
	GetAggregateRoot() *AggregateRoot
	When(evt event.Event) error
}

type AggregateRoot struct {
	entity.Entity
	version           int
	snapshotVersion   int
	uncommittedEvents []event.Event
}

func NewAggregateRoot(id string) (*AggregateRoot, error) {
	e, err := entity.NewEntity(id)
	if err != nil {
		return nil, err
	}
	return &AggregateRoot{
		Entity:          *e,
		snapshotVersion: -1,
	}, nil
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

func (a *AggregateRoot) ExpectedVersion() int {
	return a.version - len(a.uncommittedEvents)
}

func (a *AggregateRoot) SetSnapshotVersion(v int) {
	a.version = v
	a.snapshotVersion = v
}

func (a *AggregateRoot) SnapshotVersion() int {
	return a.snapshotVersion
}

func ApplyChange[T AggregateRef](agg T, ctx context.Context, evt event.Event) error {
	root := agg.GetAggregateRoot()
	if err := agg.When(evt); err != nil {
		return fmt.Errorf("apply event %T: %w", evt, err)
	}
	root.uncommittedEvents = append(root.uncommittedEvents, evt)
	root.version++
	return nil
}

func ApplyHistory[T AggregateRef](agg T, evt event.Event) error {
	root := agg.GetAggregateRoot()
	if err := agg.When(evt); err != nil {
		return fmt.Errorf("apply event %T: %w", evt, err)
	}
	root.version++
	return nil
}

func LoadFromHistory[T AggregateRef](agg T, events []event.Event) error {
	for _, evt := range events {
		if err := ApplyHistory(agg, evt); err != nil {
			return err
		}
	}
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
		Entity:          *a.Entity.Clone(),
		version:         a.version,
		snapshotVersion: a.snapshotVersion,
	}
	clone.uncommittedEvents = make([]event.Event, len(a.uncommittedEvents))
	copy(clone.uncommittedEvents, a.uncommittedEvents)
	return clone
}

type AggregateRootJSON struct {
	entity.EntityJSON
	Version         int `json:"version"`
	SnapshotVersion int `json:"snapshotVersion"`
}

func (a *AggregateRoot) ToJSON() AggregateRootJSON {
	return AggregateRootJSON{
		EntityJSON:      a.Entity.ToJSON(),
		Version:         a.version,
		SnapshotVersion: a.snapshotVersion,
	}
}

func (a *AggregateRoot) FromJSON(j AggregateRootJSON) {
	a.Entity.FromJSON(j.EntityJSON)
	a.version = j.Version
	a.snapshotVersion = j.SnapshotVersion
}

func (a *AggregateRoot) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.ToJSON())
}

func (a *AggregateRoot) UnmarshalJSON(data []byte) error {
	var aux AggregateRootJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	a.FromJSON(aux)
	return nil
}

func CloneAggregate[T AggregateRef](agg T) *AggregateRoot {
	if any(agg) == nil {
		return nil
	}
	return agg.GetAggregateRoot().Clone()
}

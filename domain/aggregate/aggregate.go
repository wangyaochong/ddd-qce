package aggregate

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/ddd-qce/core/domain/entity"
	"github.com/ddd-qce/core/domain/event"
)

// AggregateRef is the contract that every aggregate must satisfy.
// It provides access to the embedded AggregateRoot and a When method
// that applies a domain event to mutate the aggregate's state.
type AggregateRef interface {
	// GetAggregateRoot returns the embedded AggregateRoot for framework access.
	GetAggregateRoot() *AggregateRoot
	// When applies a domain event to the aggregate, mutating its domain state.
	When(evt event.Event) error
}

// AggregateRoot is the base struct for all DDD aggregates.
// It embeds Entity and manages versioning, snapshots,
// and uncommitted domain events for event sourcing patterns.
type AggregateRoot struct {
	entity.Entity
	version           int
	snapshotVersion   int
	uncommittedEvents []event.Event
}

// NewAggregateRoot creates an AggregateRoot with the given identity.
// Returns an error if id is empty. The snapshot version is initialized to -1.
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

// Equals returns true if both aggregates have the same ID.
// Nil-safe: two nils are equal, one nil and one non-nil are not.
func (a *AggregateRoot) Equals(other *AggregateRoot) bool {
	if a == nil || other == nil {
		return a == other
	}
	return a.ID() == other.ID()
}

// Version returns the current version of the aggregate, incremented by each applied event.
func (a *AggregateRoot) Version() int {
	return a.version
}

// ExpectedVersion returns the version before any uncommitted events were applied.
// Used for optimistic concurrency checks when persisting.
func (a *AggregateRoot) ExpectedVersion() int {
	return a.version - len(a.uncommittedEvents)
}

// SetSnapshotVersion sets the current version and snapshot version to v.
// Call this after loading an aggregate from a snapshot to mark the baseline.
func (a *AggregateRoot) SetSnapshotVersion(v int) {
	a.version = v
	a.snapshotVersion = v
}

// SnapshotVersion returns the version at which the last snapshot was taken.
// Returns -1 if no snapshot has been taken.
func (a *AggregateRoot) SnapshotVersion() int {
	return a.snapshotVersion
}

// ApplyChange applies a new domain event to the aggregate.
// It calls the aggregate's When method to mutate state,
// appends the event to uncommitted events, and increments the version.
// Use this when creating new events from command handlers.
func ApplyChange[T AggregateRef](agg T, ctx context.Context, evt event.Event) error {
	root := agg.GetAggregateRoot()
	if err := agg.When(evt); err != nil {
		return fmt.Errorf("apply event %T: %w", evt, err)
	}
	root.uncommittedEvents = append(root.uncommittedEvents, evt)
	root.version++
	return nil
}

// ApplyHistory replays a single domain event onto the aggregate.
// Unlike ApplyChange, it does not append to uncommitted events.
// Use this when rehydrating an aggregate from stored events.
func ApplyHistory[T AggregateRef](agg T, evt event.Event) error {
	root := agg.GetAggregateRoot()
	if err := agg.When(evt); err != nil {
		return fmt.Errorf("apply event %T: %w", evt, err)
	}
	root.version++
	return nil
}

// LoadFromHistory replays a slice of domain events onto the aggregate using ApplyHistory.
// Use this to rehydrate an aggregate from its event stream.
func LoadFromHistory[T AggregateRef](agg T, events []event.Event) error {
	for _, evt := range events {
		if err := ApplyHistory(agg, evt); err != nil {
			return err
		}
	}
	return nil
}

// UncommittedEvents returns a copy of events applied since the last MarkEventsAsCommitted call.
func (a *AggregateRoot) UncommittedEvents() []event.Event {
	evts := make([]event.Event, len(a.uncommittedEvents))
	copy(evts, a.uncommittedEvents)
	return evts
}

// MarkEventsAsCommitted clears the uncommitted events list.
// Call this after successfully persisting events to the event store.
func (a *AggregateRoot) MarkEventsAsCommitted() {
	a.uncommittedEvents = nil
}

// Validate checks that the aggregate has a valid entity ID and non-negative version.
func (a *AggregateRoot) Validate() error {
	if err := a.Entity.Validate(); err != nil {
		return fmt.Errorf("aggregate: %w", err)
	}
	if a.version < 0 {
		return fmt.Errorf("aggregate version cannot be negative")
	}
	return nil
}

// Clone returns a deep copy of the aggregate, including its uncommitted events.
// Returns nil if the receiver is nil.
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

// AggregateRootJSON is the JSON representation of AggregateRoot, used for serialization.
type AggregateRootJSON struct {
	entity.EntityJSON
	Version         int `json:"version"`
	SnapshotVersion int `json:"snapshotVersion"`
}

// ToJSON converts the aggregate to its JSON representation struct.
func (a *AggregateRoot) ToJSON() AggregateRootJSON {
	return AggregateRootJSON{
		EntityJSON:      a.Entity.ToJSON(),
		Version:         a.version,
		SnapshotVersion: a.snapshotVersion,
	}
}

// FromJSON restores aggregate fields from the JSON representation struct.
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

// CloneAggregate returns a deep copy of the aggregate's AggregateRoot.
// Returns nil if agg is nil.
func CloneAggregate[T AggregateRef](agg T) *AggregateRoot {
	if any(agg) == nil {
		return nil
	}
	return agg.GetAggregateRoot().Clone()
}

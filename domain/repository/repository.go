package repository

import (
	"context"
	"encoding/json"

	"github.com/ddd-qce/core/domain/aggregate"
)

// Repository is the standard CRUD repository contract for aggregates.
// Implementations handle persistence and retrieval by aggregate ID.
type Repository[T any] interface {
	Save(ctx context.Context, aggregate T) error
	FindByID(ctx context.Context, id string) (T, error)
	Delete(ctx context.Context, id string) error
}

// EventSourcingRepository is the repository contract for event-sourced aggregates.
// Load rehydrates an aggregate from its event stream rather than from a snapshot.
type EventSourcingRepository[T any] interface {
	Save(ctx context.Context, aggregate T) error
	Load(ctx context.Context, id string) (T, error)
}

// SnapshotSerializer defines the contract for serializing and deserializing
// aggregate snapshots to/from byte slices for persistent storage.
type SnapshotSerializer[T aggregate.AggregateRef] interface {
	Serialize(agg T) ([]byte, error)
	Deserialize(data []byte) (T, error)
}

// JSONSerializer is a SnapshotSerializer implementation that uses encoding/json.
// It marshals and unmarshals aggregates using the standard JSON library.
type JSONSerializer[T aggregate.AggregateRef] struct{}

func (JSONSerializer[T]) Serialize(agg T) ([]byte, error) { return json.Marshal(agg) }
func (JSONSerializer[T]) Deserialize(data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	return v, err
}

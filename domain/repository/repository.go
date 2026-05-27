package repository

import (
	"context"
	"encoding/json"

	"github.com/ddd-qce/core/domain/aggregate"
)

type Repository[T any] interface {
	Save(ctx context.Context, aggregate T) error
	FindByID(ctx context.Context, id string) (T, error)
	Delete(ctx context.Context, id string) error
}

type EventSourcingRepository[T any] interface {
	Save(ctx context.Context, aggregate T) error
	Load(ctx context.Context, id string) (T, error)
}

type SnapshotSerializer[T aggregate.AggregateRef] interface {
	Serialize(agg T) ([]byte, error)
	Deserialize(data []byte) (T, error)
}

type JSONSerializer[T aggregate.AggregateRef] struct{}

func (JSONSerializer[T]) Serialize(agg T) ([]byte, error) { return json.Marshal(agg) }
func (JSONSerializer[T]) Deserialize(data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	return v, err
}

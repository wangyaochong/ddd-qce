package repository

import "context"

type Repository[T any] interface {
	Save(ctx context.Context, aggregate T) error
	FindByID(ctx context.Context, id string) (T, error)
	Delete(ctx context.Context, id string) error
}

type EventSourcingRepository[T any] interface {
	Save(ctx context.Context, aggregate T) error
	Load(ctx context.Context, id string) (T, error)
}

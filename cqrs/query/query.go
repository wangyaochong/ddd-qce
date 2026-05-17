package query

import "context"

type QueryHandler[T any, R any] interface {
	Handle(ctx context.Context, query T) (R, error)
}

type QueryBus interface {
	Ask(ctx context.Context, query any) (any, error)
}

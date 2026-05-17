package command

import "context"

type CommandHandler[T any, R any] interface {
	Handle(ctx context.Context, cmd T) (R, error)
}

type CommandBus interface {
	Execute(ctx context.Context, cmd any) (any, error)
}

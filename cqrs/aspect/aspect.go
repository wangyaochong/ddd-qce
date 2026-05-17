package aspect

import (
	"context"
	"time"
)

type Aspect interface {
	Name() string
	Order() int
}

type CommandAspect interface {
	Aspect
	BeforeCommand(ctx context.Context, cmd any) (context.Context, error)
	AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error
}

type QueryAspect interface {
	Aspect
	BeforeQuery(ctx context.Context, query any) (context.Context, error)
	AfterQuery(ctx context.Context, query any, result any, err error, duration time.Duration) error
}

type EventAspect interface {
	Aspect
	BeforePublish(ctx context.Context, event any) (context.Context, error)
	AfterPublish(ctx context.Context, event any, err error, duration time.Duration) error
}

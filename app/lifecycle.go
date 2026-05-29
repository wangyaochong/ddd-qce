package app

import "context"

type Lifecycle interface {
	Shutdown(ctx context.Context) error
}

type LifecycleFunc func(ctx context.Context) error

func (f LifecycleFunc) Shutdown(ctx context.Context) error { return f(ctx) }

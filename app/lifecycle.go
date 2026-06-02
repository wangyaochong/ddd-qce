package app

import "context"

// Lifecycle represents a component that needs graceful shutdown.
type Lifecycle interface {
	Shutdown(ctx context.Context) error
}

// LifecycleFunc adapts a plain function into a Lifecycle.
type LifecycleFunc func(ctx context.Context) error

func (f LifecycleFunc) Shutdown(ctx context.Context) error { return f(ctx) }

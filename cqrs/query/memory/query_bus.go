package memory

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/query"
)

type QueryBus struct {
	handlers map[reflect.Type]any
	chain    *aspect.AspectChain
	mu       sync.RWMutex
}

type QueryBusOption func(*QueryBus)

func WithQueryBusAspectChain(chain *aspect.AspectChain) QueryBusOption {
	return func(b *QueryBus) { b.chain = chain }
}

func NewQueryBus(opts ...QueryBusOption) *QueryBus {
	b := &QueryBus{
		handlers: make(map[reflect.Type]any),
		chain:    aspect.NewAspectChain(),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func RegisterQuery[T query.Query, R any](bus *QueryBus, handler query.QueryHandler[T, R]) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	var zero T
	queryType := reflect.TypeOf(zero)
	if _, exists := bus.handlers[queryType]; exists {
		panic(fmt.Sprintf("handler already registered for query type: %s", queryType))
	}
	bus.handlers[queryType] = handler
}

func Dispatch[T query.Query, R any](ctx context.Context, bus *QueryBus, q T) (R, error) {
	queryType := reflect.TypeOf(q)

	bus.mu.RLock()
	h, exists := bus.handlers[queryType]
	bus.mu.RUnlock()

	if !exists {
		var zero R
		return zero, fmt.Errorf("no handler registered for query type: %s", queryType)
	}

	handler, ok := h.(query.QueryHandler[T, R])
	if !ok {
		var zero R
		return zero, fmt.Errorf("handler type mismatch for query type: %s", queryType)
	}
	result, err := bus.chain.ExecuteWithQueryAspects(ctx, q, func(ctx context.Context) (any, error) {
		return handler.Handle(ctx, q)
	})
	if err != nil {
		var zero R
		return zero, err
	}
	typedResult, ok := result.(R)
	if !ok {
		var zero R
		return zero, fmt.Errorf("result type mismatch for query type: %s", queryType)
	}
	return typedResult, nil
}

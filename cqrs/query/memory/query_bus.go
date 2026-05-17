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

func NewQueryBus(chain *aspect.AspectChain) *QueryBus {
	if chain == nil {
		chain = aspect.NewAspectChain()
	}
	return &QueryBus{
		handlers: make(map[reflect.Type]any),
		chain:    chain,
	}
}

func RegisterQuery[T any, R any](bus *QueryBus, handler query.QueryHandler[T, R]) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	var zero T
	queryType := reflect.TypeOf(zero)
	if _, exists := bus.handlers[queryType]; exists {
		panic(fmt.Sprintf("handler already registered for query type: %s", queryType))
	}
	bus.handlers[queryType] = handler
}

func Ask[T any, R any](bus *QueryBus, ctx context.Context, q T) (R, error) {
	queryType := reflect.TypeOf(q)

	bus.mu.RLock()
	h, exists := bus.handlers[queryType]
	bus.mu.RUnlock()

	if !exists {
		var zero R
		return zero, fmt.Errorf("no handler registered for query type: %s", queryType)
	}

	handler := h.(query.QueryHandler[T, R])
	result, err := bus.chain.ExecuteWithQueryAspects(ctx, q, func(ctx context.Context) (any, error) {
		return handler.Handle(ctx, q)
	})
	if err != nil {
		var zero R
		return zero, err
	}
	return result.(R), nil
}

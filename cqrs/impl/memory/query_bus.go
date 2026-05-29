package memory

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/query"
)

type QueryBus struct {
	handlers map[reflect.Type]any
	invokers map[reflect.Type]queryInvoker
	chain    *aspect.AspectChain
	mu       sync.RWMutex
	closed   atomic.Bool
	inFlight sync.WaitGroup
}

type queryInvoker func(query any, ctx context.Context) (any, error)

var _ query.QueryBus = (*QueryBus)(nil)

type QueryBusOption func(*QueryBus)

func WithQueryBusAspectChain(chain *aspect.AspectChain) QueryBusOption {
	return func(b *QueryBus) { b.chain = chain }
}

func NewQueryBus(opts ...QueryBusOption) *QueryBus {
	b := &QueryBus{
		handlers: make(map[reflect.Type]any),
		invokers: make(map[reflect.Type]queryInvoker),
		chain:    aspect.NewAspectChain(),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func RegisterQuery[T query.Query, R any](bus *QueryBus, handler query.QueryHandler[T, R]) error {
	return bus.RegisterHandler(handler)
}

func (b *QueryBus) RegisterHandler(handler any) error {
	handlerType := reflect.TypeOf(handler)
	queryType, ok := extractHandlerPayloadType(handlerType)
	if !ok {
		return fmt.Errorf("RegisterHandler: handler must implement query.QueryHandler[T], got %T", handler)
	}
	invoker, err := makeQueryInvoker(handler, handlerType)
	if err != nil {
		return fmt.Errorf("RegisterHandler: %w", err)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.handlers[queryType]; exists {
		return fmt.Errorf("handler already registered for query type %v", queryType)
	}
	b.handlers[queryType] = handler
	b.invokers[queryType] = invoker
	return nil
}

func (b *QueryBus) Execute(ctx context.Context, q any) (any, error) {
	if b.closed.Load() {
		return nil, ErrBusClosed
	}
	b.inFlight.Add(1)
	defer b.inFlight.Done()

	queryType := reflect.TypeOf(q)

	b.mu.RLock()
	inv, exists := b.invokers[queryType]
	b.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no handler registered for query type: %s", queryType)
	}

	return b.chain.ExecuteWithQueryAspects(ctx, q, func(ctx context.Context) (any, error) {
		return inv(q, ctx)
	})
}

func makeQueryInvoker(handler any, handlerType reflect.Type) (queryInvoker, error) {
	handleMethod, ok := handlerType.MethodByName("Handle")
	if !ok {
		return nil, fmt.Errorf("handler %T does not have a Handle method", handler)
	}
	return func(q any, ctx context.Context) (any, error) {
		args := []reflect.Value{
			reflect.ValueOf(handler),
			reflect.ValueOf(ctx),
			reflect.ValueOf(q),
		}
		results := handleMethod.Func.Call(args)
		var err error
		if len(results) >= 2 {
			if e, ok := results[1].Interface().(error); ok {
				err = e
			}
		}
		if len(results) >= 1 {
			return results[0].Interface(), err
		}
		return nil, err
	}, nil
}

func (b *QueryBus) RegisteredTypes() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return typeNamesFromMap(b.handlers)
}

func (b *QueryBus) Shutdown(ctx context.Context) error {
	return shutdownBus(&b.closed, &b.inFlight, ctx)
}
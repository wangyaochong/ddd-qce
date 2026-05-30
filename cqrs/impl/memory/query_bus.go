package memory

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/query"
)

type QueryBus struct {
	core messageBus
}

var _ query.QueryBus = (*QueryBus)(nil)

type QueryBusOption func(*QueryBus)

func WithQueryBusAspectChain(chain *aspect.AspectChain) QueryBusOption {
	return func(b *QueryBus) { b.core.chain = chain }
}

func NewQueryBus(opts ...QueryBusOption) *QueryBus {
	b := &QueryBus{
		core: newMessageBus(aspect.NewAspectChain()),
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
	invoker, err := makeInvoker(handler, reflect.TypeOf(handler))
	if err != nil {
		return fmt.Errorf("RegisterHandler: %w", err)
	}
	return b.core.registerHandler(handler, invoker)
}

func (b *QueryBus) Execute(ctx context.Context, q any) (any, error) {
	return b.core.execute(ctx, q, "query")
}

func (b *QueryBus) RegisteredTypes() []string {
	return b.core.registeredTypes()
}

func (b *QueryBus) Shutdown(ctx context.Context) error {
	return b.core.shutdown(ctx)
}
package memory

import (
	"context"
	"fmt"
	"reflect"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
)

type CommandBus struct {
	core messageBus
}

var _ command.CommandBus = (*CommandBus)(nil)

type CommandBusOption func(*CommandBus)

func WithCommandBusAspectChain(chain *aspect.AspectChain) CommandBusOption {
	return func(b *CommandBus) { b.core.chain = chain }
}

func NewCommandBus(opts ...CommandBusOption) *CommandBus {
	b := &CommandBus{
		core: newMessageBus(aspect.NewAspectChain()),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func RegisterCommand[T command.Command, R any](bus *CommandBus, handler command.CommandHandler[T, R]) error {
	return bus.RegisterHandler(handler)
}

func (b *CommandBus) RegisterHandler(handler any) error {
	invoker, err := makeInvoker(handler, reflect.TypeOf(handler))
	if err != nil {
		return fmt.Errorf("RegisterHandler: %w", err)
	}
	return b.core.registerHandler(handler, invoker)
}

func (b *CommandBus) Execute(ctx context.Context, cmd any) (any, error) {
	return b.core.execute(ctx, cmd, "command")
}

func (b *CommandBus) RegisteredTypes() []string {
	return b.core.registeredTypes()
}

func (b *CommandBus) Shutdown(ctx context.Context) error {
	return b.core.shutdown(ctx)
}

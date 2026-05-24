package memory

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
)

type commandInvoker func(cmd any, ctx context.Context) (any, error)

type CommandBus struct {
	handlers map[reflect.Type]any
	invokers map[reflect.Type]commandInvoker
	chain    *aspect.AspectChain
	mu       sync.RWMutex
}

var _ command.CommandExecutor = (*CommandBus)(nil)

type CommandBusOption func(*CommandBus)

func WithCommandBusAspectChain(chain *aspect.AspectChain) CommandBusOption {
	return func(b *CommandBus) { b.chain = chain }
}

func NewCommandBus(opts ...CommandBusOption) *CommandBus {
	b := &CommandBus{
		handlers: make(map[reflect.Type]any),
		invokers: make(map[reflect.Type]commandInvoker),
		chain:    aspect.NewAspectChain(),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func RegisterCommand[T command.Command, R any](bus *CommandBus, handler command.CommandHandler[T, R]) error {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	var zero T
	cmdType := reflect.TypeOf(zero)
	if existing, exists := bus.handlers[cmdType]; exists {
		return fmt.Errorf("handler already registered for command type %T (existing: %T, new: %T)", zero, existing, handler)
	}
	bus.handlers[cmdType] = handler
	bus.invokers[cmdType] = func(cmd any, ctx context.Context) (any, error) {
		typedCmd, ok := cmd.(T)
		if !ok {
			var zeroR R
			return zeroR, fmt.Errorf("command type mismatch: expected %T, got %T", zero, cmd)
		}
		return handler.Handle(ctx, typedCmd)
	}
	return nil
}

func Dispatch[T command.Command, R any](ctx context.Context, bus *CommandBus, cmd T) (R, error) {
	cmdType := reflect.TypeOf(cmd)

	bus.mu.RLock()
	h, exists := bus.handlers[cmdType]
	bus.mu.RUnlock()

	if !exists {
		var zero R
		return zero, fmt.Errorf("no handler registered for command type: %s", cmdType)
	}

	handler, ok := h.(command.CommandHandler[T, R])
	if !ok {
		var zero R
		return zero, fmt.Errorf("handler type mismatch for command type: %s", cmdType)
	}
	result, err := bus.chain.ExecuteWithCommandAspects(ctx, cmd, func(ctx context.Context) (any, error) {
		return handler.Handle(ctx, cmd)
	})
	if err != nil {
		var zero R
		return zero, err
	}
	typedResult, ok := result.(R)
	if !ok {
		var zero R
		return zero, fmt.Errorf("result type mismatch for command type: %s", cmdType)
	}
	return typedResult, nil
}

func (b *CommandBus) Execute(ctx context.Context, cmd any) (any, error) {
	cmdType := reflect.TypeOf(cmd)

	b.mu.RLock()
	inv, exists := b.invokers[cmdType]
	b.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no handler registered for command type: %s", cmdType)
	}

	return b.chain.ExecuteWithCommandAspects(ctx, cmd, func(ctx context.Context) (any, error) {
		return inv(cmd, ctx)
	})
}

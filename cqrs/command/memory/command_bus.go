package memory

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
)

type CommandBus struct {
	handlers map[reflect.Type]any
	chain    *aspect.AspectChain
	mu       sync.RWMutex
}

var _ command.CommandExecutor = (*CommandBus)(nil)

func NewCommandBus(chain *aspect.AspectChain) *CommandBus {
	if chain == nil {
		chain = aspect.NewAspectChain()
	}
	return &CommandBus{
		handlers: make(map[reflect.Type]any),
		chain:    chain,
	}
}

func RegisterCommand[T command.Command, R any](bus *CommandBus, handler command.CommandHandler[T, R]) {
	bus.mu.Lock()
	defer bus.mu.Unlock()
	var zero T
	cmdType := reflect.TypeOf(zero)
	if _, exists := bus.handlers[cmdType]; exists {
		panic(fmt.Sprintf("handler already registered for command type: %s", cmdType))
	}
	bus.handlers[cmdType] = handler
}

func Dispatch[T command.Command, R any](bus *CommandBus, ctx context.Context, cmd T) (R, error) {
	cmdType := reflect.TypeOf(cmd)

	bus.mu.RLock()
	h, exists := bus.handlers[cmdType]
	bus.mu.RUnlock()

	if !exists {
		var zero R
		return zero, fmt.Errorf("no handler registered for command type: %s", cmdType)
	}

	handler := h.(command.CommandHandler[T, R])
	result, err := bus.chain.ExecuteWithCommandAspects(ctx, cmd, func(ctx context.Context) (any, error) {
		return handler.Handle(ctx, cmd)
	})
	if err != nil {
		var zero R
		return zero, err
	}
	return result.(R), nil
}

func (b *CommandBus) Execute(ctx context.Context, cmd any) (any, error) {
	cmdType := reflect.TypeOf(cmd)

	b.mu.RLock()
	h, exists := b.handlers[cmdType]
	b.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no handler registered for command type: %s", cmdType)
	}

	return b.chain.ExecuteWithCommandAspects(ctx, cmd, func(ctx context.Context) (any, error) {
		return invokeHandler(h, cmd, ctx)
	})
}

func invokeHandler(handler any, cmd any, ctx context.Context) (any, error) {
	v := reflect.ValueOf(handler)
	method := v.MethodByName("Handle")
	if !method.IsValid() {
		return nil, fmt.Errorf("handler does not have Handle method")
	}

	results := method.Call([]reflect.Value{
		reflect.ValueOf(ctx),
		reflect.ValueOf(cmd),
	})

	result := results[0].Interface()
	if !results[1].IsNil() {
		err := results[1].Interface().(error)
		return result, err
	}

	return result, nil
}

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

var _ command.CommandBus = (*CommandBus)(nil)

type CommandBusOption func(*CommandBus)

func WithCommandBusAspectChain(chain *aspect.AspectChain) CommandBusOption {
	return func(b *CommandBus) { b.chain = chain }
}

func NewCommandBus(opts ...CommandBusOption) *CommandBus {
	b := &CommandBus{
		handlers: make(map[reflect.Type]any),
		chain:    aspect.NewAspectChain(),
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
	handlerType := reflect.TypeOf(handler)
	evtType, ok := extractCommandHandlerCommandType(handlerType)
	if !ok {
		return fmt.Errorf("RegisterHandler: handler must implement command.CommandHandler[T], got %T", handler)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.handlers[evtType]; exists {
		return fmt.Errorf("handler already registered for command type %v", evtType)
	}
	b.handlers[evtType] = handler
	return nil
}

func extractCommandHandlerCommandType(handlerType reflect.Type) (reflect.Type, bool) {
	if handlerType.Kind() != reflect.Ptr {
		for i := 0; i < handlerType.NumMethod(); i++ {
			method := handlerType.Method(i)
			if method.Name != "Handle" {
				continue
			}
			return extractCommandTypeFromHandleMethod(method.Type), true
		}
		return nil, false
	}

	handleMethod, ok := handlerType.MethodByName("Handle")
	if !ok {
		return nil, false
	}
	ct := extractCommandTypeFromHandleMethod(handleMethod.Type)
	if ct == nil {
		return nil, false
	}
	return ct, true
}

func extractCommandTypeFromHandleMethod(methodType reflect.Type) reflect.Type {
	if methodType.NumIn() != 3 {
		return nil
	}
	return methodType.In(2)
}

func (b *CommandBus) Execute(ctx context.Context, cmd any) (any, error) {
	cmdType := reflect.TypeOf(cmd)

	b.mu.RLock()
	h, exists := b.handlers[cmdType]
	b.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no handler registered for command type: %s", cmdType)
	}

	handlerType := reflect.TypeOf(h)
	handleMethod, ok := handlerType.MethodByName("Handle")
	if !ok {
		return nil, fmt.Errorf("handler for %s has no Handle method", cmdType)
	}

	return b.chain.ExecuteWithCommandAspects(ctx, cmd, func(ctx context.Context) (any, error) {
		args := []reflect.Value{
			reflect.ValueOf(h),
			reflect.ValueOf(ctx),
			reflect.ValueOf(cmd),
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
	})
}

// Deprecated: Use command.Dispatch instead. memory.Dispatch will be removed in a future version.
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

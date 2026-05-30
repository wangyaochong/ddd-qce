package memory

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/ddd-qce/core/aspect"
)

type invokerFunc func(ctx context.Context, msg any) (any, error)

type messageBus struct {
	handlers map[reflect.Type]any
	invokers map[reflect.Type]invokerFunc
	chain    *aspect.AspectChain
	mu       sync.RWMutex
	closed   atomic.Bool
	inFlight sync.WaitGroup
}

func newMessageBus(chain *aspect.AspectChain) messageBus {
	return messageBus{
		handlers: make(map[reflect.Type]any),
		invokers: make(map[reflect.Type]invokerFunc),
		chain:    chain,
	}
}

func (b *messageBus) registerHandler(handler any, invoker invokerFunc) error {
	handlerType := reflect.TypeOf(handler)
	payloadType, ok := extractHandlerPayloadType(handlerType)
	if !ok {
		return fmt.Errorf("RegisterHandler: handler must implement a Handle method, got %T", handler)
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	if _, exists := b.handlers[payloadType]; exists {
		return fmt.Errorf("handler already registered for type %v", payloadType)
	}
	b.handlers[payloadType] = handler
	b.invokers[payloadType] = invoker
	return nil
}

func (b *messageBus) execute(ctx context.Context, msg any, aspectKind string) (any, error) {
	if b.closed.Load() {
		return nil, ErrBusClosed
	}
	b.inFlight.Add(1)
	defer b.inFlight.Done()

	msgType := reflect.TypeOf(msg)

	b.mu.RLock()
	inv, exists := b.invokers[msgType]
	b.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("no handler registered for type: %s", msgType)
	}

	exec := func(ctx context.Context) (any, error) {
		return inv(ctx, msg)
	}

	switch aspectKind {
	case "command":
		return b.chain.ExecuteWithCommandAspects(ctx, msg, exec)
	case "query":
		return b.chain.ExecuteWithQueryAspects(ctx, msg, exec)
	default:
		return exec(ctx)
	}
}

func (b *messageBus) registeredTypes() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return typeNamesFromMap(b.handlers)
}

func (b *messageBus) shutdown(ctx context.Context) error {
	return shutdownBus(&b.closed, &b.inFlight, ctx)
}

func makeInvoker(handler any, handlerType reflect.Type) (invokerFunc, error) {
	handleMethod, ok := handlerType.MethodByName("Handle")
	if !ok {
		return nil, fmt.Errorf("handler %T does not have a Handle method", handler)
	}
	return func(ctx context.Context, msg any) (any, error) {
		args := []reflect.Value{
			reflect.ValueOf(handler),
			reflect.ValueOf(ctx),
			reflect.ValueOf(msg),
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
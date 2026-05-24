package memory

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/domain/event"
)

func handlerTypeName(h any) string {
	t := reflect.TypeOf(h)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

type handlerEntry struct {
	handler any
	invoke  func(ctx context.Context, evt event.DomainEvent) error
	name    string
}

type EventBus struct {
	handlers map[reflect.Type][]handlerEntry
	chain    *aspect.AspectChain
	mu       sync.RWMutex
}

type EventBusOption func(*EventBus)

func WithBusAspectChain(chain *aspect.AspectChain) EventBusOption {
	return func(b *EventBus) { b.chain = chain }
}

func NewEventBus(opts ...EventBusOption) *EventBus {
	b := &EventBus{
		handlers: make(map[reflect.Type][]handlerEntry),
		chain:    aspect.NewAspectChain(),
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

func (b *EventBus) SubscribeHandler(handler any) {
	handlerType := reflect.TypeOf(handler)
	evtType, ok := extractEventHandlerEventType(handlerType)
	if !ok {
		panic(fmt.Sprintf("SubscribeHandler: handler must implement event.EventHandler[T], got %T", handler))
	}

	entry := handlerEntry{
		handler: handler,
		invoke:  makeHandlerInvoker(handler, handlerType),
		name:    handlerTypeName(handler),
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, existing := range b.handlers[evtType] {
		if reflect.ValueOf(existing.handler).Pointer() == reflect.ValueOf(handler).Pointer() {
			panic(fmt.Sprintf("handler already subscribed for event type: %s", evtType))
		}
	}
	b.handlers[evtType] = append(b.handlers[evtType], entry)
}

func (b *EventBus) Publish(ctx context.Context, evt event.DomainEvent) error {
	evtType := reflect.TypeOf(evt)
	if evtType == nil {
		return fmt.Errorf("Publish: event must be a non-nil pointer implementing DomainEvent")
	}

	b.mu.RLock()
	entries := make([]handlerEntry, len(b.handlers[evtType]))
	copy(entries, b.handlers[evtType])
	b.mu.RUnlock()

	handlerCount := len(entries)
	if handlerCount == 0 {
		return nil
	}

	if handlerCount == 1 {
		hCtx := builtin.ContextWithHandlerType(ctx, entries[0].name)
		return b.chain.ExecuteWithEventAspects(hCtx, evt, func(ctx context.Context) error {
			return entries[0].invoke(ctx, evt)
		})
	}

	errCh := make(chan error, handlerCount)
	var wg sync.WaitGroup
	for _, entry := range entries {
		e := entry
		wg.Add(1)
		go func() {
			defer wg.Done()
			hCtx := builtin.ContextWithHandlerType(ctx, e.name)
			err := b.chain.ExecuteWithEventAspects(hCtx, evt, func(ctx context.Context) error {
				return e.invoke(ctx, evt)
			})
			errCh <- err
		}()
	}
	go func() {
		wg.Wait()
		close(errCh)
	}()

	var errs []error
	for err := range errCh {
		if err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (b *EventBus) HandlerCount(evtType reflect.Type) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[evtType])
}

func RegisterHandler[T event.DomainEvent](bus *EventBus, handler event.EventHandler[T]) {
	bus.SubscribeHandler(handler)
}

func Dispatch[T event.DomainEvent](ctx context.Context, bus *EventBus, evt T) error {
	return bus.Publish(ctx, evt)
}

func extractEventHandlerEventType(handlerType reflect.Type) (reflect.Type, bool) {
	eventHandlerType := reflect.TypeOf((*event.EventHandler[event.DomainEvent])(nil)).Elem()

	if handlerType.Kind() != reflect.Ptr {
		for i := 0; i < handlerType.NumMethod(); i++ {
			method := handlerType.Method(i)
			if method.Name != "Handle" {
				continue
			}
			return extractEventTypeFromHandleMethod(method.Type), true
		}
		return nil, false
	}

	handleMethod, ok := handlerType.MethodByName("Handle")
	if !ok {
		return nil, false
	}
	_ = eventHandlerType
	et := extractEventTypeFromHandleMethod(handleMethod.Type)
	if et == nil {
		return nil, false
	}
	return et, true
}

func extractEventTypeFromHandleMethod(methodType reflect.Type) reflect.Type {
	if methodType.NumIn() != 3 {
		return nil
	}
	return methodType.In(2)
}

func makeHandlerInvoker(handler any, handlerType reflect.Type) func(ctx context.Context, evt event.DomainEvent) error {
	handleMethod, ok := handlerType.MethodByName("Handle")
	if !ok {
		panic(fmt.Sprintf("handler %T does not have a Handle method", handler))
	}
	return func(ctx context.Context, evt event.DomainEvent) error {
		args := []reflect.Value{
			reflect.ValueOf(handler),
			reflect.ValueOf(ctx),
			reflect.ValueOf(evt),
		}
		results := handleMethod.Func.Call(args)
		if len(results) > 0 {
			if err, ok := results[0].Interface().(error); ok {
				return err
			}
		}
		return nil
	}
}

package memory

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/cqrs/event"
	domainevent "github.com/ddd-qce/core/domain/event"
	ddderror "github.com/ddd-qce/core/error"
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
	invoke  func(ctx context.Context, evt domainevent.Event) error
	name    string
}

const defaultHandlerTimeout = 30 * time.Second

// EventBus dispatches events to registered handlers.
// All handlers are invoked concurrently in separate goroutines; handlers MUST be concurrency-safe.
type EventBus struct {
	handlers       map[reflect.Type][]handlerEntry
	chain          *aspect.AspectChain
	handlerTimeout time.Duration
	sem            chan struct{}
	mu             sync.RWMutex
	closed         atomic.Bool
	inFlight       sync.WaitGroup
}

var _ event.EventBus = (*EventBus)(nil)

type EventBusOption func(*EventBus)

func WithEventBusAspectChain(chain *aspect.AspectChain) EventBusOption {
	return func(b *EventBus) { b.chain = chain }
}

func WithHandlerTimeout(timeout time.Duration) EventBusOption {
	return func(b *EventBus) { b.handlerTimeout = timeout }
}

func WithConcurrencyLimit(n int) EventBusOption {
	return func(b *EventBus) { b.sem = make(chan struct{}, n) }
}

func NewEventBus(opts ...EventBusOption) *EventBus {
	b := &EventBus{
		handlers:       make(map[reflect.Type][]handlerEntry),
		chain:          aspect.NewAspectChain(),
		handlerTimeout: defaultHandlerTimeout,
	}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

// SubscribeHandler registers a handler for its inferred event type.
// The same handler instance must not be registered twice for the same event type.
// Handlers are invoked concurrently; they MUST be concurrency-safe.
func (b *EventBus) SubscribeHandler(handler any) error {
	handlerType := reflect.TypeOf(handler)
	evtType, ok := extractHandlerPayloadType(handlerType)
	if !ok {
		return fmt.Errorf("SubscribeHandler: handler must implement event.EventHandler[T], got %T", handler)
	}

	invoke, err := makeHandlerInvoker(handler, handlerType)
	if err != nil {
		return fmt.Errorf("SubscribeHandler: %w", err)
	}

	entry := handlerEntry{
		handler: handler,
		invoke:  invoke,
		name:    handlerTypeName(handler),
	}

	b.mu.Lock()
	defer b.mu.Unlock()

	for _, existing := range b.handlers[evtType] {
		if reflect.ValueOf(existing.handler).Pointer() == reflect.ValueOf(handler).Pointer() {
			return fmt.Errorf("handler already subscribed for event type: %s", evtType)
		}
	}
	b.handlers[evtType] = append(b.handlers[evtType], entry)
	return nil
}

// Publish dispatches the event to all registered handlers concurrently.
// Each handler runs in its own goroutine; the call blocks until all handlers
// complete or their per-handler timeout expires.
// A MultiError is returned when more than one handler fails.
func (b *EventBus) Publish(ctx context.Context, evt domainevent.Event) error {
	if b.closed.Load() {
		return ErrBusClosed
	}
	b.inFlight.Add(1)
	defer b.inFlight.Done()

	evtType := reflect.TypeOf(evt)
	if evtType == nil {
		return fmt.Errorf("Publish: event must be a non-nil pointer implementing Event")
	}

	b.mu.RLock()
	entries := make([]handlerEntry, len(b.handlers[evtType]))
	copy(entries, b.handlers[evtType])
	b.mu.RUnlock()

	handlerCount := len(entries)

	timeout := b.handlerTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d < timeout {
			timeout = d
		}
	}

	if handlerCount == 0 {
		return b.chain.ExecuteWithEventAspects(ctx, evt, func(ctx context.Context) error {
			return nil
		})
	}

	errCh := make(chan error, handlerCount)
	var wg sync.WaitGroup
	for _, entry := range entries {
		e := entry
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					errCh <- fmt.Errorf("handler %s panicked: %v", e.name, r)
				}
			}()
			if b.sem != nil {
				b.sem <- struct{}{}
				defer func() { <-b.sem }()
			}
			hCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			hCtx = builtin.ContextWithHandlerType(hCtx, e.name)
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
	switch len(errs) {
	case 0:
		return nil
	case 1:
		return errs[0]
	default:
		return ddderror.NewMultiError(errs...)
	}
}

func (b *EventBus) HandlerCount(evtType reflect.Type) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[evtType])
}

func (b *EventBus) SubscribedTypes() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()
	names := make([]string, 0, len(b.handlers))
	for t := range b.handlers {
		name := t.Name()
		if t.Kind() == reflect.Ptr {
			name = t.Elem().Name()
		}
		names = append(names, name)
	}
	return names
}

func (b *EventBus) Shutdown(ctx context.Context) error {
	return shutdownBus(&b.closed, &b.inFlight, ctx)
}

func RegisterHandler[T domainevent.Event](bus *EventBus, handler event.EventHandler[T]) error {
	return bus.SubscribeHandler(handler)
}

func makeHandlerInvoker(handler any, handlerType reflect.Type) (func(ctx context.Context, evt domainevent.Event) error, error) {
	handleMethod, ok := handlerType.MethodByName("Handle")
	if !ok {
		return nil, fmt.Errorf("handler %T does not have a Handle method", handler)
	}
	invoker := func(ctx context.Context, evt domainevent.Event) error {
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
	return invoker, nil
}

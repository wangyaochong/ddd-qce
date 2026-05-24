package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/domain/event"
)

type testUserEvent struct {
	aggregateID string
}

func (e *testUserEvent) AggregateID() string   { return e.aggregateID }
func (e *testUserEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *testUserEvent) OccurredAt() time.Time { return time.Now() }

type testOrderEvent struct {
	aggregateID string
}

func (e *testOrderEvent) AggregateID() string   { return e.aggregateID }
func (e *testOrderEvent) EventType() string     { return event.EventTypeOf(e) }
func (e *testOrderEvent) OccurredAt() time.Time { return time.Now() }

type testUserEventHandler struct {
	called    bool
	callCount int
	mu        sync.Mutex
}

func (h *testUserEventHandler) Handle(ctx context.Context, evt *testUserEvent) error {
	h.mu.Lock()
	h.called = true
	h.callCount++
	h.mu.Unlock()
	return nil
}

type testOrderEventHandler struct {
	called bool
}

func (h *testOrderEventHandler) Handle(ctx context.Context, evt *testOrderEvent) error {
	h.called = true
	return nil
}

type testErrorEventHandler struct {
	id int
}

func (h *testErrorEventHandler) Handle(ctx context.Context, evt *testUserEvent) error {
	return errors.New("subscriber error")
}

type testErrorEventHandlerV2 struct {
	id int
}

func (h *testErrorEventHandlerV2) Handle(ctx context.Context, evt *testUserEvent) error {
	return errors.New("subscriber error 2")
}

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	bus := NewEventBus(WithBusAspectChain(aspect.NewAspectChain()))
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEventBus_MultipleHandlers(t *testing.T) {
	bus := NewEventBus(WithBusAspectChain(aspect.NewAspectChain()))
	sub1 := &testUserEventHandler{}
	sub2 := &testUserEventHandler{}
	RegisterHandler[*testUserEvent](bus, sub1)
	RegisterHandler[*testUserEvent](bus, sub2)

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sub1.called || !sub2.called {
		t.Error("not all handlers called")
	}
}

func TestEventBus_HandlerError(t *testing.T) {
	bus := NewEventBus(WithBusAspectChain(aspect.NewAspectChain()))
	okHandler := &testUserEventHandler{}
	RegisterHandler[*testUserEvent](bus, okHandler)
	RegisterHandler[*testUserEvent](bus, &testErrorEventHandler{})

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})

	if err == nil {
		t.Fatal("expected error from handler")
	}
	if !okHandler.called {
		t.Error("okHandler should still have been called")
	}
}

func TestEventBus_Concurrent(t *testing.T) {
	bus := NewEventBus(WithBusAspectChain(aspect.NewAspectChain()))
	handler := &testUserEventHandler{}
	RegisterHandler[*testUserEvent](bus, handler)

	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if err := bus.Publish(ctx, &testUserEvent{aggregateID: string(rune(id))}); err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("concurrent error: %v", err)
	}
	if handler.callCount != 100 {
		t.Errorf("expected 100 calls, got %d", handler.callCount)
	}
}

func TestEventBus_NilChain(t *testing.T) {
	bus := NewEventBus()
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEventBus_WithAspects(t *testing.T) {
	chain := aspect.NewAspectChain()
	var beforeCalled, afterCalled bool
	chain.RegisterEventAspect(&testEventAspect{
		beforeFn: func() { beforeCalled = true },
		afterFn:  func() { afterCalled = true },
	})

	bus := NewEventBus(WithBusAspectChain(chain))
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !beforeCalled {
		t.Error("BeforePublish not called")
	}
	if !afterCalled {
		t.Error("AfterPublish not called")
	}
}

func TestEventBus_MultipleEventTypes(t *testing.T) {
	bus := NewEventBus(WithBusAspectChain(aspect.NewAspectChain()))
	userHandler := &testUserEventHandler{}
	orderHandler := &testOrderEventHandler{}

	RegisterHandler[*testUserEvent](bus, userHandler)
	RegisterHandler[*testOrderEvent](bus, orderHandler)

	ctx := context.Background()

	Dispatch[*testUserEvent](ctx, bus, &testUserEvent{aggregateID: "u1"})
	Dispatch[*testOrderEvent](ctx, bus, &testOrderEvent{aggregateID: "o1"})
	Dispatch[*testUserEvent](ctx, bus, &testUserEvent{aggregateID: "u2"})

	if userHandler.callCount != 2 {
		t.Errorf("expected userHandler called 2 times, got %d", userHandler.callCount)
	}
	if !orderHandler.called {
		t.Error("orderHandler should have been called")
	}
}

func TestEventBus_DifferentEventTypesIsolated(t *testing.T) {
	bus := NewEventBus(WithBusAspectChain(aspect.NewAspectChain()))
	handler := &testUserEventHandler{}
	RegisterHandler[*testUserEvent](bus, handler)

	ctx := context.Background()
	err := Dispatch[*testOrderEvent](ctx, bus, &testOrderEvent{aggregateID: "1"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handler.called {
		t.Error("handler should not be called for different event type")
	}
}

func TestEventBus_PublishRoutesCorrectly(t *testing.T) {
	bus := NewEventBus(WithBusAspectChain(aspect.NewAspectChain()))
	userHandler := &testUserEventHandler{}
	orderHandler := &testOrderEventHandler{}

	RegisterHandler[*testUserEvent](bus, userHandler)
	RegisterHandler[*testOrderEvent](bus, orderHandler)

	ctx := context.Background()

	err := Dispatch[*testUserEvent](ctx, bus, &testUserEvent{aggregateID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !userHandler.called {
		t.Error("userHandler was not called")
	}
	if orderHandler.called {
		t.Error("orderHandler should not have been called")
	}

	err = Dispatch[*testOrderEvent](ctx, bus, &testOrderEvent{aggregateID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !orderHandler.called {
		t.Error("orderHandler was not called")
	}
}

func TestEventBus_DuplicateSubscription_ReturnsError(t *testing.T) {
	bus := NewEventBus()
	handler := &testUserEventHandler{}
	if err := RegisterHandler[*testUserEvent](bus, handler); err != nil {
		t.Fatalf("first subscription should succeed: %v", err)
	}

	err := RegisterHandler[*testUserEvent](bus, handler)
	if err == nil {
		t.Fatal("expected error for duplicate subscription")
	}
}

func TestEventBus_NilChainGroup(t *testing.T) {
	bus := NewEventBus()
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})

	ctx := context.Background()
	err := Dispatch[*testUserEvent](ctx, bus, &testUserEvent{aggregateID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

type testEventAspect struct {
	beforeFn func()
	afterFn  func()
}

func (a *testEventAspect) Name() string { return "test" }
func (a *testEventAspect) Order() int   { return 1 }
func (a *testEventAspect) BeforePublish(ctx context.Context, evt any) (context.Context, error) {
	if a.beforeFn != nil {
		a.beforeFn()
	}
	return ctx, nil
}
func (a *testEventAspect) AfterPublish(ctx context.Context, evt any, err error, d time.Duration) error {
	if a.afterFn != nil {
		a.afterFn()
	}
	return nil
}

type multiError interface{ Unwrap() []error }

func TestEventBus_MultipleHandlerErrors(t *testing.T) {
	bus := NewEventBus(WithBusAspectChain(aspect.NewAspectChain()))
	RegisterHandler[*testUserEvent](bus, &testErrorEventHandler{})
	RegisterHandler[*testUserEvent](bus, &testErrorEventHandlerV2{})

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})

	if err == nil {
		t.Fatal("expected error from handlers")
	}

	var me multiError
	if !errors.As(err, &me) {
		t.Fatalf("expected multi-error, got %T", err)
	}
	if len(me.Unwrap()) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(me.Unwrap()))
	}
}

func TestEventBus_SingleErrorNotMultiError(t *testing.T) {
	bus := NewEventBus(WithBusAspectChain(aspect.NewAspectChain()))
	RegisterHandler[*testUserEvent](bus, &testErrorEventHandler{})

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})

	if err == nil {
		t.Fatal("expected error from handler")
	}

	var me multiError
	if errors.As(err, &me) {
		t.Error("single error should not be wrapped in multi-error")
	}
	if err.Error() != "subscriber error" {
		t.Errorf("expected 'subscriber error', got '%s'", err.Error())
	}
}

func TestEventBus_AllHandlersCalledConcurrently(t *testing.T) {
	bus := NewEventBus(WithBusAspectChain(aspect.NewAspectChain()))
	h1 := &testUserEventHandler{}
	h2 := &testUserEventHandler{}
	h3 := &testUserEventHandler{}
	RegisterHandler[*testUserEvent](bus, h1)
	RegisterHandler[*testUserEvent](bus, h2)
	RegisterHandler[*testUserEvent](bus, h3)

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !h1.called || !h2.called || !h3.called {
		t.Error("all handlers should be called")
	}
}

type testSlowEventHandler struct {
	id       int
	called   bool
	mu       sync.Mutex
	duration time.Duration
}

func (h *testSlowEventHandler) Handle(ctx context.Context, evt *testUserEvent) error {
	h.mu.Lock()
	h.called = true
	h.mu.Unlock()
	if h.duration > 0 {
		select {
		case <-time.After(h.duration):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

func TestEventBus_ConcurrentHandlersFaster(t *testing.T) {
	bus := NewEventBus(WithBusAspectChain(aspect.NewAspectChain()))
	delay := 50 * time.Millisecond
	RegisterHandler[*testUserEvent](bus, &testSlowEventHandler{duration: delay, id: 1})
	RegisterHandler[*testUserEvent](bus, &testSlowEventHandler{duration: delay, id: 2})
	RegisterHandler[*testUserEvent](bus, &testSlowEventHandler{duration: delay, id: 3})

	ctx := context.Background()
	start := time.Now()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if elapsed >= 120*time.Millisecond {
		t.Errorf("handlers should run concurrently, took %v (expected ~%v)", elapsed, delay)
	}
}

func TestEventBus_WithAspects_MultipleHandlers(t *testing.T) {
	chain := aspect.NewAspectChain()
	var mu sync.Mutex
	var beforeCount, afterCount int
	chain.RegisterEventAspect(&testEventAspect{
		beforeFn: func() { mu.Lock(); beforeCount++; mu.Unlock() },
		afterFn:  func() { mu.Lock(); afterCount++; mu.Unlock() },
	})

	bus := NewEventBus(WithBusAspectChain(chain))
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	bc, ac := beforeCount, afterCount
	mu.Unlock()

	if bc != 3 {
		t.Errorf("expected 3 BeforePublish calls, got %d", bc)
	}
	if ac != 3 {
		t.Errorf("expected 3 AfterPublish calls, got %d", ac)
	}
}

func TestEventBus_NoHandlers(t *testing.T) {
	bus := NewEventBus(WithBusAspectChain(aspect.NewAspectChain()))

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})

	if err != nil {
		t.Fatalf("expected nil error with no handlers, got %v", err)
	}
}

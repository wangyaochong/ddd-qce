package memory

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/event"
	domainevent "github.com/ddd-qce/core/domain/event"
)

type testUserEvent struct {
	event.BaseEvent
}

type testOrderEvent struct {
	event.BaseEvent
}

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
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEventBus_MultipleHandlers(t *testing.T) {
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	sub1 := &testUserEventHandler{}
	sub2 := &testUserEventHandler{}
	RegisterHandler[*testUserEvent](bus, sub1)
	RegisterHandler[*testUserEvent](bus, sub2)

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sub1.called || !sub2.called {
		t.Error("not all handlers called")
	}
}

func TestEventBus_HandlerError(t *testing.T) {
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	okHandler := &testUserEventHandler{}
	RegisterHandler[*testUserEvent](bus, okHandler)
	RegisterHandler[*testUserEvent](bus, &testErrorEventHandler{})

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})

	if err == nil {
		t.Fatal("expected error from handler")
	}
	if !okHandler.called {
		t.Error("okHandler should still have been called")
	}
}

func TestEventBus_Concurrent(t *testing.T) {
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	handler := &testUserEventHandler{}
	RegisterHandler[*testUserEvent](bus, handler)

	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			if err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent(string(rune(id)), time.Now())}); err != nil {
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
	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
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

	bus := NewEventBus(WithEventBusAspectChain(chain))
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})

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
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	userHandler := &testUserEventHandler{}
	orderHandler := &testOrderEventHandler{}

	RegisterHandler[*testUserEvent](bus, userHandler)
	RegisterHandler[*testOrderEvent](bus, orderHandler)

	ctx := context.Background()

	bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("u1", time.Now())})
	bus.Publish(ctx, &testOrderEvent{BaseEvent: event.NewBaseEvent("o1", time.Now())})
	bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("u2", time.Now())})

	if userHandler.callCount != 2 {
		t.Errorf("expected userHandler called 2 times, got %d", userHandler.callCount)
	}
	if !orderHandler.called {
		t.Error("orderHandler should have been called")
	}
}

func TestEventBus_DifferentEventTypesIsolated(t *testing.T) {
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	handler := &testUserEventHandler{}
	RegisterHandler[*testUserEvent](bus, handler)

	ctx := context.Background()
	err := bus.Publish(ctx, &testOrderEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handler.called {
		t.Error("handler should not be called for different event type")
	}
}

func TestEventBus_PublishRoutesCorrectly(t *testing.T) {
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	userHandler := &testUserEventHandler{}
	orderHandler := &testOrderEventHandler{}

	RegisterHandler[*testUserEvent](bus, userHandler)
	RegisterHandler[*testOrderEvent](bus, orderHandler)

	ctx := context.Background()

	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !userHandler.called {
		t.Error("userHandler was not called")
	}
	if orderHandler.called {
		t.Error("orderHandler should not have been called")
	}

	err = bus.Publish(ctx, &testOrderEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
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
	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
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
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	RegisterHandler[*testUserEvent](bus, &testErrorEventHandler{})
	RegisterHandler[*testUserEvent](bus, &testErrorEventHandlerV2{})

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})

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
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	RegisterHandler[*testUserEvent](bus, &testErrorEventHandler{})

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})

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
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	h1 := &testUserEventHandler{}
	h2 := &testUserEventHandler{}
	h3 := &testUserEventHandler{}
	RegisterHandler[*testUserEvent](bus, h1)
	RegisterHandler[*testUserEvent](bus, h2)
	RegisterHandler[*testUserEvent](bus, h3)

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})

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

type testSlowEventHandlerWithNotify struct {
	started  chan struct{}
	duration time.Duration
}

func (h *testSlowEventHandlerWithNotify) Handle(ctx context.Context, evt *testUserEvent) error {
	close(h.started)
	select {
	case <-time.After(h.duration):
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

func TestEventBus_ConcurrentHandlersFaster(t *testing.T) {
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	delay := 50 * time.Millisecond
	RegisterHandler[*testUserEvent](bus, &testSlowEventHandler{duration: delay, id: 1})
	RegisterHandler[*testUserEvent](bus, &testSlowEventHandler{duration: delay, id: 2})
	RegisterHandler[*testUserEvent](bus, &testSlowEventHandler{duration: delay, id: 3})

	ctx := context.Background()
	start := time.Now()
	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
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

	bus := NewEventBus(WithEventBusAspectChain(chain))
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
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
	var beforeCalled, afterCalled bool
	chain := aspect.NewAspectChain()
	chain.RegisterEventAspect(&testEventAspect{
		beforeFn: func() { beforeCalled = true },
		afterFn:  func() { afterCalled = true },
	})

	bus := NewEventBus(WithEventBusAspectChain(chain))

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})

	if err != nil {
		t.Fatalf("expected nil error with no handlers, got %v", err)
	}
	if !beforeCalled {
		t.Error("expected BeforePublish to be called even with no handlers")
	}
	if !afterCalled {
		t.Error("expected AfterPublish to be called even with no handlers")
	}
}

func TestEventBus_HandlerCount(t *testing.T) {
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	h1 := &testUserEventHandler{}
	h2 := &testUserEventHandler{}
	RegisterHandler[*testUserEvent](bus, h1)

	if count := bus.HandlerCount(reflect.TypeOf(&testUserEvent{})); count != 1 {
		t.Errorf("expected 1 handler, got %d", count)
	}

	RegisterHandler[*testUserEvent](bus, h2)

	if count := bus.HandlerCount(reflect.TypeOf(&testUserEvent{})); count != 2 {
		t.Errorf("expected 2 handlers, got %d", count)
	}

	if count := bus.HandlerCount(reflect.TypeOf(&testOrderEvent{})); count != 0 {
		t.Errorf("expected 0 handlers for unregistered type, got %d", count)
	}
}

func TestEventBus_SubscribedTypes(t *testing.T) {
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})
	RegisterHandler[*testOrderEvent](bus, &testOrderEventHandler{})

	types := bus.SubscribedTypes()
	if len(types) != 2 {
		t.Fatalf("expected 2 subscribed types, got %d", len(types))
	}

	nameSet := make(map[string]bool)
	for _, name := range types {
		nameSet[name] = true
	}
	if !nameSet["testUserEvent"] {
		t.Error("expected testUserEvent in subscribed types")
	}
	if !nameSet["testOrderEvent"] {
		t.Error("expected testOrderEvent in subscribed types")
	}
}

func TestEventBus_SubscribedTypes_Empty(t *testing.T) {
	bus := NewEventBus()
	types := bus.SubscribedTypes()
	if len(types) != 0 {
		t.Errorf("expected 0 types, got %d", len(types))
	}
}

func TestEventBus_Shutdown(t *testing.T) {
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})

	err := bus.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = bus.Publish(context.Background(), &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
	if !errors.Is(err, ErrBusClosed) {
		t.Errorf("expected ErrBusClosed after shutdown, got %v", err)
	}
}

func TestEventBus_Shutdown_WaitsForInFlight(t *testing.T) {
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	handlerStarted := make(chan struct{})
	RegisterHandler[*testUserEvent](bus, &testSlowEventHandlerWithNotify{started: handlerStarted, duration: 100 * time.Millisecond})

	done := make(chan struct{})
	go func() {
		_ = bus.Publish(context.Background(), &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
		close(done)
	}()

	<-handlerStarted

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- bus.Shutdown(context.Background())
	}()

	select {
	case <-shutdownDone:
		t.Fatal("shutdown should wait for in-flight publish")
	case <-time.After(30 * time.Millisecond):
	}

	<-done

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("unexpected shutdown error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown timed out")
	}
}

func TestEventBus_Shutdown_ContextCancelled(t *testing.T) {
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))
	handlerStarted := make(chan struct{})
	RegisterHandler[*testUserEvent](bus, &testSlowEventHandlerWithNotify{started: handlerStarted, duration: 5 * time.Second})

	go func() {
		_ = bus.Publish(context.Background(), &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
	}()

	<-handlerStarted

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := bus.Shutdown(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestEventBus_WithHandlerTimeout(t *testing.T) {
	bus := NewEventBus(
		WithEventBusAspectChain(aspect.NewAspectChain()),
		WithHandlerTimeout(50*time.Millisecond),
	)

	handler := &testSlowEventHandler{duration: 5 * time.Second}
	RegisterHandler[*testUserEvent](bus, handler)

	ctx := context.Background()
	start := time.Now()
	err := bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error from handler")
	}
	if elapsed > 200*time.Millisecond {
		t.Errorf("handler should have been canceled by timeout, took %v", elapsed)
	}
}

func TestEventBus_WithConcurrencyLimit(t *testing.T) {
	bus := NewEventBus(
		WithEventBusAspectChain(aspect.NewAspectChain()),
		WithConcurrencyLimit(1),
	)

	var mu sync.Mutex
	var maxConcurrent int
	var current int32

	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})

	bus.mu.Lock()
	bus.handlers[reflect.TypeOf(&testUserEvent{})] = []handlerEntry{{
		handler: &testUserEventHandler{},
		invoke: func(ctx context.Context, evt domainevent.Event) error {
			atomic.AddInt32(&current, 1)
			mu.Lock()
			if int(atomic.LoadInt32(&current)) > maxConcurrent {
				maxConcurrent = int(atomic.LoadInt32(&current))
			}
			mu.Unlock()
			time.Sleep(50 * time.Millisecond)
			atomic.AddInt32(&current, -1)
			return nil
		},
		name: "counting",
	}}
	bus.mu.Unlock()

	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = bus.Publish(context.Background(), &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
		}()
	}
	wg.Wait()

	mu.Lock()
	mc := maxConcurrent
	mu.Unlock()

	if mc > 1 {
		t.Errorf("expected max concurrent handlers <= 1, got %d", mc)
	}
}

type nonPointerHandler struct{}

func (h nonPointerHandler) Handle(ctx context.Context, evt *testUserEvent) error {
	return nil
}

func TestEventBus_NonPointerHandler(t *testing.T) {
	bus := NewEventBus()
	err := bus.SubscribeHandler(nonPointerHandler{})
	if err != nil {
		t.Fatalf("non-pointer handler should be accepted: %v", err)
	}

	ctx := context.Background()
	err = bus.Publish(ctx, &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEventBus_Publish_PanickingHandler(t *testing.T) {
	bus := NewEventBus(WithEventBusAspectChain(aspect.NewAspectChain()))

	RegisterHandler[*testUserEvent](bus, &testUserEventHandler{})

	bus.mu.Lock()
	bus.handlers[reflect.TypeOf(&testUserEvent{})] = []handlerEntry{{
		handler: &testUserEventHandler{},
		invoke: func(ctx context.Context, evt domainevent.Event) error {
			panic("boom")
		},
		name: "panicker",
	}}
	bus.mu.Unlock()

	err := bus.Publish(context.Background(), &testUserEvent{BaseEvent: event.NewBaseEvent("1", time.Now())})
	if err == nil {
		t.Fatal("expected error from panicking handler")
	}
	if !strings.Contains(err.Error(), "panicked") {
		t.Errorf("expected panic error message, got: %v", err)
	}
}
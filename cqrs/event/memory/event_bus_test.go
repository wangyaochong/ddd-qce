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

func (h *testUserEventHandler) Handle(ctx context.Context, event *testUserEvent) error {
	h.mu.Lock()
	h.called = true
	h.callCount++
	h.mu.Unlock()
	return nil
}

type testOrderEventHandler struct {
	called bool
}

func (h *testOrderEventHandler) Handle(ctx context.Context, event *testOrderEvent) error {
	h.called = true
	return nil
}

type testErrorEventHandler struct {
}

func (h *testErrorEventHandler) Handle(ctx context.Context, event *testUserEvent) error {
	return errors.New("subscriber error")
}

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewEventBus[*testUserEvent](chain)

	handler := &testUserEventHandler{}
	bus.Subscribe(handler)

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handler.called {
		t.Error("handler was not called")
	}
}

func TestEventBus_MultipleHandlers(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewEventBus[*testUserEvent](chain)

	sub1 := &testUserEventHandler{}
	sub2 := &testUserEventHandler{}

	bus.Subscribe(sub1)
	bus.Subscribe(sub2)

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !sub1.called {
		t.Error("sub1 was not called")
	}
	if !sub2.called {
		t.Error("sub2 was not called")
	}
}

func TestEventBus_HandlerError(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewEventBus[*testUserEvent](chain)

	okHandler := &testUserEventHandler{}
	errHandler := &testErrorEventHandler{}

	bus.Subscribe(okHandler)
	bus.Subscribe(errHandler)

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
	chain := aspect.NewAspectChain()
	bus := NewEventBus[*testUserEvent](chain)

	handler := &testUserEventHandler{}
	bus.Subscribe(handler)

	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			err := bus.Publish(ctx, &testUserEvent{aggregateID: string(rune(id))})
			if err != nil {
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
	bus := NewEventBus[*testUserEvent](nil)

	handler := &testUserEventHandler{}
	bus.Subscribe(handler)

	ctx := context.Background()
	err := bus.Publish(ctx, &testUserEvent{aggregateID: "1"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handler.called {
		t.Error("handler was not called")
	}
}

func TestEventBus_WithAspects(t *testing.T) {
	chain := aspect.NewAspectChain()

	var beforeCalled, afterCalled bool
	testAspect := &testEventAspect{
		beforeFn: func() { beforeCalled = true },
		afterFn:  func() { afterCalled = true },
	}
	chain.RegisterEventAspect(testAspect)

	bus := NewEventBus[*testUserEvent](chain)
	handler := &testUserEventHandler{}
	bus.Subscribe(handler)

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

func TestEventBusGroup_BusReturnsSameInstance(t *testing.T) {
	chain := aspect.NewAspectChain()
	group := NewEventBusGroup(chain)

	bus1 := EventGroupBus[*testUserEvent](group)
	bus2 := EventGroupBus[*testUserEvent](group)

	if bus1 != bus2 {
		t.Error("EventGroupBus[T]() should return the same instance for the same type")
	}
}

func TestEventBusGroup_PublishRoutesCorrectly(t *testing.T) {
	chain := aspect.NewAspectChain()
	group := NewEventBusGroup(chain)

	userHandler := &testUserEventHandler{}
	orderHandler := &testOrderEventHandler{}

	EventGroupBus[*testUserEvent](group).Subscribe(userHandler)
	EventGroupBus[*testOrderEvent](group).Subscribe(orderHandler)

	ctx := context.Background()

	err := EventGroupPublish[*testUserEvent](group, ctx, &testUserEvent{aggregateID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !userHandler.called {
		t.Error("userHandler was not called")
	}
	if orderHandler.called {
		t.Error("orderHandler should not have been called")
	}

	err = EventGroupPublish[*testOrderEvent](group, ctx, &testOrderEvent{aggregateID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !orderHandler.called {
		t.Error("orderHandler was not called")
	}
}

func TestEventBusGroup_MultipleEventTypes(t *testing.T) {
	chain := aspect.NewAspectChain()
	group := NewEventBusGroup(chain)

	userHandler := &testUserEventHandler{}
	orderHandler := &testOrderEventHandler{}

	EventGroupBus[*testUserEvent](group).Subscribe(userHandler)
	EventGroupBus[*testOrderEvent](group).Subscribe(orderHandler)

	ctx := context.Background()

	EventGroupPublish(group, ctx, &testUserEvent{aggregateID: "u1"})
	EventGroupPublish(group, ctx, &testOrderEvent{aggregateID: "o1"})
	EventGroupPublish(group, ctx, &testUserEvent{aggregateID: "u2"})

	if userHandler.callCount != 2 {
		t.Errorf("expected userHandler called 2 times, got %d", userHandler.callCount)
	}
	if !orderHandler.called {
		t.Error("orderHandler should have been called")
	}
}

func TestEventBusGroup_DifferentEventTypesIsolated(t *testing.T) {
	chain := aspect.NewAspectChain()
	group := NewEventBusGroup(chain)

	handler := &testUserEventHandler{}
	EventGroupBus[*testUserEvent](group).Subscribe(handler)

	ctx := context.Background()
	err := EventGroupPublish[*testOrderEvent](group, ctx, &testOrderEvent{aggregateID: "1"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if handler.called {
		t.Error("handler should not be called for different event type")
	}
}

func TestEventBus_DuplicateSubscription_Panics(t *testing.T) {
	bus := NewEventBus[*testUserEvent](nil)
	handler := &testUserEventHandler{}
	bus.Subscribe(handler)

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for duplicate subscription")
		}
		msg := r.(string)
		expected := "handler already subscribed for this event type"
		if msg != expected {
			t.Errorf("expected panic message %q, got %q", expected, msg)
		}
	}()

	bus.Subscribe(handler)
}

func TestEventBusGroup_NilChain(t *testing.T) {
	group := NewEventBusGroup(nil)

	handler := &testUserEventHandler{}
	EventGroupBus[*testUserEvent](group).Subscribe(handler)

	ctx := context.Background()
	err := EventGroupPublish[*testUserEvent](group, ctx, &testUserEvent{aggregateID: "1"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handler.called {
		t.Error("handler was not called")
	}
}

type testEventAspect struct {
	beforeFn func()
	afterFn  func()
}

func (a *testEventAspect) Name() string { return "test" }
func (a *testEventAspect) Order() int   { return 1 }
func (a *testEventAspect) BeforePublish(ctx context.Context, event any) (context.Context, error) {
	if a.beforeFn != nil {
		a.beforeFn()
	}
	return ctx, nil
}
func (a *testEventAspect) AfterPublish(ctx context.Context, event any, err error, d time.Duration) error {
	if a.afterFn != nil {
		a.afterFn()
	}
	return nil
}

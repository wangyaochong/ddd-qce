package event_test

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/event"
	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
	domainevent "github.com/ddd-qce/core/domain/event"
)

type testEvent struct {
	domainevent.BaseEvent
}

type testEventHandler struct {
	called bool
}

func (h *testEventHandler) Handle(ctx context.Context, evt *testEvent) error {
	h.called = true
	return nil
}

func TestEventBus_InterfaceSatisfied(t *testing.T) {
	var _ event.EventBus = eventmemory.NewEventBus()
}

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

	var _ event.EventBus = bus

	handler := &testEventHandler{}
	bus.SubscribeHandler(handler)

	ctx := context.Background()
	err := event.Dispatch[*testEvent](ctx, bus, &testEvent{BaseEvent: domainevent.NewBaseEvent("agg-1", time.Now())})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handler.called {
		t.Error("handler was not called")
	}
}

func TestEventHandler_ImplementsDomainEventHandler(t *testing.T) {
	var _ domainevent.EventHandler[*testEvent] = &testEventHandler{}
}

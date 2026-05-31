package impl

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/impl/memory"
)

type testEvent struct {
	event.BaseEvent
}

type testEventHandler struct {
	called bool
}

func (h *testEventHandler) Handle(ctx context.Context, evt *testEvent) error {
	h.called = true
	return nil
}

func TestEventBus_InterfaceSatisfied(t *testing.T) {
	var _ event.EventBus = memory.NewEventBus()
}

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := memory.NewEventBus(memory.WithEventBusAspectChain(chain))

	var _ event.EventBus = bus

	handler := &testEventHandler{}
	bus.SubscribeHandler(handler)

	ctx := context.Background()
	err := bus.Publish(ctx, &testEvent{BaseEvent: event.NewBaseEvent("agg-1", time.Now())})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !handler.called {
		t.Error("handler was not called")
	}
}

func TestEventHandler_ImplementsDomainEventHandler(t *testing.T) {
	var _ event.EventHandler[*testEvent] = &testEventHandler{}
}

package event

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
	domainevent "github.com/ddd-qce/core/domain/event"
)

type testEvent struct {
	domainevent.BaseEvent
}

type testEventHandler struct {
	called bool
}

func (h *testEventHandler) Handle(ctx context.Context, event *testEvent) error {
	h.called = true
	return nil
}

func TestEventBus_InterfaceSatisfied(t *testing.T) {
	var _ EventBus = eventmemory.NewEventBus()
}

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

	var _ EventBus = bus

	handler := &testEventHandler{}
	eventmemory.RegisterHandler[*testEvent](bus, handler)

	ctx := context.Background()
	err := eventmemory.Dispatch[*testEvent](ctx, bus, &testEvent{BaseEvent: domainevent.NewBaseEvent("agg-1", time.Now())})
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

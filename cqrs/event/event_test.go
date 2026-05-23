package event

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	domainevent "github.com/ddd-qce/core/domain/event"
	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
)

type testEvent struct {
	AggID string
}

func (e *testEvent) AggregateID() string   { return e.AggID }
func (e *testEvent) EventType() string     { return domainevent.EventTypeOf(e) }
func (e *testEvent) OccurredAt() time.Time { return time.Now() }

type testEventHandler struct {
	called bool
}

func (h *testEventHandler) Handle(ctx context.Context, event *testEvent) error {
	h.called = true
	return nil
}

func TestEventBus_InterfaceSatisfied(t *testing.T) {
	var _ EventBus[*testEvent] = eventmemory.NewEventBus[*testEvent](nil)
}

func TestEventBus_SubscribeAndPublish(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := eventmemory.NewEventBus[*testEvent](chain)

	var _ EventBus[*testEvent] = bus

	handler := &testEventHandler{}
	bus.Subscribe(domainevent.EventHandler[*testEvent](handler))

	ctx := context.Background()
	err := bus.Publish(ctx, &testEvent{AggID: "agg-1"})
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

package memory

import (
	"context"
	"fmt"
	"reflect"
	"sync"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/domain/event"
)

type EventBus[T event.DomainEvent] struct {
	handlers []event.EventHandler[T]
	chain    *aspect.AspectChain
	mu       sync.RWMutex
}

func NewEventBus[T event.DomainEvent](chain *aspect.AspectChain) *EventBus[T] {
	if chain == nil {
		chain = aspect.NewAspectChain()
	}
	return &EventBus[T]{
		handlers: make([]event.EventHandler[T], 0),
		chain:    chain,
	}
}

func (b *EventBus[T]) Subscribe(handler event.EventHandler[T]) {
	b.mu.Lock()
	defer b.mu.Unlock()
	handlerVal := reflect.ValueOf(handler)
	for _, h := range b.handlers {
		if reflect.ValueOf(h).Pointer() == handlerVal.Pointer() {
			panic("handler already subscribed for this event type")
		}
	}
	b.handlers = append(b.handlers, handler)
}

func (b *EventBus[T]) Publish(ctx context.Context, evt T) error {
	b.mu.RLock()
	handlers := make([]event.EventHandler[T], len(b.handlers))
	copy(handlers, b.handlers)
	b.mu.RUnlock()

	var firstErr error
	for _, handler := range handlers {
		h := handler
		err := b.chain.ExecuteWithEventAspects(ctx, evt, func(ctx context.Context) error {
			return h.Handle(ctx, evt)
		})

		if err != nil && firstErr == nil {
			firstErr = err
		}
	}

	if firstErr != nil {
		return fmt.Errorf("event handler error: %w", firstErr)
	}
	return nil
}

type EventBusGroup struct {
	buses map[reflect.Type]any
	mu    sync.RWMutex
	chain *aspect.AspectChain
}

func NewEventBusGroup(chain *aspect.AspectChain) *EventBusGroup {
	if chain == nil {
		chain = aspect.NewAspectChain()
	}
	return &EventBusGroup{
		buses: make(map[reflect.Type]any),
		chain: chain,
	}
}

func EventGroupBus[T event.DomainEvent](group *EventBusGroup) *EventBus[T] {
	group.mu.Lock()
	defer group.mu.Unlock()

	eventType := reflect.TypeOf((*T)(nil)).Elem()
	if bus, ok := group.buses[eventType]; ok {
		return bus.(*EventBus[T])
	}

	bus := NewEventBus[T](group.chain)
	group.buses[eventType] = bus
	return bus
}

func EventGroupPublish[T event.DomainEvent](group *EventBusGroup, ctx context.Context, evt T) error {
	return EventGroupBus[T](group).Publish(ctx, evt)
}

package event

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ddd-qce/core/trace"
)

type testEvent struct {
	BaseEvent
	Value string
}

type testEventHandler struct {
	err error
}

func (h *testEventHandler) Handle(ctx context.Context, evt testEvent) error {
	return h.err
}

type testEventBus struct {
	handlers      map[string]any
	subscribeErr  error
	publishErr    error
	publishedEvts []Event
}

func (b *testEventBus) SubscribeHandler(handler any) error {
	if b.subscribeErr != nil {
		return b.subscribeErr
	}
	name := EventTypeOf(handler)
	b.handlers[name] = handler
	return nil
}

func (b *testEventBus) Publish(ctx context.Context, evt Event) error {
	if b.publishErr != nil {
		return b.publishErr
	}
	b.publishedEvts = append(b.publishedEvts, evt)
	return nil
}

func TestNewBaseEvent(t *testing.T) {
	now := time.Now()
	evt := NewBaseEvent("agg-1", now)

	if evt.AggregateID() != "agg-1" {
		t.Errorf("AggregateID() = %q, want %q", evt.AggregateID(), "agg-1")
	}
	if !evt.OccurredAt().Equal(now) {
		t.Errorf("OccurredAt() = %v, want %v", evt.OccurredAt(), now)
	}
}

func TestNewDomainEvent(t *testing.T) {
	before := time.Now()
	evt := NewDomainEvent("agg-1")
	after := time.Now()

	if evt.AggregateID() != "agg-1" {
		t.Errorf("AggregateID() = %q, want %q", evt.AggregateID(), "agg-1")
	}
	if evt.OccurredAt().Before(before) || evt.OccurredAt().After(after) {
		t.Error("OccurredAt() should be close to current time")
	}
}

func TestApplyCorrelation(t *testing.T) {
	evt := &testEvent{BaseEvent: NewBaseEvent("agg-1", time.Now())}
	if evt.CorrelationID() != "" {
		t.Errorf("CorrelationID() = %q, want empty before ApplyCorrelation", evt.CorrelationID())
	}

	ApplyCorrelation(evt, "corr-1", "caus-1")

	if evt.CorrelationID() != "corr-1" {
		t.Errorf("CorrelationID() = %q, want %q", evt.CorrelationID(), "corr-1")
	}
	if evt.CausationID() != "caus-1" {
		t.Errorf("CausationID() = %q, want %q", evt.CausationID(), "caus-1")
	}
}

func TestApplyCorrelation_ZeroBaseEvent(t *testing.T) {
	evt := &testEvent{}
	ApplyCorrelation(evt, "corr-1", "caus-1")
	if evt.CorrelationID() != "corr-1" {
		t.Errorf("ApplyCorrelation should work on zero-value BaseEvent")
	}
}

func TestNewDomainEventWithCorrelation(t *testing.T) {
	evt := NewDomainEventWithCorrelation("agg-1", "corr-1", "caus-1")

	if evt.AggregateID() != "agg-1" {
		t.Errorf("AggregateID() = %q, want %q", evt.AggregateID(), "agg-1")
	}
	if evt.CorrelationID() != "corr-1" {
		t.Errorf("CorrelationID() = %q, want %q", evt.CorrelationID(), "corr-1")
	}
	if evt.CausationID() != "caus-1" {
		t.Errorf("CausationID() = %q, want %q", evt.CausationID(), "caus-1")
	}
}

func TestWithCorrelation_FromTraceContext(t *testing.T) {
	traceID := trace.NewTraceID()
	spanID := trace.NewSpanID()
	ctx := trace.WithTrace(context.Background(), traceID, spanID)

	evt := WithCorrelation(ctx, "agg-1")

	if evt.AggregateID() != "agg-1" {
		t.Errorf("AggregateID() = %q, want %q", evt.AggregateID(), "agg-1")
	}
	if evt.CorrelationID() != traceID {
		t.Errorf("CorrelationID() = %q, want %q", evt.CorrelationID(), traceID)
	}
	if evt.CausationID() != spanID {
		t.Errorf("CausationID() = %q, want %q", evt.CausationID(), spanID)
	}
}

func TestWithCorrelation_EmptyContext(t *testing.T) {
	ctx := context.Background()
	evt := WithCorrelation(ctx, "agg-1")

	if evt.CorrelationID() != "" {
		t.Errorf("CorrelationID() = %q, want empty when no trace in context", evt.CorrelationID())
	}
	if evt.CausationID() != "" {
		t.Errorf("CausationID() = %q, want empty when no trace in context", evt.CausationID())
	}
}

func TestEventTypeOf(t *testing.T) {
	tests := []struct {
		name     string
		event    any
		expected string
	}{
		{"struct type", testEvent{}, "testEvent"},
		{"pointer type", &testEvent{}, "testEvent"},
		{"base event", BaseEvent{}, "BaseEvent"},
		{"nil event", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EventTypeOf(tt.event)
			if result != tt.expected {
				t.Errorf("EventTypeOf(%T) = %q, want %q", tt.event, result, tt.expected)
			}
		})
	}
}

func TestDispatch_Success(t *testing.T) {
	bus := &testEventBus{
		handlers: make(map[string]any),
	}

	evt := testEvent{Value: "test"}
	err := Dispatch(context.Background(), bus, evt)
	if err != nil {
		t.Errorf("Dispatch() error = %v, want nil", err)
	}
	if len(bus.publishedEvts) != 1 {
		t.Errorf("Dispatch() published %d events, want 1", len(bus.publishedEvts))
	}
}

func TestDispatch_PublishError(t *testing.T) {
	bus := &testEventBus{
		handlers:   make(map[string]any),
		publishErr: errors.New("publish failed"),
	}

	evt := testEvent{Value: "test"}
	err := Dispatch(context.Background(), bus, evt)
	if err == nil {
		t.Error("Dispatch() should return error from Publish")
	}
}

func TestEventSourceStoreInterface(t *testing.T) {
	// Verify EventSourceStore interface is implemented correctly
	var _ EventSourceStore[testEvent] = (*testEventSourceStore)(nil)
}

type testEventSourceStore struct{}

func (s *testEventSourceStore) Append(ctx context.Context, aggregateID string, expectedVersion int, events []testEvent) error {
	return nil
}

func (s *testEventSourceStore) Load(ctx context.Context, aggregateID string, afterVersion int) ([]testEvent, error) {
	return nil, nil
}

func (s *testEventSourceStore) LoadAll(ctx context.Context, afterPosition int64, limit int) ([]GlobalEvent[testEvent], error) {
	return nil, nil
}

func TestEventHandlerInterface(t *testing.T) {
	// Verify EventHandler interface is implemented correctly
	var _ EventHandler[testEvent] = (*testEventHandler)(nil)
}
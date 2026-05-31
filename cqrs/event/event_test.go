package event

import (
	"context"
	"testing"
	"time"

	domainevent "github.com/ddd-qce/core/domain/event"
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
	publishedEvts []domainevent.Event
}

func (b *testEventBus) SubscribeHandler(handler any) error {
	if b.subscribeErr != nil {
		return b.subscribeErr
	}
	name := EventNameOf(handler)
	b.handlers[name] = handler
	return nil
}

func (b *testEventBus) SubscribedTypes() []string {
	names := make([]string, 0, len(b.handlers))
	for k := range b.handlers {
		names = append(names, k)
	}
	return names
}

func (b *testEventBus) Publish(ctx context.Context, evt domainevent.Event) error {
	if b.publishErr != nil {
		return b.publishErr
	}
	b.publishedEvts = append(b.publishedEvts, evt)
	return nil
}

func (b *testEventBus) Shutdown(ctx context.Context) error {
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

func TestEventNameOf(t *testing.T) {
	tests := []struct {
		name     string
		event    any
		expected string
	}{
		{"struct type", testEvent{}, "testEvent"},
		{"pointer type", &testEvent{}, "testEvent"},
		{"base event", BaseEvent{}, "BaseEvent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := EventNameOf(tt.event)
			if result != tt.expected {
				t.Errorf("EventNameOf(%T) = %q, want %q", tt.event, result, tt.expected)
			}
		})
	}
}

func TestEventNameOf_NilPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("EventNameOf(nil) should panic")
		}
		if msg, ok := r.(string); !ok || msg != "event: EventNameOf called with nil event" {
			t.Errorf("unexpected panic message: %v", r)
		}
	}()
	EventNameOf(nil)
}

func TestAggregateEventStoreInterface(t *testing.T) {
	var _ AggregateEventStore[testEvent] = (*testAggregateEventStore)(nil)
}

func TestGlobalEventStoreInterface(t *testing.T) {
	var _ GlobalEventStore[testEvent] = (*testGlobalEventStore)(nil)
}

type testAggregateEventStore struct{}

func (s *testAggregateEventStore) Append(ctx context.Context, aggregateID string, expectedVersion int, events []testEvent) error {
	return nil
}

func (s *testAggregateEventStore) Load(ctx context.Context, aggregateID string, afterVersion int) ([]testEvent, error) {
	return nil, nil
}

type testGlobalEventStore struct{}

func (s *testGlobalEventStore) LoadAll(ctx context.Context, afterPosition int64, limit int) ([]GlobalEvent[testEvent], error) {
	return nil, nil
}

func TestRestoreBaseEvent_WithBaseEvent(t *testing.T) {
	evt := &testEvent{}
	now := time.Now()
	RestoreBaseEvent(evt, "agg-1", now, "corr-1", "caus-1")

	if evt.AggregateID() != "agg-1" {
		t.Errorf("AggregateID() = %q, want %q", evt.AggregateID(), "agg-1")
	}
	if !evt.OccurredAt().Equal(now) {
		t.Errorf("OccurredAt() = %v, want %v", evt.OccurredAt(), now)
	}
	if evt.CorrelationID() != "corr-1" {
		t.Errorf("CorrelationID() = %q, want %q", evt.CorrelationID(), "corr-1")
	}
	if evt.CausationID() != "caus-1" {
		t.Errorf("CausationID() = %q, want %q", evt.CausationID(), "caus-1")
	}
}

type noRestoreEvent struct {
	BaseEvent
}

func TestRestoreBaseEvent_WithBaseEventField(t *testing.T) {
	evt := &noRestoreEvent{}
	now := time.Now()
	RestoreBaseEvent(evt, "agg-1", now, "corr-1", "caus-1")

	if evt.AggregateID() != "agg-1" {
		t.Errorf("AggregateID() = %q, want %q", evt.AggregateID(), "agg-1")
	}
}

type bareEvent struct {
	aggregateID string
}

func (e *bareEvent) AggregateID() string   { return e.aggregateID }
func (e *bareEvent) OccurredAt() time.Time { return time.Time{} }
func (e *bareEvent) CorrelationID() string { return "" }
func (e *bareEvent) CausationID() string   { return "" }

func TestRestoreBaseEvent_EventWithoutBaseEventField(t *testing.T) {
	evt := &bareEvent{aggregateID: "original"}
	now := time.Now()
	RestoreBaseEvent(evt, "agg-new", now, "corr-1", "caus-1")

	if evt.AggregateID() != "original" {
		t.Errorf("AggregateID() = %q, want %q (no-op since no BaseEvent field)", evt.AggregateID(), "original")
	}
}

func TestEventHandlerInterface(t *testing.T) {
	var _ EventHandler[testEvent] = (*testEventHandler)(nil)
}

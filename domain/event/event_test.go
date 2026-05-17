package event

import (
	"testing"
	"time"
)

type testPointerEvent struct{}

func (e *testPointerEvent) AggregateID() string   { return "agg-1" }
func (e *testPointerEvent) EventType() string     { return EventTypeOf(e) }
func (e *testPointerEvent) OccurredAt() time.Time { return time.Now() }

type testValueEvent struct{}

func (e testValueEvent) AggregateID() string   { return "agg-2" }
func (e testValueEvent) EventType() string     { return EventTypeOf(e) }
func (e testValueEvent) OccurredAt() time.Time { return time.Now() }

type testPtrReceiverValueEvent struct{}

func (e *testPtrReceiverValueEvent) AggregateID() string   { return "agg-3" }
func (e *testPtrReceiverValueEvent) EventType() string     { return EventTypeOf(e) }
func (e *testPtrReceiverValueEvent) OccurredAt() time.Time { return time.Now() }

func TestEventTypeOf_PointerType(t *testing.T) {
	event := &testPointerEvent{}
	name := EventTypeOf(event)
	if name != "testPointerEvent" {
		t.Errorf("expected 'testPointerEvent', got '%s'", name)
	}
}

func TestEventTypeOf_ValueType(t *testing.T) {
	event := testValueEvent{}
	name := EventTypeOf(event)
	if name != "testValueEvent" {
		t.Errorf("expected 'testValueEvent', got '%s'", name)
	}
}

func TestEventTypeOf_PointerReceiverOnValue(t *testing.T) {
	event := &testPtrReceiverValueEvent{}
	name := EventTypeOf(event)
	if name != "testPtrReceiverValueEvent" {
		t.Errorf("expected 'testPtrReceiverValueEvent', got '%s'", name)
	}
}

func TestEventTypeOf_NamedType(t *testing.T) {
	event := &testPointerEvent{}
	name := EventTypeOf(event)
	if name != "testPointerEvent" {
		t.Errorf("expected 'testPointerEvent', got '%s'", name)
	}
}

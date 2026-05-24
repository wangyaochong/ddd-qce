package event

import (
	"testing"
	"time"
)

type testPointerEvent struct {
	BaseEvent
}

type testValueEvent struct {
	BaseEvent
}

type testPtrReceiverValueEvent struct {
	BaseEvent
}

func TestEventTypeOf_PointerType(t *testing.T) {
	evt := &testPointerEvent{BaseEvent: NewBaseEvent("agg-1", time.Now())}
	name := EventTypeOf(evt)
	if name != "testPointerEvent" {
		t.Errorf("expected 'testPointerEvent', got '%s'", name)
	}
}

func TestEventTypeOf_ValueType(t *testing.T) {
	evt := testValueEvent{BaseEvent: NewBaseEvent("agg-2", time.Now())}
	name := EventTypeOf(evt)
	if name != "testValueEvent" {
		t.Errorf("expected 'testValueEvent', got '%s'", name)
	}
}

func TestEventTypeOf_PointerReceiverOnValue(t *testing.T) {
	evt := &testPtrReceiverValueEvent{BaseEvent: NewBaseEvent("agg-3", time.Now())}
	name := EventTypeOf(evt)
	if name != "testPtrReceiverValueEvent" {
		t.Errorf("expected 'testPtrReceiverValueEvent', got '%s'", name)
	}
}

func TestEventTypeOf_NamedType(t *testing.T) {
	evt := &testPointerEvent{BaseEvent: NewBaseEvent("agg-1", time.Now())}
	name := EventTypeOf(evt)
	if name != "testPointerEvent" {
		t.Errorf("expected 'testPointerEvent', got '%s'", name)
	}
}

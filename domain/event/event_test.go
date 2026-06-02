package event

import (
	"testing"
)

type testEvent struct {
	id string
}

func (e testEvent) Metadata() any { return e }

type otherEvent struct {
	name string
}

func (e otherEvent) Metadata() any { return e }

func TestFromSlice_NonEmpty(t *testing.T) {
	events := []testEvent{
		{id: "a"},
		{id: "b"},
	}
	result := FromSlice(events)
	if len(result) != 2 {
		t.Fatalf("expected 2 events, got %d", len(result))
	}
	if result[0].(testEvent).id != "a" {
		t.Errorf("expected first event id 'a', got %v", result[0])
	}
	if result[1].(testEvent).id != "b" {
		t.Errorf("expected second event id 'b', got %v", result[1])
	}
}

func TestFromSlice_Empty(t *testing.T) {
	events := []testEvent{}
	result := FromSlice(events)
	if len(result) != 0 {
		t.Fatalf("expected 0 events, got %d", len(result))
	}
}

func TestFromSlice_Nil(t *testing.T) {
	var events []testEvent
	result := FromSlice(events)
	if len(result) != 0 {
		t.Fatalf("expected 0 events, got %d", len(result))
	}
}

func TestEventInterface(t *testing.T) {
	var _ Event = testEvent{}
	var _ Event = otherEvent{}
}

func TestEventMetadata(t *testing.T) {
	evt := testEvent{id: "test"}
	meta := evt.Metadata()
	te, ok := meta.(testEvent)
	if !ok {
		t.Fatalf("expected testEvent, got %T", meta)
	}
	if te.id != "test" {
		t.Errorf("expected id 'test', got %q", te.id)
	}
}

package entity

import (
	"strings"
	"testing"
)

func TestNewEntityWithID(t *testing.T) {
	e := NewEntityWithID()
	if e.ID == "" {
		t.Error("expected auto-generated ID to be non-empty")
	}
	if e.IsEmpty() {
		t.Error("expected entity with auto-generated ID to not be empty")
	}
}

func TestNewEntityWithID_UUIDFormat(t *testing.T) {
	e := NewEntityWithID()
	if len(e.ID) != 36 {
		t.Errorf("expected UUID format (36 chars), got %d chars: %s", len(e.ID), e.ID)
	}
	if strings.Count(e.ID, "-") != 4 {
		t.Errorf("expected UUID format (4 dashes), got %d dashes: %s", strings.Count(e.ID, "-"), e.ID)
	}
}

func TestNewEntityWithID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		e := NewEntityWithID()
		if ids[e.ID] {
			t.Errorf("generated duplicate ID: %s", e.ID)
		}
		ids[e.ID] = true
	}
}

func TestSetIDGenerator(t *testing.T) {
	original := DefaultIDGenerator()
	defer func() { SetIDGenerator(original) }()

	counter := 0
	SetIDGenerator(func() string {
		counter++
		return "custom-id"
	})

	e := NewEntityWithID()
	if e.ID != "custom-id" {
		t.Errorf("expected custom ID 'custom-id', got '%s'", e.ID)
	}
	if counter != 1 {
		t.Errorf("expected custom generator to be called once, got %d calls", counter)
	}
}

func TestSetIDGenerator_NilDoesNotOverride(t *testing.T) {
	original := DefaultIDGenerator()
	defer func() { SetIDGenerator(original) }()

	SetIDGenerator(nil)

	e := NewEntityWithID()
	if e.ID == "" {
		t.Error("expected default generator to still work after nil SetIDGenerator")
	}
}

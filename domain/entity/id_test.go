package entity

import (
	"testing"
)

func TestNewEntityWithID(t *testing.T) {
	e := NewEntityWithID()
	if e.ID() == "" {
		t.Error("expected auto-generated ID to be non-empty")
	}
	if e.IsEmpty() {
		t.Error("expected entity with auto-generated ID to not be empty")
	}
}

func TestNewEntityWithID_UUIDHexFormat(t *testing.T) {
	e := NewEntityWithID()
	if len(e.ID()) != 32 {
		t.Errorf("expected UUID hex format (32 chars), got %d chars: %s", len(e.ID()), e.ID())
	}
	for _, c := range e.ID() {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("expected lowercase hex chars only, got '%c' in %s", c, e.ID())
			break
		}
	}
}

func TestNewEntityWithID_Uniqueness(t *testing.T) {
	ids := make(map[string]bool)
	for i := 0; i < 100; i++ {
		e := NewEntityWithID()
		if ids[e.ID()] {
			t.Errorf("generated duplicate ID: %s", e.ID())
		}
		ids[e.ID()] = true
	}
}

func TestDefaultIDGenerator(t *testing.T) {
	gen := DefaultIDGenerator()
	id := gen()
	if len(id) != 32 {
		t.Errorf("expected UUID hex format (32 chars), got %d chars: %s", len(id), id)
	}
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("expected lowercase hex chars only, got '%c' in %s", c, id)
			break
		}
	}
}

package entity

import "testing"

func TestNewEntity(t *testing.T) {
	e := NewEntity("user-1")
	if e.GetID() != "user-1" {
		t.Errorf("expected ID 'user-1', got '%s'", e.GetID())
	}
}

func TestNewEntity_EmptyID(t *testing.T) {
	e := NewEntity("")
	if !e.IsEmpty() {
		t.Error("expected entity with empty ID to be empty")
	}
}

func TestGetID(t *testing.T) {
	e := NewEntity("user-1")
	if e.GetID() != "user-1" {
		t.Errorf("expected GetID 'user-1', got '%s'", e.GetID())
	}
}

func TestEquals_SameID(t *testing.T) {
	e1 := NewEntity("user-1")
	e2 := NewEntity("user-1")
	if !e1.Equals(e2) {
		t.Error("expected entities with same ID to be equal")
	}
}

func TestEquals_DifferentID(t *testing.T) {
	e1 := NewEntity("user-1")
	e2 := NewEntity("user-2")
	if e1.Equals(e2) {
		t.Error("expected entities with different IDs to not be equal")
	}
}

func TestEquals_NilReceiver(t *testing.T) {
	var e1 *Entity = nil
	e2 := NewEntity("user-1")
	if e1.Equals(e2) {
		t.Error("expected nil receiver to not equal non-nil entity")
	}
}

func TestEquals_NilOther(t *testing.T) {
	e1 := NewEntity("user-1")
	var e2 *Entity = nil
	if e1.Equals(e2) {
		t.Error("expected non-nil entity to not equal nil")
	}
}

func TestEquals_BothNil(t *testing.T) {
	var e1 *Entity = nil
	var e2 *Entity = nil
	if !e1.Equals(e2) {
		t.Error("expected both nil entities to be equal")
	}
}

func TestIsEmpty_True(t *testing.T) {
	e := NewEntity("")
	if !e.IsEmpty() {
		t.Error("expected entity with empty ID to be empty")
	}
}

func TestIsEmpty_False(t *testing.T) {
	e := NewEntity("user-1")
	if e.IsEmpty() {
		t.Error("expected entity with non-empty ID to not be empty")
	}
}

func TestValidate_Valid(t *testing.T) {
	e := NewEntity("user-1")
	if err := e.Validate(); err != nil {
		t.Errorf("expected no error for valid entity, got: %v", err)
	}
}

func TestValidate_EmptyID(t *testing.T) {
	e := NewEntity("")
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

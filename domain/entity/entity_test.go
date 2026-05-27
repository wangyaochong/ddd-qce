package entity

import (
	"encoding/json"
	"testing"
)

func TestNewEntity(t *testing.T) {
	e, err := NewEntity("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.ID() != "user-1" {
		t.Errorf("expected ID 'user-1', got '%s'", e.ID())
	}
}

func TestNewEntity_EmptyID_ReturnsError(t *testing.T) {
	_, err := NewEntity("")
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestEquals_SameID(t *testing.T) {
	e1 := MustNewEntity("user-1")
	e2 := MustNewEntity("user-1")
	if !e1.Equals(e2) {
		t.Error("expected entities with same ID to be equal")
	}
}

func TestEquals_DifferentID(t *testing.T) {
	e1 := MustNewEntity("user-1")
	e2 := MustNewEntity("user-2")
	if e1.Equals(e2) {
		t.Error("expected entities with different IDs to not be equal")
	}
}

func TestEquals_NilReceiver(t *testing.T) {
	var e1 *Entity = nil
	e2 := MustNewEntity("user-1")
	if e1.Equals(e2) {
		t.Error("expected nil receiver to not equal non-nil entity")
	}
}

func TestEquals_NilOther(t *testing.T) {
	e1 := MustNewEntity("user-1")
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
	e := &Entity{}
	if !e.IsEmpty() {
		t.Error("expected entity with empty ID to be empty")
	}
}

func TestIsEmpty_False(t *testing.T) {
	e := MustNewEntity("user-1")
	if e.IsEmpty() {
		t.Error("expected entity with non-empty ID to not be empty")
	}
}

func TestValidate_Valid(t *testing.T) {
	e := MustNewEntity("user-1")
	if err := e.Validate(); err != nil {
		t.Errorf("expected no error for valid entity, got: %v", err)
	}
}

func TestValidate_EmptyID(t *testing.T) {
	e := &Entity{}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestEntity_JSONRoundTrip(t *testing.T) {
	e := MustNewEntity("user-1")
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var e2 Entity
	if err := json.Unmarshal(data, &e2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if e2.ID() != "user-1" {
		t.Errorf("expected ID 'user-1' after round trip, got '%s'", e2.ID())
	}
}

func TestEntity_Clone(t *testing.T) {
	e := MustNewEntity("user-1")
	clone := e.Clone()
	if clone.ID() != "user-1" {
		t.Errorf("clone ID = %s, want user-1", clone.ID())
	}
	if clone == e {
		t.Error("clone should be a different pointer")
	}
}

func TestEntity_Clone_Nil(t *testing.T) {
	var e *Entity
	clone := e.Clone()
	if clone != nil {
		t.Error("cloning nil entity should return nil")
	}
}

func TestNewEntityWithID_DefaultValues(t *testing.T) {
	e := NewEntityWithID()
	if e.IsEmpty() {
		t.Error("NewEntityWithID should create entity with non-empty ID")
	}
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

func TestEntity_UnmarshalJSON_Invalid(t *testing.T) {
	e := &Entity{}
	err := json.Unmarshal([]byte("invalid json"), e)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestEntity_UnmarshalJSON_EmptyID(t *testing.T) {
	e := &Entity{}
	err := json.Unmarshal([]byte(`{"id": ""}`), e)
	if err == nil {
		t.Error("unmarshal with empty ID should error")
	}
}

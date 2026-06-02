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

func TestEquals(t *testing.T) {
	alice1 := MustNewEntity("user-1")
	alice2 := MustNewEntity("user-1")
	bob := MustNewEntity("user-2")

	tests := []struct {
		name string
		a, b *Entity
		want bool
	}{
		{"same ID", alice1, alice2, true},
		{"different ID", alice1, bob, false},
		{"nil receiver", nil, alice1, false},
		{"nil other", alice1, nil, false},
		{"both nil", nil, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.a.Equals(tt.b); got != tt.want {
				t.Errorf("Equals() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsEmpty(t *testing.T) {
	tests := []struct {
		name string
		e    *Entity
		want bool
	}{
		{"empty ID", &Entity{}, true},
		{"non-empty ID", MustNewEntity("user-1"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.e.IsEmpty(); got != tt.want {
				t.Errorf("IsEmpty() = %v, want %v", got, tt.want)
			}
		})
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

func TestEntity_ToJSON(t *testing.T) {
	e := MustNewEntity("user-1")
	j := e.ToJSON()
	if j.ID != "user-1" {
		t.Errorf("ToJSON().ID = %q, want %q", j.ID, "user-1")
	}
}

func TestEntity_FromJSON(t *testing.T) {
	e := &Entity{}
	e.FromJSON(EntityJSON{ID: "user-2"})
	if e.ID() != "user-2" {
		t.Errorf("FromJSON: ID = %q, want %q", e.ID(), "user-2")
	}
}

func TestEntity_ToJSON_FromJSON_RoundTrip(t *testing.T) {
	e := MustNewEntity("user-1")
	j := e.ToJSON()

	e2 := &Entity{}
	e2.FromJSON(j)
	if e2.ID() != "user-1" {
		t.Errorf("round-trip ID = %q, want %q", e2.ID(), "user-1")
	}
}

func TestMustNewEntity_PanicWithEmptyID(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("MustNewEntity('') should panic")
		}
	}()
	MustNewEntity("")
}

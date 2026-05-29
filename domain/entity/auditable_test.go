package entity

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewAuditableEntity(t *testing.T) {
	before := time.Now()
	e, err := NewAuditableEntity("user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	after := time.Now()

	if e.ID() != "user-1" {
		t.Errorf("expected ID 'user-1', got '%s'", e.ID())
	}
	if e.CreatedAt().Before(before) || e.CreatedAt().After(after) {
		t.Errorf("expected CreatedAt between %v and %v, got %v", before, after, e.CreatedAt())
	}
	if e.UpdatedAt().Before(before) || e.UpdatedAt().After(after) {
		t.Errorf("expected UpdatedAt between %v and %v, got %v", before, after, e.UpdatedAt())
	}
}

func TestNewAuditableEntity_EmptyID_ReturnsError(t *testing.T) {
	_, err := NewAuditableEntity("")
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestAuditableEntity_Touch(t *testing.T) {
	e := MustNewAuditableEntity("user-1")
	originalUpdatedAt := e.UpdatedAt()

	time.Sleep(10 * time.Millisecond)
	e.Touch()

	if !e.UpdatedAt().After(originalUpdatedAt) {
		t.Errorf("expected UpdatedAt to be after original, got original=%v updated=%v", originalUpdatedAt, e.UpdatedAt())
	}
}

func TestAuditableEntity_CreatedAtNotChanged(t *testing.T) {
	e := MustNewAuditableEntity("user-1")
	originalCreatedAt := e.CreatedAt()

	time.Sleep(10 * time.Millisecond)
	e.Touch()

	if e.CreatedAt() != originalCreatedAt {
		t.Error("expected CreatedAt to remain unchanged after Touch")
	}
}

func TestAuditableEntity_Validate(t *testing.T) {
	e := MustNewAuditableEntity("user-1")
	if err := e.Validate(); err != nil {
		t.Errorf("expected no error for valid auditable entity, got: %v", err)
	}
}

func TestAuditableEntity_Validate_EmptyID(t *testing.T) {
	e := &AuditableEntity{}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for empty ID in auditable entity")
	}
}

func TestNewAuditableEntityFromData(t *testing.T) {
	ct := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ut := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	e, err := NewAuditableEntityFromData("user-1", ct, ut)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.ID() != "user-1" {
		t.Errorf("expected ID 'user-1', got '%s'", e.ID())
	}
	if e.CreatedAt() != ct {
		t.Errorf("expected CreatedAt %v, got %v", ct, e.CreatedAt())
	}
	if e.UpdatedAt() != ut {
		t.Errorf("expected UpdatedAt %v, got %v", ut, e.UpdatedAt())
	}
}

func TestAuditableEntity_JSONRoundTrip(t *testing.T) {
	e := MustNewAuditableEntity("user-1")
	data, err := e.MarshalJSON()
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var e2 AuditableEntity
	if err := json.Unmarshal(data, &e2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if e2.ID() != "user-1" {
		t.Errorf("expected ID 'user-1', got '%s'", e2.ID())
	}
}

func TestNewAuditableEntityWithID(t *testing.T) {
	e := NewAuditableEntityWithID("user-1")
	if e.ID() != "user-1" {
		t.Errorf("expected ID 'user-1', got '%s'", e.ID())
	}
}

func TestMustNewAuditableEntity_PanicWithEmptyID(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("MustNewAuditableEntity('') should panic")
		}
	}()
	MustNewAuditableEntity("")
}

package entity

import (
	"testing"
	"time"
)

func TestNewAuditableEntity(t *testing.T) {
	before := time.Now()
	e := NewAuditableEntity("user-1")
	after := time.Now()

	if e.GetID() != "user-1" {
		t.Errorf("expected ID 'user-1', got '%s'", e.GetID())
	}
	if e.CreatedAt.Before(before) || e.CreatedAt.After(after) {
		t.Errorf("expected CreatedAt between %v and %v, got %v", before, after, e.CreatedAt)
	}
	if e.UpdatedAt.Before(before) || e.UpdatedAt.After(after) {
		t.Errorf("expected UpdatedAt between %v and %v, got %v", before, after, e.UpdatedAt)
	}
}

func TestNewAuditableEntity_EmptyID(t *testing.T) {
	e := NewAuditableEntity("")
	if !e.IsEmpty() {
		t.Error("expected auditable entity with empty ID to be empty")
	}
}

func TestAuditableEntity_Touch(t *testing.T) {
	e := NewAuditableEntity("user-1")
	originalUpdatedAt := e.UpdatedAt

	time.Sleep(10 * time.Millisecond)
	e.Touch()

	if !e.UpdatedAt.After(originalUpdatedAt) {
		t.Errorf("expected UpdatedAt to be after original, got original=%v updated=%v", originalUpdatedAt, e.UpdatedAt)
	}
}

func TestAuditableEntity_CreatedAtNotChanged(t *testing.T) {
	e := NewAuditableEntity("user-1")
	originalCreatedAt := e.CreatedAt

	time.Sleep(10 * time.Millisecond)
	e.Touch()

	if e.CreatedAt != originalCreatedAt {
		t.Error("expected CreatedAt to remain unchanged after Touch")
	}
}

func TestAuditableEntity_Validate(t *testing.T) {
	e := NewAuditableEntity("user-1")
	if err := e.Validate(); err != nil {
		t.Errorf("expected no error for valid auditable entity, got: %v", err)
	}
}

func TestAuditableEntity_Validate_EmptyID(t *testing.T) {
	e := NewAuditableEntity("")
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for empty ID in auditable entity")
	}
}

func TestAuditableEntity_GetID(t *testing.T) {
	e := NewAuditableEntity("user-1")
	if e.GetID() != "user-1" {
		t.Errorf("expected GetID 'user-1', got '%s'", e.GetID())
	}
}

package entity

import (
	"testing"
	"time"
)

func TestNewSoftDeletableEntity(t *testing.T) {
	e := NewSoftDeletableEntity("doc-1")
	if e.GetID() != "doc-1" {
		t.Errorf("expected ID 'doc-1', got '%s'", e.GetID())
	}
	if e.IsDeleted() {
		t.Error("expected new soft deletable entity to not be deleted")
	}
	if e.DeletedAt() != nil {
		t.Error("expected DeletedAt to be nil for new entity")
	}
}

func TestSoftDeletableEntity_SoftDelete(t *testing.T) {
	e := NewSoftDeletableEntity("doc-1")
	originalUpdatedAt := e.UpdatedAt()

	e.SoftDelete()

	if !e.IsDeleted() {
		t.Error("expected entity to be deleted after SoftDelete")
	}
	if e.DeletedAt() == nil {
		t.Fatal("expected DeletedAt to be set after SoftDelete")
	}
	if !e.UpdatedAt().After(originalUpdatedAt) {
		t.Error("expected UpdatedAt to be updated after SoftDelete")
	}
}

func TestSoftDeletableEntity_Restore(t *testing.T) {
	e := NewSoftDeletableEntity("doc-1")
	e.SoftDelete()
	originalUpdatedAt := e.UpdatedAt()

	e.Restore()

	if e.IsDeleted() {
		t.Error("expected entity to not be deleted after Restore")
	}
	if e.DeletedAt() != nil {
		t.Error("expected DeletedAt to be nil after Restore")
	}
	if !e.UpdatedAt().After(originalUpdatedAt) {
		t.Error("expected UpdatedAt to be updated after Restore")
	}
}

func TestSoftDeletableEntity_Validate(t *testing.T) {
	e := NewSoftDeletableEntity("doc-1")
	if err := e.Validate(); err != nil {
		t.Errorf("expected no error for valid entity, got: %v", err)
	}
}

func TestSoftDeletableEntity_Validate_EmptyID(t *testing.T) {
	e := NewSoftDeletableEntity("")
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestSoftDeletableEntity_EmbedsAuditable(t *testing.T) {
	e := NewSoftDeletableEntity("doc-1")
	if e.CreatedAt().IsZero() {
		t.Error("expected CreatedAt to be set from AuditableEntity")
	}
	if e.UpdatedAt().IsZero() {
		t.Error("expected UpdatedAt to be set from AuditableEntity")
	}
}

func TestNewSoftDeletableEntityFromData(t *testing.T) {
	ct := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ut := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	dt := time.Date(2025, 7, 1, 0, 0, 0, 0, time.UTC)
	e := NewSoftDeletableEntityFromData("doc-1", ct, ut, &dt)

	if e.GetID() != "doc-1" {
		t.Errorf("expected ID 'doc-1', got '%s'", e.GetID())
	}
	if !e.IsDeleted() {
		t.Error("expected entity to be deleted")
	}
	if e.DeletedAt() == nil || *e.DeletedAt() != dt {
		t.Errorf("expected DeletedAt %v, got %v", dt, e.DeletedAt())
	}
}

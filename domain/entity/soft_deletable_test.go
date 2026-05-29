package entity

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewSoftDeletableEntity(t *testing.T) {
	e, err := NewSoftDeletableEntity("doc-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if e.ID() != "doc-1" {
		t.Errorf("expected ID 'doc-1', got '%s'", e.ID())
	}
	if e.IsDeleted() {
		t.Error("expected new soft deletable entity to not be deleted")
	}
	if e.DeletedAt() != nil {
		t.Error("expected DeletedAt to be nil for new entity")
	}
}

func TestSoftDeletableEntity_SoftDelete(t *testing.T) {
	e := mustNewSoftDeletableEntity("doc-1")
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
	e := mustNewSoftDeletableEntity("doc-1")
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
	e := mustNewSoftDeletableEntity("doc-1")
	if err := e.Validate(); err != nil {
		t.Errorf("expected no error for valid entity, got: %v", err)
	}
}

func TestSoftDeletableEntity_Validate_EmptyID(t *testing.T) {
	e := &SoftDeletableEntity{}
	if err := e.Validate(); err == nil {
		t.Fatal("expected error for empty ID")
	}
}

func TestSoftDeletableEntity_EmbedsAuditable(t *testing.T) {
	e := mustNewSoftDeletableEntity("doc-1")
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
	e, err := NewSoftDeletableEntityFromData("doc-1", ct, ut, &dt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if e.ID() != "doc-1" {
		t.Errorf("expected ID 'doc-1', got '%s'", e.ID())
	}
	if !e.IsDeleted() {
		t.Error("expected entity to be deleted")
	}
	if e.DeletedAt() == nil || *e.DeletedAt() != dt {
		t.Errorf("expected DeletedAt %v, got %v", dt, e.DeletedAt())
	}
}

func TestSoftDeletableEntity_JSONRoundTrip(t *testing.T) {
	e := mustNewSoftDeletableEntity("doc-1")
	e.SoftDelete()
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var e2 SoftDeletableEntity
	if err := json.Unmarshal(data, &e2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if e2.ID() != "doc-1" {
		t.Errorf("expected ID 'doc-1', got '%s'", e2.ID())
	}
	if !e2.IsDeleted() {
		t.Error("expected entity to be deleted after unmarshal")
	}
}

func TestSoftDeletableEntity_JSONRoundTrip_NotDeleted(t *testing.T) {
	e := mustNewSoftDeletableEntity("doc-1")
	data, err := json.Marshal(e)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var e2 SoftDeletableEntity
	if err := json.Unmarshal(data, &e2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if e2.ID() != "doc-1" {
		t.Errorf("expected ID 'doc-1', got '%s'", e2.ID())
	}
	if e2.IsDeleted() {
		t.Error("expected entity to not be deleted after unmarshal")
	}
}

func mustNewSoftDeletableEntity(id string) *SoftDeletableEntity {
	e, err := NewSoftDeletableEntity(id)
	if err != nil {
		panic(err)
	}
	return e
}

func TestNewSoftDeletableEntity_EmptyID_ReturnsError(t *testing.T) {
	_, err := NewSoftDeletableEntity("")
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestNewSoftDeletableEntityFromData_EmptyID_ReturnsError(t *testing.T) {
	ct := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ut := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)
	_, err := NewSoftDeletableEntityFromData("", ct, ut, nil)
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

package entity

import (
	"encoding/json"
	"fmt"
	"time"
)

// SoftDeletableEntity extends AuditableEntity with a deletedAt timestamp.
// Instead of hard-deleting, entities are marked as deleted by setting deletedAt.
// Use IsDeleted() to check soft-delete status.
type SoftDeletableEntity struct {
	AuditableEntity
	deletedAt *time.Time
}

func (e *SoftDeletableEntity) DeletedAt() *time.Time { return e.deletedAt }

// NewSoftDeletableEntity creates a SoftDeletableEntity with the given identity.
// Returns an error if id is empty.
func NewSoftDeletableEntity(id string) (*SoftDeletableEntity, error) {
	ae, err := NewAuditableEntity(id)
	if err != nil {
		return nil, err
	}
	return &SoftDeletableEntity{
		AuditableEntity: *ae,
	}, nil
}

// NewSoftDeletableEntityFromData creates a SoftDeletableEntity with explicit timestamps.
// Use this when rehydrating from persistent storage.
// Returns an error if id is empty.
func NewSoftDeletableEntityFromData(id string, createdAt, updatedAt time.Time, deletedAt *time.Time) (*SoftDeletableEntity, error) {
	ae, err := NewAuditableEntityFromData(id, createdAt, updatedAt)
	if err != nil {
		return nil, err
	}
	return &SoftDeletableEntity{
		AuditableEntity: *ae,
		deletedAt:       deletedAt,
	}, nil
}

// IsDeleted returns true if the entity has been soft-deleted.
func (e *SoftDeletableEntity) IsDeleted() bool {
	return e.deletedAt != nil
}

// SoftDelete marks the entity as deleted by setting deletedAt to the current time
// and updating the UpdatedAt timestamp.
func (e *SoftDeletableEntity) SoftDelete() {
	now := time.Now()
	e.deletedAt = &now
	e.Touch()
}

// Restore undoes a soft-delete by clearing the deletedAt timestamp
// and updating the UpdatedAt timestamp.
func (e *SoftDeletableEntity) Restore() {
	e.deletedAt = nil
	e.Touch()
}

func (e *SoftDeletableEntity) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ID        string     `json:"id"`
		CreatedAt time.Time  `json:"createdAt"`
		UpdatedAt time.Time  `json:"updatedAt"`
		DeletedAt *time.Time `json:"deletedAt,omitempty"`
	}{
		ID:        e.id,
		CreatedAt: e.createdAt,
		UpdatedAt: e.updatedAt,
		DeletedAt: e.deletedAt,
	})
}

func (e *SoftDeletableEntity) UnmarshalJSON(data []byte) error {
	var aux struct {
		ID        string     `json:"id"`
		CreatedAt time.Time  `json:"createdAt"`
		UpdatedAt time.Time  `json:"updatedAt"`
		DeletedAt *time.Time `json:"deletedAt,omitempty"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.ID == "" {
		return fmt.Errorf("entity: id is required")
	}
	e.id = aux.ID
	e.createdAt = aux.CreatedAt
	e.updatedAt = aux.UpdatedAt
	e.deletedAt = aux.DeletedAt
	return nil
}

// SoftDeletableEntityJSON is the JSON representation of SoftDeletableEntity.
type SoftDeletableEntityJSON struct {
	AuditableEntityJSON
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

func (e *SoftDeletableEntity) ToJSON() SoftDeletableEntityJSON {
	return SoftDeletableEntityJSON{
		AuditableEntityJSON: e.AuditableEntity.ToJSON(),
		DeletedAt:           e.deletedAt,
	}
}

func (e *SoftDeletableEntity) FromJSON(j SoftDeletableEntityJSON) {
	e.AuditableEntity.FromJSON(j.AuditableEntityJSON)
	e.deletedAt = j.DeletedAt
}

package entity

import (
	"encoding/json"
	"time"
)

type SoftDeletableEntity struct {
	AuditableEntity
	deletedAt *time.Time
}

func NewSoftDeletableEntity(id string) *SoftDeletableEntity {
	return &SoftDeletableEntity{
		AuditableEntity: *NewAuditableEntity(id),
	}
}

func NewSoftDeletableEntityFromData(id string, createdAt, updatedAt time.Time, deletedAt *time.Time) *SoftDeletableEntity {
	return &SoftDeletableEntity{
		AuditableEntity: *NewAuditableEntityFromData(id, createdAt, updatedAt),
		deletedAt:       deletedAt,
	}
}

func (e *SoftDeletableEntity) DeletedAt() *time.Time {
	return e.deletedAt
}

func (e *SoftDeletableEntity) IsDeleted() bool {
	return e.deletedAt != nil
}

func (e *SoftDeletableEntity) SoftDelete() {
	now := time.Now()
	e.deletedAt = &now
	e.Touch()
}

func (e *SoftDeletableEntity) Restore() {
	e.deletedAt = nil
	e.Touch()
}

type softDeletableJSON struct {
	ID        string     `json:"id"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

func (e *SoftDeletableEntity) MarshalJSON() ([]byte, error) {
	return json.Marshal(&softDeletableJSON{
		ID:        e.id,
		CreatedAt: e.createdAt,
		UpdatedAt: e.updatedAt,
		DeletedAt: e.deletedAt,
	})
}

func (e *SoftDeletableEntity) UnmarshalJSON(data []byte) error {
	var v softDeletableJSON
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	e.id = v.ID
	e.createdAt = v.CreatedAt
	e.updatedAt = v.UpdatedAt
	e.deletedAt = v.DeletedAt
	return nil
}

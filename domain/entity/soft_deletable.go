package entity

import "time"

type SoftDeletableEntity struct {
	AuditableEntity
	deletedAt *time.Time `json:"deletedAt,omitempty"`
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

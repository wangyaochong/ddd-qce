package entity

import "time"

type SoftDeletableEntity struct {
	AuditableEntity
	DeletedAt *time.Time `json:"deletedAt,omitempty"`
}

func NewSoftDeletableEntity(id string) *SoftDeletableEntity {
	return &SoftDeletableEntity{
		AuditableEntity: *NewAuditableEntity(id),
	}
}

func NewSoftDeletableEntityFromData(id string, createdAt, updatedAt time.Time, deletedAt *time.Time) *SoftDeletableEntity {
	return &SoftDeletableEntity{
		AuditableEntity: *NewAuditableEntityFromData(id, createdAt, updatedAt),
		DeletedAt:       deletedAt,
	}
}

func (e *SoftDeletableEntity) IsDeleted() bool {
	return e.DeletedAt != nil
}

func (e *SoftDeletableEntity) SoftDelete() {
	now := time.Now()
	e.DeletedAt = &now
	e.Touch()
}

func (e *SoftDeletableEntity) Restore() {
	e.DeletedAt = nil
	e.Touch()
}

package entity

import "time"

type AuditableEntity struct {
	Entity
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func NewAuditableEntity(id string) *AuditableEntity {
	now := time.Now()
	return &AuditableEntity{
		Entity:    Entity{ID: id},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func NewAuditableEntityFromData(id string, createdAt, updatedAt time.Time) *AuditableEntity {
	return &AuditableEntity{
		Entity:    Entity{ID: id},
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

func (e *AuditableEntity) Touch() {
	e.UpdatedAt = time.Now()
}

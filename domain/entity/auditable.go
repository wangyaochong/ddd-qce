package entity

import "time"

type AuditableEntity struct {
	Entity
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewAuditableEntity(id string) *AuditableEntity {
	now := time.Now()
	return &AuditableEntity{
		Entity:    Entity{id: id},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func (e *AuditableEntity) Touch() {
	e.UpdatedAt = time.Now()
}

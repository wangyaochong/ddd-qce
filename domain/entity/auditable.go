package entity

import "time"

type AuditableEntity struct {
	Entity
	createdAt time.Time `json:"createdAt"`
	updatedAt time.Time `json:"updatedAt"`
}

func NewAuditableEntity(id string) *AuditableEntity {
	now := time.Now()
	return &AuditableEntity{
		Entity:    Entity{id: id},
		createdAt: now,
		updatedAt: now,
	}
}

func NewAuditableEntityFromData(id string, createdAt, updatedAt time.Time) *AuditableEntity {
	return &AuditableEntity{
		Entity:    Entity{id: id},
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

func (e *AuditableEntity) CreatedAt() time.Time {
	return e.createdAt
}

func (e *AuditableEntity) UpdatedAt() time.Time {
	return e.updatedAt
}

func (e *AuditableEntity) Touch() {
	e.updatedAt = time.Now()
}

package entity

import (
	"encoding/json"
	"time"
)

type AuditableEntity struct {
	Entity
	createdAt time.Time
	updatedAt time.Time
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

type auditableJSON struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (e *AuditableEntity) MarshalJSON() ([]byte, error) {
	return json.Marshal(&auditableJSON{
		ID:        e.id,
		CreatedAt: e.createdAt,
		UpdatedAt: e.updatedAt,
	})
}

func (e *AuditableEntity) UnmarshalJSON(data []byte) error {
	var v auditableJSON
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	e.id = v.ID
	e.createdAt = v.CreatedAt
	e.updatedAt = v.UpdatedAt
	return nil
}

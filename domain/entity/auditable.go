package entity

import (
	"encoding/json"
	"fmt"
	"time"
)

type AuditableEntity struct {
	Entity
	createdAt time.Time
	updatedAt time.Time
}

func (e *AuditableEntity) CreatedAt() time.Time { return e.createdAt }
func (e *AuditableEntity) UpdatedAt() time.Time { return e.updatedAt }

func NewAuditableEntity(id string) (*AuditableEntity, error) {
	e, err := NewEntity(id)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	return &AuditableEntity{
		Entity:    *e,
		createdAt: now,
		updatedAt: now,
	}, nil
}

func NewAuditableEntityFromData(id string, createdAt, updatedAt time.Time) (*AuditableEntity, error) {
	e, err := NewEntity(id)
	if err != nil {
		return nil, err
	}
	return &AuditableEntity{
		Entity:    *e,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}, nil
}

func (e *AuditableEntity) Touch() {
	e.updatedAt = time.Now()
}

func (e *AuditableEntity) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ID        string    `json:"id"`
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
	}{
		ID:        e.id,
		CreatedAt: e.createdAt,
		UpdatedAt: e.updatedAt,
	})
}

func (e *AuditableEntity) UnmarshalJSON(data []byte) error {
	var aux struct {
		ID        string    `json:"id"`
		CreatedAt time.Time `json:"createdAt"`
		UpdatedAt time.Time `json:"updatedAt"`
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
	return nil
}

type AuditableEntityJSON struct {
	EntityJSON
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func (e *AuditableEntity) ToJSON() AuditableEntityJSON {
	return AuditableEntityJSON{
		EntityJSON: e.Entity.ToJSON(),
		CreatedAt:  e.createdAt,
		UpdatedAt:  e.updatedAt,
	}
}

func (e *AuditableEntity) FromJSON(j AuditableEntityJSON) {
	e.Entity.FromJSON(j.EntityJSON)
	e.createdAt = j.CreatedAt
	e.updatedAt = j.UpdatedAt
}

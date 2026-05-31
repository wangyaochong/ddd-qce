package entity

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

type Entity struct {
	id string
}

func NewEntity(id string) (*Entity, error) {
	if id == "" {
		return nil, fmt.Errorf("entity: id is required")
	}
	return &Entity{id: id}, nil
}

func MustNewEntity(id string) *Entity {
	e, err := NewEntity(id)
	if err != nil {
		panic(err)
	}
	return e
}

func (e *Entity) ID() string {
	return e.id
}

func (e *Entity) Equals(other *Entity) bool {
	if e == nil || other == nil {
		return e == other
	}
	return e.id == other.id
}

func (e *Entity) IsEmpty() bool {
	return e.id == ""
}

func (e *Entity) Validate() error {
	if e.id == "" {
		return fmt.Errorf("entity ID cannot be empty")
	}
	return nil
}

func (e *Entity) Clone() *Entity {
	if e == nil {
		return nil
	}
	clone := *e
	return &clone
}

func (e *Entity) MarshalJSON() ([]byte, error) {
	return json.Marshal(&struct {
		ID string `json:"id"`
	}{
		ID: e.id,
	})
}

func (e *Entity) UnmarshalJSON(data []byte) error {
	var aux struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if aux.ID == "" {
		return fmt.Errorf("entity: id is required")
	}
	e.id = aux.ID
	return nil
}

type EntityJSON struct {
	ID string `json:"id"`
}

func (e *Entity) ToJSON() EntityJSON {
	return EntityJSON{ID: e.id}
}

func (e *Entity) FromJSON(j EntityJSON) {
	e.id = j.ID
}

func NewEntityWithID() *Entity {
	id := uuid.New()
	return MustNewEntity(hex.EncodeToString(id[:]))
}

func NewAuditableEntityWithID(id string) *AuditableEntity {
	return MustNewAuditableEntity(id)
}

func MustNewAuditableEntity(id string) *AuditableEntity {
	e, err := NewAuditableEntity(id)
	if err != nil {
		panic(err)
	}
	return e
}

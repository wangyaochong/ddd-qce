package entity

import (
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

// Entity is the base struct for all DDD domain entities.
// It holds a unique string identity and provides equality,
// validation, cloning, and JSON serialization support.
type Entity struct {
	id string
}

// NewEntity creates an Entity with the given identity string.
// Returns an error if id is empty.
func NewEntity(id string) (*Entity, error) {
	if id == "" {
		return nil, fmt.Errorf("entity: id is required")
	}
	return &Entity{id: id}, nil
}

// MustNewEntity creates an Entity with the given identity, panicking if id is empty.
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

// Equals returns true if both entities have the same ID. Nil-safe.
func (e *Entity) Equals(other *Entity) bool {
	if e == nil || other == nil {
		return e == other
	}
	return e.id == other.id
}

// IsEmpty returns true if the entity's ID is the zero value.
func (e *Entity) IsEmpty() bool {
	return e.id == ""
}

// Validate returns an error if the entity's ID is empty.
func (e *Entity) Validate() error {
	if e.id == "" {
		return fmt.Errorf("entity ID cannot be empty")
	}
	return nil
}

// Clone returns a shallow copy of the entity. Returns nil if the receiver is nil.
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

// EntityJSON is the JSON representation of Entity.
type EntityJSON struct {
	ID string `json:"id"`
}

// ToJSON converts the entity to its JSON representation struct.
// ToJSON converts the entity to its JSON representation struct.
func (e *Entity) ToJSON() EntityJSON {
	return EntityJSON{ID: e.id}
}

// FromJSON restores entity fields from the JSON representation struct.
func (e *Entity) FromJSON(j EntityJSON) {
	e.id = j.ID
}

// NewEntityWithID creates an Entity with a randomly generated UUID as its identity.
func NewEntityWithID() *Entity {
	id := uuid.New()
	return MustNewEntity(hex.EncodeToString(id[:]))
}

// NewAuditableEntityWithID creates an AuditableEntity with the given identity.
// Deprecated: Use MustNewAuditableEntity instead.
func NewAuditableEntityWithID(id string) *AuditableEntity {
	return MustNewAuditableEntity(id)
}

// MustNewAuditableEntity creates an AuditableEntity with the given identity, panicking on error.
func MustNewAuditableEntity(id string) *AuditableEntity {
	e, err := NewAuditableEntity(id)
	if err != nil {
		panic(err)
	}
	return e
}

package entity

import (
	"encoding/json"
	"fmt"
)

type Entity struct {
	id string
}

func NewEntity(id string) *Entity {
	return &Entity{id: id}
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
	e.id = aux.ID
	return nil
}

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

type entityJSON struct {
	ID string `json:"id"`
}

func (e *Entity) MarshalJSON() ([]byte, error) {
	return json.Marshal(&entityJSON{ID: e.id})
}

func (e *Entity) UnmarshalJSON(data []byte) error {
	var v entityJSON
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	e.id = v.ID
	return nil
}

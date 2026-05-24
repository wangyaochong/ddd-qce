package entity

import "fmt"

type Entity struct {
	ID string `json:"id"`
}

func NewEntity(id string) *Entity {
	return &Entity{ID: id}
}

func (e *Entity) Equals(other *Entity) bool {
	if e == nil || other == nil {
		return e == other
	}
	return e.ID == other.ID
}

func (e *Entity) IsEmpty() bool {
	return e.ID == ""
}

func (e *Entity) Validate() error {
	if e.ID == "" {
		return fmt.Errorf("entity ID cannot be empty")
	}
	return nil
}

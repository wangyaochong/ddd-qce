package entity

import "fmt"

type Entity struct {
	id string
}

func NewEntity(id string) *Entity {
	return &Entity{id: id}
}

func (e *Entity) GetID() string {
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

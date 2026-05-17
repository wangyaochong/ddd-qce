package entity

type Entity struct {
	ID string
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

package entity

import (
	"encoding/hex"

	"github.com/google/uuid"
)

type IDGenerator func() string

func DefaultIDGenerator() IDGenerator {
	return func() string {
		id := uuid.New()
		return hex.EncodeToString(id[:])
	}
}

func NewEntityWithID() *Entity {
	id := uuid.New()
	return &Entity{id: hex.EncodeToString(id[:])}
}

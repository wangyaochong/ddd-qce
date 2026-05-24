package entity

import (
	"sync"

	"github.com/google/uuid"
)

type IDGenerator func() string

type idGeneratorHolder struct {
	mu  sync.RWMutex
	gen IDGenerator
}

var idGenHolder = &idGeneratorHolder{ //nolint:gochecknoglobals // protected singleton for configurable ID generation
	gen: func() string {
		return uuid.New().String()
	},
}

func DefaultIDGenerator() IDGenerator {
	idGenHolder.mu.RLock()
	defer idGenHolder.mu.RUnlock()
	return idGenHolder.gen
}

func NewEntityWithID() *Entity {
	idGenHolder.mu.RLock()
	gen := idGenHolder.gen
	idGenHolder.mu.RUnlock()
	return &Entity{ID: gen()}
}

func SetIDGenerator(gen IDGenerator) {
	if gen == nil {
		return
	}
	idGenHolder.mu.Lock()
	idGenHolder.gen = gen
	idGenHolder.mu.Unlock()
}

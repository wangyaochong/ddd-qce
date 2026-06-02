package infra

import (
	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/impl/memory"
	"github.com/ddd-qce/core/cqrs/query"
)

// BusFactory creates command, query, and event buses wired with an aspect chain.
type BusFactory interface {
	CreateCommandBus(chain *aspect.AspectChain) command.CommandBus
	CreateQueryBus(chain *aspect.AspectChain) query.QueryBus
	CreateEventBus(chain *aspect.AspectChain) event.EventBus
}

type memoryBusFactory struct{}

// NewMemoryBusFactory returns a BusFactory that creates in-memory buses.
func NewMemoryBusFactory() BusFactory {
	return &memoryBusFactory{}
}

func (f *memoryBusFactory) CreateCommandBus(chain *aspect.AspectChain) command.CommandBus {
	return memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
}

func (f *memoryBusFactory) CreateQueryBus(chain *aspect.AspectChain) query.QueryBus {
	return memory.NewQueryBus(memory.WithQueryBusAspectChain(chain))
}

func (f *memoryBusFactory) CreateEventBus(chain *aspect.AspectChain) event.EventBus {
	return memory.NewEventBus(memory.WithEventBusAspectChain(chain))
}

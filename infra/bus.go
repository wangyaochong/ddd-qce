package infra

import (
	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	"github.com/ddd-qce/core/cqrs/impl/memory"
)

type BusFactory struct {
	NewCommandBus func(chain *aspect.AspectChain) command.CommandBus
	NewQueryBus   func(chain *aspect.AspectChain) query.QueryBus
	NewEventBus   func(chain *aspect.AspectChain) event.EventBus
}

func NewMemoryBusFactory() *BusFactory {
	return &BusFactory{
		NewCommandBus: func(chain *aspect.AspectChain) command.CommandBus {
			return memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
		},
		NewQueryBus: func(chain *aspect.AspectChain) query.QueryBus {
			return memory.NewQueryBus(memory.WithQueryBusAspectChain(chain))
		},
		NewEventBus: func(chain *aspect.AspectChain) event.EventBus {
			return memory.NewEventBus(memory.WithBusAspectChain(chain))
		},
	}
}

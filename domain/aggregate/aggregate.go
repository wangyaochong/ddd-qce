package aggregate

import (
	"fmt"

	"github.com/ddd-qce/core/domain/event"
)

type AggregateRoot struct {
	ID              string
	Version         int
	uncommittedEvents []event.DomainEvent
}

func NewAggregateRoot(id string) *AggregateRoot {
	return &AggregateRoot{
		ID:      id,
		Version: 0,
	}
}

func (a *AggregateRoot) Apply(evt event.DomainEvent) {
	a.uncommittedEvents = append(a.uncommittedEvents, evt)
	a.Version++
}

func (a *AggregateRoot) UncommittedEvents() []event.DomainEvent {
	evts := make([]event.DomainEvent, len(a.uncommittedEvents))
	copy(evts, a.uncommittedEvents)
	return evts
}

func (a *AggregateRoot) MarkEventsAsCommitted() {
	a.uncommittedEvents = nil
}

func (a *AggregateRoot) LoadFromHistory(events []event.DomainEvent) {
	for _, evt := range events {
		a.applyEvent(evt)
	}
}

func (a *AggregateRoot) applyEvent(evt event.DomainEvent) {
	a.Version++
}

type AggregateRootValidator interface {
	Validate() error
}

func (a *AggregateRoot) Validate() error {
	if a.ID == "" {
		return fmt.Errorf("aggregate ID cannot be empty")
	}
	if a.Version < 0 {
		return fmt.Errorf("aggregate version cannot be negative")
	}
	return nil
}

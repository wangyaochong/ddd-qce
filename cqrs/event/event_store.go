package event

import (
	domainevent "github.com/ddd-qce/core/domain/event"
)

type EventStore[T domainevent.DomainEvent] = domainevent.EventStore[T]

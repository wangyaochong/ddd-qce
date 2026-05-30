// Package domainevent defines the core domain event abstraction used by the DDD layer.
//
// The Event interface provides the contract for domain events:
// identity (AggregateID), timing (OccurredAt), and correlation metadata
// (CorrelationID, CausationID). This package has zero external dependencies,
// allowing domain models to use events without pulling in CQRS or infrastructure concerns.
//
// For event construction helpers (WithCorrelation, NewDomainEvent, etc.), see
// github.com/ddd-qce/core/cqrs/event.
package domainevent

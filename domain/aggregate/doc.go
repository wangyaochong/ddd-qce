// Package aggregate provides the AggregateRoot and AggregateRef interfaces
// for DDD aggregates. It supports event sourcing via ApplyChange/ApplyHistory,
// optimistic locking via Version/ExpectedVersion, and JSON serialization.
//
// Embed AggregateRoot in your aggregate types, implement When(evt) to handle
// state mutations, and use Apply(ctx, evt) to apply domain events.
package aggregate

// Package query defines the Query interface, QueryHandler[T,R] interface,
// and QueryBus interface for CQRS read operations.
//
// Use Dispatch[T,R](ctx, bus, q) to execute a query through the bus with
// generic type safety. Queries return results without side effects.
package query

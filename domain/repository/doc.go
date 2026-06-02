// Package repository defines the Repository[T] and EventSourcingRepository[T]
// interfaces for persisting DDD aggregates, plus a JSONSerializer[T] for
// snapshot persistence.
//
// Implementations are provided in infra/repository/memory and infra/repository/pg.
package repository

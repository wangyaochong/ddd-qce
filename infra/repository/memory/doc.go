// Package memory provides in-memory implementations of Repository[T] and
// EventSourcedRepository[T]. InMemoryRepository stores aggregates by ID;
// InMemoryEventSourcedRepository stores aggregates as event snapshots.
//
// Both support generic aggregates and custom SnapshotSerializer[T].
package memory

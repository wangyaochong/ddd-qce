// Package memory provides in-memory implementations of all CQRS buses:
//
//   - CommandBus: single-handler dispatch with aspect chain support
//   - QueryBus: single-handler dispatch with aspect chain support
//   - EventBus: fan-out dispatch to multiple subscribers with panic recovery
//   - EventSourceStore: in-memory event persistence with concurrency checking
//
// Suitable for development, testing, and low-throughput scenarios.
package memory

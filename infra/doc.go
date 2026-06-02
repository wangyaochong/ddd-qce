// Package infra provides the Backend abstraction — a unified dependency
// bundle combining TransactionManager, JobStore, TraceStore, MessageStore,
// Migrator, and BusFactory for infrastructure-level dependency injection.
//
// Use NewMemoryBackend() or NewPgBackend(db) to create a pre-configured
// backend. Backend switching can be driven by DDD_STORE_TYPE env var.
package infra

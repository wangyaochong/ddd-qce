package infra

import (
	"context"
	"fmt"

	"github.com/ddd-qce/core/aspect/builtin"
	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/trace"
)

// Migrator runs schema migrations against the backing store.
type Migrator interface {
	Migrate(ctx context.Context) error
}

// NopMigrator is a no-op Migrator that does nothing.
type NopMigrator struct{}

func (NopMigrator) Migrate(_ context.Context) error { return nil }

// Backend bundles all infrastructure dependencies required by a DDD application.
type Backend struct {
	TransactionManager builtin.TransactionManager
	JobStore           jobcore.JobStore
	TypeRegistry       *jobcore.TypeRegistry
	TraceStore         trace.TraceStore
	MessageStore       builtin.MessageStore
	Migrator           Migrator
	BusFactory         BusFactory
	closer             func() error
}

// Close releases resources held by the backend using the configured closer function.
func (b *Backend) Close() error {
	if b.closer != nil {
		return b.closer()
	}
	return nil
}

// Shutdown gracefully closes the trace store and backend resources.
func (b *Backend) Shutdown(ctx context.Context) error {
	var errs []error
	if b.TraceStore != nil {
		if err := b.TraceStore.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if b.closer != nil {
		if err := b.closer(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("backend shutdown errors: %v", errs)
	}
	return nil
}

// BackendOption configures a Backend during construction.
type BackendOption func(*Backend)

// WithTransactionManager sets the transaction manager for the backend.
func WithTransactionManager(tm builtin.TransactionManager) BackendOption {
	return func(b *Backend) { b.TransactionManager = tm }
}

// WithJobStore sets the job store for the backend.
func WithJobStore(store jobcore.JobStore) BackendOption {
	return func(b *Backend) { b.JobStore = store }
}

// WithTypeRegistry sets the job type registry for the backend.
func WithTypeRegistry(registry *jobcore.TypeRegistry) BackendOption {
	return func(b *Backend) { b.TypeRegistry = registry }
}

// WithTraceStore sets the trace store for the backend.
func WithTraceStore(store trace.TraceStore) BackendOption {
	return func(b *Backend) { b.TraceStore = store }
}

// WithMessageStore sets the message store for the backend.
func WithMessageStore(store builtin.MessageStore) BackendOption {
	return func(b *Backend) { b.MessageStore = store }
}

// WithMigrator sets the schema migrator for the backend.
func WithMigrator(m Migrator) BackendOption {
	return func(b *Backend) { b.Migrator = m }
}

// WithBusFactory sets the bus factory for the backend.
func WithBusFactory(f BusFactory) BackendOption {
	return func(b *Backend) { b.BusFactory = f }
}

// WithCloser sets a cleanup function called on backend close.
func WithCloser(closer func() error) BackendOption {
	return func(b *Backend) { b.closer = closer }
}

// NewBackend creates a Backend with the provided options.
func NewBackend(opts ...BackendOption) *Backend {
	b := &Backend{}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

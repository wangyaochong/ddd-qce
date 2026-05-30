package infra

import (
	"context"
	"fmt"

	"github.com/ddd-qce/core/aspect/builtin"
	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/trace"
)

type Migrator interface {
	Migrate(ctx context.Context) error
}

type NopMigrator struct{}

func (NopMigrator) Migrate(_ context.Context) error { return nil }

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

func (b *Backend) Close() error {
	if b.closer != nil {
		return b.closer()
	}
	return nil
}

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

type BackendOption func(*Backend)

func WithTransactionManager(tm builtin.TransactionManager) BackendOption {
	return func(b *Backend) { b.TransactionManager = tm }
}

func WithJobStore(store jobcore.JobStore) BackendOption {
	return func(b *Backend) { b.JobStore = store }
}

func WithTypeRegistry(registry *jobcore.TypeRegistry) BackendOption {
	return func(b *Backend) { b.TypeRegistry = registry }
}

func WithTraceStore(store trace.TraceStore) BackendOption {
	return func(b *Backend) { b.TraceStore = store }
}

func WithMessageStore(store builtin.MessageStore) BackendOption {
	return func(b *Backend) { b.MessageStore = store }
}

func WithMigrator(m Migrator) BackendOption {
	return func(b *Backend) { b.Migrator = m }
}

func WithBusFactory(f BusFactory) BackendOption {
	return func(b *Backend) { b.BusFactory = f }
}

func WithCloser(closer func() error) BackendOption {
	return func(b *Backend) { b.closer = closer }
}

func NewBackend(opts ...BackendOption) *Backend {
	b := &Backend{}
	for _, opt := range opts {
		opt(b)
	}
	return b
}

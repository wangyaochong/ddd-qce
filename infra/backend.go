package infra

import (
	"context"

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
	closer             func() error
}

func (b *Backend) Close() error {
	if b.closer != nil {
		return b.closer()
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

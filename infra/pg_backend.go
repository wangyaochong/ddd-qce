package infra

import (
	"context"
	"database/sql"

	builtinpg "github.com/ddd-qce/core/aspect/builtin/pg"
	jobpg "github.com/ddd-qce/core/job/pg"
	corepg "github.com/ddd-qce/core/pg"
	tracepg "github.com/ddd-qce/core/trace/pg"
)

type pgMigrator struct {
	db *sql.DB
}

func (m *pgMigrator) Migrate(_ context.Context) error {
	return corepg.Migrate(m.db)
}

func NewPgBackend(db *sql.DB, opts ...BackendOption) *Backend {
	defaults := []BackendOption{
		WithTransactionManager(corepg.NewTransactionManager(db)),
		WithJobStore(jobpg.NewJobStore(db)),
		WithTraceStore(tracepg.NewTraceStore(db)),
		WithMessageStore(builtinpg.NewMessageStore(db)),
		WithMigrator(&pgMigrator{db: db}),
	}
	return NewBackend(append(defaults, opts...)...)
}

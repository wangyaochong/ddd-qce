package pgx

import (
	"database/sql"

	pgmsg "github.com/ddd-qce/core/aspect/builtin/pg"
	"github.com/ddd-qce/core/infra"
	pgjob "github.com/ddd-qce/core/job/pg"
	corepg "github.com/ddd-qce/core/pg"
	pgtrace "github.com/ddd-qce/core/trace/pg"
)

func NewPGBackend(db *sql.DB, opts ...infra.BackendOption) *infra.Backend {
	jobStoreOpts := []pgjob.JobStoreOption{}
	defaults := []infra.BackendOption{
		infra.WithTransactionManager(corepg.NewTransactionManager(db)),
		infra.WithJobStore(pgjob.NewJobStore(db, jobStoreOpts...)),
		infra.WithTraceStore(pgtrace.NewTraceStore(db)),
		infra.WithMessageStore(pgmsg.NewMessageStore(db)),
		infra.WithMigrate(func() error { return corepg.Migrate(db) }),
	}
	return infra.NewBackend(append(defaults, opts...)...)
}

package pg

import (
	"context"
	"database/sql"

	"github.com/ddd-qce/core/aspect/builtin"
	corepg "github.com/ddd-qce/core/pg"
)

type PgMessageStore struct {
	db *sql.DB
}

func NewMessageStore(db *sql.DB) *PgMessageStore {
	return &PgMessageStore{db: db}
}

func (s *PgMessageStore) RecordCommand(ctx context.Context, entry *builtin.CommandEntry) error {
	q := corepg.GetQuerier(ctx, s.db)
	_, err := q.ExecContext(ctx,
		`INSERT INTO ddd_command_log (trace_id, span_id, command_type, command_data, result_type, result_data, error, duration_ns, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		entry.TraceID, entry.SpanID, entry.CommandType, entry.CommandData,
		corepg.NullString(entry.ResultType), entry.ResultData, corepg.NullString(entry.Error),
		entry.Duration.Nanoseconds(), entry.CreatedAt,
	)
	return err
}

func (s *PgMessageStore) RecordQuery(ctx context.Context, entry *builtin.QueryEntry) error {
	q := corepg.GetQuerier(ctx, s.db)
	_, err := q.ExecContext(ctx,
		`INSERT INTO ddd_query_log (trace_id, span_id, query_type, query_data, result_type, result_data, error, duration_ns, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		entry.TraceID, entry.SpanID, entry.QueryType, entry.QueryData,
		corepg.NullString(entry.ResultType), entry.ResultData, corepg.NullString(entry.Error),
		entry.Duration.Nanoseconds(), entry.CreatedAt,
	)
	return err
}

func (s *PgMessageStore) RecordEvent(ctx context.Context, entry *builtin.EventEntry) error {
	q := corepg.GetQuerier(ctx, s.db)
	_, err := q.ExecContext(ctx,
		`INSERT INTO ddd_event_log (trace_id, span_id, aggregate_id, event_type, event_data, handler_count, error, duration_ns, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		entry.TraceID, entry.SpanID, entry.AggregateID, entry.EventType,
		entry.EventData, entry.HandlerCount, corepg.NullString(entry.Error),
		entry.Duration.Nanoseconds(), entry.CreatedAt,
	)
	return err
}

func (s *PgMessageStore) RecordEventHandler(ctx context.Context, entry *builtin.EventHandlerEntry) error {
	q := corepg.GetQuerier(ctx, s.db)
	_, err := q.ExecContext(ctx,
		`INSERT INTO ddd_event_handler_log (trace_id, span_id, aggregate_id, event_type, handler_type, status, error, duration_ns, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		entry.TraceID, entry.SpanID, entry.AggregateID, entry.EventType,
		entry.HandlerType, entry.Status, corepg.NullString(entry.Error),
		entry.Duration.Nanoseconds(), entry.CreatedAt,
	)
	return err
}

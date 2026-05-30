package pg

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	ddderror "github.com/ddd-qce/core/error"
	corepg "github.com/ddd-qce/core/pg"
	"github.com/ddd-qce/core/trace"
)

type PgTraceStore struct {
	db *sql.DB
}

func NewTraceStore(db *sql.DB) *PgTraceStore {
	return &PgTraceStore{db: db}
}

func (s *PgTraceStore) Close() error {
	return nil
}

func (s *PgTraceStore) RecordSpan(ctx context.Context, span *trace.Span) error {
	q := corepg.GetQuerier(ctx, s.db)
	_, err := q.ExecContext(ctx,
		`INSERT INTO ddd_spans (id, trace_id, parent_id, type, name, status, error, started_at, duration_ns)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		span.ID, span.TraceID, corepg.NullString(span.ParentID),
		span.Type, span.Name, span.Status, corepg.NullString(span.Error),
		span.StartedAt, span.Duration.Nanoseconds(),
	)
	return err
}

func (s *PgTraceStore) GetTrace(ctx context.Context, traceID string) ([]*trace.Span, error) {
	q := corepg.GetQuerier(ctx, s.db)
	rows, err := q.QueryContext(ctx,
		`SELECT id, trace_id, parent_id, type, name, status, error, started_at, duration_ns
		 FROM ddd_spans WHERE trace_id = $1 ORDER BY started_at`,
		traceID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var spans []*trace.Span
	for rows.Next() {
		span := &trace.Span{}
		var parentID sql.NullString
		var errStr sql.NullString
		var durationNs int64
		if err := rows.Scan(&span.ID, &span.TraceID, &parentID, &span.Type, &span.Name, &span.Status, &errStr, &span.StartedAt, &durationNs); err != nil {
			return nil, err
		}
		if parentID.Valid {
			span.ParentID = parentID.String
		}
		if errStr.Valid {
			span.Error = errStr.String
		}
		span.Duration = time.Duration(durationNs)
		spans = append(spans, span)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spans: %w", err)
	}
	if len(spans) == 0 {
		return nil, fmt.Errorf("trace %s: %w", traceID, ddderror.ErrNotFound)
	}
	return spans, nil
}

func (s *PgTraceStore) ListTraces(ctx context.Context, filter trace.TraceFilter) ([]string, error) {
	q := corepg.GetQuerier(ctx, s.db)
	query := "SELECT DISTINCT trace_id FROM ddd_spans WHERE 1=1"
	var args []any
	argIdx := 1

	if filter.TraceID != "" {
		query += fmt.Sprintf(" AND trace_id = $%d", argIdx)
		args = append(args, filter.TraceID)
		argIdx++
	}
	if filter.Type != "" {
		query += fmt.Sprintf(" AND type = $%d", argIdx)
		args = append(args, filter.Type)
		argIdx++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(" AND status = $%d", argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if !filter.StartTime.IsZero() {
		query += fmt.Sprintf(" AND started_at >= $%d", argIdx)
		args = append(args, filter.StartTime)
		argIdx++
	}
	if !filter.EndTime.IsZero() {
		query += fmt.Sprintf(" AND started_at <= $%d", argIdx)
		args = append(args, filter.EndTime)
		argIdx++
	}
	if filter.NameContains != "" {
		query += fmt.Sprintf(" AND name LIKE $%d ESCAPE '\\'", argIdx)
		args = append(args, "%"+escapeLike(filter.NameContains)+"%")
		argIdx++
	}

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traceIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		traceIDs = append(traceIDs, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate traces: %w", err)
	}
	return traceIDs, nil
}

func escapeLike(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `%`, `\%`)
	s = strings.ReplaceAll(s, `_`, `\_`)
	return s
}

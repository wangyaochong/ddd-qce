package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/observability"
	corepg "github.com/ddd-qce/core/pg"
)

type PgMessageStoreReader struct {
	db *sql.DB
}

func NewMessageStoreReader(db *sql.DB) *PgMessageStoreReader {
	return &PgMessageStoreReader{db: db}
}

func (r *PgMessageStoreReader) QueryCommands(ctx context.Context, filter observability.MessageFilter) ([]builtin.CommandEntry, error) {
	q := corepg.GetQuerier(ctx, r.db)

	where, args := buildWhereClause([]wherePart{
		{"command_type", filter.Type},
		{"trace_id", filter.TraceID},
	}, filter.Status, filter.Since)
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit)

	query := fmt.Sprintf(
		`SELECT trace_id, span_id, command_type, command_data, result_type, result_data, error, duration_ns, created_at
		 FROM ddd_command_log%s ORDER BY created_at DESC LIMIT $%d`,
		where, len(args),
	)

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query commands: %w", err)
	}
	defer rows.Close()

	var result []builtin.CommandEntry
	for rows.Next() {
		var e builtin.CommandEntry
		var durationNs int64
		if err := rows.Scan(&e.TraceID, &e.SpanID, &e.CommandType, &e.CommandData, &e.ResultType, &e.ResultData, &e.Error, &durationNs, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan command: %w", err)
		}
		e.Duration = time.Duration(durationNs)
		result = append(result, e)
	}
	return result, rows.Err()
}

func (r *PgMessageStoreReader) QueryQueries(ctx context.Context, filter observability.MessageFilter) ([]builtin.QueryEntry, error) {
	q := corepg.GetQuerier(ctx, r.db)

	where, args := buildWhereClause([]wherePart{
		{"query_type", filter.Type},
		{"trace_id", filter.TraceID},
	}, filter.Status, filter.Since)
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit)

	query := fmt.Sprintf(
		`SELECT trace_id, span_id, query_type, query_data, result_type, result_data, error, duration_ns, created_at
		 FROM ddd_query_log%s ORDER BY created_at DESC LIMIT $%d`,
		where, len(args),
	)

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query queries: %w", err)
	}
	defer rows.Close()

	var result []builtin.QueryEntry
	for rows.Next() {
		var e builtin.QueryEntry
		var durationNs int64
		if err := rows.Scan(&e.TraceID, &e.SpanID, &e.QueryType, &e.QueryData, &e.ResultType, &e.ResultData, &e.Error, &durationNs, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan query: %w", err)
		}
		e.Duration = time.Duration(durationNs)
		result = append(result, e)
	}
	return result, rows.Err()
}

func (r *PgMessageStoreReader) QueryEvents(ctx context.Context, filter observability.MessageFilter) ([]builtin.EventEntry, error) {
	q := corepg.GetQuerier(ctx, r.db)

	parts := []wherePart{
		{"event_type", filter.Type},
		{"trace_id", filter.TraceID},
	}
	if filter.AggregateID != "" {
		parts = append(parts, wherePart{"aggregate_id", filter.AggregateID})
	}
	where, args := buildWhereClause(parts, filter.Status, filter.Since)
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	args = append(args, limit)

	query := fmt.Sprintf(
		`SELECT trace_id, span_id, aggregate_id, event_type, event_data, handler_count, error, duration_ns, created_at
		 FROM ddd_event_log%s ORDER BY created_at DESC LIMIT $%d`,
		where, len(args),
	)

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var result []builtin.EventEntry
	for rows.Next() {
		var e builtin.EventEntry
		var durationNs int64
		var eventData json.RawMessage
		if err := rows.Scan(&e.TraceID, &e.SpanID, &e.AggregateID, &e.EventType, &eventData, &e.HandlerCount, &e.Error, &durationNs, &e.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		e.EventData = eventData
		e.Duration = time.Duration(durationNs)
		result = append(result, e)
	}
	return result, rows.Err()
}

type wherePart struct {
	column string
	value  string
}

func buildWhereClause(parts []wherePart, status string, since time.Time) (string, []any) {
	var conds []string
	var args []any
	idx := 1

	for _, p := range parts {
		if p.value != "" {
			conds = append(conds, fmt.Sprintf("%s = $%d", p.column, idx))
			args = append(args, p.value)
			idx++
		}
	}

	if status == "error" {
		conds = append(conds, "error != ''")
	} else if status == "success" {
		conds = append(conds, "error = ''")
	}

	if !since.IsZero() {
		conds = append(conds, fmt.Sprintf("created_at >= $%d", idx))
		args = append(args, since)
		idx++
	}

	if len(conds) == 0 {
		return "", args
	}
	return " WHERE " + strings.Join(conds, " AND "), args
}

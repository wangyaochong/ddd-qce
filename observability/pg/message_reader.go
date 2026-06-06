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

func (r *PgMessageStoreReader) QueryCommands(ctx context.Context, filter observability.MessageFilter) (observability.QueryResult[builtin.CommandEntry], error) {
	q := corepg.GetQuerier(ctx, r.db)

	where, args := buildWhereClause([]wherePart{
		{"command_type", filter.Type},
		{"trace_id", filter.TraceID},
	}, filter.Status, filter.Since)

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM ddd_command_log%s", where)
	var total int
	if err := q.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return observability.QueryResult[builtin.CommandEntry]{}, fmt.Errorf("count commands: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset
	queryArgs := append(args, limit, offset)

	query := fmt.Sprintf(
		`SELECT trace_id, span_id, command_type, command_data, result_type, result_data, error, duration_ns, created_at
		 FROM ddd_command_log%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(queryArgs)-1, len(queryArgs),
	)

	rows, err := q.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return observability.QueryResult[builtin.CommandEntry]{}, fmt.Errorf("query commands: %w", err)
	}
	defer rows.Close()

	var items []builtin.CommandEntry
	for rows.Next() {
		var e builtin.CommandEntry
		var traceID, spanID, resultType sql.NullString
		var commandData, resultData json.RawMessage
		var errMsg sql.NullString
		var durationNs sql.NullInt64
		if err := rows.Scan(&traceID, &spanID, &e.CommandType, &commandData, &resultType, &resultData, &errMsg, &durationNs, &e.CreatedAt); err != nil {
			return observability.QueryResult[builtin.CommandEntry]{}, fmt.Errorf("scan command: %w", err)
		}
		e.TraceID = traceID.String
		e.SpanID = spanID.String
		e.CommandData = commandData
		e.ResultType = resultType.String
		e.ResultData = resultData
		e.Error = errMsg.String
		if durationNs.Valid {
			e.Duration = time.Duration(durationNs.Int64)
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return observability.QueryResult[builtin.CommandEntry]{}, err
	}
	return observability.QueryResult[builtin.CommandEntry]{Items: items, Total: total}, nil
}

func (r *PgMessageStoreReader) QueryQueries(ctx context.Context, filter observability.MessageFilter) (observability.QueryResult[builtin.QueryEntry], error) {
	q := corepg.GetQuerier(ctx, r.db)

	where, args := buildWhereClause([]wherePart{
		{"query_type", filter.Type},
		{"trace_id", filter.TraceID},
	}, filter.Status, filter.Since)

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM ddd_query_log%s", where)
	var total int
	if err := q.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return observability.QueryResult[builtin.QueryEntry]{}, fmt.Errorf("count queries: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset
	queryArgs := append(args, limit, offset)

	query := fmt.Sprintf(
		`SELECT trace_id, span_id, query_type, query_data, result_type, result_data, error, duration_ns, created_at
		 FROM ddd_query_log%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(queryArgs)-1, len(queryArgs),
	)

	rows, err := q.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return observability.QueryResult[builtin.QueryEntry]{}, fmt.Errorf("query queries: %w", err)
	}
	defer rows.Close()

	var items []builtin.QueryEntry
	for rows.Next() {
		var e builtin.QueryEntry
		var traceID, spanID, resultType sql.NullString
		var queryData json.RawMessage
		var resultData json.RawMessage
		var errMsg sql.NullString
		var durationNs sql.NullInt64
		if err := rows.Scan(&traceID, &spanID, &e.QueryType, &queryData, &resultType, &resultData, &errMsg, &durationNs, &e.CreatedAt); err != nil {
			return observability.QueryResult[builtin.QueryEntry]{}, fmt.Errorf("scan query: %w", err)
		}
		e.TraceID = traceID.String
		e.SpanID = spanID.String
		e.QueryData = queryData
		e.ResultType = resultType.String
		e.ResultData = resultData
		e.Error = errMsg.String
		if durationNs.Valid {
			e.Duration = time.Duration(durationNs.Int64)
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return observability.QueryResult[builtin.QueryEntry]{}, err
	}
	return observability.QueryResult[builtin.QueryEntry]{Items: items, Total: total}, nil
}

func (r *PgMessageStoreReader) QueryEvents(ctx context.Context, filter observability.MessageFilter) (observability.QueryResult[builtin.EventEntry], error) {
	q := corepg.GetQuerier(ctx, r.db)

	parts := []wherePart{
		{"event_type", filter.Type},
		{"trace_id", filter.TraceID},
	}
	if filter.AggregateID != "" {
		parts = append(parts, wherePart{"aggregate_id", filter.AggregateID})
	}
	where, args := buildWhereClause(parts, filter.Status, filter.Since)

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM ddd_event_log%s", where)
	var total int
	if err := q.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return observability.QueryResult[builtin.EventEntry]{}, fmt.Errorf("count events: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset
	queryArgs := append(args, limit, offset)

	query := fmt.Sprintf(
		`SELECT trace_id, span_id, aggregate_id, event_type, event_data, handler_count, error, duration_ns, created_at
		 FROM ddd_event_log%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(queryArgs)-1, len(queryArgs),
	)

	rows, err := q.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return observability.QueryResult[builtin.EventEntry]{}, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var items []builtin.EventEntry
	for rows.Next() {
		var e builtin.EventEntry
		var traceID, spanID, aggregateID sql.NullString
		var eventData json.RawMessage
		var handlerCount sql.NullInt64
		var errMsg sql.NullString
		var durationNs sql.NullInt64
		if err := rows.Scan(&traceID, &spanID, &aggregateID, &e.EventType, &eventData, &handlerCount, &errMsg, &durationNs, &e.CreatedAt); err != nil {
			return observability.QueryResult[builtin.EventEntry]{}, fmt.Errorf("scan event: %w", err)
		}
		e.TraceID = traceID.String
		e.SpanID = spanID.String
		e.AggregateID = aggregateID.String
		e.EventData = eventData
		if handlerCount.Valid {
			e.HandlerCount = int(handlerCount.Int64)
		}
		e.Error = errMsg.String
		if durationNs.Valid {
			e.Duration = time.Duration(durationNs.Int64)
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return observability.QueryResult[builtin.EventEntry]{}, err
	}
	return observability.QueryResult[builtin.EventEntry]{Items: items, Total: total}, nil
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
		conds = append(conds, "(error IS NOT NULL AND error != '')")
	} else if status == "success" {
		conds = append(conds, "(error IS NULL OR error = '')")
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

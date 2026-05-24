package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	ddderror "github.com/ddd-qce/core/error"
	"github.com/ddd-qce/core/domain/event"
	corepg "github.com/ddd-qce/core/pg"
)

type EventStore[T event.DomainEvent] struct {
	db      *sql.DB
	pool    sync.Pool
	newFunc func() T
}

type EventStoreOption[T event.DomainEvent] func(*EventStore[T])

func WithFactory[T event.DomainEvent](factory func() T) EventStoreOption[T] {
	return func(s *EventStore[T]) { s.newFunc = factory }
}

func NewEventStore[T event.DomainEvent](db *sql.DB, opts ...EventStoreOption[T]) (*EventStore[T], error) {
	var zero T
	t := reflect.TypeOf(zero)
	if t == nil || t.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("PgEventStore[T]: T must be a pointer type, got %v", t)
	}

	s := &EventStore[T]{
		db: db,
	}

	for _, opt := range opts {
		opt(s)
	}

	if s.newFunc != nil {
		return s, nil
	}

	s.pool = sync.Pool{
		New: func() any {
			return reflect.New(t.Elem()).Interface()
		},
	}

	if v := s.pool.Get(); v != nil {
		if _, ok := v.(T); !ok {
			return nil, fmt.Errorf("PgEventStore[T]: pool New returned unexpected type %T, expected %T", v, zero)
		}
		s.pool.Put(v)
	}

	return s, nil
}

func isUniqueViolation(err error) bool {
	if sq, ok := err.(interface{ SQLState() string }); ok {
		return sq.SQLState() == "23505"
	}
	return false
}

func (s *EventStore[T]) alloc() (T, error) {
	if s.newFunc != nil {
		return s.newFunc(), nil
	}
	v, ok := s.pool.Get().(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("PgEventStore[T]: pool returned unexpected type, expected %T", zero)
	}
	return v, nil
}

func (s *EventStore[T]) Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error {
	q := corepg.GetQuerier(ctx, s.db)
	for i, evt := range events {
		data, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}
		version := expectedVersion + i + 1
		_, err = q.ExecContext(ctx,
			`INSERT INTO ddd_domain_events (aggregate_id, event_type, event_data, occurred_at, version)
			 VALUES ($1, $2, $3, $4, $5)`,
			evt.AggregateID(), evt.EventType(), data, evt.OccurredAt(), version,
		)
		if err != nil {
			if isUniqueViolation(err) {
				return fmt.Errorf("concurrency conflict: version %d already exists for aggregate %s: %w", version, aggregateID, ddderror.ErrConcurrency)
			}
			return fmt.Errorf("insert event: %w", err)
		}
	}
	return nil
}

func (s *EventStore[T]) Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error) {
	q := corepg.GetQuerier(ctx, s.db)
	rows, err := q.QueryContext(ctx,
		`SELECT event_data FROM ddd_domain_events
		 WHERE aggregate_id = $1 AND version > $2
		 ORDER BY version`,
		aggregateID, afterVersion,
	)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	result := make([]T, 0)
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		evt, err := s.alloc()
		if err != nil {
			return nil, fmt.Errorf("allocate event: %w", err)
		}
		if err := json.Unmarshal(data, evt); err != nil {
			return nil, fmt.Errorf("unmarshal event: %w", err)
		}
		result = append(result, evt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return result, nil
}

package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"

	"github.com/ddd-qce/core/domain/event"
	corepg "github.com/ddd-qce/core/pg"
)

type EventStore[T event.DomainEvent] struct {
	db      *sql.DB
	once    sync.Once
	pool    sync.Pool
	newFunc func() T
}

type EventStoreOption[T event.DomainEvent] func(*EventStore[T])

func WithFactory[T event.DomainEvent](factory func() T) EventStoreOption[T] {
	return func(s *EventStore[T]) { s.newFunc = factory }
}

func NewEventStore[T event.DomainEvent](db *sql.DB, opts ...EventStoreOption[T]) *EventStore[T] {
	s := &EventStore[T]{
		db: db,
		pool: sync.Pool{
			New: func() any {
				var zero T
				v := reflect.New(reflect.TypeOf(zero).Elem()).Interface()
				typed, ok := v.(T)
				if !ok {
					panic(fmt.Sprintf("PgEventStore[T]: pool New returned unexpected type %T, expected %T", v, zero))
				}
				return typed
			},
		},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func isUniqueViolation(err error) bool {
	if sq, ok := err.(interface{ SQLState() string }); ok {
		return sq.SQLState() == "23505"
	}
	return false
}

func (s *EventStore[T]) assertPointerType() {
	s.once.Do(func() {
		var zero T
		if reflect.TypeOf(zero).Kind() != reflect.Ptr {
			panic(fmt.Sprintf("PgEventStore[T]: T must be a pointer type, got %v", reflect.TypeOf(zero)))
		}
	})
}

func (s *EventStore[T]) alloc() T {
	if s.newFunc != nil {
		return s.newFunc()
	}
	v, ok := s.pool.Get().(T)
	if !ok {
		var zero T
		panic(fmt.Sprintf("PgEventStore[T]: pool returned unexpected type, expected %T", zero))
	}
	return v
}

func (s *EventStore[T]) Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error {
	s.assertPointerType()
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
				return fmt.Errorf("concurrency conflict: version %d already exists for aggregate %s", version, aggregateID)
			}
			return fmt.Errorf("insert event: %w", err)
		}
	}
	return nil
}

func (s *EventStore[T]) Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error) {
	s.assertPointerType()
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

	var result []T
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		evt := s.alloc()
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

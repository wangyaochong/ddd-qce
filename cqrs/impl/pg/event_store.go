package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/ddd-qce/core/cqrs/event"
	domainevent "github.com/ddd-qce/core/domain/event"
	ddderror "github.com/ddd-qce/core/error"
	corepg "github.com/ddd-qce/core/pg"
)

type EventSourceStore[T domainevent.Event] struct {
	db         *sql.DB
	pool       sync.Pool
	newFunc    func() T
	factoryMap map[string]func() T
}

type EventSourceStoreOption[T domainevent.Event] func(*EventSourceStore[T])

func WithFactory[T domainevent.Event](factory func() T) EventSourceStoreOption[T] {
	return func(s *EventSourceStore[T]) { s.newFunc = factory }
}

func WithFactoryMap[T domainevent.Event](m map[string]func() T) EventSourceStoreOption[T] {
	return func(s *EventSourceStore[T]) {
		if s.factoryMap == nil {
			s.factoryMap = make(map[string]func() T, len(m))
		}
		for k, v := range m {
			s.factoryMap[k] = v
		}
	}
}

func NewEventSourceStore[T domainevent.Event](db *sql.DB, opts ...EventSourceStoreOption[T]) (*EventSourceStore[T], error) {
	var zero T
	t := reflect.TypeOf(zero)

	s := &EventSourceStore[T]{
		db: db,
	}

	for _, opt := range opts {
		opt(s)
	}

	if t == nil {
		if s.newFunc == nil && s.factoryMap == nil {
			return nil, fmt.Errorf("PgEventSourceStore[T]: WithFactory or WithFactoryMap is required when T is an interface type")
		}
		return s, nil
	}

	if t.Kind() != reflect.Ptr {
		return nil, fmt.Errorf("PgEventSourceStore[T]: T must be a pointer type, got %v", t)
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
			return nil, fmt.Errorf("PgEventSourceStore[T]: pool New returned unexpected type %T, expected %T", v, zero)
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

func (s *EventSourceStore[T]) alloc(eventType string) (T, error) {
	if s.factoryMap != nil {
		if fn, ok := s.factoryMap[eventType]; ok {
			return fn(), nil
		}
	}
	if s.newFunc != nil {
		return s.newFunc(), nil
	}
	v, ok := s.pool.Get().(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("PgEventSourceStore[T]: pool returned unexpected type, expected %T", zero)
	}
	return v, nil
}

func (s *EventSourceStore[T]) Append(ctx context.Context, aggregateID string, expectedVersion int, events []T) error {
	if corepg.HasTransaction(ctx) {
		return s.appendEvents(ctx, corepg.GetQuerier(ctx, s.db), aggregateID, expectedVersion, events)
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := s.appendEvents(ctx, tx, aggregateID, expectedVersion, events); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *EventSourceStore[T]) appendEvents(ctx context.Context, q corepg.DBTX, aggregateID string, expectedVersion int, events []T) error {
	for i, evt := range events {
		data, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("marshal event: %w", err)
		}
		version := expectedVersion + i + 1
		_, err = q.ExecContext(ctx,
			`INSERT INTO ddd_domain_events (aggregate_id, event_type, event_data, occurred_at, version, correlation_id, causation_id)
			 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			evt.AggregateID(), event.EventNameOf(evt), data, evt.OccurredAt(), version,
			evt.CorrelationID(), evt.CausationID(),
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

func (s *EventSourceStore[T]) Load(ctx context.Context, aggregateID string, afterVersion int) ([]T, error) {
	q := corepg.GetQuerier(ctx, s.db)
	rows, err := q.QueryContext(ctx,
		`SELECT event_type, event_data, aggregate_id, occurred_at, correlation_id, causation_id FROM ddd_domain_events
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
		var eventType string
		var data []byte
		var aggID string
		var occurredAt time.Time
		var correlationID string
		var causationID string
		if err := rows.Scan(&eventType, &data, &aggID, &occurredAt, &correlationID, &causationID); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		evt, err := s.alloc(eventType)
		if err != nil {
			return nil, fmt.Errorf("allocate event: %w", err)
		}
		if err := json.Unmarshal(data, evt); err != nil {
			return nil, fmt.Errorf("unmarshal event: %w", err)
		}
		restoreBaseEvent(evt, aggID, occurredAt, correlationID, causationID)
		result = append(result, evt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return result, nil
}

func (s *EventSourceStore[T]) LoadAll(ctx context.Context, afterPosition int64, limit int) ([]event.GlobalEvent[T], error) {
	q := corepg.GetQuerier(ctx, s.db)

	query := `SELECT id, event_type, event_data, aggregate_id, occurred_at, correlation_id, causation_id FROM ddd_domain_events
			  WHERE id > $1 ORDER BY id ASC`
	args := []any{afterPosition}

	if limit > 0 {
		query += ` LIMIT $2`
		args = append(args, limit)
	}

	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query all events: %w", err)
	}
	defer rows.Close()

	result := make([]event.GlobalEvent[T], 0)
	for rows.Next() {
		var id int64
		var eventType string
		var data []byte
		var aggID string
		var occurredAt time.Time
		var correlationID string
		var causationID string
		if err := rows.Scan(&id, &eventType, &data, &aggID, &occurredAt, &correlationID, &causationID); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		evt, err := s.alloc(eventType)
		if err != nil {
			return nil, fmt.Errorf("allocate event: %w", err)
		}
		if err := json.Unmarshal(data, evt); err != nil {
			return nil, fmt.Errorf("unmarshal event: %w", err)
		}
		restoreBaseEvent(evt, aggID, occurredAt, correlationID, causationID)
		result = append(result, event.GlobalEvent[T]{
			Position: id,
			Event:    evt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return result, nil
}

func restoreBaseEvent(evt any, aggregateID string, occurredAt time.Time, correlationID string, causationID string) {
	e, ok := evt.(domainevent.Event)
	if !ok {
		return
	}
	event.RestoreBaseEvent(e, aggregateID, occurredAt, correlationID, causationID)
}

package pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	cqevent "github.com/ddd-qce/core/cqrs/event/pg"
	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/domain/event"
	corepg "github.com/ddd-qce/core/pg"
)

type SnapshotSerializer[T aggregate.AggregateRef] interface {
	Serialize(agg T) ([]byte, error)
	Deserialize(data []byte) (T, error)
}

type JSONSerializer[T aggregate.AggregateRef] struct{}

func (JSONSerializer[T]) Serialize(agg T) ([]byte, error) { return json.Marshal(agg) }
func (JSONSerializer[T]) Deserialize(data []byte) (T, error) {
	var v T
	err := json.Unmarshal(data, &v)
	return v, err
}

type PgRepository[T aggregate.AggregateRef] struct {
	db         *sql.DB
	serializer SnapshotSerializer[T]
	typeName   string
}

type RepoOption[T aggregate.AggregateRef] func(*PgRepository[T])

func WithRepoSerializer[T aggregate.AggregateRef](s SnapshotSerializer[T]) RepoOption[T] {
	return func(r *PgRepository[T]) { r.serializer = s }
}

func NewRepository[T aggregate.AggregateRef](db *sql.DB, opts ...RepoOption[T]) *PgRepository[T] {
	var zero T
	tName := reflect.TypeOf(zero).Name()
	if reflect.TypeOf(zero).Kind() == reflect.Ptr {
		tName = reflect.TypeOf(zero).Elem().Name()
	}
	r := &PgRepository[T]{
		db:         db,
		serializer: JSONSerializer[T]{},
		typeName:   tName,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func (r *PgRepository[T]) Save(ctx context.Context, agg T) error {
	q := corepg.GetQuerier(ctx, r.db)
	data, err := r.serializer.Serialize(agg)
	if err != nil {
		return fmt.Errorf("serialize aggregate: %w", err)
	}
	root := agg.GetAggregateRoot()
	res, err := q.ExecContext(ctx,
		`INSERT INTO ddd_aggregate_snapshots (aggregate_id, aggregate_type, snapshot_data, version, updated_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (aggregate_id) DO UPDATE SET snapshot_data = $3, version = $4, updated_at = $5
		 WHERE ddd_aggregate_snapshots.version < $4`,
		root.GetID(), r.typeName, data, root.GetVersion(), time.Now(),
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if n == 0 {
		return &OptimisticLockError{AggregateID: root.GetID(), ExpectedVersion: root.GetVersion()}
	}
	return nil
}

func (r *PgRepository[T]) FindByID(ctx context.Context, id string) (T, error) {
	q := corepg.GetQuerier(ctx, r.db)
	var data []byte
	var version int
	err := q.QueryRowContext(ctx,
		`SELECT snapshot_data, version FROM ddd_aggregate_snapshots WHERE aggregate_id = $1 AND aggregate_type = $2`,
		id, r.typeName,
	).Scan(&data, &version)
	if err == sql.ErrNoRows {
		var zero T
		return zero, fmt.Errorf("aggregate %s not found", id)
	}
	if err != nil {
		var zero T
		return zero, err
	}
	agg, err := r.serializer.Deserialize(data)
	if err != nil {
		return agg, fmt.Errorf("deserialize aggregate: %w", err)
	}
	return agg, nil
}

func (r *PgRepository[T]) Delete(ctx context.Context, id string) error {
	q := corepg.GetQuerier(ctx, r.db)
	res, err := q.ExecContext(ctx,
		`DELETE FROM ddd_aggregate_snapshots WHERE aggregate_id = $1 AND aggregate_type = $2`,
		id, r.typeName,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("aggregate %s not found", id)
	}
	return nil
}

type AggregateReconstructor[T aggregate.AggregateRef] func(id string) T

type PgEventSourcedRepository[T aggregate.AggregateRef] struct {
	db            *sql.DB
	eventStore    *cqevent.EventStore[event.DomainEvent]
	reconstructor AggregateReconstructor[T]
	serializer    SnapshotSerializer[T]
	typeName      string
	snapshotEvery int
}

func NewEventSourcedRepository[T aggregate.AggregateRef](
	db *sql.DB,
	eventStore *cqevent.EventStore[event.DomainEvent],
	reconstructor AggregateReconstructor[T],
	opts ...EventSourcedRepoOption[T],
) *PgEventSourcedRepository[T] {
	var zero T
	tName := reflect.TypeOf(zero).Name()
	if reflect.TypeOf(zero).Kind() == reflect.Ptr {
		tName = reflect.TypeOf(zero).Elem().Name()
	}
	r := &PgEventSourcedRepository[T]{
		db:            db,
		eventStore:    eventStore,
		reconstructor: reconstructor,
		serializer:    JSONSerializer[T]{},
		typeName:      tName,
		snapshotEvery: 10,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

type EventSourcedRepoOption[T aggregate.AggregateRef] func(*PgEventSourcedRepository[T])

func WithSnapshotEvery[T aggregate.AggregateRef](n int) EventSourcedRepoOption[T] {
	return func(r *PgEventSourcedRepository[T]) { r.snapshotEvery = n }
}

func WithSerializer[T aggregate.AggregateRef](s SnapshotSerializer[T]) EventSourcedRepoOption[T] {
	return func(r *PgEventSourcedRepository[T]) { r.serializer = s }
}

func (r *PgEventSourcedRepository[T]) Save(ctx context.Context, agg T) error {
	root := agg.GetAggregateRoot()
	events := root.UncommittedEvents()
	if len(events) == 0 {
		return nil
	}
	typedEvents := make([]event.DomainEvent, len(events))
	copy(typedEvents, events)
	if err := r.eventStore.Append(ctx, root.GetID(), root.GetVersion()-len(events), typedEvents); err != nil {
		return fmt.Errorf("append events: %w", err)
	}
	if r.snapshotEvery > 0 && root.GetVersion()%r.snapshotEvery == 0 {
		if err := r.saveSnapshot(ctx, agg, root); err != nil {
			return fmt.Errorf("save snapshot: %w", err)
		}
	}
	root.MarkEventsAsCommitted()
	return nil
}

func (r *PgEventSourcedRepository[T]) Load(ctx context.Context, id string) (T, error) {
	snapshotVersion := -1
	agg := r.reconstructor(id)
	root := agg.GetAggregateRoot()

	data, version, err := r.loadSnapshot(ctx, id)
	if err == nil {
		deserAgg, err2 := r.serializer.Deserialize(data)
		if err2 != nil {
			return agg, fmt.Errorf("deserialize snapshot for aggregate %s: %w", id, err2)
		}
		agg = deserAgg
		root = agg.GetAggregateRoot()
		root.SetSnapshotVersion(version)
		snapshotVersion = version
	}

	events, err := r.eventStore.Load(ctx, id, snapshotVersion)
	if err != nil {
		if snapshotVersion < 0 {
			return agg, fmt.Errorf("load events for aggregate %s: %w", id, err)
		}
		return agg, fmt.Errorf("load events for aggregate %s after snapshot v%d: %w", id, snapshotVersion, err)
	}

	typedEvents := make([]event.DomainEvent, len(events))
	copy(typedEvents, events)
	root.LoadFromHistory(typedEvents)
	return agg, nil
}

func (r *PgEventSourcedRepository[T]) saveSnapshot(ctx context.Context, agg T, root *aggregate.AggregateRoot) error {
	q := corepg.GetQuerier(ctx, r.db)
	data, err := r.serializer.Serialize(agg)
	if err != nil {
		return err
	}
	res, err := q.ExecContext(ctx,
		`INSERT INTO ddd_aggregate_snapshots (aggregate_id, aggregate_type, snapshot_data, version, updated_at)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (aggregate_id) DO UPDATE SET snapshot_data = $3, version = $4, updated_at = $5
		 WHERE ddd_aggregate_snapshots.version < $4`,
		root.GetID(), r.typeName, data, root.GetVersion(), time.Now(),
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if n == 0 {
		return &OptimisticLockError{AggregateID: root.GetID(), ExpectedVersion: root.GetVersion()}
	}
	return nil
}

func (r *PgEventSourcedRepository[T]) loadSnapshot(ctx context.Context, id string) ([]byte, int, error) {
	q := corepg.GetQuerier(ctx, r.db)
	var data []byte
	var version int
	err := q.QueryRowContext(ctx,
		`SELECT snapshot_data, version FROM ddd_aggregate_snapshots WHERE aggregate_id = $1 AND aggregate_type = $2`,
		id, r.typeName,
	).Scan(&data, &version)
	return data, version, err
}

type OptimisticLockError struct {
	AggregateID     string
	ExpectedVersion int
}

func (e *OptimisticLockError) Error() string {
	return fmt.Sprintf("optimistic lock error: aggregate %s version %d was already updated by another transaction", e.AggregateID, e.ExpectedVersion)
}

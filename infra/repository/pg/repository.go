package pg

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"time"

	ddderror "github.com/ddd-qce/core/error"
	"github.com/ddd-qce/core/domain/aggregate"
	domainevent "github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/domain/repository"
	"github.com/ddd-qce/core/cqrs/event"
	corepg "github.com/ddd-qce/core/pg"
	infrarepo "github.com/ddd-qce/core/infra/repository"
)

type PgRepository[T aggregate.AggregateRef] struct {
	db         *sql.DB
	serializer repository.SnapshotSerializer[T]
	typeName   string
}

type RepoOption[T aggregate.AggregateRef] func(*PgRepository[T])

func WithRepoSerializer[T aggregate.AggregateRef](s repository.SnapshotSerializer[T]) RepoOption[T] {
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
		serializer: repository.JSONSerializer[T]{},
		typeName:   tName,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

func isUniqueViolation(err error) bool {
	if sq, ok := err.(interface{ SQLState() string }); ok {
		return sq.SQLState() == "23505"
	}
	return false
}

func (r *PgRepository[T]) Save(ctx context.Context, agg T) error {
	q := corepg.GetQuerier(ctx, r.db)
	data, err := r.serializer.Serialize(agg)
	if err != nil {
		return fmt.Errorf("serialize aggregate: %w", err)
	}
	root := agg.GetAggregateRoot()
	snapshotVersion := root.SnapshotVersion()
	newVersion := root.Version()

	if snapshotVersion < 0 {
		res, err := q.ExecContext(ctx,
			`INSERT INTO ddd_aggregate_snapshots (aggregate_id, aggregate_type, snapshot_data, version, updated_at)
			 VALUES ($1, $2, $3, $4, $5)`,
			root.ID(), r.typeName, data, newVersion, time.Now(),
		)
		if err != nil {
			if isUniqueViolation(err) {
				return &infrarepo.OptimisticLockError{AggregateID: root.ID(), ExpectedVersion: snapshotVersion}
			}
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return &infrarepo.OptimisticLockError{AggregateID: root.ID(), ExpectedVersion: snapshotVersion}
		}
		root.SetSnapshotVersion(newVersion)
		return nil
	}

	res, err := q.ExecContext(ctx,
		`UPDATE ddd_aggregate_snapshots SET snapshot_data = $3, version = $4, updated_at = $5
		 WHERE aggregate_id = $1 AND aggregate_type = $2 AND version = $6`,
		root.ID(), r.typeName, data, newVersion, time.Now(), snapshotVersion,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if n == 0 {
		return &infrarepo.OptimisticLockError{AggregateID: root.ID(), ExpectedVersion: snapshotVersion}
	}
	root.SetSnapshotVersion(newVersion)
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
		return zero, fmt.Errorf("aggregate %s: %w", id, ddderror.ErrNotFound)
	}
	if err != nil {
		var zero T
		return zero, err
	}
	agg, err := r.serializer.Deserialize(data)
	if err != nil {
		return agg, fmt.Errorf("deserialize aggregate: %w", err)
	}
	agg.GetAggregateRoot().SetSnapshotVersion(version)
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
		return fmt.Errorf("aggregate %s: %w", id, ddderror.ErrNotFound)
	}
	return nil
}

type AggregateReconstructor[T aggregate.AggregateRef] func(id string) T

type PgEventSourcedRepository[T aggregate.AggregateRef] struct {
	db            *sql.DB
	eventStore    event.EventSourceStore[domainevent.Event]
	reconstructor AggregateReconstructor[T]
	serializer    repository.SnapshotSerializer[T]
	typeName      string
	snapshotEvery int
}

func NewEventSourcedRepository[T aggregate.AggregateRef](
	db *sql.DB,
	eventStore event.EventSourceStore[domainevent.Event],
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
		serializer:    repository.JSONSerializer[T]{},
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

func WithSerializer[T aggregate.AggregateRef](s repository.SnapshotSerializer[T]) EventSourcedRepoOption[T] {
	return func(r *PgEventSourcedRepository[T]) { r.serializer = s }
}

func (r *PgEventSourcedRepository[T]) Save(ctx context.Context, agg T) error {
	root := agg.GetAggregateRoot()
	domainEvents := root.UncommittedEvents()
	if len(domainEvents) == 0 {
		return nil
	}
	if err := r.eventStore.Append(ctx, root.ID(), root.Version()-len(domainEvents), domainEvents); err != nil {
		return fmt.Errorf("append events: %w", err)
	}
	if r.snapshotEvery > 0 && root.Version()%r.snapshotEvery == 0 {
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
		return agg, fmt.Errorf("load events for aggregate %s: %w", id, err)
	}

	if len(events) == 0 && snapshotVersion < 0 {
		return agg, fmt.Errorf("aggregate %s: %w", id, ddderror.ErrNotFound)
	}

	if err := aggregate.LoadFromHistory(agg, events); err != nil {
		return agg, fmt.Errorf("load from history for aggregate %s: %w", id, err)
	}
	return agg, nil
}

func (r *PgEventSourcedRepository[T]) saveSnapshot(ctx context.Context, agg T, root *aggregate.AggregateRoot) error {
	q := corepg.GetQuerier(ctx, r.db)
	data, err := r.serializer.Serialize(agg)
	if err != nil {
		return err
	}
	snapshotVersion := root.SnapshotVersion()
	newVersion := root.Version()

	if snapshotVersion < 0 {
		res, err := q.ExecContext(ctx,
			`INSERT INTO ddd_aggregate_snapshots (aggregate_id, aggregate_type, snapshot_data, version, updated_at)
			 VALUES ($1, $2, $3, $4, $5)`,
			root.ID(), r.typeName, data, newVersion, time.Now(),
		)
		if err != nil {
			if isUniqueViolation(err) {
				return &infrarepo.OptimisticLockError{AggregateID: root.ID(), ExpectedVersion: snapshotVersion}
			}
			return err
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return &infrarepo.OptimisticLockError{AggregateID: root.ID(), ExpectedVersion: snapshotVersion}
		}
		root.SetSnapshotVersion(newVersion)
		return nil
	}

	res, err := q.ExecContext(ctx,
		`UPDATE ddd_aggregate_snapshots SET snapshot_data = $3, version = $4, updated_at = $5
		 WHERE aggregate_id = $1 AND aggregate_type = $2 AND version = $6`,
		root.ID(), r.typeName, data, newVersion, time.Now(), snapshotVersion,
	)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("check rows affected: %w", err)
	}
	if n == 0 {
		return &infrarepo.OptimisticLockError{AggregateID: root.ID(), ExpectedVersion: snapshotVersion}
	}
	root.SetSnapshotVersion(newVersion)
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


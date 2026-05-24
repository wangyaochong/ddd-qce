package infra_pg

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	pgevent "github.com/ddd-qce/core/cqrs/event/pg"
	ddderror "github.com/ddd-qce/core/error"
	"github.com/ddd-qce/core/domain/aggregate"
	"github.com/ddd-qce/core/domain/event"
	pgrepo "github.com/ddd-qce/core/infra/repository/pg"
	"github.com/ddd-qce/it/testutil"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type testOrder struct {
	aggregate.AggregateRoot
	Name   string
	Amount float64
}

func newTestOrder(id string) *testOrder {
	o := &testOrder{}
	o.AggregateRoot = *aggregate.NewAggregateRootWithApplier(id, o)
	return o
}

func (o *testOrder) When(evt event.DomainEvent) {}

type testOrderJSON struct {
	ID     string  `json:"id"`
	Name   string  `json:"name"`
	Amount float64 `json:"amount"`
}

func (o *testOrder) MarshalJSON() ([]byte, error) {
	return json.Marshal(testOrderJSON{
		ID:     o.GetID(),
		Name:   o.Name,
		Amount: o.Amount,
	})
}

func (o *testOrder) UnmarshalJSON(data []byte) error {
	var aux testOrderJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	o.AggregateRoot = *aggregate.NewAggregateRootWithApplier(aux.ID, o)
	o.Name = aux.Name
	o.Amount = aux.Amount
	return nil
}

type testOrderEvent struct {
	AggID string    `json:"agg_id"`
	EType string    `json:"e_type"`
	EAt   time.Time `json:"e_at"`
}

func (e *testOrderEvent) AggregateID() string   { return e.AggID }
func (e *testOrderEvent) EventType() string     { return e.EType }
func (e *testOrderEvent) OccurredAt() time.Time { return e.EAt }

func newEventStore(db *sql.DB) *pgevent.EventStore[event.DomainEvent] {
	store, err := pgevent.NewEventStore[event.DomainEvent](
		db,
		pgevent.WithFactory[event.DomainEvent](func() event.DomainEvent {
			return &testOrderEvent{}
		}),
	)
	if err != nil {
		panic(fmt.Sprintf("create event store: %v", err))
	}
	return store
}

func TestPgRepository_SaveAndFindByID(t *testing.T) {
	db := testutil.OpenTestDB(t, "ddd_qce_repo_test")
	repo := pgrepo.NewRepository[*testOrder](db)
	ctx := context.Background()

	order := newTestOrder("order-1")
	order.Name = "test order"
	order.Amount = 99.5

	if err := repo.Save(ctx, order); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	found, err := repo.FindByID(ctx, "order-1")
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if found.GetID() != "order-1" {
		t.Errorf("expected ID 'order-1', got %s", found.GetID())
	}
	if found.Name != "test order" {
		t.Errorf("expected Name 'test order', got %s", found.Name)
	}
	if found.Amount != 99.5 {
		t.Errorf("expected Amount 99.5, got %f", found.Amount)
	}
	if found.Version() != 0 {
		t.Errorf("expected version 0 after load, got %d", found.Version())
	}
}

func TestPgRepository_FindByID_VersionRestoredForOptimisticLock(t *testing.T) {
	db := testutil.OpenTestDB(t, "ddd_qce_repo_test")
	repo := pgrepo.NewRepository[*testOrder](db)
	ctx := context.Background()

	order := newTestOrder("order-version")
	order.Name = "v1"
	order.Amount = 10
	if err := repo.Save(ctx, order); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := repo.FindByID(ctx, "order-version")
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}

	loaded.Name = "v2"
	loaded.Amount = 20
	if err := repo.Save(ctx, loaded); err != nil {
		t.Fatalf("Save after FindByID should succeed with correct version, got: %v", err)
	}

	duplicate := newTestOrder("order-version")
	duplicate.Name = "conflict"
	duplicate.Amount = 30
	err = repo.Save(ctx, duplicate)
	if err == nil {
		t.Fatal("expected OptimisticLockError for stale version")
	}
	var ole *pgrepo.OptimisticLockError
	if !errors.As(err, &ole) {
		t.Errorf("expected *OptimisticLockError, got %T: %v", err, err)
	}
}

func TestPgRepository_FindByID_NotFound(t *testing.T) {
	db := testutil.OpenTestDB(t, "ddd_qce_repo_test")
	repo := pgrepo.NewRepository[*testOrder](db)
	ctx := context.Background()

	_, err := repo.FindByID(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent aggregate")
	}
	if !errors.Is(err, ddderror.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPgRepository_Delete(t *testing.T) {
	db := testutil.OpenTestDB(t, "ddd_qce_repo_test")
	repo := pgrepo.NewRepository[*testOrder](db)
	ctx := context.Background()

	order := newTestOrder("order-del")
	order.Name = "to delete"
	order.Amount = 10
	if err := repo.Save(ctx, order); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if err := repo.Delete(ctx, "order-del"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err := repo.FindByID(ctx, "order-del")
	if !errors.Is(err, ddderror.ErrNotFound) {
		t.Errorf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestPgRepository_Delete_NotFound(t *testing.T) {
	db := testutil.OpenTestDB(t, "ddd_qce_repo_test")
	repo := pgrepo.NewRepository[*testOrder](db)
	ctx := context.Background()

	err := repo.Delete(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for deleting nonexistent aggregate")
	}
	if !errors.Is(err, ddderror.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPgRepository_OptimisticLock(t *testing.T) {
	db := testutil.OpenTestDB(t, "ddd_qce_repo_test")
	repo := pgrepo.NewRepository[*testOrder](db)
	ctx := context.Background()

	order := newTestOrder("order-lock")
	order.Name = "first save"
	order.Amount = 1
	if err := repo.Save(ctx, order); err != nil {
		t.Fatalf("first Save failed: %v", err)
	}

	duplicate := newTestOrder("order-lock")
	duplicate.Name = "second save"
	duplicate.Amount = 2

	err := repo.Save(ctx, duplicate)
	if err == nil {
		t.Fatal("expected OptimisticLockError")
	}
	var ole *pgrepo.OptimisticLockError
	if !errors.As(err, &ole) {
		t.Errorf("expected *OptimisticLockError, got %T: %v", err, err)
	}
	if !errors.Is(err, ddderror.ErrConcurrency) {
		t.Errorf("expected ErrConcurrency unwrap, got %v", err)
	}
}

func TestPgEventSourcedRepository_SaveAndLoad(t *testing.T) {
	db := testutil.OpenTestDB(t, "ddd_qce_repo_test")
	eventStore := newEventStore(db)
	repo := pgrepo.NewEventSourcedRepository[*testOrder](
		db,
		eventStore,
		func(id string) *testOrder { return newTestOrder(id) },
	)
	ctx := context.Background()

	order := newTestOrder("order-es-1")
	order.Apply(&testOrderEvent{AggID: "order-es-1", EType: "OrderCreated", EAt: time.Now()})
	order.Name = "created order"

	if err := repo.Save(ctx, order); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := repo.Load(ctx, "order-es-1")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.GetID() != "order-es-1" {
		t.Errorf("expected ID 'order-es-1', got %s", loaded.GetID())
	}
	if loaded.Version() != 1 {
		t.Errorf("expected version 1, got %d", loaded.Version())
	}
}

func TestPgEventSourcedRepository_Load_NotFound(t *testing.T) {
	db := testutil.OpenTestDB(t, "ddd_qce_repo_test")
	eventStore := newEventStore(db)
	repo := pgrepo.NewEventSourcedRepository[*testOrder](
		db,
		eventStore,
		func(id string) *testOrder { return newTestOrder(id) },
	)
	ctx := context.Background()

	_, err := repo.Load(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent aggregate")
	}
	if !errors.Is(err, ddderror.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPgEventSourcedRepository_Snapshot(t *testing.T) {
	db := testutil.OpenTestDB(t, "ddd_qce_repo_test")
	eventStore := newEventStore(db)
	repo := pgrepo.NewEventSourcedRepository[*testOrder](
		db,
		eventStore,
		func(id string) *testOrder { return newTestOrder(id) },
		pgrepo.WithSnapshotEvery[*testOrder](3),
	)
	ctx := context.Background()

	order := newTestOrder("order-snap")
	for i := 0; i < 6; i++ {
		order.Apply(&testOrderEvent{AggID: "order-snap", EType: "OrderUpdated", EAt: time.Now()})
	}

	if err := repo.Save(ctx, order); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if order.Version() != 6 {
		t.Fatalf("expected version 6, got %d", order.Version())
	}

	var snapVersion int
	err := db.QueryRow("SELECT version FROM ddd_aggregate_snapshots WHERE aggregate_id = $1", "order-snap").Scan(&snapVersion)
	if err != nil {
		t.Fatalf("snapshot query failed: %v", err)
	}
	if snapVersion != 6 {
		t.Errorf("expected snapshot version 6, got %d", snapVersion)
	}

	loaded, err := repo.Load(ctx, "order-snap")
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if loaded.GetID() != "order-snap" {
		t.Errorf("expected ID 'order-snap', got %s", loaded.GetID())
	}
	if loaded.Version() != 6 {
		t.Errorf("expected version 6 after load, got %d", loaded.Version())
	}
}

package pg

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ddd-qce/core/cqrs/event"
	pgevent "github.com/ddd-qce/core/cqrs/impl/pg"
)

type mockTestStoreEvent struct {
	event.BaseEvent
	Data string
}

func (e *mockTestStoreEvent) AggregateID() string   { return e.BaseEvent.AggregateID() }
func (e *mockTestStoreEvent) OccurredAt() time.Time { return e.BaseEvent.OccurredAt() }

func TestMockNewEventSourceStore(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	store, err := pgevent.NewEventSourceStore[*mockTestStoreEvent](db)
	if err != nil {
		t.Errorf("NewEventSourceStore() error = %v, want nil", err)
	}
	if store == nil {
		t.Error("NewEventSourceStore() returned nil")
	}
}

func TestMockNewEventSourceStore_WithFactory(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	store, err := pgevent.NewEventSourceStore[event.Event](db, pgevent.WithFactory(func() event.Event {
		return &mockTestStoreEvent{}
	}))
	if err != nil {
		t.Errorf("NewEventSourceStore() error = %v, want nil", err)
	}
	if store == nil {
		t.Error("NewEventSourceStore() returned nil")
	}
}

func TestMockNewEventSourceStore_InvalidType(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	_, err = pgevent.NewEventSourceStore[event.Event](db)
	if err == nil {
		t.Error("NewEventSourceStore() should error for interface type without factory")
	}
}

func TestMockEventSourceStore_Append(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO ddd_domain_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store, err := pgevent.NewEventSourceStore[*mockTestStoreEvent](db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	evt := &mockTestStoreEvent{
		BaseEvent: event.NewBaseEvent("agg-1", time.Now()),
		Data:      "test-data",
	}

	err = store.Append(context.Background(), "agg-1", 0, []*mockTestStoreEvent{evt})
	if err != nil {
		t.Errorf("Append() error = %v, want nil", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("mock expectations not met: %v", err)
	}
}

func TestMockEventSourceStore_Append_Error(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO ddd_domain_events").WillReturnError(errors.New("db error"))
	mock.ExpectRollback()

	store, err := pgevent.NewEventSourceStore[*mockTestStoreEvent](db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	evt := &mockTestStoreEvent{
		BaseEvent: event.NewBaseEvent("agg-1", time.Now()),
		Data:      "test-data",
	}

	err = store.Append(context.Background(), "agg-1", 0, []*mockTestStoreEvent{evt})
	if err == nil {
		t.Error("Append() should return error on db error")
	}
}

func TestMockEventSourceStore_Append_InTransaction(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	mock.ExpectExec("INSERT INTO ddd_domain_events").WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectCommit()

	store, err := pgevent.NewEventSourceStore[*mockTestStoreEvent](db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	evt := &mockTestStoreEvent{
		BaseEvent: event.NewBaseEvent("agg-1", time.Now()),
		Data:      "test-data",
	}

	err = store.Append(context.Background(), "agg-1", 0, []*mockTestStoreEvent{evt})
	if err != nil {
		t.Errorf("Append() error = %v, want nil", err)
	}
}

func TestMockEventSourceStore_Load(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	eventData, _ := json.Marshal(&mockTestStoreEvent{Data: "test-data"})
	rows := sqlmock.NewRows([]string{"event_type", "event_data", "aggregate_id", "occurred_at", "correlation_id", "causation_id"}).
		AddRow("mockTestStoreEvent", eventData, "agg-1", time.Now(), "", "")

	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	store, err := pgevent.NewEventSourceStore[*mockTestStoreEvent](db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	events, err := store.Load(context.Background(), "agg-1", 0)
	if err != nil {
		t.Errorf("Load() error = %v, want nil", err)
	}
	if len(events) != 1 {
		t.Errorf("Load() returned %d events, want 1", len(events))
	}
	if events[0].AggregateID() != "agg-1" {
		t.Errorf("AggregateID() = %q, want %q", events[0].AggregateID(), "agg-1")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("mock expectations not met: %v", err)
	}
}

func TestMockEventSourceStore_Load_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"event_type", "event_data", "aggregate_id", "occurred_at", "correlation_id", "causation_id"})
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	store, err := pgevent.NewEventSourceStore[*mockTestStoreEvent](db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	events, err := store.Load(context.Background(), "agg-1", 0)
	if err != nil {
		t.Errorf("Load() error = %v, want nil", err)
	}
	if len(events) != 0 {
		t.Errorf("Load() returned %d events, want 0", len(events))
	}
}

func TestMockEventSourceStore_LoadAll(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	eventData, _ := json.Marshal(&mockTestStoreEvent{Data: "test-data"})
	rows := sqlmock.NewRows([]string{"id", "event_type", "event_data", "aggregate_id", "occurred_at", "correlation_id", "causation_id"}).
		AddRow(int64(1), "mockTestStoreEvent", eventData, "agg-1", time.Now(), "", "").
		AddRow(int64(2), "mockTestStoreEvent", eventData, "agg-2", time.Now(), "", "")

	mock.ExpectQuery("SELECT id, event_type, event_data, aggregate_id, occurred_at, correlation_id, causation_id FROM ddd_domain_events").
		WithArgs(int64(0)).
		WillReturnRows(rows)

	store, err := pgevent.NewEventSourceStore[*mockTestStoreEvent](db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	events, err := store.LoadAll(context.Background(), 0, 0)
	if err != nil {
		t.Errorf("LoadAll() error = %v, want nil", err)
	}
	if len(events) != 2 {
		t.Errorf("LoadAll() returned %d events, want 2", len(events))
	}
	if events[0].Position != 1 {
		t.Errorf("first event position = %d, want 1", events[0].Position)
	}
	if events[1].Position != 2 {
		t.Errorf("second event position = %d, want 2", events[1].Position)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("mock expectations not met: %v", err)
	}
}

func TestMockEventSourceStore_LoadAll_WithLimit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	eventData, _ := json.Marshal(&mockTestStoreEvent{Data: "test-data"})
	rows := sqlmock.NewRows([]string{"id", "event_type", "event_data", "aggregate_id", "occurred_at", "correlation_id", "causation_id"}).
		AddRow(int64(3), "mockTestStoreEvent", eventData, "agg-1", time.Now(), "", "")

	mock.ExpectQuery("SELECT id, event_type, event_data, aggregate_id, occurred_at, correlation_id, causation_id FROM ddd_domain_events").
		WithArgs(int64(2), 10).
		WillReturnRows(rows)

	store, err := pgevent.NewEventSourceStore[*mockTestStoreEvent](db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	events, err := store.LoadAll(context.Background(), 2, 10)
	if err != nil {
		t.Errorf("LoadAll() error = %v, want nil", err)
	}
	if len(events) != 1 {
		t.Errorf("LoadAll() returned %d events, want 1", len(events))
	}
	if events[0].Position != 3 {
		t.Errorf("event position = %d, want 3", events[0].Position)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("mock expectations not met: %v", err)
	}
}

func TestMockEventSourceStore_LoadAll_Empty(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "event_type", "event_data", "aggregate_id", "occurred_at", "correlation_id", "causation_id"})
	mock.ExpectQuery("SELECT").WillReturnRows(rows)

	store, err := pgevent.NewEventSourceStore[*mockTestStoreEvent](db)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}

	events, err := store.LoadAll(context.Background(), 100, 0)
	if err != nil {
		t.Errorf("LoadAll() error = %v, want nil", err)
	}
	if len(events) != 0 {
		t.Errorf("LoadAll() returned %d events, want 0", len(events))
	}
}

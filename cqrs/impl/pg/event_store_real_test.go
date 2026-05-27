package pg

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/ddd-qce/core/cqrs/event"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TEST_PG_DSN")
	if dsn == "" {
		dsn = "host=/var/run/postgresql dbname=test_event_store user=" + os.Getenv("USER") + " sslmode=disable"
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatalf("open db failed: %v", err)
	}
	if err := db.Ping(); err != nil {
		t.Fatalf("ping db failed: %v", err)
	}
	return db
}

type testPgEvent struct {
	event.BaseEvent
	Data string
}

func (e *testPgEvent) AggregateID() string   { return e.BaseEvent.AggregateID() }
func (e *testPgEvent) OccurredAt() time.Time { return e.BaseEvent.OccurredAt() }

func TestEventSourceStore_RealDB_AppendAndLoad(t *testing.T) {
	if os.Getenv("RUN_REAL_DB_TESTS") != "1" {
		t.Skip("Set RUN_REAL_DB_TESTS=1 to run real DB tests")
	}

	db := openTestDB(t)
	defer db.Close()

	// Setup: create table
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ddd_domain_events (
			id            BIGSERIAL PRIMARY KEY,
			aggregate_id  TEXT NOT NULL,
			event_type    TEXT NOT NULL,
			event_data    JSONB NOT NULL,
			occurred_at   TIMESTAMPTZ NOT NULL,
			version       INT NOT NULL DEFAULT 0,
			UNIQUE(aggregate_id, version)
		);
		CREATE INDEX IF NOT EXISTS idx_ddd_events_aggregate ON ddd_domain_events(aggregate_id, version);
	`)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer db.Exec("DROP TABLE IF EXISTS ddd_domain_events")

	store, err := NewEventSourceStore[*testPgEvent](db)
	if err != nil {
		t.Fatalf("NewEventSourceStore failed: %v", err)
	}

	// Test Append
	evt := &testPgEvent{
		BaseEvent: event.NewBaseEvent("agg-1", time.Now()),
		Data:      "test-data",
	}
	err = store.Append(context.Background(), "agg-1", 0, []*testPgEvent{evt})
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Test Load
	events, err := store.Load(context.Background(), "agg-1", 0)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Data != "test-data" {
		t.Errorf("expected data 'test-data', got '%s'", events[0].Data)
	}
}

func TestEventSourceStore_RealDB_AppendMultipleEvents(t *testing.T) {
	if os.Getenv("RUN_REAL_DB_TESTS") != "1" {
		t.Skip("Set RUN_REAL_DB_TESTS=1 to run real DB tests")
	}

	db := openTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ddd_domain_events (
			id            BIGSERIAL PRIMARY KEY,
			aggregate_id  TEXT NOT NULL,
			event_type    TEXT NOT NULL,
			event_data    JSONB NOT NULL,
			occurred_at   TIMESTAMPTZ NOT NULL,
			version       INT NOT NULL DEFAULT 0,
			UNIQUE(aggregate_id, version)
		)
	`)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer db.Exec("DROP TABLE IF EXISTS ddd_domain_events")

	store, err := NewEventSourceStore[*testPgEvent](db)
	if err != nil {
		t.Fatalf("NewEventSourceStore failed: %v", err)
	}

	// Append multiple events
	events := []*testPgEvent{
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "event1"},
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "event2"},
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "event3"},
	}
	err = store.Append(context.Background(), "agg-1", 0, events)
	if err != nil {
		t.Fatalf("Append multiple failed: %v", err)
	}

	// Load and verify
	loaded, err := store.Load(context.Background(), "agg-1", 0)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(loaded) != 3 {
		t.Fatalf("expected 3 events, got %d", len(loaded))
	}
}

func TestEventSourceStore_RealDB_ConcurrencyConflict(t *testing.T) {
	if os.Getenv("RUN_REAL_DB_TESTS") != "1" {
		t.Skip("Set RUN_REAL_DB_TESTS=1 to run real DB tests")
	}

	db := openTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ddd_domain_events (
			id            BIGSERIAL PRIMARY KEY,
			aggregate_id  TEXT NOT NULL,
			event_type    TEXT NOT NULL,
			event_data    JSONB NOT NULL,
			occurred_at   TIMESTAMPTZ NOT NULL,
			version       INT NOT NULL DEFAULT 0,
			UNIQUE(aggregate_id, version)
		)
	`)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer db.Exec("DROP TABLE IF EXISTS ddd_domain_events")

	store, err := NewEventSourceStore[*testPgEvent](db)
	if err != nil {
		t.Fatalf("NewEventSourceStore failed: %v", err)
	}

	// First append succeeds
	evt := &testPgEvent{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "first"}
	err = store.Append(context.Background(), "agg-1", 0, []*testPgEvent{evt})
	if err != nil {
		t.Fatalf("First append failed: %v", err)
	}

	// Second append with same version should fail
	evt2 := &testPgEvent{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "second"}
	err = store.Append(context.Background(), "agg-1", 0, []*testPgEvent{evt2})
	if err == nil {
		t.Fatal("Expected concurrency conflict error")
	}
}

func TestEventSourceStore_RealDB_LoadAfterVersion(t *testing.T) {
	if os.Getenv("RUN_REAL_DB_TESTS") != "1" {
		t.Skip("Set RUN_REAL_DB_TESTS=1 to run real DB tests")
	}

	db := openTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ddd_domain_events (
			id            BIGSERIAL PRIMARY KEY,
			aggregate_id  TEXT NOT NULL,
			event_type    TEXT NOT NULL,
			event_data    JSONB NOT NULL,
			occurred_at   TIMESTAMPTZ NOT NULL,
			version       INT NOT NULL DEFAULT 0,
			UNIQUE(aggregate_id, version)
		)
	`)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer db.Exec("DROP TABLE IF EXISTS ddd_domain_events")

	store, err := NewEventSourceStore[*testPgEvent](db)
	if err != nil {
		t.Fatalf("NewEventSourceStore failed: %v", err)
	}

	// Append 3 events
	for i := 1; i <= 3; i++ {
		evt := &testPgEvent{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "event"}
		err = store.Append(context.Background(), "agg-1", i-1, []*testPgEvent{evt})
		if err != nil {
			t.Fatalf("Append %d failed: %v", i, err)
		}
	}

	// Load after version 1 - should get events 2 and 3
	events, err := store.Load(context.Background(), "agg-1", 1)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
}

func TestEventSourceStore_RealDB_LoadAll(t *testing.T) {
	if os.Getenv("RUN_REAL_DB_TESTS") != "1" {
		t.Skip("Set RUN_REAL_DB_TESTS=1 to run real DB tests")
	}

	db := openTestDB(t)
	defer db.Close()

	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS ddd_domain_events (
			id            BIGSERIAL PRIMARY KEY,
			aggregate_id  TEXT NOT NULL,
			event_type    TEXT NOT NULL,
			event_data    JSONB NOT NULL,
			occurred_at   TIMESTAMPTZ NOT NULL,
			version       INT NOT NULL DEFAULT 0,
			UNIQUE(aggregate_id, version)
		);
		TRUNCATE ddd_domain_events;
	`)
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	defer db.Exec("DROP TABLE IF EXISTS ddd_domain_events")

	store, err := NewEventSourceStore[*testPgEvent](db)
	if err != nil {
		t.Fatalf("NewEventSourceStore failed: %v", err)
	}

	store.Append(context.Background(), "agg-1", 0, []*testPgEvent{
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "a1-e1"},
		{BaseEvent: event.NewBaseEvent("agg-1", time.Now()), Data: "a1-e2"},
	})
	store.Append(context.Background(), "agg-2", 0, []*testPgEvent{
		{BaseEvent: event.NewBaseEvent("agg-2", time.Now()), Data: "a2-e1"},
	})

	all, err := store.LoadAll(context.Background(), 0, 0)
	if err != nil {
		t.Fatalf("LoadAll failed: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 events, got %d", len(all))
	}
	if all[0].Position != 1 {
		t.Errorf("first event position = %d, want 1", all[0].Position)
	}
	if all[1].Position != 2 {
		t.Errorf("second event position = %d, want 2", all[1].Position)
	}
	if all[2].Position != 3 {
		t.Errorf("third event position = %d, want 3", all[2].Position)
	}
	if all[0].Event.Data != "a1-e1" {
		t.Errorf("first event data = %s, want a1-e1", all[0].Event.Data)
	}
	if all[2].Event.Data != "a2-e1" {
		t.Errorf("third event data = %s, want a2-e1", all[2].Event.Data)
	}

	allAfter, err := store.LoadAll(context.Background(), 2, 0)
	if err != nil {
		t.Fatalf("LoadAll after position failed: %v", err)
	}
	if len(allAfter) != 1 {
		t.Fatalf("expected 1 event after position 2, got %d", len(allAfter))
	}

	allLimited, err := store.LoadAll(context.Background(), 0, 2)
	if err != nil {
		t.Fatalf("LoadAll with limit failed: %v", err)
	}
	if len(allLimited) != 2 {
		t.Fatalf("expected 2 events with limit 2, got %d", len(allLimited))
	}
}
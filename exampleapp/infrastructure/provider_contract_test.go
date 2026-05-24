package infrastructure

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/infra/infratest"
	"github.com/ddd-qce/exampleapp/domain"
)

func TestProviderContract_Memory(t *testing.T) {
	cfg := &Config{StoreType: StoreTypeMemory}
	store, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("create memory provider: %v", err)
	}
	testProviderContract(t, store)
}

func TestProviderContract_PostgreSQL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping PostgreSQL provider test in short mode")
	}
	dsn := pgTestDSN(t)
	cfg := &Config{StoreType: StoreTypePostgreSQL, PostgresURI: dsn}
	store, err := NewProvider(cfg)
	if err != nil {
		t.Fatalf("create pg provider: %v", err)
	}
	defer store.DB.Close()
	testProviderContract(t, store)
}

func testProviderContract(t *testing.T, store *StoreComponents) {
	t.Helper()

	t.Run("BackendContract", func(t *testing.T) {
		infratest.TestBackendContract(t, store.Backend)
	})

	t.Run("EventStoreAppendAndLoad", func(t *testing.T) {
		ctx := context.Background()
		evt := &domain.OrderPlacedEvent{
			BaseEvent:   event.NewBaseEvent("contract-agg-1", time.Now()),
			UserID:      "u1",
			TotalAmount: 100,
		}
		err := store.EventStore.Append(ctx, "contract-agg-1", 0, []event.DomainEvent{evt})
		if err != nil {
			t.Fatalf("append failed: %v", err)
		}
		events, err := store.EventStore.Load(ctx, "contract-agg-1", 0)
		if err != nil {
			t.Fatalf("load failed: %v", err)
		}
		if len(events) != 1 {
			t.Errorf("expected 1 event, got %d", len(events))
		}
	})

	t.Run("OrderRepositorySaveAndFind", func(t *testing.T) {
		ctx := context.Background()
		order, err := domain.NewOrder("contract-ord-1", "user-001", []*domain.OrderItem{
			domain.NewOrderItem("laptop", "Laptop", 999, 1),
		})
		if err != nil {
			t.Fatalf("create order: %v", err)
		}
		if err := store.OrderRepo.Save(ctx, order); err != nil {
			t.Fatalf("save failed: %v", err)
		}
		found, err := store.OrderRepo.FindByID(ctx, "contract-ord-1")
		if err != nil {
			t.Fatalf("find failed: %v", err)
		}
		if found.GetID() != "contract-ord-1" {
			t.Errorf("expected contract-ord-1, got %s", found.GetID())
		}
	})
}

func pgTestDSN(t *testing.T) string {
	t.Helper()
	dsn := testDSNFromEnv()
	if dsn == "" {
		t.Skip("DDD_POSTGRES_URI not set, skipping PostgreSQL test")
	}
	return dsn
}

func testDSNFromEnv() string {
	dsn := os.Getenv("DDD_POSTGRES_URI")
	if dsn != "" {
		return dsn
	}
	db, err := sql.Open("pgx", "host=/var/run/postgresql dbname=postgres user=root password=root sslmode=disable")
	if err != nil {
		return ""
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		return ""
	}
	return "host=/var/run/postgresql dbname=postgres user=root password=root sslmode=disable"
}

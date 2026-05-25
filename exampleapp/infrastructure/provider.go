package infrastructure

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ddd-qce/core/infra"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	eventpg "github.com/ddd-qce/core/cqrs/impl/pg"
	domainevent "github.com/ddd-qce/core/cqrs/event"
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/ddd-qce/exampleapp/application"
)

type StoreComponents struct {
	Backend      *infra.Backend
	EventStore   domainevent.EventSourceStore[domainevent.DomainEvent]
	OrderRepo    application.OrderRepositoryAdapter
	DB           *sql.DB
}

func NewProvider(cfg *Config) (*StoreComponents, error) {
	switch cfg.StoreType {
	case StoreTypeMemory:
		return newMemoryProvider()
	case StoreTypePostgreSQL:
		if cfg.PostgresURI == "" {
			return nil, fmt.Errorf("DDD_POSTGRES_URI is required when DDD_STORE_TYPE=postgresql")
		}
		return newPgProvider(cfg.PostgresURI)
	default:
		return nil, fmt.Errorf("unknown DDD_STORE_TYPE: %s (supported: memory, postgresql)", cfg.StoreType)
	}
}

func newMemoryProvider() (*StoreComponents, error) {
	eventStore, err := newMemoryEventStore()
	if err != nil {
		return nil, fmt.Errorf("create memory event store: %w", err)
	}
	return &StoreComponents{
		Backend:    infra.NewMemoryBackend(),
		EventStore: eventStore,
		OrderRepo:  application.NewOrderRepository(),
	}, nil
}

func newMemoryEventStore() (domainevent.EventSourceStore[domainevent.DomainEvent], error) {
	return eventmemory.NewEventSourceStore[domainevent.DomainEvent]()
}

func newPgProvider(dsn string) (*StoreComponents, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	backend := infra.NewPgBackend(db)
	if err := backend.Migrator.Migrate(context.Background()); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	eventStore, err := eventpg.NewEventSourceStore[domainevent.DomainEvent](db)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create pg event store: %w", err)
	}

	return &StoreComponents{
		Backend:    backend,
		EventStore: eventStore,
		OrderRepo:  application.NewOrderRepository(),
		DB:         db,
	}, nil
}

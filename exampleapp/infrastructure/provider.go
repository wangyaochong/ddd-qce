package infrastructure

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ddd-qce/core/infra"
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	eventpg "github.com/ddd-qce/core/cqrs/impl/pg"
	domainevent "github.com/ddd-qce/core/domain/event"
	_ "github.com/jackc/pgx/v5/stdlib"

	inventoryevent "github.com/ddd-qce/exampleapp/ddd/inventory/event"
	orderevent "github.com/ddd-qce/exampleapp/ddd/order/event"
	"github.com/ddd-qce/exampleapp/ddd/order/repository"
)

type StoreComponents struct {
	Backend      *infra.Backend
	EventStore   cqrsevent.EventSourceStore[domainevent.Event]
	OrderRepo    repository.OrderRepositoryAdapter
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

func domainEventFactoryMap() map[string]func() domainevent.Event {
	return map[string]func() domainevent.Event{
		"OrderPlacedEvent":       func() domainevent.Event { return &orderevent.OrderPlacedEvent{} },
		"PaymentConfirmedEvent":  func() domainevent.Event { return &orderevent.PaymentConfirmedEvent{} },
		"OrderShippedEvent":      func() domainevent.Event { return &orderevent.OrderShippedEvent{} },
		"OrderCancelledEvent":    func() domainevent.Event { return &orderevent.OrderCancelledEvent{} },
		"InventoryReservedEvent": func() domainevent.Event { return &inventoryevent.InventoryReservedEvent{} },
		"InventoryReleasedEvent": func() domainevent.Event { return &inventoryevent.InventoryReleasedEvent{} },
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
		OrderRepo:  repository.NewOrderRepository(),
	}, nil
}

func newMemoryEventStore() (cqrsevent.EventSourceStore[domainevent.Event], error) {
	return eventmemory.NewEventSourceStore[domainevent.Event]()
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

	eventStore, err := eventpg.NewEventSourceStore[domainevent.Event](db,
		eventpg.WithFactoryMap(domainEventFactoryMap()),
	)
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("create pg event store: %w", err)
	}

	return &StoreComponents{
		Backend:    backend,
		EventStore: eventStore,
		OrderRepo:  repository.NewOrderRepository(),
		DB:         db,
	}, nil
}

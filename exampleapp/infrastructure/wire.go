package infrastructure

import (
	"context"
	"fmt"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/cqrs/cmd"
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	querymemory "github.com/ddd-qce/core/cqrs/impl/memory"
	domainevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/infra"
	jobcore "github.com/ddd-qce/core/job/core"
	jobmemory "github.com/ddd-qce/core/job/memory"
	"github.com/ddd-qce/exampleapp/application"
	"github.com/ddd-qce/exampleapp/domain"
)

type AppContext struct {
	Chain      *aspect.AspectChain
	CmdBus     command.CommandBus
	QueryBus   query.QueryBus
	EventBus   cqrsevent.EventBus
	Backend    *infra.Backend
	JobManager *jobmemory.JobManager

	OrderRepo        application.OrderRepositoryAdapter
	EventSourcedRepo *application.OrderEventSourcedRepository
	EventStore       domainevent.EventSourceStore[domainevent.DomainEvent]
	Inventory        *domain.Inventory

	MetricsRecorder *AppMetricsRecorder
	TxManager       *AppTransactionManager

	store *StoreComponents
}

func (app *AppContext) Close() {
	if app.store != nil && app.store.DB != nil {
		app.store.DB.Close()
	}
}

func WireApp() *AppContext {
	app, err := WireAppWithConfig(LoadConfig())
	if err != nil {
		panic(err)
	}
	return app
}

func WireAppWithConfig(cfg *Config) (*AppContext, error) {
	store, err := NewProvider(cfg)
	if err != nil {
		return nil, err
	}
	recoveryEnabled := cfg.StoreType == StoreTypePostgreSQL
	return WireAppWithStore(store, recoveryEnabled)
}

func WireAppWithStore(store *StoreComponents, recoveryEnabled bool) (*AppContext, error) {
	backend := store.Backend

	logger := NewAppLogger()
	metricsRecorder := NewAppMetricsRecorder()
	txManager := NewAppTransactionManager()

	chain := aspect.NewAspectChain()
	chain.RegisterAspect(builtin.NewTracingAspect(backend.TraceStore))
	chain.RegisterAspect(builtin.NewLoggingAspect(logger))
	chain.RegisterAspect(builtin.NewMetricsAspect(metricsRecorder))
	chain.RegisterCommandAspect(builtin.NewTransactionAspect(txManager))

	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	queryBus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
	eventBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

	inventory := domain.NewInventory()
	orderRepo := store.OrderRepo
	eventStore := store.EventStore
	eventSourcedRepo := application.NewOrderEventSourcedRepository(eventStore, orderRepo)

	if err := cmdBus.RegisterHandler(application.NewPlaceOrderHandler(orderRepo, eventBus)); err != nil {
		return nil, fmt.Errorf("register PlaceOrderHandler: %w", err)
	}
	if err := cmdBus.RegisterHandler(application.NewConfirmPaymentHandler(orderRepo, eventBus)); err != nil {
		return nil, fmt.Errorf("register ConfirmPaymentHandler: %w", err)
	}
	if err := cmdBus.RegisterHandler(application.NewShipOrderHandler(orderRepo, eventBus)); err != nil {
		return nil, fmt.Errorf("register ShipOrderHandler: %w", err)
	}
	if err := cmdBus.RegisterHandler(application.NewCancelOrderHandler(orderRepo, eventBus)); err != nil {
		return nil, fmt.Errorf("register CancelOrderHandler: %w", err)
	}
	if err := cmdBus.RegisterHandler(application.NewReserveInventoryHandler(inventory, eventBus)); err != nil {
		return nil, fmt.Errorf("register ReserveInventoryHandler: %w", err)
	}
	if err := cmdBus.RegisterHandler(application.NewReleaseInventoryHandler(inventory, eventBus)); err != nil {
		return nil, fmt.Errorf("register ReleaseInventoryHandler: %w", err)
	}
	if err := cmdBus.RegisterHandler(application.NewGenerateReportHandler()); err != nil {
		return nil, fmt.Errorf("register GenerateReportHandler: %w", err)
	}

	if err := queryBus.RegisterHandler(application.NewGetOrderHandler(orderRepo)); err != nil {
		return nil, fmt.Errorf("register GetOrderHandler: %w", err)
	}
	if err := queryBus.RegisterHandler(application.NewListOrdersHandler(orderRepo)); err != nil {
		return nil, fmt.Errorf("register ListOrdersHandler: %w", err)
	}
	if err := queryBus.RegisterHandler(application.NewGetInventoryHandler(inventory)); err != nil {
		return nil, fmt.Errorf("register GetInventoryHandler: %w", err)
	}

	if err := eventBus.SubscribeHandler(application.NewOrderPlacedInventoryHandler(cmdBus)); err != nil {
		return nil, fmt.Errorf("register OrderPlacedInventoryHandler: %w", err)
	}
	if err := eventBus.SubscribeHandler(application.NewOrderPlacedNotificationHandler()); err != nil {
		return nil, fmt.Errorf("register OrderPlacedNotificationHandler: %w", err)
	}
	if err := eventBus.SubscribeHandler(application.NewOrderCancelledInventoryHandler(cmdBus)); err != nil {
		return nil, fmt.Errorf("register OrderCancelledInventoryHandler: %w", err)
	}

	jobManagerOpts := []jobmemory.JobManagerOption{
		jobmemory.WithStoreErrorHandler(func(ctx context.Context, storeErr *jobcore.StoreError) {
			logger.Error("job store error: %v", storeErr)
		}),
	}
	if recoveryEnabled {
		jobManagerOpts = append(jobManagerOpts, jobmemory.WithRecovery())
	}
	jobManager := jobmemory.NewJobManager(backend.JobStore, cmdBus, jobManagerOpts...)

	return &AppContext{
		Chain:            chain,
		CmdBus:           cmdBus,
		QueryBus:         queryBus,
		EventBus:         eventBus,
		Backend:          backend,
		JobManager:       jobManager,
		OrderRepo:        orderRepo,
		EventSourcedRepo: eventSourcedRepo,
		EventStore:       eventStore,
		Inventory:        inventory,
		MetricsRecorder:  metricsRecorder,
		TxManager:        txManager,
		store:            store,
	}, nil
}

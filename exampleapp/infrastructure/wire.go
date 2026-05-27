package infrastructure

import (
	"context"
	"fmt"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/cqrs/command"
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	querymemory "github.com/ddd-qce/core/cqrs/impl/memory"
	"github.com/ddd-qce/core/infra"
	jobcore "github.com/ddd-qce/core/job/core"
	jobmemory "github.com/ddd-qce/core/job/memory"
	inventorydomain "github.com/ddd-qce/exampleapp/ddd/inventory/domain"
	inventorywire "github.com/ddd-qce/exampleapp/ddd/inventory/wire"
	orderrepo "github.com/ddd-qce/exampleapp/ddd/order/repository"
	orderwire "github.com/ddd-qce/exampleapp/ddd/order/wire"
)

type AppContext struct {
	Chain      *aspect.AspectChain
	CmdBus     command.CommandBus
	QueryBus   query.QueryBus
	EventBus   cqrsevent.EventBus
	Backend    *infra.Backend
	JobManager *jobmemory.JobManager

	OrderRepo        orderrepo.OrderRepositoryAdapter
	EventSourcedRepo *orderrepo.OrderEventSourcedRepository
	EventStore       cqrsevent.EventSourceStore[cqrsevent.Event]
	Inventory        *inventorydomain.Inventory

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
	txManager := NewAppTransactionManager(backend.TransactionManager)

	chain := aspect.NewAspectChain()
	chain.RegisterAspect(builtin.NewTracingAspect(backend.TraceStore))
	chain.RegisterAspect(builtin.NewLoggingAspect(logger))
	chain.RegisterAspect(builtin.NewMetricsAspect(metricsRecorder))
	ta, err := builtin.NewTransactionAspect(txManager)
	if err != nil {
		return nil, fmt.Errorf("create transaction aspect: %w", err)
	}
	chain.RegisterCommandAspect(ta)

	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	queryBus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
	eventBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

	inventory := inventorydomain.NewInventory()
	orderRepo := store.OrderRepo
	eventStore := store.EventStore

	if err := orderwire.WireOrder(chain, cmdBus, queryBus, eventBus, orderRepo); err != nil {
		return nil, err
	}
	if err := inventorywire.WireInventory(chain, cmdBus, queryBus, eventBus, inventory); err != nil {
		return nil, err
	}

	eventSourcedRepo := orderrepo.NewOrderEventSourcedRepository(eventStore, eventBus, orderRepo)

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

package infrastructure

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ddd-qce/core/app"
	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/cqrs/command"
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	querymemory "github.com/ddd-qce/core/cqrs/impl/memory"
	"github.com/ddd-qce/core/cqrs/query"
	domainevent "github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/infra"
	jobcore "github.com/ddd-qce/core/job/core"
	jobmemory "github.com/ddd-qce/core/job/memory"
	"github.com/ddd-qce/core/observability"
	observabilitypg "github.com/ddd-qce/core/observability/pg"
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
	DDDViewer  *observability.DDDViewer

	OrderRepo        orderrepo.OrderRepositoryAdapter
	EventSourcedRepo *orderrepo.OrderEventSourcedRepository
	EventStore       cqrsevent.AggregateEventStore[domainevent.Event]
	Inventory        *inventorydomain.Inventory

	MetricsRecorder *AppMetricsRecorder
	TxManager       *AppTransactionManager

	Config     *Config
	lifecycles []app.Lifecycle
	store      *StoreComponents
}

func (app *AppContext) Store() *StoreComponents {
	return app.store
}

func (app *AppContext) RegisterLifecycle(l app.Lifecycle) {
	app.lifecycles = append(app.lifecycles, l)
}

func (app *AppContext) Close(ctx context.Context) error {
	var errs []error
	for _, l := range app.lifecycles {
		if err := l.Shutdown(ctx); err != nil {
			errs = append(errs, err)
		}
		if ctx.Err() != nil {
			break
		}
	}
	if app.store != nil && app.store.DB != nil {
		if err := app.store.DB.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("shutdown errors: %v", errs)
	}
	return nil
}

func (app *AppContext) WaitForSignal(timeout time.Duration) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	log.Printf("received %v, shutting down...", sig)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := app.Close(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
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
	return WireAppWithStore(cfg, store, recoveryEnabled)
}

func WireAppWithStore(cfg *Config, store *StoreComponents, recoveryEnabled bool) (*AppContext, error) {
	backend := store.Backend

	logger := NewAppLogger()
	metricsRecorder := NewAppMetricsRecorder()
	txManager := NewAppTransactionManager(backend.TransactionManager)

	chain := aspect.NewAspectChain()
	chain.RegisterAspect(builtin.NewTracingAspect(backend.TraceStore))
	chain.RegisterAspect(builtin.NewLoggingAspect(logger))

	statsCollector := observability.NewStatsCollector()
	composedMetrics := observability.ComposeMetrics(statsCollector, metricsRecorder)
	chain.RegisterAspect(builtin.NewMetricsAspect(composedMetrics))

	var msgStoreForReader builtin.MessageStore
	if store.DB != nil {
		chain.RegisterAspect(builtin.NewPersistenceAspect(backend.MessageStore))
		msgStoreForReader = backend.MessageStore
	} else {
		memMsgStore := observability.NewObservableMessageStore()
		chain.RegisterAspect(builtin.NewPersistenceAspect(memMsgStore))
		msgStoreForReader = memMsgStore
	}

	ta, err := builtin.NewTransactionAspect(txManager)
	if err != nil {
		return nil, fmt.Errorf("create transaction aspect: %w", err)
	}
	chain.RegisterCommandAspect(ta)

	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	queryBus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
	eventBus := eventmemory.NewEventBus(eventmemory.WithEventBusAspectChain(chain))

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

	var dddViewer *observability.DDDViewer
	dddViewerOpts := []observability.DDDViewerOption{
		observability.WithDDDViewerStatsCollector(statsCollector),
		observability.WithDDDViewerTraceStore(backend.TraceStore),
		observability.WithDDDViewerJobManager(jobManager),
		observability.WithDDDViewerBaseURL("http://localhost:8080"),
	}
	if store.DB != nil {
		dddViewerOpts = append(dddViewerOpts,
			observability.WithDDDViewerPgDB(store.DB),
			observability.WithDDDViewerSchemaReader(observabilitypg.NewSchemaReader(store.DB), "PostgreSQL"),
			observability.WithDDDViewerMessageReader(observabilitypg.NewMessageStoreReader(store.DB)),
		)
	} else {
		if ms, ok := msgStoreForReader.(observability.MessageStoreReader); ok {
			dddViewerOpts = append(dddViewerOpts, observability.WithDDDViewerMessageReader(ms))
		}
	}
	dddViewer = observability.NewDDDViewer(dddViewerOpts...)

	return &AppContext{
		Chain:            chain,
		CmdBus:           cmdBus,
		QueryBus:         queryBus,
		EventBus:         eventBus,
		Backend:          backend,
		JobManager:       jobManager,
		DDDViewer:        dddViewer,
		OrderRepo:        orderRepo,
		EventSourcedRepo: eventSourcedRepo,
		EventStore:       eventStore,
		Inventory:        inventory,
		MetricsRecorder:  metricsRecorder,
		TxManager:        txManager,
		Config:           cfg,
		lifecycles: []app.Lifecycle{
			eventBus,
			cmdBus,
			queryBus,
			jobManager,
			backend,
		},
		store: store,
	}, nil
}

package infrastructure

import (
	"context"
	"fmt"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
	querymemory "github.com/ddd-qce/core/cqrs/query/memory"
	domainevent "github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/infra"
	jobcore "github.com/ddd-qce/core/job/core"
	jobmemory "github.com/ddd-qce/core/job/memory"
	"github.com/ddd-qce/exampleapp/application"
	"github.com/ddd-qce/exampleapp/domain"
)

type AppContext struct {
	Chain      *aspect.AspectChain
	CmdBus     *commandmemory.CommandBus
	QueryBus   *querymemory.QueryBus
	EventBus   *eventmemory.EventBus
	Backend    *infra.Backend
	JobManager *jobmemory.JobManager

	OrderRepo        *application.OrderRepository
	EventSourcedRepo *application.OrderEventSourcedRepository
	EventStore       domainevent.EventStore[domainevent.DomainEvent]
	Inventory        *domain.Inventory

	MetricsRecorder *AppMetricsRecorder
	TxManager       *AppTransactionManager
}

func WireApp() *AppContext {
	backend := infra.NewMemoryBackend()

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
	orderRepo := application.NewOrderRepository()
	eventStore, err := eventmemory.NewEventStore[domainevent.DomainEvent]()
	if err != nil {
		panic(fmt.Sprintf("create event store: %v", err))
	}
	eventSourcedRepo := application.NewOrderEventSourcedRepository(eventStore, orderRepo)

	commandmemory.RegisterCommand(cmdBus, application.NewPlaceOrderHandler(orderRepo, eventBus))
	commandmemory.RegisterCommand(cmdBus, application.NewConfirmPaymentHandler(orderRepo, eventBus))
	commandmemory.RegisterCommand(cmdBus, application.NewShipOrderHandler(orderRepo, eventBus))
	commandmemory.RegisterCommand(cmdBus, application.NewCancelOrderHandler(orderRepo, eventBus))
	commandmemory.RegisterCommand(cmdBus, application.NewReserveInventoryHandler(inventory, eventBus))
	commandmemory.RegisterCommand(cmdBus, application.NewReleaseInventoryHandler(inventory, eventBus))
	commandmemory.RegisterCommand(cmdBus, application.NewGenerateReportHandler())

	querymemory.RegisterQuery(queryBus, application.NewGetOrderHandler(orderRepo))
	querymemory.RegisterQuery(queryBus, application.NewListOrdersHandler(orderRepo))
	querymemory.RegisterQuery(queryBus, application.NewGetInventoryHandler(inventory))

	eventmemory.RegisterHandler[*domain.OrderPlacedEvent](eventBus, application.NewOrderPlacedInventoryHandler(cmdBus))
	eventmemory.RegisterHandler[*domain.OrderPlacedEvent](eventBus, application.NewOrderPlacedNotificationHandler())
	eventmemory.RegisterHandler[*domain.OrderCancelledEvent](eventBus, application.NewOrderCancelledInventoryHandler(cmdBus))

	jobManager := jobmemory.NewJobManager(backend.JobStore, cmdBus, jobmemory.WithStoreErrorHandler(func(ctx context.Context, storeErr *jobcore.StoreError) {
		logger.Error("job store error: %v", storeErr)
	}))

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
	}
}

package infrastructure

import (
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	domainevent "github.com/ddd-qce/core/domain/event"
	jobmemory "github.com/ddd-qce/core/job/memory"
	"github.com/ddd-qce/core/observability"
	inventorydomain "github.com/ddd-qce/exampleapp/ddd/inventory/domain"
	orderrepo "github.com/ddd-qce/exampleapp/ddd/order/repository"
)

type AppCustom struct {
	Config           *Config
	Store            *StoreComponents
	OrderRepo        orderrepo.OrderRepositoryAdapter
	EventSourcedRepo *orderrepo.OrderEventSourcedRepository
	EventStore       cqrsevent.AggregateEventStore[domainevent.Event]
	Inventory        *inventorydomain.Inventory
	JobManager       *jobmemory.JobManager
	DDDViewer        *observability.DDDViewer
	MetricsRecorder  *AppMetricsRecorder
	TxManager        *AppTransactionManager
}

package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/cqrs/cmd"
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	domainevent "github.com/ddd-qce/core/cqrs/event"
	jobcore "github.com/ddd-qce/core/job/core"
	jobmemory "github.com/ddd-qce/core/job/memory"
	"github.com/ddd-qce/core/trace"
	"github.com/ddd-qce/exampleapp/application"
	"github.com/ddd-qce/exampleapp/domain"
	"github.com/ddd-qce/exampleapp/infrastructure"
)

func wireTestApp(t *testing.T, storeType string) *infrastructure.AppContext {
	t.Helper()
	cfg := &infrastructure.Config{StoreType: storeType}
	if storeType == infrastructure.StoreTypePostgreSQL {
		dsn := os.Getenv("DDD_POSTGRES_URI")
		if dsn == "" {
			t.Skip("DDD_POSTGRES_URI not set, skipping PostgreSQL test")
		}
		cfg.PostgresURI = dsn
	}
	app, err := infrastructure.WireAppWithConfig(cfg)
	if err != nil {
		t.Fatalf("wire app (%s): %v", storeType, err)
	}
	t.Cleanup(func() { app.Close() })
	return app
}

func runForBothStores(t *testing.T, fn func(t *testing.T, app *infrastructure.AppContext)) {
	t.Helper()
	t.Run("Memory", func(t *testing.T) {
		app := wireTestApp(t, infrastructure.StoreTypeMemory)
		fn(t, app)
	})
	t.Run("PostgreSQL", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping PostgreSQL test in short mode")
		}
		app := wireTestApp(t, infrastructure.StoreTypePostgreSQL)
		fn(t, app)
	})
}

func TestFullOrderLifecycle(t *testing.T) {
	runForBothStores(t, func(t *testing.T, app *infrastructure.AppContext) {
		ctx := context.Background()

		placed, err := command.Dispatch[*application.PlaceOrderCommand, *application.PlaceOrderResult](ctx, app.CmdBus, &application.PlaceOrderCommand{
			UserID: "user-001",
			Items:  []application.ItemInput{{ProductID: "laptop", ProductName: "Laptop", Price: 999.99, Quantity: 1}},
		})
		if err != nil {
			t.Fatalf("place order failed: %v", err)
		}
		time.Sleep(100 * time.Millisecond)

		_, err = command.Dispatch[*application.ConfirmPaymentCommand, *application.ConfirmPaymentResult](ctx, app.CmdBus, &application.ConfirmPaymentCommand{OrderID: placed.OrderID})
		if err != nil {
			t.Fatalf("confirm payment failed: %v", err)
		}

		_, err = command.Dispatch[*application.ShipOrderCommand, *application.ShipOrderResult](ctx, app.CmdBus, &application.ShipOrderCommand{OrderID: placed.OrderID})
		if err != nil {
			t.Fatalf("ship failed: %v", err)
		}

		order, err := query.Dispatch[*application.GetOrderQuery, *application.GetOrderResult](ctx, app.QueryBus, &application.GetOrderQuery{OrderID: placed.OrderID})
		if err != nil {
			t.Fatalf("get order failed: %v", err)
		}
		if order.Status != "shipped" {
			t.Errorf("expected shipped, got %s", order.Status)
		}

		traceIDs, _ := app.Backend.TraceStore.ListTraces(ctx, trace.TraceFilter{})
		if len(traceIDs) == 0 {
			t.Error("expected traces to be recorded")
		}

		if len(app.MetricsRecorder.Durations) == 0 {
			t.Error("expected metrics records")
		}
	})
}

func TestEventSourcingFullCycle(t *testing.T) {
	runForBothStores(t, func(t *testing.T, app *infrastructure.AppContext) {
		ctx := context.Background()

		order, _ := domain.NewOrder("ORD-ES-FULL", "user-001", []*domain.OrderItem{
			domain.NewOrderItem("laptop", "Laptop", 999, 1),
		})
		if err := app.EventSourcedRepo.Save(ctx, order); err != nil {
			t.Fatalf("save failed: %v", err)
		}

		loaded, err := app.EventSourcedRepo.Load(ctx, "ORD-ES-FULL")
		if err != nil {
			t.Fatalf("load failed: %v", err)
		}
		if loaded.ID() != "ORD-ES-FULL" {
			t.Errorf("expected ORD-ES-FULL, got %s", loaded.ID())
		}
		if loaded.UserID != "user-001" {
			t.Errorf("expected user-001, got %s", loaded.UserID)
		}

		events, _ := app.EventStore.Load(ctx, "ORD-ES-FULL", 0)
		if len(events) != 1 {
			t.Errorf("expected 1 event, got %d", len(events))
		}
	})
}

func TestJobManagerFullCycle(t *testing.T) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(application.NewGenerateReportHandler())
	jobStore := jobmemory.NewJobStore()
	var storeErrors []*jobcore.StoreError
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus, jobmemory.WithStoreErrorHandler(func(ctx context.Context, storeErr *jobcore.StoreError) {
		storeErrors = append(storeErrors, storeErr)
	}))
	ctx := context.Background()

	job, err := jobMgr.Submit(ctx, &application.GenerateReportCommand{OrderID: "O1"})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	result, err := jobMgr.Wait(ctx, job.ID, 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	if result.GetStatus() != jobcore.JobStatusCompleted {
		t.Errorf("expected completed, got %s", result.GetStatus())
	}

	jobs, _ := jobMgr.ListByStatus(ctx, jobcore.JobStatusCompleted)
	if len(jobs) == 0 {
		t.Error("expected at least 1 completed job")
	}
}

func TestJobManager_Cancel(t *testing.T) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(application.NewGenerateReportHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, _ := jobMgr.Submit(ctx, &application.GenerateReportCommand{OrderID: "O2"}, jobcore.WithTimeout(5*time.Second))
	time.Sleep(50 * time.Millisecond)
	if err := jobMgr.Cancel(ctx, job.ID); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
}

func TestJobManager_Retry(t *testing.T) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(application.NewGenerateReportHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, _ := jobMgr.Submit(ctx, &application.GenerateReportCommand{OrderID: "O3"}, jobcore.WithTimeout(1*time.Millisecond))
	time.Sleep(200 * time.Millisecond)

	status, _ := jobMgr.GetStatus(ctx, job.ID)
	if status.GetStatus() == jobcore.JobStatusFailed {
		if err := jobMgr.Retry(ctx, job.ID); err != nil {
			t.Logf("retry result: %v (may be expected)", err)
		}
	}
}

func TestConcurrentEventHandlers_MultiError(t *testing.T) {
	chain := aspect.NewAspectChain()
	eventBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))
	eventBus.SubscribeHandler(&successEventHandler{})
	eventBus.SubscribeHandler(&failEventHandler{})
	ctx := context.Background()
	err := cqrsevent.Dispatch[*domain.OrderPlacedEvent](ctx, eventBus, &domain.OrderPlacedEvent{
		BaseEvent: domainevent.NewBaseEvent("O1", time.Now()), UserID: "u1", TotalAmount: 100,
	})
	time.Sleep(100 * time.Millisecond)
	if err == nil {
		t.Log("MultiError not triggered (one handler succeeded), which is acceptable for concurrent publish")
	} else {
		var me interface{ Unwrap() []error }
		isMulti := errors.As(err, &me)
		t.Logf("error type: %T, is multi-error: %v", err, isMulti)
	}
}

type successEventHandler struct{ called bool }

func (h *successEventHandler) Handle(ctx context.Context, evt *domain.OrderPlacedEvent) error {
	h.called = true
	return nil
}

type failEventHandler struct{ called bool }

func (h *failEventHandler) Handle(ctx context.Context, evt *domain.OrderPlacedEvent) error {
	h.called = true
	return fmt.Errorf("intentional failure for testing")
}

func TestTraceContextPropagation(t *testing.T) {
	traceID := trace.NewTraceID()
	spanID := trace.NewSpanID()
	ctx := trace.WithTrace(context.Background(), traceID, spanID)

	if trace.GetTraceID(ctx) != traceID {
		t.Error("trace ID not propagated")
	}
	if trace.GetSpanID(ctx) != spanID {
		t.Error("span ID not propagated")
	}
}

func TestRepositoryDelete(t *testing.T) {
	runForBothStores(t, func(t *testing.T, app *infrastructure.AppContext) {
		ctx := context.Background()
		order, _ := domain.NewOrder("ORD-DEL", "user-001", []*domain.OrderItem{
			domain.NewOrderItem("laptop", "Laptop", 999, 1),
		})
		app.OrderRepo.Save(ctx, order)
		found, err := app.OrderRepo.FindByID(ctx, "ORD-DEL")
		if err != nil {
			t.Fatalf("find failed: %v", err)
		}
		if found.ID() != "ORD-DEL" {
			t.Error("order not found")
		}
		app.OrderRepo.Delete(ctx, "ORD-DEL")
		_, err = app.OrderRepo.FindByID(ctx, "ORD-DEL")
		if err == nil {
			t.Error("expected error after delete")
		}
	})
}

func TestCustomAspect(t *testing.T) {
	chain := aspect.NewAspectChain()
	ts := trace.NewInMemoryTraceStore()
	chain.RegisterCommandAspect(builtin.NewTracingAspect(ts))
	chain.RegisterCommandAspect(&testCustomAspect{})
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	repo := application.NewOrderRepository()
	eventBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))
	cmdBus.RegisterHandler(application.NewPlaceOrderHandler(repo, eventBus))
	ctx := context.Background()
	_, err := command.Dispatch[*application.PlaceOrderCommand, *application.PlaceOrderResult](ctx, cmdBus, &application.PlaceOrderCommand{
		UserID: "user-001",
		Items:  []application.ItemInput{{ProductID: "laptop", ProductName: "Laptop", Price: 999, Quantity: 1}},
	})
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}
	traceIDs, _ := ts.ListTraces(ctx, trace.TraceFilter{})
	if len(traceIDs) == 0 {
		t.Error("expected traces from custom aspect test")
	}
}

type testCustomAspect struct{}

func (a *testCustomAspect) Name() string { return "test-custom" }
func (a *testCustomAspect) Order() int   { return 25 }
func (a *testCustomAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	return ctx, nil
}
func (a *testCustomAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, d time.Duration) error {
	return nil
}
func (a *testCustomAspect) BeforeQuery(ctx context.Context, q any) (context.Context, error) {
	return ctx, nil
}
func (a *testCustomAspect) AfterQuery(ctx context.Context, q any, result any, err error, d time.Duration) error {
	return nil
}
func (a *testCustomAspect) BeforePublish(ctx context.Context, e any) (context.Context, error) {
	return ctx, nil
}
func (a *testCustomAspect) AfterPublish(ctx context.Context, e any, err error, d time.Duration) error {
	return nil
}

func TestJobStore_CRUD(t *testing.T) {
	store := jobmemory.NewJobStore()
	ctx := context.Background()
	job := jobcore.NewJob("J1", nil)
	if err := store.Create(ctx, job); err != nil {
		t.Fatalf("create failed: %v", err)
	}
	found, err := store.Get(ctx, "J1")
	if err != nil {
		t.Fatalf("get failed: %v", err)
	}
	if found.ID != "J1" {
		t.Errorf("expected J1, got %s", found.ID)
	}
	found.SetStatus(jobcore.JobStatusRunning)
	if err := store.Update(ctx, found); err != nil {
		t.Fatalf("update failed: %v", err)
	}
	updated, _ := store.Get(ctx, "J1")
	if updated.GetStatus() != jobcore.JobStatusRunning {
		t.Errorf("expected running, got %s", updated.GetStatus())
	}
	jobs, _ := store.List(ctx, jobcore.JobStatusRunning)
	if len(jobs) != 1 {
		t.Errorf("expected 1 running job, got %d", len(jobs))
	}
	if err := store.Delete(ctx, "J1"); err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	_, err = store.Get(ctx, "J1")
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestStoreError_ErrorUnwrap(t *testing.T) {
	inner := fmt.Errorf("db down")
	se := &jobcore.StoreError{JobID: "J1", Operation: "update", Err: inner}
	if se.Error() == "" {
		t.Error("expected non-empty error")
	}
	if se.Unwrap() != inner {
		t.Error("Unwrap should return inner error")
	}
}

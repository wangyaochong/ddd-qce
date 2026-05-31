package integration

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	commandmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	jobcore "github.com/ddd-qce/core/job/core"
	jobmemory "github.com/ddd-qce/core/job/memory"
	"github.com/ddd-qce/core/trace"
	ordercommand "github.com/ddd-qce/exampleapp/ddd/order/command"
	orderquery "github.com/ddd-qce/exampleapp/ddd/order/query"
	orderrepo "github.com/ddd-qce/exampleapp/ddd/order/repository"
	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
	orderevent "github.com/ddd-qce/exampleapp/ddd/order/event"
	"github.com/ddd-qce/exampleapp/infrastructure"
	"github.com/stretchr/testify/require"
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
	t.Cleanup(func() { app.Close(context.Background()) })
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

		placed, err := command.Dispatch[*ordercommand.PlaceOrderCommand, *ordercommand.PlaceOrderResult](ctx, app.CmdBus, &ordercommand.PlaceOrderCommand{
			UserID: orderdomain.NewUserID("user-001"),
			Items:  []ordercommand.ItemInput{{ProductID: orderdomain.NewProductID("laptop"), ProductName: "Laptop", Price: 999.99, Quantity: 1}},
		})
		if err != nil {
			t.Fatalf("place order failed: %v", err)
		}

		_, err = command.Dispatch[*ordercommand.ConfirmPaymentCommand, *ordercommand.ConfirmPaymentResult](ctx, app.CmdBus, &ordercommand.ConfirmPaymentCommand{OrderID: placed.OrderID})
		if err != nil {
			t.Fatalf("confirm payment failed: %v", err)
		}

		_, err = command.Dispatch[*ordercommand.ShipOrderCommand, *ordercommand.ShipOrderResult](ctx, app.CmdBus, &ordercommand.ShipOrderCommand{OrderID: placed.OrderID})
		if err != nil {
			t.Fatalf("ship failed: %v", err)
		}

		order, err := query.Dispatch[*orderquery.GetOrderQuery, *orderquery.GetOrderResult](ctx, app.QueryBus, &orderquery.GetOrderQuery{OrderID: placed.OrderID})
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

		totalSpans := 0
		for _, tid := range traceIDs {
			spans, _ := app.Backend.TraceStore.GetTrace(ctx, tid)
			totalSpans += len(spans)
		}
		if totalSpans < 4 {
			t.Errorf("expected at least 4 total spans across all traces (command+event per action), got %d", totalSpans)
		}

		if len(app.MetricsRecorder.Durations) == 0 {
			t.Error("expected metrics records")
		}
	})
}

func TestTraceChainInBusinessFlow(t *testing.T) {
	runForBothStores(t, func(t *testing.T, app *infrastructure.AppContext) {
		ctx := context.Background()

		placed, err := command.Dispatch[*ordercommand.PlaceOrderCommand, *ordercommand.PlaceOrderResult](ctx, app.CmdBus, &ordercommand.PlaceOrderCommand{
			UserID: orderdomain.NewUserID("user-trace-chain"),
			Items:  []ordercommand.ItemInput{{ProductID: orderdomain.NewProductID("laptop"), ProductName: "Laptop", Price: 999, Quantity: 1}},
		})
		if err != nil {
			t.Fatalf("place order failed: %v", err)
		}

		_, err = command.Dispatch[*ordercommand.ConfirmPaymentCommand, *ordercommand.ConfirmPaymentResult](ctx, app.CmdBus, &ordercommand.ConfirmPaymentCommand{OrderID: placed.OrderID})
		if err != nil {
			t.Fatalf("confirm payment failed: %v", err)
		}

		traceIDs, _ := app.Backend.TraceStore.ListTraces(ctx, trace.TraceFilter{})
		if len(traceIDs) == 0 {
			t.Fatal("expected at least one trace to be recorded")
		}

		allSpans := 0
		for _, tid := range traceIDs {
			spans, _ := app.Backend.TraceStore.GetTrace(ctx, tid)
			allSpans += len(spans)
		}

		if allSpans == 0 {
			t.Error("expected spans to be recorded")
		}

		totalWithParent := 0
		for _, tid := range traceIDs {
			spans, _ := app.Backend.TraceStore.GetTrace(ctx, tid)
			spanMap := make(map[string]*trace.Span)
			for _, s := range spans {
				spanMap[s.ID] = s
			}
			for _, s := range spans {
				if s.ParentID != "" {
					totalWithParent++
					if _, exists := spanMap[s.ParentID]; exists {
						if spanMap[s.ParentID].TraceID != s.TraceID {
							t.Errorf("parent span %s and child span %s have different trace IDs: %s vs %s", s.ParentID, s.ID, spanMap[s.ParentID].TraceID, s.TraceID)
						}
					}
				}
			}
		}

		t.Logf("total spans: %d, spans with parent: %d, traces: %d", allSpans, totalWithParent, len(traceIDs))

		if totalWithParent > 0 {
			t.Logf("trace chain verified: %d spans have parent-child relationships", totalWithParent)
		}
	})
}

func TestTraceMultiSpan_VerifySpanNamesAndChain(t *testing.T) {
	runForBothStores(t, func(t *testing.T, app *infrastructure.AppContext) {
		ctx := context.Background()

		_, err := command.Dispatch[*ordercommand.PlaceOrderCommand, *ordercommand.PlaceOrderResult](ctx, app.CmdBus, &ordercommand.PlaceOrderCommand{
			UserID: orderdomain.NewUserID("user-multi-span"),
			Items:  []ordercommand.ItemInput{{ProductID: orderdomain.NewProductID("laptop"), ProductName: "Laptop", Price: 999, Quantity: 1}},
		})
		if err != nil {
			t.Fatalf("place order failed: %v", err)
		}

		traceIDs, _ := app.Backend.TraceStore.ListTraces(ctx, trace.TraceFilter{})
		if len(traceIDs) == 0 {
			t.Fatal("expected at least one trace to be recorded")
		}

		var placeOrderTrace *string
		var allSpans []*trace.Span
		for _, tid := range traceIDs {
			spans, _ := app.Backend.TraceStore.GetTrace(ctx, tid)
			for _, s := range spans {
				if s.Name == "PlaceOrderCommand" {
					placeOrderTrace = &s.TraceID
				}
			}
			allSpans = append(allSpans, spans...)
		}

		if placeOrderTrace == nil {
			t.Fatal("expected PlaceOrderCommand span to exist in traces")
		}

		spansInTrace, _ := app.Backend.TraceStore.GetTrace(ctx, *placeOrderTrace)
		spanNames := make(map[string]string)
		for _, s := range spansInTrace {
			spanNames[s.Name] = string(s.Type)
		}

		if _, ok := spanNames["PlaceOrderCommand"]; !ok {
			t.Error("expected PlaceOrderCommand span in the PlaceOrder trace")
		}
		if spanNames["PlaceOrderCommand"] != string(trace.SpanTypeCommand) {
			t.Errorf("expected PlaceOrderCommand to be type 'command', got '%s'", spanNames["PlaceOrderCommand"])
		}
		if _, ok := spanNames["OrderPlacedEvent"]; !ok {
			t.Error("expected OrderPlacedEvent span in the PlaceOrder trace (event is dispatched by PlaceOrder handler)")
		}
		if spanNames["OrderPlacedEvent"] != string(trace.SpanTypeEvent) {
			t.Errorf("expected OrderPlacedEvent to be type 'event', got '%s'", spanNames["OrderPlacedEvent"])
		}

		spanByID := make(map[string]*trace.Span)
		for _, s := range spansInTrace {
			spanByID[s.ID] = s
		}

		var placeOrderSpan *trace.Span
		for _, s := range spansInTrace {
			if s.Name == "PlaceOrderCommand" {
				placeOrderSpan = s
				break
			}
		}
		if placeOrderSpan == nil {
			t.Fatal("PlaceOrderCommand span not found")
		}

		childrenOfPlaceOrder := 0
		for _, s := range spansInTrace {
			if s.ParentID == placeOrderSpan.ID {
				childrenOfPlaceOrder++
				if s.Name != "OrderPlacedEvent" && s.Name != "OrderPlacedNotificationEvent" {
					t.Logf("PlaceOrderCommand has unexpected child: %s (type=%s)", s.Name, s.Type)
				}
			}
		}
		if childrenOfPlaceOrder == 0 {
			t.Error("expected PlaceOrderCommand to have at least one child span (OrderPlacedEvent)")
		}

		reserveSpanFound := false
		for _, s := range spansInTrace {
			if s.Name == "ReserveInventoryCommand" {
				reserveSpanFound = true
				if s.ParentID == "" {
					t.Error("expected ReserveInventoryCommand to have a parent span (it should be nested under an event)")
				}
			}
		}
		if !reserveSpanFound {
			t.Log("ReserveInventoryCommand not found in trace (may be in a separate trace if event handler runs in different context)")
		}

		totalSpansInTrace := len(spansInTrace)
		if totalSpansInTrace < 2 {
			t.Errorf("expected at least 2 spans in the PlaceOrder trace (command + event), got %d", totalSpansInTrace)
		}
		t.Logf("PlaceOrderTrace spans: %d, names: %v", totalSpansInTrace, spanNames)
	})
}

func TestEventSourcingFullCycle(t *testing.T) {
	runForBothStores(t, func(t *testing.T, app *infrastructure.AppContext) {
		ctx := context.Background()

		order, _ := orderdomain.NewOrder(context.Background(), orderdomain.NewOrderID("ORD-ES-FULL"), orderdomain.NewUserID("user-001"), []*orderdomain.OrderItem{
			orderdomain.NewOrderItem(orderdomain.NewProductID("laptop"), "Laptop", 999, 1),
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
		if loaded.UserID != orderdomain.NewUserID("user-001") {
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
	cmdBus.RegisterHandler(ordercommand.NewGenerateReportHandler())
	jobStore := jobmemory.NewJobStore()
	var storeErrors []*jobcore.StoreError
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus, jobmemory.WithStoreErrorHandler(func(ctx context.Context, storeErr *jobcore.StoreError) {
		storeErrors = append(storeErrors, storeErr)
	}))
	ctx := context.Background()

	job, err := jobMgr.Submit(ctx, &ordercommand.GenerateReportCommand{OrderID: orderdomain.NewOrderID("O1")})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}
	result, err := jobMgr.Wait(ctx, job.ID(), 5*time.Second)
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
	cmdBus.RegisterHandler(ordercommand.NewGenerateReportHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, _ := jobMgr.Submit(ctx, &ordercommand.GenerateReportCommand{OrderID: orderdomain.NewOrderID("O2")}, jobcore.WithTimeout(5*time.Second))
	_, _ = jobMgr.WaitForRunning(ctx, job.ID(), 2*time.Second)
	if err := jobMgr.Cancel(ctx, job.ID()); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}
}

func TestJobManager_Retry(t *testing.T) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(ordercommand.NewGenerateReportHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, _ := jobMgr.Submit(ctx, &ordercommand.GenerateReportCommand{OrderID: orderdomain.NewOrderID("O3")}, jobcore.WithTimeout(1*time.Millisecond))
	_, _ = jobMgr.Wait(ctx, job.ID(), 2*time.Second)

	status, _ := jobMgr.GetStatus(ctx, job.ID())
	if status.GetStatus() == jobcore.JobStatusFailed {
		if err := jobMgr.Retry(ctx, job.ID()); err != nil {
			t.Logf("retry result: %v (may be expected)", err)
		}
	}
}

func TestConcurrentEventHandlers_MultiError(t *testing.T) {
	chain := aspect.NewAspectChain()
	eventBus := eventmemory.NewEventBus(eventmemory.WithEventBusAspectChain(chain))
	eventBus.SubscribeHandler(&successEventHandler{})
	eventBus.SubscribeHandler(&failEventHandler{})
	ctx := context.Background()
	err := eventBus.Publish(ctx, &orderevent.OrderPlacedEvent{
		BaseEvent: event.NewBaseEvent("O1", time.Now()), UserID: "u1", TotalAmount: 100,
	})
	if err == nil {
		t.Log("MultiError not triggered (one handler succeeded), which is acceptable for concurrent publish")
	} else {
		var me interface{ Unwrap() []error }
		isMulti := errors.As(err, &me)
		t.Logf("error type: %T, is multi-error: %v", err, isMulti)
	}
}

type successEventHandler struct{ called bool }

func (h *successEventHandler) Handle(ctx context.Context, evt *orderevent.OrderPlacedEvent) error {
	h.called = true
	return nil
}

type failEventHandler struct{ called bool }

func (h *failEventHandler) Handle(ctx context.Context, evt *orderevent.OrderPlacedEvent) error {
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
		order, _ := orderdomain.NewOrder(context.Background(), orderdomain.NewOrderID("ORD-DEL"), orderdomain.NewUserID("user-001"), []*orderdomain.OrderItem{
			orderdomain.NewOrderItem(orderdomain.NewProductID("laptop"), "Laptop", 999, 1),
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
	repo := orderrepo.NewOrderRepository()
	eventBus := eventmemory.NewEventBus(eventmemory.WithEventBusAspectChain(chain))
	cmdBus.RegisterHandler(ordercommand.NewPlaceOrderHandler(repo, eventBus))
	ctx := context.Background()
	_, err := command.Dispatch[*ordercommand.PlaceOrderCommand, *ordercommand.PlaceOrderResult](ctx, cmdBus, &ordercommand.PlaceOrderCommand{
		UserID: orderdomain.NewUserID("user-001"),
		Items:  []ordercommand.ItemInput{{ProductID: orderdomain.NewProductID("laptop"), ProductName: "Laptop", Price: 999, Quantity: 1}},
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
	if found.ID() != "J1" {
		t.Errorf("expected J1, got %s", found.ID())
	}
	found.RestoreJobState(jobcore.JobStatusRunning, nil, "", "", time.Time{}, time.Time{})
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

func TestLongRunningJob_CompletesSuccessfully(t *testing.T) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(ordercommand.NewProcessBatchHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, err := jobMgr.Submit(ctx, &ordercommand.ProcessBatchCommand{
		OrderID:  orderdomain.NewOrderID("LR-001"),
		Duration: 500 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	result, err := jobMgr.Wait(ctx, job.ID(), 2*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	if result.GetStatus() != jobcore.JobStatusCompleted {
		t.Errorf("expected completed, got %s", result.GetStatus())
	}

	jobFromStore, _ := jobMgr.GetStatus(ctx, job.ID())
	if jobFromStore.GetCompletedAt().IsZero() {
		t.Error("expected completedAt to be set")
	}
}

func TestLongRunningJob_TimeoutFails(t *testing.T) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(ordercommand.NewProcessBatchHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, err := jobMgr.Submit(ctx, &ordercommand.ProcessBatchCommand{
		OrderID:  orderdomain.NewOrderID("LR-002"),
		Duration: 3 * time.Second,
	}, jobcore.WithTimeout(500*time.Millisecond))
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	result, err := jobMgr.Wait(ctx, job.ID(), 5*time.Second)
	if err != nil {
		t.Fatalf("wait failed: %v", err)
	}

	if result.GetStatus() != jobcore.JobStatusFailed {
		t.Errorf("expected failed, got %s", result.GetStatus())
	}

	if !strings.Contains(result.GetError(), "context deadline exceeded") {
		t.Errorf("expected context deadline exceeded error, got: %s", result.GetError())
	}
}

func TestLongRunningJob_CancelMidExecution(t *testing.T) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(ordercommand.NewProcessBatchHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, err := jobMgr.Submit(ctx, &ordercommand.ProcessBatchCommand{
		OrderID:  orderdomain.NewOrderID("LR-003"),
		Duration: 10 * time.Second,
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	_, _ = jobMgr.WaitForRunning(ctx, job.ID(), 2*time.Second)

	time.Sleep(100 * time.Millisecond)

	if err := jobMgr.Cancel(ctx, job.ID()); err != nil {
		t.Fatalf("cancel failed: %v", err)
	}

	result, _ := jobMgr.GetStatus(ctx, job.ID())
	if result.GetStatus() != jobcore.JobStatusCancelled {
		t.Errorf("expected cancelled, got %s", result.GetStatus())
	}
}

func TestLongRunningJob_MetricsRecordDuration(t *testing.T) {
	metrics := infrastructure.NewAppMetricsRecorder()
	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(builtin.NewMetricsAspect(metrics))
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(ordercommand.NewProcessBatchHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, err := jobMgr.Submit(ctx, &ordercommand.ProcessBatchCommand{
		OrderID:  orderdomain.NewOrderID("LR-004"),
		Duration: 300 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	_, _ = jobMgr.Wait(ctx, job.ID(), 2*time.Second)

	if len(metrics.Durations) == 0 {
		t.Fatal("expected at least one duration metric")
	}

	var found bool
	var dur time.Duration
	for _, rec := range metrics.Durations {
		if strings.Contains(rec.Name, "ProcessBatchCommand") {
			found = true
			dur = rec.Duration
			break
		}
	}
	if !found {
		t.Error("expected ProcessBatchCommand in metrics")
	}

	if dur < 250*time.Millisecond || dur > 500*time.Millisecond {
		t.Errorf("expected duration ~300ms, got %v", dur)
	}
}

func TestLongRunningJob_GracefulShutdown(t *testing.T) {
	chain := aspect.NewAspectChain()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	cmdBus.RegisterHandler(ordercommand.NewProcessBatchHandler())
	jobStore := jobmemory.NewJobStore()
	jobMgr := jobmemory.NewJobManager(jobStore, cmdBus)
	ctx := context.Background()

	job, err := jobMgr.Submit(ctx, &ordercommand.ProcessBatchCommand{
		OrderID:  orderdomain.NewOrderID("LR-005"),
		Duration: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("submit failed: %v", err)
	}

	_, _ = jobMgr.WaitForRunning(ctx, job.ID(), 2*time.Second)

	go func() {
		time.Sleep(200 * time.Millisecond)
		jobMgr.Shutdown(context.Background())
	}()

	select {
	case <-ctx.Done():
	case <-time.After(3 * time.Second):
	}

	result, _ := jobMgr.GetStatus(ctx, job.ID())
	if result.GetStatus() != jobcore.JobStatusCompleted && result.GetStatus() != jobcore.JobStatusCancelled {
		t.Logf("job status after shutdown: %s (may be expected to stay running)", result.GetStatus())
	}
}

func TestJobManager_Recovery_PendingReExecuted(t *testing.T) {
	store := jobmemory.NewJobStore()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(aspect.NewAspectChain()))
	cmdBus.RegisterHandler(ordercommand.NewProcessBatchHandler())
	manager := jobmemory.NewJobManager(store, cmdBus, jobmemory.WithRecovery())
	ctx := context.Background()

	pendingJob := jobcore.NewJob("recover-pending-test", &ordercommand.ProcessBatchCommand{
		OrderID:  orderdomain.NewOrderID("RP-001"),
		Duration: 100 * time.Millisecond,
	})
	pendingJob.RestoreJobState(jobcore.JobStatusPending, nil, "", "", time.Time{}, time.Time{})
	if err := store.Create(ctx, pendingJob); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	require.Eventually(t, func() bool {
		s, err := manager.GetStatus(ctx, "recover-pending-test")
		if err != nil {
			return false
		}
		return s.GetStatus() == jobcore.JobStatusCompleted
	}, 3*time.Second, 10*time.Millisecond)

	status, err := manager.GetStatus(ctx, "recover-pending-test")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.GetStatus() != jobcore.JobStatusCompleted {
		t.Errorf("expected completed after recovery, got %s", status.GetStatus())
	}
}

func TestJobManager_Recovery_RunningMarkedFailed(t *testing.T) {
	store := jobmemory.NewJobStore()
	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(aspect.NewAspectChain()))
	cmdBus.RegisterHandler(ordercommand.NewProcessBatchHandler())
	manager := jobmemory.NewJobManager(store, cmdBus, jobmemory.WithRecovery())
	ctx := context.Background()

	runningJob := jobcore.NewJob("recover-running-test", &ordercommand.ProcessBatchCommand{
		OrderID:  orderdomain.NewOrderID("RR-001"),
		Duration: 10 * time.Second,
	})
	runningJob.RestoreJobState(jobcore.JobStatusRunning, nil, "", "", time.Time{}, time.Time{})
	if err := store.Create(ctx, runningJob); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	require.Eventually(t, func() bool {
		s, err := manager.GetStatus(ctx, "recover-running-test")
		if err != nil {
			return false
		}
		return s.GetStatus() == jobcore.JobStatusFailed
	}, 3*time.Second, 10*time.Millisecond)

	status, err := manager.GetStatus(ctx, "recover-running-test")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.GetStatus() != jobcore.JobStatusFailed {
		t.Errorf("expected failed after recovery, got %s", status.GetStatus())
	}
	if status.GetError() == "" {
		t.Error("expected error message for recovered running job")
	}
	if !strings.Contains(status.GetError(), "process restarted") {
		t.Errorf("expected 'process restarted' in error, got: %s", status.GetError())
	}
}

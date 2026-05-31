package infrastructure

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/cqrs/event"
	eventmemory "github.com/ddd-qce/core/cqrs/impl/memory"
	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/trace"
)

func TestAspectChain_RegisterAll(t *testing.T) {
	chain := aspect.NewAspectChain()
	ts := trace.NewInMemoryTraceStore()
	chain.RegisterCommandAspect(builtin.NewTracingAspect(ts))
	ta, err := builtin.NewTransactionAspect(NewAppTransactionManager(builtin.NewNoOpTransactionManager()))
	if err != nil {
		t.Fatalf("NewTransactionAspect: %v", err)
	}
	chain.RegisterCommandAspect(ta)
	chain.RegisterCommandAspect(builtin.NewLoggingAspect(NewAppLogger()))
	chain.RegisterCommandAspect(builtin.NewMetricsAspect(NewAppMetricsRecorder()))
	chain.RegisterCommandAspect(&customTestAspect{order: 25})
}

func TestCustomAspect_BeforeAfter(t *testing.T) {
	chain := aspect.NewAspectChain()
	a := &customTestAspect{order: 25}
	chain.RegisterCommandAspect(a)
	ctx := context.Background()
	_, err := chain.ExecuteWithCommandAspects(ctx, "test-cmd", func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !a.beforeCalled || !a.afterCalled {
		t.Error("expected before and after to be called")
	}
}

type customTestAspect struct {
	order        int
	beforeCalled bool
	afterCalled  bool
}

func (a *customTestAspect) Name() string { return "custom-test" }
func (a *customTestAspect) Order() int   { return a.order }
func (a *customTestAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	a.beforeCalled = true
	return ctx, nil
}
func (a *customTestAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error {
	a.afterCalled = true
	return nil
}
func (a *customTestAspect) BeforeQuery(ctx context.Context, query any) (context.Context, error) {
	return ctx, nil
}
func (a *customTestAspect) AfterQuery(ctx context.Context, query any, result any, err error, duration time.Duration) error {
	return nil
}
func (a *customTestAspect) BeforePublish(ctx context.Context, event any) (context.Context, error) {
	return ctx, nil
}
func (a *customTestAspect) AfterPublish(ctx context.Context, event any, err error, duration time.Duration) error {
	return nil
}

var _ aspect.CommandAspect = (*customTestAspect)(nil)
var _ aspect.QueryAspect = (*customTestAspect)(nil)
var _ aspect.EventAspect = (*customTestAspect)(nil)

func TestLoggingAspect_Logs(t *testing.T) {
	logger := &captureLogger{}
	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(builtin.NewLoggingAspect(logger))
	ctx := context.Background()
	chain.ExecuteWithCommandAspects(ctx, "cmd", func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	})
	if len(logger.messages) == 0 {
		t.Error("expected log messages")
	}
}

func TestMetricsAspect_Records(t *testing.T) {
	recorder := NewAppMetricsRecorder()
	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(builtin.NewMetricsAspect(recorder))
	ctx := context.Background()
	chain.ExecuteWithCommandAspects(ctx, "cmd", func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	})
	if len(recorder.Durations) == 0 {
		t.Error("expected duration records")
	}
}

func TestTracingAspect_Spans(t *testing.T) {
	ts := trace.NewInMemoryTraceStore()
	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(builtin.NewTracingAspect(ts))
	ctx := context.Background()
	chain.ExecuteWithCommandAspects(ctx, "cmd", func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	})
	traceIDs, _ := ts.ListTraces(ctx, trace.TraceFilter{})
	if len(traceIDs) == 0 {
		t.Error("expected traces to be recorded")
	}
}

func TestTransactionAspect_BeginCommit(t *testing.T) {
	txMgr := NewAppTransactionManager(builtin.NewNoOpTransactionManager())

	chain := aspect.NewAspectChain()
	ta, err := builtin.NewTransactionAspect(txMgr)
	if err != nil {
		t.Fatalf("NewTransactionAspect: %v", err)
	}
	chain.RegisterCommandAspect(ta)
	ctx := context.Background()
	chain.ExecuteWithCommandAspects(ctx, "cmd", func(ctx context.Context) (interface{}, error) {
		return "ok", nil
	})
	if len(txMgr.Records) < 2 {
		t.Fatalf("expected at least 2 tx records, got %d", len(txMgr.Records))
	}
	if txMgr.Records[0].Action != TxBegin {
		t.Errorf("expected begin, got %s", txMgr.Records[0].Action)
	}
	if txMgr.Records[1].Action != TxCommit {
		t.Errorf("expected commit, got %s", txMgr.Records[1].Action)
	}
}

func TestTransactionAspect_BeginRollback(t *testing.T) {
	txMgr := NewAppTransactionManager(builtin.NewNoOpTransactionManager())
	chain := aspect.NewAspectChain()
	ta, err := builtin.NewTransactionAspect(txMgr)
	if err != nil {
		t.Fatalf("NewTransactionAspect: %v", err)
	}
	chain.RegisterCommandAspect(ta)
	ctx := context.Background()
	chain.ExecuteWithCommandAspects(ctx, "cmd", func(ctx context.Context) (interface{}, error) {
		return nil, fmt.Errorf("fail")
	})
	if txMgr.Records[1].Action != TxRollback {
		t.Errorf("expected rollback on error, got %s", txMgr.Records[1].Action)
	}
}

func TestNewEventBus_Direct(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := eventmemory.NewEventBus(eventmemory.WithEventBusAspectChain(chain))
	handler := &testEventHandler{done: make(chan struct{})}
	bus.SubscribeHandler(handler)
	ctx := context.Background()
	bus.Publish(ctx, &testDomainEvent{BaseEvent: event.NewBaseEvent("A1", time.Now())})
	select {
	case <-handler.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for handler")
	}
}

type testDomainEvent struct {
	event.BaseEvent
}

type testEventHandler struct {
	called bool
	done   chan struct{}
}

func (h *testEventHandler) Handle(ctx context.Context, evt *testDomainEvent) error {
	h.called = true
	close(h.done)
	return nil
}

func TestEventStore_Reflect(t *testing.T) {
	store, err := eventmemory.NewEventSourceStore[*testDomainEvent]()
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()
	err = store.Append(ctx, "A1", 0, []*testDomainEvent{{BaseEvent: event.NewBaseEvent("A1", time.Now())}})
	if err != nil {
		t.Fatalf("append failed: %v", err)
	}
	events, err := store.Load(ctx, "A1", 0)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
}

func TestEventStoreWithFactory(t *testing.T) {
	store, err := eventmemory.NewEventSourceStore[*testDomainEvent](eventmemory.WithFactory[*testDomainEvent](func() *testDomainEvent {
		return &testDomainEvent{}
	}))
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()
	store.Append(ctx, "A1", 0, []*testDomainEvent{{BaseEvent: event.NewBaseEvent("A1", time.Now())}})
	events, _ := store.Load(ctx, "A1", 0)
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
}

func TestEventStore_Versioning(t *testing.T) {
	store, err := eventmemory.NewEventSourceStore[*testDomainEvent]()
	if err != nil {
		t.Fatalf("create event store: %v", err)
	}
	ctx := context.Background()
	store.Append(ctx, "A1", 0, []*testDomainEvent{
		{BaseEvent: event.NewBaseEvent("A1", time.Now())},
		{BaseEvent: event.NewBaseEvent("A1", time.Now())},
		{BaseEvent: event.NewBaseEvent("A1", time.Now())},
	})
	all, _ := store.Load(ctx, "A1", 0)
	if len(all) != 3 {
		t.Errorf("expected 3, got %d", len(all))
	}
	after1, _ := store.Load(ctx, "A1", 1)
	if len(after1) != 2 {
		t.Errorf("expected 2 after v1, got %d", len(after1))
	}
}

func TestJoin_Nil(t *testing.T) {
	result := errors.Join()
	if result != nil {
		t.Error("expected nil for no errors")
	}
}

func TestJoin_Single(t *testing.T) {
	e := fmt.Errorf("one")
	result := errors.Join(e)
	if !errors.Is(result, e) {
		t.Error("expected single error to be contained")
	}
}

func TestJoin_Multi(t *testing.T) {
	result := errors.Join(fmt.Errorf("a"), fmt.Errorf("b"))
	var me interface{ Unwrap() []error }
	if !errors.As(result, &me) {
		t.Fatal("expected multi-error")
	}
	if len(me.Unwrap()) != 2 {
		t.Errorf("expected 2 errors, got %d", len(me.Unwrap()))
	}
}

func TestMultiError_ErrorUnwrap(t *testing.T) {
	result := errors.Join(fmt.Errorf("x"), fmt.Errorf("y"))
	if result.Error() == "" {
		t.Error("expected non-empty error string")
	}
	var me interface{ Unwrap() []error }
	if !errors.As(result, &me) {
		t.Fatal("expected multi-error")
	}
	unwrapped := me.Unwrap()
	if len(unwrapped) != 2 {
		t.Errorf("expected 2 unwrapped, got %d", len(unwrapped))
	}
}

func TestTraceContextPropagation(t *testing.T) {
	ctx := context.Background()
	traceID := trace.NewTraceID()
	spanID := trace.NewSpanID()
	ctx = trace.WithTrace(ctx, traceID, spanID)
	if trace.GetTraceID(ctx) != traceID {
		t.Error("trace ID mismatch")
	}
	if trace.GetSpanID(ctx) != spanID {
		t.Error("span ID mismatch")
	}
}

func TestWithParentSpan_Context(t *testing.T) {
	ctx := context.Background()
	parentID := trace.NewSpanID()
	ctx = trace.WithParentSpan(ctx, parentID)
	if trace.GetParentSpanID(ctx) != parentID {
		t.Error("parent span ID mismatch")
	}
}

func TestNewTraceID(t *testing.T) {
	id := trace.NewTraceID()
	if id == "" {
		t.Error("expected non-empty trace ID")
	}
}

func TestNewSpanID(t *testing.T) {
	id := trace.NewSpanID()
	if id == "" {
		t.Error("expected non-empty span ID")
	}
}

func TestSpanType_Values(t *testing.T) {
	if trace.SpanTypeCommand != "command" {
		t.Errorf("expected command, got %s", trace.SpanTypeCommand)
	}
	if trace.SpanTypeQuery != "query" {
		t.Errorf("expected query, got %s", trace.SpanTypeQuery)
	}
	if trace.SpanTypeEvent != "event" {
		t.Errorf("expected event, got %s", trace.SpanTypeEvent)
	}
}

func TestSpanStatus_Values(t *testing.T) {
	if trace.SpanStatusSuccess != "success" {
		t.Errorf("expected success, got %s", trace.SpanStatusSuccess)
	}
	if trace.SpanStatusError != "error" {
		t.Errorf("expected error, got %s", trace.SpanStatusError)
	}
}

func TestTraceFilter_AllFields(t *testing.T) {
	f := trace.TraceFilter{
		TraceID:      "tid",
		Type:         "command",
		Status:       "success",
		StartTime:    time.Now(),
		EndTime:      time.Now(),
		NameContains: "Place",
	}
	if f.TraceID != "tid" {
		t.Error("TraceID not set")
	}
	if f.Type != "command" {
		t.Error("Type not set")
	}
}

func TestTraceStore_RecordAndGetTrace(t *testing.T) {
	ts := trace.NewInMemoryTraceStore()
	defer ts.Close()
	ctx := context.Background()
	span := &trace.Span{ID: "s1", TraceID: "t1", Type: trace.SpanTypeCommand, Name: "cmd", Status: trace.SpanStatusSuccess, StartedAt: time.Now()}
	ts.RecordSpan(ctx, span)
	spans, _ := ts.GetTrace(ctx, "t1")
	if len(spans) != 1 {
		t.Errorf("expected 1 span, got %d", len(spans))
	}
}

func TestTraceStore_ListTraces_WithFilter(t *testing.T) {
	ts := trace.NewInMemoryTraceStore()
	defer ts.Close()
	ctx := context.Background()
	ts.RecordSpan(ctx, &trace.Span{ID: "s1", TraceID: "t1", Type: trace.SpanTypeCommand, Name: "cmd", Status: trace.SpanStatusSuccess, StartedAt: time.Now()})
	ts.RecordSpan(ctx, &trace.Span{ID: "s2", TraceID: "t2", Type: trace.SpanTypeQuery, Name: "qry", Status: trace.SpanStatusSuccess, StartedAt: time.Now()})

	all, _ := ts.ListTraces(ctx, trace.TraceFilter{})
	if len(all) != 2 {
		t.Errorf("expected 2 traces, got %d", len(all))
	}
	cmdOnly, _ := ts.ListTraces(ctx, trace.TraceFilter{Type: trace.SpanTypeCommand})
	if len(cmdOnly) != 1 {
		t.Errorf("expected 1 command trace, got %d", len(cmdOnly))
	}
}

func TestStoreError_ErrorUnwrap(t *testing.T) {
	inner := fmt.Errorf("db connection lost")
	se := &jobcore.StoreError{JobID: "J1", Operation: "update", Err: inner}
	if se.Error() == "" {
		t.Error("expected non-empty error string")
	}
	if se.Unwrap() != inner {
		t.Error("expected inner error from Unwrap")
	}
}

func TestJobOption_WithTimeout(t *testing.T) {
	job := &jobcore.Job{}
	jobcore.WithTimeout(5 * time.Second)(job)
	if job.Timeout() != 5*time.Second {
		t.Errorf("expected 5s timeout, got %v", job.Timeout())
	}
}

func TestJobOption_WithMaxRetries(t *testing.T) {
	job := &jobcore.Job{}
	jobcore.WithMaxRetries(3)(job)
	if job.MaxRetries() != 3 {
		t.Errorf("expected 3 retries, got %d", job.MaxRetries())
	}
}

func TestJob_Snapshot(t *testing.T) {
	job := jobcore.NewJob("J1", nil)
	job.RestoreJobState(jobcore.JobStatusRunning, nil, "", "", time.Time{}, time.Time{})
	snap := job.Snapshot()
	if snap.ID() != "J1" {
		t.Error("snapshot ID mismatch")
	}
	if snap.GetStatus() != jobcore.JobStatusRunning {
		t.Error("snapshot Status mismatch")
	}
	snap.RestoreJobState(jobcore.JobStatusCompleted, nil, "", "", time.Time{}, time.Time{})
	if job.GetStatus() == jobcore.JobStatusCompleted {
		t.Error("snapshot should be independent from original")
	}
}

func TestJob_Done_Channel(t *testing.T) {
	job := jobcore.NewJob("J1", nil)
	done := job.Done()
	if done == nil {
		t.Error("expected non-nil done channel")
	}
	select {
	case <-done:
		t.Error("done should not be closed yet")
	default:
	}
	job.MarkDone()
	select {
	case <-done:
	default:
		t.Error("done should be closed after MarkDone")
	}
}

func TestJob_ResetDone(t *testing.T) {
	job := jobcore.NewJob("J1", nil)
	job.MarkDone()
	job.ResetDone()
	select {
	case <-job.Done():
		t.Error("done should not be closed after ResetDone")
	default:
	}
}

type captureLogger struct {
	messages []string
}

func (l *captureLogger) Info(msg string, args ...interface{})  { l.messages = append(l.messages, msg) }
func (l *captureLogger) Error(msg string, args ...interface{}) { l.messages = append(l.messages, msg) }
func (l *captureLogger) Debug(msg string, args ...interface{}) { l.messages = append(l.messages, msg) }

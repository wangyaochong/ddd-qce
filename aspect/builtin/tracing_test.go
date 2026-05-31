package builtin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ddd-qce/core/cqrs/event"
	ddderror "github.com/ddd-qce/core/error"
	"github.com/ddd-qce/core/trace"
)

type tracingTestCommand struct {
	Name string
}

type tracingTestQuery struct {
	ID string
}

type tracingTestEvent struct {
	event.BaseEvent
}

func TestTracingAspect_NameAndOrder(t *testing.T) {
	aspect := &TracingAspect{store: trace.NewInMemoryTraceStore()}

	if aspect.Name() != "tracing" {
		t.Errorf("expected name 'tracing', got '%s'", aspect.Name())
	}
	if aspect.Order() != 0 {
		t.Errorf("expected order 0, got %d", aspect.Order())
	}
}

func TestTracingAspect_BeforeCommand_CreatesSpan(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := &TracingAspect{store: store}

	ctx := context.Background()
	cmd := &tracingTestCommand{Name: "test"}

	newCtx, err := aspect.BeforeCommand(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceID := trace.GetTraceID(newCtx)
	if traceID == "" {
		t.Error("expected trace ID to be set")
	}

	spanID := trace.GetSpanID(newCtx)
	if spanID == "" {
		t.Error("expected span ID to be set")
	}
}

func TestTracingAspect_BeforeCommand_ExistingTrace(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := &TracingAspect{store: store}

	ctx := context.Background()
	ctx = trace.WithTrace(ctx, "existing-trace", "existing-span")
	cmd := &tracingTestCommand{Name: "test"}

	newCtx, err := aspect.BeforeCommand(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceID := trace.GetTraceID(newCtx)
	if traceID != "existing-trace" {
		t.Errorf("expected existing trace ID, got '%s'", traceID)
	}
}

func TestTracingAspect_AfterCommand_Success(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := &TracingAspect{store: store}

	ctx := context.Background()
	cmd := &tracingTestCommand{Name: "test"}

	newCtx, _ := aspect.BeforeCommand(ctx, cmd)
	err := aspect.AfterCommand(newCtx, cmd, nil, nil, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, _ := store.ListTraces(context.Background(), trace.TraceFilter{})
	if len(traceIDs) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traceIDs))
	}

	spans, _ := store.GetTrace(context.Background(), traceIDs[0])
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	if spans[0].Status != trace.SpanStatusSuccess {
		t.Errorf("expected status success, got '%s'", spans[0].Status)
	}
	if spans[0].Duration != 50*time.Millisecond {
		t.Errorf("expected duration 50ms, got %v", spans[0].Duration)
	}
}

func TestTracingAspect_AfterCommand_Error(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := &TracingAspect{store: store}

	ctx := context.Background()
	cmd := &tracingTestCommand{Name: "test"}

	newCtx, _ := aspect.BeforeCommand(ctx, cmd)
	testErr := &testTracingError{"command failed"}
	err := aspect.AfterCommand(newCtx, cmd, nil, testErr, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, _ := store.ListTraces(context.Background(), trace.TraceFilter{})
	spans, _ := store.GetTrace(context.Background(), traceIDs[0])

	if spans[0].Status != trace.SpanStatusError {
		t.Errorf("expected status error, got '%s'", spans[0].Status)
	}
	if spans[0].Error != "command failed" {
		t.Errorf("expected error 'command failed', got '%s'", spans[0].Error)
	}
}

func TestTracingAspect_BeforeQuery_CreatesSpan(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := &TracingAspect{store: store}

	ctx := context.Background()
	query := &tracingTestQuery{ID: "1"}

	newCtx, err := aspect.BeforeQuery(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceID := trace.GetTraceID(newCtx)
	if traceID == "" {
		t.Error("expected trace ID to be set")
	}

	spanID := trace.GetSpanID(newCtx)
	if spanID == "" {
		t.Error("expected span ID to be set")
	}
}

func TestTracingAspect_AfterQuery_Success(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := &TracingAspect{store: store}

	ctx := context.Background()
	query := &tracingTestQuery{ID: "1"}

	newCtx, _ := aspect.BeforeQuery(ctx, query)
	err := aspect.AfterQuery(newCtx, query, nil, nil, 30*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, _ := store.ListTraces(context.Background(), trace.TraceFilter{})
	spans, _ := store.GetTrace(context.Background(), traceIDs[0])

	if spans[0].Status != trace.SpanStatusSuccess {
		t.Errorf("expected status success, got '%s'", spans[0].Status)
	}
	if spans[0].Type != trace.SpanTypeQuery {
		t.Errorf("expected type query, got '%s'", spans[0].Type)
	}
}

func TestTracingAspect_AfterQuery_Error(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := &TracingAspect{store: store}

	ctx := context.Background()
	query := &tracingTestQuery{ID: "1"}

	newCtx, _ := aspect.BeforeQuery(ctx, query)
	testErr := &testTracingError{"query failed"}
	err := aspect.AfterQuery(newCtx, query, nil, testErr, 30*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, _ := store.ListTraces(context.Background(), trace.TraceFilter{})
	spans, _ := store.GetTrace(context.Background(), traceIDs[0])

	if spans[0].Status != trace.SpanStatusError {
		t.Errorf("expected status error, got '%s'", spans[0].Status)
	}
}

func TestTracingAspect_BeforePublish_CreatesSpan(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := &TracingAspect{store: store}

	ctx := context.Background()
	evt := &tracingTestEvent{BaseEvent: event.NewBaseEvent("agg-1", time.Now())}

	newCtx, err := aspect.BeforePublish(ctx, evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceID := trace.GetTraceID(newCtx)
	if traceID == "" {
		t.Error("expected trace ID to be set")
	}

	spanID := trace.GetSpanID(newCtx)
	if spanID == "" {
		t.Error("expected span ID to be set")
	}
}

func TestTracingAspect_AfterPublish_Success(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := &TracingAspect{store: store}

	ctx := context.Background()
	evt := &tracingTestEvent{BaseEvent: event.NewBaseEvent("agg-1", time.Now())}

	newCtx, _ := aspect.BeforePublish(ctx, evt)
	err := aspect.AfterPublish(newCtx, evt, nil, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, _ := store.ListTraces(context.Background(), trace.TraceFilter{})
	spans, _ := store.GetTrace(context.Background(), traceIDs[0])

	if spans[0].Status != trace.SpanStatusSuccess {
		t.Errorf("expected status success, got '%s'", spans[0].Status)
	}
	if spans[0].Type != trace.SpanTypeEvent {
		t.Errorf("expected type event, got '%s'", spans[0].Type)
	}
}

func TestTracingAspect_AfterPublish_Error(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := &TracingAspect{store: store}

	ctx := context.Background()
	evt := &tracingTestEvent{BaseEvent: event.NewBaseEvent("agg-1", time.Now())}

	newCtx, _ := aspect.BeforePublish(ctx, evt)
	testErr := &testTracingError{"publish failed"}
	err := aspect.AfterPublish(newCtx, evt, testErr, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, _ := store.ListTraces(context.Background(), trace.TraceFilter{})
	spans, _ := store.GetTrace(context.Background(), traceIDs[0])

	if spans[0].Status != trace.SpanStatusError {
		t.Errorf("expected status error, got '%s'", spans[0].Status)
	}
}

func TestTracingAspect_TypeName_Pointer(t *testing.T) {
	cmd := &tracingTestCommand{Name: "test"}
	name := typeName(cmd)
	if name != "tracingTestCommand" {
		t.Errorf("expected type name 'tracingTestCommand', got '%s'", name)
	}
}

func TestTracingAspect_TypeName_Value(t *testing.T) {
	cmd := tracingTestCommand{Name: "test"}
	name := typeName(cmd)
	if name != "tracingTestCommand" {
		t.Errorf("expected type name 'tracingTestCommand', got '%s'", name)
	}
}

func TestLoggingAspect_BeforeQuery(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{logger: logger}

	ctx := context.Background()
	query := &tracingTestQuery{ID: "1"}

	newCtx, err := aspect.BeforeQuery(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newCtx != ctx {
		t.Error("expected context to be unchanged")
	}

	if len(logger.debugCalls) != 1 {
		t.Errorf("expected 1 debug call, got %d", len(logger.debugCalls))
	}
}

func TestLoggingAspect_BeforeCommand(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{logger: logger}

	ctx := context.Background()
	cmd := &tracingTestCommand{Name: "test"}

	newCtx, err := aspect.BeforeCommand(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newCtx != ctx {
		t.Error("expected context to be unchanged")
	}

	if len(logger.debugCalls) != 1 {
		t.Errorf("expected 1 debug call, got %d", len(logger.debugCalls))
	}
}

func TestLoggingAspect_BeforePublish(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{logger: logger}

	ctx := context.Background()
	evt := &tracingTestEvent{BaseEvent: event.NewBaseEvent("agg-1", time.Now())}

	newCtx, err := aspect.BeforePublish(ctx, evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newCtx != ctx {
		t.Error("expected context to be unchanged")
	}

	if len(logger.debugCalls) != 1 {
		t.Errorf("expected 1 debug call, got %d", len(logger.debugCalls))
	}
}

func TestMetricsAspect_BeforeQuery_NoOp(t *testing.T) {
	recorder := newMockMetricsRecorder()
	aspect := &MetricsAspect{recorder: recorder}

	ctx := context.Background()
	query := &tracingTestQuery{ID: "1"}

	newCtx, err := aspect.BeforeQuery(ctx, query)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newCtx != ctx {
		t.Error("expected context to be unchanged")
	}

	if len(recorder.durations) != 0 {
		t.Error("expected no durations recorded for BeforeQuery")
	}
}

func TestMetricsAspect_BeforeCommand_NoOp(t *testing.T) {
	recorder := newMockMetricsRecorder()
	aspect := &MetricsAspect{recorder: recorder}

	ctx := context.Background()
	cmd := &tracingTestCommand{Name: "test"}

	newCtx, err := aspect.BeforeCommand(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newCtx != ctx {
		t.Error("expected context to be unchanged")
	}

	if len(recorder.durations) != 0 {
		t.Error("expected no durations recorded for BeforeCommand")
	}
}

func TestMetricsAspect_BeforePublish_NoOp(t *testing.T) {
	recorder := newMockMetricsRecorder()
	aspect := &MetricsAspect{recorder: recorder}

	ctx := context.Background()
	evt := &tracingTestEvent{BaseEvent: event.NewBaseEvent("agg-1", time.Now())}

	newCtx, err := aspect.BeforePublish(ctx, evt)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if newCtx != ctx {
		t.Error("expected context to be unchanged")
	}

	if len(recorder.durations) != 0 {
		t.Error("expected no durations recorded for BeforePublish")
	}
}

func TestTracingAspect_FullCommandLifecycle(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := &TracingAspect{store: store}

	ctx := context.Background()
	cmd := &tracingTestCommand{Name: "full-test"}

	newCtx, err := aspect.BeforeCommand(ctx, cmd)
	if err != nil {
		t.Fatalf("BeforeCommand error: %v", err)
	}

	err = aspect.AfterCommand(newCtx, cmd, "result", nil, 100*time.Millisecond)
	if err != nil {
		t.Fatalf("AfterCommand error: %v", err)
	}

	traceIDs, _ := store.ListTraces(context.Background(), trace.TraceFilter{})
	if len(traceIDs) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traceIDs))
	}

	spans, _ := store.GetTrace(context.Background(), traceIDs[0])
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Type != trace.SpanTypeCommand {
		t.Errorf("expected type command, got '%s'", span.Type)
	}
	if span.Name != "tracingTestCommand" {
		t.Errorf("expected name 'tracingTestCommand', got '%s'", span.Name)
	}
	if span.Status != trace.SpanStatusSuccess {
		t.Errorf("expected status success, got '%s'", span.Status)
	}
	if span.Duration != 100*time.Millisecond {
		t.Errorf("expected duration 100ms, got %v", span.Duration)
	}
}

type testTracingError struct {
	msg string
}

func (e *testTracingError) Error() string { return e.msg }

type mockFailingTraceStore struct {
	recordSpanErr error
}

func (m *mockFailingTraceStore) RecordSpan(_ context.Context, _ *trace.Span) error {
	return m.recordSpanErr
}

func (m *mockFailingTraceStore) GetTrace(_ context.Context, _ string) ([]*trace.Span, error) {
	return nil, nil
}

func (m *mockFailingTraceStore) ListTraces(_ context.Context, _ trace.TraceFilter) ([]string, error) {
	return nil, nil
}

func (m *mockFailingTraceStore) Close() error { return nil }

func TestTracingAspect_NewTracingAspect(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := NewTracingAspect(store)
	if aspect == nil {
		t.Fatal("expected non-nil TracingAspect")
	}
}

func TestTracingAspect_GetStore(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := NewTracingAspect(store)
	if aspect.GetStore() != store {
		t.Error("expected GetStore to return the same store")
	}
}

func TestTracingAspect_GetLogger_InitiallyNil(t *testing.T) {
	aspect := NewTracingAspect(trace.NewInMemoryTraceStore())
	if aspect.GetLogger() != nil {
		t.Error("expected initial logger to be nil")
	}
}

func TestTracingAspect_SetLogger(t *testing.T) {
	aspect := NewTracingAspect(trace.NewInMemoryTraceStore())
	logger := &mockLogger{}
	aspect.SetLogger(logger)
	if aspect.GetLogger() != logger {
		t.Error("expected GetLogger to return the set logger")
	}
}

func TestTracingAspect_AfterCommand_BusinessError(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := NewTracingAspect(store)

	ctx := context.Background()
	cmd := &tracingTestCommand{Name: "test"}

	newCtx, _ := aspect.BeforeCommand(ctx, cmd)
	domainErr := ddderror.NewDomainError("INVALID_STATE", "bad state")
	err := aspect.AfterCommand(newCtx, cmd, nil, domainErr, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, _ := store.ListTraces(context.Background(), trace.TraceFilter{})
	spans, _ := store.GetTrace(context.Background(), traceIDs[0])

	if spans[0].Status != trace.SpanStatusBusinessError {
		t.Errorf("expected status 'business_error', got '%s'", spans[0].Status)
	}
	if spans[0].Error != domainErr.Error() {
		t.Errorf("expected error '%s', got '%s'", domainErr.Error(), spans[0].Error)
	}
}

func TestTracingAspect_AfterQuery_BusinessError(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := NewTracingAspect(store)

	ctx := context.Background()
	query := &tracingTestQuery{ID: "1"}

	newCtx, _ := aspect.BeforeQuery(ctx, query)
	domainErr := ddderror.NewDomainError("NOT_FOUND", "not found")
	err := aspect.AfterQuery(newCtx, query, nil, domainErr, 30*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, _ := store.ListTraces(context.Background(), trace.TraceFilter{})
	spans, _ := store.GetTrace(context.Background(), traceIDs[0])

	if spans[0].Status != trace.SpanStatusBusinessError {
		t.Errorf("expected status 'business_error', got '%s'", spans[0].Status)
	}
}

func TestTracingAspect_AfterPublish_BusinessError(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	aspect := NewTracingAspect(store)

	ctx := context.Background()
	evt := &tracingTestEvent{BaseEvent: event.NewBaseEvent("agg-1", time.Now())}

	newCtx, _ := aspect.BeforePublish(ctx, evt)
	domainErr := ddderror.NewDomainError("CONCURRENCY", "conflict")
	err := aspect.AfterPublish(newCtx, evt, domainErr, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, _ := store.ListTraces(context.Background(), trace.TraceFilter{})
	spans, _ := store.GetTrace(context.Background(), traceIDs[0])

	if spans[0].Status != trace.SpanStatusBusinessError {
		t.Errorf("expected status 'business_error', got '%s'", spans[0].Status)
	}
}

func TestTracingAspect_AfterCommand_RecordSpanError_WithLogger(t *testing.T) {
	store := &mockFailingTraceStore{recordSpanErr: errors.New("store failed")}
	logger := &mockLogger{}
	aspect := NewTracingAspect(store)
	aspect.SetLogger(logger)

	ctx := context.Background()
	cmd := &tracingTestCommand{Name: "test"}

	newCtx, _ := aspect.BeforeCommand(ctx, cmd)
	err := aspect.AfterCommand(newCtx, cmd, nil, nil, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(logger.errorCalls) == 0 {
		t.Error("expected logger Error to be called when RecordSpan fails")
	}
}

func TestTracingAspect_AfterCommand_RecordSpanError_WithoutLogger(t *testing.T) {
	store := &mockFailingTraceStore{recordSpanErr: errors.New("store failed")}
	aspect := NewTracingAspect(store)

	ctx := context.Background()
	cmd := &tracingTestCommand{Name: "test"}

	newCtx, _ := aspect.BeforeCommand(ctx, cmd)
	err := aspect.AfterCommand(newCtx, cmd, nil, nil, 50*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

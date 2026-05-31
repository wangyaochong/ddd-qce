package builtin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/trace"
)

type testQuery struct{}

type testCommand struct{}

type testEvent struct {
	event.BaseEvent
}

type mockMetricsRecorder struct {
	durations map[string]time.Duration
	errors    map[string]error
}

func newMockMetricsRecorder() *mockMetricsRecorder {
	return &mockMetricsRecorder{
		durations: make(map[string]time.Duration),
		errors:    make(map[string]error),
	}
}

func (r *mockMetricsRecorder) RecordDuration(name string, duration time.Duration) {
	r.durations[name] = duration
}

func (r *mockMetricsRecorder) RecordError(name string, err error) {
	r.errors[name] = err
}

type mockLogger struct {
	infoCalls  []string
	errorCalls []string
	debugCalls []string
}

func (l *mockLogger) Info(msg string, args ...interface{}) {
	l.infoCalls = append(l.infoCalls, msg)
}

func (l *mockLogger) Error(msg string, args ...interface{}) {
	l.errorCalls = append(l.errorCalls, msg)
}

func (l *mockLogger) Debug(msg string, args ...interface{}) {
	l.debugCalls = append(l.debugCalls, msg)
}

func TestMetricsAspect_Query(t *testing.T) {
	recorder := newMockMetricsRecorder()
	aspect := &MetricsAspect{recorder: recorder}

	ctx := context.Background()
	_ = aspect.AfterQuery(ctx, &testQuery{}, nil, nil, 100*time.Millisecond)

	name := "Query/*builtin.testQuery"
	if _, ok := recorder.durations[name]; !ok {
		t.Errorf("expected duration recorded for %s", name)
	}
}

func TestMetricsAspect_QueryError(t *testing.T) {
	recorder := newMockMetricsRecorder()
	aspect := &MetricsAspect{recorder: recorder}

	ctx := context.Background()
	err := errors.New("query error")
	_ = aspect.AfterQuery(ctx, &testQuery{}, nil, err, 100*time.Millisecond)

	name := "Query/*builtin.testQuery"
	if _, ok := recorder.errors[name]; !ok {
		t.Errorf("expected error recorded for %s", name)
	}
}

func TestMetricsAspect_Command(t *testing.T) {
	recorder := newMockMetricsRecorder()
	aspect := &MetricsAspect{recorder: recorder}

	ctx := context.Background()
	_ = aspect.AfterCommand(ctx, &testCommand{}, nil, nil, 200*time.Millisecond)

	name := "Command/*builtin.testCommand"
	if d, ok := recorder.durations[name]; !ok {
		t.Errorf("expected duration recorded for %s", name)
	} else if d != 200*time.Millisecond {
		t.Errorf("expected 200ms, got %v", d)
	}
}

func TestMetricsAspect_Event(t *testing.T) {
	recorder := newMockMetricsRecorder()
	aspect := &MetricsAspect{recorder: recorder}

	ctx := context.Background()
	_ = aspect.AfterPublish(ctx, &testEvent{BaseEvent: event.NewBaseEvent("1", time.Now())}, nil, 50*time.Millisecond)

	name := "Event/*builtin.testEvent"
	if _, ok := recorder.durations[name]; !ok {
		t.Errorf("expected duration recorded for %s", name)
	}
}

func TestLoggingAspect_Query(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{logger: logger}

	ctx := context.Background()
	_ = aspect.AfterQuery(ctx, &testQuery{}, nil, nil, 100*time.Millisecond)

	if len(logger.debugCalls) == 0 {
		t.Error("expected debug log for successful query")
	}
}

func TestLoggingAspect_QueryError(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{logger: logger}

	ctx := context.Background()
	_ = aspect.AfterQuery(ctx, &testQuery{}, nil, errors.New("error"), 100*time.Millisecond)

	if len(logger.errorCalls) == 0 {
		t.Error("expected error log for failed query")
	}
}

func TestLoggingAspect_Command(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{logger: logger}

	ctx := context.Background()
	_ = aspect.AfterCommand(ctx, &testCommand{}, nil, nil, 100*time.Millisecond)

	if len(logger.debugCalls) == 0 {
		t.Error("expected debug log for successful command")
	}
}

func TestLoggingAspect_Event(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{logger: logger}

	ctx := context.Background()
	_ = aspect.AfterPublish(ctx, &testEvent{BaseEvent: event.NewBaseEvent("1", time.Now())}, nil, 100*time.Millisecond)

	if len(logger.debugCalls) == 0 {
		t.Error("expected debug log for successful event")
	}
}

func TestMetricsAspect_NameAndOrder(t *testing.T) {
	aspect := &MetricsAspect{recorder: newMockMetricsRecorder()}

	if aspect.Name() != "metrics" {
		t.Errorf("expected name 'metrics', got '%s'", aspect.Name())
	}
	if aspect.Order() != 100 {
		t.Errorf("expected order 100, got %d", aspect.Order())
	}
}

func TestLoggingAspect_NameAndOrder(t *testing.T) {
	aspect := &LoggingAspect{logger: &mockLogger{}}

	if aspect.Name() != "logging" {
		t.Errorf("expected name 'logging', got '%s'", aspect.Name())
	}
	if aspect.Order() != 50 {
		t.Errorf("expected order 50, got %d", aspect.Order())
	}
}

type mockTransactionManager struct {
	beginCalled    bool
	commitCalled   bool
	rollbackCalled bool
	beginErr       error
	commitErr      error
	rollbackErr    error
}

func (m *mockTransactionManager) Begin(ctx context.Context) (context.Context, error) {
	m.beginCalled = true
	return ctx, m.beginErr
}

func (m *mockTransactionManager) Commit(ctx context.Context) error {
	m.commitCalled = true
	return m.commitErr
}

func (m *mockTransactionManager) Rollback(ctx context.Context) error {
	m.rollbackCalled = true
	return m.rollbackErr
}

func TestTransactionAspect_NameAndOrder(t *testing.T) {
	txMgr := &mockTransactionManager{}
	aspect := &TransactionAspect{txManager: txMgr}

	if aspect.Name() != "transaction" {
		t.Errorf("expected name 'transaction', got '%s'", aspect.Name())
	}
	if aspect.Order() != 10 {
		t.Errorf("expected order 10, got %d", aspect.Order())
	}
}

func TestTransactionAspect_Command_Success(t *testing.T) {
	txMgr := &mockTransactionManager{}
	aspect := &TransactionAspect{txManager: txMgr}

	ctx := context.Background()
	err := aspect.AfterCommand(ctx, &testCommand{}, "result", nil, 100*time.Millisecond)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !txMgr.commitCalled {
		t.Error("expected Commit to be called on success")
	}
}

func TestTransactionAspect_Command_ErrorWithRollback(t *testing.T) {
	txMgr := &mockTransactionManager{}
	aspect := &TransactionAspect{txManager: txMgr}

	ctx := context.Background()
	cmdErr := errors.New("command failed")
	err := aspect.AfterCommand(ctx, &testCommand{}, nil, cmdErr, 100*time.Millisecond)

	if err != cmdErr {
		t.Errorf("expected command error, got %v", err)
	}
	if !txMgr.rollbackCalled {
		t.Error("expected Rollback to be called on error")
	}
}

func TestTransactionAspect_Command_ErrorWithRollbackFailure(t *testing.T) {
	txMgr := &mockTransactionManager{
		rollbackErr: errors.New("rollback failed"),
	}
	aspect := &TransactionAspect{txManager: txMgr}

	ctx := context.Background()
	cmdErr := errors.New("command failed")
	err := aspect.AfterCommand(ctx, &testCommand{}, nil, cmdErr, 100*time.Millisecond)

	if err == nil {
		t.Fatal("expected error when rollback fails")
	}
	if !txMgr.rollbackCalled {
		t.Error("expected Rollback to be attempted")
	}
}

func TestLoggingAspect_AfterCommand_Error(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{logger: logger}

	ctx := context.Background()
	_ = aspect.AfterCommand(ctx, &testCommand{}, nil, errors.New("error"), 100*time.Millisecond)

	if len(logger.errorCalls) == 0 {
		t.Error("expected error log for failed command")
	}
}

func TestLoggingAspect_AfterPublish_Error(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{logger: logger}

	ctx := context.Background()
	_ = aspect.AfterPublish(ctx, &testEvent{BaseEvent: event.NewBaseEvent("1", time.Now())}, errors.New("error"), 100*time.Millisecond)

	if len(logger.errorCalls) == 0 {
		t.Error("expected error log for failed event")
	}
}

func TestMetricsAspect_AfterCommand_Error(t *testing.T) {
	recorder := newMockMetricsRecorder()
	aspect := &MetricsAspect{recorder: recorder}

	ctx := context.Background()
	_ = aspect.AfterCommand(ctx, &testCommand{}, nil, errors.New("error"), 100*time.Millisecond)

	name := "Command/*builtin.testCommand"
	if _, ok := recorder.errors[name]; !ok {
		t.Errorf("expected error recorded for %s", name)
	}
}

func TestNewTransactionAspect_NilReturnsError(t *testing.T) {
	_, err := NewTransactionAspect(nil)
	if err == nil {
		t.Fatal("expected error when TxManager is nil")
	}
	if !strings.Contains(err.Error(), "TxManager") || !strings.Contains(err.Error(), "NoOpTransactionManager") {
		t.Errorf("error message should mention TxManager and NoOpTransactionManager, got: %s", err.Error())
	}
}

func TestMetricsAspect_AfterPublish_Error(t *testing.T) {
	recorder := newMockMetricsRecorder()
	aspect := &MetricsAspect{recorder: recorder}

	ctx := context.Background()
	_ = aspect.AfterPublish(ctx, &testEvent{BaseEvent: event.NewBaseEvent("1", time.Now())}, errors.New("error"), 100*time.Millisecond)

	name := "Event/*builtin.testEvent"
	if _, ok := recorder.errors[name]; !ok {
		t.Errorf("expected error recorded for %s", name)
	}
}

func TestTransactionAspect_NilTxManager_ReturnsError(t *testing.T) {
	_, err := NewTransactionAspect(nil)
	if err == nil {
		t.Fatal("expected error when TxManager is nil")
	}
	if !strings.Contains(err.Error(), "TxManager") {
		t.Errorf("error should mention TxManager, got: %v", err)
	}
}

func TestStdLogger_WithContext_TraceInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := &StdLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}

	ctx := trace.WithTrace(context.Background(), "test-trace-123", "test-span-456")
	l := logger.WithContext(ctx)
	l.Info("test message")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}
	if entry["trace_id"] != "test-trace-123" {
		t.Errorf("expected trace_id=test-trace-123, got %v", entry["trace_id"])
	}
	if entry["span_id"] != "test-span-456" {
		t.Errorf("expected span_id=test-span-456, got %v", entry["span_id"])
	}
}

func TestNewStdLogger(t *testing.T) {
	logger := NewStdLogger()
	if logger == nil {
		t.Fatal("expected non-nil StdLogger")
	}
}

func TestNewStdLoggerWithOptions(t *testing.T) {
	logger := NewStdLoggerWithOptions(&slog.HandlerOptions{Level: slog.LevelWarn})
	if logger == nil {
		t.Fatal("expected non-nil StdLogger")
	}
}

func TestStdLogger_Info(t *testing.T) {
	var buf bytes.Buffer
	logger := &StdLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
	logger.Info("info message")
	if !strings.Contains(buf.String(), "info message") {
		t.Errorf("expected log to contain 'info message', got: %s", buf.String())
	}
}

func TestStdLogger_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := &StdLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})),
	}
	logger.Error("error message")
	if !strings.Contains(buf.String(), "error message") {
		t.Errorf("expected log to contain 'error message', got: %s", buf.String())
	}
}

func TestStdLogger_Debug(t *testing.T) {
	var buf bytes.Buffer
	logger := &StdLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})),
	}
	logger.Debug("debug message")
	if !strings.Contains(buf.String(), "debug message") {
		t.Errorf("expected log to contain 'debug message', got: %s", buf.String())
	}
}

func TestStdLogger_LogCommand_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := &StdLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
	ctx := trace.WithTrace(context.Background(), "trace-1", "span-1")
	logger.LogCommand(ctx, "CreateOrder", 150*time.Millisecond, nil)

	output := buf.String()
	if !strings.Contains(output, "command executed") {
		t.Error("expected 'command executed' in log")
	}
	if !strings.Contains(output, "CreateOrder") {
		t.Error("expected command name in log")
	}
	if !strings.Contains(output, "trace-1") {
		t.Error("expected trace_id in log")
	}
}

func TestStdLogger_LogCommand_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := &StdLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})),
	}
	logger.LogCommand(context.Background(), "CreateOrder", 100*time.Millisecond, errors.New("fail"))

	output := buf.String()
	if !strings.Contains(output, "command failed") {
		t.Error("expected 'command failed' in log")
	}
	if !strings.Contains(output, "CreateOrder") {
		t.Error("expected command name in log")
	}
}

func TestStdLogger_LogQuery_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := &StdLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
	ctx := trace.WithTrace(context.Background(), "trace-2", "span-2")
	logger.LogQuery(ctx, "GetOrder", 50*time.Millisecond, nil)

	output := buf.String()
	if !strings.Contains(output, "query executed") {
		t.Error("expected 'query executed' in log")
	}
	if !strings.Contains(output, "GetOrder") {
		t.Error("expected query name in log")
	}
	if !strings.Contains(output, "trace-2") {
		t.Error("expected trace_id in log")
	}
}

func TestStdLogger_LogQuery_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := &StdLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})),
	}
	logger.LogQuery(context.Background(), "GetOrder", 50*time.Millisecond, errors.New("fail"))

	output := buf.String()
	if !strings.Contains(output, "query failed") {
		t.Error("expected 'query failed' in log")
	}
}

func TestStdLogger_LogEvent_Success(t *testing.T) {
	var buf bytes.Buffer
	logger := &StdLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}
	ctx := trace.WithTrace(context.Background(), "trace-3", "span-3")
	logger.LogEvent(ctx, "OrderCreated", nil)

	output := buf.String()
	if !strings.Contains(output, "event published") {
		t.Error("expected 'event published' in log")
	}
	if !strings.Contains(output, "OrderCreated") {
		t.Error("expected event name in log")
	}
	if !strings.Contains(output, "trace-3") {
		t.Error("expected trace_id in log")
	}
}

func TestStdLogger_LogEvent_Error(t *testing.T) {
	var buf bytes.Buffer
	logger := &StdLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError})),
	}
	logger.LogEvent(context.Background(), "OrderCreated", errors.New("fail"))

	output := buf.String()
	if !strings.Contains(output, "event publish failed") {
		t.Error("expected 'event publish failed' in log")
	}
}

func TestNewLoggingAspect(t *testing.T) {
	logger := &mockLogger{}
	aspect := NewLoggingAspect(logger)
	if aspect == nil {
		t.Fatal("expected non-nil LoggingAspect")
	}
}

func TestLoggingAspect_GetLogger(t *testing.T) {
	logger := &mockLogger{}
	aspect := NewLoggingAspect(logger)
	if aspect.GetLogger() != logger {
		t.Error("expected GetLogger to return the same logger")
	}
}

func TestNewInMemMetricsRecorder(t *testing.T) {
	recorder := NewInMemMetricsRecorder()
	if recorder == nil {
		t.Fatal("expected non-nil InMemMetricsRecorder")
	}
}

func TestInMemMetricsRecorder_GetDurations(t *testing.T) {
	recorder := NewInMemMetricsRecorder()
	recorder.RecordDuration("test", 100*time.Millisecond)
	recorder.RecordDuration("test", 200*time.Millisecond)

	durations := recorder.GetDurations("test")
	if len(durations) != 2 {
		t.Fatalf("expected 2 durations, got %d", len(durations))
	}
	if durations[0] != 100*time.Millisecond || durations[1] != 200*time.Millisecond {
		t.Errorf("unexpected durations: %v", durations)
	}
}

func TestInMemMetricsRecorder_GetErrorCount(t *testing.T) {
	recorder := NewInMemMetricsRecorder()
	recorder.RecordError("test", errors.New("err1"))
	recorder.RecordError("test", errors.New("err2"))

	if count := recorder.GetErrorCount("test"); count != 2 {
		t.Errorf("expected error count 2, got %d", count)
	}
}

func TestInMemMetricsRecorder_GetTotalCount(t *testing.T) {
	recorder := NewInMemMetricsRecorder()
	recorder.RecordDuration("test", 100*time.Millisecond)
	recorder.RecordError("test", errors.New("err"))

	if count := recorder.GetTotalCount("test"); count != 2 {
		t.Errorf("expected total count 2, got %d", count)
	}
}

func TestInMemMetricsRecorder_GetAverageDuration(t *testing.T) {
	recorder := NewInMemMetricsRecorder()
	recorder.RecordDuration("test", 100*time.Millisecond)
	recorder.RecordDuration("test", 200*time.Millisecond)

	avg := recorder.GetAverageDuration("test")
	if avg != 150*time.Millisecond {
		t.Errorf("expected average 150ms, got %v", avg)
	}
}

func TestInMemMetricsRecorder_GetAverageDuration_Empty(t *testing.T) {
	recorder := NewInMemMetricsRecorder()
	avg := recorder.GetAverageDuration("nonexistent")
	if avg != 0 {
		t.Errorf("expected 0 for nonexistent key, got %v", avg)
	}
}

func TestInMemMetricsRecorder_Reset(t *testing.T) {
	recorder := NewInMemMetricsRecorder()
	recorder.RecordDuration("test", 100*time.Millisecond)
	recorder.RecordError("test", errors.New("err"))

	recorder.Reset()

	if len(recorder.GetDurations("test")) != 0 {
		t.Error("expected durations to be cleared after reset")
	}
	if recorder.GetErrorCount("test") != 0 {
		t.Error("expected error count to be 0 after reset")
	}
	if recorder.GetTotalCount("test") != 0 {
		t.Error("expected total count to be 0 after reset")
	}
}

func TestNewMetricsAspect(t *testing.T) {
	recorder := NewInMemMetricsRecorder()
	aspect := NewMetricsAspect(recorder)
	if aspect == nil {
		t.Fatal("expected non-nil MetricsAspect")
	}
}

func TestMetricsAspect_GetRecorder(t *testing.T) {
	recorder := NewInMemMetricsRecorder()
	aspect := NewMetricsAspect(recorder)
	if aspect.GetRecorder() != recorder {
		t.Error("expected GetRecorder to return the same recorder")
	}
}

func TestNoOpTransactionManager(t *testing.T) {
	mgr := NewNoOpTransactionManager()
	ctx := context.Background()

	ctx2, err := mgr.Begin(ctx)
	if err != nil {
		t.Fatalf("unexpected Begin error: %v", err)
	}
	if ctx2 != ctx {
		t.Error("expected same context from Begin")
	}

	if err := mgr.Commit(ctx); err != nil {
		t.Fatalf("unexpected Commit error: %v", err)
	}

	if err := mgr.Rollback(ctx); err != nil {
		t.Fatalf("unexpected Rollback error: %v", err)
	}
}

func TestNewTransactionAspect_Valid(t *testing.T) {
	txMgr := &mockTransactionManager{}
	aspect, err := NewTransactionAspect(txMgr)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if aspect == nil {
		t.Fatal("expected non-nil TransactionAspect")
	}
}

func TestTransactionAspect_GetTxManager(t *testing.T) {
	txMgr := &mockTransactionManager{}
	aspect, _ := NewTransactionAspect(txMgr)
	if aspect.GetTxManager() != txMgr {
		t.Error("expected GetTxManager to return the same manager")
	}
}

func TestTransactionAspect_BeforeQuery_NoOp(t *testing.T) {
	txMgr := &mockTransactionManager{}
	aspect, _ := NewTransactionAspect(txMgr)
	ctx := context.Background()
	ctx2, err := aspect.BeforeQuery(ctx, &testQuery{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx2 != ctx {
		t.Error("expected same context returned")
	}
	if txMgr.beginCalled {
		t.Error("Begin should not be called for BeforeQuery")
	}
}

func TestTransactionAspect_AfterQuery_NoOp(t *testing.T) {
	txMgr := &mockTransactionManager{}
	aspect, _ := NewTransactionAspect(txMgr)
	ctx := context.Background()
	err := aspect.AfterQuery(ctx, &testQuery{}, nil, nil, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransactionAspect_BeforeCommand_CallsBegin(t *testing.T) {
	txMgr := &mockTransactionManager{}
	aspect, _ := NewTransactionAspect(txMgr)
	ctx := context.Background()
	_, err := aspect.BeforeCommand(ctx, &testCommand{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !txMgr.beginCalled {
		t.Error("expected Begin to be called in BeforeCommand")
	}
}

func TestTransactionAspect_BeforePublish_NoOp(t *testing.T) {
	txMgr := &mockTransactionManager{}
	aspect, _ := NewTransactionAspect(txMgr)
	ctx := context.Background()
	ctx2, err := aspect.BeforePublish(ctx, &testEvent{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx2 != ctx {
		t.Error("expected same context returned")
	}
}

func TestTransactionAspect_AfterPublish_NoOp(t *testing.T) {
	txMgr := &mockTransactionManager{}
	aspect, _ := NewTransactionAspect(txMgr)
	ctx := context.Background()
	err := aspect.AfterPublish(ctx, &testEvent{}, nil, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStdLogger_WithContext_NoTraceInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := &StdLogger{
		logger: slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo})),
	}

	ctx := context.Background()
	l := logger.WithContext(ctx)
	l.Info("test message")

	var entry map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse log output: %v", err)
	}
	if _, ok := entry["trace_id"]; ok {
		t.Error("expected no trace_id when context has no trace info")
	}
	if _, ok := entry["span_id"]; ok {
		t.Error("expected no span_id when context has no trace info")
	}
}

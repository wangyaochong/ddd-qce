package builtin

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testQuery struct{}

type testCommand struct{}

type testEvent struct{}

func (e *testEvent) AggregateID() string   { return "1" }
func (e *testEvent) EventType() string     { return "TestEvent" }
func (e *testEvent) OccurredAt() time.Time { return time.Now() }

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
	aspect := &MetricsAspect{Recorder: recorder}

	ctx := context.Background()
	_ = aspect.AfterQuery(ctx, &testQuery{}, nil, nil, 100*time.Millisecond)

	name := "Query/*builtin.testQuery"
	if _, ok := recorder.durations[name]; !ok {
		t.Errorf("expected duration recorded for %s", name)
	}
}

func TestMetricsAspect_QueryError(t *testing.T) {
	recorder := newMockMetricsRecorder()
	aspect := &MetricsAspect{Recorder: recorder}

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
	aspect := &MetricsAspect{Recorder: recorder}

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
	aspect := &MetricsAspect{Recorder: recorder}

	ctx := context.Background()
	_ = aspect.AfterPublish(ctx, &testEvent{}, nil, 50*time.Millisecond)

	name := "Event/*builtin.testEvent"
	if _, ok := recorder.durations[name]; !ok {
		t.Errorf("expected duration recorded for %s", name)
	}
}

func TestLoggingAspect_Query(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{Logger: logger}

	ctx := context.Background()
	_ = aspect.AfterQuery(ctx, &testQuery{}, nil, nil, 100*time.Millisecond)

	if len(logger.debugCalls) == 0 {
		t.Error("expected debug log for successful query")
	}
}

func TestLoggingAspect_QueryError(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{Logger: logger}

	ctx := context.Background()
	_ = aspect.AfterQuery(ctx, &testQuery{}, nil, errors.New("error"), 100*time.Millisecond)

	if len(logger.errorCalls) == 0 {
		t.Error("expected error log for failed query")
	}
}

func TestLoggingAspect_Command(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{Logger: logger}

	ctx := context.Background()
	_ = aspect.AfterCommand(ctx, &testCommand{}, nil, nil, 100*time.Millisecond)

	if len(logger.debugCalls) == 0 {
		t.Error("expected debug log for successful command")
	}
}

func TestLoggingAspect_Event(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{Logger: logger}

	ctx := context.Background()
	_ = aspect.AfterPublish(ctx, &testEvent{}, nil, 100*time.Millisecond)

	if len(logger.debugCalls) == 0 {
		t.Error("expected debug log for successful event")
	}
}

func TestMetricsAspect_NameAndOrder(t *testing.T) {
	aspect := &MetricsAspect{Recorder: newMockMetricsRecorder()}

	if aspect.Name() != "metrics" {
		t.Errorf("expected name 'metrics', got '%s'", aspect.Name())
	}
	if aspect.Order() != 100 {
		t.Errorf("expected order 100, got %d", aspect.Order())
	}
}

func TestLoggingAspect_NameAndOrder(t *testing.T) {
	aspect := &LoggingAspect{Logger: &mockLogger{}}

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
	aspect := &TransactionAspect{TxManager: txMgr}

	if aspect.Name() != "transaction" {
		t.Errorf("expected name 'transaction', got '%s'", aspect.Name())
	}
	if aspect.Order() != 10 {
		t.Errorf("expected order 10, got %d", aspect.Order())
	}
}

func TestTransactionAspect_Command_Success(t *testing.T) {
	txMgr := &mockTransactionManager{}
	aspect := &TransactionAspect{TxManager: txMgr}

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
	aspect := &TransactionAspect{TxManager: txMgr}

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
	aspect := &TransactionAspect{TxManager: txMgr}

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
	aspect := &LoggingAspect{Logger: logger}

	ctx := context.Background()
	_ = aspect.AfterCommand(ctx, &testCommand{}, nil, errors.New("error"), 100*time.Millisecond)

	if len(logger.errorCalls) == 0 {
		t.Error("expected error log for failed command")
	}
}

func TestLoggingAspect_AfterPublish_Error(t *testing.T) {
	logger := &mockLogger{}
	aspect := &LoggingAspect{Logger: logger}

	ctx := context.Background()
	_ = aspect.AfterPublish(ctx, &testEvent{}, errors.New("error"), 100*time.Millisecond)

	if len(logger.errorCalls) == 0 {
		t.Error("expected error log for failed event")
	}
}

func TestMetricsAspect_AfterCommand_Error(t *testing.T) {
	recorder := newMockMetricsRecorder()
	aspect := &MetricsAspect{Recorder: recorder}

	ctx := context.Background()
	_ = aspect.AfterCommand(ctx, &testCommand{}, nil, errors.New("error"), 100*time.Millisecond)

	name := "Command/*builtin.testCommand"
	if _, ok := recorder.errors[name]; !ok {
		t.Errorf("expected error recorded for %s", name)
	}
}

func TestMetricsAspect_AfterPublish_Error(t *testing.T) {
	recorder := newMockMetricsRecorder()
	aspect := &MetricsAspect{Recorder: recorder}

	ctx := context.Background()
	_ = aspect.AfterPublish(ctx, &testEvent{}, errors.New("error"), 100*time.Millisecond)

	name := "Event/*builtin.testEvent"
	if _, ok := recorder.errors[name]; !ok {
		t.Errorf("expected error recorded for %s", name)
	}
}

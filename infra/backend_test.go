package infra

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/trace"
)

func TestNewMemoryBackend(t *testing.T) {
	b := NewMemoryBackend()

	if b.TransactionManager == nil {
		t.Error("expected TransactionManager to be set")
	}
	if b.JobStore == nil {
		t.Error("expected JobStore to be set")
	}
	if b.TraceStore == nil {
		t.Error("expected TraceStore to be set")
	}
	if b.MessageStore == nil {
		t.Error("expected MessageStore to be set")
	}
	if b.Migrator == nil {
		t.Error("expected Migrator to be set")
	}
}

func TestMemoryBackend_TransactionManager(t *testing.T) {
	b := NewMemoryBackend()
	ctx := context.Background()

	txCtx, err := b.TransactionManager.Begin(ctx)
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}
	if txCtx == nil {
		t.Error("expected non-nil context from Begin")
	}

	if err := b.TransactionManager.Commit(txCtx); err != nil {
		t.Fatalf("Commit failed: %v", err)
	}
}

func TestMemoryBackend_TraceStore(t *testing.T) {
	b := NewMemoryBackend()
	ctx := context.Background()

	span := &trace.Span{
		ID: "s1", TraceID: "t1", Type: "command",
		Name: "Test", Status: "success",
		StartedAt: trace.Span{}.StartedAt, Duration: 0,
	}
	if err := b.TraceStore.RecordSpan(ctx, span); err != nil {
		t.Fatalf("RecordSpan failed: %v", err)
	}
}

func TestMemoryBackend_JobStore(t *testing.T) {
	b := NewMemoryBackend()
	ctx := context.Background()

	job := jobcore.NewJob("j1", "test")
	if err := b.JobStore.Create(ctx, job); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := b.JobStore.Get(ctx, "j1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID() != "j1" {
		t.Errorf("expected ID 'j1', got %s", got.ID())
	}
}

func TestMemoryBackend_MessageStore(t *testing.T) {
	b := NewMemoryBackend()
	ctx := context.Background()

	entry := &builtin.CommandEntry{
		CommandType: "Test", CreatedAt: trace.Span{}.StartedAt,
	}
	if err := b.MessageStore.RecordCommand(ctx, entry); err != nil {
		t.Fatalf("RecordCommand failed: %v", err)
	}
}

func TestMemoryBackend_Migrate(t *testing.T) {
	b := NewMemoryBackend()
	if err := b.Migrator.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
}

func TestBackend_InterfaceConformance(t *testing.T) {
	b := NewMemoryBackend()

	var _ jobcore.JobStore = b.JobStore
	var _ trace.TraceStore = b.TraceStore
	var _ builtin.MessageStore = b.MessageStore
	var _ builtin.TransactionManager = b.TransactionManager
}

func TestMemoryTransactionManager_NestedBeginCommit(t *testing.T) {
	m := NewMemoryTransactionManager()
	ctx := context.Background()

	txCtx, err := m.Begin(ctx)
	if err != nil {
		t.Fatalf("outer Begin failed: %v", err)
	}

	innerCtx, err := m.Begin(txCtx)
	if err != nil {
		t.Fatalf("inner Begin failed: %v", err)
	}
	if innerCtx != txCtx {
		t.Error("nested Begin should return same context")
	}

	if err := m.Commit(innerCtx); err != nil {
		t.Fatalf("inner Commit failed: %v", err)
	}

	if err := m.Commit(txCtx); err != nil {
		t.Fatalf("outer Commit failed: %v", err)
	}
}

func TestMemoryTransactionManager_NestedRollbackAbortsOuter(t *testing.T) {
	m := NewMemoryTransactionManager()
	ctx := context.Background()

	txCtx, err := m.Begin(ctx)
	if err != nil {
		t.Fatalf("outer Begin failed: %v", err)
	}

	innerCtx, err := m.Begin(txCtx)
	if err != nil {
		t.Fatalf("inner Begin failed: %v", err)
	}

	if err := m.Rollback(innerCtx); err != nil {
		t.Fatalf("inner Rollback failed: %v", err)
	}

	err = m.Commit(txCtx)
	if err == nil {
		t.Fatal("expected outer Commit to fail after inner Rollback")
	}
}

func TestMemoryTransactionManager_NoTransaction(t *testing.T) {
	m := NewMemoryTransactionManager()
	ctx := context.Background()

	if err := m.Commit(ctx); err == nil {
		t.Error("expected error when Commit without transaction")
	}
	if err := m.Rollback(ctx); err == nil {
		t.Error("expected error when Rollback without transaction")
	}
}

func TestMemoryTransactionManager_TripleNesting(t *testing.T) {
	m := NewMemoryTransactionManager()
	ctx := context.Background()

	l1, err := m.Begin(ctx)
	if err != nil {
		t.Fatalf("l1 Begin failed: %v", err)
	}
	l2, err := m.Begin(l1)
	if err != nil {
		t.Fatalf("l2 Begin failed: %v", err)
	}
	l3, err := m.Begin(l2)
	if err != nil {
		t.Fatalf("l3 Begin failed: %v", err)
	}

	if err := m.Commit(l3); err != nil {
		t.Fatalf("l3 Commit failed: %v", err)
	}
	if err := m.Commit(l2); err != nil {
		t.Fatalf("l2 Commit failed: %v", err)
	}
	if err := m.Commit(l1); err != nil {
		t.Fatalf("l1 Commit failed: %v", err)
	}
}

func TestBackend_Shutdown_NilCloserNilTraceStore(t *testing.T) {
	b := &Backend{}
	if err := b.Shutdown(context.Background()); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestBackend_Shutdown_CloserError(t *testing.T) {
	b := &Backend{closer: func() error { return fmt.Errorf("close failed") }}
	err := b.Shutdown(context.Background())
	if err == nil {
		t.Fatal("expected error from closer")
	}
	if err.Error() != "backend shutdown errors: [close failed]" {
		t.Errorf("unexpected error message: %v", err)
	}
}

type closeableTraceStore struct {
	trace.TraceStore
	closed bool
}

func (c *closeableTraceStore) Close() error { c.closed = true; return nil }

func TestBackend_Shutdown_TraceStoreWithClose(t *testing.T) {
	ts := &closeableTraceStore{}
	b := &Backend{TraceStore: ts}
	if err := b.Shutdown(context.Background()); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !ts.closed {
		t.Error("expected TraceStore.Close() to be called")
	}
}

func TestBackend_Shutdown_BothErrors(t *testing.T) {
	ts := &closeableTraceStore{}
	b := &Backend{
		TraceStore: ts,
		closer:     func() error { return fmt.Errorf("closer error") },
	}
	err := b.Shutdown(context.Background())
	if err == nil {
		t.Fatal("expected aggregated error")
	}
	if !ts.closed {
		t.Error("expected TraceStore.Close() to be called")
	}
	if err.Error() != "backend shutdown errors: [closer error]" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestBackend_Close_NilCloser(t *testing.T) {
	b := &Backend{}
	if err := b.Close(); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
}

func TestBackend_Close_WithCloserError(t *testing.T) {
	b := &Backend{closer: func() error { return fmt.Errorf("close err") }}
	err := b.Close()
	if err == nil {
		t.Fatal("expected error from closer")
	}
	if err.Error() != "close err" {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestWithTypeRegistry(t *testing.T) {
	reg := jobcore.NewTypeRegistry()
	b := NewBackend(WithTypeRegistry(reg))
	if b.TypeRegistry != reg {
		t.Error("expected TypeRegistry to be set")
	}
}

func TestWithCloser(t *testing.T) {
	called := false
	b := NewBackend(WithCloser(func() error { called = true; return nil }))
	if err := b.Close(); err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if !called {
		t.Error("expected closer to be called")
	}
}

func TestNewMemoryBusFactory(t *testing.T) {
	f := NewMemoryBusFactory()
	if f == nil {
		t.Fatal("expected non-nil BusFactory")
	}
	chain := aspect.NewAspectChain()
	if f.CreateCommandBus(chain) == nil {
		t.Error("expected CreateCommandBus to return non-nil CommandBus")
	}
	if f.CreateQueryBus(chain) == nil {
		t.Error("expected CreateQueryBus to return non-nil QueryBus")
	}
	if f.CreateEventBus(chain) == nil {
		t.Error("expected CreateEventBus to return non-nil EventBus")
	}
}

type testCommand struct{ command.BaseCommand }

type testCommandHandler struct{}

func (testCommandHandler) Handle(_ context.Context, _ testCommand) (string, error) {
	return "ok", nil
}

type testQuery struct{ query.BaseQuery }

type testQueryHandler struct{}

func (testQueryHandler) Handle(_ context.Context, _ testQuery) (string, error) {
	return "result", nil
}

type testEvent struct{ event.BaseEvent }

func (testEvent) AggregateID() string   { return "agg1" }
func (testEvent) OccurredAt() time.Time { return time.Now() }
func (testEvent) CorrelationID() string { return "" }
func (testEvent) CausationID() string   { return "" }

type testEventHandler struct{}

func (testEventHandler) Handle(_ context.Context, _ testEvent) error { return nil }

func TestNewMemoryBusFactory_CommandBus(t *testing.T) {
	f := NewMemoryBusFactory()
	chain := aspect.NewAspectChain()
	bus := f.CreateCommandBus(chain)
	if bus == nil {
		t.Fatal("expected non-nil CommandBus")
	}
	if err := bus.RegisterHandler(testCommandHandler{}); err != nil {
		t.Fatalf("RegisterHandler failed: %v", err)
	}
	result, err := bus.Execute(context.Background(), testCommand{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "ok" {
		t.Errorf("expected 'ok', got %v", result)
	}
}

func TestNewMemoryBusFactory_QueryBus(t *testing.T) {
	f := NewMemoryBusFactory()
	chain := aspect.NewAspectChain()
	bus := f.CreateQueryBus(chain)
	if bus == nil {
		t.Fatal("expected non-nil QueryBus")
	}
	if err := bus.RegisterHandler(testQueryHandler{}); err != nil {
		t.Fatalf("RegisterHandler failed: %v", err)
	}
	result, err := bus.Execute(context.Background(), testQuery{})
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if result != "result" {
		t.Errorf("expected 'result', got %v", result)
	}
}

func TestNewMemoryBusFactory_EventBus(t *testing.T) {
	f := NewMemoryBusFactory()
	chain := aspect.NewAspectChain()
	bus := f.CreateEventBus(chain)
	if bus == nil {
		t.Fatal("expected non-nil EventBus")
	}
	if err := bus.SubscribeHandler(testEventHandler{}); err != nil {
		t.Fatalf("SubscribeHandler failed: %v", err)
	}
	if err := bus.Publish(context.Background(), testEvent{}); err != nil {
		t.Fatalf("Publish failed: %v", err)
	}
}

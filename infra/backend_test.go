package infra

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
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

	job := &jobcore.Job{
		ID: "j1", Command: "test",
		CreatedAt: trace.Span{}.StartedAt, Timeout: 0, MaxRetries: 0,
	}
	job.RestoreJobState(jobcore.JobStatusPending, nil, "", "", time.Time{}, time.Time{})
	if err := b.JobStore.Create(ctx, job); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	got, err := b.JobStore.Get(ctx, "j1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.ID != "j1" {
		t.Errorf("expected ID 'j1', got %s", got.ID)
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

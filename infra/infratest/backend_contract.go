package infratest

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/infra"
	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/trace"
)

func TestBackendContract(t *testing.T, b *infra.Backend) {
	t.Helper()

	t.Run("InterfaceConformance", func(t *testing.T) {
		var _ builtin.TransactionManager = b.TransactionManager
		var _ jobcore.JobStore = b.JobStore
		var _ trace.TraceStore = b.TraceStore
		var _ builtin.MessageStore = b.MessageStore
		var _ infra.Migrator = b.Migrator
	})

	t.Run("TransactionManagerBeginCommit", func(t *testing.T) {
		ctx := context.Background()
		txCtx, err := b.TransactionManager.Begin(ctx)
		if err != nil {
			t.Fatalf("Begin failed: %v", err)
		}
		if txCtx == nil {
			t.Fatal("expected non-nil context from Begin")
		}
		if err := b.TransactionManager.Commit(txCtx); err != nil {
			t.Fatalf("Commit failed: %v", err)
		}
	})

	t.Run("JobStoreCreateAndGet", func(t *testing.T) {
		ctx := context.Background()
		job := jobcore.NewJob("contract-job-1", "test-cmd")
		if err := b.JobStore.Create(ctx, job); err != nil {
			t.Fatalf("Create failed: %v", err)
		}
		got, err := b.JobStore.Get(ctx, "contract-job-1")
		if err != nil {
			t.Fatalf("Get failed: %v", err)
		}
		if got.ID() != "contract-job-1" {
			t.Errorf("expected ID 'contract-job-1', got %s", got.ID())
		}
	})

	t.Run("TraceStoreRecordAndGet", func(t *testing.T) {
		ctx := context.Background()
		span := &trace.Span{
			ID:        "span-1",
			TraceID:   "trace-1",
			Type:      trace.SpanTypeCommand,
			Name:      "TestSpan",
			Status:    trace.SpanStatusSuccess,
			StartedAt: time.Now(),
			Duration:  time.Millisecond,
		}
		if err := b.TraceStore.RecordSpan(ctx, span); err != nil {
			t.Fatalf("RecordSpan failed: %v", err)
		}
		spans, err := b.TraceStore.GetTrace(ctx, "trace-1")
		if err != nil {
			t.Fatalf("GetTrace failed: %v", err)
		}
		if len(spans) == 0 {
			t.Fatal("expected at least one span")
		}
		if spans[0].ID != "span-1" {
			t.Errorf("expected span ID 'span-1', got %s", spans[0].ID)
		}
	})

	t.Run("MessageStoreRecordCommand", func(t *testing.T) {
		ctx := context.Background()
		entry := &builtin.CommandEntry{
			CommandType: "TestCmd",
			CreatedAt:   time.Now(),
		}
		if err := b.MessageStore.RecordCommand(ctx, entry); err != nil {
			t.Fatalf("RecordCommand failed: %v", err)
		}
	})

	t.Run("Migrate", func(t *testing.T) {
		if err := b.Migrator.Migrate(context.Background()); err != nil {
			t.Fatalf("Migrate failed: %v", err)
		}
	})
}

package pg

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ddd-qce/core/trace"
	pgtrace "github.com/ddd-qce/core/trace/pg"
	"github.com/ddd-qce/core/trace/tracetest"
	"github.com/ddd-qce/integrationtest/testutil"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPgTraceStore_RecordAndGetTrace(t *testing.T) {
	db := testutil.OpenTestDB(t)
	store := pgtrace.NewTraceStore(db)
	ctx := context.Background()

	spans := []*trace.Span{
		{
			ID: "span-1", TraceID: "trace-1", ParentID: "",
			Type: "command", Name: "CreateOrder", Status: "success",
			StartedAt: time.Now().Truncate(time.Microsecond), Duration: 50 * time.Millisecond,
		},
		{
			ID: "span-2", TraceID: "trace-1", ParentID: "span-1",
			Type: "event", Name: "OrderPlaced", Status: "success",
			StartedAt: time.Now().Truncate(time.Microsecond), Duration: 20 * time.Millisecond,
		},
	}

	for _, s := range spans {
		if err := store.RecordSpan(ctx, s); err != nil {
			t.Fatalf("RecordSpan failed: %v", err)
		}
	}

	got, err := store.GetTrace(ctx, "trace-1")
	if err != nil {
		t.Fatalf("GetTrace failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(got))
	}
	if got[0].Name != "CreateOrder" {
		t.Errorf("expected first span 'CreateOrder', got %s", got[0].Name)
	}
	if got[1].ParentID != "span-1" {
		t.Errorf("expected parent_id 'span-1', got %s", got[1].ParentID)
	}
}

func TestPgTraceStore_GetTraceNotFound(t *testing.T) {
	db := testutil.OpenTestDB(t)
	store := pgtrace.NewTraceStore(db)
	ctx := context.Background()

	_, err := store.GetTrace(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent trace")
	}
}

func TestPgTraceStore_RecordSpanWithError(t *testing.T) {
	db := testutil.OpenTestDB(t)
	store := pgtrace.NewTraceStore(db)
	ctx := context.Background()

	span := &trace.Span{
		ID: "span-err", TraceID: "trace-err", Type: "command",
		Name: "FailCommand", Status: "error", Error: "something went wrong",
		StartedAt: time.Now().Truncate(time.Microsecond), Duration: 10 * time.Millisecond,
	}

	if err := store.RecordSpan(ctx, span); err != nil {
		t.Fatalf("RecordSpan failed: %v", err)
	}

	got, _ := store.GetTrace(ctx, "trace-err")
	if len(got) != 1 {
		t.Fatalf("expected 1 span, got %d", len(got))
	}
	if got[0].Error != "something went wrong" {
		t.Errorf("expected error 'something went wrong', got %s", got[0].Error)
	}
}

func TestPgTraceStore_ListTraces(t *testing.T) {
	db := testutil.OpenTestDB(t)
	store := pgtrace.NewTraceStore(db)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		span := &trace.Span{
			ID: fmt.Sprintf("span-list-%d", i), TraceID: fmt.Sprintf("trace-list-%d", i),
			Type: "command", Name: "TestCommand", Status: "success",
			StartedAt: time.Now().Truncate(time.Microsecond), Duration: time.Millisecond,
		}
		store.RecordSpan(ctx, span)
	}

	ids, err := store.ListTraces(ctx, trace.TraceFilter{Type: "command"})
	if err != nil {
		t.Fatalf("ListTraces failed: %v", err)
	}
	if len(ids) != 3 {
		t.Errorf("expected 3 traces, got %d", len(ids))
	}
}

func TestPgTraceStore_Contract(t *testing.T) {
	db := testutil.OpenTestDB(t)
	tracetest.TestTraceStoreContract(t, func() trace.TraceStore {
		testutil.CleanDB(t, db)
		return pgtrace.NewTraceStore(db)
	})
}

func TestPgTraceStore_ListTracesByName(t *testing.T) {
	db := testutil.OpenTestDB(t)
	store := pgtrace.NewTraceStore(db)
	ctx := context.Background()

	span1 := &trace.Span{
		ID: "s1", TraceID: "t1", Type: "command",
		Name: "CreateOrder", Status: "success",
		StartedAt: time.Now(), Duration: time.Millisecond,
	}
	span2 := &trace.Span{
		ID: "s2", TraceID: "t2", Type: "command",
		Name: "DeleteOrder", Status: "success",
		StartedAt: time.Now(), Duration: time.Millisecond,
	}
	store.RecordSpan(ctx, span1)
	store.RecordSpan(ctx, span2)

	ids, err := store.ListTraces(ctx, trace.TraceFilter{NameContains: "Create"})
	if err != nil {
		t.Fatalf("ListTraces failed: %v", err)
	}
	if len(ids) != 1 {
		t.Errorf("expected 1 trace matching 'Create', got %d", len(ids))
	}
}

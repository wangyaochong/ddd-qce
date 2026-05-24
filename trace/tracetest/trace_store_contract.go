package tracetest

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/trace"
)

func TestTraceStoreContract(t *testing.T, newStore func() trace.TraceStore) {
	t.Helper()
	ctx := context.Background()

	t.Run("RecordAndGetTrace", func(t *testing.T) {
		store := newStore()
		span := &trace.Span{
			ID:        "contract-span-1",
			TraceID:   "contract-trace-1",
			Type:      trace.SpanTypeCommand,
			Name:      "ContractCommand",
			Status:    trace.SpanStatusSuccess,
			StartedAt: time.Now().Truncate(time.Microsecond),
			Duration:  10 * time.Millisecond,
		}
		if err := store.RecordSpan(ctx, span); err != nil {
			t.Fatalf("RecordSpan failed: %v", err)
		}

		spans, err := store.GetTrace(ctx, "contract-trace-1")
		if err != nil {
			t.Fatalf("GetTrace failed: %v", err)
		}
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		if spans[0].Name != "ContractCommand" {
			t.Errorf("expected Name 'ContractCommand', got %q", spans[0].Name)
		}
		if spans[0].TraceID != "contract-trace-1" {
			t.Errorf("expected TraceID 'contract-trace-1', got %q", spans[0].TraceID)
		}
	})

	t.Run("GetTraceNotFound", func(t *testing.T) {
		store := newStore()
		_, err := store.GetTrace(ctx, "contract-nonexistent")
		if err == nil {
			t.Fatal("expected error for nonexistent traceID, got nil")
		}
	})

	t.Run("RecordSpanWithError", func(t *testing.T) {
		store := newStore()
		span := &trace.Span{
			ID:        "contract-span-err",
			TraceID:   "contract-trace-err",
			Type:      trace.SpanTypeCommand,
			Name:      "FailCommand",
			Status:    trace.SpanStatusError,
			Error:     "something went wrong",
			StartedAt: time.Now().Truncate(time.Microsecond),
			Duration:  5 * time.Millisecond,
		}
		if err := store.RecordSpan(ctx, span); err != nil {
			t.Fatalf("RecordSpan failed: %v", err)
		}

		spans, err := store.GetTrace(ctx, "contract-trace-err")
		if err != nil {
			t.Fatalf("GetTrace failed: %v", err)
		}
		if len(spans) != 1 {
			t.Fatalf("expected 1 span, got %d", len(spans))
		}
		if spans[0].Status != trace.SpanStatusError {
			t.Errorf("expected Status %q, got %q", trace.SpanStatusError, spans[0].Status)
		}
		if spans[0].Error != "something went wrong" {
			t.Errorf("expected Error 'something went wrong', got %q", spans[0].Error)
		}
	})

	t.Run("ListTracesByType", func(t *testing.T) {
		store := newStore()
		spans := []*trace.Span{
			{ID: "contract-type-1", TraceID: "contract-type-t1", Type: trace.SpanTypeCommand, Name: "Cmd1", Status: trace.SpanStatusSuccess, StartedAt: time.Now().Truncate(time.Microsecond)},
			{ID: "contract-type-2", TraceID: "contract-type-t2", Type: trace.SpanTypeQuery, Name: "Qry1", Status: trace.SpanStatusSuccess, StartedAt: time.Now().Truncate(time.Microsecond)},
			{ID: "contract-type-3", TraceID: "contract-type-t3", Type: trace.SpanTypeCommand, Name: "Cmd2", Status: trace.SpanStatusSuccess, StartedAt: time.Now().Truncate(time.Microsecond)},
		}
		for _, s := range spans {
			if err := store.RecordSpan(ctx, s); err != nil {
				t.Fatalf("RecordSpan failed: %v", err)
			}
		}

		ids, err := store.ListTraces(ctx, trace.TraceFilter{Type: trace.SpanTypeCommand})
		if err != nil {
			t.Fatalf("ListTraces failed: %v", err)
		}
		if len(ids) != 2 {
			t.Errorf("expected 2 traces with type=command, got %d", len(ids))
		}
	})

	t.Run("ListTracesByNameContains", func(t *testing.T) {
		store := newStore()
		spans := []*trace.Span{
			{ID: "contract-name-1", TraceID: "contract-name-t1", Type: trace.SpanTypeCommand, Name: "CreateOrder", Status: trace.SpanStatusSuccess, StartedAt: time.Now().Truncate(time.Microsecond)},
			{ID: "contract-name-2", TraceID: "contract-name-t2", Type: trace.SpanTypeCommand, Name: "DeleteUser", Status: trace.SpanStatusSuccess, StartedAt: time.Now().Truncate(time.Microsecond)},
		}
		for _, s := range spans {
			if err := store.RecordSpan(ctx, s); err != nil {
				t.Fatalf("RecordSpan failed: %v", err)
			}
		}

		ids, err := store.ListTraces(ctx, trace.TraceFilter{NameContains: "Order"})
		if err != nil {
			t.Fatalf("ListTraces failed: %v", err)
		}
		if len(ids) != 1 {
			t.Errorf("expected 1 trace with NameContains 'Order', got %d", len(ids))
		}
	})

	t.Run("ListTracesByStatus", func(t *testing.T) {
		store := newStore()
		spans := []*trace.Span{
			{ID: "contract-status-1", TraceID: "contract-status-t1", Type: trace.SpanTypeCommand, Name: "OK", Status: trace.SpanStatusSuccess, StartedAt: time.Now().Truncate(time.Microsecond)},
			{ID: "contract-status-2", TraceID: "contract-status-t2", Type: trace.SpanTypeCommand, Name: "Bad", Status: trace.SpanStatusError, Error: "fail", StartedAt: time.Now().Truncate(time.Microsecond)},
		}
		for _, s := range spans {
			if err := store.RecordSpan(ctx, s); err != nil {
				t.Fatalf("RecordSpan failed: %v", err)
			}
		}

		ids, err := store.ListTraces(ctx, trace.TraceFilter{Status: trace.SpanStatusError})
		if err != nil {
			t.Fatalf("ListTraces failed: %v", err)
		}
		if len(ids) != 1 {
			t.Errorf("expected 1 trace with status=error, got %d", len(ids))
		}
	})

	t.Run("ListTracesByTimeRange", func(t *testing.T) {
		store := newStore()
		now := time.Now().Truncate(time.Microsecond)
		past := now.Add(-2 * time.Hour)
		recent := now.Add(-5 * time.Minute)

		spans := []*trace.Span{
			{ID: "contract-time-1", TraceID: "contract-time-t1", Type: trace.SpanTypeCommand, Name: "Old", Status: trace.SpanStatusSuccess, StartedAt: past},
			{ID: "contract-time-2", TraceID: "contract-time-t2", Type: trace.SpanTypeCommand, Name: "Recent", Status: trace.SpanStatusSuccess, StartedAt: recent},
		}
		for _, s := range spans {
			if err := store.RecordSpan(ctx, s); err != nil {
				t.Fatalf("RecordSpan failed: %v", err)
			}
		}

		ids, err := store.ListTraces(ctx, trace.TraceFilter{StartTime: now.Add(-1 * time.Hour)})
		if err != nil {
			t.Fatalf("ListTraces failed: %v", err)
		}
		if len(ids) != 1 {
			t.Errorf("expected 1 trace within time range, got %d", len(ids))
		}
	})

	t.Run("ListTracesEmptyResult", func(t *testing.T) {
		store := newStore()
		ids, err := store.ListTraces(ctx, trace.TraceFilter{Type: "nonexistent-type"})
		if err != nil {
			t.Fatalf("ListTraces failed: %v", err)
		}
		if len(ids) != 0 {
			t.Errorf("expected 0 traces for nonexistent type, got %d", len(ids))
		}
	})

	t.Run("MultipleSpansPerTrace", func(t *testing.T) {
		store := newStore()
		traceID := "contract-multi-trace"
		spans := []*trace.Span{
			{ID: "contract-multi-1", TraceID: traceID, Type: trace.SpanTypeCommand, Name: "Cmd", Status: trace.SpanStatusSuccess, StartedAt: time.Now().Truncate(time.Microsecond)},
			{ID: "contract-multi-2", TraceID: traceID, Type: trace.SpanTypeEvent, Name: "Evt", Status: trace.SpanStatusSuccess, StartedAt: time.Now().Truncate(time.Microsecond)},
		}
		for _, s := range spans {
			if err := store.RecordSpan(ctx, s); err != nil {
				t.Fatalf("RecordSpan failed: %v", err)
			}
		}

		got, err := store.GetTrace(ctx, traceID)
		if err != nil {
			t.Fatalf("GetTrace failed: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("expected 2 spans for trace %q, got %d", traceID, len(got))
		}
	})
}

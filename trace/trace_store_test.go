package trace

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestNewTraceID(t *testing.T) {
	id1 := NewTraceID()
	id2 := NewTraceID()

	if id1 == "" {
		t.Error("expected non-empty trace ID")
	}
	if id1 == id2 {
		t.Error("expected unique trace IDs")
	}
}

func TestNewSpanID(t *testing.T) {
	id1 := NewSpanID()
	id2 := NewSpanID()

	if id1 == "" {
		t.Error("expected non-empty span ID")
	}
	if id1 == id2 {
		t.Error("expected unique span IDs")
	}
}

func TestWithTrace_GetTraceID(t *testing.T) {
	ctx := context.Background()
	ctx = WithTrace(ctx, "trace-1", "span-1")

	traceID := GetTraceID(ctx)
	if traceID != "trace-1" {
		t.Errorf("expected trace ID 'trace-1', got '%s'", traceID)
	}
}

func TestWithTrace_GetSpanID(t *testing.T) {
	ctx := context.Background()
	ctx = WithTrace(ctx, "trace-1", "span-1")

	spanID := GetSpanID(ctx)
	if spanID != "span-1" {
		t.Errorf("expected span ID 'span-1', got '%s'", spanID)
	}
}

func TestGetTraceID_EmptyContext(t *testing.T) {
	ctx := context.Background()
	traceID := GetTraceID(ctx)
	if traceID != "" {
		t.Errorf("expected empty trace ID, got '%s'", traceID)
	}
}

func TestGetSpanID_EmptyContext(t *testing.T) {
	ctx := context.Background()
	spanID := GetSpanID(ctx)
	if spanID != "" {
		t.Errorf("expected empty span ID, got '%s'", spanID)
	}
}

func TestWithParentSpan_GetParentSpanID(t *testing.T) {
	ctx := context.Background()
	ctx = WithParentSpan(ctx, "parent-span-1")

	parentID := GetParentSpanID(ctx)
	if parentID != "parent-span-1" {
		t.Errorf("expected parent span ID 'parent-span-1', got '%s'", parentID)
	}
}

func TestGetParentSpanID_EmptyContext(t *testing.T) {
	ctx := context.Background()
	parentID := GetParentSpanID(ctx)
	if parentID != "" {
		t.Errorf("expected empty parent span ID, got '%s'", parentID)
	}
}

func TestTraceContext_Propagation(t *testing.T) {
	ctx := context.Background()
	ctx = WithTrace(ctx, "trace-1", "span-1")
	ctx = WithParentSpan(ctx, "parent-1")

	if GetTraceID(ctx) != "trace-1" {
		t.Error("trace ID not propagated")
	}
	if GetSpanID(ctx) != "span-1" {
		t.Error("span ID not propagated")
	}
	if GetParentSpanID(ctx) != "parent-1" {
		t.Error("parent span ID not propagated")
	}
}

func TestInMemoryTraceStore_New(t *testing.T) {
	store := NewInMemoryTraceStore()
	if store == nil {
		t.Fatal("expected non-nil store")
	}
	if store.spans == nil {
		t.Error("expected initialized spans slice")
	}
	if store.traceIndex == nil {
		t.Error("expected initialized trace index")
	}
}

func TestInMemoryTraceStore_RecordSpan(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()
	span := &Span{
		ID:        "span-1",
		TraceID:   "trace-1",
		Type:      SpanTypeCommand,
		Name:      "TestCommand",
		StartedAt: time.Now(),
	}

	err := store.RecordSpan(ctx, span)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.spans) != 1 {
		t.Errorf("expected 1 span, got %d", len(store.spans))
	}
	if len(store.traceIndex["trace-1"]) != 1 {
		t.Errorf("expected 1 index entry for trace-1, got %d", len(store.traceIndex["trace-1"]))
	}
}

func TestInMemoryTraceStore_GetTrace_Found(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	span1 := &Span{ID: "span-1", TraceID: "trace-1", Type: SpanTypeCommand, Name: "Cmd1", StartedAt: time.Now()}
	span2 := &Span{ID: "span-2", TraceID: "trace-1", Type: SpanTypeEvent, Name: "Evt1", StartedAt: time.Now()}

	store.RecordSpan(ctx, span1)
	store.RecordSpan(ctx, span2)

	spans, err := store.GetTrace(ctx, "trace-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(spans) != 2 {
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}
}

func TestInMemoryTraceStore_GetTrace_NotFound(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	_, err := store.GetTrace(ctx, "non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent trace")
	}
}

func TestInMemoryTraceStore_RecordSpan_Concurrent(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			span := &Span{
				ID:        "span",
				TraceID:   "trace-1",
				Type:      SpanTypeCommand,
				Name:      "Cmd",
				StartedAt: time.Now(),
			}
			store.RecordSpan(ctx, span)
		}(i)
	}

	wg.Wait()

	if len(store.spans) != 100 {
		t.Errorf("expected 100 spans, got %d", len(store.spans))
	}
}

func TestInMemoryTraceStore_ListTraces_NoFilter(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	store.RecordSpan(ctx, &Span{ID: "s1", TraceID: "t1", Type: SpanTypeCommand, StartedAt: time.Now()})
	store.RecordSpan(ctx, &Span{ID: "s2", TraceID: "t2", Type: SpanTypeQuery, StartedAt: time.Now()})
	store.RecordSpan(ctx, &Span{ID: "s3", TraceID: "t1", Type: SpanTypeEvent, StartedAt: time.Now()})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 2 {
		t.Errorf("expected 2 traces, got %d", len(traceIDs))
	}
}

func TestInMemoryTraceStore_ListTraces_ByTraceID(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	store.RecordSpan(ctx, &Span{ID: "s1", TraceID: "t1", Type: SpanTypeCommand, StartedAt: time.Now()})
	store.RecordSpan(ctx, &Span{ID: "s2", TraceID: "t2", Type: SpanTypeQuery, StartedAt: time.Now()})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{TraceID: "t1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 1 {
		t.Errorf("expected 1 trace, got %d", len(traceIDs))
	}
	if traceIDs[0] != "t1" {
		t.Errorf("expected trace ID 't1', got '%s'", traceIDs[0])
	}
}

func TestInMemoryTraceStore_ListTraces_ByType(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	store.RecordSpan(ctx, &Span{ID: "s1", TraceID: "t1", Type: SpanTypeCommand, StartedAt: time.Now()})
	store.RecordSpan(ctx, &Span{ID: "s2", TraceID: "t2", Type: SpanTypeQuery, StartedAt: time.Now()})
	store.RecordSpan(ctx, &Span{ID: "s3", TraceID: "t3", Type: SpanTypeCommand, StartedAt: time.Now()})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{Type: SpanTypeCommand})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 2 {
		t.Errorf("expected 2 traces with command type, got %d", len(traceIDs))
	}
}

func TestInMemoryTraceStore_ListTraces_ByStatus(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	store.RecordSpan(ctx, &Span{ID: "s1", TraceID: "t1", Type: SpanTypeCommand, Status: SpanStatusSuccess, StartedAt: time.Now()})
	store.RecordSpan(ctx, &Span{ID: "s2", TraceID: "t2", Type: SpanTypeCommand, Status: SpanStatusError, StartedAt: time.Now()})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{Status: SpanStatusError})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 1 {
		t.Errorf("expected 1 trace with error status, got %d", len(traceIDs))
	}
}

func TestInMemoryTraceStore_ListTraces_ByTimeRange(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	now := time.Now()
	past := now.Add(-1 * time.Hour)
	future := now.Add(1 * time.Hour)

	store.RecordSpan(ctx, &Span{ID: "s1", TraceID: "t1", Type: SpanTypeCommand, StartedAt: past})
	store.RecordSpan(ctx, &Span{ID: "s2", TraceID: "t2", Type: SpanTypeCommand, StartedAt: now})
	store.RecordSpan(ctx, &Span{ID: "s3", TraceID: "t3", Type: SpanTypeCommand, StartedAt: future})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{StartTime: now.Add(-30 * time.Minute)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) < 1 {
		t.Error("expected at least 1 trace after start time")
	}
}

func TestInMemoryTraceStore_ListTraces_ByNameContains(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	store.RecordSpan(ctx, &Span{ID: "s1", TraceID: "t1", Type: SpanTypeCommand, Name: "CreateUserCommand", StartedAt: time.Now()})
	store.RecordSpan(ctx, &Span{ID: "s2", TraceID: "t2", Type: SpanTypeCommand, Name: "DeleteOrderCommand", StartedAt: time.Now()})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{NameContains: "User"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 1 {
		t.Errorf("expected 1 trace with 'User' in name, got %d", len(traceIDs))
	}
}

func TestInMemoryTraceStore_ListTraces_CombinedFilters(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	store.RecordSpan(ctx, &Span{ID: "s1", TraceID: "t1", Type: SpanTypeCommand, Status: SpanStatusSuccess, Name: "CreateUser", StartedAt: time.Now()})
	store.RecordSpan(ctx, &Span{ID: "s2", TraceID: "t2", Type: SpanTypeCommand, Status: SpanStatusError, Name: "DeleteUser", StartedAt: time.Now()})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{Type: SpanTypeCommand, Status: SpanStatusError})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 1 {
		t.Errorf("expected 1 trace matching combined filters, got %d", len(traceIDs))
	}
}

func TestSpan_Constants(t *testing.T) {
	if SpanTypeCommand != "command" {
		t.Errorf("expected SpanTypeCommand 'command', got '%s'", SpanTypeCommand)
	}
	if SpanTypeQuery != "query" {
		t.Errorf("expected SpanTypeQuery 'query', got '%s'", SpanTypeQuery)
	}
	if SpanTypeEvent != "event" {
		t.Errorf("expected SpanTypeEvent 'event', got '%s'", SpanTypeEvent)
	}
	if SpanStatusSuccess != "success" {
		t.Errorf("expected SpanStatusSuccess 'success', got '%s'", SpanStatusSuccess)
	}
	if SpanStatusError != "error" {
		t.Errorf("expected SpanStatusError 'error', got '%s'", SpanStatusError)
	}
}

func TestInMemoryTraceStore_ListTraces_FilterNoMatch(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	store.RecordSpan(ctx, &Span{ID: "s1", TraceID: "t1", Type: SpanTypeCommand, Status: SpanStatusSuccess, Name: "CreateUser", StartedAt: time.Now()})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{Type: SpanTypeQuery})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 0 {
		t.Errorf("expected 0 traces, got %d", len(traceIDs))
	}
}

func TestInMemoryTraceStore_ListTraces_StatusFilterNoMatch(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	store.RecordSpan(ctx, &Span{ID: "s1", TraceID: "t1", Type: SpanTypeCommand, Status: SpanStatusSuccess, StartedAt: time.Now()})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{Status: SpanStatusError})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 0 {
		t.Errorf("expected 0 traces, got %d", len(traceIDs))
	}
}

func TestInMemoryTraceStore_ListTraces_TimeRangeNoMatch(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	past := time.Now().Add(-2 * time.Hour)
	store.RecordSpan(ctx, &Span{ID: "s1", TraceID: "t1", Type: SpanTypeCommand, StartedAt: past})

	future := time.Now().Add(1 * time.Hour)
	traceIDs, err := store.ListTraces(ctx, TraceFilter{StartTime: future})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 0 {
		t.Errorf("expected 0 traces, got %d", len(traceIDs))
	}
}

func TestInMemoryTraceStore_ListTraces_EndTimeFilter(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	now := time.Now()
	store.RecordSpan(ctx, &Span{ID: "s1", TraceID: "t1", Type: SpanTypeCommand, StartedAt: now})
	store.RecordSpan(ctx, &Span{ID: "s2", TraceID: "t2", Type: SpanTypeCommand, StartedAt: now.Add(-1 * time.Hour)})

	past := now.Add(-30 * time.Minute)
	traceIDs, err := store.ListTraces(ctx, TraceFilter{EndTime: past})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) < 1 {
		t.Error("expected at least 1 trace before end time")
	}
}

func TestInMemoryTraceStore_ListTraces_NameContainsNoMatch(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	store.RecordSpan(ctx, &Span{ID: "s1", TraceID: "t1", Type: SpanTypeCommand, Name: "CreateUser", StartedAt: time.Now()})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{NameContains: "Order"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 0 {
		t.Errorf("expected 0 traces, got %d", len(traceIDs))
	}
}

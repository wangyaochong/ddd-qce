package trace

import (
	"context"
	"testing"
	"time"
)

func TestMatchesFilter_Type(t *testing.T) {
	spans := []*Span{
		{Type: SpanTypeCommand},
		{Type: SpanTypeEvent},
	}

	filter := TraceFilter{Type: SpanTypeCommand}
	if !matchesFilter(spans, filter) {
		t.Error("expected filter by type to match")
	}

	filter = TraceFilter{Type: "unknown"}
	if matchesFilter(spans, filter) {
		t.Error("expected filter by unknown type to not match")
	}
}

func TestMatchesFilter_Status(t *testing.T) {
	spans := []*Span{
		{Status: SpanStatusSuccess},
		{Status: SpanStatusError},
	}

	filter := TraceFilter{Status: SpanStatusError}
	if !matchesFilter(spans, filter) {
		t.Error("expected filter by status to match")
	}

	filter = TraceFilter{Status: "unknown"}
	if matchesFilter(spans, filter) {
		t.Error("expected filter by unknown status to not match")
	}
}

func TestMatchesFilter_StartTime(t *testing.T) {
	now := time.Now()
	spans := []*Span{
		{StartedAt: now.Add(-time.Hour)},
		{StartedAt: now},
	}

	filter := TraceFilter{StartTime: now.Add(-30 * time.Minute)}
	if !matchesFilter(spans, filter) {
		t.Error("expected filter by start time to match")
	}

	filter = TraceFilter{StartTime: now.Add(time.Hour)}
	if matchesFilter(spans, filter) {
		t.Error("expected filter by future start time to not match")
	}
}

func TestMatchesFilter_EndTime(t *testing.T) {
	now := time.Now()
	spans := []*Span{
		{StartedAt: now.Add(-time.Hour)},
		{StartedAt: now},
	}

	filter := TraceFilter{EndTime: now.Add(30 * time.Minute)}
	if !matchesFilter(spans, filter) {
		t.Error("expected filter by end time to match")
	}

	filter = TraceFilter{EndTime: now.Add(-2 * time.Hour)}
	if matchesFilter(spans, filter) {
		t.Error("expected filter by past end time to not match")
	}
}

func TestMatchesFilter_NameContains(t *testing.T) {
	spans := []*Span{
		{Name: "CreateOrderCommand"},
		{Name: "UpdateUserCommand"},
	}

	filter := TraceFilter{NameContains: "Order"}
	if !matchesFilter(spans, filter) {
		t.Error("expected filter by name contains to match")
	}

	filter = TraceFilter{NameContains: "NotFound"}
	if matchesFilter(spans, filter) {
		t.Error("expected filter by name not contains to not match")
	}
}

func TestMatchesFilter_Empty(t *testing.T) {
	spans := []*Span{
		{Name: "Test", Type: SpanTypeCommand, Status: SpanStatusSuccess},
	}

	filter := TraceFilter{}
	if !matchesFilter(spans, filter) {
		t.Error("expected empty filter to match")
	}
}

func TestListTraces_WithTypeFilter(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	store.RecordSpan(ctx, &Span{
		TraceID:   "trace-1",
		ID:        "span-1",
		Type:      SpanTypeCommand,
		Name:      "CreateOrder",
		Status:    SpanStatusSuccess,
		StartedAt: time.Now(),
	})

	store.RecordSpan(ctx, &Span{
		TraceID:   "trace-2",
		ID:        "span-2",
		Type:      SpanTypeEvent,
		Name:      "OrderCreated",
		Status:    SpanStatusSuccess,
		StartedAt: time.Now(),
	})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{Type: SpanTypeCommand})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traceIDs))
	}
	if traceIDs[0] != "trace-1" {
		t.Errorf("expected trace-1, got %s", traceIDs[0])
	}
}

func TestListTraces_WithNameContains(t *testing.T) {
	store := NewInMemoryTraceStore()
	ctx := context.Background()

	store.RecordSpan(ctx, &Span{
		TraceID:   "trace-1",
		ID:        "span-1",
		Type:      SpanTypeCommand,
		Name:      "CreateOrder",
		Status:    SpanStatusSuccess,
		StartedAt: time.Now(),
	})

	store.RecordSpan(ctx, &Span{
		TraceID:   "trace-2",
		ID:        "span-2",
		Type:      SpanTypeCommand,
		Name:      "UpdateUser",
		Status:    SpanStatusSuccess,
		StartedAt: time.Now(),
	})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{NameContains: "Order"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traceIDs))
	}
}

func TestMatchesFilter_TraceID(t *testing.T) {
	spans := []*Span{
		{TraceID: "trace-1"},
		{TraceID: "trace-2"},
	}

	filter := TraceFilter{TraceID: "trace-1"}
	if !matchesFilter(spans, filter) {
		t.Error("expected filter by trace ID to match")
	}

	filter = TraceFilter{TraceID: "trace-999"}
	if matchesFilter(spans, filter) {
		t.Error("expected filter by non-existent trace ID to not match")
	}
}

func TestInMemoryTraceStore_WithBackgroundCleanup(t *testing.T) {
	store := NewInMemoryTraceStore(
		WithTTL(100*time.Millisecond),
		WithBackgroundCleanup(50*time.Millisecond),
	)
	defer store.Close()

	ctx := context.Background()
	store.RecordSpan(ctx, &Span{
		TraceID:   "trace-old",
		ID:        "span-old",
		Name:      "OldSpan",
		StartedAt: time.Now().Add(-time.Hour),
	})

	time.Sleep(200 * time.Millisecond)

	traceIDs, err := store.ListTraces(ctx, TraceFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 0 {
		t.Errorf("expected 0 traces after background cleanup, got %d", len(traceIDs))
	}
}

func TestInMemoryTraceStore_WithTTL(t *testing.T) {
	store := NewInMemoryTraceStore(WithTTL(1 * time.Hour))
	ctx := context.Background()

	store.RecordSpan(ctx, &Span{
		TraceID:   "trace-old",
		ID:        "span-old",
		Name:      "OldSpan",
		StartedAt: time.Now().Add(-2 * time.Hour),
	})

	store.RecordSpan(ctx, &Span{
		TraceID:   "trace-new",
		ID:        "span-new",
		Name:      "NewSpan",
		StartedAt: time.Now(),
	})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 1 || traceIDs[0] != "trace-new" {
		t.Errorf("expected only trace-new after TTL eviction, got %v", traceIDs)
	}
}

func TestInMemoryTraceStore_WithMaxSpans(t *testing.T) {
	store := NewInMemoryTraceStore(WithMaxSpans(2))
	ctx := context.Background()

	store.RecordSpan(ctx, &Span{
		TraceID:   "trace-1",
		ID:        "span-1",
		Name:      "Span1",
		StartedAt: time.Now(),
	})
	store.RecordSpan(ctx, &Span{
		TraceID:   "trace-2",
		ID:        "span-2",
		Name:      "Span2",
		StartedAt: time.Now(),
	})
	store.RecordSpan(ctx, &Span{
		TraceID:   "trace-3",
		ID:        "span-3",
		Name:      "Span3",
		StartedAt: time.Now(),
	})

	traceIDs, err := store.ListTraces(ctx, TraceFilter{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(traceIDs) != 2 {
		t.Errorf("expected 2 traces after max spans eviction, got %d", len(traceIDs))
	}
}

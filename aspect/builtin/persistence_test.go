package builtin

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/trace"
)

type persistenceTestCommand struct {
	Name string
}

type persistenceTestCommandResult struct {
	ID string
}

type persistenceTestQuery struct {
	ID string
}

type persistenceTestQueryResult struct {
	Value string
}

type persistenceTestEvent struct {
	event.BaseEvent
}

func TestPersistenceAspect_AfterCommand(t *testing.T) {
	store := NewInMemoryMessageStore()
	aspect := NewPersistenceAspect(store)

	ctx := context.Background()
	cmd := &persistenceTestCommand{Name: "create"}
	result := &persistenceTestCommandResult{ID: "abc"}
	duration := 50 * time.Millisecond

	err := aspect.AfterCommand(ctx, cmd, result, nil, duration)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.Commands) != 1 {
		t.Fatalf("expected 1 command entry, got %d", len(store.Commands))
	}

	entry := store.Commands[0]
	if entry.CommandType != "persistenceTestCommand" {
		t.Errorf("expected CommandType 'persistenceTestCommand', got '%s'", entry.CommandType)
	}
	if entry.ResultType != "persistenceTestCommandResult" {
		t.Errorf("expected ResultType 'persistenceTestCommandResult', got '%s'", entry.ResultType)
	}
	if entry.Duration != duration {
		t.Errorf("expected Duration %v, got %v", duration, entry.Duration)
	}
	if entry.Error != "" {
		t.Errorf("expected empty Error, got '%s'", entry.Error)
	}
	if entry.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestPersistenceAspect_AfterCommandWithError(t *testing.T) {
	store := NewInMemoryMessageStore()
	aspect := NewPersistenceAspect(store)

	ctx := context.Background()
	cmd := &persistenceTestCommand{Name: "fail"}
	cmdErr := errors.New("command failed")

	err := aspect.AfterCommand(ctx, cmd, nil, cmdErr, 10*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.Commands) != 1 {
		t.Fatalf("expected 1 command entry, got %d", len(store.Commands))
	}

	entry := store.Commands[0]
	if entry.Error != "command failed" {
		t.Errorf("expected Error 'command failed', got '%s'", entry.Error)
	}
	if entry.ResultType != "" {
		t.Errorf("expected empty ResultType, got '%s'", entry.ResultType)
	}
}

func TestPersistenceAspect_AfterCommandWithTraceContext(t *testing.T) {
	store := NewInMemoryMessageStore()
	aspect := NewPersistenceAspect(store)

	ctx := trace.WithTrace(context.Background(), "trace-123", "span-456")
	cmd := &persistenceTestCommand{Name: "traced"}

	err := aspect.AfterCommand(ctx, cmd, nil, nil, time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entry := store.Commands[0]
	if entry.TraceID != "trace-123" {
		t.Errorf("expected TraceID 'trace-123', got '%s'", entry.TraceID)
	}
	if entry.SpanID != "span-456" {
		t.Errorf("expected SpanID 'span-456', got '%s'", entry.SpanID)
	}
}

func TestPersistenceAspect_AfterQuery(t *testing.T) {
	store := NewInMemoryMessageStore()
	aspect := NewPersistenceAspect(store)

	ctx := context.Background()
	query := &persistenceTestQuery{ID: "q1"}
	result := &persistenceTestQueryResult{Value: "found"}
	duration := 30 * time.Millisecond

	err := aspect.AfterQuery(ctx, query, result, nil, duration)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.Queries) != 1 {
		t.Fatalf("expected 1 query entry, got %d", len(store.Queries))
	}

	entry := store.Queries[0]
	if entry.QueryType != "persistenceTestQuery" {
		t.Errorf("expected QueryType 'persistenceTestQuery', got '%s'", entry.QueryType)
	}
	if entry.ResultType != "persistenceTestQueryResult" {
		t.Errorf("expected ResultType 'persistenceTestQueryResult', got '%s'", entry.ResultType)
	}
	if entry.Duration != duration {
		t.Errorf("expected Duration %v, got %v", duration, entry.Duration)
	}
	if entry.Error != "" {
		t.Errorf("expected empty Error, got '%s'", entry.Error)
	}
	if entry.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestPersistenceAspect_AfterPublish_Event(t *testing.T) {
	store := NewInMemoryMessageStore()
	aspect := NewPersistenceAspect(store)

	ctx := context.Background()
	evt := &persistenceTestEvent{
		BaseEvent: event.NewBaseEvent("agg-1", time.Now()),
	}

	err := aspect.AfterPublish(ctx, evt, nil, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.Events) != 1 {
		t.Fatalf("expected 1 event entry, got %d", len(store.Events))
	}
	if len(store.Handlers) != 0 {
		t.Fatalf("expected 0 handler entries, got %d", len(store.Handlers))
	}

	entry := store.Events[0]
	if entry.AggregateID != "agg-1" {
		t.Errorf("expected AggregateID 'agg-1', got '%s'", entry.AggregateID)
	}
	if entry.EventType != "persistenceTestEvent" {
		t.Errorf("expected EventType 'persistenceTestEvent', got '%s'", entry.EventType)
	}
	if entry.Duration != 20*time.Millisecond {
		t.Errorf("expected Duration 20ms, got %v", entry.Duration)
	}
	if entry.Error != "" {
		t.Errorf("expected empty Error, got '%s'", entry.Error)
	}
	if entry.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}
}

func TestPersistenceAspect_AfterPublish_EventHandler(t *testing.T) {
	store := NewInMemoryMessageStore()
	aspect := NewPersistenceAspect(store)

	ctx := ContextWithHandlerType(context.Background(), "MyHandler")
	evt := &persistenceTestEvent{
		BaseEvent: event.NewBaseEvent("agg-2", time.Now()),
	}

	err := aspect.AfterPublish(ctx, evt, nil, 15*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.Handlers) != 1 {
		t.Fatalf("expected 1 handler entry, got %d", len(store.Handlers))
	}
	if len(store.Events) != 0 {
		t.Fatalf("expected 0 event entries, got %d", len(store.Events))
	}

	entry := store.Handlers[0]
	if entry.HandlerType != "MyHandler" {
		t.Errorf("expected HandlerType 'MyHandler', got '%s'", entry.HandlerType)
	}
	if entry.Status != "success" {
		t.Errorf("expected Status 'success', got '%s'", entry.Status)
	}
	if entry.AggregateID != "agg-2" {
		t.Errorf("expected AggregateID 'agg-2', got '%s'", entry.AggregateID)
	}
	if entry.EventType != "persistenceTestEvent" {
		t.Errorf("expected EventType 'persistenceTestEvent', got '%s'", entry.EventType)
	}
	if entry.Error != "" {
		t.Errorf("expected empty Error, got '%s'", entry.Error)
	}
}

func TestPersistenceAspect_AfterPublish_EventHandlerWithError(t *testing.T) {
	store := NewInMemoryMessageStore()
	aspect := NewPersistenceAspect(store)

	ctx := ContextWithHandlerType(context.Background(), "FailingHandler")
	evt := &persistenceTestEvent{
		BaseEvent: event.NewBaseEvent("agg-3", time.Now()),
	}
	handlerErr := errors.New("handler crashed")

	err := aspect.AfterPublish(ctx, evt, handlerErr, 5*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.Handlers) != 1 {
		t.Fatalf("expected 1 handler entry, got %d", len(store.Handlers))
	}

	entry := store.Handlers[0]
	if entry.Status != "error" {
		t.Errorf("expected Status 'error', got '%s'", entry.Status)
	}
	if entry.Error != "handler crashed" {
		t.Errorf("expected Error 'handler crashed', got '%s'", entry.Error)
	}
}

func TestPersistenceAspect_AfterPublish_EventWithError(t *testing.T) {
	store := NewInMemoryMessageStore()
	aspect := NewPersistenceAspect(store)

	ctx := context.Background()
	evt := &persistenceTestEvent{
		BaseEvent: event.NewBaseEvent("agg-4", time.Now()),
	}
	evtErr := errors.New("publish failed")

	err := aspect.AfterPublish(ctx, evt, evtErr, 3*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(store.Events) != 1 {
		t.Fatalf("expected 1 event entry, got %d", len(store.Events))
	}

	entry := store.Events[0]
	if entry.Error != "publish failed" {
		t.Errorf("expected Error 'publish failed', got '%s'", entry.Error)
	}
}

func TestPersistenceAspect_NameAndOrder(t *testing.T) {
	store := NewInMemoryMessageStore()
	aspect := NewPersistenceAspect(store)

	if aspect.Name() != "persistence" {
		t.Errorf("expected name 'persistence', got '%s'", aspect.Name())
	}
	if aspect.Order() != 200 {
		t.Errorf("expected order 200, got %d", aspect.Order())
	}
}

package trace_test

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/command"
	commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
	eventmemory "github.com/ddd-qce/core/cqrs/event/memory"
	"github.com/ddd-qce/core/trace"
)

type level1Command struct {
	command.BaseCommand
}

type level1Result struct {
	Message string
}

type level1Handler struct{}

func (h *level1Handler) Handle(ctx context.Context, cmd *level1Command) (*level1Result, error) {
	eventmemory.Dispatch[*level2Event](ctx, testEventBus, &level2Event{})
	return &level1Result{Message: "level1 done"}, nil
}

type level2Event struct{}

func (e *level2Event) AggregateID() string   { return "agg-2" }
func (e *level2Event) EventType() string     { return "Level2Event" }
func (e *level2Event) OccurredAt() time.Time { return time.Now() }

type level2Handler struct{}

func (h *level2Handler) Handle(ctx context.Context, event *level2Event) error {
	commandmemory.Dispatch[*level3Command, *level3Result](ctx, testCmdBus, &level3Command{})
	return nil
}

var testCmdBus *commandmemory.CommandBus
var testEventBus *eventmemory.EventBus

type level3Command struct {
	command.BaseCommand
}

type level3Result struct {
	Message string
}

type level3Handler struct{}

func (h *level3Handler) Handle(ctx context.Context, cmd *level3Command) (*level3Result, error) {
	eventmemory.Dispatch[*level4Event](ctx, testEventBus, &level4Event{})
	return &level3Result{Message: "level3 done"}, nil
}

type level4Event struct{}

func (e *level4Event) AggregateID() string   { return "agg-4" }
func (e *level4Event) EventType() string     { return "Level4Event" }
func (e *level4Event) OccurredAt() time.Time { return time.Now() }

type level4Handler struct{}

func (h *level4Handler) Handle(ctx context.Context, event *level4Event) error {
	commandmemory.Dispatch[*level5Command, *level5Result](ctx, testCmdBus, &level5Command{})
	return nil
}

type level5Command struct {
	command.BaseCommand
}

type level5Result struct {
	Message string
}

type level5Handler struct{}

func (h *level5Handler) Handle(ctx context.Context, cmd *level5Command) (*level5Result, error) {
	return &level5Result{Message: "level5 done"}, nil
}

type failingLevel5Handler struct{}

func (h *failingLevel5Handler) Handle(ctx context.Context, cmd *level5Command) (*level5Result, error) {
	return nil, &testError{"level5 failed"}
}

type testError struct {
	msg string
}

func (e *testError) Error() string { return e.msg }

func setupFiveLevelChain(t *testing.T) (*commandmemory.CommandBus, *eventmemory.EventBus, *trace.InMemoryTraceStore) {
	t.Helper()

	store := trace.NewInMemoryTraceStore()
	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(builtin.NewTracingAspect(store))
	chain.RegisterEventAspect(builtin.NewTracingAspect(store))

	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	eventBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

	testCmdBus = cmdBus
	testEventBus = eventBus

	commandmemory.RegisterCommand(cmdBus, &level1Handler{})
	commandmemory.RegisterCommand(cmdBus, &level3Handler{})
	commandmemory.RegisterCommand(cmdBus, &level5Handler{})

	eventmemory.RegisterHandler[*level2Event](eventBus, &level2Handler{})
	eventmemory.RegisterHandler[*level4Event](eventBus, &level4Handler{})

	return cmdBus, eventBus, store
}

func TestTrace_FiveLevelCallChain(t *testing.T) {
	cmdBus, _, store := setupFiveLevelChain(t)

	ctx := context.Background()
	_, err := commandmemory.Dispatch[*level1Command, *level1Result](ctx, cmdBus, &level1Command{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, err := store.ListTraces(ctx, trace.TraceFilter{})
	if err != nil {
		t.Fatalf("failed to list traces: %v", err)
	}

	if len(traceIDs) != 1 {
		t.Fatalf("expected 1 trace, got %d", len(traceIDs))
	}

	traceID := traceIDs[0]
	spans, err := store.GetTrace(ctx, traceID)
	if err != nil {
		t.Fatalf("failed to get trace: %v", err)
	}

	if len(spans) != 5 {
		t.Fatalf("expected 5 spans, got %d", len(spans))
	}
}

func TestTrace_FiveLevelCallChain_SameTraceID(t *testing.T) {
	cmdBus, _, store := setupFiveLevelChain(t)

	ctx := context.Background()
	_, err := commandmemory.Dispatch[*level1Command, *level1Result](ctx, cmdBus, &level1Command{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, _ := store.ListTraces(ctx, trace.TraceFilter{})
	spans, _ := store.GetTrace(ctx, traceIDs[0])

	for i, span := range spans {
		if span.TraceID != traceIDs[0] {
			t.Errorf("span %d has different traceID: expected %s, got %s", i, traceIDs[0], span.TraceID)
		}
	}
}

func TestTrace_FiveLevelCallChain_ParentChildRelationship(t *testing.T) {
	cmdBus, _, store := setupFiveLevelChain(t)

	ctx := context.Background()
	_, err := commandmemory.Dispatch[*level1Command, *level1Result](ctx, cmdBus, &level1Command{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, _ := store.ListTraces(ctx, trace.TraceFilter{})
	spans, _ := store.GetTrace(ctx, traceIDs[0])

	var root *trace.Span
	for _, s := range spans {
		if s.ParentID == "" {
			root = s
			break
		}
	}

	if root == nil {
		t.Fatal("no root span found")
	}

	if root.Name != "level1Command" {
		t.Errorf("expected root span name 'level1Command', got '%s'", root.Name)
	}

	current := root
	expectedNames := []string{"level1Command", "level2Event", "level3Command", "level4Event", "level5Command"}
	expectedTypes := []string{trace.SpanTypeCommand, trace.SpanTypeEvent, trace.SpanTypeCommand, trace.SpanTypeEvent, trace.SpanTypeCommand}

	for i := 0; i < 5; i++ {
		if current.Name != expectedNames[i] {
			t.Errorf("level %d: expected name '%s', got '%s'", i+1, expectedNames[i], current.Name)
		}
		if current.Type != expectedTypes[i] {
			t.Errorf("level %d: expected type '%s', got '%s'", i+1, expectedTypes[i], current.Type)
		}

		if i < 4 {
			var child *trace.Span
			for _, s := range spans {
				if s.ParentID == current.ID {
					child = s
					break
				}
			}
			if child == nil {
				t.Fatalf("level %d: no child span found for parent '%s'", i+1, current.Name)
			}
			current = child
		}
	}
}

func TestTrace_FiveLevelCallChain_SpanTypes(t *testing.T) {
	cmdBus, _, store := setupFiveLevelChain(t)

	ctx := context.Background()
	_, err := commandmemory.Dispatch[*level1Command, *level1Result](ctx, cmdBus, &level1Command{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, _ := store.ListTraces(ctx, trace.TraceFilter{})
	spans, _ := store.GetTrace(ctx, traceIDs[0])

	expectedSequence := []string{
		trace.SpanTypeCommand,
		trace.SpanTypeEvent,
		trace.SpanTypeCommand,
		trace.SpanTypeEvent,
		trace.SpanTypeCommand,
	}

	for i, span := range spans {
		if span.Type != expectedSequence[i] {
			t.Errorf("span %d: expected type '%s', got '%s'", i, expectedSequence[i], span.Type)
		}
	}
}

func TestTrace_FiveLevelCallChain_AllSpansRecorded(t *testing.T) {
	cmdBus, _, store := setupFiveLevelChain(t)

	ctx := context.Background()
	_, err := commandmemory.Dispatch[*level1Command, *level1Result](ctx, cmdBus, &level1Command{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	traceIDs, _ := store.ListTraces(ctx, trace.TraceFilter{})
	spans, _ := store.GetTrace(ctx, traceIDs[0])

	for _, span := range spans {
		if span.StartedAt.IsZero() {
			t.Errorf("span %s has zero StartedAt", span.Name)
		}
		if span.Duration < 0 {
			t.Errorf("span %s has negative duration", span.Name)
		}
		if span.Status != trace.SpanStatusSuccess {
			t.Errorf("span %s has unexpected status: %s", span.Name, span.Status)
		}
	}
}

func TestTrace_FiveLevelCallChain_ErrorPropagation(t *testing.T) {
	store := trace.NewInMemoryTraceStore()
	chain := aspect.NewAspectChain()
	chain.RegisterCommandAspect(builtin.NewTracingAspect(store))
	chain.RegisterEventAspect(builtin.NewTracingAspect(store))

	cmdBus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	eventBus := eventmemory.NewEventBus(eventmemory.WithBusAspectChain(chain))

	testCmdBus = cmdBus
	testEventBus = eventBus

	commandmemory.RegisterCommand(cmdBus, &level1Handler{})
	commandmemory.RegisterCommand(cmdBus, &level3Handler{})
	commandmemory.RegisterCommand(cmdBus, &failingLevel5Handler{})

	eventmemory.RegisterHandler[*level2Event](eventBus, &level2Handler{})
	eventmemory.RegisterHandler[*level4Event](eventBus, &level4Handler{})

	ctx := context.Background()
	_, _ = commandmemory.Dispatch[*level1Command, *level1Result](ctx, cmdBus, &level1Command{})

	traceIDs, _ := store.ListTraces(ctx, trace.TraceFilter{})
	spans, _ := store.GetTrace(ctx, traceIDs[0])

	if len(spans) != 5 {
		t.Fatalf("expected 5 spans, got %d", len(spans))
	}

	var level5Span *trace.Span
	for _, s := range spans {
		if s.Name == "level5Command" {
			level5Span = s
			break
		}
	}

	if level5Span == nil {
		t.Fatal("level5Command span not found")
	}
	if level5Span.Status != trace.SpanStatusError {
		t.Errorf("expected level5 span status 'error', got '%s'", level5Span.Status)
	}
	if level5Span.Error == "" {
		t.Error("expected level5 span to have error message")
	}
}

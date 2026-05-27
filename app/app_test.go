package app

import (
	"context"
	"testing"

	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/config"
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/impl/memory"
	"github.com/ddd-qce/core/cqrs/query"
	"github.com/ddd-qce/core/infra"
)

type testCommand struct{ command.BaseCommand }
type testCommandHandler struct{}

func (h *testCommandHandler) Handle(ctx context.Context, cmd testCommand) (string, error) {
	return "cmd-result", nil
}

type testQuery struct{ query.BaseQuery }
type testQueryHandler struct{}

func (h *testQueryHandler) Handle(ctx context.Context, q testQuery) (string, error) {
	return "query-result", nil
}

type testEvent struct{ event.BaseEvent }
type testEventHandler struct {
	called bool
}

func (h *testEventHandler) Handle(ctx context.Context, evt testEvent) error {
	h.called = true
	return nil
}

func TestWithBuses(t *testing.T) {
	cmdBus := memory.NewCommandBus()
	queryBus := memory.NewQueryBus()
	eventBus := memory.NewEventBus()

	app, err := NewApp(
		WithBuses(cmdBus, queryBus, eventBus),
	)
	if err != nil {
		t.Fatalf("NewApp with WithBuses: %v", err)
	}

	if app.CmdBus != cmdBus {
		t.Error("CmdBus not set correctly via WithBuses")
	}
	if app.QueryBus != queryBus {
		t.Error("QueryBus not set correctly via WithBuses")
	}
	if app.EventBus != eventBus {
		t.Error("EventBus not set correctly via WithBuses")
	}
}

func TestWithCommandBus(t *testing.T) {
	cmdBus := memory.NewCommandBus()

	app, err := NewApp(
		WithCommandBus(cmdBus),
	)
	if err != nil {
		t.Fatalf("NewApp with WithCommandBus: %v", err)
	}

	if app.CmdBus != cmdBus {
		t.Error("CmdBus not set correctly via WithCommandBus")
	}
}

func TestWithQueryBus(t *testing.T) {
	queryBus := memory.NewQueryBus()

	app, err := NewApp(
		WithQueryBus(queryBus),
	)
	if err != nil {
		t.Fatalf("NewApp with WithQueryBus: %v", err)
	}

	if app.QueryBus != queryBus {
		t.Error("QueryBus not set correctly via WithQueryBus")
	}
}

func TestWithEventBus(t *testing.T) {
	eventBus := memory.NewEventBus()

	app, err := NewApp(
		WithEventBus(eventBus),
	)
	if err != nil {
		t.Fatalf("NewApp with WithEventBus: %v", err)
	}

	if app.EventBus != eventBus {
		t.Error("EventBus not set correctly via WithEventBus")
	}
}

func TestWithBuses_AllowsHandlerRegistration(t *testing.T) {
	cmdBus := memory.NewCommandBus()
	queryBus := memory.NewQueryBus()
	eventBus := memory.NewEventBus()

	if err := cmdBus.RegisterHandler(&testCommandHandler{}); err != nil {
		t.Fatalf("register command handler: %v", err)
	}
	if err := queryBus.RegisterHandler(&testQueryHandler{}); err != nil {
		t.Fatalf("register query handler: %v", err)
	}
	handler := &testEventHandler{}
	if err := eventBus.SubscribeHandler(handler); err != nil {
		t.Fatalf("subscribe event handler: %v", err)
	}

	app, err := NewApp(
		WithBuses(cmdBus, queryBus, eventBus),
	)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}

	cmdResult, err := command.Dispatch[testCommand, string](context.Background(), app.CmdBus, testCommand{})
	if err != nil {
		t.Fatalf("dispatch command: %v", err)
	}
	if cmdResult != "cmd-result" {
		t.Errorf("command result = %q, want %q", cmdResult, "cmd-result")
	}

	queryResult, err := query.Dispatch[testQuery, string](context.Background(), app.QueryBus, testQuery{})
	if err != nil {
		t.Fatalf("dispatch query: %v", err)
	}
	if queryResult != "query-result" {
		t.Errorf("query result = %q, want %q", queryResult, "query-result")
	}

	evt := testEvent{event.NewDomainEvent("test-aggregate")}
	if err := event.Dispatch(context.Background(), app.EventBus, evt); err != nil {
		t.Fatalf("dispatch event: %v", err)
	}
	if !handler.called {
		t.Error("event handler was not called")
	}
}

func TestWithBuses_NilBusUsesDefault(t *testing.T) {
	app, err := NewApp(
		WithBuses(nil, nil, nil),
	)
	if err != nil {
		t.Fatalf("NewApp with nil buses: %v", err)
	}

	if app.CmdBus != nil {
		t.Error("expected CmdBus to be nil when nil passed")
	}
	if app.QueryBus != nil {
		t.Error("expected QueryBus to be nil when nil passed")
	}
	if app.EventBus != nil {
		t.Error("expected EventBus to be nil when nil passed")
	}
}

func TestWithDefaultAspects_NilBackend_NoPanic(t *testing.T) {
	app, err := NewApp(WithDefaultAspects())
	if err != nil {
		t.Fatalf("WithDefaultAspects without Backend should not error, got: %v", err)
	}
	if app.Chain == nil {
		t.Error("expected AspectChain to be created")
	}
}

func TestWithDefaultAspects_BackendNilTraceStore_NoPanic(t *testing.T) {
	backend := infra.NewBackend()
	app, err := NewApp(
		WithBackend(backend),
		WithDefaultAspects(),
	)
	if err != nil {
		t.Fatalf("WithDefaultAspects with Backend but nil TraceStore should not error, got: %v", err)
	}
	if app.Chain == nil {
		t.Error("expected AspectChain to be created")
	}
}

func TestWithDefaultAspects_TracingDisabled_NoPanic(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Aspect.EnableTracing = false
	app, err := NewApp(
		WithConfig(cfg),
		WithDefaultAspects(),
	)
	if err != nil {
		t.Fatalf("WithDefaultAspects with tracing disabled should not error, got: %v", err)
	}
	if app.Chain == nil {
		t.Error("expected AspectChain to be created")
	}
}

func TestWithDefaultAspects_NilStoreRecordSpan_NoPanic(t *testing.T) {
	aspect := builtin.NewTracingAspect(nil)
	ctx := context.Background()
	ctx = context.WithValue(ctx, struct{}{}, &struct{}{})
	aspect.AfterCommand(ctx, "cmd", nil, nil, 0)
	aspect.AfterQuery(ctx, "query", nil, nil, 0)
	aspect.AfterPublish(ctx, "event", nil, 0)
}

func WithConfig(cfg *config.Config) AppOption {
	return func(a *App) error {
		a.Config = cfg
		return nil
	}
}

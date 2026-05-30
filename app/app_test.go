package app

import (
	"context"
	"errors"
	"testing"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/config"
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
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

func testBusFactory() infra.BusFactory {
	return infra.NewMemoryBusFactory()
}

func TestWithBuses(t *testing.T) {
	factory := testBusFactory()
	cmdBus := factory.CreateCommandBus(aspect.NewAspectChain())
	queryBus := factory.CreateQueryBus(aspect.NewAspectChain())
	eventBus := factory.CreateEventBus(aspect.NewAspectChain())

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
	cmdBus := testBusFactory().CreateCommandBus(nil)

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
	queryBus := testBusFactory().CreateQueryBus(nil)

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
	eventBus := testBusFactory().CreateEventBus(nil)

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
	factory := testBusFactory()
	cmdBus := factory.CreateCommandBus(aspect.NewAspectChain())
	queryBus := factory.CreateQueryBus(aspect.NewAspectChain())
	eventBus := factory.CreateEventBus(aspect.NewAspectChain())

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
	if err := app.EventBus.Publish(context.Background(), evt); err != nil {
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

func TestClose_NoLifecycles(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if err := app.Close(context.Background()); err != nil {
		t.Fatalf("Close with no lifecycles should return nil, got: %v", err)
	}
}

func TestClose_WithLifecycles(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	called := false
	app.RegisterLifecycle(LifecycleFunc(func(ctx context.Context) error {
		called = true
		return nil
	}))
	if err := app.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !called {
		t.Error("lifecycle Shutdown was not called")
	}
}

func TestClose_LifecycleError(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	app.RegisterLifecycle(LifecycleFunc(func(ctx context.Context) error {
		return errors.New("shutdown-err-1")
	}))
	app.RegisterLifecycle(LifecycleFunc(func(ctx context.Context) error {
		return errors.New("shutdown-err-2")
	}))
	err = app.Close(context.Background())
	if err == nil {
		t.Fatal("expected error from Close with failing lifecycles")
	}
	if len(err.Error()) == 0 {
		t.Error("error message should contain aggregated errors")
	}
}

func TestClose_CleanupFunctions(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	cleanupCalled := false
	app.cleanup = append(app.cleanup, func() error {
		cleanupCalled = true
		return nil
	})
	if err := app.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !cleanupCalled {
		t.Error("cleanup function was not called")
	}
}

func TestClose_CancelledContext(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	var secondCalled bool
	app.RegisterLifecycle(LifecycleFunc(func(ctx context.Context) error {
		return nil
	}))
	app.RegisterLifecycle(LifecycleFunc(func(ctx context.Context) error {
		secondCalled = true
		return nil
	}))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = app.Close(ctx)
	if secondCalled {
		t.Error("second lifecycle should not be called when context is already cancelled, but Close iterates after checking ctx.Err()")
	}
}

func TestRegisterLifecycle(t *testing.T) {
	app, err := NewApp()
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	called := false
	app.RegisterLifecycle(LifecycleFunc(func(ctx context.Context) error {
		called = true
		return nil
	}))
	if err := app.Close(context.Background()); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !called {
		t.Error("RegisterLifecycle did not register the lifecycle; Shutdown was not called on Close")
	}
}

func TestLifecycleFunc(t *testing.T) {
	called := false
	fn := LifecycleFunc(func(ctx context.Context) error {
		called = true
		return nil
	})
	if err := fn.Shutdown(context.Background()); err != nil {
		t.Fatalf("LifecycleFunc.Shutdown: %v", err)
	}
	if !called {
		t.Error("LifecycleFunc did not call the underlying function")
	}
}

func TestWithCommandHandlers(t *testing.T) {
	app, err := NewApp(WithCommandHandlers(&testCommandHandler{}))
	if err != nil {
		t.Fatalf("NewApp with WithCommandHandlers: %v", err)
	}
	if app.CmdBus == nil {
		t.Fatal("CmdBus should be created by WithCommandHandlers")
	}
	result, err := command.Dispatch[testCommand, string](context.Background(), app.CmdBus, testCommand{})
	if err != nil {
		t.Fatalf("dispatch command: %v", err)
	}
	if result != "cmd-result" {
		t.Errorf("command result = %q, want %q", result, "cmd-result")
	}
}

func TestWithCommandHandlers_ExistingBus(t *testing.T) {
	factory := testBusFactory()
	cmdBus := factory.CreateCommandBus(aspect.NewAspectChain())
	app, err := NewApp(
		WithCommandBus(cmdBus),
		WithCommandHandlers(&testCommandHandler{}),
	)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.CmdBus != cmdBus {
		t.Error("WithCommandHandlers should use existing bus, not create a new one")
	}
}

func TestWithCommandHandlers_RegistrationError(t *testing.T) {
	_, err := NewApp(WithCommandHandlers("not-a-handler"))
	if err == nil {
		t.Fatal("expected error when registering invalid command handler")
	}
}

func TestWithQueryHandlers(t *testing.T) {
	app, err := NewApp(WithQueryHandlers(&testQueryHandler{}))
	if err != nil {
		t.Fatalf("NewApp with WithQueryHandlers: %v", err)
	}
	if app.QueryBus == nil {
		t.Fatal("QueryBus should be created by WithQueryHandlers")
	}
	result, err := query.Dispatch[testQuery, string](context.Background(), app.QueryBus, testQuery{})
	if err != nil {
		t.Fatalf("dispatch query: %v", err)
	}
	if result != "query-result" {
		t.Errorf("query result = %q, want %q", result, "query-result")
	}
}

func TestWithQueryHandlers_ExistingBus(t *testing.T) {
	factory := testBusFactory()
	queryBus := factory.CreateQueryBus(aspect.NewAspectChain())
	app, err := NewApp(
		WithQueryBus(queryBus),
		WithQueryHandlers(&testQueryHandler{}),
	)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.QueryBus != queryBus {
		t.Error("WithQueryHandlers should use existing bus, not create a new one")
	}
}

func TestWithEventSubscriptions(t *testing.T) {
	handler := &testEventHandler{}
	app, err := NewApp(WithEventSubscriptions(handler))
	if err != nil {
		t.Fatalf("NewApp with WithEventSubscriptions: %v", err)
	}
	if app.EventBus == nil {
		t.Fatal("EventBus should be created by WithEventSubscriptions")
	}
	evt := testEvent{event.NewDomainEvent("test-aggregate")}
	if err := app.EventBus.Publish(context.Background(), evt); err != nil {
		t.Fatalf("dispatch event: %v", err)
	}
	if !handler.called {
		t.Error("event handler was not called")
	}
}

func TestWithEventSubscriptions_ExistingBus(t *testing.T) {
	factory := testBusFactory()
	eventBus := factory.CreateEventBus(aspect.NewAspectChain())
	app, err := NewApp(
		WithEventBus(eventBus),
		WithEventSubscriptions(&testEventHandler{}),
	)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.EventBus != eventBus {
		t.Error("WithEventSubscriptions should use existing bus, not create a new one")
	}
}

func TestWithLogger(t *testing.T) {
	logger := builtin.NewStdLogger()
	app, err := NewApp(WithLogger(logger))
	if err != nil {
		t.Fatalf("NewApp with WithLogger: %v", err)
	}
	if app.Chain == nil {
		t.Fatal("AspectChain should be created by WithLogger")
	}
}

func TestWithLogger_ExistingChain(t *testing.T) {
	existingChain := aspect.NewAspectChain()
	app, err := NewApp(
		func(a *App) error {
			a.Chain = existingChain
			return nil
		},
		WithLogger(builtin.NewStdLogger()),
	)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.Chain != existingChain {
		t.Error("WithLogger should use existing chain, not create a new one")
	}
}

func TestWithMetrics(t *testing.T) {
	recorder := builtin.NewInMemMetricsRecorder()
	app, err := NewApp(WithMetrics(recorder))
	if err != nil {
		t.Fatalf("NewApp with WithMetrics: %v", err)
	}
	if app.Chain == nil {
		t.Fatal("AspectChain should be created by WithMetrics")
	}
}

func TestWithMetrics_ExistingChain(t *testing.T) {
	existingChain := aspect.NewAspectChain()
	app, err := NewApp(
		func(a *App) error {
			a.Chain = existingChain
			return nil
		},
		WithMetrics(builtin.NewInMemMetricsRecorder()),
	)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.Chain != existingChain {
		t.Error("WithMetrics should use existing chain, not create a new one")
	}
}

func TestNewApp_OptionError(t *testing.T) {
	optErr := errors.New("option-failed")
	_, err := NewApp(func(a *App) error {
		return optErr
	})
	if err == nil {
		t.Fatal("expected error when option fails")
	}
}

func TestWithConfigFile_NotFound(t *testing.T) {
	_, err := NewApp(WithConfigFile("/nonexistent/path/config.toml"))
	if err == nil {
		t.Fatal("expected error for non-existent config file")
	}
}

func TestWithDefaultAspects_AllEnabled(t *testing.T) {
	cfg := config.DefaultConfig()
	app, err := NewApp(
		WithConfig(cfg),
		WithDefaultAspects(),
	)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.Chain == nil {
		t.Fatal("AspectChain should be created")
	}
}

func TestWithDefaultAspects_WithBackendTxManager(t *testing.T) {
	backend := infra.NewMemoryBackend()
	cfg := config.DefaultConfig()
	app, err := NewApp(
		WithConfig(cfg),
		WithBackend(backend),
		WithDefaultAspects(),
	)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.Chain == nil {
		t.Fatal("AspectChain should be created")
	}
}

func TestWithCommandHandlers_WithBackend(t *testing.T) {
	backend := infra.NewMemoryBackend()
	app, err := NewApp(
		WithBackend(backend),
		WithCommandHandlers(&testCommandHandler{}),
	)
	if err != nil {
		t.Fatalf("NewApp: %v", err)
	}
	if app.CmdBus == nil {
		t.Fatal("CmdBus should be created")
	}
	result, err := command.Dispatch[testCommand, string](context.Background(), app.CmdBus, testCommand{})
	if err != nil {
		t.Fatalf("dispatch command: %v", err)
	}
	if result != "cmd-result" {
		t.Errorf("command result = %q, want %q", result, "cmd-result")
	}
}

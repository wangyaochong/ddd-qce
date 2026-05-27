package impl

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/impl/memory"
)

type integrationCommand struct {
	command.BaseCommand
	Name string
}

type integrationResult struct {
	ID string
}

type integrationHandler struct{}

func (h *integrationHandler) Handle(ctx context.Context, c *integrationCommand) (*integrationResult, error) {
	return &integrationResult{ID: "int-" + c.Name}, nil
}

type integrationFailingCommand struct {
	command.BaseCommand
}

type integrationFailingResult struct{}

type integrationFailingHandler struct{}

func (h *integrationFailingHandler) Handle(ctx context.Context, c *integrationFailingCommand) (*integrationFailingResult, error) {
	return nil, context.DeadlineExceeded
}

func TestCommandBus_InterfaceSatisfaction(t *testing.T) {
	chain := aspect.NewAspectChain()
	var _ 	command.CommandBus = memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
}

func TestCommandBus_Execute(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &integrationHandler{})

	var executor 	command.CommandBus = bus

	result, err := executor.Execute(context.Background(), &integrationCommand{Name: "via-executor"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r, ok := result.(*integrationResult)
	if !ok {
		t.Fatalf("expected *integrationResult, got %T", result)
	}
	if r.ID != "int-via-executor" {
		t.Errorf("expected 'int-via-executor', got '%s'", r.ID)
	}
}

func TestCommandBus_Execute_NoHandler(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))

	var executor 	command.CommandBus = bus

	_, err := executor.Execute(context.Background(), &integrationCommand{Name: "no-handler"})
	if err == nil {
		t.Fatal("expected error for unregistered command")
	}
}

func TestCommandBus_Execute_Error(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &integrationFailingHandler{})

	var executor 	command.CommandBus = bus

	_, err := executor.Execute(context.Background(), &integrationFailingCommand{})
	if err == nil {
		t.Fatal("expected error from failing handler")
	}
}

func TestCommandDispatch_GenericAPI(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &integrationHandler{})

	result, err := 	command.Dispatch[*integrationCommand, *integrationResult](context.Background(), bus, &integrationCommand{Name: "dispatch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "int-dispatch" {
		t.Errorf("expected 'int-dispatch', got '%s'", result.ID)
	}
}

func TestDispatch_GenericAPI_NoHandler(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))

	_, err := 	command.Dispatch[*integrationCommand, *integrationResult](context.Background(), bus, &integrationCommand{Name: "no-handler"})
	if err == nil {
		t.Fatal("expected error for unregistered command")
	}
}

func TestCommandBus_Execute_WithAspects(t *testing.T) {
	chain := aspect.NewAspectChain()

	var beforeCalled, afterCalled bool
	chain.RegisterCommandAspect(&testIntegrationAspect{
		beforeFn: func() { beforeCalled = true },
		afterFn:  func() { afterCalled = true },
	})

	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &integrationHandler{})

	var executor 	command.CommandBus = bus

	_, err := executor.Execute(context.Background(), &integrationCommand{Name: "aspected"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !beforeCalled {
		t.Error("BeforeCommand not called")
	}
	if !afterCalled {
		t.Error("AfterCommand not called")
	}
}

func TestCommandNameOf_ViaInterface(t *testing.T) {
	c := &integrationCommand{Name: "test"}
	name := 	command.CommandNameOf(c)
	if name != "integrationCommand" {
		t.Errorf("expected 'integrationCommand', got '%s'", name)
	}
}

type testIntegrationAspect struct {
	beforeFn func()
	afterFn  func()
}

func (a *testIntegrationAspect) Name() string { return "test-integration" }
func (a *testIntegrationAspect) Order() int   { return 1 }
func (a *testIntegrationAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	if a.beforeFn != nil {
		a.beforeFn()
	}
	return ctx, nil
}
func (a *testIntegrationAspect) AfterCommand(ctx context.Context, cmd any, r any, err error, d time.Duration) error {
	if a.afterFn != nil {
		a.afterFn()
	}
	return nil
}

func TestCommandDispatch_InterfaceLevel(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &integrationHandler{})

	result, err := 	command.Dispatch[*integrationCommand, *integrationResult](context.Background(), bus, &integrationCommand{Name: "iface-dispatch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "int-iface-dispatch" {
		t.Errorf("expected 'int-iface-dispatch', got '%s'", result.ID)
	}
}

func TestCommandDispatch_InterfaceLevel_NoHandler(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))

	_, err := 	command.Dispatch[*integrationCommand, *integrationResult](context.Background(), bus, &integrationCommand{Name: "no-handler"})
	if err == nil {
		t.Fatal("expected error for unregistered command")
	}
}

func TestCommandDispatch_InterfaceLevel_HandlerError(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &integrationFailingHandler{})

	_, err := 	command.Dispatch[*integrationFailingCommand, *integrationFailingResult](context.Background(), bus, &integrationFailingCommand{})
	if err == nil {
		t.Fatal("expected error from failing handler")
	}
}

func TestCommandDispatch_InterfaceLevel_NilResult(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := memory.NewCommandBus(memory.WithCommandBusAspectChain(chain))
	memory.RegisterCommand(bus, &nilResultHandler{})

	result, err := 	command.Dispatch[*nilResultCommand, *nilResult](context.Background(), bus, &nilResultCommand{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got %v", result)
	}
}

type nilResultCommand struct {
	command.BaseCommand
}

type nilResult struct{}

type nilResultHandler struct{}

func (h *nilResultHandler) Handle(ctx context.Context, c *nilResultCommand) (*nilResult, error) {
	return nil, nil
}
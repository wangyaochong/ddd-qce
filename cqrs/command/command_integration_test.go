package command_test

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
	commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
)

type integrationCommand struct {
	command.BaseCommand
	Name string
}

type integrationResult struct {
	ID string
}

type integrationHandler struct{}

func (h *integrationHandler) Handle(ctx context.Context, cmd *integrationCommand) (*integrationResult, error) {
	return &integrationResult{ID: "int-" + cmd.Name}, nil
}

type integrationFailingCommand struct {
	command.BaseCommand
}

type integrationFailingResult struct{}

type integrationFailingHandler struct{}

func (h *integrationFailingHandler) Handle(ctx context.Context, cmd *integrationFailingCommand) (*integrationFailingResult, error) {
	return nil, context.DeadlineExceeded
}

func TestCommandExecutor_InterfaceSatisfaction(t *testing.T) {
	chain := aspect.NewAspectChain()
	var _ command.CommandExecutor = commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
}

func TestCommandExecutor_Execute(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	commandmemory.RegisterCommand(bus, &integrationHandler{})

	var executor command.CommandExecutor = bus

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

func TestCommandExecutor_Execute_NoHandler(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))

	var executor command.CommandExecutor = bus

	_, err := executor.Execute(context.Background(), &integrationCommand{Name: "no-handler"})
	if err == nil {
		t.Fatal("expected error for unregistered command")
	}
}

func TestCommandExecutor_Execute_Error(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	commandmemory.RegisterCommand(bus, &integrationFailingHandler{})

	var executor command.CommandExecutor = bus

	_, err := executor.Execute(context.Background(), &integrationFailingCommand{})
	if err == nil {
		t.Fatal("expected error from failing handler")
	}
}

func TestDispatch_GenericAPI(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	commandmemory.RegisterCommand(bus, &integrationHandler{})

	result, err := commandmemory.Dispatch[*integrationCommand, *integrationResult](context.Background(), bus, &integrationCommand{Name: "dispatch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "int-dispatch" {
		t.Errorf("expected 'int-dispatch', got '%s'", result.ID)
	}
}

func TestDispatch_GenericAPI_NoHandler(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))

	_, err := commandmemory.Dispatch[*integrationCommand, *integrationResult](context.Background(), bus, &integrationCommand{Name: "no-handler"})
	if err == nil {
		t.Fatal("expected error for unregistered command")
	}
}

func TestCommandExecutor_Execute_WithAspects(t *testing.T) {
	chain := aspect.NewAspectChain()

	var beforeCalled, afterCalled bool
	chain.RegisterCommandAspect(&testIntegrationAspect{
		beforeFn: func() { beforeCalled = true },
		afterFn:  func() { afterCalled = true },
	})

	bus := commandmemory.NewCommandBus(commandmemory.WithCommandBusAspectChain(chain))
	commandmemory.RegisterCommand(bus, &integrationHandler{})

	var executor command.CommandExecutor = bus

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
	cmd := &integrationCommand{Name: "test"}
	name := command.CommandNameOf(cmd)
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

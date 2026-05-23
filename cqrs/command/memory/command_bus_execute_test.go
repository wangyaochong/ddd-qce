package memory

import (
	"context"
	"testing"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
)

type executeTestCommand struct {
	command.BaseCommand
	Value int
}

type executeTestResult struct {
	Doubled int
}

type executeTestHandler struct{}

func (h *executeTestHandler) Handle(ctx context.Context, cmd *executeTestCommand) (*executeTestResult, error) {
	return &executeTestResult{Doubled: cmd.Value * 2}, nil
}

type executeErrorCommand struct {
	command.BaseCommand
}

type executeErrorResult struct{}

type executeErrorHandler struct{}

func (h *executeErrorHandler) Handle(ctx context.Context, cmd *executeErrorCommand) (*executeErrorResult, error) {
	return nil, &executeTestErr{"execute failed"}
}

type executeTestErr struct {
	msg string
}

func (e *executeTestErr) Error() string { return e.msg }

func TestCommandBus_Execute(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewCommandBus(chain)
	RegisterCommand(bus, &executeTestHandler{})

	ctx := context.Background()
	cmd := &executeTestCommand{Value: 21}

	result, err := bus.Execute(ctx, cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	r, ok := result.(*executeTestResult)
	if !ok {
		t.Fatalf("expected *executeTestResult, got %T", result)
	}
	if r.Doubled != 42 {
		t.Errorf("expected doubled 42, got %d", r.Doubled)
	}
}

func TestCommandBus_Execute_NoHandler(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewCommandBus(chain)

	ctx := context.Background()
	cmd := &executeTestCommand{Value: 1}

	_, err := bus.Execute(ctx, cmd)
	if err == nil {
		t.Fatal("expected error for unregistered command")
	}
}

func TestCommandBus_Execute_Error(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewCommandBus(chain)
	RegisterCommand(bus, &executeErrorHandler{})

	ctx := context.Background()
	cmd := &executeErrorCommand{}

	_, err := bus.Execute(ctx, cmd)
	if err == nil {
		t.Fatal("expected error from handler")
	}
	if err.Error() != "execute failed" {
		t.Errorf("expected 'execute failed', got '%v'", err)
	}
}

func TestCommandBus_Execute_MultipleCommands(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewCommandBus(chain)
	RegisterCommand(bus, &executeTestHandler{})
	RegisterCommand(bus, &testCreateUserHandler{})

	ctx := context.Background()

	r1, err := bus.Execute(ctx, &executeTestCommand{Value: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r1.(*executeTestResult).Doubled != 20 {
		t.Errorf("expected 20, got %d", r1.(*executeTestResult).Doubled)
	}

	r2, err := bus.Execute(ctx, &testCreateUserCommand{Name: "bob"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r2.(*testCreateUserResult).ID != "cmd-bob" {
		t.Errorf("expected 'cmd-bob', got '%s'", r2.(*testCreateUserResult).ID)
	}
}

func TestCommandBus_Execute_WithAspects(t *testing.T) {
	chain := aspect.NewAspectChain()

	var beforeCalled, afterCalled bool
	testAspect := &testCommandAspect{
		beforeFn: func() { beforeCalled = true },
		afterFn:  func() { afterCalled = true },
	}
	chain.RegisterCommandAspect(testAspect)

	bus := NewCommandBus(chain)
	RegisterCommand(bus, &executeTestHandler{})

	ctx := context.Background()
	_, err := bus.Execute(ctx, &executeTestCommand{Value: 5})
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

func TestInvokeHandler_NoHandleMethod(t *testing.T) {
	type badHandler struct{}

	handler := &badHandler{}
	cmd := &executeTestCommand{Value: 1}
	ctx := context.Background()

	_, err := invokeHandler(handler, cmd, ctx)
	if err == nil {
		t.Fatal("expected error for handler without Handle method")
	}
	if err.Error() != "handler does not have Handle method" {
		t.Errorf("expected 'handler does not have Handle method', got '%v'", err)
	}
}

type interfaceHandler struct{}

func (h *interfaceHandler) Handle(ctx context.Context, cmd command.CommandHandler[*executeTestCommand, *executeTestResult]) (*executeTestResult, error) {
	return nil, nil
}

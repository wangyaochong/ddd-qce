package impl

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ddd-qce/core/cqrs/cmd"
)

type testCommand struct {
	cmd.BaseCommand
	Name string
}

type testCommandResult struct {
	ID string
}

type testCommandHandler struct{}

func (h *testCommandHandler) Handle(ctx context.Context, c *testCommand) (*testCommandResult, error) {
	return &testCommandResult{ID: "result-" + c.Name}, nil
}

type testFailingCommand struct {
	cmd.BaseCommand
}

type testFailingCommandResult struct{}

type testFailingCommandHandler struct{}

func (h *testFailingCommandHandler) Handle(ctx context.Context, c *testFailingCommand) (*testFailingCommandResult, error) {
	return nil, errors.New("command failed")
}

type testCommandWithoutBase struct {
	cmd.BaseCommand
	Name string
}

func TestBaseCommand_ImplementsCommand(t *testing.T) {
	var _ cmd.Command = (*testCommand)(nil)
	var _ cmd.Command = (*testFailingCommand)(nil)
	var _ cmd.Command = (*testCommandWithoutBase)(nil)
	var _ cmd.Command = cmd.BaseCommand{}
}

func TestCommandNameOf_PointerType(t *testing.T) {
	c := &testCommand{Name: "test"}
	name := cmd.CommandNameOf(c)
	if name != "testCommand" {
		t.Errorf("expected 'testCommand', got '%s'", name)
	}
}

func TestCommandNameOf_ValueType(t *testing.T) {
	c := testCommand{Name: "test"}
	name := cmd.CommandNameOf(c)
	if name != "testCommand" {
		t.Errorf("expected 'testCommand', got '%s'", name)
	}
}

func TestCommandNameOf_DifferentTypes(t *testing.T) {
	tests := []struct {
		cmd  any
		want string
	}{
		{&testCommand{}, "testCommand"},
		{&testFailingCommand{}, "testFailingCommand"},
		{&testCommandWithoutBase{}, "testCommandWithoutBase"},
		{testCommand{}, "testCommand"},
		{cmd.BaseCommand{}, "BaseCommand"},
	}
	for _, tt := range tests {
		got := cmd.CommandNameOf(tt.cmd)
		if got != tt.want {
			t.Errorf("CommandNameOf(%T) = '%s', want '%s'", tt.cmd, got, tt.want)
		}
	}
}

func TestCommandHandler_Handle(t *testing.T) {
	ctx := context.Background()
	handler := &testCommandHandler{}

	result, err := handler.Handle(ctx, &testCommand{Name: "hello"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "result-hello" {
		t.Errorf("expected 'result-hello', got '%s'", result.ID)
	}
}

func TestCommandHandler_Error(t *testing.T) {
	ctx := context.Background()
	handler := &testFailingCommandHandler{}

	_, err := handler.Handle(ctx, &testFailingCommand{})
	if err == nil {
		t.Fatal("expected error from failing handler")
	}
	if err.Error() != "command failed" {
		t.Errorf("expected 'command failed', got '%v'", err)
	}
}

func TestCommandHandler_InterfaceSatisfaction(t *testing.T) {
	var _ cmd.CommandHandler[*testCommand, *testCommandResult] = &testCommandHandler{}
	var _ cmd.CommandHandler[*testFailingCommand, *testFailingCommandResult] = &testFailingCommandHandler{}
}

func TestCommand_InterfaceEnforcement(t *testing.T) {
	c := &testCommand{Name: "test"}
	var _ cmd.Command = c

	if _, ok := any(c).(cmd.Command); !ok {
		t.Error("testCommand should implement Command interface")
	}

	plain := &struct{ Name string }{Name: "not-a-command"}
	if _, ok := any(plain).(cmd.Command); ok {
		t.Error("plain struct should NOT implement Command interface")
	}
}

func TestCommandNameOf_UnexportedType(t *testing.T) {
	type internalCommand struct {
		cmd.BaseCommand
		Data string
	}
	c := &internalCommand{Data: "test"}
	name := cmd.CommandNameOf(c)
	if name != "internalCommand" {
		t.Errorf("expected 'internalCommand', got '%s'", name)
	}
}

func TestBaseCommand_MarkerMethod(t *testing.T) {
	var c cmd.Command = cmd.BaseCommand{}
	_ = c
}

func TestCommandNameOf_ReflectTypeConsistency(t *testing.T) {
	c := &testCommand{Name: "test"}
	ptrName := cmd.CommandNameOf(c)

	var zero *testCommand
	typeName := reflect.TypeOf(zero).Elem().Name()

	if ptrName != typeName {
		t.Errorf("CommandNameOf pointer '%s' doesn't match reflect type name '%s'", ptrName, typeName)
	}
}

func TestCommandHandler_ContextPropagation(t *testing.T) {
	ctx := context.WithValue(context.Background(), "testKey", "testValue")
	handler := &testCommandHandler{}

	result, err := handler.Handle(ctx, &testCommand{Name: "ctx"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "result-ctx" {
		t.Errorf("expected 'result-ctx', got '%s'", result.ID)
	}
}

func TestCommand_WithCustomIsCommand(t *testing.T) {
	type customCmd struct{ id int }
	if _, ok := any(&customCmd{}).(cmd.Command); ok {
		t.Error("type without isCommand() should NOT satisfy Command")
	}
}
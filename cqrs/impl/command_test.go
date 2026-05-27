package impl

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ddd-qce/core/cqrs/command"
)

type testCommand struct {
	command.BaseCommand
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
	command.BaseCommand
}

type testFailingCommandResult struct{}

type testFailingCommandHandler struct{}

func (h *testFailingCommandHandler) Handle(ctx context.Context, c *testFailingCommand) (*testFailingCommandResult, error) {
	return nil, errors.New("command failed")
}

type testCommandWithoutBase struct {
	command.BaseCommand
	Name string
}

func TestBaseCommand_ImplementsCommand(t *testing.T) {
	var _ 	command.Command = (*testCommand)(nil)
	var _ 	command.Command = (*testFailingCommand)(nil)
	var _ 	command.Command = (*testCommandWithoutBase)(nil)
	var _ 	command.Command = command.BaseCommand{}
}

func TestCommandNameOf_PointerType(t *testing.T) {
	c := &testCommand{Name: "test"}
	name := 	command.CommandNameOf(c)
	if name != "testCommand" {
		t.Errorf("expected 'testCommand', got '%s'", name)
	}
}

func TestCommandNameOf_ValueType(t *testing.T) {
	c := testCommand{Name: "test"}
	name := 	command.CommandNameOf(c)
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
		{command.BaseCommand{}, "BaseCommand"},
	}
	for _, tt := range tests {
		got := 	command.CommandNameOf(tt.cmd)
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
	var _ 	command.CommandHandler[*testCommand, *testCommandResult] = &testCommandHandler{}
	var _ 	command.CommandHandler[*testFailingCommand, *testFailingCommandResult] = &testFailingCommandHandler{}
}

func TestCommand_InterfaceEnforcement(t *testing.T) {
	c := &testCommand{Name: "test"}
	var _ 	command.Command = c

	if _, ok := any(c).(	command.Command); !ok {
		t.Error("testCommand should implement Command interface")
	}

	plain := &struct{ Name string }{Name: "not-a-command"}
	if _, ok := any(plain).(	command.Command); ok {
		t.Error("plain struct should NOT implement Command interface")
	}
}

func TestCommandNameOf_UnexportedType(t *testing.T) {
	type internalCommand struct {
		command.BaseCommand
		Data string
	}
	c := &internalCommand{Data: "test"}
	name := 	command.CommandNameOf(c)
	if name != "internalCommand" {
		t.Errorf("expected 'internalCommand', got '%s'", name)
	}
}

func TestBaseCommand_MarkerMethod(t *testing.T) {
	var c 	command.Command = command.BaseCommand{}
	_ = c
}

func TestCommandNameOf_ReflectTypeConsistency(t *testing.T) {
	c := &testCommand{Name: "test"}
	ptrName := 	command.CommandNameOf(c)

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
	if _, ok := any(&customCmd{}).(	command.Command); ok {
		t.Error("type without isCommand() should NOT satisfy Command")
	}
}
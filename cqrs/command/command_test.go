package command

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type testCommand struct {
	BaseCommand
	Name string
}

type testCommandResult struct {
	ID string
}

type testCommandHandler struct{}

func (h *testCommandHandler) Handle(ctx context.Context, cmd *testCommand) (*testCommandResult, error) {
	return &testCommandResult{ID: "result-" + cmd.Name}, nil
}

type testFailingCommand struct {
	BaseCommand
}

type testFailingCommandResult struct{}

type testFailingCommandHandler struct{}

func (h *testFailingCommandHandler) Handle(ctx context.Context, cmd *testFailingCommand) (*testFailingCommandResult, error) {
	return nil, errors.New("command failed")
}

type testCommandWithoutBase struct {
	Name string
}

func (testCommandWithoutBase) isCommand() {}

func TestBaseCommand_ImplementsCommand(t *testing.T) {
	var _ Command = (*testCommand)(nil)
	var _ Command = (*testFailingCommand)(nil)
	var _ Command = (*testCommandWithoutBase)(nil)
	var _ Command = BaseCommand{}
}

func TestCommandNameOf_PointerType(t *testing.T) {
	cmd := &testCommand{Name: "test"}
	name := CommandNameOf(cmd)
	if name != "testCommand" {
		t.Errorf("expected 'testCommand', got '%s'", name)
	}
}

func TestCommandNameOf_ValueType(t *testing.T) {
	cmd := testCommand{Name: "test"}
	name := CommandNameOf(cmd)
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
		{BaseCommand{}, "BaseCommand"},
	}
	for _, tt := range tests {
		got := CommandNameOf(tt.cmd)
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
	var _ CommandHandler[*testCommand, *testCommandResult] = &testCommandHandler{}
	var _ CommandHandler[*testFailingCommand, *testFailingCommandResult] = &testFailingCommandHandler{}
}

func TestCommand_InterfaceEnforcement(t *testing.T) {
	cmd := &testCommand{Name: "test"}
	var _ Command = cmd

	if _, ok := any(cmd).(Command); !ok {
		t.Error("testCommand should implement Command interface")
	}

	plain := &struct{ Name string }{Name: "not-a-command"}
	if _, ok := any(plain).(Command); ok {
		t.Error("plain struct should NOT implement Command interface")
	}
}

func TestCommandNameOf_UnexportedType(t *testing.T) {
	type internalCommand struct {
		BaseCommand
		Data string
	}
	cmd := &internalCommand{Data: "test"}
	name := CommandNameOf(cmd)
	if name != "internalCommand" {
		t.Errorf("expected 'internalCommand', got '%s'", name)
	}
}

func TestBaseCommand_MarkerMethod(t *testing.T) {
	var cmd Command = BaseCommand{}
	cmd.isCommand()
}

func TestCommandNameOf_ReflectTypeConsistency(t *testing.T) {
	cmd := &testCommand{Name: "test"}
	ptrName := CommandNameOf(cmd)

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
	// customCmd does NOT embed BaseCommand, so it doesn't implement Command
	// unless isCommand() is added. This test verifies the marker interface pattern.
	if _, ok := any(&customCmd{}).(Command); ok {
		t.Error("type without isCommand() should NOT satisfy Command")
	}
}

package command

import (
	"context"
	"errors"
	"testing"
)

type testCommand struct {
	BaseCommand
	Value string
}

func (testCommand) isCommand() {}

type testResult struct {
	Result string
}

type testCommandHandler struct {
	result string
	err    error
}

func (h *testCommandHandler) Handle(ctx context.Context, cmd testCommand) (testResult, error) {
	if h.err != nil {
		return testResult{}, h.err
	}
	return testResult{Result: h.result}, nil
}

type testCommandBus struct {
	handlers    map[string]any
	executeErr  error
	executeRet  any
}

func (b *testCommandBus) Execute(ctx context.Context, cmd any) (any, error) {
	if b.executeErr != nil {
		return nil, b.executeErr
	}
	return b.executeRet, nil
}

func (b *testCommandBus) RegisterHandler(handler any) error {
	name := CommandNameOf(handler)
	b.handlers[name] = handler
	return nil
}

func (b *testCommandBus) RegisteredTypes() []string {
	names := make([]string, 0, len(b.handlers))
	for k := range b.handlers {
		names = append(names, k)
	}
	return names
}

func (b *testCommandBus) Shutdown(ctx context.Context) error {
	return nil
}

func TestCommandNameOf(t *testing.T) {
	tests := []struct {
		name     string
		cmd      any
		expected string
	}{
		{"struct type", testCommand{}, "testCommand"},
		{"pointer type", &testCommand{}, "testCommand"},
		{"base command", BaseCommand{}, "BaseCommand"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CommandNameOf(tt.cmd)
			if result != tt.expected {
				t.Errorf("CommandNameOf(%T) = %q, want %q", tt.cmd, result, tt.expected)
			}
		})
	}
}

func TestCommandNameOf_NilPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("CommandNameOf(nil) should panic")
		}
		if msg, ok := r.(string); !ok || msg != "command: CommandNameOf called with nil command" {
			t.Errorf("unexpected panic message: %v", r)
		}
	}()
	CommandNameOf(nil)
}

func TestDispatch_Success(t *testing.T) {
	bus := &testCommandBus{
		handlers:   make(map[string]any),
		executeRet: testResult{Result: "success"},
	}

	result, err := Dispatch[testCommand, testResult](context.Background(), bus, testCommand{Value: "test"})
	if err != nil {
		t.Errorf("Dispatch() error = %v, want nil", err)
	}
	if result.Result != "success" {
		t.Errorf("Dispatch() = %v, want testResult{Result: \"success\"}", result)
	}
}

func TestDispatch_ExecuteError(t *testing.T) {
	bus := &testCommandBus{
		handlers:   make(map[string]any),
		executeErr: errors.New("execute failed"),
	}

	result, err := Dispatch[testCommand, testResult](context.Background(), bus, testCommand{})
	if err == nil {
		t.Error("Dispatch() should return error from Execute")
	}
	if result != (testResult{}) {
		t.Error("Dispatch() should return zero value on error")
	}
}

func TestDispatch_TypeMismatch(t *testing.T) {
	bus := &testCommandBus{
		handlers:   make(map[string]any),
		executeRet: "wrong type",
	}

	result, err := Dispatch[testCommand, testResult](context.Background(), bus, testCommand{})
	if err == nil {
		t.Error("Dispatch() should return error on type mismatch")
	}
	if result != (testResult{}) {
		t.Error("Dispatch() should return zero value on error")
	}
}

func TestDispatch_NilResult(t *testing.T) {
	bus := &testCommandBus{
		handlers:   make(map[string]any),
		executeRet: nil,
	}

	result, err := Dispatch[testCommand, testResult](context.Background(), bus, testCommand{})
	if err != nil {
		t.Errorf("Dispatch() error = %v, want nil", err)
	}
	if result != (testResult{}) {
		t.Error("Dispatch() should return zero value for nil result")
	}
}

func TestTypeAssert_NilResult(t *testing.T) {
	result, err := typeAssert[testResult](nil, testCommand{})
	if err != nil {
		t.Errorf("typeAssert() error = %v, want nil", err)
	}
	if result != (testResult{}) {
		t.Error("typeAssert() should return zero value for nil result")
	}
}

func TestTypeAssert_TypeMismatch(t *testing.T) {
	result, err := typeAssert[testResult]("wrong type", testCommand{})
	if err == nil {
		t.Error("typeAssert() should return error on type mismatch")
	}
	if result != (testResult{}) {
		t.Error("typeAssert() should return zero value on error")
	}
}

func TestTypeAssert_Success(t *testing.T) {
	result, err := typeAssert[testResult](testResult{Result: "ok"}, testCommand{})
	if err != nil {
		t.Errorf("typeAssert() error = %v, want nil", err)
	}
	if result.Result != "ok" {
		t.Errorf("typeAssert() = %v, want testResult{Result: \"ok\"}", result)
	}
}
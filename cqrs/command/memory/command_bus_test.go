package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/command"
)

type testCreateUserCommand struct {
	command.BaseCommand
	Name string
}

type testCreateUserResult struct {
	ID string
}

type testCreateUserHandler struct{}

func (h *testCreateUserHandler) Handle(ctx context.Context, cmd *testCreateUserCommand) (*testCreateUserResult, error) {
	return &testCreateUserResult{ID: "cmd-" + cmd.Name}, nil
}

type testDeleteUserCommand struct {
	command.BaseCommand
	UserID string
}

type testDeleteUserResult struct {
	Deleted bool
}

type testDeleteUserHandler struct{}

func (h *testDeleteUserHandler) Handle(ctx context.Context, cmd *testDeleteUserCommand) (*testDeleteUserResult, error) {
	return &testDeleteUserResult{Deleted: cmd.UserID != ""}, nil
}

type testErrorCommand struct {
	command.BaseCommand
}

type testErrorCommandResult struct{}

type testErrorCommandHandler struct{}

func (h *testErrorCommandHandler) Handle(ctx context.Context, cmd *testErrorCommand) (*testErrorCommandResult, error) {
	return nil, errors.New("command handler error")
}

type testSlowCommand struct {
	command.BaseCommand
	Duration time.Duration
}

type testSlowCommandResult struct {
	Done bool
}

type testSlowCommandHandler struct{}

func (h *testSlowCommandHandler) Handle(ctx context.Context, cmd *testSlowCommand) (*testSlowCommandResult, error) {
	select {
	case <-time.After(cmd.Duration):
		return &testSlowCommandResult{Done: true}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestCommandBus_Dispatch(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewCommandBus(chain)
	RegisterCommand(bus, &testCreateUserHandler{})

	ctx := context.Background()
	result, err := Dispatch[*testCreateUserCommand, *testCreateUserResult](bus, ctx, &testCreateUserCommand{Name: "test"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "cmd-test" {
		t.Errorf("expected ID 'cmd-test', got '%s'", result.ID)
	}
}

func TestCommandBus_Dispatch_NoHandler(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewCommandBus(chain)

	ctx := context.Background()
	_, err := Dispatch[*testCreateUserCommand, *testCreateUserResult](bus, ctx, &testCreateUserCommand{Name: "test"})

	if err == nil {
		t.Fatal("expected error for unregistered command type")
	}
}

func TestCommandBus_MultipleHandlers(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewCommandBus(chain)
	RegisterCommand(bus, &testCreateUserHandler{})
	RegisterCommand(bus, &testDeleteUserHandler{})

	ctx := context.Background()

	r1, err := Dispatch[*testCreateUserCommand, *testCreateUserResult](bus, ctx, &testCreateUserCommand{Name: "alice"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r1.ID != "cmd-alice" {
		t.Errorf("unexpected result: %v", r1)
	}

	r2, err := Dispatch[*testDeleteUserCommand, *testDeleteUserResult](bus, ctx, &testDeleteUserCommand{UserID: "123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !r2.Deleted {
		t.Errorf("expected deleted=true, got: %v", r2)
	}
}

func TestCommandBus_HandlerError(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewCommandBus(chain)
	RegisterCommand(bus, &testErrorCommandHandler{})

	ctx := context.Background()
	_, err := Dispatch[*testErrorCommand, *testErrorCommandResult](bus, ctx, &testErrorCommand{})

	if err == nil {
		t.Fatal("expected error from handler")
	}
	if err.Error() != "command handler error" {
		t.Errorf("expected 'command handler error', got '%v'", err)
	}
}

func TestCommandBus_Concurrent(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewCommandBus(chain)
	RegisterCommand(bus, &testCreateUserHandler{})

	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := Dispatch[*testCreateUserCommand, *testCreateUserResult](bus, ctx, &testCreateUserCommand{Name: string(rune(id))})
			if err != nil {
				errs <- err
			}
		}(i)
	}

	wg.Wait()
	close(errs)

	for err := range errs {
		t.Errorf("concurrent error: %v", err)
	}
}

func TestCommandBus_NilChain(t *testing.T) {
	bus := NewCommandBus(nil)
	RegisterCommand(bus, &testCreateUserHandler{})

	ctx := context.Background()
	result, err := Dispatch[*testCreateUserCommand, *testCreateUserResult](bus, ctx, &testCreateUserCommand{Name: "test"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "cmd-test" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestCommandBus_WithContextCancel(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewCommandBus(chain)
	RegisterCommand(bus, &testSlowCommandHandler{})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	_, err := Dispatch[*testSlowCommand, *testSlowCommandResult](bus, ctx, &testSlowCommand{Duration: 5 * time.Second})

	if err == nil {
		t.Fatal("expected context cancelled error")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got: %v", err)
	}
}

func TestCommandBus_WithAspects(t *testing.T) {
	chain := aspect.NewAspectChain()

	var beforeCalled, afterCalled bool
	testAspect := &testCommandAspect{
		beforeFn: func() { beforeCalled = true },
		afterFn:  func() { afterCalled = true },
	}
	chain.RegisterCommandAspect(testAspect)

	bus := NewCommandBus(chain)
	RegisterCommand(bus, &testCreateUserHandler{})

	ctx := context.Background()
	_, err := Dispatch[*testCreateUserCommand, *testCreateUserResult](bus, ctx, &testCreateUserCommand{Name: "test"})

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

func TestCommandBus_DuplicateRegistration_Panics(t *testing.T) {
	bus := NewCommandBus(nil)
	RegisterCommand(bus, &testCreateUserHandler{})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for duplicate registration")
		}
		msg := r.(string)
		expected := "handler already registered for command type: *memory.testCreateUserCommand"
		if msg != expected {
			t.Errorf("expected panic message %q, got %q", expected, msg)
		}
	}()

	RegisterCommand(bus, &testCreateUserHandler{})
}

type testCommandAspect struct {
	beforeFn func()
	afterFn  func()
}

func (a *testCommandAspect) Name() string { return "test" }
func (a *testCommandAspect) Order() int   { return 1 }
func (a *testCommandAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	if a.beforeFn != nil {
		a.beforeFn()
	}
	return ctx, nil
}
func (a *testCommandAspect) AfterCommand(ctx context.Context, cmd any, r any, err error, d time.Duration) error {
	if a.afterFn != nil {
		a.afterFn()
	}
	return nil
}

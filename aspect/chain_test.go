package aspect

import (
	"context"
	"errors"
	"testing"
	"time"
)

type testQuery struct {
	ID string
}

type testQueryResult struct {
	Value string
}

type testCommand struct {
	ID string
}

type testCommandResult struct {
	Success bool
}

type testEvent struct {
	id string
}

func (e *testEvent) AggregateID() string   { return e.id }
func (e *testEvent) EventType() string     { return "TestEvent" }
func (e *testEvent) OccurredAt() time.Time { return time.Now() }

type testQueryAspect struct {
	beforeCalled bool
	afterCalled  bool
}

func (a *testQueryAspect) Name() string { return "test-query" }
func (a *testQueryAspect) Order() int   { return 1 }
func (a *testQueryAspect) BeforeQuery(ctx context.Context, q any) (context.Context, error) {
	a.beforeCalled = true
	return ctx, nil
}
func (a *testQueryAspect) AfterQuery(ctx context.Context, q any, r any, err error, d time.Duration) error {
	a.afterCalled = true
	return nil
}

type testCommandAspect struct {
	beforeCalled bool
	afterCalled  bool
}

func (a *testCommandAspect) Name() string { return "test-command" }
func (a *testCommandAspect) Order() int   { return 1 }
func (a *testCommandAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	a.beforeCalled = true
	return ctx, nil
}
func (a *testCommandAspect) AfterCommand(ctx context.Context, cmd any, r any, err error, d time.Duration) error {
	a.afterCalled = true
	return nil
}

type testEventAspect struct {
	beforeCalled bool
	afterCalled  bool
}

func (a *testEventAspect) Name() string { return "test-event" }
func (a *testEventAspect) Order() int   { return 1 }
func (a *testEventAspect) BeforePublish(ctx context.Context, e any) (context.Context, error) {
	a.beforeCalled = true
	return ctx, nil
}
func (a *testEventAspect) AfterPublish(ctx context.Context, e any, err error, d time.Duration) error {
	a.afterCalled = true
	return nil
}

func TestAspectChain_QueryAspects(t *testing.T) {
	chain := NewAspectChain()
	aspect := &testQueryAspect{}
	chain.RegisterQueryAspect(aspect)

	q := &testQuery{ID: "1"}
	result, err := chain.ExecuteWithQueryAspects(context.Background(), q, func(ctx context.Context) (any, error) {
		return &testQueryResult{Value: "ok"}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !aspect.beforeCalled {
		t.Error("BeforeQuery not called")
	}
	if !aspect.afterCalled {
		t.Error("AfterQuery not called")
	}
	if result.(*testQueryResult).Value != "ok" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestAspectChain_CommandAspects(t *testing.T) {
	chain := NewAspectChain()
	aspect := &testCommandAspect{}
	chain.RegisterCommandAspect(aspect)

	cmd := &testCommand{ID: "1"}
	result, err := chain.ExecuteWithCommandAspects(context.Background(), cmd, func(ctx context.Context) (any, error) {
		return &testCommandResult{Success: true}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !aspect.beforeCalled {
		t.Error("BeforeCommand not called")
	}
	if !aspect.afterCalled {
		t.Error("AfterCommand not called")
	}
	if !result.(*testCommandResult).Success {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestAspectChain_EventAspects(t *testing.T) {
	chain := NewAspectChain()
	aspect := &testEventAspect{}
	chain.RegisterEventAspect(aspect)

	event := &testEvent{id: "1"}
	err := chain.ExecuteWithEventAspects(context.Background(), event, func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !aspect.beforeCalled {
		t.Error("BeforePublish not called")
	}
	if !aspect.afterCalled {
		t.Error("AfterPublish not called")
	}
}

func TestAspectChain_BeforeError(t *testing.T) {
	chain := NewAspectChain()
	chain.RegisterQueryAspect(&testQueryAspect{})
	chain.RegisterQueryAspect(&queryAspectErrorBefore{})

	q := &testQuery{ID: "1"}
	_, err := chain.ExecuteWithQueryAspects(context.Background(), q, func(ctx context.Context) (any, error) {
		return &testQueryResult{Value: "ok"}, nil
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

type queryAspectErrorBefore struct{}

func (a *queryAspectErrorBefore) Name() string { return "error-before" }
func (a *queryAspectErrorBefore) Order() int   { return 2 }
func (a *queryAspectErrorBefore) BeforeQuery(ctx context.Context, q any) (context.Context, error) {
	return ctx, errors.New("before error")
}
func (a *queryAspectErrorBefore) AfterQuery(ctx context.Context, q any, r any, err error, d time.Duration) error {
	return nil
}

func TestAspectChain_Ordering(t *testing.T) {
	chain := NewAspectChain()

	var order []string
	chain.RegisterQueryAspect(&orderedQueryAspect{name: "second", order: 2, fn: func() { order = append(order, "second") }})
	chain.RegisterQueryAspect(&orderedQueryAspect{name: "first", order: 1, fn: func() { order = append(order, "first") }})

	q := &testQuery{ID: "1"}
	_, _ = chain.ExecuteWithQueryAspects(context.Background(), q, func(ctx context.Context) (any, error) {
		return &testQueryResult{Value: "ok"}, nil
	})

	if len(order) != 2 || order[0] != "first" || order[1] != "second" {
		t.Errorf("expected order [first, second], got %v", order)
	}
}

type orderedQueryAspect struct {
	name  string
	order int
	fn    func()
}

func (a *orderedQueryAspect) Name() string { return a.name }
func (a *orderedQueryAspect) Order() int   { return a.order }
func (a *orderedQueryAspect) BeforeQuery(ctx context.Context, q any) (context.Context, error) {
	a.fn()
	return ctx, nil
}
func (a *orderedQueryAspect) AfterQuery(ctx context.Context, q any, r any, err error, d time.Duration) error {
	return nil
}

func TestAspectChain_MultipleAspects(t *testing.T) {
	chain := NewAspectChain()

	var calls []string
	chain.RegisterCommandAspect(&multiAspect{name: "A", order: 1, calls: &calls})
	chain.RegisterCommandAspect(&multiAspect{name: "B", order: 2, calls: &calls})
	chain.RegisterCommandAspect(&multiAspect{name: "C", order: 3, calls: &calls})

	cmd := &testCommand{ID: "1"}
	_, _ = chain.ExecuteWithCommandAspects(context.Background(), cmd, func(ctx context.Context) (any, error) {
		calls = append(calls, "handler")
		return &testCommandResult{Success: true}, nil
	})

	expected := []string{"A-before", "B-before", "C-before", "handler", "C-after", "B-after", "A-after"}
	if len(calls) != len(expected) {
		t.Fatalf("expected %d calls, got %d", len(expected), len(calls))
	}
	for i, c := range expected {
		if calls[i] != c {
			t.Errorf("call %d: expected %s, got %s", i, c, calls[i])
		}
	}
}

type multiAspect struct {
	name  string
	order int
	calls *[]string
}

func (a *multiAspect) Name() string { return a.name }
func (a *multiAspect) Order() int   { return a.order }
func (a *multiAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	*a.calls = append(*a.calls, a.name+"-before")
	return ctx, nil
}
func (a *multiAspect) AfterCommand(ctx context.Context, cmd any, r any, err error, d time.Duration) error {
	*a.calls = append(*a.calls, a.name+"-after")
	return nil
}

func TestAspectChain_AfterErrorOverrides(t *testing.T) {
	chain := NewAspectChain()
	chain.RegisterCommandAspect(&commandAspectAfterError{})

	cmd := &testCommand{ID: "1"}
	_, err := chain.ExecuteWithCommandAspects(context.Background(), cmd, func(ctx context.Context) (any, error) {
		return &testCommandResult{Success: true}, nil
	})

	if err == nil {
		t.Fatal("expected error from AfterCommand")
	}
	if err.Error() != "after error" {
		t.Errorf("expected 'after error', got '%v'", err)
	}
}

type commandAspectAfterError struct{}

func (a *commandAspectAfterError) Name() string { return "after-error" }
func (a *commandAspectAfterError) Order() int   { return 1 }
func (a *commandAspectAfterError) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	return ctx, nil
}
func (a *commandAspectAfterError) AfterCommand(ctx context.Context, cmd any, r any, err error, d time.Duration) error {
	return errors.New("after error")
}

func TestAspectChain_DurationMeasurement(t *testing.T) {
	chain := NewAspectChain()

	var measuredDuration time.Duration
	chain.RegisterCommandAspect(&durationMeasuringAspect{duration: &measuredDuration})

	cmd := &testCommand{ID: "1"}
	_, _ = chain.ExecuteWithCommandAspects(context.Background(), cmd, func(ctx context.Context) (any, error) {
		time.Sleep(50 * time.Millisecond)
		return &testCommandResult{Success: true}, nil
	})

	if measuredDuration < 50*time.Millisecond {
		t.Errorf("expected duration >= 50ms, got %v", measuredDuration)
	}
}

type durationMeasuringAspect struct {
	duration *time.Duration
}

func (a *durationMeasuringAspect) Name() string { return "duration" }
func (a *durationMeasuringAspect) Order() int   { return 1 }
func (a *durationMeasuringAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	return ctx, nil
}
func (a *durationMeasuringAspect) AfterCommand(ctx context.Context, cmd any, r any, err error, d time.Duration) error {
	*a.duration = d
	return nil
}

func TestAspectChain_EmptyChain(t *testing.T) {
	chain := NewAspectChain()

	q := &testQuery{ID: "1"}
	result, err := chain.ExecuteWithQueryAspects(context.Background(), q, func(ctx context.Context) (any, error) {
		return &testQueryResult{Value: "ok"}, nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.(*testQueryResult).Value != "ok" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestAspectChain_ContextPropagation(t *testing.T) {
	chain := NewAspectChain()

	type ctxKey struct{}
	chain.RegisterQueryAspect(&contextSettingAspect{key: ctxKey{}, value: "test-value"})

	q := &testQuery{ID: "1"}
	_, _ = chain.ExecuteWithQueryAspects(context.Background(), q, func(ctx context.Context) (any, error) {
		val := ctx.Value(ctxKey{})
		if val != "test-value" {
			return nil, errors.New("context value not propagated")
		}
		return &testQueryResult{Value: "ok"}, nil
	})
}

type contextSettingAspect struct {
	key   any
	value any
}

func (a *contextSettingAspect) Name() string { return "context-setting" }
func (a *contextSettingAspect) Order() int   { return 1 }
func (a *contextSettingAspect) BeforeQuery(ctx context.Context, q any) (context.Context, error) {
	return context.WithValue(ctx, a.key, a.value), nil
}
func (a *contextSettingAspect) AfterQuery(ctx context.Context, q any, r any, err error, d time.Duration) error {
	return nil
}

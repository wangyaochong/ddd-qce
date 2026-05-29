package aspect

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/ddd-qce/core/cqrs/event"
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
	event.BaseEvent
}

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

	event := &testEvent{BaseEvent: event.NewBaseEvent("1", time.Now())}
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

func TestAspectChain_QueryAspectAfterError(t *testing.T) {
	chain := NewAspectChain()
	chain.RegisterQueryAspect(&queryAspectAfterError{})

	ctx := context.Background()
	q := &testQuery{ID: "1"}
	_, err := chain.ExecuteWithQueryAspects(ctx, q, func(ctx context.Context) (any, error) {
		return &testQueryResult{Value: "ok"}, nil
	})

	if err == nil {
		t.Fatal("expected error from AfterQuery")
	}
	if err.Error() != "after query error" {
		t.Errorf("expected 'after query error', got '%v'", err)
	}
}

type queryAspectAfterError struct{}

func (a *queryAspectAfterError) Name() string { return "error-after" }
func (a *queryAspectAfterError) Order() int   { return 1 }
func (a *queryAspectAfterError) BeforeQuery(ctx context.Context, q any) (context.Context, error) {
	return ctx, nil
}
func (a *queryAspectAfterError) AfterQuery(ctx context.Context, q any, r any, err error, d time.Duration) error {
	return errors.New("after query error")
}

func TestAspectChain_CommandAspectBeforeError(t *testing.T) {
	chain := NewAspectChain()
	chain.RegisterCommandAspect(&commandAspectErrorBefore{})

	ctx := context.Background()
	cmd := &testCommand{ID: "1"}
	_, err := chain.ExecuteWithCommandAspects(ctx, cmd, func(ctx context.Context) (any, error) {
		return &testCommandResult{Success: true}, nil
	})

	if err == nil {
		t.Fatal("expected error from BeforeCommand")
	}
	if err.Error() != "before command error" {
		t.Errorf("expected 'before command error', got '%v'", err)
	}
}

type commandAspectErrorBefore struct{}

func (a *commandAspectErrorBefore) Name() string { return "error-before" }
func (a *commandAspectErrorBefore) Order() int   { return 1 }
func (a *commandAspectErrorBefore) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	return ctx, errors.New("before command error")
}
func (a *commandAspectErrorBefore) AfterCommand(ctx context.Context, cmd any, r any, err error, d time.Duration) error {
	return nil
}

func TestAspectChain_RegisterEventAspect(t *testing.T) {
	chain := NewAspectChain()

	aspect1 := &eventTestAspect{}
	aspect2 := &eventOrderedTestAspect{name: "first", order: 1}
	aspect3 := &eventOrderedTestAspect{name: "second", order: 2}

	chain.RegisterEventAspect(aspect3)
	chain.RegisterEventAspect(aspect2)
	chain.RegisterEventAspect(aspect1)

	ctx := context.Background()
	err := chain.ExecuteWithEventAspects(ctx, &testEvent{BaseEvent: event.NewBaseEvent("1", time.Now())}, func(ctx context.Context) error {
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !aspect1.beforeCalled {
		t.Error("BeforePublish not called")
	}
	if !aspect1.afterCalled {
		t.Error("AfterPublish not called")
	}
}

type eventOrderedTestAspect struct {
	name  string
	order int
}

func (a *eventOrderedTestAspect) Name() string { return a.name }
func (a *eventOrderedTestAspect) Order() int   { return a.order }
func (a *eventOrderedTestAspect) BeforePublish(ctx context.Context, e any) (context.Context, error) {
	return ctx, nil
}
func (a *eventOrderedTestAspect) AfterPublish(ctx context.Context, e any, err error, d time.Duration) error {
	return nil
}

type eventTestAspect struct {
	beforeCalled bool
	afterCalled  bool
}

func (a *eventTestAspect) Name() string { return "test-event" }
func (a *eventTestAspect) Order() int   { return 1 }
func (a *eventTestAspect) BeforePublish(ctx context.Context, e any) (context.Context, error) {
	a.beforeCalled = true
	return ctx, nil
}
func (a *eventTestAspect) AfterPublish(ctx context.Context, e any, err error, d time.Duration) error {
	a.afterCalled = true
	return nil
}

func TestAspectChain_EventAspectBeforeError(t *testing.T) {
	chain := NewAspectChain()
	chain.RegisterEventAspect(&eventAspectErrorBefore{})

	ctx := context.Background()
	err := chain.ExecuteWithEventAspects(ctx, &testEvent{BaseEvent: event.NewBaseEvent("1", time.Now())}, func(ctx context.Context) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error from BeforePublish")
	}
	if err.Error() != "before publish error" {
		t.Errorf("expected 'before publish error', got '%v'", err)
	}
}

type eventAspectErrorBefore struct{}

func (a *eventAspectErrorBefore) Name() string { return "error-before" }
func (a *eventAspectErrorBefore) Order() int   { return 1 }
func (a *eventAspectErrorBefore) BeforePublish(ctx context.Context, e any) (context.Context, error) {
	return ctx, errors.New("before publish error")
}
func (a *eventAspectErrorBefore) AfterPublish(ctx context.Context, e any, err error, d time.Duration) error {
	return nil
}

func TestAspectChain_RegisterAspect_ImplementsAll(t *testing.T) {
	chain := NewAspectChain()
	a := &allAspect{}
	chain.RegisterAspect(a)

	cmd := &testCommand{ID: "1"}
	_, _ = chain.ExecuteWithCommandAspects(context.Background(), cmd, func(ctx context.Context) (any, error) {
		return &testCommandResult{Success: true}, nil
	})

	q := &testQuery{ID: "1"}
	_, _ = chain.ExecuteWithQueryAspects(context.Background(), q, func(ctx context.Context) (any, error) {
		return &testQueryResult{Value: "ok"}, nil
	})

	evt := &testEvent{BaseEvent: event.NewBaseEvent("1", time.Now())}
	_ = chain.ExecuteWithEventAspects(context.Background(), evt, func(ctx context.Context) error {
		return nil
	})

	if !a.commandBefore {
		t.Error("BeforeCommand not called")
	}
	if !a.commandAfter {
		t.Error("AfterCommand not called")
	}
	if !a.queryBefore {
		t.Error("BeforeQuery not called")
	}
	if !a.queryAfter {
		t.Error("AfterQuery not called")
	}
	if !a.eventBefore {
		t.Error("BeforePublish not called")
	}
	if !a.eventAfter {
		t.Error("AfterPublish not called")
	}
}

func TestAspectChain_RegisterAspect_CommandOnly(t *testing.T) {
	chain := NewAspectChain()
	a := &commandOnlyAspect{}
	chain.RegisterAspect(a)

	cmd := &testCommand{ID: "1"}
	_, _ = chain.ExecuteWithCommandAspects(context.Background(), cmd, func(ctx context.Context) (any, error) {
		return &testCommandResult{Success: true}, nil
	})

	q := &testQuery{ID: "1"}
	_, _ = chain.ExecuteWithQueryAspects(context.Background(), q, func(ctx context.Context) (any, error) {
		return &testQueryResult{Value: "ok"}, nil
	})

	if !a.beforeCalled {
		t.Error("BeforeCommand not called")
	}
	if !a.afterCalled {
		t.Error("AfterCommand not called")
	}
	if chain.queryAspects != nil && len(chain.queryAspects) > 0 {
		t.Error("expected no query aspects registered from command-only aspect")
	}
	if chain.eventAspects != nil && len(chain.eventAspects) > 0 {
		t.Error("expected no event aspects registered from command-only aspect")
	}
}

type allAspect struct {
	commandBefore bool
	commandAfter  bool
	queryBefore   bool
	queryAfter    bool
	eventBefore   bool
	eventAfter    bool
}

func (a *allAspect) Name() string { return "all" }
func (a *allAspect) Order() int   { return 1 }
func (a *allAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	a.commandBefore = true
	return ctx, nil
}
func (a *allAspect) AfterCommand(ctx context.Context, cmd any, r any, err error, d time.Duration) error {
	a.commandAfter = true
	return nil
}
func (a *allAspect) BeforeQuery(ctx context.Context, q any) (context.Context, error) {
	a.queryBefore = true
	return ctx, nil
}
func (a *allAspect) AfterQuery(ctx context.Context, q any, r any, err error, d time.Duration) error {
	a.queryAfter = true
	return nil
}
func (a *allAspect) BeforePublish(ctx context.Context, e any) (context.Context, error) {
	a.eventBefore = true
	return ctx, nil
}
func (a *allAspect) AfterPublish(ctx context.Context, e any, err error, d time.Duration) error {
	a.eventAfter = true
	return nil
}

type commandOnlyAspect struct {
	beforeCalled bool
	afterCalled  bool
}

func (a *commandOnlyAspect) Name() string { return "command-only" }
func (a *commandOnlyAspect) Order() int   { return 1 }
func (a *commandOnlyAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	a.beforeCalled = true
	return ctx, nil
}
func (a *commandOnlyAspect) AfterCommand(ctx context.Context, cmd any, r any, err error, d time.Duration) error {
	a.afterCalled = true
	return nil
}

func TestAspectChain_HasAspect(t *testing.T) {
	chain := NewAspectChain()

	if chain.HasAspect("test-query") {
		t.Error("expected HasAspect to return false for empty chain")
	}

	chain.RegisterQueryAspect(&testQueryAspect{})

	if !chain.HasAspect("test-query") {
		t.Error("expected HasAspect to return true after registration")
	}

	if chain.HasAspect("non-existent") {
		t.Error("expected HasAspect to return false for unregistered name")
	}
}

func TestAspectChain_HasAspect_DedupAcrossCategories(t *testing.T) {
	chain := NewAspectChain()

	chain.RegisterCommandAspect(&testCommandAspect{})

	if !chain.HasAspect("test-command") {
		t.Error("expected HasAspect to find command aspect")
	}

	chain.RegisterQueryAspect(&testQueryAspect{})

	if !chain.HasAspect("test-query") {
		t.Error("expected HasAspect to find query aspect")
	}

	chain.RegisterEventAspect(&testEventAspect{})

	if !chain.HasAspect("test-event") {
		t.Error("expected HasAspect to find event aspect")
	}
}

func TestAspectChain_RegisteredNames(t *testing.T) {
	chain := NewAspectChain()

	names := chain.RegisteredNames()
	if len(names) != 0 {
		t.Errorf("expected empty names for empty chain, got %v", names)
	}

	chain.RegisterCommandAspect(&multiAspect{name: "A", order: 1, calls: nil})
	chain.RegisterCommandAspect(&multiAspect{name: "B", order: 2, calls: nil})
	chain.RegisterEventAspect(&testEventAspect{})

	names = chain.RegisteredNames()
	if len(names) != 3 {
		t.Errorf("expected 3 names, got %d: %v", len(names), names)
	}
}

func TestAspectChain_RegisteredNames_DedupSameName(t *testing.T) {
	chain := NewAspectChain()

	chain.RegisterCommandAspect(&multiAspect{name: "A", order: 1, calls: nil})
	chain.RegisterCommandAspect(&multiAspect{name: "A", order: 1, calls: nil})

	names := chain.RegisteredNames()
	if len(names) != 1 {
		t.Errorf("expected 1 unique name, got %d: %v", len(names), names)
	}
}

func TestAspectChain_EventAspectAfterError(t *testing.T) {
	chain := NewAspectChain()
	chain.RegisterEventAspect(&eventAspectAfterError{})

	ctx := context.Background()
	err := chain.ExecuteWithEventAspects(ctx, &testEvent{BaseEvent: event.NewBaseEvent("1", time.Now())}, func(ctx context.Context) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error from AfterPublish")
	}
	if err.Error() != "after publish error" {
		t.Errorf("expected 'after publish error', got '%v'", err)
	}
}

type eventAspectAfterError struct{}

func (a *eventAspectAfterError) Name() string { return "error-after" }
func (a *eventAspectAfterError) Order() int   { return 1 }
func (a *eventAspectAfterError) BeforePublish(ctx context.Context, e any) (context.Context, error) {
	return ctx, nil
}
func (a *eventAspectAfterError) AfterPublish(ctx context.Context, e any, err error, d time.Duration) error {
	return errors.New("after publish error")
}

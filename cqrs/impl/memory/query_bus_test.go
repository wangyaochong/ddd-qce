package memory

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/query"
)

type testGetUserQuery struct {
	query.BaseQuery
	UserID string
}

type testGetUserResult struct {
	ID   string
	Name string
}

type testGetUserHandler struct{}

func (h *testGetUserHandler) Handle(ctx context.Context, q *testGetUserQuery) (*testGetUserResult, error) {
	return &testGetUserResult{ID: q.UserID, Name: "test"}, nil
}

type testListUsersQuery struct {
	query.BaseQuery
	Page int
}

type testListUsersResult struct {
	Count int
}

type testListUsersHandler struct{}

func (h *testListUsersHandler) Handle(ctx context.Context, q *testListUsersQuery) (*testListUsersResult, error) {
	return &testListUsersResult{Count: 10}, nil
}

type testErrorQuery struct {
	query.BaseQuery
}

type testErrorQueryResult struct{}

type testErrorQueryHandler struct{}

func (h *testErrorQueryHandler) Handle(ctx context.Context, q *testErrorQuery) (*testErrorQueryResult, error) {
	return nil, errors.New("query handler error")
}

type testSlowQuery struct {
	query.BaseQuery
	Duration time.Duration
}

type testSlowQueryResult struct {
	Done bool
}

type testSlowQueryHandler struct{}

func (h *testSlowQueryHandler) Handle(ctx context.Context, q *testSlowQuery) (*testSlowQueryResult, error) {
	select {
	case <-time.After(q.Duration):
		return &testSlowQueryResult{Done: true}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

type testSlowQueryHandlerWithNotify struct {
	started chan struct{}
}

func (h *testSlowQueryHandlerWithNotify) Handle(ctx context.Context, q *testSlowQuery) (*testSlowQueryResult, error) {
	close(h.started)
	select {
	case <-time.After(q.Duration):
		return &testSlowQueryResult{Done: true}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestQueryBus_Ask(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewQueryBus(WithQueryBusAspectChain(chain))
	RegisterQuery(bus, &testGetUserHandler{})

	ctx := context.Background()
	result, err := query.Dispatch[*testGetUserQuery, *testGetUserResult](ctx, bus, &testGetUserQuery{UserID: "123"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "123" {
		t.Errorf("expected ID '123', got '%s'", result.ID)
	}
}

func TestQueryBus_Ask_NoHandler(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewQueryBus(WithQueryBusAspectChain(chain))

	ctx := context.Background()
	_, err := query.Dispatch[*testGetUserQuery, *testGetUserResult](ctx, bus, &testGetUserQuery{UserID: "123"})

	if err == nil {
		t.Fatal("expected error for unregistered query type")
	}
}

func TestQueryBus_MultipleHandlers(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewQueryBus(WithQueryBusAspectChain(chain))
	RegisterQuery(bus, &testGetUserHandler{})
	RegisterQuery(bus, &testListUsersHandler{})

	ctx := context.Background()

	r1, err := query.Dispatch[*testGetUserQuery, *testGetUserResult](ctx, bus, &testGetUserQuery{UserID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r1.Name != "test" {
		t.Errorf("unexpected result: %v", r1)
	}

	r2, err := query.Dispatch[*testListUsersQuery, *testListUsersResult](ctx, bus, &testListUsersQuery{Page: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r2.Count != 10 {
		t.Errorf("expected count 10, got: %d", r2.Count)
	}
}

func TestQueryBus_HandlerError(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewQueryBus(WithQueryBusAspectChain(chain))
	RegisterQuery(bus, &testErrorQueryHandler{})

	ctx := context.Background()
	_, err := query.Dispatch[*testErrorQuery, *testErrorQueryResult](ctx, bus, &testErrorQuery{})

	if err == nil {
		t.Fatal("expected error from handler")
	}
	if err.Error() != "query handler error" {
		t.Errorf("expected 'query handler error', got '%v'", err)
	}
}

func TestQueryBus_Concurrent(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewQueryBus(WithQueryBusAspectChain(chain))
	RegisterQuery(bus, &testGetUserHandler{})

	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := query.Dispatch[*testGetUserQuery, *testGetUserResult](ctx, bus, &testGetUserQuery{UserID: string(rune(id))})
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

func TestQueryBus_NilChain(t *testing.T) {
	bus := NewQueryBus()
	RegisterQuery(bus, &testGetUserHandler{})

	ctx := context.Background()
	result, err := query.Dispatch[*testGetUserQuery, *testGetUserResult](ctx, bus, &testGetUserQuery{UserID: "123"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "123" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestQueryBus_WithAspects(t *testing.T) {
	chain := aspect.NewAspectChain()

	var beforeCalled, afterCalled bool
	testAspect := &testQueryAspect{
		beforeFn: func() { beforeCalled = true },
		afterFn:  func() { afterCalled = true },
	}
	chain.RegisterQueryAspect(testAspect)

	bus := NewQueryBus(WithQueryBusAspectChain(chain))
	RegisterQuery(bus, &testGetUserHandler{})

	ctx := context.Background()
	_, err := query.Dispatch[*testGetUserQuery, *testGetUserResult](ctx, bus, &testGetUserQuery{UserID: "123"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !beforeCalled {
		t.Error("BeforeQuery not called")
	}
	if !afterCalled {
		t.Error("AfterQuery not called")
	}
}

func TestQueryBus_DuplicateRegistration_ReturnsError(t *testing.T) {
	bus := NewQueryBus()
	if err := RegisterQuery(bus, &testGetUserHandler{}); err != nil {
		t.Fatalf("first registration should succeed: %v", err)
	}

	err := RegisterQuery(bus, &testGetUserHandler{})
	if err == nil {
		t.Fatal("expected error for duplicate registration")
	}
}

type testQueryAspect struct {
	beforeFn func()
	afterFn  func()
}

func (a *testQueryAspect) Name() string { return "test" }
func (a *testQueryAspect) Order() int   { return 1 }
func (a *testQueryAspect) BeforeQuery(ctx context.Context, query any) (context.Context, error) {
	if a.beforeFn != nil {
		a.beforeFn()
	}
	return ctx, nil
}
func (a *testQueryAspect) AfterQuery(ctx context.Context, query any, r any, err error, d time.Duration) error {
	if a.afterFn != nil {
		a.afterFn()
	}
	return nil
}

func TestQueryBus_RegisteredTypes(t *testing.T) {
	bus := NewQueryBus()
	RegisterQuery(bus, &testGetUserHandler{})
	RegisterQuery(bus, &testListUsersHandler{})

	types := bus.RegisteredTypes()
	if len(types) != 2 {
		t.Fatalf("expected 2 registered types, got %d", len(types))
	}

	nameSet := make(map[string]bool)
	for _, name := range types {
		nameSet[name] = true
	}
	if !nameSet["testGetUserQuery"] {
		t.Error("expected testGetUserQuery in registered types")
	}
	if !nameSet["testListUsersQuery"] {
		t.Error("expected testListUsersQuery in registered types")
	}
}

func TestQueryBus_RegisteredTypes_Empty(t *testing.T) {
	bus := NewQueryBus()
	types := bus.RegisteredTypes()
	if len(types) != 0 {
		t.Errorf("expected 0 types, got %d", len(types))
	}
}

func TestQueryBus_Shutdown(t *testing.T) {
	bus := NewQueryBus()
	RegisterQuery(bus, &testGetUserHandler{})

	err := bus.Shutdown(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = bus.Execute(context.Background(), &testGetUserQuery{UserID: "1"})
	if !errors.Is(err, ErrBusClosed) {
		t.Errorf("expected ErrBusClosed after shutdown, got %v", err)
	}
}

func TestQueryBus_Shutdown_WaitsForInFlight(t *testing.T) {
	bus := NewQueryBus()
	handlerStarted := make(chan struct{})
	RegisterQuery(bus, &testSlowQueryHandlerWithNotify{started: handlerStarted})

	done := make(chan struct{})
	go func() {
		_, _ = bus.Execute(context.Background(), &testSlowQuery{Duration: 100 * time.Millisecond})
		close(done)
	}()

	<-handlerStarted

	shutdownDone := make(chan error, 1)
	go func() {
		shutdownDone <- bus.Shutdown(context.Background())
	}()

	select {
	case <-shutdownDone:
		t.Fatal("shutdown should wait for in-flight query")
	case <-time.After(30 * time.Millisecond):
	}

	<-done

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("unexpected shutdown error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown timed out")
	}
}

func TestQueryBus_Shutdown_ContextCancelled(t *testing.T) {
	bus := NewQueryBus()
	handlerStarted := make(chan struct{})
	RegisterQuery(bus, &testSlowQueryHandlerWithNotify{started: handlerStarted})

	go func() {
		_, _ = bus.Execute(context.Background(), &testSlowQuery{Duration: 5 * time.Second})
	}()

	<-handlerStarted

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := bus.Shutdown(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}
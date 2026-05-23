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

func (h *testGetUserHandler) Handle(ctx context.Context, query *testGetUserQuery) (*testGetUserResult, error) {
	return &testGetUserResult{ID: query.UserID, Name: "test"}, nil
}

type testListUsersQuery struct {
	query.BaseQuery
	Page int
}

type testListUsersResult struct {
	Count int
}

type testListUsersHandler struct{}

func (h *testListUsersHandler) Handle(ctx context.Context, query *testListUsersQuery) (*testListUsersResult, error) {
	return &testListUsersResult{Count: 10}, nil
}

type testErrorQuery struct {
	query.BaseQuery
}

type testErrorQueryResult struct{}

type testErrorQueryHandler struct{}

func (h *testErrorQueryHandler) Handle(ctx context.Context, query *testErrorQuery) (*testErrorQueryResult, error) {
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

func (h *testSlowQueryHandler) Handle(ctx context.Context, query *testSlowQuery) (*testSlowQueryResult, error) {
	select {
	case <-time.After(query.Duration):
		return &testSlowQueryResult{Done: true}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestQueryBus_Ask(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewQueryBus(chain)
	RegisterQuery(bus, &testGetUserHandler{})

	ctx := context.Background()
	result, err := Ask[*testGetUserQuery, *testGetUserResult](bus, ctx, &testGetUserQuery{UserID: "123"})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != "123" {
		t.Errorf("expected ID '123', got '%s'", result.ID)
	}
}

func TestQueryBus_Ask_NoHandler(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewQueryBus(chain)

	ctx := context.Background()
	_, err := Ask[*testGetUserQuery, *testGetUserResult](bus, ctx, &testGetUserQuery{UserID: "123"})

	if err == nil {
		t.Fatal("expected error for unregistered query type")
	}
}

func TestQueryBus_MultipleHandlers(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewQueryBus(chain)
	RegisterQuery(bus, &testGetUserHandler{})
	RegisterQuery(bus, &testListUsersHandler{})

	ctx := context.Background()

	r1, err := Ask[*testGetUserQuery, *testGetUserResult](bus, ctx, &testGetUserQuery{UserID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r1.Name != "test" {
		t.Errorf("unexpected result: %v", r1)
	}

	r2, err := Ask[*testListUsersQuery, *testListUsersResult](bus, ctx, &testListUsersQuery{Page: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r2.Count != 10 {
		t.Errorf("expected count 10, got: %d", r2.Count)
	}
}

func TestQueryBus_HandlerError(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewQueryBus(chain)
	RegisterQuery(bus, &testErrorQueryHandler{})

	ctx := context.Background()
	_, err := Ask[*testErrorQuery, *testErrorQueryResult](bus, ctx, &testErrorQuery{})

	if err == nil {
		t.Fatal("expected error from handler")
	}
	if err.Error() != "query handler error" {
		t.Errorf("expected 'query handler error', got '%v'", err)
	}
}

func TestQueryBus_Concurrent(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := NewQueryBus(chain)
	RegisterQuery(bus, &testGetUserHandler{})

	ctx := context.Background()
	var wg sync.WaitGroup
	errs := make(chan error, 100)

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := Ask[*testGetUserQuery, *testGetUserResult](bus, ctx, &testGetUserQuery{UserID: string(rune(id))})
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
	bus := NewQueryBus(nil)
	RegisterQuery(bus, &testGetUserHandler{})

	ctx := context.Background()
	result, err := Ask[*testGetUserQuery, *testGetUserResult](bus, ctx, &testGetUserQuery{UserID: "123"})

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

	bus := NewQueryBus(chain)
	RegisterQuery(bus, &testGetUserHandler{})

	ctx := context.Background()
	_, err := Ask[*testGetUserQuery, *testGetUserResult](bus, ctx, &testGetUserQuery{UserID: "123"})

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

func TestQueryBus_DuplicateRegistration_Panics(t *testing.T) {
	bus := NewQueryBus(nil)
	RegisterQuery(bus, &testGetUserHandler{})

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic for duplicate registration")
		}
		msg := r.(string)
		expected := "handler already registered for query type: *memory.testGetUserQuery"
		if msg != expected {
			t.Errorf("expected panic message %q, got %q", expected, msg)
		}
	}()

	RegisterQuery(bus, &testGetUserHandler{})
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

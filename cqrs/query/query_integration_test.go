package query_test

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/query"
	querymemory "github.com/ddd-qce/core/cqrs/query/memory"
)

type integrationQuery struct {
	query.BaseQuery
	UserID string
}

type integrationQueryResult struct {
	ID   string
	Name string
}

type integrationQueryHandler struct {
	data map[string]*integrationQueryResult
}

func newIntegrationQueryHandler() *integrationQueryHandler {
	return &integrationQueryHandler{
		data: map[string]*integrationQueryResult{
			"1": {ID: "1", Name: "Alice"},
			"2": {ID: "2", Name: "Bob"},
		},
	}
}

func (h *integrationQueryHandler) Handle(ctx context.Context, q *integrationQuery) (*integrationQueryResult, error) {
	if result, ok := h.data[q.UserID]; ok {
		return result, nil
	}
	return nil, nil
}

type integrationFailingQuery struct {
	query.BaseQuery
}

type integrationFailingQueryResult struct{}

type integrationFailingQueryHandler struct{}

func (h *integrationFailingQueryHandler) Handle(ctx context.Context, q *integrationFailingQuery) (*integrationFailingQueryResult, error) {
	return nil, context.DeadlineExceeded
}

type integrationListQuery struct {
	query.BaseQuery
	Page int
}

type integrationListResult struct {
	Total int
}

type integrationListQueryHandler struct{}

func (h *integrationListQueryHandler) Handle(ctx context.Context, q *integrationListQuery) (*integrationListResult, error) {
	return &integrationListResult{Total: 42}, nil
}

func TestAsk_GenericAPI(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
	querymemory.RegisterQuery(bus, newIntegrationQueryHandler())

	result, err := querymemory.Dispatch[*integrationQuery, *integrationQueryResult](context.Background(), bus, &integrationQuery{UserID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "Alice" {
		t.Errorf("expected 'Alice', got '%s'", result.Name)
	}
}

func TestAsk_GenericAPI_NoHandler(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))

	_, err := querymemory.Dispatch[*integrationQuery, *integrationQueryResult](context.Background(), bus, &integrationQuery{UserID: "1"})
	if err == nil {
		t.Fatal("expected error for unregistered query")
	}
}

func TestAsk_GenericAPI_NotFound(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
	querymemory.RegisterQuery(bus, newIntegrationQueryHandler())

	result, err := querymemory.Dispatch[*integrationQuery, *integrationQueryResult](context.Background(), bus, &integrationQuery{UserID: "999"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for non-existent ID, got %v", result)
	}
}

func TestAsk_GenericAPI_Error(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
	querymemory.RegisterQuery(bus, &integrationFailingQueryHandler{})

	_, err := querymemory.Dispatch[*integrationFailingQuery, *integrationFailingQueryResult](context.Background(), bus, &integrationFailingQuery{})
	if err == nil {
		t.Fatal("expected error from failing handler")
	}
}

func TestAsk_GenericAPI_MultipleHandlers(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
	querymemory.RegisterQuery(bus, newIntegrationQueryHandler())
	querymemory.RegisterQuery(bus, &integrationListQueryHandler{})

	r1, err := querymemory.Dispatch[*integrationQuery, *integrationQueryResult](context.Background(), bus, &integrationQuery{UserID: "2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r1.Name != "Bob" {
		t.Errorf("expected 'Bob', got '%s'", r1.Name)
	}

	r2, err := querymemory.Dispatch[*integrationListQuery, *integrationListResult](context.Background(), bus, &integrationListQuery{Page: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r2.Total != 42 {
		t.Errorf("expected 42, got %d", r2.Total)
	}
}

func TestAsk_GenericAPI_WithAspects(t *testing.T) {
	chain := aspect.NewAspectChain()

	var beforeCalled, afterCalled bool
	chain.RegisterQueryAspect(&testIntegrationQueryAspect{
		beforeFn: func() { beforeCalled = true },
		afterFn:  func() { afterCalled = true },
	})

	bus := querymemory.NewQueryBus(querymemory.WithQueryBusAspectChain(chain))
	querymemory.RegisterQuery(bus, newIntegrationQueryHandler())

	_, err := querymemory.Dispatch[*integrationQuery, *integrationQueryResult](context.Background(), bus, &integrationQuery{UserID: "1"})
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

func TestQueryNameOf_ViaInterface(t *testing.T) {
	q := &integrationQuery{UserID: "1"}
	name := query.QueryNameOf(q)
	if name != "integrationQuery" {
		t.Errorf("expected 'integrationQuery', got '%s'", name)
	}
}

type testIntegrationQueryAspect struct {
	beforeFn func()
	afterFn  func()
}

func (a *testIntegrationQueryAspect) Name() string { return "test-integration" }
func (a *testIntegrationQueryAspect) Order() int   { return 1 }
func (a *testIntegrationQueryAspect) BeforeQuery(ctx context.Context, q any) (context.Context, error) {
	if a.beforeFn != nil {
		a.beforeFn()
	}
	return ctx, nil
}
func (a *testIntegrationQueryAspect) AfterQuery(ctx context.Context, q any, r any, err error, d time.Duration) error {
	if a.afterFn != nil {
		a.afterFn()
	}
	return nil
}

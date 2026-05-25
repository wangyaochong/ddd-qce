package impl

import (
	"context"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect"
	"github.com/ddd-qce/core/cqrs/query"
	"github.com/ddd-qce/core/cqrs/impl/memory"
)

type integrationQuery struct {
	query.BaseQuery
	UserID string
}

type integrationQueryResult struct {
	Name string
}

type integrationQueryHandler struct{}

func (h *integrationQueryHandler) Handle(ctx context.Context, q *integrationQuery) (*integrationQueryResult, error) {
	return &integrationQueryResult{Name: "int-" + q.UserID}, nil
}

func TestQueryBus_InterfaceSatisfaction(t *testing.T) {
	chain := aspect.NewAspectChain()
	var _ query.QueryBus = memory.NewQueryBus(memory.WithQueryBusAspectChain(chain))
}

func TestQueryBus_Ask(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := memory.NewQueryBus(memory.WithQueryBusAspectChain(chain))
	memory.RegisterQuery(bus, &integrationQueryHandler{})

	var executor query.QueryBus = bus

	result, err := executor.Execute(context.Background(), &integrationQuery{UserID: "123"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	r, ok := result.(*integrationQueryResult)
	if !ok {
		t.Fatalf("expected *integrationQueryResult, got %T", result)
	}
	if r.Name != "int-123" {
		t.Errorf("expected 'int-123', got '%s'", r.Name)
	}
}

func TestQueryBus_Ask_NoHandler(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := memory.NewQueryBus(memory.WithQueryBusAspectChain(chain))

	var executor query.QueryBus = bus

	_, err := executor.Execute(context.Background(), &integrationQuery{UserID: "no-handler"})
	if err == nil {
		t.Fatal("expected error for unregistered query")
	}
}

func TestQueryDispatch_GenericAPI(t *testing.T) {
	chain := aspect.NewAspectChain()
	bus := memory.NewQueryBus(memory.WithQueryBusAspectChain(chain))
	memory.RegisterQuery(bus, &integrationQueryHandler{})

	result, err := query.Dispatch[*integrationQuery, *integrationQueryResult](context.Background(), bus, &integrationQuery{UserID: "dispatch"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "int-dispatch" {
		t.Errorf("expected 'int-dispatch', got '%s'", result.Name)
	}
}

func TestQueryBus_Execute_WithAspects(t *testing.T) {
	chain := aspect.NewAspectChain()

	var beforeCalled, afterCalled bool
	chain.RegisterQueryAspect(&testQueryIntegrationAspect{
		beforeFn: func() { beforeCalled = true },
		afterFn:  func() { afterCalled = true },
	})

	bus := memory.NewQueryBus(memory.WithQueryBusAspectChain(chain))
	memory.RegisterQuery(bus, &integrationQueryHandler{})

	var executor query.QueryBus = bus

	_, err := executor.Execute(context.Background(), &integrationQuery{UserID: "aspected"})
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

type testQueryIntegrationAspect struct {
	beforeFn func()
	afterFn  func()
}

func (a *testQueryIntegrationAspect) Name() string { return "test-integration" }
func (a *testQueryIntegrationAspect) Order() int   { return 1 }
func (a *testQueryIntegrationAspect) BeforeQuery(ctx context.Context, query any) (context.Context, error) {
	if a.beforeFn != nil {
		a.beforeFn()
	}
	return ctx, nil
}
func (a *testQueryIntegrationAspect) AfterQuery(ctx context.Context, query any, r any, err error, d time.Duration) error {
	if a.afterFn != nil {
		a.afterFn()
	}
	return nil
}
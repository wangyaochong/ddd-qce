package impl

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ddd-qce/core/cqrs/query"
)

type testQuery struct {
	query.BaseQuery
	UserID string
}

type testQueryResult struct {
	ID   string
	Name string
}

type testQueryHandler struct {
	data map[string]*testQueryResult
}

func newTestQueryHandler() *testQueryHandler {
	return &testQueryHandler{
		data: map[string]*testQueryResult{
			"1": {ID: "1", Name: "Alice"},
			"2": {ID: "2", Name: "Bob"},
		},
	}
}

func (h *testQueryHandler) Handle(ctx context.Context, q *testQuery) (*testQueryResult, error) {
	if result, ok := h.data[q.UserID]; ok {
		return result, nil
	}
	return nil, nil
}

type testFailingQuery struct {
	query.BaseQuery
}

type testFailingQueryResult struct{}

type testFailingQueryHandler struct{}

func (h *testFailingQueryHandler) Handle(ctx context.Context, q *testFailingQuery) (*testFailingQueryResult, error) {
	return nil, errors.New("query failed")
}

type testQueryWithoutBase struct {
	query.BaseQuery
	Key string
}

type testListQuery struct {
	query.BaseQuery
	Page int
}

type testListResult struct {
	Total int
}

type testListQueryHandler struct{}

func (h *testListQueryHandler) Handle(ctx context.Context, q *testListQuery) (*testListResult, error) {
	return &testListResult{Total: 42}, nil
}

func TestBaseQuery_ImplementsQuery(t *testing.T) {
	var _ query.Query = (*testQuery)(nil)
	var _ query.Query = (*testFailingQuery)(nil)
	var _ query.Query = (*testQueryWithoutBase)(nil)
	var _ query.Query = query.BaseQuery{}
}

func TestQueryNameOf_PointerType(t *testing.T) {
	q := &testQuery{UserID: "1"}
	name := query.QueryNameOf(q)
	if name != "testQuery" {
		t.Errorf("expected 'testQuery', got '%s'", name)
	}
}

func TestQueryNameOf_ValueType(t *testing.T) {
	q := testQuery{UserID: "1"}
	name := query.QueryNameOf(q)
	if name != "testQuery" {
		t.Errorf("expected 'testQuery', got '%s'", name)
	}
}

func TestQueryNameOf_DifferentTypes(t *testing.T) {
	tests := []struct {
		query any
		want  string
	}{
		{&testQuery{}, "testQuery"},
		{&testFailingQuery{}, "testFailingQuery"},
		{&testQueryWithoutBase{}, "testQueryWithoutBase"},
		{testQuery{}, "testQuery"},
		{query.BaseQuery{}, "BaseQuery"},
	}
	for _, tt := range tests {
		got := query.QueryNameOf(tt.query)
		if got != tt.want {
			t.Errorf("QueryNameOf(%T) = '%s', want '%s'", tt.query, got, tt.want)
		}
	}
}

func TestQueryHandler_Handle(t *testing.T) {
	ctx := context.Background()
	handler := newTestQueryHandler()

	result, err := handler.Handle(ctx, &testQuery{UserID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Name != "Alice" {
		t.Errorf("expected 'Alice', got '%s'", result.Name)
	}
}

func TestQueryHandler_Handle_NotFound(t *testing.T) {
	ctx := context.Background()
	handler := newTestQueryHandler()

	result, err := handler.Handle(ctx, &testQuery{UserID: "999"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil for non-existent ID, got %v", result)
	}
}

func TestQueryHandler_Error(t *testing.T) {
	ctx := context.Background()
	handler := &testFailingQueryHandler{}

	_, err := handler.Handle(ctx, &testFailingQuery{})
	if err == nil {
		t.Fatal("expected error from failing handler")
	}
	if err.Error() != "query failed" {
		t.Errorf("expected 'query failed', got '%v'", err)
	}
}

func TestQueryHandler_InterfaceSatisfaction(t *testing.T) {
	var _ query.QueryHandler[*testQuery, *testQueryResult] = &testQueryHandler{}
	var _ query.QueryHandler[*testFailingQuery, *testFailingQueryResult] = &testFailingQueryHandler{}
	var _ query.QueryHandler[*testListQuery, *testListResult] = &testListQueryHandler{}
}

func TestQuery_InterfaceEnforcement(t *testing.T) {
	q := &testQuery{UserID: "1"}
	var _ query.Query = q

	if _, ok := any(q).(query.Query); !ok {
		t.Error("testQuery should implement Query interface")
	}

	plain := &struct{ Key string }{Key: "not-a-query"}
	if _, ok := any(plain).(query.Query); ok {
		t.Error("plain struct should NOT implement Query interface")
	}
}

func TestQueryNameOf_UnexportedType(t *testing.T) {
	type internalQuery struct {
		query.BaseQuery
		Filter string
	}
	q := &internalQuery{Filter: "test"}
	name := query.QueryNameOf(q)
	if name != "internalQuery" {
		t.Errorf("expected 'internalQuery', got '%s'", name)
	}
}

func TestBaseQuery_MarkerMethod(t *testing.T) {
	var q query.Query = query.BaseQuery{}
	_ = q
}

func TestQueryNameOf_ReflectTypeConsistency(t *testing.T) {
	q := &testQuery{UserID: "1"}
	ptrName := query.QueryNameOf(q)

	var zero *testQuery
	typeName := reflect.TypeOf(zero).Elem().Name()

	if ptrName != typeName {
		t.Errorf("QueryNameOf pointer '%s' doesn't match reflect type name '%s'", ptrName, typeName)
	}
}

func TestQueryHandler_ContextPropagation(t *testing.T) {
	ctx := context.WithValue(context.Background(), "testKey", "testValue")
	handler := newTestQueryHandler()

	result, err := handler.Handle(ctx, &testQuery{UserID: "1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Name != "Alice" {
		t.Errorf("expected 'Alice', got '%s'", result.Name)
	}
}

func TestQuery_WithCustomIsQuery(t *testing.T) {
	type customQuery struct{ id int }
	if _, ok := any(&customQuery{}).(query.Query); ok {
		t.Error("type without isQuery() should NOT satisfy Query")
	}
}

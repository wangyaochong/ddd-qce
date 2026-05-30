package query

import (
	"context"
	"errors"
	"testing"
)

type testQuery struct {
	BaseQuery
	Filter string
}

func (testQuery) isQuery() {}

type testQueryResult struct {
	Data string
}

type testQueryHandler struct {
	result string
	err    error
}

func (h *testQueryHandler) Handle(ctx context.Context, q testQuery) (testQueryResult, error) {
	if h.err != nil {
		return testQueryResult{}, h.err
	}
	return testQueryResult{Data: h.result}, nil
}

type testQueryBus struct {
	handlers   map[string]any
	executeErr error
	executeRet any
}

func (b *testQueryBus) Execute(ctx context.Context, q any) (any, error) {
	if b.executeErr != nil {
		return nil, b.executeErr
	}
	return b.executeRet, nil
}

func (b *testQueryBus) RegisterHandler(handler any) error {
	name := QueryNameOf(handler)
	b.handlers[name] = handler
	return nil
}

func (b *testQueryBus) RegisteredTypes() []string {
	names := make([]string, 0, len(b.handlers))
	for k := range b.handlers {
		names = append(names, k)
	}
	return names
}

func (b *testQueryBus) Shutdown(ctx context.Context) error {
	return nil
}

func TestQueryNameOf(t *testing.T) {
	tests := []struct {
		name     string
		query    any
		expected string
	}{
		{"struct type", testQuery{}, "testQuery"},
		{"pointer type", &testQuery{}, "testQuery"},
		{"base query", BaseQuery{}, "BaseQuery"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := QueryNameOf(tt.query)
			if result != tt.expected {
				t.Errorf("QueryNameOf(%T) = %q, want %q", tt.query, result, tt.expected)
			}
		})
	}
}

func TestQueryNameOf_NilPanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Error("QueryNameOf(nil) should panic")
		}
		if msg, ok := r.(string); !ok || msg != "query: QueryNameOf called with nil query" {
			t.Errorf("unexpected panic message: %v", r)
		}
	}()
	QueryNameOf(nil)
}

func TestDispatch_Success(t *testing.T) {
	bus := &testQueryBus{
		handlers:   make(map[string]any),
		executeRet: testQueryResult{Data: "success"},
	}

	result, err := Dispatch[testQuery, testQueryResult](context.Background(), bus, testQuery{Filter: "test"})
	if err != nil {
		t.Errorf("Dispatch() error = %v, want nil", err)
	}
	if result.Data != "success" {
		t.Errorf("Dispatch() = %v, want testQueryResult{Data: \"success\"}", result)
	}
}

func TestDispatch_ExecuteError(t *testing.T) {
	bus := &testQueryBus{
		handlers:   make(map[string]any),
		executeErr: errors.New("execute failed"),
	}

	result, err := Dispatch[testQuery, testQueryResult](context.Background(), bus, testQuery{})
	if err == nil {
		t.Error("Dispatch() should return error from Execute")
	}
	if result != (testQueryResult{}) {
		t.Error("Dispatch() should return zero value on error")
	}
}

func TestDispatch_TypeMismatch(t *testing.T) {
	bus := &testQueryBus{
		handlers:   make(map[string]any),
		executeRet: "wrong type",
	}

	result, err := Dispatch[testQuery, testQueryResult](context.Background(), bus, testQuery{})
	if err == nil {
		t.Error("Dispatch() should return error on type mismatch")
	}
	if result != (testQueryResult{}) {
		t.Error("Dispatch() should return zero value on error")
	}
}

func TestDispatch_NilResult(t *testing.T) {
	bus := &testQueryBus{
		handlers:   make(map[string]any),
		executeRet: nil,
	}

	result, err := Dispatch[testQuery, testQueryResult](context.Background(), bus, testQuery{})
	if err != nil {
		t.Errorf("Dispatch() error = %v, want nil", err)
	}
	if result != (testQueryResult{}) {
		t.Error("Dispatch() should return zero value for nil result")
	}
}

func TestTypeAssert_NilResult(t *testing.T) {
	result, err := typeAssert[testQueryResult](nil, testQuery{})
	if err != nil {
		t.Errorf("typeAssert() error = %v, want nil", err)
	}
	if result != (testQueryResult{}) {
		t.Error("typeAssert() should return zero value for nil result")
	}
}

func TestTypeAssert_TypeMismatch(t *testing.T) {
	result, err := typeAssert[testQueryResult]("wrong type", testQuery{})
	if err == nil {
		t.Error("typeAssert() should return error on type mismatch")
	}
	if result != (testQueryResult{}) {
		t.Error("typeAssert() should return zero value on error")
	}
}

func TestTypeAssert_Success(t *testing.T) {
	result, err := typeAssert[testQueryResult](testQueryResult{Data: "ok"}, testQuery{})
	if err != nil {
		t.Errorf("typeAssert() error = %v, want nil", err)
	}
	if result.Data != "ok" {
		t.Errorf("typeAssert() = %v, want testQueryResult{Data: \"ok\"}", result)
	}
}
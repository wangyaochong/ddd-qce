package query

import (
	"context"
	"fmt"
	"reflect"
)

type Query interface {
	isQuery()
}

type BaseQuery struct{}

func (BaseQuery) isQuery() {}

// QueryNameOf returns the short type name of the given query.
// Panics if q is nil — nil is a programming error that should be
// caught early rather than silently producing an empty string that
// could be used as a map key or persisted to storage.
// Note: a typed nil pointer like (*MyQuery)(nil) is NOT nil (the
// interface has a type), so QueryNameOf((*MyQuery)(nil)) == "MyQuery".
func QueryNameOf(q any) string {
	if q == nil {
		panic("query: QueryNameOf called with nil query")
	}
	t := reflect.TypeOf(q)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

type QueryHandler[T Query, R any] interface {
	Handle(ctx context.Context, query T) (R, error)
}

type QueryBus interface {
	Execute(ctx context.Context, query any) (any, error)
	RegisterHandler(handler any) error
	RegisteredTypes() []string
}

type Shutdownable interface {
	Shutdown(ctx context.Context) error
}

func Dispatch[T Query, R any](ctx context.Context, bus QueryBus, q T) (R, error) {
	result, err := bus.Execute(ctx, q)
	if err != nil {
		var zero R
		return zero, err
	}
	typed, err := typeAssert[R](result, q)
	if err != nil {
		var zero R
		return zero, err
	}
	return typed, nil
}

func typeAssert[R any](result any, q any) (R, error) {
	if result == nil {
		var zero R
		return zero, nil
	}
	typed, ok := result.(R)
	if !ok {
		var zero R
		return zero, fmt.Errorf("query Dispatch: result type mismatch for query %T, got %T", q, result)
	}
	return typed, nil
}
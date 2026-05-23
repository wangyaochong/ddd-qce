package query

import (
	"context"
	"reflect"
)

type Query interface {
	isQuery()
}

type BaseQuery struct{}

func (BaseQuery) isQuery() {}

func QueryNameOf(q any) string {
	t := reflect.TypeOf(q)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

type QueryHandler[T Query, R any] interface {
	Handle(ctx context.Context, query T) (R, error)
}

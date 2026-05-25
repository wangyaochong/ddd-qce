package cmd

import (
	"context"
	"fmt"
	"reflect"
)

type Command interface {
	isCommand()
}

type BaseCommand struct{}

func (BaseCommand) isCommand() {}

func CommandNameOf(cmd any) string {
	t := reflect.TypeOf(cmd)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

type CommandHandler[T Command, R any] interface {
	Handle(ctx context.Context, cmd T) (R, error)
}

type CommandBus interface {
	Execute(ctx context.Context, cmd any) (any, error)
	RegisterHandler(handler any) error
}

func Dispatch[T Command, R any](ctx context.Context, bus CommandBus, cmd T) (R, error) {
	result, err := bus.Execute(ctx, cmd)
	if err != nil {
		var zero R
		return zero, err
	}
	typed, err := typeAssert[R](result, cmd)
	if err != nil {
		var zero R
		return zero, err
	}
	return typed, nil
}

func typeAssert[R any](result any, cmd any) (R, error) {
	if result == nil {
		var zero R
		return zero, nil
	}
	typed, ok := result.(R)
	if !ok {
		var zero R
		return zero, fmt.Errorf("command Dispatch: result type mismatch for command %T, got %T", cmd, result)
	}
	return typed, nil
}
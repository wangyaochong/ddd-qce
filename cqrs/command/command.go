package command

import (
	"context"
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

type CommandExecutor interface {
	Execute(ctx context.Context, cmd any) (any, error)
}

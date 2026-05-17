package builtin

import (
	"context"
	"fmt"
	"time"
)

type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

type LoggingAspect struct {
	Logger Logger
}

func (l *LoggingAspect) Name() string {
	return "logging"
}

func (l *LoggingAspect) Order() int {
	return 50
}

func (l *LoggingAspect) BeforeQuery(ctx context.Context, query any) (context.Context, error) {
	l.Logger.Debug("BeforeQuery", "type", fmt.Sprintf("%T", query))
	return ctx, nil
}

func (l *LoggingAspect) AfterQuery(ctx context.Context, query any, result any, err error, duration time.Duration) error {
	if err != nil {
		l.Logger.Error("AfterQuery failed", "type", fmt.Sprintf("%T", query), "error", err, "duration", duration)
	} else {
		l.Logger.Debug("AfterQuery", "type", fmt.Sprintf("%T", query), "duration", duration)
	}
	return nil
}

func (l *LoggingAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	l.Logger.Debug("BeforeCommand", "type", fmt.Sprintf("%T", cmd))
	return ctx, nil
}

func (l *LoggingAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error {
	if err != nil {
		l.Logger.Error("AfterCommand failed", "type", fmt.Sprintf("%T", cmd), "error", err, "duration", duration)
	} else {
		l.Logger.Debug("AfterCommand", "type", fmt.Sprintf("%T", cmd), "duration", duration)
	}
	return nil
}

func (l *LoggingAspect) BeforePublish(ctx context.Context, event any) (context.Context, error) {
	l.Logger.Debug("BeforePublish", "event", fmt.Sprintf("%T", event))
	return ctx, nil
}

func (l *LoggingAspect) AfterPublish(ctx context.Context, event any, err error, duration time.Duration) error {
	if err != nil {
		l.Logger.Error("AfterPublish failed", "event", fmt.Sprintf("%T", event), "error", err, "duration", duration)
	} else {
		l.Logger.Debug("AfterPublish", "event", fmt.Sprintf("%T", event), "duration", duration)
	}
	return nil
}

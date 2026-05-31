package builtin

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/ddd-qce/core/trace"
)

type StdLogger struct {
	logger *slog.Logger
}

func NewStdLogger() *StdLogger {
	return &StdLogger{
		logger: slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: slog.LevelDebug,
		})),
	}
}

func NewStdLoggerWithOptions(opts *slog.HandlerOptions) *StdLogger {
	return &StdLogger{
		logger: slog.New(slog.NewTextHandler(os.Stdout, opts)),
	}
}

func (l *StdLogger) Info(msg string, args ...interface{}) {
	l.logger.Info(msg, args...)
}

func (l *StdLogger) Error(msg string, args ...interface{}) {
	l.logger.Error(msg, args...)
}

func (l *StdLogger) Debug(msg string, args ...interface{}) {
	l.logger.Debug(msg, args...)
}

var _ Logger = (*StdLogger)(nil)

type ContextKey string

const (
	LogKeyAggregateID ContextKey = "aggregate_id"
	LogKeyCommand     ContextKey = "command"
	LogKeyQuery       ContextKey = "query"
	LogKeyEvent       ContextKey = "event"
	LogKeyDuration    ContextKey = "duration_ms"
	LogKeyError       ContextKey = "error"
)

func (l *StdLogger) WithContext(ctx context.Context) *slog.Logger {
	logger := l.logger

	if traceID := trace.GetTraceID(ctx); traceID != "" {
		logger = logger.With("trace_id", traceID)
	}
	if spanID := trace.GetSpanID(ctx); spanID != "" {
		logger = logger.With("span_id", spanID)
	}

	return logger
}

func (l *StdLogger) LogCommand(ctx context.Context, name string, duration time.Duration, err error) {
	logger := l.WithContext(ctx).With("command", name, "duration_ms", duration.Milliseconds())
	if err != nil {
		logger.Error("command failed", "error", err.Error())
	} else {
		logger.Info("command executed")
	}
}

func (l *StdLogger) LogQuery(ctx context.Context, name string, duration time.Duration, err error) {
	logger := l.WithContext(ctx).With("query", name, "duration_ms", duration.Milliseconds())
	if err != nil {
		logger.Error("query failed", "error", err.Error())
	} else {
		logger.Info("query executed")
	}
}

func (l *StdLogger) LogEvent(ctx context.Context, name string, err error) {
	logger := l.WithContext(ctx).With("event", name)
	if err != nil {
		logger.Error("event publish failed", "error", err.Error())
	} else {
		logger.Info("event published")
	}
}

package builtin

import (
	"context"
	"reflect"
	"time"

	ddderror "github.com/ddd-qce/core/error"
	"github.com/ddd-qce/core/trace"
)

// TracingAspect creates distributed-tracing spans for commands, queries, and
// events, recording span metadata (trace ID, span ID, duration, status) into a
// trace store.
type TracingAspect struct {
	store  trace.TraceStore
	logger Logger
}

// NewTracingAspect creates a TracingAspect with the given trace store.
// The store may be nil for a no-op tracing mode.
func NewTracingAspect(store trace.TraceStore) *TracingAspect {
	return &TracingAspect{store: store}
}

// GetStore returns the underlying trace store used to persist span data.
func (a *TracingAspect) GetStore() trace.TraceStore { return a.store }

// GetLogger returns the logger used for tracing error output, or nil if unset.
func (a *TracingAspect) GetLogger() Logger { return a.logger }

// SetLogger sets the logger for tracing error output.
func (a *TracingAspect) SetLogger(logger Logger) { a.logger = logger }

// Name returns the aspect identifier "tracing".
func (a *TracingAspect) Name() string {
	return "tracing"
}

// Order returns 0, placing TracingAspect first in the aspect chain
// so that it captures the full duration of the operation.
func (a *TracingAspect) Order() int {
	return 0
}

// BeforeCommand creates a new command span, propagating or generating trace and
// span IDs in the context.
func (a *TracingAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	traceID := trace.GetTraceID(ctx)
	parentSpanID := trace.GetSpanID(ctx)

	if traceID == "" {
		traceID = trace.NewTraceID()
	}

	spanID := trace.NewSpanID()
	spanName := typeName(cmd)

	newCtx := trace.WithTrace(ctx, traceID, spanID)
	newCtx = trace.WithParentSpan(newCtx, parentSpanID)

	span := &trace.Span{
		ID:        spanID,
		TraceID:   traceID,
		ParentID:  parentSpanID,
		Type:      trace.SpanTypeCommand,
		Name:      spanName,
		StartedAt: time.Now(),
	}

	return context.WithValue(newCtx, spanKey{}, span), nil
}

// AfterCommand finalizes the command span with duration, status, and error info,
// then persists it to the trace store.
func (a *TracingAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error {
	if span, ok := ctx.Value(spanKey{}).(*trace.Span); ok {
		span.Duration = duration
		if err != nil {
			if ddderror.IsDomainError(err) {
				span.Status = trace.SpanStatusBusinessError
			} else {
				span.Status = trace.SpanStatusError
			}
			span.Error = err.Error()
		} else {
			span.Status = trace.SpanStatusSuccess
		}
		if a.store != nil {
			if err := a.store.RecordSpan(ctx, span); err != nil {
				if a.logger != nil {
					a.logger.Error("TracingAspect RecordSpan failed", "error", err)
				}
			}
		}
	}
	return nil
}

// BeforeQuery creates a new query span, propagating or generating trace and
// span IDs in the context.
func (a *TracingAspect) BeforeQuery(ctx context.Context, query any) (context.Context, error) {
	traceID := trace.GetTraceID(ctx)
	parentSpanID := trace.GetSpanID(ctx)

	if traceID == "" {
		traceID = trace.NewTraceID()
	}

	spanID := trace.NewSpanID()
	spanName := typeName(query)

	newCtx := trace.WithTrace(ctx, traceID, spanID)
	newCtx = trace.WithParentSpan(newCtx, parentSpanID)

	span := &trace.Span{
		ID:        spanID,
		TraceID:   traceID,
		ParentID:  parentSpanID,
		Type:      trace.SpanTypeQuery,
		Name:      spanName,
		StartedAt: time.Now(),
	}

	return context.WithValue(newCtx, spanKey{}, span), nil
}

// AfterQuery finalizes the query span with duration, status, and error info,
// then persists it to the trace store.
func (a *TracingAspect) AfterQuery(ctx context.Context, query any, result any, err error, duration time.Duration) error {
	if span, ok := ctx.Value(spanKey{}).(*trace.Span); ok {
		span.Duration = duration
		if err != nil {
			if ddderror.IsDomainError(err) {
				span.Status = trace.SpanStatusBusinessError
			} else {
				span.Status = trace.SpanStatusError
			}
			span.Error = err.Error()
		} else {
			span.Status = trace.SpanStatusSuccess
		}
		if a.store != nil {
			if err := a.store.RecordSpan(ctx, span); err != nil {
				if a.logger != nil {
					a.logger.Error("TracingAspect RecordSpan failed", "error", err)
				}
			}
		}
	}
	return nil
}

// BeforePublish creates a new event span, propagating or generating trace and
// span IDs in the context.
func (a *TracingAspect) BeforePublish(ctx context.Context, event any) (context.Context, error) {
	traceID := trace.GetTraceID(ctx)
	parentSpanID := trace.GetSpanID(ctx)

	if traceID == "" {
		traceID = trace.NewTraceID()
	}

	spanID := trace.NewSpanID()
	spanName := typeName(event)

	newCtx := trace.WithTrace(ctx, traceID, spanID)
	newCtx = trace.WithParentSpan(newCtx, parentSpanID)

	span := &trace.Span{
		ID:        spanID,
		TraceID:   traceID,
		ParentID:  parentSpanID,
		Type:      trace.SpanTypeEvent,
		Name:      spanName,
		StartedAt: time.Now(),
	}

	return context.WithValue(newCtx, spanKey{}, span), nil
}

// AfterPublish finalizes the event span with duration, status, and error info,
// then persists it to the trace store.
func (a *TracingAspect) AfterPublish(ctx context.Context, event any, err error, duration time.Duration) error {
	if span, ok := ctx.Value(spanKey{}).(*trace.Span); ok {
		span.Duration = duration
		if err != nil {
			if ddderror.IsDomainError(err) {
				span.Status = trace.SpanStatusBusinessError
			} else {
				span.Status = trace.SpanStatusError
			}
			span.Error = err.Error()
		} else {
			span.Status = trace.SpanStatusSuccess
		}
		if a.store != nil {
			if err := a.store.RecordSpan(ctx, span); err != nil {
				if a.logger != nil {
					a.logger.Error("TracingAspect RecordSpan failed", "error", err)
				}
			}
		}
	}
	return nil
}

func typeName(v any) string {
	t := reflect.TypeOf(v)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

type spanKey struct{}

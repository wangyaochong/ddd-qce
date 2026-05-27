package builtin

import (
	"context"
	"reflect"
	"time"

	ddderror "github.com/ddd-qce/core/error"
	"github.com/ddd-qce/core/trace"
)

type TracingAspect struct {
	store  trace.TraceStore
	logger Logger
}

func NewTracingAspect(store trace.TraceStore) *TracingAspect {
	return &TracingAspect{store: store}
}

func (a *TracingAspect) GetStore() trace.TraceStore  { return a.store }
func (a *TracingAspect) GetLogger() Logger            { return a.logger }
func (a *TracingAspect) SetLogger(logger Logger)       { a.logger = logger }

func (a *TracingAspect) Name() string {
	return "tracing"
}

func (a *TracingAspect) Order() int {
	return 0
}

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

func (a *TracingAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error {
	if span, ok := ctx.Value(spanKey{}).(*trace.Span); ok {
		span.Duration = duration
		if err != nil {
			if ddderror.IsDomainError(err) {
				span.Status = "business_error"
			} else {
				span.Status = trace.SpanStatusError
			}
			span.Error = err.Error()
		} else {
			span.Status = trace.SpanStatusSuccess
		}
		if err := a.store.RecordSpan(ctx, span); err != nil {
			if a.logger != nil {
				a.logger.Error("TracingAspect RecordSpan failed", "error", err)
			}
		}
	}
	return nil
}

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

func (a *TracingAspect) AfterQuery(ctx context.Context, query any, result any, err error, duration time.Duration) error {
	if span, ok := ctx.Value(spanKey{}).(*trace.Span); ok {
		span.Duration = duration
		if err != nil {
			if ddderror.IsDomainError(err) {
				span.Status = "business_error"
			} else {
				span.Status = trace.SpanStatusError
			}
			span.Error = err.Error()
		} else {
			span.Status = trace.SpanStatusSuccess
		}
		if err := a.store.RecordSpan(ctx, span); err != nil {
			if a.logger != nil {
				a.logger.Error("TracingAspect RecordSpan failed", "error", err)
			}
		}
	}
	return nil
}

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

func (a *TracingAspect) AfterPublish(ctx context.Context, event any, err error, duration time.Duration) error {
	if span, ok := ctx.Value(spanKey{}).(*trace.Span); ok {
		span.Duration = duration
		if err != nil {
			if ddderror.IsDomainError(err) {
				span.Status = "business_error"
			} else {
				span.Status = trace.SpanStatusError
			}
			span.Error = err.Error()
		} else {
			span.Status = trace.SpanStatusSuccess
		}
		if err := a.store.RecordSpan(ctx, span); err != nil {
			if a.logger != nil {
				a.logger.Error("TracingAspect RecordSpan failed", "error", err)
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

package trace

import (
	"context"

	"github.com/google/uuid"
)

type traceContextKey struct{}
type spanContextKey struct{}

type traceContext struct {
	TraceID string
	SpanID  string
}

func NewTraceID() string {
	return uuid.New().String()
}

func NewSpanID() string {
	return uuid.New().String()
}

func WithTrace(ctx context.Context, traceID, spanID string) context.Context {
	return context.WithValue(ctx, traceContextKey{}, traceContext{
		TraceID: traceID,
		SpanID:  spanID,
	})
}

func GetTraceID(ctx context.Context) string {
	if tc, ok := ctx.Value(traceContextKey{}).(traceContext); ok {
		return tc.TraceID
	}
	return ""
}

func GetSpanID(ctx context.Context) string {
	if tc, ok := ctx.Value(traceContextKey{}).(traceContext); ok {
		return tc.SpanID
	}
	return ""
}

func WithParentSpan(ctx context.Context, parentSpanID string) context.Context {
	return context.WithValue(ctx, spanContextKey{}, parentSpanID)
}

func GetParentSpanID(ctx context.Context) string {
	if id, ok := ctx.Value(spanContextKey{}).(string); ok {
		return id
	}
	return ""
}

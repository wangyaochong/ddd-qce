package trace

import (
	"context"
	"encoding/hex"

	"github.com/google/uuid"
)

type traceContextKey struct{}
type spanContextKey struct{}

type traceContext struct {
	TraceID string
	SpanID  string
}

// NewTraceID generates a new unique trace identifier as a hex-encoded UUID.
func NewTraceID() string {
	id := uuid.New()
	return hex.EncodeToString(id[:])
}

// NewSpanID generates a new unique span identifier as a hex-encoded UUID.
func NewSpanID() string {
	id := uuid.New()
	return hex.EncodeToString(id[:])
}

// WithTrace returns a context carrying the given trace and span identifiers.
func WithTrace(ctx context.Context, traceID, spanID string) context.Context {
	return context.WithValue(ctx, traceContextKey{}, traceContext{
		TraceID: traceID,
		SpanID:  spanID,
	})
}

// GetTraceID extracts the trace ID from the context, or returns an empty string.
func GetTraceID(ctx context.Context) string {
	if tc, ok := ctx.Value(traceContextKey{}).(traceContext); ok {
		return tc.TraceID
	}
	return ""
}

// GetSpanID extracts the span ID from the context, or returns an empty string.
func GetSpanID(ctx context.Context) string {
	if tc, ok := ctx.Value(traceContextKey{}).(traceContext); ok {
		return tc.SpanID
	}
	return ""
}

// WithParentSpan returns a context carrying the given parent span identifier.
func WithParentSpan(ctx context.Context, parentSpanID string) context.Context {
	return context.WithValue(ctx, spanContextKey{}, parentSpanID)
}

// GetParentSpanID extracts the parent span ID from the context, or returns an empty string.
func GetParentSpanID(ctx context.Context) string {
	if id, ok := ctx.Value(spanContextKey{}).(string); ok {
		return id
	}
	return ""
}

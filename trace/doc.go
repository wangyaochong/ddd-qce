// Package trace provides distributed tracing: TraceID/SpanID generation,
// context propagation via WithTrace/GetTraceID/GetSpanID, parent-child
// span linking via WithParentSpan, and an InMemoryTraceStore with TTL
// and background cleanup.
//
// The TracingAspect (aspect/builtin) automatically creates spans for
// commands, queries, and events.
package trace

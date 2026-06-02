package trace

import "time"

// SpanType identifies the kind of operation a span represents.
type SpanType string

const (
	// SpanTypeCommand indicates a command execution span.
	SpanTypeCommand SpanType = "command"
	// SpanTypeQuery indicates a query execution span.
	SpanTypeQuery SpanType = "query"
	// SpanTypeEvent indicates an event processing span.
	SpanTypeEvent SpanType = "event"
)

// SpanStatus indicates the outcome of an operation.
type SpanStatus string

const (
	// SpanStatusSuccess indicates the operation completed normally.
	SpanStatusSuccess SpanStatus = "success"
	// SpanStatusError indicates the operation failed with an infrastructure error.
	SpanStatusError SpanStatus = "error"
	// SpanStatusBusinessError indicates the operation failed with a domain/business error.
	SpanStatusBusinessError SpanStatus = "business_error"
)

// Span records timing and metadata for a single operation within a trace.
type Span struct {
	ID        string        `json:"id"`
	TraceID   string        `json:"trace_id"`
	ParentID  string        `json:"parent_id"`
	Type      SpanType      `json:"type"`
	Name      string        `json:"name"`
	Status    SpanStatus    `json:"status"`
	Error     string        `json:"error,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	Duration  time.Duration `json:"duration"`
}

// TraceFilter filters spans when querying trace data.
type TraceFilter struct {
	TraceID      string
	Type         SpanType
	Status       SpanStatus
	StartTime    time.Time
	EndTime      time.Time
	NameContains string
}

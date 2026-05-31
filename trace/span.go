package trace

import "time"

type SpanType string

const (
	SpanTypeCommand SpanType = "command"
	SpanTypeQuery   SpanType = "query"
	SpanTypeEvent   SpanType = "event"
)

type SpanStatus string

const (
	SpanStatusSuccess       SpanStatus = "success"
	SpanStatusError         SpanStatus = "error"
	SpanStatusBusinessError SpanStatus = "business_error"
)

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

type TraceFilter struct {
	TraceID      string
	Type         SpanType
	Status       SpanStatus
	StartTime    time.Time
	EndTime      time.Time
	NameContains string
}

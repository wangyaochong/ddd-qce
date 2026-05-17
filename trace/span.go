package trace

import "time"

const (
	SpanTypeCommand = "command"
	SpanTypeQuery   = "query"
	SpanTypeEvent   = "event"

	SpanStatusSuccess = "success"
	SpanStatusError   = "error"
)

type Span struct {
	ID        string        `json:"id"`
	TraceID   string        `json:"trace_id"`
	ParentID  string        `json:"parent_id"`
	Type      string        `json:"type"`
	Name      string        `json:"name"`
	Status    string        `json:"status"`
	Error     string        `json:"error,omitempty"`
	StartedAt time.Time     `json:"started_at"`
	Duration  time.Duration `json:"duration"`
}

type TraceFilter struct {
	TraceID    string
	Type       string
	Status     string
	StartTime  time.Time
	EndTime    time.Time
	NameContains string
}

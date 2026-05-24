package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ddd-qce/core/domain/event"
	"github.com/ddd-qce/core/trace"
)

type CommandEntry struct {
	TraceID     string
	SpanID      string
	CommandType string
	CommandData json.RawMessage
	ResultType  string
	ResultData  json.RawMessage
	Error       string
	Duration    time.Duration
	CreatedAt   time.Time
}

type QueryEntry struct {
	TraceID    string
	SpanID     string
	QueryType  string
	QueryData  json.RawMessage
	ResultType string
	ResultData json.RawMessage
	Error      string
	Duration   time.Duration
	CreatedAt  time.Time
}

type EventEntry struct {
	TraceID      string
	SpanID       string
	AggregateID  string
	EventType    string
	EventData    json.RawMessage
	HandlerCount int
	Error        string
	Duration     time.Duration
	CreatedAt    time.Time
}

type EventHandlerEntry struct {
	TraceID     string
	SpanID      string
	AggregateID string
	EventType   string
	HandlerType string
	Status      string
	Error       string
	Duration    time.Duration
	CreatedAt   time.Time
}

type MessageStore interface {
	RecordCommand(ctx context.Context, entry *CommandEntry) error
	RecordQuery(ctx context.Context, entry *QueryEntry) error
	RecordEvent(ctx context.Context, entry *EventEntry) error
	RecordEventHandler(ctx context.Context, entry *EventHandlerEntry) error
}

type PersistenceAspect struct {
	Store MessageStore
}

func NewPersistenceAspect(store MessageStore) *PersistenceAspect {
	return &PersistenceAspect{Store: store}
}

func (a *PersistenceAspect) Name() string { return "persistence" }
func (a *PersistenceAspect) Order() int   { return 200 }

func (a *PersistenceAspect) BeforeCommand(ctx context.Context, cmd any) (context.Context, error) {
	return ctx, nil
}

func (a *PersistenceAspect) AfterCommand(ctx context.Context, cmd any, result any, err error, duration time.Duration) error {
	cmdData, marshalErr := json.Marshal(cmd)
	if marshalErr != nil {
		cmdData = json.RawMessage(fmt.Sprintf(`{"_marshal_error":%q}`, marshalErr.Error()))
	}
	entry := &CommandEntry{
		TraceID:     trace.GetTraceID(ctx),
		SpanID:      trace.GetSpanID(ctx),
		CommandType: typeName(cmd),
		CommandData: cmdData,
		Duration:    duration,
		CreatedAt:   time.Now(),
	}
	if result != nil {
		entry.ResultType = typeName(result)
		rd, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			rd = json.RawMessage(fmt.Sprintf(`{"_marshal_error":%q}`, marshalErr.Error()))
		}
		entry.ResultData = rd
	}
	if err != nil {
		entry.Error = err.Error()
	}
	return a.Store.RecordCommand(ctx, entry)
}

func (a *PersistenceAspect) BeforeQuery(ctx context.Context, query any) (context.Context, error) {
	return ctx, nil
}

func (a *PersistenceAspect) AfterQuery(ctx context.Context, query any, result any, err error, duration time.Duration) error {
	queryData, marshalErr := json.Marshal(query)
	if marshalErr != nil {
		queryData = json.RawMessage(fmt.Sprintf(`{"_marshal_error":%q}`, marshalErr.Error()))
	}
	entry := &QueryEntry{
		TraceID:    trace.GetTraceID(ctx),
		SpanID:     trace.GetSpanID(ctx),
		QueryType:  typeName(query),
		QueryData:  queryData,
		Duration:   duration,
		CreatedAt:  time.Now(),
	}
	if result != nil {
		entry.ResultType = typeName(result)
		rd, marshalErr := json.Marshal(result)
		if marshalErr != nil {
			rd = json.RawMessage(fmt.Sprintf(`{"_marshal_error":%q}`, marshalErr.Error()))
		}
		entry.ResultData = rd
	}
	if err != nil {
		entry.Error = err.Error()
	}
	return a.Store.RecordQuery(ctx, entry)
}

func (a *PersistenceAspect) BeforePublish(ctx context.Context, evt any) (context.Context, error) {
	return ctx, nil
}

func (a *PersistenceAspect) AfterPublish(ctx context.Context, evt any, err error, duration time.Duration) error {
	evtData, marshalErr := json.Marshal(evt)
	if marshalErr != nil {
		evtData = json.RawMessage(fmt.Sprintf(`{"_marshal_error":%q}`, marshalErr.Error()))
	}
	aggregateID, eventType := extractEventMeta(evt)
	handlerType := extractHandlerType(ctx)

	if handlerType != "" {
		entry := &EventHandlerEntry{
			TraceID:     trace.GetTraceID(ctx),
			SpanID:      trace.GetSpanID(ctx),
			AggregateID: aggregateID,
			EventType:   eventType,
			HandlerType: handlerType,
			Duration:    duration,
			CreatedAt:   time.Now(),
		}
		if err != nil {
			entry.Status = "error"
			entry.Error = err.Error()
		} else {
			entry.Status = "success"
		}
		return a.Store.RecordEventHandler(ctx, entry)
	}

	entry := &EventEntry{
		TraceID:     trace.GetTraceID(ctx),
		SpanID:      trace.GetSpanID(ctx),
		AggregateID: aggregateID,
		EventType:   eventType,
		EventData:   evtData,
		Duration:    duration,
		CreatedAt:   time.Now(),
	}
	if err != nil {
		entry.Error = err.Error()
	}
	return a.Store.RecordEvent(ctx, entry)
}

type handlerTypeKey struct{}

func ContextWithHandlerType(ctx context.Context, handlerType string) context.Context {
	return context.WithValue(ctx, handlerTypeKey{}, handlerType)
}

func extractHandlerType(ctx context.Context) string {
	v, _ := ctx.Value(handlerTypeKey{}).(string)
	return v
}

func extractEventMeta(evt any) (aggregateID, eventType string) {
	if de, ok := evt.(interface{ AggregateID() string }); ok {
		aggregateID = de.AggregateID()
	}
	eventType = event.EventTypeOf(evt)
	return
}

package builtintest

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
)

func TestMessageStoreContract(t *testing.T, store builtin.MessageStore) {
	t.Helper()

	t.Run("RecordCommand", func(t *testing.T) {
		entry := &builtin.CommandEntry{
			TraceID:     "tc-trace-1",
			SpanID:      "tc-span-1",
			CommandType: "CreateOrder",
			CommandData: json.RawMessage(`{"item":"widget"}`),
			ResultType:  "OrderResult",
			ResultData:  json.RawMessage(`{"id":"123"}`),
			Duration:    50 * time.Millisecond,
			CreatedAt:   time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordCommand(context.Background(), entry); err != nil {
			t.Fatalf("RecordCommand returned error: %v", err)
		}
	})

	t.Run("RecordCommandWithError", func(t *testing.T) {
		entry := &builtin.CommandEntry{
			TraceID:     "tc-trace-2",
			SpanID:      "tc-span-2",
			CommandType: "DeleteOrder",
			CommandData: json.RawMessage(`{"id":"ord-2"}`),
			Error:       "order not found",
			Duration:    10 * time.Millisecond,
			CreatedAt:   time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordCommand(context.Background(), entry); err != nil {
			t.Fatalf("RecordCommand with error returned error: %v", err)
		}
	})

	t.Run("RecordQuery", func(t *testing.T) {
		entry := &builtin.QueryEntry{
			TraceID:    "tc-trace-3",
			SpanID:     "tc-span-3",
			QueryType:  "GetOrder",
			QueryData:  json.RawMessage(`{"id":"123"}`),
			ResultType: "OrderView",
			ResultData: json.RawMessage(`{"status":"shipped"}`),
			Duration:   10 * time.Millisecond,
			CreatedAt:  time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordQuery(context.Background(), entry); err != nil {
			t.Fatalf("RecordQuery returned error: %v", err)
		}
	})

	t.Run("RecordQueryWithError", func(t *testing.T) {
		entry := &builtin.QueryEntry{
			TraceID:   "tc-trace-4",
			SpanID:    "tc-span-4",
			QueryType: "GetOrder",
			QueryData: json.RawMessage(`{"id":"999"}`),
			Error:     "not found",
			Duration:  3 * time.Millisecond,
			CreatedAt: time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordQuery(context.Background(), entry); err != nil {
			t.Fatalf("RecordQuery with error returned error: %v", err)
		}
	})

	t.Run("RecordEvent", func(t *testing.T) {
		entry := &builtin.EventEntry{
			TraceID:      "tc-trace-5",
			SpanID:       "tc-span-5",
			AggregateID:  "agg-1",
			EventType:    "OrderCreated",
			EventData:    json.RawMessage(`{"orderId":"123"}`),
			HandlerCount: 2,
			Duration:     30 * time.Millisecond,
			CreatedAt:    time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordEvent(context.Background(), entry); err != nil {
			t.Fatalf("RecordEvent returned error: %v", err)
		}
	})

	t.Run("RecordEventWithError", func(t *testing.T) {
		entry := &builtin.EventEntry{
			TraceID:     "tc-trace-6",
			SpanID:      "tc-span-6",
			AggregateID: "agg-2",
			EventType:   "OrderFailed",
			EventData:   json.RawMessage(`{"reason":"timeout"}`),
			Error:       "handler panic",
			Duration:    5 * time.Millisecond,
			CreatedAt:   time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordEvent(context.Background(), entry); err != nil {
			t.Fatalf("RecordEvent with error returned error: %v", err)
		}
	})

	t.Run("RecordEventHandler", func(t *testing.T) {
		entry := &builtin.EventHandlerEntry{
			TraceID:     "tc-trace-7",
			SpanID:      "tc-span-7",
			AggregateID: "agg-1",
			EventType:   "OrderCreated",
			HandlerType: "SendConfirmationEmail",
			Status:      "success",
			Duration:    5 * time.Millisecond,
			CreatedAt:   time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordEventHandler(context.Background(), entry); err != nil {
			t.Fatalf("RecordEventHandler returned error: %v", err)
		}
	})

	t.Run("RecordEventHandlerWithErrorStatus", func(t *testing.T) {
		entry := &builtin.EventHandlerEntry{
			TraceID:     "tc-trace-8",
			SpanID:      "tc-span-8",
			AggregateID: "agg-1",
			EventType:   "OrderCreated",
			HandlerType: "UpdateInventory",
			Status:      "error",
			Error:       "connection refused",
			Duration:    2 * time.Millisecond,
			CreatedAt:   time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordEventHandler(context.Background(), entry); err != nil {
			t.Fatalf("RecordEventHandler with error status returned error: %v", err)
		}
	})

	t.Run("AllEntryTypesTogether", func(t *testing.T) {
		cmd := &builtin.CommandEntry{
			TraceID:     "tc-trace-all",
			SpanID:      "tc-span-cmd",
			CommandType: "CreateOrder",
			CommandData: json.RawMessage(`{"item":"widget"}`),
			Duration:    50 * time.Millisecond,
			CreatedAt:   time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordCommand(context.Background(), cmd); err != nil {
			t.Fatalf("RecordCommand returned error: %v", err)
		}

		query := &builtin.QueryEntry{
			TraceID:   "tc-trace-all",
			SpanID:    "tc-span-qry",
			QueryType: "GetOrder",
			QueryData: json.RawMessage(`{"id":"123"}`),
			Duration:  10 * time.Millisecond,
			CreatedAt: time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordQuery(context.Background(), query); err != nil {
			t.Fatalf("RecordQuery returned error: %v", err)
		}

		evt := &builtin.EventEntry{
			TraceID:      "tc-trace-all",
			SpanID:       "tc-span-evt",
			AggregateID:  "agg-all",
			EventType:    "OrderCreated",
			EventData:    json.RawMessage(`{"orderId":"123"}`),
			HandlerCount: 1,
			Duration:     30 * time.Millisecond,
			CreatedAt:    time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordEvent(context.Background(), evt); err != nil {
			t.Fatalf("RecordEvent returned error: %v", err)
		}

		handler := &builtin.EventHandlerEntry{
			TraceID:     "tc-trace-all",
			SpanID:      "tc-span-hdl",
			AggregateID: "agg-all",
			EventType:   "OrderCreated",
			HandlerType: "NotifyUser",
			Status:      "success",
			Duration:    5 * time.Millisecond,
			CreatedAt:   time.Now().Truncate(time.Microsecond),
		}
		if err := store.RecordEventHandler(context.Background(), handler); err != nil {
			t.Fatalf("RecordEventHandler returned error: %v", err)
		}
	})
}

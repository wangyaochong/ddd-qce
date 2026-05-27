package builtin

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestInMemoryMessageStore_RecordCommand(t *testing.T) {
	s := NewInMemoryMessageStore()
	entry := &CommandEntry{
		TraceID:     "t1",
		SpanID:      "s1",
		CommandType: "CreateOrder",
		CommandData: json.RawMessage(`{"item":"widget"}`),
		ResultType:  "OrderResult",
		ResultData:  json.RawMessage(`{"id":"123"}`),
		Duration:    50 * time.Millisecond,
		CreatedAt:   time.Now(),
	}
	if err := s.RecordCommand(context.Background(), entry); err != nil {
		t.Fatalf("RecordCommand returned error: %v", err)
	}
	if len(s.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(s.commands))
	}
	got := s.commands[0]
	if got.TraceID != "t1" {
		t.Errorf("expected TraceID t1, got %s", got.TraceID)
	}
	if got.CommandType != "CreateOrder" {
		t.Errorf("expected CommandType CreateOrder, got %s", got.CommandType)
	}
	if got.ResultType != "OrderResult" {
		t.Errorf("expected ResultType OrderResult, got %s", got.ResultType)
	}
	if got.Duration != 50*time.Millisecond {
		t.Errorf("expected Duration 50ms, got %v", got.Duration)
	}
}

func TestInMemoryMessageStore_RecordQuery(t *testing.T) {
	s := NewInMemoryMessageStore()
	entry := &QueryEntry{
		TraceID:    "t2",
		SpanID:     "s2",
		QueryType:  "GetOrder",
		QueryData:  json.RawMessage(`{"id":"123"}`),
		ResultType: "OrderView",
		ResultData: json.RawMessage(`{"status":"shipped"}`),
		Duration:   10 * time.Millisecond,
		CreatedAt:  time.Now(),
	}
	if err := s.RecordQuery(context.Background(), entry); err != nil {
		t.Fatalf("RecordQuery returned error: %v", err)
	}
	if len(s.queries) != 1 {
		t.Fatalf("expected 1 query, got %d", len(s.queries))
	}
	got := s.queries[0]
	if got.TraceID != "t2" {
		t.Errorf("expected TraceID t2, got %s", got.TraceID)
	}
	if got.QueryType != "GetOrder" {
		t.Errorf("expected QueryType GetOrder, got %s", got.QueryType)
	}
	if got.ResultType != "OrderView" {
		t.Errorf("expected ResultType OrderView, got %s", got.ResultType)
	}
}

func TestInMemoryMessageStore_RecordEvent(t *testing.T) {
	s := NewInMemoryMessageStore()
	entry := &EventEntry{
		TraceID:      "t3",
		SpanID:       "s3",
		AggregateID:  "agg1",
		EventType:    "OrderCreated",
		EventData:    json.RawMessage(`{"orderId":"123"}`),
		HandlerCount: 2,
		Duration:     30 * time.Millisecond,
		CreatedAt:    time.Now(),
	}
	if err := s.RecordEvent(context.Background(), entry); err != nil {
		t.Fatalf("RecordEvent returned error: %v", err)
	}
	if len(s.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(s.events))
	}
	got := s.events[0]
	if got.TraceID != "t3" {
		t.Errorf("expected TraceID t3, got %s", got.TraceID)
	}
	if got.EventType != "OrderCreated" {
		t.Errorf("expected EventType OrderCreated, got %s", got.EventType)
	}
	if got.AggregateID != "agg1" {
		t.Errorf("expected AggregateID agg1, got %s", got.AggregateID)
	}
	if got.HandlerCount != 2 {
		t.Errorf("expected HandlerCount 2, got %d", got.HandlerCount)
	}
}

func TestInMemoryMessageStore_RecordEventHandler(t *testing.T) {
	s := NewInMemoryMessageStore()
	entry := &EventHandlerEntry{
		TraceID:     "t4",
		SpanID:      "s4",
		AggregateID: "agg1",
		EventType:   "OrderCreated",
		HandlerType: "SendConfirmationEmail",
		Status:      "success",
		Duration:    5 * time.Millisecond,
		CreatedAt:   time.Now(),
	}
	if err := s.RecordEventHandler(context.Background(), entry); err != nil {
		t.Fatalf("RecordEventHandler returned error: %v", err)
	}
	if len(s.handlers) != 1 {
		t.Fatalf("expected 1 handler, got %d", len(s.handlers))
	}
	got := s.handlers[0]
	if got.TraceID != "t4" {
		t.Errorf("expected TraceID t4, got %s", got.TraceID)
	}
	if got.HandlerType != "SendConfirmationEmail" {
		t.Errorf("expected HandlerType SendConfirmationEmail, got %s", got.HandlerType)
	}
	if got.Status != "success" {
		t.Errorf("expected Status success, got %s", got.Status)
	}
	if got.EventType != "OrderCreated" {
		t.Errorf("expected EventType OrderCreated, got %s", got.EventType)
	}
}

func TestInMemoryMessageStore_MaxSizeTrimsOldest(t *testing.T) {
	s := NewInMemoryMessageStore(WithInMemoryMaxSize(3))
	for i := 0; i < 5; i++ {
		entry := &CommandEntry{
			TraceID:     "trace",
			CommandType: "Cmd",
			Duration:    time.Duration(i) * time.Millisecond,
			CreatedAt:   time.Now(),
		}
		if err := s.RecordCommand(context.Background(), entry); err != nil {
			t.Fatalf("RecordCommand returned error: %v", err)
		}
	}
	if len(s.commands) != 3 {
		t.Fatalf("expected 3 commands after trim, got %d", len(s.commands))
	}
	if s.commands[0].Duration != 2*time.Millisecond {
		t.Errorf("expected oldest kept duration 2ms, got %v", s.commands[0].Duration)
	}
	if s.commands[2].Duration != 4*time.Millisecond {
		t.Errorf("expected newest duration 4ms, got %v", s.commands[2].Duration)
	}
}

func TestInMemoryMessageStore_DefaultMaxSize(t *testing.T) {
	s := NewInMemoryMessageStore()
	if s.maxSize != 1000 {
		t.Errorf("expected default maxSize 1000, got %d", s.maxSize)
	}
}

func TestInMemoryMessageStore_InterfaceConformance(t *testing.T) {
	var _ MessageStore = NewInMemoryMessageStore()
}

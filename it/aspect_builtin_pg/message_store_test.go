package pg

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/aspect/builtin/builtintest"
	pgmsg "github.com/ddd-qce/core/aspect/builtin/pg"
	corepg "github.com/ddd-qce/core/pg"
	"github.com/ddd-qce/it/testutil"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func openTestDB(t *testing.T) *sql.DB {
	return testutil.OpenTestDB(t, "ddd_qce_aspect_test")
}

func TestPgMessageStore_RecordCommand(t *testing.T) {
	db := openTestDB(t)
	store := pgmsg.NewMessageStore(db)
	ctx := context.Background()

	entry := &builtin.CommandEntry{
		TraceID:     "trace-1",
		SpanID:      "span-1",
		CommandType: "CreateOrder",
		CommandData: []byte(`{"id":"ord-1"}`),
		ResultType:  "OrderCreated",
		ResultData:  []byte(`{"status":"created"}`),
		Duration:    50 * time.Millisecond,
		CreatedAt:   time.Now(),
	}

	if err := store.RecordCommand(ctx, entry); err != nil {
		t.Fatalf("RecordCommand failed: %v", err)
	}

	var cmdType string
	err := db.QueryRow("SELECT command_type FROM ddd_command_log WHERE trace_id = $1", "trace-1").Scan(&cmdType)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if cmdType != "CreateOrder" {
		t.Errorf("expected command_type 'CreateOrder', got %s", cmdType)
	}
}

func TestPgMessageStore_RecordCommandWithError(t *testing.T) {
	db := openTestDB(t)
	store := pgmsg.NewMessageStore(db)
	ctx := context.Background()

	entry := &builtin.CommandEntry{
		TraceID:     "trace-2",
		SpanID:      "span-2",
		CommandType: "DeleteOrder",
		CommandData: []byte(`{"id":"ord-2"}`),
		Error:       "order not found",
		Duration:    10 * time.Millisecond,
		CreatedAt:   time.Now(),
	}

	if err := store.RecordCommand(ctx, entry); err != nil {
		t.Fatalf("RecordCommand failed: %v", err)
	}

	var errStr *string
	db.QueryRow("SELECT error FROM ddd_command_log WHERE trace_id = $1", "trace-2").Scan(&errStr)
	if errStr == nil || *errStr != "order not found" {
		t.Errorf("expected error 'order not found', got %v", errStr)
	}
}

func TestPgMessageStore_RecordQuery(t *testing.T) {
	db := openTestDB(t)
	store := pgmsg.NewMessageStore(db)
	ctx := context.Background()

	entry := &builtin.QueryEntry{
		TraceID:    "trace-3",
		SpanID:     "span-3",
		QueryType:  "GetOrder",
		QueryData:  []byte(`{"id":"ord-1"}`),
		ResultData: []byte(`{"status":"created"}`),
		Duration:   5 * time.Millisecond,
		CreatedAt:  time.Now(),
	}

	if err := store.RecordQuery(ctx, entry); err != nil {
		t.Fatalf("RecordQuery failed: %v", err)
	}

	var qType string
	err := db.QueryRow("SELECT query_type FROM ddd_query_log WHERE trace_id = $1", "trace-3").Scan(&qType)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if qType != "GetOrder" {
		t.Errorf("expected query_type 'GetOrder', got %s", qType)
	}
}

func TestPgMessageStore_RecordEvent(t *testing.T) {
	db := openTestDB(t)
	store := pgmsg.NewMessageStore(db)
	ctx := context.Background()

	entry := &builtin.EventEntry{
		TraceID:      "trace-4",
		SpanID:       "span-4",
		AggregateID:  "agg-1",
		EventType:    "OrderPlaced",
		EventData:    []byte(`{"amount":100}`),
		HandlerCount: 3,
		Duration:     20 * time.Millisecond,
		CreatedAt:    time.Now(),
	}

	if err := store.RecordEvent(ctx, entry); err != nil {
		t.Fatalf("RecordEvent failed: %v", err)
	}

	var eType string
	var hCount int
	err := db.QueryRow("SELECT event_type, handler_count FROM ddd_event_log WHERE trace_id = $1", "trace-4").Scan(&eType, &hCount)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if eType != "OrderPlaced" {
		t.Errorf("expected event_type 'OrderPlaced', got %s", eType)
	}
	if hCount != 3 {
		t.Errorf("expected handler_count 3, got %d", hCount)
	}
}

func TestPgMessageStore_RecordEventHandler(t *testing.T) {
	db := openTestDB(t)
	store := pgmsg.NewMessageStore(db)
	ctx := context.Background()

	entry := &builtin.EventHandlerEntry{
		TraceID:     "trace-5",
		SpanID:      "span-5",
		AggregateID: "agg-1",
		EventType:   "OrderPlaced",
		HandlerType: "InventoryHandler",
		Status:      "success",
		Duration:    8 * time.Millisecond,
		CreatedAt:   time.Now(),
	}

	if err := store.RecordEventHandler(ctx, entry); err != nil {
		t.Fatalf("RecordEventHandler failed: %v", err)
	}

	var hType, status string
	err := db.QueryRow("SELECT handler_type, status FROM ddd_event_handler_log WHERE trace_id = $1", "trace-5").Scan(&hType, &status)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if hType != "InventoryHandler" {
		t.Errorf("expected handler_type 'InventoryHandler', got %s", hType)
	}
	if status != "success" {
		t.Errorf("expected status 'success', got %s", status)
	}
}

func TestPgMessageStore_WithContextTransaction(t *testing.T) {
	db := openTestDB(t)
	store := pgmsg.NewMessageStore(db)
	ctx := context.Background()

	tm := corepg.NewTransactionManager(db)
	txCtx, err := tm.Begin(ctx)
	if err != nil {
		t.Fatalf("begin tx failed: %v", err)
	}

	entry := &builtin.CommandEntry{
		TraceID:     "trace-tx",
		CommandType: "TxCommand",
		CommandData: []byte(`{}`),
		Duration:    time.Millisecond,
		CreatedAt:   time.Now(),
	}
	if err := store.RecordCommand(txCtx, entry); err != nil {
		t.Fatalf("RecordCommand in tx failed: %v", err)
	}

	var count int
	db.QueryRow("SELECT count(*) FROM ddd_command_log WHERE trace_id = $1", "trace-tx").Scan(&count)
	if count != 0 {
		t.Error("expected 0 rows before commit (read from different connection)")
	}

	if err := tm.Commit(txCtx); err != nil {
		t.Fatalf("commit failed: %v", err)
	}

	db.QueryRow("SELECT count(*) FROM ddd_command_log WHERE trace_id = $1", "trace-tx").Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 row after commit, got %d", count)
	}
}

func TestPgMessageStore_Contract(t *testing.T) {
	db := openTestDB(t)
	store := pgmsg.NewMessageStore(db)
	builtintest.TestMessageStoreContract(t, store)
}

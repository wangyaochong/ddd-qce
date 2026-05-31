package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/trace"
)

func TestStatsCollector_ImplementsMetricsRecorder(t *testing.T) {
	var _ builtin.MetricsRecorder = NewStatsCollector()
}

func TestStatsCollector_RecordDurationAndGetStats(t *testing.T) {
	s := NewStatsCollector()

	s.RecordDuration("Command/PlaceOrder", 100*time.Millisecond)
	s.RecordDuration("Command/PlaceOrder", 200*time.Millisecond)
	s.RecordDuration("Command/PlaceOrder", 300*time.Millisecond)

	stats, ok := s.GetStats("Command/PlaceOrder")
	if !ok {
		t.Fatal("expected stats to exist")
	}

	if stats.Count != 3 {
		t.Errorf("expected count 3, got %d", stats.Count)
	}
	if stats.Type != "command" {
		t.Errorf("expected type command, got %s", stats.Type)
	}
	if stats.MinDuration != 100*time.Millisecond {
		t.Errorf("expected min 100ms, got %v", stats.MinDuration)
	}
	if stats.MaxDuration != 300*time.Millisecond {
		t.Errorf("expected max 300ms, got %v", stats.MaxDuration)
	}
	if stats.AvgDuration != 200*time.Millisecond {
		t.Errorf("expected avg 200ms, got %v", stats.AvgDuration)
	}
	if stats.ErrorCount != 0 {
		t.Errorf("expected 0 errors, got %d", stats.ErrorCount)
	}
}

func TestStatsCollector_RecordError(t *testing.T) {
	s := NewStatsCollector()

	s.RecordDuration("Command/PlaceOrder", 50*time.Millisecond)
	s.RecordError("Command/PlaceOrder", fmt.Errorf("test error"))

	stats, ok := s.GetStats("Command/PlaceOrder")
	if !ok {
		t.Fatal("expected stats to exist")
	}

	if stats.Count != 1 {
		t.Errorf("expected count 1, got %d", stats.Count)
	}
	if stats.ErrorCount != 1 {
		t.Errorf("expected 1 error, got %d", stats.ErrorCount)
	}
	if stats.LastError != "test error" {
		t.Errorf("expected last error 'test error', got %s", stats.LastError)
	}
}

func TestStatsCollector_GetAllStats(t *testing.T) {
	s := NewStatsCollector()

	s.RecordDuration("Command/PlaceOrder", 50*time.Millisecond)
	s.RecordDuration("Query/GetOrder", 30*time.Millisecond)
	s.RecordDuration("Event/OrderPlaced", 10*time.Millisecond)

	all := s.GetAllStats()
	if len(all) != 3 {
		t.Fatalf("expected 3 stats, got %d", len(all))
	}

	names := make([]string, len(all))
	for i, s := range all {
		names[i] = s.Name
	}
	if names[0] != "Command/PlaceOrder" || names[1] != "Event/OrderPlaced" || names[2] != "Query/GetOrder" {
		t.Errorf("expected sorted names, got %v", names)
	}
}

func TestStatsCollector_GetStatsByType(t *testing.T) {
	s := NewStatsCollector()

	s.RecordDuration("Command/PlaceOrder", 50*time.Millisecond)
	s.RecordDuration("Command/CancelOrder", 30*time.Millisecond)
	s.RecordDuration("Query/GetOrder", 20*time.Millisecond)

	cmdStats := s.GetStatsByType("command")
	if len(cmdStats) != 2 {
		t.Fatalf("expected 2 command stats, got %d", len(cmdStats))
	}

	queryStats := s.GetStatsByType("query")
	if len(queryStats) != 1 {
		t.Fatalf("expected 1 query stat, got %d", len(queryStats))
	}
}

func TestStatsCollector_GetStatsNotFound(t *testing.T) {
	s := NewStatsCollector()

	_, ok := s.GetStats("Command/NonExistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestStatsCollector_WindowEviction(t *testing.T) {
	s := NewStatsCollector(WithWindowSeconds(1))

	s.RecordDuration("Command/Test", 50*time.Millisecond)
	time.Sleep(1100 * time.Millisecond)
	s.RecordDuration("Command/Test", 100*time.Millisecond)

	stats, _ := s.GetStats("Command/Test")
	if stats.Count != 2 {
		t.Errorf("expected count 2 (total), got %d", stats.Count)
	}
	if stats.MinDuration != 100*time.Millisecond {
		t.Errorf("expected min 100ms (only recent in window), got %v", stats.MinDuration)
	}
}

func TestDashboard_StatsEndpoint(t *testing.T) {
	s := NewStatsCollector()
	s.RecordDuration("Command/PlaceOrder", 100*time.Millisecond)
	s.RecordDuration("Command/PlaceOrder", 200*time.Millisecond)

	d := NewDashboard(s, WithPrefix("/api/test"))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/test/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var stats []OperationStats
	json.NewDecoder(w.Body).Decode(&stats)
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	if stats[0].Name != "Command/PlaceOrder" {
		t.Errorf("expected Command/PlaceOrder, got %s", stats[0].Name)
	}
}

func TestDashboard_StatsByName(t *testing.T) {
	s := NewStatsCollector()
	s.RecordDuration("Command/PlaceOrder", 100*time.Millisecond)

	d := NewDashboard(s)
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/stats?name=Command/PlaceOrder", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDashboard_StatsByNameNotFound(t *testing.T) {
	s := NewStatsCollector()

	d := NewDashboard(s)
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/stats?name=Command/NonExistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDashboard_HealthEndpoint(t *testing.T) {
	s := NewStatsCollector()

	d := NewDashboard(s)
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["status"] != "ok" {
		t.Errorf("expected ok, got %v", result["status"])
	}
}

func TestDashboard_DisabledEndpoint(t *testing.T) {
	s := NewStatsCollector()

	d := NewDashboard(s, WithoutStats())
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for disabled endpoint, got %d", w.Code)
	}
}

func TestDashboard_CustomPrefix(t *testing.T) {
	s := NewStatsCollector()

	d := NewDashboard(s, WithPrefix("/debug/ddd"))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/debug/ddd/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with custom prefix, got %d", w.Code)
	}
}

func TestDashboard_JobsWithoutManager(t *testing.T) {
	s := NewStatsCollector()

	d := NewDashboard(s)
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/jobs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestObservableMessageStore_Commands(t *testing.T) {
	store := NewObservableMessageStore(WithMaxSize(5))

	now := time.Now()
	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "PlaceOrder", TraceID: "t1", CreatedAt: now, Duration: 100 * time.Millisecond,
	})
	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "CancelOrder", TraceID: "t2", CreatedAt: now, Error: "not found", Duration: 50 * time.Millisecond,
	})

	entries, err := store.QueryCommands(context.Background(), MessageFilter{Type: "PlaceOrder"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1, got %d", len(entries))
	}
	if entries[0].CommandType != "PlaceOrder" {
		t.Errorf("expected PlaceOrder, got %s", entries[0].CommandType)
	}

	all, _ := store.QueryCommands(context.Background(), MessageFilter{})
	if len(all) != 2 {
		t.Errorf("expected 2, got %d", len(all))
	}

	errOnly, _ := store.QueryCommands(context.Background(), MessageFilter{Status: "error"})
	if len(errOnly) != 1 {
		t.Errorf("expected 1 error, got %d", len(errOnly))
	}
}

func TestObservableMessageStore_MaxSize(t *testing.T) {
	store := NewObservableMessageStore(WithMaxSize(3))

	for i := 0; i < 5; i++ {
		store.RecordQuery(context.Background(), &builtin.QueryEntry{
			QueryType: fmt.Sprintf("Q%d", i), CreatedAt: time.Now(),
		})
	}

	entries, _ := store.QueryQueries(context.Background(), MessageFilter{})
	if len(entries) != 3 {
		t.Fatalf("expected 3 (maxSize), got %d", len(entries))
	}
	if entries[0].QueryType != "Q4" {
		t.Errorf("expected most recent first, got %s", entries[0].QueryType)
	}
}

func TestObservableMessageStore_Events(t *testing.T) {
	store := NewObservableMessageStore()

	store.RecordEvent(context.Background(), &builtin.EventEntry{
		EventType: "OrderPlaced", AggregateID: "A1", CreatedAt: time.Now(),
	})
	store.RecordEvent(context.Background(), &builtin.EventEntry{
		EventType: "OrderCancelled", AggregateID: "A2", CreatedAt: time.Now(),
	})

	byAgg, _ := store.QueryEvents(context.Background(), MessageFilter{AggregateID: "A1"})
	if len(byAgg) != 1 {
		t.Fatalf("expected 1, got %d", len(byAgg))
	}
	if byAgg[0].EventType != "OrderPlaced" {
		t.Errorf("expected OrderPlaced, got %s", byAgg[0].EventType)
	}
}

func TestObservableMessageStore_ImplementsMessageStore(t *testing.T) {
	var _ builtin.MessageStore = NewObservableMessageStore()
}

func TestObservableMessageStore_ImplementsReader(t *testing.T) {
	var _ MessageStoreReader = NewObservableMessageStore()
}

func TestComposeMetrics(t *testing.T) {
	s1 := NewStatsCollector()
	s2 := NewStatsCollector()

	composed := ComposeMetrics(s1, s2)
	composed.RecordDuration("Command/Test", 100*time.Millisecond)
	composed.RecordError("Command/Test", fmt.Errorf("err"))

	st1, _ := s1.GetStats("Command/Test")
	st2, _ := s2.GetStats("Command/Test")

	if st1.Count != 1 || st2.Count != 1 {
		t.Errorf("expected both recorders to have count 1, got %d and %d", st1.Count, st2.Count)
	}
	if st1.ErrorCount != 1 || st2.ErrorCount != 1 {
		t.Errorf("expected both recorders to have 1 error, got %d and %d", st1.ErrorCount, st2.ErrorCount)
	}
}

func TestDashboard_CommandsWithReader(t *testing.T) {
	s := NewStatsCollector()
	store := NewObservableMessageStore()
	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "PlaceOrder", TraceID: "t1", CreatedAt: time.Now(), Duration: 100 * time.Millisecond,
	})

	d := NewDashboard(s, WithMessageReader(store))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/commands", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestComposeMetrics_RecordErrorAndDuration(t *testing.T) {
	rec1 := NewStatsCollector()
	rec2 := NewStatsCollector()

	composed := ComposeMetrics(rec1, rec2)
	if composed == nil {
		t.Error("ComposeMetrics should not return nil")
	}

	// Test RecordDuration delegates to all recorders
	composed.RecordDuration("test", 100*time.Millisecond)

	stats, ok := rec1.GetStats("test")
	if !ok {
		t.Fatal("expected stats in rec1")
	}
	if stats.Count != 1 {
		t.Errorf("expected count 1, got %d", stats.Count)
	}

	stats2, ok := rec2.GetStats("test")
	if !ok {
		t.Fatal("expected stats in rec2")
	}
	if stats2.Count != 1 {
		t.Errorf("expected count 1 in rec2, got %d", stats2.Count)
	}
}

func TestComposeMetrics_DelegatesToAll(t *testing.T) {
	rec1 := NewStatsCollector()
	rec2 := NewStatsCollector()

	composed := ComposeMetrics(rec1, rec2)
	composed.RecordError("test", fmt.Errorf("test error"))

	stats, ok := rec1.GetStats("test")
	if !ok {
		t.Fatal("expected stats in rec1")
	}
	if stats.ErrorCount != 1 {
		t.Errorf("expected error count 1, got %d", stats.ErrorCount)
	}
}

func TestClassifyOpType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"command", "Command/PlaceOrder", "command"},
		{"query", "Query/GetOrder", "query"},
		{"event", "Event/OrderPlaced", "event"},
		{"unknown", "SomeOther", "unknown"},
		{"empty", "", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := classifyOpType(tt.input)
			if result != tt.expected {
				t.Errorf("classifyOpType(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestStatsCollector_GetStatsByType_Filtering(t *testing.T) {
	s := NewStatsCollector()

	s.RecordDuration("Command/PlaceOrder", 100*time.Millisecond)
	s.RecordDuration("Query/GetOrder", 50*time.Millisecond)
	s.RecordDuration("Event/OrderPlaced", 25*time.Millisecond)

	cmdStats := s.GetStatsByType("command")
	if len(cmdStats) != 1 {
		t.Errorf("expected 1 command stat, got %d", len(cmdStats))
	}

	queryStats := s.GetStatsByType("query")
	if len(queryStats) != 1 {
		t.Errorf("expected 1 query stat, got %d", len(queryStats))
	}

	eventStats := s.GetStatsByType("event")
	if len(eventStats) != 1 {
		t.Errorf("expected 1 event stat, got %d", len(eventStats))
	}

	unknownStats := s.GetStatsByType("unknown")
	if len(unknownStats) != 0 {
		t.Errorf("expected 0 unknown stats, got %d", len(unknownStats))
	}
}

func TestStatsCollector_GetAllStats_ReturnsSortedList(t *testing.T) {
	s := NewStatsCollector()

	s.RecordDuration("Command/PlaceOrder", 100*time.Millisecond)
	s.RecordDuration("Query/GetOrder", 50*time.Millisecond)

	allStats := s.GetAllStats()
	if len(allStats) != 2 {
		t.Errorf("expected 2 stats, got %d", len(allStats))
	}

	// Verify sorted by name
	if allStats[0].Name != "Command/PlaceOrder" {
		t.Errorf("expected first to be Command/PlaceOrder, got %s", allStats[0].Name)
	}
	if allStats[1].Name != "Query/GetOrder" {
		t.Errorf("expected second to be Query/GetOrder, got %s", allStats[1].Name)
	}
}

func TestStatsCollector_GetStats_NotFound(t *testing.T) {
	s := NewStatsCollector()

	_, ok := s.GetStats("nonexistent")
	if ok {
		t.Error("expected false for nonexistent stats")
	}
}

func TestStatsCollector_RecordError_NilError(t *testing.T) {
	s := NewStatsCollector()
	s.RecordError("test", nil)

	stats, ok := s.GetStats("test")
	if !ok {
		t.Fatal("expected stats to exist")
	}
	if stats.ErrorCount != 1 {
		t.Errorf("expected error count 1, got %d", stats.ErrorCount)
	}
	if stats.LastError != "" {
		t.Errorf("expected empty last error, got %s", stats.LastError)
	}
}

func TestStatsCollector_WithWindowSeconds(t *testing.T) {
	s := NewStatsCollector(WithWindowSeconds(60))
	if s.windowSec != 60 {
		t.Errorf("expected windowSec 60, got %d", s.windowSec)
	}
}

type mockJobManager struct {
	jobs []*jobcore.Job
	err  error
}

func (m *mockJobManager) Submit(_ context.Context, _ any, _ ...jobcore.JobOption) (*jobcore.Job, error) {
	return nil, nil
}
func (m *mockJobManager) GetStatus(_ context.Context, _ string) (*jobcore.Job, error) {
	return nil, nil
}
func (m *mockJobManager) Cancel(_ context.Context, _ string) error { return nil }
func (m *mockJobManager) Retry(_ context.Context, _ string) error  { return nil }
func (m *mockJobManager) Wait(_ context.Context, _ string, _ time.Duration) (*jobcore.Job, error) {
	return nil, nil
}
func (m *mockJobManager) WaitForRunning(_ context.Context, _ string, _ time.Duration) (*jobcore.Job, error) {
	return nil, nil
}
func (m *mockJobManager) ListByStatus(_ context.Context, _ jobcore.JobStatus) ([]*jobcore.Job, error) {
	return m.jobs, m.err
}
func (m *mockJobManager) Shutdown(_ context.Context) error { return nil }

type errTraceStore struct{}

func (e *errTraceStore) RecordSpan(_ context.Context, _ *trace.Span) error { return nil }
func (e *errTraceStore) GetTrace(_ context.Context, _ string) ([]*trace.Span, error) {
	return nil, fmt.Errorf("trace error")
}
func (e *errTraceStore) ListTraces(_ context.Context, _ trace.TraceFilter) ([]string, error) {
	return nil, fmt.Errorf("trace store unavailable")
}
func (e *errTraceStore) Close() error { return nil }

func TestObservableMessageStore_RecordEventHandler(t *testing.T) {
	store := NewObservableMessageStore()
	err := store.RecordEventHandler(context.Background(), &builtin.EventHandlerEntry{
		EventType:   "OrderPlaced",
		HandlerType: "OrderProjection",
		Status:      "success",
		CreatedAt:   time.Now(),
	})
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}

func TestObservableMessageStore_QueryCommands_TraceID(t *testing.T) {
	store := NewObservableMessageStore()
	now := time.Now()
	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "PlaceOrder", TraceID: "t1", CreatedAt: now,
	})
	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "CancelOrder", TraceID: "t2", CreatedAt: now,
	})

	entries, err := store.QueryCommands(context.Background(), MessageFilter{TraceID: "t1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1, got %d", len(entries))
	}
	if entries[0].TraceID != "t1" {
		t.Errorf("expected TraceID t1, got %s", entries[0].TraceID)
	}
}

func TestObservableMessageStore_QueryCommands_Since(t *testing.T) {
	store := NewObservableMessageStore()
	old := time.Now().Add(-2 * time.Hour)
	recent := time.Now()
	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "OldCmd", CreatedAt: old,
	})
	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "NewCmd", CreatedAt: recent,
	})

	entries, _ := store.QueryCommands(context.Background(), MessageFilter{Since: recent.Add(-1 * time.Hour)})
	if len(entries) != 1 {
		t.Fatalf("expected 1, got %d", len(entries))
	}
	if entries[0].CommandType != "NewCmd" {
		t.Errorf("expected NewCmd, got %s", entries[0].CommandType)
	}
}

func TestObservableMessageStore_QueryQueries_VariousFilters(t *testing.T) {
	store := NewObservableMessageStore()
	now := time.Now()
	store.RecordQuery(context.Background(), &builtin.QueryEntry{
		QueryType: "GetOrder", TraceID: "t1", CreatedAt: now,
	})
	store.RecordQuery(context.Background(), &builtin.QueryEntry{
		QueryType: "ListOrders", TraceID: "t2", Error: "timeout", CreatedAt: now,
	})
	store.RecordQuery(context.Background(), &builtin.QueryEntry{
		QueryType: "GetOrder", TraceID: "t3", CreatedAt: now.Add(-2 * time.Hour),
	})

	byType, _ := store.QueryQueries(context.Background(), MessageFilter{Type: "GetOrder"})
	if len(byType) != 2 {
		t.Errorf("expected 2 for type filter, got %d", len(byType))
	}

	byTraceID, _ := store.QueryQueries(context.Background(), MessageFilter{TraceID: "t1"})
	if len(byTraceID) != 1 {
		t.Errorf("expected 1 for TraceID filter, got %d", len(byTraceID))
	}

	byError, _ := store.QueryQueries(context.Background(), MessageFilter{Status: "error"})
	if len(byError) != 1 {
		t.Errorf("expected 1 for error filter, got %d", len(byError))
	}

	bySuccess, _ := store.QueryQueries(context.Background(), MessageFilter{Status: "success"})
	if len(bySuccess) != 2 {
		t.Errorf("expected 2 for success filter, got %d", len(bySuccess))
	}

	since := now.Add(-1 * time.Hour)
	bySince, _ := store.QueryQueries(context.Background(), MessageFilter{Since: since})
	if len(bySince) != 2 {
		t.Errorf("expected 2 for since filter, got %d", len(bySince))
	}
}

func TestObservableMessageStore_QueryEvents_TraceID(t *testing.T) {
	store := NewObservableMessageStore()
	now := time.Now()
	store.RecordEvent(context.Background(), &builtin.EventEntry{
		EventType: "OrderPlaced", TraceID: "t1", AggregateID: "A1", CreatedAt: now,
	})
	store.RecordEvent(context.Background(), &builtin.EventEntry{
		EventType: "OrderCancelled", TraceID: "t2", AggregateID: "A2", CreatedAt: now,
	})

	entries, _ := store.QueryEvents(context.Background(), MessageFilter{TraceID: "t1"})
	if len(entries) != 1 {
		t.Fatalf("expected 1, got %d", len(entries))
	}
	if entries[0].TraceID != "t1" {
		t.Errorf("expected TraceID t1, got %s", entries[0].TraceID)
	}
}

func TestDashboard_HandleCommands_NoReader(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s)
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/commands", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestDashboard_HandleCommands_WithReader(t *testing.T) {
	s := NewStatsCollector()
	store := NewObservableMessageStore()
	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "PlaceOrder", CreatedAt: time.Now(),
	})

	d := NewDashboard(s, WithMessageReader(store))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/commands", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDashboard_HandleCommands_MethodNotAllowed(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s, WithMessageReader(NewObservableMessageStore()))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/ddd/commands", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestDashboard_HandleQueries_NoReader(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s)
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/queries", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestDashboard_HandleQueries_WithReader(t *testing.T) {
	s := NewStatsCollector()
	store := NewObservableMessageStore()
	store.RecordQuery(context.Background(), &builtin.QueryEntry{
		QueryType: "GetOrder", CreatedAt: time.Now(),
	})

	d := NewDashboard(s, WithMessageReader(store))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/queries", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDashboard_HandleQueries_MethodNotAllowed(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s, WithMessageReader(NewObservableMessageStore()))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/ddd/queries", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestDashboard_HandleEvents_NoReader(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s)
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/events", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestDashboard_HandleEvents_WithReader(t *testing.T) {
	s := NewStatsCollector()
	store := NewObservableMessageStore()
	store.RecordEvent(context.Background(), &builtin.EventEntry{
		EventType: "OrderPlaced", AggregateID: "A1", CreatedAt: time.Now(),
	})

	d := NewDashboard(s, WithMessageReader(store))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/events", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDashboard_HandleEvents_MethodNotAllowed(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s, WithMessageReader(NewObservableMessageStore()))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/ddd/events", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestDashboard_HandleTraces_NoStore(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s)
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/traces", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestDashboard_HandleTraces_WithStore(t *testing.T) {
	s := NewStatsCollector()
	ts := trace.NewInMemoryTraceStore()
	ts.RecordSpan(context.Background(), &trace.Span{
		ID:        "s1",
		TraceID:   "t1",
		Type:      "command",
		Name:      "PlaceOrder",
		Status:    "success",
		StartedAt: time.Now(),
	})

	d := NewDashboard(s, WithDashboardTraceStore(ts))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/traces", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var traceIDs []string
	json.NewDecoder(w.Body).Decode(&traceIDs)
	if len(traceIDs) != 1 || traceIDs[0] != "t1" {
		t.Errorf("expected [t1], got %v", traceIDs)
	}
}

func TestDashboard_HandleTraces_WithTraceID(t *testing.T) {
	s := NewStatsCollector()
	ts := trace.NewInMemoryTraceStore()
	ts.RecordSpan(context.Background(), &trace.Span{
		ID:        "s1",
		TraceID:   "t1",
		Type:      "command",
		Name:      "PlaceOrder",
		Status:    "success",
		StartedAt: time.Now(),
	})

	d := NewDashboard(s, WithDashboardTraceStore(ts))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/traces?traceID=t1", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var spans []*trace.Span
	json.NewDecoder(w.Body).Decode(&spans)
	if len(spans) != 1 {
		t.Errorf("expected 1 span, got %d", len(spans))
	}
}

func TestDashboard_HandleHealth_Degraded(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s, WithDashboardTraceStore(&errTraceStore{}))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["status"] != "degraded" {
		t.Errorf("expected degraded, got %v", result["status"])
	}
}

func TestDashboard_HandleHealth_WithJobManager(t *testing.T) {
	s := NewStatsCollector()
	jm := &mockJobManager{jobs: []*jobcore.Job{}}
	d := NewDashboard(s, WithDashboardJobManager(jm))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	if result["status"] != "ok" {
		t.Errorf("expected ok, got %v", result["status"])
	}
	checks := result["checks"].(map[string]any)
	if checks["jobManager"] != "ok" {
		t.Errorf("expected jobManager ok, got %v", checks["jobManager"])
	}
}

func TestDashboard_HandleHealth_Disabled(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s, WithoutHealth())
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDashboard_HandleStats_TypeFilter(t *testing.T) {
	s := NewStatsCollector()
	s.RecordDuration("Command/PlaceOrder", 100*time.Millisecond)
	s.RecordDuration("Query/GetOrder", 50*time.Millisecond)

	d := NewDashboard(s)
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/stats?type=command", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var stats []OperationStats
	json.NewDecoder(w.Body).Decode(&stats)
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat, got %d", len(stats))
	}
	if stats[0].Name != "Command/PlaceOrder" {
		t.Errorf("expected Command/PlaceOrder, got %s", stats[0].Name)
	}
}

func TestDashboard_HandleStats_MethodNotAllowed(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s)
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/ddd/stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestDashboard_ParseMessageFilter_EdgeCases(t *testing.T) {
	s := NewStatsCollector()
	store := NewObservableMessageStore()
	now := time.Now()
	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "PlaceOrder", TraceID: "t1", CreatedAt: now, Duration: 100 * time.Millisecond,
	})

	d := NewDashboard(s, WithMessageReader(store), WithQueryLimit(50))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/commands?limit=abc&since=notanumber", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with invalid limit/since, got %d", w.Code)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/ddd/commands?limit=-1", nil)
	w2 := httptest.NewRecorder()
	mux.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("expected 200 with negative limit, got %d", w2.Code)
	}
}

func TestMatchStringFilter_StatusSuccess(t *testing.T) {
	store := NewObservableMessageStore()
	now := time.Now()
	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "PlaceOrder", CreatedAt: now, Duration: 50 * time.Millisecond,
	})
	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "CancelOrder", Error: "not found", CreatedAt: now, Duration: 30 * time.Millisecond,
	})

	success, _ := store.QueryCommands(context.Background(), MessageFilter{Status: "success"})
	if len(success) != 1 {
		t.Errorf("expected 1 success, got %d", len(success))
	}
	if success[0].CommandType != "PlaceOrder" {
		t.Errorf("expected PlaceOrder, got %s", success[0].CommandType)
	}
}

func TestDashboard_WithQueryLimit(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s, WithQueryLimit(5))
	if d.config.QueryLimit != 5 {
		t.Errorf("expected QueryLimit 5, got %d", d.config.QueryLimit)
	}
}

func TestDashboard_JobsWithManager(t *testing.T) {
	s := NewStatsCollector()
	jm := &mockJobManager{jobs: []*jobcore.Job{
		jobcore.NewJob("j1", "cmd1"),
	}}
	d := NewDashboard(s, WithDashboardJobManager(jm))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/jobs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	for _, key := range []string{"pending", "running", "completed", "failed", "cancelled"} {
		if _, ok := result[key]; !ok {
			t.Errorf("expected key %q in response", key)
		}
	}
}

func TestDashboard_JobsWithStatusFilter(t *testing.T) {
	s := NewStatsCollector()
	jm := &mockJobManager{jobs: []*jobcore.Job{
		jobcore.NewJob("j1", "cmd1"),
	}}
	d := NewDashboard(s, WithDashboardJobManager(jm))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/jobs?status=completed", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var jobs []*jobcore.Job
	json.NewDecoder(w.Body).Decode(&jobs)
	if len(jobs) != 1 {
		t.Errorf("expected 1 job, got %d", len(jobs))
	}
}

func TestDashboard_JobsMethodNotAllowed(t *testing.T) {
	s := NewStatsCollector()
	jm := &mockJobManager{}
	d := NewDashboard(s, WithDashboardJobManager(jm))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/ddd/jobs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestDashboard_JobsListByStatusError(t *testing.T) {
	s := NewStatsCollector()
	jm := &mockJobManager{err: fmt.Errorf("db error")}
	d := NewDashboard(s, WithDashboardJobManager(jm))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/jobs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestDashboard_TracesWithFilters(t *testing.T) {
	s := NewStatsCollector()
	ts := trace.NewInMemoryTraceStore()
	now := time.Now()
	ts.RecordSpan(context.Background(), &trace.Span{
		ID: "s1", TraceID: "t1", Type: "command", Name: "PlaceOrder", Status: "success", StartedAt: now,
	})
	ts.RecordSpan(context.Background(), &trace.Span{
		ID: "s2", TraceID: "t2", Type: "query", Name: "GetOrder", Status: "success", StartedAt: now.Add(-2 * time.Hour),
	})

	d := NewDashboard(s, WithDashboardTraceStore(ts))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	tests := []struct {
		name   string
		query  string
		expect int
	}{
		{"type filter", "type=command", 1},
		{"name filter", "name=Place", 1},
		{"start filter", fmt.Sprintf("start=%d", now.Add(-1*time.Hour).Unix()), 1},
		{"end filter", fmt.Sprintf("end=%d", now.Add(-1*time.Hour).Unix()), 1},
		{"no filter", "", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url := "/api/ddd/traces"
			if tt.query != "" {
				url += "?" + tt.query
			}
			req := httptest.NewRequest(http.MethodGet, url, nil)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d", w.Code)
			}
			var traceIDs []string
			json.NewDecoder(w.Body).Decode(&traceIDs)
			if len(traceIDs) != tt.expect {
				t.Errorf("expected %d traces, got %d", tt.expect, len(traceIDs))
			}
		})
	}
}

func TestDashboard_TracesMethodNotAllowed(t *testing.T) {
	s := NewStatsCollector()
	ts := trace.NewInMemoryTraceStore()
	d := NewDashboard(s, WithDashboardTraceStore(ts))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/ddd/traces", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestDashboard_TracesGetTraceError(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s, WithDashboardTraceStore(&errTraceStore{}))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/traces?traceID=nonexistent", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestDashboard_TracesListTracesError(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s, WithDashboardTraceStore(&errTraceStore{}))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/traces", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestDashboard_ParseMessageFilter_InvalidLimit(t *testing.T) {
	s := NewStatsCollector()
	store := NewObservableMessageStore()
	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "PlaceOrder", CreatedAt: time.Now(),
	})

	d := NewDashboard(s, WithMessageReader(store), WithQueryLimit(50))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/commands?limit=abc", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with invalid limit, got %d", w.Code)
	}

	var entries []builtin.CommandEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry (default limit used), got %d", len(entries))
	}
}

func TestDashboard_ParseMessageFilter_InvalidSince(t *testing.T) {
	s := NewStatsCollector()
	store := NewObservableMessageStore()
	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "PlaceOrder", CreatedAt: time.Now(),
	})

	d := NewDashboard(s, WithMessageReader(store), WithQueryLimit(50))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/commands?since=notanumber", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with invalid since, got %d", w.Code)
	}

	var entries []builtin.CommandEntry
	json.NewDecoder(w.Body).Decode(&entries)
	if len(entries) != 1 {
		t.Errorf("expected 1 entry (since ignored), got %d", len(entries))
	}
}

func TestDashboard_CommandsMethodNotAllowed(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s, WithMessageReader(NewObservableMessageStore()))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/ddd/commands", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestDashboard_QueriesMethodNotAllowed(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s, WithMessageReader(NewObservableMessageStore()))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/ddd/queries", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestDashboard_EventsMethodNotAllowed(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s, WithMessageReader(NewObservableMessageStore()))
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodPost, "/api/ddd/events", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", w.Code)
	}
}

func TestDashboard_AllDisabled(t *testing.T) {
	s := NewStatsCollector()
	d := NewDashboard(s,
		WithoutStats(),
		WithoutCommands(),
		WithoutQueries(),
		WithoutEvents(),
		WithoutJobs(),
		WithoutTraces(),
		WithoutHealth(),
	)
	mux := http.NewServeMux()
	d.RegisterRoutes(mux)

	endpoints := []string{"/api/ddd/stats", "/api/ddd/commands", "/api/ddd/queries",
		"/api/ddd/events", "/api/ddd/jobs", "/api/ddd/traces", "/api/ddd/health"}
	for _, ep := range endpoints {
		req := httptest.NewRequest(http.MethodGet, ep, nil)
		w := httptest.NewRecorder()
		mux.ServeHTTP(w, req)
		if w.Code != http.StatusNotFound {
			t.Errorf("expected 404 for %s, got %d", ep, w.Code)
		}
	}
}

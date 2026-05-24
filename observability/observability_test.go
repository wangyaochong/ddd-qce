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

func TestInMemoryMessageStore_Commands(t *testing.T) {
	store := NewInMemoryMessageStore(WithMaxSize(5))

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

func TestInMemoryMessageStore_MaxSize(t *testing.T) {
	store := NewInMemoryMessageStore(WithMaxSize(3))

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

func TestInMemoryMessageStore_Events(t *testing.T) {
	store := NewInMemoryMessageStore()

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

func TestInMemoryMessageStore_ImplementsMessageStore(t *testing.T) {
	var _ builtin.MessageStore = NewInMemoryMessageStore()
}

func TestInMemoryMessageStore_ImplementsReader(t *testing.T) {
	var _ MessageStoreReader = NewInMemoryMessageStore()
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
	store := NewInMemoryMessageStore()
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


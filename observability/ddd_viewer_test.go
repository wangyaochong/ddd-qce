package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/trace"
)

func TestDDDViewer_OverviewPage(t *testing.T) {
	v := NewDDDViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_overview", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "ddd_domain_events") {
		t.Error("expected table name in overview")
	}
}

func TestDDDViewer_CommandsPage(t *testing.T) {
	v := NewDDDViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_commands", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDDDViewer_QueriesPage(t *testing.T) {
	v := NewDDDViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_queries", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDDDViewer_EventsPage(t *testing.T) {
	v := NewDDDViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_events", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDDDViewer_StatsPage(t *testing.T) {
	v := NewDDDViewer()
	v.StatsCollector().RecordDuration("Command/PlaceOrder", 100*time.Millisecond)
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_stats", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "PlaceOrder") {
		t.Error("expected PlaceOrder in stats page")
	}
}

func TestDDDViewer_HealthEndpoint(t *testing.T) {
	v := NewDDDViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDDDViewer_Aspects(t *testing.T) {
	v := NewDDDViewer()
	aspects := v.Aspects()
	if len(aspects) != 2 {
		t.Fatalf("expected 2 aspects, got %d", len(aspects))
	}
}

func TestDDDViewer_CommandsWithData(t *testing.T) {
	v := NewDDDViewer()
	store := v.MessageStore()
	if store == nil {
		t.Fatal("expected message store to be set")
	}

	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "PlaceOrder",
		TraceID:     "t1",
		CreatedAt:   time.Now(),
		Duration:    100 * time.Millisecond,
	})

	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_commands", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "PlaceOrder") {
		t.Error("expected PlaceOrder in commands page")
	}
}

func TestDDDViewer_EventsWithData(t *testing.T) {
	v := NewDDDViewer()
	store := v.MessageStore()

	store.RecordEvent(context.Background(), &builtin.EventEntry{
		EventType:   "OrderPlaced",
		AggregateID: "agg1",
		TraceID:     "t1",
		CreatedAt:   time.Now(),
		Duration:    50 * time.Millisecond,
	})

	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_events", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "OrderPlaced") {
		t.Error("expected OrderPlaced in events page")
	}
}

func TestDDDViewer_SchemaPages(t *testing.T) {
	v := NewDDDViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_schema/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("schema list: expected 200, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_schema/ddd_domain_events", nil)
	w = httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("schema detail: expected 200, got %d", w.Code)
	}
}

func TestDDDViewer_CustomPrefix(t *testing.T) {
	v := NewDDDViewer(WithDDDViewerPrefix("/debug/ddd"))
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/debug/ddd/ddd_overview", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with custom prefix, got %d", w.Code)
	}
}

func TestDDDViewer_WithStatsCollector(t *testing.T) {
	sc := NewStatsCollector()
	v := NewDDDViewer(WithDDDViewerStatsCollector(sc))
	if v.StatsCollector() != sc {
		t.Error("expected shared stats collector")
	}
}

func TestDDDViewer_NewDDDViewer(t *testing.T) {
	v := NewDDDViewer()
	if v == nil {
		t.Fatal("expected non-nil viewer")
	}
	if v.config.Prefix != "/api/ddd" {
		t.Errorf("expected default prefix /api/ddd, got %s", v.config.Prefix)
	}
	if v.config.QueryLimit != 100 {
		t.Errorf("expected default QueryLimit 100, got %d", v.config.QueryLimit)
	}
	if v.backendType != "Memory" {
		t.Errorf("expected backendType Memory, got %s", v.backendType)
	}
	if v.msgStore == nil {
		t.Error("expected default msgStore")
	}
	if v.msgReader == nil {
		t.Error("expected default msgReader")
	}
	if v.schemaReader == nil {
		t.Error("expected default schemaReader")
	}
}

func TestDDDViewer_WithDDDViewerPgDB(t *testing.T) {
	v := NewDDDViewer(WithDDDViewerPgDB(nil))
	if v.backendType != "PostgreSQL" {
		t.Errorf("expected PostgreSQL, got %s", v.backendType)
	}
}

func TestDDDViewer_WithDDDViewerBaseURL(t *testing.T) {
	v := NewDDDViewer(WithDDDViewerBaseURL("http://example.com"))
	if v.baseURL != "http://example.com" {
		t.Errorf("expected http://example.com, got %s", v.baseURL)
	}
}

func TestDDDViewer_WithDDDViewerTraceStore(t *testing.T) {
	ts := trace.NewInMemoryTraceStore()
	v := NewDDDViewer(WithDDDViewerTraceStore(ts))
	if v.traceStore == nil {
		t.Error("expected traceStore to be set")
	}
}

func TestDDDViewer_WithDDDViewerJobManager(t *testing.T) {
	jm := &mockJobManager{}
	v := NewDDDViewer(WithDDDViewerJobManager(jm))
	if v.jobMgr == nil {
		t.Error("expected jobMgr to be set")
	}
}

func TestDDDViewer_WithDDDViewerMessageStore(t *testing.T) {
	store := NewObservableMessageStore()
	v := NewDDDViewer(WithDDDViewerMessageStore(store))
	if v.msgStore == nil {
		t.Error("expected msgStore to be set")
	}
}

func TestDDDViewer_WithDDDViewerMessageReader(t *testing.T) {
	store := NewObservableMessageStore()
	v := NewDDDViewer(WithDDDViewerMessageReader(store))
	if v.msgReader == nil {
		t.Error("expected msgReader to be set")
	}
}

func TestDDDViewer_WithDDDViewerSchemaReader(t *testing.T) {
	r := NewInMemorySchemaReader()
	v := NewDDDViewer(WithDDDViewerSchemaReader(r, "PostgreSQL"))
	if v.schemaReader == nil {
		t.Error("expected schemaReader to be set")
	}
	if v.backendType != "PostgreSQL" {
		t.Errorf("expected PostgreSQL, got %s", v.backendType)
	}
}

func TestDDDViewer_WithDDDViewerPrefix(t *testing.T) {
	v := NewDDDViewer(WithDDDViewerPrefix("/debug/ddd"))
	if v.config.Prefix != "/debug/ddd" {
		t.Errorf("expected /debug/ddd, got %s", v.config.Prefix)
	}
}

func TestDDDViewer_StatsCollectorGetter(t *testing.T) {
	sc := NewStatsCollector()
	v := NewDDDViewer(WithDDDViewerStatsCollector(sc))
	if v.StatsCollector() != sc {
		t.Error("expected StatsCollector to return same instance")
	}
}

func TestDDDViewer_MessageStoreGetter(t *testing.T) {
	store := NewObservableMessageStore()
	v := NewDDDViewer(WithDDDViewerMessageStore(store))
	if v.MessageStore() != store {
		t.Error("expected MessageStore to return same instance")
	}
}

func TestDDDViewer_Aspects_WithMessageStore(t *testing.T) {
	v := NewDDDViewer()
	aspects := v.Aspects()
	if len(aspects) != 2 {
		t.Fatalf("expected 2 aspects, got %d", len(aspects))
	}
}

func TestDDDViewer_handleOverviewWithStats(t *testing.T) {
	sc := NewStatsCollector()
	sc.RecordDuration("Command/PlaceOrder", 100*time.Millisecond)
	sc.RecordDuration("Query/GetOrder", 50*time.Millisecond)
	sc.RecordDuration("Event/OrderPlaced", 25*time.Millisecond)
	v := NewDDDViewer(WithDDDViewerStatsCollector(sc))
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_overview", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDDDViewer_handleSchemaTableList(t *testing.T) {
	v := NewDDDViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_schema/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "ddd_") {
		t.Error("expected body to contain ddd_ table names")
	}
}

func TestDDDViewer_handleSchemaTableDetail(t *testing.T) {
	v := NewDDDViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_schema/ddd_jobs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "ddd_jobs") {
		t.Error("expected body to contain ddd_jobs")
	}
}

func TestDDDViewer_handleSchemaTableDetail_NotFound(t *testing.T) {
	v := NewDDDViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_schema/invalid_table", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestDDDViewer_handleSchemaTableDetail_NonDDD(t *testing.T) {
	v := NewDDDViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_schema/users", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for non-ddd table, got %d", w.Code)
	}
}

func TestDDDViewer_handleQueriesWithData(t *testing.T) {
	v := NewDDDViewer()
	store := v.MessageStore()
	store.RecordQuery(context.Background(), &builtin.QueryEntry{
		QueryType: "GetOrder", TraceID: "t1", CreatedAt: time.Now(), Duration: 50 * time.Millisecond,
	})
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_queries", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "GetOrder") {
		t.Error("expected body to contain GetOrder")
	}
}

func TestDDDViewer_handleJobs(t *testing.T) {
	jm := &mockJobManager{jobs: []*jobcore.Job{
		jobcore.NewJob("j1", "cmd1"),
	}}
	v := NewDDDViewer(WithDDDViewerJobManager(jm))
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_jobs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "pending") && !strings.Contains(body, "completed") {
		t.Error("expected body to contain job status groups")
	}
}

func TestDDDViewer_handleJobs_NoManager(t *testing.T) {
	v := NewDDDViewer()
	v.jobMgr = nil
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_jobs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (unavailable page), got %d", w.Code)
	}
}

func TestDDDViewer_handleJobs_ListError(t *testing.T) {
	jm := &mockJobManager{err: fmt.Errorf("db error")}
	v := NewDDDViewer(WithDDDViewerJobManager(jm))
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_jobs", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestDDDViewer_handleTraces(t *testing.T) {
	ts := trace.NewInMemoryTraceStore()
	ts.RecordSpan(context.Background(), &trace.Span{
		ID: "s1", TraceID: "t1", Type: "command", Name: "PlaceOrder", Status: "success", StartedAt: time.Now(),
	})
	v := NewDDDViewer(WithDDDViewerTraceStore(ts))
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_traces", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "t1") {
		t.Error("expected body to contain trace ID t1")
	}
}

func TestDDDViewer_handleTraces_NoStore(t *testing.T) {
	v := NewDDDViewer()
	v.traceStore = nil
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_traces", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 (unavailable page), got %d", w.Code)
	}
}

func TestDDDViewer_handleTraces_ListError(t *testing.T) {
	v := NewDDDViewer(WithDDDViewerTraceStore(&errTraceStore{}))
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_traces", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", w.Code)
	}
}

func TestDDDViewer_handleTraces_WithTypeFilter(t *testing.T) {
	ts := trace.NewInMemoryTraceStore()
	ts.RecordSpan(context.Background(), &trace.Span{
		ID: "s1", TraceID: "t1", Type: "command", Name: "PlaceOrder", Status: "success", StartedAt: time.Now(),
	})
	v := NewDDDViewer(WithDDDViewerTraceStore(ts))
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_traces?type=command", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestDDDViewer_handleHealth_WithTraceStore(t *testing.T) {
	ts := trace.NewInMemoryTraceStore()
	v := NewDDDViewer(WithDDDViewerTraceStore(ts))
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	checks := result["checks"].(map[string]any)
	if checks["traceStore"] != "ok" {
		t.Errorf("expected traceStore ok, got %v", checks["traceStore"])
	}
}

func TestDDDViewer_handleHealth_WithJobManager(t *testing.T) {
	jm := &mockJobManager{jobs: []*jobcore.Job{}}
	v := NewDDDViewer(WithDDDViewerJobManager(jm))
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var result map[string]any
	json.NewDecoder(w.Body).Decode(&result)
	checks := result["checks"].(map[string]any)
	if checks["jobManager"] != "ok" {
		t.Errorf("expected jobManager ok, got %v", checks["jobManager"])
	}
}

func TestDDDViewer_handleHealth_Degraded(t *testing.T) {
	v := NewDDDViewer(WithDDDViewerTraceStore(&errTraceStore{}))
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_health", nil)
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

func TestDDDViewer_handleHealth_JobManagerError(t *testing.T) {
	jm := &mockJobManager{err: fmt.Errorf("db down")}
	v := NewDDDViewer(WithDDDViewerJobManager(jm))
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", w.Code)
	}
}

func TestDDDViewer_parseMessageFilter(t *testing.T) {
	v := NewDDDViewer()

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_commands?type=PlaceOrder&traceID=t1&aggregateID=A1&status=success&limit=10&since=1700000000", nil)
	filter := v.parseMessageFilter(req)

	if filter.Type != "PlaceOrder" {
		t.Errorf("expected type PlaceOrder, got %s", filter.Type)
	}
	if filter.TraceID != "t1" {
		t.Errorf("expected traceID t1, got %s", filter.TraceID)
	}
	if filter.AggregateID != "A1" {
		t.Errorf("expected aggregateID A1, got %s", filter.AggregateID)
	}
	if filter.Status != "success" {
		t.Errorf("expected status success, got %s", filter.Status)
	}
	if filter.Limit != 10 {
		t.Errorf("expected limit 10, got %d", filter.Limit)
	}
	if filter.Since.IsZero() {
		t.Error("expected since to be set")
	}
}

func TestDDDViewer_parseMessageFilter_InvalidLimit(t *testing.T) {
	v := NewDDDViewer()

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_commands?limit=abc", nil)
	filter := v.parseMessageFilter(req)

	if filter.Limit != 100 {
		t.Errorf("expected default limit 100, got %d", filter.Limit)
	}
}

func TestDDDViewer_parseMessageFilter_NegativeLimit(t *testing.T) {
	v := NewDDDViewer()

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_commands?limit=-5", nil)
	filter := v.parseMessageFilter(req)

	if filter.Limit != 100 {
		t.Errorf("expected default limit 100 for negative, got %d", filter.Limit)
	}
}

func TestDDDViewer_parseMessageFilter_InvalidSince(t *testing.T) {
	v := NewDDDViewer()

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_commands?since=notanumber", nil)
	filter := v.parseMessageFilter(req)

	if !filter.Since.IsZero() {
		t.Error("expected since to be zero for invalid input")
	}
}

func TestFormatData(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{"nil", nil, ""},
		{"empty string", "", ""},
		{"empty bytes", []byte{}, ""},
		{"empty raw message", json.RawMessage{}, ""},
		{"string json", `{"key":"value"}`, "{\n  \"key\": \"value\"\n}"},
		{"bytes json", []byte(`{"key":"value"}`), "{\n  \"key\": \"value\"\n}"},
		{"raw message", json.RawMessage(`{"a":1}`), "{\n  \"a\": 1\n}"},
		{"invalid json string", "not json", "not json"},
		{"invalid json bytes", []byte("not json"), "not json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatData(tt.input)
			if result != tt.expected {
				t.Errorf("formatData(%v) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestDDDViewer_CommandsPageReadableJSON(t *testing.T) {
	v := NewDDDViewer()
	store := v.MessageStore()

	store.RecordCommand(context.Background(), &builtin.CommandEntry{
		CommandType: "PlaceOrder",
		TraceID:     "t1",
		CommandData: json.RawMessage(`{"UserID":"user-001","Items":[{"ProductID":"laptop"}]}`),
		ResultData:  json.RawMessage(`{"OrderID":"ord-123","Success":true}`),
		CreatedAt:   time.Now(),
		Duration:    100 * time.Millisecond,
	})

	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_commands", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	body := w.Body.String()
	if strings.Contains(body, "[123 34") {
		t.Error("command data should not be rendered as byte array")
	}
	if !strings.Contains(body, "UserID") {
		t.Error("expected readable JSON with UserID key in command data")
	}
	if !strings.Contains(body, "OrderID") {
		t.Error("expected readable JSON with OrderID key in result data")
	}
}

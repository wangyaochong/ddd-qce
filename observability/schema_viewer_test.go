package observability

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInMemorySchemaReader_ListTables(t *testing.T) {
	r := NewInMemorySchemaReader()

	tables, err := r.ListTables(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 9 {
		t.Fatalf("expected 9 tables, got %d", len(tables))
	}

	names := make(map[string]bool, len(tables))
	for _, tbl := range tables {
		names[tbl.Name] = true
	}
	expectedNames := []string{
		"ddd_domain_events", "ddd_jobs", "ddd_job_execution_log", "ddd_spans",
		"ddd_aggregate_snapshots", "ddd_command_log", "ddd_query_log",
		"ddd_event_log", "ddd_event_handler_log",
	}
	for _, name := range expectedNames {
		if !names[name] {
			t.Errorf("missing table %s", name)
		}
	}
}

func TestInMemorySchemaReader_TableInfo(t *testing.T) {
	r := NewInMemorySchemaReader()

	tables, _ := r.ListTables(context.Background())
	for _, tbl := range tables {
		if tbl.Description == "" {
			t.Errorf("table %s missing description", tbl.Name)
		}
		if tbl.RowCount != -1 {
			t.Errorf("table %s: expected RowCount -1, got %d", tbl.Name, tbl.RowCount)
		}
		if tbl.DiskSize != -1 {
			t.Errorf("table %s: expected DiskSize -1, got %d", tbl.Name, tbl.DiskSize)
		}
		if tbl.LastUpdated != nil {
			t.Errorf("table %s: expected nil LastUpdated", tbl.Name)
		}
	}
}

func TestInMemorySchemaReader_GetTable(t *testing.T) {
	r := NewInMemorySchemaReader()

	detail, err := r.GetTable(context.Background(), "ddd_domain_events")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Name != "ddd_domain_events" {
		t.Errorf("expected ddd_domain_events, got %s", detail.Name)
	}
	if len(detail.Columns) != 8 {
		t.Errorf("expected 8 columns, got %d", len(detail.Columns))
	}
	if detail.Columns[0].Name != "id" {
		t.Errorf("expected first column 'id', got %s", detail.Columns[0].Name)
	}
	if !detail.Columns[0].IsPrimaryKey {
		t.Error("expected id to be primary key")
	}
	if len(detail.Indexes) < 1 {
		t.Errorf("expected at least 1 index, got %d", len(detail.Indexes))
	}
}

func TestInMemorySchemaReader_GetTableNotFound(t *testing.T) {
	r := NewInMemorySchemaReader()

	_, err := r.GetTable(context.Background(), "nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent table")
	}
}

func TestInMemorySchemaReader_ListRelations(t *testing.T) {
	r := NewInMemorySchemaReader()

	relations, err := r.ListRelations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(relations) != 2 {
		t.Fatalf("expected 2 relations, got %d", len(relations))
	}

	found := false
	for _, rel := range relations {
		if rel.FromTable == "ddd_job_execution_log" && rel.FromColumn == "job_id" &&
			rel.ToTable == "ddd_jobs" && rel.ToColumn == "id" {
			found = true
		}
	}
	if !found {
		t.Error("expected ddd_job_execution_log.job_id -> ddd_jobs.id relation")
	}
}

func TestSchemaViewer_TableListPage(t *testing.T) {
	v := NewSchemaViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_schema/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "ddd_domain_events") {
		t.Error("expected ddd_domain_events in response")
	}
	if !strings.Contains(body, "事件溯源事件存储") {
		t.Error("expected Chinese description in response")
	}
	if !strings.Contains(body, "ddd_schema/ddd_domain_events") {
		t.Error("expected ddd_ prefixed schema detail link")
	}
}

func TestSchemaViewer_TableDetailPage(t *testing.T) {
	v := NewSchemaViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_schema/ddd_domain_events", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "BIGSERIAL") {
		t.Error("expected BIGSERIAL type in response")
	}
	if !strings.Contains(body, "aggregate_id") {
		t.Error("expected aggregate_id column in response")
	}
}

func TestSchemaViewer_TableDetailNotFound(t *testing.T) {
	v := NewSchemaViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_schema/nonexistent_table", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestSchemaViewer_CustomPrefix(t *testing.T) {
	v := NewSchemaViewer(WithSchemaPrefix("/debug/schema"))
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/debug/schema/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with custom prefix, got %d", w.Code)
	}
}

func TestSchemaViewer_WithSchemaReader(t *testing.T) {
	r := NewInMemorySchemaReader()
	v := NewSchemaViewer(WithSchemaReader(r, "Test"))
	if v.backendType != "Test" {
		t.Errorf("expected backendType 'Test', got %s", v.backendType)
	}
}

func TestSchemaViewer_RelationsInTableList(t *testing.T) {
	v := NewSchemaViewer()
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/api/ddd/ddd_schema/", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	body := w.Body.String()
	if !strings.Contains(body, "ddd_job_execution_log") && !strings.Contains(body, "job_id") {
		t.Error("expected relation info in table list page")
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		input    int64
		expected string
	}{
		{-1, "N/A"},
		{0, "0 B"},
		{512, "512 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}
	for _, tt := range tests {
		result := formatSize(tt.input)
		if result != tt.expected {
			t.Errorf("formatSize(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestFormatCount(t *testing.T) {
	if formatCount(-1) != "N/A" {
		t.Errorf("expected N/A for -1, got %s", formatCount(-1))
	}
	if formatCount(0) != "0" {
		t.Errorf("expected 0, got %s", formatCount(0))
	}
	if formatCount(1234) != "1234" {
		t.Errorf("expected 1234, got %s", formatCount(1234))
	}
}

func TestFormatTime(t *testing.T) {
	if formatTime(nil) != "N/A" {
		t.Errorf("expected N/A for nil time")
	}
}

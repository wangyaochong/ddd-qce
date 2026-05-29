# DDD Schema Viewer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a framework-level SchemaViewer that lets any project using ddd-qce view DDD table structures, statistics, and relations via a browser with 2 lines of integration code.

**Architecture:** New files in `observability/` package — SchemaReader interface with InMemory (static) and PG (live) implementations, SchemaViewer struct with embed.FS HTML templates, RegisterRoutes + StartServer dual-mode, startup URL logging.

**Tech Stack:** Go 1.22+ (embed.FS, http.ServeMux method patterns), html/template, database/sql, information_schema queries

---

### Task 1: Schema Types

**Files:**
- Create: `observability/schema_types.go`

- [ ] **Step 1: Create schema_types.go with all data model types**

```go
package observability

import "time"

type TableInfo struct {
	Name        string
	Description string
	RowCount    int64
	LastUpdated *time.Time
	DiskSize    int64
	AvgRowSize  int64
}

type ColumnInfo struct {
	Name         string
	Type         string
	Nullable     bool
	DefaultValue string
	IsPrimaryKey bool
	Description  string
}

type IndexInfo struct {
	Name    string
	Columns []string
	Unique  bool
}

type TableDetail struct {
	TableInfo
	Columns []ColumnInfo
	Indexes []IndexInfo
}

type TableRelation struct {
	FromTable  string
	FromColumn string
	ToTable    string
	ToColumn   string
}
```

- [ ] **Step 2: Verify compilation**

Run: `cd /home/wyc/projects/ddd-qce && go build ./observability/...`
Expected: success

---

### Task 2: SchemaReader Interface + InMemory Implementation

**Files:**
- Create: `observability/schema_reader.go`

- [ ] **Step 1: Create schema_reader.go with SchemaReader interface and InMemorySchemaReader**

The InMemorySchemaReader hardcodes all 9 table definitions from `pg/migrate.go` with Chinese descriptions.

Key implementation details:
- All 9 tables from migrate.go with full column definitions, indexes, and descriptions
- `RowCount` returns -1, `LastUpdated` returns nil, `DiskSize` returns -1, `AvgRowSize` returns -1
- `ListRelations` returns hardcoded logical relations between tables

The hardcoded relations:
- `ddd_job_execution_log.job_id` → `ddd_jobs.id`
- `ddd_event_handler_log.event_log_id` → `ddd_event_log.id`

- [ ] **Step 2: Verify compilation**

Run: `cd /home/wyc/projects/ddd-qce && go build ./observability/...`
Expected: success

---

### Task 3: PgSchemaReader Implementation

**Files:**
- Create: `observability/pg/schema_reader.go`

- [ ] **Step 1: Create pg/schema_reader.go**

Uses `corepg.GetQuerier(ctx, s.db)` pattern.

`ListTables`:
```sql
SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_name LIKE 'ddd_%'
```
Then for each table: `SELECT count(*) FROM {table}` and `SELECT pg_total_relation_size('{table}')`.

For LastUpdated, try `SELECT max(created_at) FROM {table}` (only for tables that have `created_at`).

Map table names to Chinese descriptions using same map as InMemorySchemaReader.

`GetTable`:
```sql
SELECT column_name, data_type, is_nullable, column_default
FROM information_schema.columns WHERE table_schema = 'public' AND table_name = $1
ORDER BY ordinal_position
```
Plus pg_indexes for index info, plus count(*) for row count.

Primary key detection: query `information_schema.table_constraints` + `information_schema.key_column_usage` where constraint_type = 'PRIMARY KEY'.

`ListRelations`:
- Query foreign keys from `information_schema.table_constraints` + `key_column_usage`
- Append hardcoded logical relations same as InMemory

- [ ] **Step 2: Verify compilation**

Run: `cd /home/wyc/projects/ddd-qce && go build ./observability/pg/...`
Expected: success

---

### Task 4: HTML Templates

**Files:**
- Create: `observability/templates/schema_layout.html`
- Create: `observability/templates/schema_tables.html`
- Create: `observability/templates/schema_detail.html`

- [ ] **Step 1: Create schema_layout.html**

Self-contained layout with:
- Full CSS (light/dark mode via prefers-color-scheme)
- Nav bar with "DDD Schema Viewer" brand + back link
- header/footer define blocks like exampleapp pattern
- Styles for tables, cards, search input, relations diagram, badges

- [ ] **Step 2: Create schema_tables.html**

Table list page with:
- Search input with JS oninput handler for real-time filtering
- Sortable columns (click header to sort by name/rows/size) via JS
- Table showing: Name, Description, Rows, Last Updated, Disk Size
- Each row links to detail page
- Memory backend shows "N/A" for Rows/LastUpdated/DiskSize
- Relations section at bottom showing arrow-style relations

- [ ] **Step 3: Create schema_detail.html**

Single table detail page with:
- Back link to table list
- Table info header (name, description, rows, size)
- Columns table: Name, Type, Nullable, Default, PK, Description
- Indexes table: Name, Columns, Unique

---

### Task 5: SchemaViewer with embed.FS, Routes, Handlers, StartServer, Startup Log

**Files:**
- Create: `observability/schema_viewer.go`

- [ ] **Step 1: Create schema_viewer.go**

Key components:

```go
//go:embed templates/*.html
var templateFS embed.FS

type SchemaViewer struct {
	reader      SchemaReader
	config      SchemaViewerConfig
	tmpl        *template.Template
	backendType string // "PostgreSQL" or "Memory"
	baseURL     string
}

type SchemaViewerConfig struct {
	Prefix string
}

type SchemaViewerOption func(*SchemaViewer)

func WithSchemaPrefix(prefix string) SchemaViewerOption
func WithPgDB(db *sql.DB) SchemaViewerOption  // creates PgSchemaReader internally
func WithBaseURL(url string) SchemaViewerOption
```

Constructor:
- Default prefix: "/api/ddd/schema"
- Default backendType: "Memory"
- Default reader: NewInMemorySchemaReader()
- WithPgDB sets reader to pg.NewPgSchemaReader(db) and backendType to "PostgreSQL"
- Load templates via `template.ParseFS(templateFS, "templates/*.html")`

RegisterRoutes:
```go
func (v *SchemaViewer) RegisterRoutes(mux *http.ServeMux) {
	p := v.config.Prefix
	mux.HandleFunc("GET "+p+"/", v.handleTableList)
	mux.HandleFunc("GET "+p+"/{table}", v.handleTableDetail)

	tables, _ := v.reader.ListTables(context.Background())
	url := v.baseURL + p + "/"
	if v.baseURL == "" {
		url = p + "/"
	}
	log.Printf("[DDD Schema] Schema viewer registered at %s (%s, %d tables)", url, v.backendType, len(tables))
}
```

StartServer:
```go
func (v *SchemaViewer) StartServer(addr string) error {
	mux := http.NewServeMux()
	v.baseURL = "http://localhost" + addr
	v.RegisterRoutes(mux)

	tables, _ := v.reader.ListTables(context.Background())
	log.Printf("[DDD Schema] Schema viewer started at http://localhost%s%s/ (%s, %d tables)", addr, v.config.Prefix, v.backendType, len(tables))

	server := &http.Server{Addr: addr, Handler: mux}
	return server.ListenAndServe()
}
```

Handlers:
- handleTableList: calls ListTables + ListRelations, renders schema_tables.html
- handleTableDetail: gets {table} path value, validates ddd_ prefix, calls GetTable, renders schema_detail.html
- render helper method similar to exampleapp's render pattern

Template FuncMap: "formatSize" (bytes to human-readable), "formatTime" (nil-safe time formatting), "formatCount" (int64 with N/A for -1)

- [ ] **Step 2: Verify compilation**

Run: `cd /home/wyc/projects/ddd-qce && go build ./observability/...`
Expected: success

---

### Task 6: Unit Tests

**Files:**
- Create: `observability/schema_viewer_test.go`

- [ ] **Step 1: Write tests for InMemorySchemaReader**

Test cases:
- ListTables returns 9 tables
- Each table has correct Name and Description
- RowCount is -1 for all tables
- GetTable returns full column and index details for ddd_domain_events
- GetTable returns error for non-ddd_ table
- ListRelations returns expected relations

- [ ] **Step 2: Write tests for SchemaViewer HTTP handlers**

Test cases:
- RegisterRoutes with default prefix, hit GET /api/ddd/schema/ → 200
- RegisterRoutes with custom prefix, hit GET /debug/schema/ → 200
- Table detail page for ddd_domain_events → 200
- Table detail page for non-ddd_ table → 404
- StartServer creates working server (test with httptest if feasible, or just test RegisterRoutes)

- [ ] **Step 3: Run all tests**

Run: `cd /home/wyc/projects/ddd-qce && go test ./observability/... -v`
Expected: all pass

---

### Task 7: Integrate into exampleapp

**Files:**
- Modify: `exampleapp/infrastructure/wire.go`
- Modify: `exampleapp/interfaces/http/server.go`

- [ ] **Step 1: Add SchemaViewer field to AppContext**

In wire.go, add `SchemaViewer *observability.SchemaViewer` to AppContext struct.

- [ ] **Step 2: Create SchemaViewer in WireAppWithStore**

In wire.go, after backend creation, create SchemaViewer:
- If `store.DB != nil`: `observability.NewSchemaViewer(observability.WithPgDB(store.DB), observability.WithBaseURL("http://localhost:8080"))`
- Else: `observability.NewSchemaViewer()`

- [ ] **Step 3: Register SchemaViewer routes in server.go**

In server.go, after existing routes, add:
```go
app.SchemaViewer.RegisterRoutes(mux)
```

- [ ] **Step 4: Verify compilation and test**

Run: `cd /home/wyc/projects/ddd-qce && go build ./exampleapp/...`
Expected: success, and startup log shows [DDD Schema] line

---

### Task 8: Full verification

- [ ] **Step 1: Run all observability tests**

Run: `cd /home/wyc/projects/ddd-qce && go test ./observability/... -v`
Expected: all pass

- [ ] **Step 2: Run full project tests**

Run: `cd /home/wyc/projects/ddd-qce && go test ./...`
Expected: all pass

- [ ] **Step 3: Build exampleapp**

Run: `cd /home/wyc/projects/ddd-qce && go build ./exampleapp/...`
Expected: success

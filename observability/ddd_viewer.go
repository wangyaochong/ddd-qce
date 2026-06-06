package observability

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ddd-qce/core/aspect/builtin"
	"github.com/ddd-qce/core/cqrs/command"
	cqrsevent "github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/trace"
)

type Pagination struct {
	Page     int
	PageSize int
	Total    int
	Pages    int
	HasPrev  bool
	HasNext  bool
}

func newPagination(page, pageSize, total int) Pagination {
	if pageSize <= 0 {
		pageSize = 50
	}
	if page <= 0 {
		page = 1
	}
	pages := total / pageSize
	if total%pageSize > 0 {
		pages++
	}
	if pages == 0 {
		pages = 1
	}
	return Pagination{
		Page:     page,
		PageSize: pageSize,
		Total:    total,
		Pages:    pages,
		HasPrev:  page > 1,
		HasNext:  page < pages,
	}
}

type DDDViewer struct {
	statsCollector *StatsCollector
	msgStore       builtin.MessageStore
	msgReader      MessageStoreReader
	schemaReader   SchemaReader
	traceStore     trace.TraceStore
	jobMgr         jobcore.JobManager
	typeRegistry   *TypePrototypeRegistry
	config         DDDViewerConfig
	backendType    string
	baseURL        string
	tmpl           *template.Template
}

type DDDViewerConfig struct {
	Prefix     string
	QueryLimit int
}

type DDDViewerOption func(*DDDViewer)

func WithDDDViewerPrefix(prefix string) DDDViewerOption {
	return func(v *DDDViewer) { v.config.Prefix = prefix }
}

func WithDDDViewerPgDB(db *sql.DB) DDDViewerOption {
	return func(v *DDDViewer) {
		v.backendType = "PostgreSQL"
	}
}

func WithDDDViewerBaseURL(url string) DDDViewerOption {
	return func(v *DDDViewer) { v.baseURL = url }
}

func WithDDDViewerTraceStore(ts trace.TraceStore) DDDViewerOption {
	return func(v *DDDViewer) { v.traceStore = ts }
}

func WithDDDViewerJobManager(jm jobcore.JobManager) DDDViewerOption {
	return func(v *DDDViewer) { v.jobMgr = jm }
}

func WithDDDViewerMessageStore(ms builtin.MessageStore) DDDViewerOption {
	return func(v *DDDViewer) { v.msgStore = ms }
}

func WithDDDViewerMessageReader(r MessageStoreReader) DDDViewerOption {
	return func(v *DDDViewer) { v.msgReader = r }
}

func WithDDDViewerSchemaReader(r SchemaReader, backendType string) DDDViewerOption {
	return func(v *DDDViewer) {
		v.schemaReader = r
		v.backendType = backendType
	}
}

func WithDDDViewerStatsCollector(sc *StatsCollector) DDDViewerOption {
	return func(v *DDDViewer) { v.statsCollector = sc }
}

// EventSample describes an event type by name and a sample instance.
// The sample is used to extract field metadata and infer the domain via reflection.
type EventSample struct {
	Name   string
	Sample any
}

// WithDDDViewerExtraEvents registers additional event types that have no bus
// subscribers (fire-and-forget) and therefore cannot be auto-discovered by
// CollectFromBuses. Each EventSample provides a name and a struct sample
// for reflection-based field extraction and domain inference.
func WithDDDViewerExtraEvents(events []EventSample) DDDViewerOption {
	return func(v *DDDViewer) {
		for _, e := range events {
			v.typeRegistry.RegisterFromSample("event", e.Name, e.Sample, nil)
		}
	}
}

// NewDDDViewer creates a DDDViewer with the given buses and sample provider as
// mandatory parameters. This ensures the TypePrototypeRegistry is always populated,
// preventing DDD Viewer pages from silently showing "not available".
//
// The provider supplies struct samples for reflection-based field extraction.
// Pass nil to register type names only (without field details).
// Use WithDDDViewerExtraEvents to register fire-and-forget events that have
// no bus subscribers.
func NewDDDViewer(
	cmdBus command.CommandBus,
	queryBus query.QueryBus,
	evtBus cqrsevent.EventBus,
	provider BusTypeSampleProvider,
	opts ...DDDViewerOption,
) *DDDViewer {
	cfg := DDDViewerConfig{
		Prefix:     "/api/ddd",
		QueryLimit: 100,
	}
	v := &DDDViewer{
		typeRegistry:   NewTypePrototypeRegistryFromBuses(cmdBus, queryBus, evtBus, provider),
		config:         cfg,
		backendType:    "Memory",
		statsCollector: NewStatsCollector(),
	}

	for _, opt := range opts {
		opt(v)
	}

	if v.msgStore == nil && v.msgReader == nil {
		store := NewObservableMessageStore()
		v.msgStore = store
		v.msgReader = store
	}

	if v.schemaReader == nil {
		v.schemaReader = NewInMemorySchemaReader()
	}

	funcMap := template.FuncMap{
		"formatSize":  formatSize,
		"formatTime":  formatTime,
		"formatCount": formatCount,
		"formatDuration": func(d time.Duration) string {
			if d == 0 {
				return "-"
			}
			return d.Round(time.Microsecond).String()
		},
		"formatData": formatData,
		"shortID": func(s string) string {
			if len(s) > 12 {
				return s[:12] + "..."
			}
			return s
		},
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
	}

	tmpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"))
	v.tmpl = tmpl

	return v
}

func (v *DDDViewer) StatsCollector() *StatsCollector {
	return v.statsCollector
}

func (v *DDDViewer) MessageStore() builtin.MessageStore {
	return v.msgStore
}

func (v *DDDViewer) Aspects() []any {
	var aspects []any
	aspects = append(aspects, builtin.NewMetricsAspect(v.statsCollector))
	if v.msgStore != nil {
		aspects = append(aspects, builtin.NewPersistenceAspect(v.msgStore))
	}
	return aspects
}

func (v *DDDViewer) RegisterRoutes(mux *http.ServeMux) {
	p := v.config.Prefix

	mux.HandleFunc("GET "+p+"/ddd_overview", v.handleOverview)
	mux.HandleFunc("GET "+p+"/ddd_schema/", v.handleSchemaTableList)
	mux.HandleFunc("GET "+p+"/ddd_schema/{table}", v.handleSchemaTableDetail)
	mux.HandleFunc("GET "+p+"/ddd_commands", v.handleCommands)
	mux.HandleFunc("GET "+p+"/ddd_queries", v.handleQueries)
	mux.HandleFunc("GET "+p+"/ddd_events", v.handleEvents)
	mux.HandleFunc("GET "+p+"/ddd_domains", v.handleDomains)
	mux.HandleFunc("GET "+p+"/ddd_command_types", v.handleCommandTypes)
	mux.HandleFunc("GET "+p+"/ddd_query_types", v.handleQueryTypes)
	mux.HandleFunc("GET "+p+"/ddd_event_types", v.handleEventTypes)
	mux.HandleFunc("GET "+p+"/ddd_stats", v.handleStats)
	mux.HandleFunc("GET "+p+"/ddd_jobs", v.handleJobs)
	mux.HandleFunc("GET "+p+"/ddd_traces", v.handleTraces)
	mux.HandleFunc("GET "+p+"/ddd_health", v.handleHealth)

	tables, _ := v.schemaReader.ListTables(context.Background())
	base := v.baseURL + p
	if v.baseURL == "" {
		base = p
	}
	log.Printf("[DDD] Viewer registered at %s/ddd_overview (%s, %d tables)", base, v.backendType, len(tables))
	log.Printf("[DDD]   Overview:  %s/ddd_overview", base)
	log.Printf("[DDD]   Schema:    %s/ddd_schema/", base)
	log.Printf("[DDD]   Commands:  %s/ddd_commands", base)
	log.Printf("[DDD]   Queries:   %s/ddd_queries", base)
	log.Printf("[DDD]   Events:    %s/ddd_events", base)
	log.Printf("[DDD]   Cmd Types: %s/ddd_command_types", base)
	log.Printf("[DDD]   Qry Types: %s/ddd_query_types", base)
	log.Printf("[DDD]   Evt Types: %s/ddd_event_types", base)
	log.Printf("[DDD]   Domains:   %s/ddd_domains", base)
	log.Printf("[DDD]   Stats:     %s/ddd_stats", base)
	if v.jobMgr != nil {
		log.Printf("[DDD]   Jobs:      %s/ddd_jobs", base)
	}
	if v.traceStore != nil {
		log.Printf("[DDD]   Traces:    %s/ddd_traces", base)
	}
	log.Printf("[DDD]   Health:    %s/ddd_health", base)
}

func (v *DDDViewer) StartServer(addr string) error {
	v.baseURL = "http://localhost" + addr
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)
	server := &http.Server{Addr: addr, Handler: mux}
	return server.ListenAndServe()
}

func (v *DDDViewer) handleOverview(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tables, _ := v.schemaReader.ListTables(ctx)
	relations, _ := v.schemaReader.ListRelations(ctx)
	allStats := v.statsCollector.GetAllStats()

	cmdCount, queryCount, eventCount := 0, 0, 0
	for _, s := range allStats {
		switch s.Type {
		case "command":
			cmdCount += int(s.Count)
		case "query":
			queryCount += int(s.Count)
		case "event":
			eventCount += int(s.Count)
		}
	}

	v.render(w, "ddd_overview", map[string]any{
		"Tables":        tables,
		"Relations":     relations,
		"AllStats":      allStats,
		"CmdCount":      cmdCount,
		"QueryCount":    queryCount,
		"EventCount":    eventCount,
		"Prefix":        v.config.Prefix,
		"HasMsgReader":  v.msgReader != nil,
		"HasTraceStore": v.traceStore != nil,
		"HasJobMgr":     v.jobMgr != nil,
	})
}

func (v *DDDViewer) handleSchemaTableList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tables, err := v.schemaReader.ListTables(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	relations, _ := v.schemaReader.ListRelations(ctx)

	v.render(w, "schema_tables", map[string]any{
		"Tables":    tables,
		"Relations": relations,
		"Prefix":    v.config.Prefix,
	})
}

func (v *DDDViewer) handleSchemaTableDetail(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("table")
	if !strings.HasPrefix(tableName, "ddd_") {
		http.Error(w, "table not found", http.StatusNotFound)
		return
	}

	ctx := r.Context()
	detail, err := v.schemaReader.GetTable(ctx, tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	v.render(w, "schema_detail", map[string]any{
		"Table":  detail,
		"Prefix": v.config.Prefix,
	})
}

func (v *DDDViewer) handleCommands(w http.ResponseWriter, r *http.Request) {
	if v.msgReader == nil {
		v.render(w, "ddd_unavailable", map[string]any{"Feature": "Commands", "Prefix": v.config.Prefix})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := v.parseMessageFilter(r)
	result, err := v.msgReader.QueryCommands(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize := 50
	if ps := r.URL.Query().Get("pageSize"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n > 0 && n <= 500 {
			pageSize = n
		}
	}

	v.render(w, "ddd_commands", map[string]any{
		"Entries":    result.Items,
		"Filter":     filter,
		"Prefix":     v.config.Prefix,
		"Pagination": newPagination(page, pageSize, result.Total),
	})
}

func (v *DDDViewer) handleQueries(w http.ResponseWriter, r *http.Request) {
	if v.msgReader == nil {
		v.render(w, "ddd_unavailable", map[string]any{"Feature": "Queries", "Prefix": v.config.Prefix})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := v.parseMessageFilter(r)
	result, err := v.msgReader.QueryQueries(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize := 50
	if ps := r.URL.Query().Get("pageSize"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n > 0 && n <= 500 {
			pageSize = n
		}
	}

	v.render(w, "ddd_queries", map[string]any{
		"Entries":    result.Items,
		"Filter":     filter,
		"Prefix":     v.config.Prefix,
		"Pagination": newPagination(page, pageSize, result.Total),
	})
}

func (v *DDDViewer) handleEvents(w http.ResponseWriter, r *http.Request) {
	if v.msgReader == nil {
		v.render(w, "ddd_unavailable", map[string]any{"Feature": "Events", "Prefix": v.config.Prefix})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := v.parseMessageFilter(r)
	result, err := v.msgReader.QueryEvents(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}
	pageSize := 50
	if ps := r.URL.Query().Get("pageSize"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n > 0 && n <= 500 {
			pageSize = n
		}
	}

	v.render(w, "ddd_events", map[string]any{
		"Entries":    result.Items,
		"Filter":     filter,
		"Prefix":     v.config.Prefix,
		"Pagination": newPagination(page, pageSize, result.Total),
	})
}

func (v *DDDViewer) handleStats(w http.ResponseWriter, r *http.Request) {
	allStats := v.statsCollector.GetAllStats()
	v.render(w, "ddd_stats", map[string]any{
		"AllStats": allStats,
		"Prefix":   v.config.Prefix,
	})
}

func (v *DDDViewer) handleJobs(w http.ResponseWriter, r *http.Request) {
	if v.jobMgr == nil {
		v.render(w, "ddd_unavailable", map[string]any{"Feature": "Jobs", "Prefix": v.config.Prefix})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	statuses := []jobcore.JobStatus{
		jobcore.JobStatusPending, jobcore.JobStatusRunning,
		jobcore.JobStatusCompleted, jobcore.JobStatusFailed, jobcore.JobStatusCancelled,
	}

	type jobGroup struct {
		Status string
		Jobs   []*jobcore.Job
	}
	var groups []jobGroup
	for _, s := range statuses {
		jobs, err := v.jobMgr.ListByStatus(ctx, s)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		groups = append(groups, jobGroup{Status: string(s), Jobs: jobs})
	}

	v.render(w, "ddd_jobs", map[string]any{
		"Groups": groups,
		"Prefix": v.config.Prefix,
	})
}

func (v *DDDViewer) handleTraces(w http.ResponseWriter, r *http.Request) {
	if v.traceStore == nil {
		v.render(w, "ddd_unavailable", map[string]any{"Feature": "Traces", "Prefix": v.config.Prefix})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	filter := trace.TraceFilter{
		Status: trace.SpanStatus(r.URL.Query().Get("status")),
	}
	if typ := r.URL.Query().Get("type"); typ != "" {
		filter.Type = trace.SpanType(typ)
	}

	traceIDs, err := v.traceStore.ListTraces(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageSize := 50
	if ps := r.URL.Query().Get("pageSize"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n > 0 && n <= 500 {
			pageSize = n
		}
	}
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}

	total := len(traceIDs)
	offset := (page - 1) * pageSize
	if offset > total {
		offset = total
	}
	end := offset + pageSize
	if end > total {
		end = total
	}
	pagedIDs := traceIDs[offset:end]

	type SpanView struct {
		ID       string
		TraceID  string
		ParentID string
		Type     string
		Name     string
		Status   string
		Error    string
		Duration time.Duration
	}
	type TraceView struct {
		TraceID string
		Spans   []SpanView
	}

	var traces []TraceView
	for _, tid := range pagedIDs {
		spans, err := v.traceStore.GetTrace(ctx, tid)
		if err != nil {
			continue
		}
		var sv []SpanView
		for _, s := range spans {
			sv = append(sv, SpanView{
				ID: s.ID, TraceID: s.TraceID, ParentID: s.ParentID,
				Type: string(s.Type), Name: s.Name, Status: string(s.Status),
				Error: s.Error, Duration: s.Duration,
			})
		}
		traces = append(traces, TraceView{TraceID: tid, Spans: sv})
	}

	v.render(w, "ddd_traces", map[string]any{
		"Traces":       traces,
		"FilterStatus": filter.Status,
		"FilterType":   filter.Type,
		"Prefix":       v.config.Prefix,
		"Pagination":   newPagination(page, pageSize, total),
	})
}

func (v *DDDViewer) handleHealth(w http.ResponseWriter, r *http.Request) {
	checks := make(map[string]string)
	overall := "ok"

	if v.traceStore != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		_, err := v.traceStore.ListTraces(ctx, trace.TraceFilter{})
		cancel()
		if err != nil {
			checks["traceStore"] = fmt.Sprintf("error: %v", err)
			overall = "degraded"
		} else {
			checks["traceStore"] = "ok"
		}
	}

	if v.jobMgr != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		_, err := v.jobMgr.ListByStatus(ctx, jobcore.JobStatusCompleted)
		cancel()
		if err != nil {
			checks["jobManager"] = fmt.Sprintf("error: %v", err)
			overall = "degraded"
		} else {
			checks["jobManager"] = "ok"
		}
	}

	checks["statsCollector"] = "ok"
	if v.msgReader != nil {
		checks["messageStore"] = "ok"
	}

	code := http.StatusOK
	if overall == "degraded" {
		code = http.StatusServiceUnavailable
	}

	writeJSON(w, code, map[string]any{
		"status": overall,
		"checks": checks,
	})
}

func (v *DDDViewer) handleCommandTypes(w http.ResponseWriter, r *http.Request) {
	if v.typeRegistry == nil {
		v.render(w, "ddd_unavailable", map[string]any{"Feature": "Command Types", "Prefix": v.config.Prefix})
		return
	}
	commands := v.typeRegistry.ListByCategory("command")
	count := v.typeRegistry.CountByCategory("command")
	v.render(w, "ddd_command_types", map[string]any{
		"Commands": commands,
		"Count":    count,
		"Prefix":   v.config.Prefix,
	})
}

func (v *DDDViewer) handleQueryTypes(w http.ResponseWriter, r *http.Request) {
	if v.typeRegistry == nil {
		v.render(w, "ddd_unavailable", map[string]any{"Feature": "Query Types", "Prefix": v.config.Prefix})
		return
	}
	queries := v.typeRegistry.ListByCategory("query")
	count := v.typeRegistry.CountByCategory("query")
	v.render(w, "ddd_query_types", map[string]any{
		"Queries": queries,
		"Count":   count,
		"Prefix":  v.config.Prefix,
	})
}

func (v *DDDViewer) handleEventTypes(w http.ResponseWriter, r *http.Request) {
	if v.typeRegistry == nil {
		v.render(w, "ddd_unavailable", map[string]any{"Feature": "Event Types", "Prefix": v.config.Prefix})
		return
	}
	events := v.typeRegistry.ListByCategory("event")
	count := v.typeRegistry.CountByCategory("event")
	v.render(w, "ddd_event_types", map[string]any{
		"Events": events,
		"Count":  count,
		"Prefix": v.config.Prefix,
	})
}

func (v *DDDViewer) handleDomains(w http.ResponseWriter, r *http.Request) {
	if v.typeRegistry == nil {
		v.render(w, "ddd_unavailable", map[string]any{"Feature": "Domains", "Prefix": v.config.Prefix})
		return
	}

	domains := v.typeRegistry.ListDomains()
	selectedDomain := r.URL.Query().Get("domain")
	if selectedDomain == "" && len(domains) > 0 {
		selectedDomain = domains[0]
	}

	var domainInfo *DomainInfo
	var stats DomainStats
	var entries []DomainEntry

	if selectedDomain != "" {
		domainInfo = v.typeRegistry.GetDomainInfo(selectedDomain)

		stats = DomainStats{}
		if v.statsCollector != nil {
			allStats := v.statsCollector.GetAllStats()
			for _, s := range allStats {
				sDomain := v.typeRegistry.GetTypeDomain(s.Name)
				if sDomain != selectedDomain {
					continue
				}
				switch s.Type {
				case "command":
					stats.CommandCount += int(s.Count)
					stats.CommandErrors += int(s.ErrorCount)
				case "query":
					stats.QueryCount += int(s.Count)
					stats.QueryErrors += int(s.ErrorCount)
				case "event":
					stats.EventCount += int(s.Count)
					stats.EventErrors += int(s.ErrorCount)
				}
			}
		}

		if v.msgReader != nil {
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			pageSize := 50
			if ps := r.URL.Query().Get("pageSize"); ps != "" {
				if n, err := strconv.Atoi(ps); err == nil && n > 0 && n <= 500 {
					pageSize = n
				}
			}
			page := 1
			if p := r.URL.Query().Get("page"); p != "" {
				if n, err := strconv.Atoi(p); err == nil && n > 0 {
					page = n
				}
			}
			filter := MessageFilter{Limit: pageSize, Offset: (page - 1) * pageSize}

			cmdResult, _ := v.msgReader.QueryCommands(ctx, filter)
			for _, e := range cmdResult.Items {
				domain := v.typeRegistry.GetTypeDomain(e.CommandType)
				if domain == selectedDomain {
					entries = append(entries, DomainEntry{
						Type: e.CommandType, Category: "command",
						Status: "ok", Duration: e.Duration.String(),
						CreatedAt: e.CreatedAt.Format("2006-01-02 15:04:05"),
						Data: formatData(e.CommandData), Result: formatData(e.ResultData),
					})
					if e.Error != "" {
						entries[len(entries)-1].Status = "error"
						entries[len(entries)-1].Error = e.Error
					}
				}
			}

			qryResult, _ := v.msgReader.QueryQueries(ctx, filter)
			for _, e := range qryResult.Items {
				domain := v.typeRegistry.GetTypeDomain(e.QueryType)
				if domain == selectedDomain {
					entries = append(entries, DomainEntry{
						Type: e.QueryType, Category: "query",
						Status: "ok", Duration: e.Duration.String(),
						CreatedAt: e.CreatedAt.Format("2006-01-02 15:04:05"),
						Data: formatData(e.QueryData), Result: formatData(e.ResultData),
					})
					if e.Error != "" {
						entries[len(entries)-1].Status = "error"
						entries[len(entries)-1].Error = e.Error
					}
				}
			}

			evtResult, _ := v.msgReader.QueryEvents(ctx, filter)
			for _, e := range evtResult.Items {
				domain := v.typeRegistry.GetTypeDomain(e.EventType)
				if domain == selectedDomain {
					entries = append(entries, DomainEntry{
						Type: e.EventType, Category: "event",
						Status: "ok", Duration: e.Duration.String(),
						CreatedAt: e.CreatedAt.Format("2006-01-02 15:04:05"),
						Data: formatData(e.EventData),
					})
					if e.Error != "" {
						entries[len(entries)-1].Status = "error"
						entries[len(entries)-1].Error = e.Error
					}
				}
			}

			v.render(w, "ddd_domains", map[string]any{
				"Domains":        domains,
				"SelectedDomain": selectedDomain,
				"Stats":          stats,
				"DomainInfo":     domainInfo,
				"Entries":        entries,
				"Prefix":         v.config.Prefix,
				"Pagination":     newPagination(page, pageSize, len(entries)),
			})
			return
		}
	}

	v.render(w, "ddd_domains", map[string]any{
		"Domains":        domains,
		"SelectedDomain": selectedDomain,
		"Stats":          stats,
		"DomainInfo":     domainInfo,
		"Entries":        entries,
		"Prefix":         v.config.Prefix,
	})
}

func (v *DDDViewer) parseMessageFilter(r *http.Request) MessageFilter {
	pageSize := 50
	if ps := r.URL.Query().Get("pageSize"); ps != "" {
		if n, err := strconv.Atoi(ps); err == nil && n > 0 && n <= 500 {
			pageSize = n
		}
	}
	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}

	filter := MessageFilter{
		Type:        r.URL.Query().Get("type"),
		TraceID:     r.URL.Query().Get("traceID"),
		AggregateID: r.URL.Query().Get("aggregateID"),
		Status:      r.URL.Query().Get("status"),
		Limit:       pageSize,
		Offset:      (page - 1) * pageSize,
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if ts, err := strconv.ParseInt(since, 10, 64); err == nil {
			filter.Since = time.Unix(ts, 0)
		}
	}
	return filter
}

func (v *DDDViewer) render(w http.ResponseWriter, name string, data map[string]any) {
	data["Page"] = name
	data["Title"] = "DDD Viewer"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := v.tmpl.ExecuteTemplate(w, name, data); err != nil {
		fmt.Fprintf(w, "template error: %v", err)
	}
}

func formatData(v any) string {
	switch d := v.(type) {
	case json.RawMessage:
		if len(d) == 0 {
			return ""
		}
		var pretty bytes.Buffer
		if json.Indent(&pretty, d, "", "  ") == nil {
			return pretty.String()
		}
		return string(d)
	case []byte:
		if len(d) == 0 {
			return ""
		}
		var pretty bytes.Buffer
		if json.Indent(&pretty, d, "", "  ") == nil {
			return pretty.String()
		}
		return string(d)
	case string:
		if d == "" {
			return ""
		}
		var pretty bytes.Buffer
		if json.Indent(&pretty, []byte(d), "", "  ") == nil {
			return pretty.String()
		}
		return d
	case nil:
		return ""
	default:
		b, err := json.MarshalIndent(d, "", "  ")
		if err != nil {
			return fmt.Sprintf("%v", d)
		}
		return string(b)
	}
}

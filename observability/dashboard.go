package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/trace"
)

type Dashboard struct {
	stats      *StatsCollector
	traceStore trace.TraceStore
	jobMgr     jobcore.JobManager
	config     DashboardConfig
}

func NewDashboard(stats *StatsCollector, opts ...DashboardOption) *Dashboard {
	cfg := defaultConfig()
	d := &Dashboard{
		stats:  stats,
		config: cfg,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

type DashboardOption func(*Dashboard)

func WithPrefix(prefix string) DashboardOption {
	return func(d *Dashboard) { d.config.Prefix = prefix }
}

func WithoutStats() DashboardOption {
	return func(d *Dashboard) { d.config.EnableStats = false }
}

func WithoutCommands() DashboardOption {
	return func(d *Dashboard) { d.config.EnableCommands = false }
}

func WithoutQueries() DashboardOption {
	return func(d *Dashboard) { d.config.EnableQueries = false }
}

func WithoutEvents() DashboardOption {
	return func(d *Dashboard) { d.config.EnableEvents = false }
}

func WithoutJobs() DashboardOption {
	return func(d *Dashboard) { d.config.EnableJobs = false }
}

func WithoutTraces() DashboardOption {
	return func(d *Dashboard) { d.config.EnableTraces = false }
}

func WithoutHealth() DashboardOption {
	return func(d *Dashboard) { d.config.EnableHealth = false }
}

func WithQueryLimit(n int) DashboardOption {
	return func(d *Dashboard) { d.config.QueryLimit = n }
}

func WithDashboardTraceStore(ts trace.TraceStore) DashboardOption {
	return func(d *Dashboard) { d.traceStore = ts }
}

func WithDashboardJobManager(jm jobcore.JobManager) DashboardOption {
	return func(d *Dashboard) { d.jobMgr = jm }
}

type DashboardConfig struct {
	Prefix         string
	EnableStats    bool
	EnableCommands bool
	EnableQueries  bool
	EnableEvents   bool
	EnableJobs     bool
	EnableTraces   bool
	EnableHealth   bool
	QueryLimit     int
}

func defaultConfig() DashboardConfig {
	return DashboardConfig{
		Prefix:         "/api/ddd",
		EnableStats:    true,
		EnableCommands: true,
		EnableQueries:  true,
		EnableEvents:   true,
		EnableJobs:     true,
		EnableTraces:   true,
		EnableHealth:   true,
		QueryLimit:     100,
	}
}

func (d *Dashboard) RegisterRoutes(mux *http.ServeMux) {
	p := d.config.Prefix

	if d.config.EnableStats {
		mux.HandleFunc(p+"/stats", d.handleStats)
	}
	if d.config.EnableCommands {
		mux.HandleFunc(p+"/commands", d.handleNotImplemented)
	}
	if d.config.EnableQueries {
		mux.HandleFunc(p+"/queries", d.handleNotImplemented)
	}
	if d.config.EnableEvents {
		mux.HandleFunc(p+"/events", d.handleNotImplemented)
	}
	if d.config.EnableJobs {
		mux.HandleFunc(p+"/jobs", d.handleJobs)
	}
	if d.config.EnableTraces {
		mux.HandleFunc(p+"/traces", d.handleTraces)
	}
	if d.config.EnableHealth {
		mux.HandleFunc(p+"/health", d.handleHealth)
	}
}

func (d *Dashboard) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := r.URL.Query().Get("name")
	opType := r.URL.Query().Get("type")

	var result any
	if name != "" {
		stats, ok := d.stats.GetStats(name)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "operation not found"})
			return
		}
		result = stats
	} else if opType != "" {
		result = d.stats.GetStatsByType(opType)
	} else {
		result = d.stats.GetAllStats()
	}
	writeJSON(w, http.StatusOK, result)
}

func (d *Dashboard) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.jobMgr == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "job manager not configured"})
		return
	}

	statusStr := r.URL.Query().Get("status")
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if statusStr != "" {
		jobs, err := d.jobMgr.ListByStatus(ctx, jobcore.JobStatus(statusStr))
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, jobs)
		return
	}

	type jobList struct {
		Pending   []*jobcore.Job `json:"pending"`
		Running   []*jobcore.Job `json:"running"`
		Completed []*jobcore.Job `json:"completed"`
		Failed    []*jobcore.Job `json:"failed"`
		Cancelled []*jobcore.Job `json:"cancelled"`
	}

	var list jobList
	statuses := []jobcore.JobStatus{
		jobcore.JobStatusPending,
		jobcore.JobStatusRunning,
		jobcore.JobStatusCompleted,
		jobcore.JobStatusFailed,
		jobcore.JobStatusCancelled,
	}
	dests := [5]*[]*jobcore.Job{&list.Pending, &list.Running, &list.Completed, &list.Failed, &list.Cancelled}

	for i, status := range statuses {
		jobs, err := d.jobMgr.ListByStatus(ctx, status)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		*dests[i] = jobs
	}
	writeJSON(w, http.StatusOK, list)
}

func (d *Dashboard) handleTraces(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.traceStore == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "trace store not configured"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	traceID := r.URL.Query().Get("traceID")
	if traceID != "" {
		spans, err := d.traceStore.GetTrace(ctx, traceID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, spans)
		return
	}

	filter := trace.TraceFilter{
		Status: r.URL.Query().Get("status"),
	}

	if typ := r.URL.Query().Get("type"); typ != "" {
		filter.Type = typ
	}
	if name := r.URL.Query().Get("name"); name != "" {
		filter.NameContains = name
	}

	if startStr := r.URL.Query().Get("start"); startStr != "" {
		if ts, err := strconv.ParseInt(startStr, 10, 64); err == nil {
			filter.StartTime = time.Unix(ts, 0)
		}
	}
	if endStr := r.URL.Query().Get("end"); endStr != "" {
		if ts, err := strconv.ParseInt(endStr, 10, 64); err == nil {
			filter.EndTime = time.Unix(ts, 0)
		}
	}

	traceIDs, err := d.traceStore.ListTraces(ctx, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, traceIDs)
}

func (d *Dashboard) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	checks := make(map[string]string)
	overall := "ok"

	if d.traceStore != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		_, err := d.traceStore.ListTraces(ctx, trace.TraceFilter{})
		cancel()
		if err != nil {
			checks["traceStore"] = fmt.Sprintf("error: %v", err)
			overall = "degraded"
		} else {
			checks["traceStore"] = "ok"
		}
	}

	if d.jobMgr != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		_, err := d.jobMgr.ListByStatus(ctx, jobcore.JobStatusCompleted)
		cancel()
		if err != nil {
			checks["jobManager"] = fmt.Sprintf("error: %v", err)
			overall = "degraded"
		} else {
			checks["jobManager"] = "ok"
		}
	}

	checks["statsCollector"] = "ok"

	code := http.StatusOK
	if overall == "degraded" {
		code = http.StatusServiceUnavailable
	}

	writeJSON(w, code, map[string]any{
		"status": overall,
		"checks": checks,
	})
}

func (d *Dashboard) handleNotImplemented(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusNotImplemented, map[string]string{
		"error": "endpoint requires MessageStoreReader, coming in P1",
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

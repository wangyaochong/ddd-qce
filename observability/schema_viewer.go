package observability

import (
	"context"
	"embed"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed templates/*.html
var templateFS embed.FS

type SchemaViewer struct {
	reader      SchemaReader
	config      SchemaViewerConfig
	tmpl        *template.Template
	backendType string
	baseURL     string
}

type SchemaViewerConfig struct {
	Prefix string
}

type SchemaViewerOption func(*SchemaViewer)

func WithSchemaPrefix(prefix string) SchemaViewerOption {
	return func(v *SchemaViewer) { v.config.Prefix = prefix }
}

func WithSchemaReader(reader SchemaReader, backendType string) SchemaViewerOption {
	return func(v *SchemaViewer) {
		v.reader = reader
		v.backendType = backendType
	}
}

func WithBaseURL(url string) SchemaViewerOption {
	return func(v *SchemaViewer) { v.baseURL = url }
}

func NewSchemaViewer(opts ...SchemaViewerOption) *SchemaViewer {
	cfg := SchemaViewerConfig{Prefix: "/api/ddd/ddd_schema"}
	v := &SchemaViewer{
		config:      cfg,
		backendType: "Memory",
	}

	for _, opt := range opts {
		opt(v)
	}

	if v.reader == nil {
		v.reader = NewInMemorySchemaReader()
	}

	funcMap := template.FuncMap{
		"formatSize":     formatSize,
		"formatTime":     formatTime,
		"formatCount":    formatCount,
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
	}

	tmpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html"))
	v.tmpl = tmpl

	return v
}

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

func (v *SchemaViewer) StartServer(addr string) error {
	v.baseURL = "http://localhost" + addr
	mux := http.NewServeMux()
	v.RegisterRoutes(mux)

	tables, _ := v.reader.ListTables(context.Background())
	log.Printf("[DDD Schema] Schema viewer started at http://localhost%s%s/ (%s, %d tables)", addr, v.config.Prefix, v.backendType, len(tables))

	server := &http.Server{Addr: addr, Handler: mux}
	return server.ListenAndServe()
}

func (v *SchemaViewer) handleTableList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tables, err := v.reader.ListTables(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	relations, _ := v.reader.ListRelations(ctx)

	v.render(w, "schema_tables", map[string]any{
		"Tables":    tables,
		"Relations": relations,
		"Prefix":    v.config.Prefix,
	})
}

func (v *SchemaViewer) handleTableDetail(w http.ResponseWriter, r *http.Request) {
	tableName := r.PathValue("table")
	if !strings.HasPrefix(tableName, "ddd_") {
		http.Error(w, "table not found", http.StatusNotFound)
		return
	}

	ctx := r.Context()
	detail, err := v.reader.GetTable(ctx, tableName)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	v.render(w, "schema_detail", map[string]any{
		"Table":  detail,
		"Prefix": v.config.Prefix,
	})
}

func (v *SchemaViewer) render(w http.ResponseWriter, name string, data map[string]any) {
	data["Page"] = name
	data["Title"] = "DDD Schema Viewer"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := v.tmpl.ExecuteTemplate(w, name, data); err != nil {
		fmt.Fprintf(w, "template error: %v", err)
	}
}

func formatSize(bytes int64) string {
	if bytes < 0 {
		return "N/A"
	}
	if bytes == 0 {
		return "0 B"
	}
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatTime(t *time.Time) string {
	if t == nil {
		return "N/A"
	}
	return t.Format("2006-01-02 15:04:05")
}

func formatCount(n int64) string {
	if n < 0 {
		return "N/A"
	}
	return fmt.Sprintf("%d", n)
}

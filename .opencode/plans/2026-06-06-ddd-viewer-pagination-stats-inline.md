# DDD Viewer Pagination + Stats Inline Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add pagination (default 50/page, Prev/Next controls) to all data pages and inline type counts as badges on Types pages.

**Architecture:** Replace `[]XxxEntry` return values with `QueryResult[T]{Items, Total}` on `MessageStoreReader` interface. Handler calculates `Pagination` struct from Total/PageSize. Templates render Prev/Next controls preserving existing filter params.

**Tech Stack:** Go 1.26, html/template, net/http

---

## File Structure

| File | Responsibility |
|------|---------------|
| `observability/message_reader.go` | `QueryResult[T]`, updated `MessageStoreReader` interface, updated `ObservableMessageStore` |
| `observability/pg/message_reader.go` | Updated `PgMessageStoreReader` with COUNT+SELECT |
| `observability/ddd_viewer.go` | `Pagination` struct, `parsePagination`, updated handlers with Offset/Limit, `Pagination` to templates |
| `observability/dashboard.go` | Updated handlers for new return type |
| `observability/templates/ddd_layout.html` | `.pagination`, `.count-badge` CSS |
| `observability/templates/ddd_commands.html` | Pagination controls |
| `observability/templates/ddd_queries.html` | Pagination controls |
| `observability/templates/ddd_events.html` | Pagination controls |
| `observability/templates/ddd_traces.html` | Pagination controls |
| `observability/templates/ddd_domains.html` | Pagination controls + inline stats |
| `observability/templates/ddd_command_types.html` | Inline badge, remove `.stats` |
| `observability/templates/ddd_query_types.html` | Inline badge, remove `.stats` |
| `observability/templates/ddd_event_types.html` | Inline badge, remove `.stats` |

---

### Task 1: Define QueryResult and update MessageStoreReader interface

**Files:**
- Modify: `observability/message_reader.go:10-24`

- [ ] **Step 1: Add QueryResult type and update interface**

Add `QueryResult` generic type above `MessageFilter`. Update `MessageStoreReader` interface to return `QueryResult` instead of slices.

```go
type QueryResult[T any] struct {
	Items []T
	Total int
}

type MessageStoreReader interface {
	QueryCommands(ctx context.Context, filter MessageFilter) (QueryResult[builtin.CommandEntry], error)
	QueryQueries(ctx context.Context, filter MessageFilter) (QueryResult[builtin.QueryEntry], error)
	QueryEvents(ctx context.Context, filter MessageFilter) (QueryResult[builtin.EventEntry], error)
}
```

- [ ] **Step 2: Update ObservableMessageStore.QueryCommands**

```go
func (s *ObservableMessageStore) QueryCommands(_ context.Context, filter MessageFilter) (QueryResult[builtin.CommandEntry], error) {
	commands := s.inner.GetCommands()

	var filtered []builtin.CommandEntry
	for i := len(commands) - 1; i >= 0; i-- {
		e := commands[i]
		if !matchCommandFilter(e, filter) {
			continue
		}
		filtered = append(filtered, e)
	}
	total := len(filtered)

	var items []builtin.CommandEntry
	offset := filter.Offset
	if offset > total {
		offset = total
	}
	remaining := filtered[offset:]
	limit := filter.Limit
	if limit <= 0 || limit > len(remaining) {
		limit = len(remaining)
	}
	items = remaining[:limit]

	return QueryResult[builtin.CommandEntry]{Items: items, Total: total}, nil
}
```

- [ ] **Step 3: Update ObservableMessageStore.QueryQueries**

Same pattern as QueryCommands but for queries:

```go
func (s *ObservableMessageStore) QueryQueries(_ context.Context, filter MessageFilter) (QueryResult[builtin.QueryEntry], error) {
	queries := s.inner.GetQueries()

	var filtered []builtin.QueryEntry
	for i := len(queries) - 1; i >= 0; i-- {
		e := queries[i]
		if !matchQueryFilter(e, filter) {
			continue
		}
		filtered = append(filtered, e)
	}
	total := len(filtered)

	var items []builtin.QueryEntry
	offset := filter.Offset
	if offset > total {
		offset = total
	}
	remaining := filtered[offset:]
	limit := filter.Limit
	if limit <= 0 || limit > len(remaining) {
		limit = len(remaining)
	}
	items = remaining[:limit]

	return QueryResult[builtin.QueryEntry]{Items: items, Total: total}, nil
}
```

- [ ] **Step 4: Update ObservableMessageStore.QueryEvents**

```go
func (s *ObservableMessageStore) QueryEvents(_ context.Context, filter MessageFilter) (QueryResult[builtin.EventEntry], error) {
	events := s.inner.GetEvents()

	var filtered []builtin.EventEntry
	for i := len(events) - 1; i >= 0; i-- {
		e := events[i]
		if !matchEventFilter(e, filter) {
			continue
		}
		filtered = append(filtered, e)
	}
	total := len(filtered)

	var items []builtin.EventEntry
	offset := filter.Offset
	if offset > total {
		offset = total
	}
	remaining := filtered[offset:]
	limit := filter.Limit
	if limit <= 0 || limit > len(remaining) {
		limit = len(remaining)
	}
	items = remaining[:limit]

	return QueryResult[builtin.EventEntry]{Items: items, Total: total}, nil
}
```

- [ ] **Step 5: Run tests to verify compilation**

Run: `go build ./observability/...`
Expected: Build errors in callers (dashboard.go, ddd_viewer.go, tests) — this is expected, fixed in later tasks.

---

### Task 2: Update PgMessageStoreReader

**Files:**
- Modify: `observability/pg/message_reader.go:24-177`

- [ ] **Step 1: Update QueryCommands to return QueryResult with COUNT**

Add a COUNT query before the SELECT query. Apply same WHERE clause to both. Use Offset from filter.

```go
func (r *PgMessageStoreReader) QueryCommands(ctx context.Context, filter observability.MessageFilter) (observability.QueryResult[builtin.CommandEntry], error) {
	q := corepg.GetQuerier(ctx, r.db)

	where, args := buildWhereClause([]wherePart{
		{"command_type", filter.Type},
		{"trace_id", filter.TraceID},
	}, filter.Status, filter.Since)

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM ddd_command_log%s`, where)
	var total int
	if err := q.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return observability.QueryResult[builtin.CommandEntry]{}, fmt.Errorf("count commands: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset

	queryArgs := append(args, limit, offset)
	query := fmt.Sprintf(
		`SELECT trace_id, span_id, command_type, command_data, result_type, result_data, error, duration_ns, created_at
		 FROM ddd_command_log%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(queryArgs)-1, len(queryArgs),
	)

	rows, err := q.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return observability.QueryResult[builtin.CommandEntry]{}, fmt.Errorf("query commands: %w", err)
	}
	defer rows.Close()

	var items []builtin.CommandEntry
	for rows.Next() {
		var e builtin.CommandEntry
		var traceID, spanID, resultType sql.NullString
		var commandData, resultData json.RawMessage
		var errMsg sql.NullString
		var durationNs sql.NullInt64
		if err := rows.Scan(&traceID, &spanID, &e.CommandType, &commandData, &resultType, &resultData, &errMsg, &durationNs, &e.CreatedAt); err != nil {
			return observability.QueryResult[builtin.CommandEntry]{}, fmt.Errorf("scan command: %w", err)
		}
		e.TraceID = traceID.String
		e.SpanID = spanID.String
		e.CommandData = commandData
		e.ResultType = resultType.String
		e.ResultData = resultData
		e.Error = errMsg.String
		if durationNs.Valid {
			e.Duration = time.Duration(durationNs.Int64)
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return observability.QueryResult[builtin.CommandEntry]{}, err
	}
	return observability.QueryResult[builtin.CommandEntry]{Items: items, Total: total}, nil
}
```

- [ ] **Step 2: Update QueryQueries to return QueryResult with COUNT**

Same pattern: COUNT query + OFFSET support.

```go
func (r *PgMessageStoreReader) QueryQueries(ctx context.Context, filter observability.MessageFilter) (observability.QueryResult[builtin.QueryEntry], error) {
	q := corepg.GetQuerier(ctx, r.db)

	where, args := buildWhereClause([]wherePart{
		{"query_type", filter.Type},
		{"trace_id", filter.TraceID},
	}, filter.Status, filter.Since)

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM ddd_query_log%s`, where)
	var total int
	if err := q.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return observability.QueryResult[builtin.QueryEntry]{}, fmt.Errorf("count queries: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset

	queryArgs := append(args, limit, offset)
	query := fmt.Sprintf(
		`SELECT trace_id, span_id, query_type, query_data, result_type, result_data, error, duration_ns, created_at
		 FROM ddd_query_log%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(queryArgs)-1, len(queryArgs),
	)

	rows, err := q.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return observability.QueryResult[builtin.QueryEntry]{}, fmt.Errorf("query queries: %w", err)
	}
	defer rows.Close()

	var items []builtin.QueryEntry
	for rows.Next() {
		var e builtin.QueryEntry
		var traceID, spanID, resultType sql.NullString
		var queryData json.RawMessage
		var resultData json.RawMessage
		var errMsg sql.NullString
		var durationNs sql.NullInt64
		if err := rows.Scan(&traceID, &spanID, &e.QueryType, &queryData, &resultType, &resultData, &errMsg, &durationNs, &e.CreatedAt); err != nil {
			return observability.QueryResult[builtin.QueryEntry]{}, fmt.Errorf("scan query: %w", err)
		}
		e.TraceID = traceID.String
		e.SpanID = spanID.String
		e.QueryData = queryData
		e.ResultType = resultType.String
		e.ResultData = resultData
		e.Error = errMsg.String
		if durationNs.Valid {
			e.Duration = time.Duration(durationNs.Int64)
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return observability.QueryResult[builtin.QueryEntry]{}, err
	}
	return observability.QueryResult[builtin.QueryEntry]{Items: items, Total: total}, nil
}
```

- [ ] **Step 3: Update QueryEvents to return QueryResult with COUNT**

Same pattern with aggregate_id filter:

```go
func (r *PgMessageStoreReader) QueryEvents(ctx context.Context, filter observability.MessageFilter) (observability.QueryResult[builtin.EventEntry], error) {
	q := corepg.GetQuerier(ctx, r.db)

	parts := []wherePart{
		{"event_type", filter.Type},
		{"trace_id", filter.TraceID},
	}
	if filter.AggregateID != "" {
		parts = append(parts, wherePart{"aggregate_id", filter.AggregateID})
	}
	where, args := buildWhereClause(parts, filter.Status, filter.Since)

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM ddd_event_log%s`, where)
	var total int
	if err := q.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return observability.QueryResult[builtin.EventEntry]{}, fmt.Errorf("count events: %w", err)
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	offset := filter.Offset

	queryArgs := append(args, limit, offset)
	query := fmt.Sprintf(
		`SELECT trace_id, span_id, aggregate_id, event_type, event_data, handler_count, error, duration_ns, created_at
		 FROM ddd_event_log%s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		where, len(queryArgs)-1, len(queryArgs),
	)

	rows, err := q.QueryContext(ctx, query, queryArgs...)
	if err != nil {
		return observability.QueryResult[builtin.EventEntry]{}, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var items []builtin.EventEntry
	for rows.Next() {
		var e builtin.EventEntry
		var traceID, spanID, aggregateID sql.NullString
		var eventData json.RawMessage
		var handlerCount sql.NullInt64
		var errMsg sql.NullString
		var durationNs sql.NullInt64
		if err := rows.Scan(&traceID, &spanID, &aggregateID, &e.EventType, &eventData, &handlerCount, &errMsg, &durationNs, &e.CreatedAt); err != nil {
			return observability.QueryResult[builtin.EventEntry]{}, fmt.Errorf("scan event: %w", err)
		}
		e.TraceID = traceID.String
		e.SpanID = spanID.String
		e.AggregateID = aggregateID.String
		e.EventData = eventData
		if handlerCount.Valid {
			e.HandlerCount = int(handlerCount.Int64)
		}
		e.Error = errMsg.String
		if durationNs.Valid {
			e.Duration = time.Duration(durationNs.Int64)
		}
		items = append(items, e)
	}
	if err := rows.Err(); err != nil {
		return observability.QueryResult[builtin.EventEntry]{}, err
	}
	return observability.QueryResult[builtin.EventEntry]{Items: items, Total: total}, nil
}
```

- [ ] **Step 4: Run build to verify pg package compiles**

Run: `go build ./observability/pg/...`
Expected: Build succeeds (pg package is self-contained after these changes).

---

### Task 3: Add Pagination struct and update DDDViewer handlers

**Files:**
- Modify: `observability/ddd_viewer.go`

- [ ] **Step 1: Add Pagination struct**

Add near top of file, after imports:

```go
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
```

- [ ] **Step 2: Update parseMessageFilter to support page/pageSize**

Replace the existing `parseMessageFilter` method:

```go
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

	limit := pageSize
	offset := (page - 1) * pageSize

	filter := MessageFilter{
		Type:        r.URL.Query().Get("type"),
		TraceID:     r.URL.Query().Get("traceID"),
		AggregateID: r.URL.Query().Get("aggregateID"),
		Status:      r.URL.Query().Get("status"),
		Limit:       limit,
		Offset:      offset,
	}
	if since := r.URL.Query().Get("since"); since != "" {
		if ts, err := strconv.ParseInt(since, 10, 64); err == nil {
			filter.Since = time.Unix(ts, 0)
		}
	}
	return filter
}
```

- [ ] **Step 3: Update handleCommands**

```go
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
```

- [ ] **Step 4: Update handleQueries**

```go
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
```

- [ ] **Step 5: Update handleEvents**

```go
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
```

- [ ] **Step 6: Update handleTraces**

```go
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
```

- [ ] **Step 7: Update handleDomains — entries pagination**

In `handleDomains`, change the `filter := MessageFilter{Limit: 50}` to use page/pageSize/offset:

```go
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
```

- [ ] **Step 8: Run build to verify**

Run: `go build ./observability/...`
Expected: Still may have dashboard.go issues — fixed in next task.

---

### Task 4: Update Dashboard handlers

**Files:**
- Modify: `observability/dashboard.go:322-410`

- [ ] **Step 1: Update handleCommands**

```go
func (d *Dashboard) handleCommands(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.msgReader == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "message store reader not configured"})
		return
	}

	filter := d.parseMessageFilter(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result, err := d.msgReader.QueryCommands(ctx, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 2: Update handleQueries**

```go
func (d *Dashboard) handleQueries(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.msgReader == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "message store reader not configured"})
		return
	}

	filter := d.parseMessageFilter(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result, err := d.msgReader.QueryQueries(ctx, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 3: Update handleEvents**

```go
func (d *Dashboard) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if d.msgReader == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "message store reader not configured"})
		return
	}

	filter := d.parseMessageFilter(r)
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	result, err := d.msgReader.QueryEvents(ctx, filter)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, result)
}
```

- [ ] **Step 4: Run build to verify full compilation**

Run: `go build ./observability/...`
Expected: BUILD SUCCESS

---

### Task 5: Update existing tests for QueryResult return type

**Files:**
- Modify: `observability/observability_test.go`
- Modify: `observability/ddd_viewer_test.go`

- [ ] **Step 1: Update observability_test.go — all QueryXxx callers**

Every call like `entries, err := store.QueryCommands(ctx, filter)` must change to `result, err := store.QueryCommands(ctx, filter)` and references to `entries` become `result.Items`. Key locations (line numbers approximate):

- `TestObservableMessageStore_QueryCommands`: `result.Items` instead of `entries`
- `TestObservableMessageStore_QueryQueries_VariousFilters`: same pattern
- `TestObservableMessageStore_QueryEvents_TraceID`: same pattern
- `TestObservableMessageStore_QueryCommands_TraceID`: same pattern
- `TestObservableMessageStore_QueryCommands_Since`: same pattern
- `TestObservableMessageStore_QueryCommands_StatusFilter`: same pattern
- All other test functions referencing `QueryCommands`, `QueryQueries`, `QueryEvents`

Pattern for each:

```go
// Before:
entries, err := store.QueryCommands(ctx, filter)
// After:
result, err := store.QueryCommands(ctx, filter)
entries := result.Items
```

This allows minimal diff for assertions that check `len(entries)`, `entries[0].X`, etc.

- [ ] **Step 2: Update ddd_viewer_test.go — parseMessageFilter default limit**

Change test at `TestDDDViewer_parseMessageFilter` line ~665: `filter.Limit` default changes from 100 to 50. Update:

```go
if filter.Limit != 50 {
    t.Errorf("expected default limit 50, got %d", filter.Limit)
}
```

Also update `TestDDDViewer_parseMessageFilter_InvalidLimit` and `TestDDDViewer_parseMessageFilter_NegativeLimit` similarly.

Update `TestDDDViewer_NewDDDViewer`: default QueryLimit remains 100 in config (that's a different thing — it's the DDDViewerConfig default). The parseMessageFilter now defaults to 50 for page size regardless of QueryLimit. The QueryLimit config field may no longer be needed — but keep it for backward compat for now.

- [ ] **Step 3: Run tests**

Run: `go test ./observability/... -count=1`
Expected: ALL PASS

---

### Task 6: Add pagination CSS and template controls

**Files:**
- Modify: `observability/templates/ddd_layout.html`
- Modify: `observability/templates/ddd_commands.html`
- Modify: `observability/templates/ddd_queries.html`
- Modify: `observability/templates/ddd_events.html`
- Modify: `observability/templates/ddd_traces.html`
- Modify: `observability/templates/ddd_domains.html`

- [ ] **Step 1: Add CSS to ddd_layout.html**

Add after the existing `.execution-table` styles (before `</style>`):

```css
.pagination { display: flex; align-items: center; gap: 1rem; margin-top: 1rem; padding: 0.5rem 0; }
.pagination .btn { min-width: 80px; text-align: center; }
.pagination .page-info { color: #666; font-size: 0.9rem; }
.pagination .disabled { opacity: 0.4; cursor: default; pointer-events: none; }
.count-badge { display: inline-block; background: #1a1a2e; color: #fff; border-radius: 12px; padding: 0.15rem 0.6rem; font-size: 0.85rem; font-weight: 600; vertical-align: middle; margin-left: 0.5rem; }
```

- [ ] **Step 2: Add pagination controls to ddd_commands.html**

Add after the closing `</div>` of the card, before `{{template "ddd_footer"}}`:

```html
{{if .Pagination}}
<div class="pagination">
    {{if .Pagination.HasPrev}}
    <a href="?page={{math .Pagination.Page 1}}&pageSize={{.Pagination.PageSize}}&type={{.Filter.Type}}&traceID={{.Filter.TraceID}}&status={{.Filter.Status}}" class="btn btn-primary btn-sm">← Prev</a>
    {{else}}
    <span class="btn btn-primary btn-sm disabled">← Prev</span>
    {{end}}
    <span class="page-info">Page {{.Pagination.Page}} of {{.Pagination.Pages}} ({{.Pagination.Total}} total)</span>
    {{if .Pagination.HasNext}}
    <a href="?page={{math .Pagination.Page 1}}&pageSize={{.Pagination.PageSize}}&type={{.Filter.Type}}&traceID={{.Filter.TraceID}}&status={{.Filter.Status}}" class="btn btn-primary btn-sm">Next →</a>
    {{else}}
    <span class="btn btn-primary btn-sm disabled">Next →</span>
    {{end}}
</div>
{{end}}
```

Note: Need to add `math` template func (sub) in ddd_viewer.go. See Task 8.

- [ ] **Step 3: Add same pagination controls to ddd_queries.html**

Same template snippet, with filter params matching queries (type, traceID, status).

- [ ] **Step 4: Add same pagination controls to ddd_events.html**

Same template snippet, with filter params matching events (type, aggregateID, traceID, status).

- [ ] **Step 5: Add pagination controls to ddd_traces.html**

Same template snippet, with filter params matching traces (type, status).

- [ ] **Step 6: Add pagination controls to ddd_domains.html**

After the "Recent Executions" section, add pagination controls with domain filter param.

---

### Task 7: Inline stats badge on Types pages

**Files:**
- Modify: `observability/templates/ddd_command_types.html`
- Modify: `observability/templates/ddd_query_types.html`
- Modify: `observability/templates/ddd_event_types.html`

- [ ] **Step 1: Update ddd_command_types.html**

Replace:
```html
<h1>Command Types</h1>

<div class="stats">
    <div class="stat"><div class="number">{{.Count}}</div><div class="label">Commands</div></div>
</div>
```

With:
```html
<h1>Command Types <span class="count-badge">{{.Count}}</span></h1>
```

- [ ] **Step 2: Update ddd_query_types.html**

Replace:
```html
<h1>Query Types</h1>

<div class="stats">
    <div class="stat"><div class="number">{{.Count}}</div><div class="label">Queries</div></div>
</div>
```

With:
```html
<h1>Query Types <span class="count-badge">{{.Count}}</span></h1>
```

- [ ] **Step 3: Update ddd_event_types.html**

Replace:
```html
<h1>Event Types</h1>

<div class="stats">
    <div class="stat"><div class="number">{{.Count}}</div><div class="label">Events</div></div>
</div>
```

With:
```html
<h1>Event Types <span class="count-badge">{{.Count}}</span></h1>
```

---

### Task 8: Add template helper funcs for pagination

**Files:**
- Modify: `observability/ddd_viewer.go:145-162`

- [ ] **Step 1: Add `sub` and `add` template funcs**

Add to the `funcMap` in `NewDDDViewer`:

```go
"add": func(a, b int) int { return a + b },
"sub": func(a, b int) int { return a - b },
```

- [ ] **Step 2: Update pagination template links to use sub/add**

In templates, use `{{sub .Pagination.Page 1}}` for Prev and `{{add .Pagination.Page 1}}` for Next:

```html
<a href="?page={{sub .Pagination.Page 1}}&pageSize={{.Pagination.PageSize}}..." class="btn btn-primary btn-sm">← Prev</a>
```
```html
<a href="?page={{add .Pagination.Page 1}}&pageSize={{.Pagination.PageSize}}..." class="btn btn-primary btn-sm">Next →</a>
```

---

### Task 9: Update exampleapp tests

**Files:**
- Modify: `exampleapp/interfaces/http/e2e_test.go`

- [ ] **Step 1: Find and update any references to QueryXxx or Dashboard API responses**

If e2e tests call dashboard JSON endpoints (`/api/ddd/commands` etc.), the response shape changes from `[]entry` to `{"Items":[...],"Total":N}`. Update assertions accordingly.

- [ ] **Step 2: Run exampleapp tests**

Run: `go test ./exampleapp/... -count=1 -run TestE2E`
Expected: PASS (or note any failures requiring fixes)

---

### Task 10: Final verification

- [ ] **Step 1: Run full test suite**

Run: `go test ./... -count=1`
Expected: ALL PASS

- [ ] **Step 2: Run linter**

Run: `go vet ./...`
Expected: No issues

- [ ] **Step 3: Commit**

```bash
git add -A
git commit -m "feat: add pagination to DDD Viewer pages and inline type count badges

- MessageStoreReader returns QueryResult[T]{Items, Total} instead of []T
- Default page size 50, Prev/Next pagination controls on all data pages
- Types pages show count as inline badge instead of separate stats block
- PgMessageStoreReader uses COUNT + OFFSET for efficient pagination
- Breaking change: MessageStoreReader interface signatures updated"
```

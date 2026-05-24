package http

import (
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	commandmemory "github.com/ddd-qce/core/cqrs/command/memory"
	querymemory "github.com/ddd-qce/core/cqrs/query/memory"
	jobcore "github.com/ddd-qce/core/job/core"
	"github.com/ddd-qce/core/trace"
	"github.com/ddd-qce/exampleapp/application"
	"github.com/ddd-qce/exampleapp/domain"
	"github.com/ddd-qce/exampleapp/infrastructure"
)

type Handler struct {
	app  *infrastructure.AppContext
	tmpl *template.Template
}

func NewHandler(app *infrastructure.AppContext) *Handler {
	fm := template.FuncMap{"add": func(a, b int) int { return a + b }}
	tmpl := template.Must(template.New("").Funcs(fm).ParseGlob(resolveTemplatePath()))
	return &Handler{app: app, tmpl: tmpl}
}

func resolveTemplatePath() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine source file path")
	}
	templateDir := filepath.Join(filepath.Dir(thisFile), "templates")
	pattern := filepath.Join(templateDir, "*.html")
	if matches, err := filepath.Glob(pattern); err == nil && len(matches) > 0 {
		return pattern
	}
	panic(fmt.Sprintf("no templates found at %s", pattern))
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ordersResult, err := querymemory.Dispatch[*application.ListOrdersQuery, *application.ListOrdersResult](ctx, h.app.QueryBus, &application.ListOrdersQuery{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	inventoryResult, err := querymemory.Dispatch[*application.GetInventoryQuery, *application.GetInventoryResult](ctx, h.app.QueryBus, &application.GetInventoryQuery{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pending, paid, shipped, cancelled := 0, 0, 0, 0
	for _, o := range ordersResult.Orders {
		switch o.Status {
		case "pending":
			pending++
		case "paid":
			paid++
		case "shipped":
			shipped++
		case "cancelled":
			cancelled++
		}
	}

	traceIDs, err := h.app.Backend.TraceStore.ListTraces(ctx, trace.TraceFilter{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.render(w, "dashboard", map[string]interface{}{
		"TotalOrders":  len(ordersResult.Orders),
		"Pending":      pending,
		"Paid":         paid,
		"Shipped":      shipped,
		"Cancelled":    cancelled,
		"Products":     inventoryResult.Products,
		"RecentOrders": ordersResult.Orders,
		"TraceCount":   len(traceIDs),
	})
}

func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result, err := querymemory.Dispatch[*application.ListOrdersQuery, *application.ListOrdersResult](ctx, h.app.QueryBus, &application.ListOrdersQuery{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "orders", map[string]interface{}{"Orders": result.Orders})
}

func (h *Handler) NewOrderForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	inventoryResult, err := querymemory.Dispatch[*application.GetInventoryQuery, *application.GetInventoryResult](ctx, h.app.QueryBus, &application.GetInventoryQuery{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "order_new", map[string]interface{}{"Products": inventoryResult.Products})
}

func (h *Handler) PlaceOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	userID := r.FormValue("user_id")
	var items []application.ItemInput
	products := h.app.Inventory.GetAll()
	for _, p := range products {
		qtyStr := r.FormValue("qty_" + p.ID)
		if qty, err := strconv.Atoi(qtyStr); err == nil && qty > 0 {
			items = append(items, application.ItemInput{
				ProductID:   p.ID,
				ProductName: p.Name,
				Price:       p.Price,
				Quantity:    qty,
			})
		}
	}

	if len(items) == 0 {
		http.Error(w, "at least one item required", http.StatusBadRequest)
		return
	}

	result, err := commandmemory.Dispatch[*application.PlaceOrderCommand, *application.PlaceOrderResult](ctx, h.app.CmdBus, &application.PlaceOrderCommand{
		UserID: userID,
		Items:  items,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/orders/"+result.OrderID, http.StatusSeeOther)
}

func (h *Handler) OrderDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := r.PathValue("id")
	result, err := querymemory.Dispatch[*application.GetOrderQuery, *application.GetOrderResult](ctx, h.app.QueryBus, &application.GetOrderQuery{OrderID: orderID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	h.render(w, "order_detail", map[string]interface{}{"Order": result})
}

func (h *Handler) ConfirmPayment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := r.PathValue("id")
	_, err := commandmemory.Dispatch[*application.ConfirmPaymentCommand, *application.ConfirmPaymentResult](ctx, h.app.CmdBus, &application.ConfirmPaymentCommand{OrderID: orderID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/orders/"+orderID, http.StatusSeeOther)
}

func (h *Handler) ShipOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := r.PathValue("id")
	_, err := commandmemory.Dispatch[*application.ShipOrderCommand, *application.ShipOrderResult](ctx, h.app.CmdBus, &application.ShipOrderCommand{OrderID: orderID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/orders/"+orderID, http.StatusSeeOther)
}

func (h *Handler) CancelOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := r.PathValue("id")
	reason := r.FormValue("reason")
	_, err := commandmemory.Dispatch[*application.CancelOrderCommand, *application.CancelOrderResult](ctx, h.app.CmdBus, &application.CancelOrderCommand{OrderID: orderID, Reason: reason})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/orders/"+orderID, http.StatusSeeOther)
}

func (h *Handler) DeleteOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := r.PathValue("id")
	if err := h.app.OrderRepo.Delete(ctx, orderID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	http.Redirect(w, r, "/orders", http.StatusSeeOther)
}

func (h *Handler) OrderEvents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := r.PathValue("id")
	events, err := h.app.DomainEventStore.Load(ctx, orderID, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	type EventView struct {
		EventType  string
		OrderID    string
		Details    string
		OccurredAt string
	}
	views := make([]EventView, len(events))
	for i, e := range events {
		view := EventView{
			EventType:  e.EventType(),
			OccurredAt: e.OccurredAt().Format(time.RFC3339),
		}
		switch evt := e.(type) {
		case *domain.OrderPlacedEvent:
			view.OrderID = evt.OrderID
			view.Details = fmt.Sprintf("UserID: %s, Amount: %.2f", evt.UserID, evt.TotalAmount)
		case *domain.PaymentConfirmedEvent:
			view.OrderID = evt.OrderID
			view.Details = "Payment confirmed"
		case *domain.OrderShippedEvent:
			view.OrderID = evt.OrderID
			view.Details = "Order shipped"
		case *domain.OrderCancelledEvent:
			view.OrderID = evt.OrderID
			view.Details = fmt.Sprintf("Cancelled: %s", evt.Reason)
		default:
			view.OrderID = e.AggregateID()
		}
		views[i] = view
	}

	order, err := querymemory.Dispatch[*application.GetOrderQuery, *application.GetOrderResult](ctx, h.app.QueryBus, &application.GetOrderQuery{OrderID: orderID})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "order_events", map[string]interface{}{"OrderID": orderID, "Events": views, "Order": order})
}

func (h *Handler) Inventory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result, err := querymemory.Dispatch[*application.GetInventoryQuery, *application.GetInventoryResult](ctx, h.app.QueryBus, &application.GetInventoryQuery{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "inventory", map[string]interface{}{"Products": result.Products})
}

func (h *Handler) ListJobs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var allJobs []*jobcore.Job
	for _, status := range []jobcore.JobStatus{
		jobcore.JobStatusPending, jobcore.JobStatusRunning,
		jobcore.JobStatusCompleted, jobcore.JobStatusFailed, jobcore.JobStatusCancelled,
	} {
		jobs, err := h.app.JobManager.ListByStatus(ctx, status)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		allJobs = append(allJobs, jobs...)
	}
	h.render(w, "jobs", map[string]interface{}{"Jobs": allJobs})
}

func (h *Handler) SubmitJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	orderID := r.FormValue("order_id")
	timeoutStr := r.FormValue("timeout")
	retriesStr := r.FormValue("retries")

	var opts []jobcore.JobOption
	if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
		opts = append(opts, jobcore.WithTimeout(time.Duration(timeout)*time.Millisecond))
	}
	if retries, err := strconv.Atoi(retriesStr); err == nil && retries > 0 {
		opts = append(opts, jobcore.WithMaxRetries(retries))
	}

	_, err := h.app.JobManager.Submit(ctx, &application.GenerateReportCommand{OrderID: orderID}, opts...)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/jobs", http.StatusSeeOther)
}

func (h *Handler) CancelJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobID := r.PathValue("id")
	if err := h.app.JobManager.Cancel(ctx, jobID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/jobs", http.StatusSeeOther)
}

func (h *Handler) RetryJob(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	jobID := r.PathValue("id")
	if err := h.app.JobManager.Retry(ctx, jobID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/jobs", http.StatusSeeOther)
}

func (h *Handler) ListTraces(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	filterType := r.URL.Query().Get("type")
	filterStatus := r.URL.Query().Get("status")

	filter := trace.TraceFilter{Type: filterType, Status: filterStatus}
	traceIDs, err := h.app.Backend.TraceStore.ListTraces(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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

	traces := make([]TraceView, 0, len(traceIDs))
	for _, tid := range traceIDs {
		spans, err := h.app.Backend.TraceStore.GetTrace(ctx, tid)
		if err != nil {
			continue
		}
		var spanViews []SpanView
		for _, s := range spans {
			spanViews = append(spanViews, SpanView{
				ID: s.ID, TraceID: s.TraceID, ParentID: s.ParentID,
				Type: s.Type, Name: s.Name, Status: s.Status,
				Error: s.Error, Duration: s.Duration,
			})
		}
		traces = append(traces, TraceView{TraceID: tid, Spans: spanViews})
	}

	h.render(w, "traces", map[string]interface{}{
		"Traces":       traces,
		"FilterType":   filterType,
		"FilterStatus": filterStatus,
	})
}

func (h *Handler) render(w http.ResponseWriter, name string, data map[string]interface{}) {
	data["Page"] = name
	data["Title"] = "DDD-QCE Shop"
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tmpl.ExecuteTemplate(w, name, data); err != nil {
		fmt.Fprintf(w, "template error: %v", err)
	}
}

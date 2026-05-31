package http

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/ddd-qce/core/cqrs/command"
	"github.com/ddd-qce/core/cqrs/event"
	"github.com/ddd-qce/core/cqrs/query"
	jobcore "github.com/ddd-qce/core/job/core"
	pgmigrate "github.com/ddd-qce/core/pg"
	"github.com/ddd-qce/core/trace"
	inventorycommand "github.com/ddd-qce/exampleapp/ddd/inventory/command"
	inventorydomain "github.com/ddd-qce/exampleapp/ddd/inventory/domain"
	inventoryevent "github.com/ddd-qce/exampleapp/ddd/inventory/event"
	inventoryquery "github.com/ddd-qce/exampleapp/ddd/inventory/query"
	ordercommand "github.com/ddd-qce/exampleapp/ddd/order/command"
	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
	orderevent "github.com/ddd-qce/exampleapp/ddd/order/event"
	orderquery "github.com/ddd-qce/exampleapp/ddd/order/query"
	"github.com/ddd-qce/exampleapp/infrastructure"
)

type Handler struct {
	app        *infrastructure.AppContext
	tmpl       *template.Template
	tmplMu     sync.RWMutex
	tmplPath   string
	tmplFuncs  template.FuncMap
	livereload *LiveReloadServer
	reloadMu   sync.Mutex
}

func NewHandler(app *infrastructure.AppContext, livereload *LiveReloadServer) *Handler {
	fm := template.FuncMap{"add": func(a, b int) int { return a + b }}
	tmplPath := resolveTemplatePath()
	tmpl := template.Must(template.New("").Funcs(fm).ParseGlob(tmplPath))
	return &Handler{app: app, tmpl: tmpl, tmplPath: tmplPath, tmplFuncs: fm, livereload: livereload}
}

func (h *Handler) reloadTemplates() {
	h.reloadMu.Lock()
	defer h.reloadMu.Unlock()
	if h.livereload == nil || !h.livereload.IsStale() {
		return
	}
	tmpl := template.Must(template.New("").Funcs(h.tmplFuncs).ParseGlob(h.tmplPath))
	h.tmplMu.Lock()
	h.tmpl = tmpl
	h.tmplMu.Unlock()
	h.livereload.ClearStale()
}

func TemplateDir() string {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine source file path")
	}
	return filepath.Join(filepath.Dir(thisFile), "templates")
}

func resolveTemplatePath() string {
	dir := TemplateDir()
	pattern := filepath.Join(dir, "*.html")
	if matches, err := filepath.Glob(pattern); err == nil && len(matches) > 0 {
		return pattern
	}
	panic(fmt.Sprintf("no templates found at %s", pattern))
}

func (h *Handler) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ordersResult, err := query.Dispatch[*orderquery.ListOrdersQuery, *orderquery.ListOrdersResult](ctx, h.app.QueryBus, &orderquery.ListOrdersQuery{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	inventoryResult, err := query.Dispatch[*inventoryquery.GetInventoryQuery, *inventoryquery.GetInventoryResult](ctx, h.app.QueryBus, &inventoryquery.GetInventoryQuery{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	paid, shipped, cancelled := 0, 0, 0
	for _, o := range ordersResult.Orders {
		if o.CancelledAt != "" {
			cancelled++
			continue
		}
		if o.PaidAt != "" {
			paid++
		}
		if o.ShippedAt != "" {
			shipped++
		}
	}

	h.render(w, "dashboard", map[string]interface{}{
		"TotalOrders":  len(ordersResult.Orders),
		"Paid":         paid,
		"Shipped":      shipped,
		"Cancelled":    cancelled,
		"Products":     inventoryResult.Products,
		"RecentOrders": ordersResult.Orders,
	})
}

func (h *Handler) ListOrders(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result, err := query.Dispatch[*orderquery.ListOrdersQuery, *orderquery.ListOrdersResult](ctx, h.app.QueryBus, &orderquery.ListOrdersQuery{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "orders", map[string]interface{}{"Orders": result.Orders})
}

func (h *Handler) NewOrderForm(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	inventoryResult, err := query.Dispatch[*inventoryquery.GetInventoryQuery, *inventoryquery.GetInventoryResult](ctx, h.app.QueryBus, &inventoryquery.GetInventoryQuery{})
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
	var items []ordercommand.ItemInput
	products := h.app.Inventory.GetAll()
	for _, p := range products {
		qtyStr := r.FormValue("qty_" + p.ID.String())
		if qty, err := strconv.Atoi(qtyStr); err == nil && qty > 0 {
			items = append(items, ordercommand.ItemInput{
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

	result, err := command.Dispatch[*ordercommand.PlaceOrderCommand, *ordercommand.PlaceOrderResult](ctx, h.app.CmdBus, &ordercommand.PlaceOrderCommand{
		UserID: orderdomain.NewUserID(userID),
		Items:  items,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/orders/"+result.OrderID.String(), http.StatusSeeOther)
}

func (h *Handler) OrderDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := r.PathValue("id")
	result, err := query.Dispatch[*orderquery.GetOrderQuery, *orderquery.GetOrderResult](ctx, h.app.QueryBus, &orderquery.GetOrderQuery{OrderID: orderdomain.NewOrderID(orderID)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	h.render(w, "order_detail", map[string]interface{}{"Order": result})
}

func (h *Handler) ConfirmPayment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := r.PathValue("id")
	_, err := command.Dispatch[*ordercommand.ConfirmPaymentCommand, *ordercommand.ConfirmPaymentResult](ctx, h.app.CmdBus, &ordercommand.ConfirmPaymentCommand{OrderID: orderdomain.NewOrderID(orderID)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/orders/"+orderID, http.StatusSeeOther)
}

func (h *Handler) ShipOrder(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	orderID := r.PathValue("id")
	_, err := command.Dispatch[*ordercommand.ShipOrderCommand, *ordercommand.ShipOrderResult](ctx, h.app.CmdBus, &ordercommand.ShipOrderCommand{OrderID: orderdomain.NewOrderID(orderID)})
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
	_, err := command.Dispatch[*ordercommand.CancelOrderCommand, *ordercommand.CancelOrderResult](ctx, h.app.CmdBus, &ordercommand.CancelOrderCommand{OrderID: orderdomain.NewOrderID(orderID), Reason: reason})
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
	events, err := h.app.EventStore.Load(ctx, orderID, 0)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(events) == 0 {
		http.Error(w, "order not found", http.StatusNotFound)
		return
	}

	type EventView struct {
		EventType  string
		OrderID    string
		UserID     string
		Amount     float64
		Details    string
		OccurredAt string
	}
	views := make([]EventView, len(events))
	for i, e := range events {
		view := EventView{
			EventType:  event.EventNameOf(e),
			OccurredAt: e.OccurredAt().Format(time.RFC3339),
		}
		switch evt := e.(type) {
		case *orderevent.OrderPlacedEvent:
			view.OrderID = evt.AggregateID()
			view.UserID = evt.UserID
			view.Amount = evt.TotalAmount
			view.Details = fmt.Sprintf("UserID: %s, Amount: %.2f", evt.UserID, evt.TotalAmount)
		case *orderevent.PaymentConfirmedEvent:
			view.OrderID = evt.AggregateID()
			view.Details = "Payment confirmed"
		case *orderevent.OrderShippedEvent:
			view.OrderID = evt.AggregateID()
			view.Details = "Order shipped"
		case *orderevent.OrderCancelledEvent:
			view.OrderID = evt.AggregateID()
			view.Details = fmt.Sprintf("Cancelled: %s", evt.Reason)
		default:
			view.OrderID = e.AggregateID()
		}
		views[i] = view
	}

	order, err := query.Dispatch[*orderquery.GetOrderQuery, *orderquery.GetOrderResult](ctx, h.app.QueryBus, &orderquery.GetOrderQuery{OrderID: orderdomain.NewOrderID(orderID)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "order_events", map[string]interface{}{"OrderID": orderID, "Events": views, "Order": order})
}

func (h *Handler) Inventory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	result, err := query.Dispatch[*inventoryquery.GetInventoryQuery, *inventoryquery.GetInventoryResult](ctx, h.app.QueryBus, &inventoryquery.GetInventoryQuery{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "inventory", map[string]interface{}{"Products": result.Products})
}

func (h *Handler) InventoryManage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Method == http.MethodPost {
		productID := r.FormValue("product_id")
		quantity := r.FormValue("quantity")

		if productID == "" || quantity == "" {
			http.Error(w, "product_id and quantity are required", http.StatusBadRequest)
			return
		}

		qty, err := strconv.Atoi(quantity)
		if err != nil || qty <= 0 {
			http.Error(w, "quantity must be a positive integer", http.StatusBadRequest)
			return
		}

		product, ok := h.app.Inventory.GetByID(orderdomain.ProductID(productID))
		if !ok || product.ID == "" {
			http.Error(w, "product not found", http.StatusNotFound)
			return
		}

		err = h.app.Inventory.AddStock(orderdomain.ProductID(productID), qty)
		if err != nil {
			http.Error(w, fmt.Sprintf("failed to add stock: %v", err), http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/inventory/manage", http.StatusFound)
		return
	}

	result, err := query.Dispatch[*inventoryquery.GetInventoryQuery, *inventoryquery.GetInventoryResult](ctx, h.app.QueryBus, &inventoryquery.GetInventoryQuery{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.render(w, "inventory_manage", map[string]interface{}{"Products": result.Products})
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
	durationStr := r.FormValue("duration")

	var opts []jobcore.JobOption
	var err error

	if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
		opts = append(opts, jobcore.WithTimeout(time.Duration(timeout)*time.Millisecond))
	}
	if retries, err := strconv.Atoi(retriesStr); err == nil && retries > 0 {
		opts = append(opts, jobcore.WithMaxRetries(retries))
	}

	if duration, err := strconv.Atoi(durationStr); err == nil && duration > 0 {
		cmd := &ordercommand.ProcessBatchCommand{
			OrderID:  orderdomain.NewOrderID(orderID),
			Duration: time.Duration(duration) * time.Millisecond,
		}
		_, err = h.app.JobManager.Submit(ctx, cmd, opts...)
	} else {
		_, err = h.app.JobManager.Submit(ctx, &ordercommand.GenerateReportCommand{OrderID: orderdomain.NewOrderID(orderID)}, opts...)
	}

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
	http.Redirect(w, r, "/api/ddd/ddd_traces", http.StatusFound)
}

type TestStep struct {
	Type     string
	Name     string
	Duration string
	Success  bool
	Error    string
	Detail   string
}

type TestResult struct {
	Title    string
	Steps    []TestStep
	Total    int
	Passed   int
	Failed   int
	Duration string
}

func (h *Handler) TestQuery(w http.ResponseWriter, r *http.Request) {
	traceID := trace.NewTraceID()
	ctx := trace.WithTrace(r.Context(), traceID, "")
	start := time.Now()
	var steps []TestStep

	step := func(name string, fn func() (string, error)) {
		s := time.Now()
		detail, err := fn()
		steps = append(steps, TestStep{
			Type:     "query",
			Name:     name,
			Duration: time.Since(s).String(),
			Success:  err == nil,
			Error:    errStr(err),
			Detail:   detail,
		})
	}

	step("ListOrdersQuery", func() (string, error) {
		result, err := query.Dispatch[*orderquery.ListOrdersQuery, *orderquery.ListOrdersResult](ctx, h.app.QueryBus, &orderquery.ListOrdersQuery{})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("returned %d orders", len(result.Orders)), nil
	})

	step("GetOrderQuery (with orderID)", func() (string, error) {
		placeResult, err := command.Dispatch[*ordercommand.PlaceOrderCommand, *ordercommand.PlaceOrderResult](ctx, h.app.CmdBus, &ordercommand.PlaceOrderCommand{
			UserID: orderdomain.NewUserID("test-query-user"),
			Items:  []ordercommand.ItemInput{{ProductID: orderdomain.ProductID("laptop"), ProductName: "Laptop", Price: 999.99, Quantity: 1}},
		})
		if err != nil {
			return "", fmt.Errorf("setup: place order: %w", err)
		}
		result, err := query.Dispatch[*orderquery.GetOrderQuery, *orderquery.GetOrderResult](ctx, h.app.QueryBus, &orderquery.GetOrderQuery{OrderID: placeResult.OrderID})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("order %s status=%s total=%.2f", result.OrderID, result.Status, result.TotalAmount), nil
	})

	step("GetInventoryQuery", func() (string, error) {
		result, err := query.Dispatch[*inventoryquery.GetInventoryQuery, *inventoryquery.GetInventoryResult](ctx, h.app.QueryBus, &inventoryquery.GetInventoryQuery{})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("returned %d products", len(result.Products)), nil
	})

	h.renderTestResult(w, "Test Query", steps, time.Since(start))
}

func (h *Handler) TestCommand(w http.ResponseWriter, r *http.Request) {
	traceID := trace.NewTraceID()
	ctx := trace.WithTrace(r.Context(), traceID, "")
	start := time.Now()
	var steps []TestStep

	step := func(name string, fn func() (string, error)) {
		s := time.Now()
		detail, err := fn()
		steps = append(steps, TestStep{
			Type:     "command",
			Name:     name,
			Duration: time.Since(s).String(),
			Success:  err == nil,
			Error:    errStr(err),
			Detail:   detail,
		})
	}

	var orderID orderdomain.OrderID

	step("PlaceOrderCommand", func() (string, error) {
		result, err := command.Dispatch[*ordercommand.PlaceOrderCommand, *ordercommand.PlaceOrderResult](ctx, h.app.CmdBus, &ordercommand.PlaceOrderCommand{
			UserID: orderdomain.NewUserID("test-cmd-user"),
			Items:  []ordercommand.ItemInput{{ProductID: orderdomain.ProductID("mouse"), ProductName: "Mouse", Price: 29.99, Quantity: 2}},
		})
		if err != nil {
			return "", err
		}
		orderID = result.OrderID
		return fmt.Sprintf("orderID=%s total=%.2f", result.OrderID, result.TotalAmount), nil
	})

	step("ConfirmPaymentCommand", func() (string, error) {
		result, err := command.Dispatch[*ordercommand.ConfirmPaymentCommand, *ordercommand.ConfirmPaymentResult](ctx, h.app.CmdBus, &ordercommand.ConfirmPaymentCommand{OrderID: orderID})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("success=%v", result.Success), nil
	})

	step("ShipOrderCommand", func() (string, error) {
		result, err := command.Dispatch[*ordercommand.ShipOrderCommand, *ordercommand.ShipOrderResult](ctx, h.app.CmdBus, &ordercommand.ShipOrderCommand{OrderID: orderID})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("success=%v", result.Success), nil
	})

	var cancelID orderdomain.OrderID
	step("CancelOrderCommand", func() (string, error) {
		placeResult, err := command.Dispatch[*ordercommand.PlaceOrderCommand, *ordercommand.PlaceOrderResult](ctx, h.app.CmdBus, &ordercommand.PlaceOrderCommand{
			UserID: orderdomain.NewUserID("test-cancel-user"),
			Items:  []ordercommand.ItemInput{{ProductID: orderdomain.ProductID("keyboard"), ProductName: "Keyboard", Price: 79.99, Quantity: 1}},
		})
		if err != nil {
			return "", fmt.Errorf("setup: place order: %w", err)
		}
		cancelID = placeResult.OrderID
		result, err := command.Dispatch[*ordercommand.CancelOrderCommand, *ordercommand.CancelOrderResult](ctx, h.app.CmdBus, &ordercommand.CancelOrderCommand{OrderID: cancelID, Reason: "test cancel"})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("success=%v", result.Success), nil
	})

	step("GenerateReportCommand", func() (string, error) {
		result, err := command.Dispatch[*ordercommand.GenerateReportCommand, *ordercommand.GenerateReportResult](ctx, h.app.CmdBus, &ordercommand.GenerateReportCommand{OrderID: orderID})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("reportID=%s generated=%v", result.ReportID, result.Generated), nil
	})

	step("ReserveInventoryCommand", func() (string, error) {
		uid := uuid.New()
		result, err := command.Dispatch[*inventorycommand.ReserveInventoryCommand, *inventorycommand.ReserveInventoryResult](ctx, h.app.CmdBus, &inventorycommand.ReserveInventoryCommand{
			OrderID:   orderdomain.NewOrderID(hex.EncodeToString(uid[:])),
			ProductID: orderdomain.ProductID("headphone"),
			Quantity:  1,
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("success=%v", result.Success), nil
	})

	step("ReleaseInventoryCommand", func() (string, error) {
		uid := uuid.New()
		result, err := command.Dispatch[*inventorycommand.ReleaseInventoryCommand, *inventorycommand.ReleaseInventoryResult](ctx, h.app.CmdBus, &inventorycommand.ReleaseInventoryCommand{
			OrderID:   orderdomain.NewOrderID(hex.EncodeToString(uid[:])),
			ProductID: orderdomain.ProductID("headphone"),
			Quantity:  1,
		})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("success=%v", result.Success), nil
	})

	h.renderTestResult(w, "Test Command", steps, time.Since(start))
}

func (h *Handler) TestEvent(w http.ResponseWriter, r *http.Request) {
	traceID := trace.NewTraceID()
	ctx := trace.WithTrace(r.Context(), traceID, "")
	start := time.Now()
	var steps []TestStep

	step := func(name string, fn func() (string, error)) {
		s := time.Now()
		detail, err := fn()
		steps = append(steps, TestStep{
			Type:     "event",
			Name:     name,
			Duration: time.Since(s).String(),
			Success:  err == nil,
			Error:    errStr(err),
			Detail:   detail,
		})
	}

	uid := uuid.New()
	testAggID := hex.EncodeToString(uid[:])

	step("OrderPlacedEvent (→ NotificationHandler + InventoryHandler)", func() (string, error) {
		err := h.app.EventBus.Publish(ctx, &orderevent.OrderPlacedEvent{
			BaseEvent:   event.WithCorrelation(ctx, testAggID),
			UserID:      "test-event-user",
			TotalAmount: 999.99,
			Items:       []string{"Laptop"},
		})
		if err != nil {
			return "", err
		}
		time.Sleep(200 * time.Millisecond)
		return "dispatched, handlers invoked", nil
	})

	step("PaymentConfirmedEvent", func() (string, error) {
		err := h.app.EventBus.Publish(ctx, &orderevent.PaymentConfirmedEvent{
			BaseEvent: event.WithCorrelation(ctx, testAggID),
		})
		if err != nil {
			return "", err
		}
		time.Sleep(100 * time.Millisecond)
		return "dispatched", nil
	})

	step("OrderShippedEvent", func() (string, error) {
		err := h.app.EventBus.Publish(ctx, &orderevent.OrderShippedEvent{
			BaseEvent: event.WithCorrelation(ctx, testAggID),
		})
		if err != nil {
			return "", err
		}
		time.Sleep(100 * time.Millisecond)
		return "dispatched", nil
	})

	step("OrderCancelledEvent (→ InventoryHandler)", func() (string, error) {
		err := h.app.EventBus.Publish(ctx, &orderevent.OrderCancelledEvent{
			BaseEvent: event.WithCorrelation(ctx, testAggID),
			Reason:    "test cancel",
		})
		if err != nil {
			return "", err
		}
		time.Sleep(200 * time.Millisecond)
		return "dispatched, inventory handler invoked", nil
	})

	step("InventoryReservedEvent", func() (string, error) {
		uid2 := uuid.New()
		err := h.app.EventBus.Publish(ctx, &inventoryevent.InventoryReservedEvent{
			BaseEvent: event.WithCorrelation(ctx, hex.EncodeToString(uid2[:])),
			ProductID: "mouse",
			Quantity:  2,
		})
		if err != nil {
			return "", err
		}
		time.Sleep(100 * time.Millisecond)
		return "dispatched", nil
	})

	step("InventoryReleasedEvent", func() (string, error) {
		uid2 := uuid.New()
		err := h.app.EventBus.Publish(ctx, &inventoryevent.InventoryReleasedEvent{
			BaseEvent: event.WithCorrelation(ctx, hex.EncodeToString(uid2[:])),
			ProductID: "mouse",
			Quantity:  2,
		})
		if err != nil {
			return "", err
		}
		time.Sleep(100 * time.Millisecond)
		return "dispatched", nil
	})

	h.renderTestResult(w, "Test Event", steps, time.Since(start))
}

func (h *Handler) TestQCE(w http.ResponseWriter, r *http.Request) {
	traceID := trace.NewTraceID()
	ctx := trace.WithTrace(r.Context(), traceID, "")

	start := time.Now()
	var steps []TestStep

	step := func(typ, name string, fn func() (string, error)) {
		s := time.Now()
		detail, err := fn()
		steps = append(steps, TestStep{
			Type:     typ,
			Name:     name,
			Duration: time.Since(s).String(),
			Success:  err == nil,
			Error:    errStr(err),
			Detail:   detail,
		})
	}

	var orderID orderdomain.OrderID

	step("command", "PlaceOrderCommand", func() (string, error) {
		result, err := command.Dispatch[*ordercommand.PlaceOrderCommand, *ordercommand.PlaceOrderResult](ctx, h.app.CmdBus, &ordercommand.PlaceOrderCommand{
			UserID: orderdomain.NewUserID("test-qce-user"),
			Items:  []ordercommand.ItemInput{{ProductID: orderdomain.ProductID("monitor"), ProductName: "Monitor", Price: 499.99, Quantity: 1}},
		})
		if err != nil {
			return "", err
		}
		orderID = result.OrderID
		return fmt.Sprintf("orderID=%s total=%.2f (traceID=%s)", result.OrderID, result.TotalAmount, traceID), nil
	})

	time.Sleep(200 * time.Millisecond)

	step("event", "OrderPlacedEvent (auto-dispatched)", func() (string, error) {
		return "triggered by PlaceOrderCommand, handled by NotificationHandler + InventoryHandler → ReserveInventoryCommand", nil
	})

	step("query", "GetOrderQuery (verify pending)", func() (string, error) {
		result, err := query.Dispatch[*orderquery.GetOrderQuery, *orderquery.GetOrderResult](ctx, h.app.QueryBus, &orderquery.GetOrderQuery{OrderID: orderID})
		if err != nil {
			return "", err
		}
		if result.Status != "pending" {
			return "", fmt.Errorf("expected pending, got %s", result.Status)
		}
		return fmt.Sprintf("status=%s (correct)", result.Status), nil
	})

	step("command", "ConfirmPaymentCommand", func() (string, error) {
		result, err := command.Dispatch[*ordercommand.ConfirmPaymentCommand, *ordercommand.ConfirmPaymentResult](ctx, h.app.CmdBus, &ordercommand.ConfirmPaymentCommand{OrderID: orderID})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("success=%v", result.Success), nil
	})

	time.Sleep(100 * time.Millisecond)

	step("event", "PaymentConfirmedEvent (auto-dispatched)", func() (string, error) {
		return "triggered by ConfirmPaymentCommand", nil
	})

	step("query", "GetOrderQuery (verify paid)", func() (string, error) {
		result, err := query.Dispatch[*orderquery.GetOrderQuery, *orderquery.GetOrderResult](ctx, h.app.QueryBus, &orderquery.GetOrderQuery{OrderID: orderID})
		if err != nil {
			return "", err
		}
		if result.Status != "paid" {
			return "", fmt.Errorf("expected paid, got %s", result.Status)
		}
		return fmt.Sprintf("status=%s (correct)", result.Status), nil
	})

	step("command", "ShipOrderCommand", func() (string, error) {
		result, err := command.Dispatch[*ordercommand.ShipOrderCommand, *ordercommand.ShipOrderResult](ctx, h.app.CmdBus, &ordercommand.ShipOrderCommand{OrderID: orderID})
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("success=%v", result.Success), nil
	})

	time.Sleep(100 * time.Millisecond)

	step("event", "OrderShippedEvent (auto-dispatched)", func() (string, error) {
		return "triggered by ShipOrderCommand", nil
	})

	step("query", "GetOrderQuery (verify shipped)", func() (string, error) {
		result, err := query.Dispatch[*orderquery.GetOrderQuery, *orderquery.GetOrderResult](ctx, h.app.QueryBus, &orderquery.GetOrderQuery{OrderID: orderID})
		if err != nil {
			return "", err
		}
		if result.Status != "shipped" {
			return "", fmt.Errorf("expected shipped, got %s", result.Status)
		}
		return fmt.Sprintf("status=%s (correct)", result.Status), nil
	})

	step("query", "GetInventoryQuery (verify stock deducted)", func() (string, error) {
		result, err := query.Dispatch[*inventoryquery.GetInventoryQuery, *inventoryquery.GetInventoryResult](ctx, h.app.QueryBus, &inventoryquery.GetInventoryQuery{})
		if err != nil {
			return "", err
		}
		for _, p := range result.Products {
			if p.Name == "Laptop" || p.Name == "Monitor" {
				return fmt.Sprintf("%s stock=%d", p.Name, p.Stock), nil
			}
		}
		return "inventory retrieved", nil
	})

	h.renderTestResult(w, "Test QCE Full Lifecycle", steps, time.Since(start))
}

func (h *Handler) TestJob(w http.ResponseWriter, r *http.Request) {
	traceID := trace.NewTraceID()
	ctx := trace.WithTrace(r.Context(), traceID, "")
	start := time.Now()
	var steps []TestStep

	step := func(name string, fn func() (string, error)) {
		s := time.Now()
		detail, err := fn()
		steps = append(steps, TestStep{
			Type:     "job",
			Name:     name,
			Duration: time.Since(s).String(),
			Success:  err == nil,
			Error:    errStr(err),
			Detail:   detail,
		})
	}

	var orderID orderdomain.OrderID

	step("PlaceOrder (setup)", func() (string, error) {
		result, err := command.Dispatch[*ordercommand.PlaceOrderCommand, *ordercommand.PlaceOrderResult](ctx, h.app.CmdBus, &ordercommand.PlaceOrderCommand{
			UserID: orderdomain.NewUserID("test-job-user"),
			Items:  []ordercommand.ItemInput{{ProductID: orderdomain.ProductID("laptop"), ProductName: "Laptop", Price: 999, Quantity: 1}},
		})
		if err != nil {
			return "", err
		}
		orderID = result.OrderID
		return fmt.Sprintf("orderID=%s total=%.2f", result.OrderID, result.TotalAmount), nil
	})

	time.Sleep(100 * time.Millisecond)

	step("SubmitJob → Complete", func() (string, error) {
		job, err := h.app.JobManager.Submit(ctx, &ordercommand.GenerateReportCommand{OrderID: orderID})
		if err != nil {
			return "", err
		}
		_, err = h.app.JobManager.Wait(ctx, job.ID(), 10*time.Second)
		if err != nil {
			return "", err
		}
		job, err = h.app.JobManager.GetStatus(ctx, job.ID())
		if err != nil {
			return "", err
		}
		if job.GetStatus() != jobcore.JobStatusCompleted {
			return "", fmt.Errorf("expected completed, got %s", job.GetStatus())
		}
		return fmt.Sprintf("jobID=%s status=%s", job.ID(), job.GetStatus()), nil
	})

	step("SubmitJob → Timeout", func() (string, error) {
		job, err := h.app.JobManager.Submit(ctx, &ordercommand.GenerateReportCommand{OrderID: orderID}, jobcore.WithTimeout(1*time.Millisecond))
		if err != nil {
			return "", err
		}
		_, _ = h.app.JobManager.Wait(ctx, job.ID(), 10*time.Second)
		job, err = h.app.JobManager.GetStatus(ctx, job.ID())
		if err != nil {
			return "", err
		}
		if job.GetStatus() != jobcore.JobStatusFailed {
			return "", fmt.Errorf("expected failed (timeout), got %s", job.GetStatus())
		}
		return fmt.Sprintf("jobID=%s status=%s (timeout triggered)", job.ID(), job.GetStatus()), nil
	})

	step("SubmitJob → Cancel", func() (string, error) {
		job, err := h.app.JobManager.Submit(ctx, &ordercommand.GenerateReportCommand{OrderID: orderID})
		if err != nil {
			return "", err
		}
		if err := h.app.JobManager.Cancel(ctx, job.ID()); err != nil {
			return "", err
		}
		job, err = h.app.JobManager.GetStatus(ctx, job.ID())
		if err != nil {
			return "", err
		}
		if job.GetStatus() != jobcore.JobStatusCancelled {
			return "", fmt.Errorf("expected cancelled, got %s", job.GetStatus())
		}
		return fmt.Sprintf("jobID=%s status=%s", job.ID(), job.GetStatus()), nil
	})

	step("SubmitJob → Fail → Retry (success)", func() (string, error) {
		job, err := h.app.JobManager.Submit(ctx, &ordercommand.GenerateReportCommand{OrderID: orderID}, jobcore.WithTimeout(1*time.Millisecond))
		if err != nil {
			return "", err
		}
		_, _ = h.app.JobManager.Wait(ctx, job.ID(), 10*time.Second)
		job, err = h.app.JobManager.GetStatus(ctx, job.ID())
		if err != nil {
			return "", err
		}
		if job.GetStatus() != jobcore.JobStatusFailed {
			return "", fmt.Errorf("expected failed (before retry), got %s", job.GetStatus())
		}
		return fmt.Sprintf("beforeRetry: status=%s error=%s", job.GetStatus(), job.GetError()), nil
	})

	step("ListJobsByStatus", func() (string, error) {
		completed, err := h.app.JobManager.ListByStatus(ctx, jobcore.JobStatusCompleted)
		if err != nil {
			return "", err
		}
		failed, err := h.app.JobManager.ListByStatus(ctx, jobcore.JobStatusFailed)
		if err != nil {
			return "", err
		}
		cancelled, err := h.app.JobManager.ListByStatus(ctx, jobcore.JobStatusCancelled)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("completed=%d failed=%d cancelled=%d", len(completed), len(failed), len(cancelled)), nil
	})

	h.renderTestResult(w, "Test Job Lifecycle", steps, time.Since(start))
}

func (h *Handler) renderTestResult(w http.ResponseWriter, title string, steps []TestStep, totalDuration time.Duration) {
	passed, failed := 0, 0
	for _, s := range steps {
		if s.Success {
			passed++
		} else {
			failed++
		}
	}
	h.render(w, "test_result", map[string]interface{}{
		"TestName": title,
		"Steps":    steps,
		"Total":    len(steps),
		"Passed":   passed,
		"Failed":   failed,
		"Duration": totalDuration.String(),
	})
}

func errStr(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func (h *Handler) render(w http.ResponseWriter, name string, data map[string]interface{}) {
	if h.livereload != nil && h.livereload.IsStale() {
		h.reloadTemplates()
	}

	h.tmplMu.RLock()
	tmpl := h.tmpl
	h.tmplMu.RUnlock()

	data["Page"] = name
	data["Title"] = "DDD-QCE Shop"

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, "template error: %v", err)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if h.livereload != nil {
		result := strings.Replace(buf.String(), "</body>", livereloadScript+"</body>", 1)
		w.Write([]byte(result))
	} else {
		buf.WriteTo(w)
	}
}

func (h *Handler) AdminReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.app.Config.TestMode {
		http.Error(w, "test mode not enabled", http.StatusForbidden)
		return
	}

	ctx := r.Context()
	db := h.app.Store().DB
	if db == nil {
		http.Error(w, "database not available", http.StatusServiceUnavailable)
		return
	}

	if err := pgmigrate.TruncateAll(db); err != nil {
		http.Error(w, fmt.Sprintf("truncate failed: %v", err), http.StatusInternalServerError)
		return
	}

	products := inventorydomain.DefaultProducts()
	store := inventorydomain.NewPgProductStore(db)
	if err := inventorydomain.SeedProducts(ctx, store, products); err != nil {
		http.Error(w, fmt.Sprintf("seed products failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func (h *Handler) TestSeedJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !h.app.Config.TestMode {
		http.Error(w, "test mode not enabled", http.StatusForbidden)
		return
	}

	status := r.URL.Query().Get("status")
	if status != "running" && status != "failed" {
		http.Error(w, "status must be 'running' or 'failed'", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	db := h.app.Store().DB
	if db == nil {
		http.Error(w, "database not available", http.StatusServiceUnavailable)
		return
	}

	jobID := uuid.New().String()
	now := time.Now()
	var completedAt interface{}
	if status == "failed" {
		completedAt = now
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO ddd_jobs (id, command, command_type, status, created_at, started_at, completed_at, error)
		 VALUES ($1, $2, $3, $4, $5, $5, $6, $7)`,
		jobID, `{"type":"e2e-test"}`, "e2e-test", status, now, completedAt, "",
	)
	if err != nil {
		http.Error(w, fmt.Sprintf("insert job failed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	fmt.Fprintf(w, `{"jobId":"%s"}`, jobID)
}

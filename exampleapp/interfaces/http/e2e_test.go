package http

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/ddd-qce/core/cqrs/command"
	ordercommand "github.com/ddd-qce/exampleapp/ddd/order/command"
	orderdomain "github.com/ddd-qce/exampleapp/ddd/order/domain"
	"github.com/ddd-qce/exampleapp/infrastructure"
)

func setupE2ETest(t *testing.T, storeType string) (*httptest.Server, *infrastructure.AppContext) {
	t.Helper()
	cfg := &infrastructure.Config{StoreType: storeType}
	if storeType == infrastructure.StoreTypePostgreSQL {
		dsn := os.Getenv("DDD_POSTGRES_URI")
		if dsn == "" {
			t.Skip("DDD_POSTGRES_URI not set, skipping PostgreSQL test")
		}
		cfg.PostgresURI = dsn
	}
	app, err := infrastructure.WireAppWithConfig(cfg)
	if err != nil {
		t.Fatalf("wire app (%s): %v", storeType, err)
	}
	t.Cleanup(func() { app.Close(context.Background()) })

	mux := http.NewServeMux()
	h := NewHandler(app, nil)
	mux.HandleFunc("/", h.Dashboard)
	mux.HandleFunc("GET /orders", h.ListOrders)
	mux.HandleFunc("GET /orders/new", h.NewOrderForm)
	mux.HandleFunc("POST /orders", h.PlaceOrder)
	mux.HandleFunc("GET /orders/{id}", h.OrderDetail)
	mux.HandleFunc("POST /orders/{id}/confirm", h.ConfirmPayment)
	mux.HandleFunc("POST /orders/{id}/ship", h.ShipOrder)
	mux.HandleFunc("POST /orders/{id}/cancel", h.CancelOrder)
	mux.HandleFunc("POST /orders/{id}/delete", h.DeleteOrder)
	mux.HandleFunc("GET /orders/{id}/events", h.OrderEvents)
	mux.HandleFunc("GET /inventory", h.Inventory)
	mux.HandleFunc("GET /jobs", h.ListJobs)
	mux.HandleFunc("POST /jobs", h.SubmitJob)
	mux.HandleFunc("POST /jobs/{id}/cancel", h.CancelJob)
	mux.HandleFunc("POST /jobs/{id}/retry", h.RetryJob)
	mux.HandleFunc("GET /traces", h.ListTraces)
	app.DDDViewer.RegisterRoutes(mux)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, app
}

func runE2EForBothStores(t *testing.T, fn func(t *testing.T, server *httptest.Server, app *infrastructure.AppContext)) {
	t.Helper()
	t.Run("Memory", func(t *testing.T) {
		server, app := setupE2ETest(t, infrastructure.StoreTypeMemory)
		fn(t, server, app)
	})
	t.Run("PostgreSQL", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping PostgreSQL test in short mode")
		}
		server, app := setupE2ETest(t, infrastructure.StoreTypePostgreSQL)
		fn(t, server, app)
	})
}

func getBody(t *testing.T, server *httptest.Server, path string) (int, string) {
	t.Helper()
	resp, err := http.Get(server.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestE2E_BusinessPages(t *testing.T) {
	runE2EForBothStores(t, func(t *testing.T, server *httptest.Server, _ *infrastructure.AppContext) {
		for _, tc := range []struct {
			path   string
			expect string
		}{
			{"/", "Dashboard"},
			{"/orders", "Orders"},
			{"/orders/new", "Place Order"},
			{"/inventory", "Inventory"},
			{"/jobs", "Jobs"},
			{"/traces", "Traces"},
		} {
			code, body := getBody(t, server, tc.path)
			if code != http.StatusOK {
				t.Errorf("GET %s: expected 200, got %d", tc.path, code)
			}
			if !strings.Contains(body, tc.expect) {
				t.Errorf("GET %s: expected %q in body", tc.path, tc.expect)
			}
		}
	})
}

func TestE2E_DDDViewerPages(t *testing.T) {
	runE2EForBothStores(t, func(t *testing.T, server *httptest.Server, _ *infrastructure.AppContext) {
		for _, tc := range []struct {
			path   string
			expect string
		}{
			{"/api/ddd/ddd_overview", "DDD Overview"},
			{"/api/ddd/ddd_schema/", "DDD Tables"},
			{"/api/ddd/ddd_schema/ddd_command_log", "ddd_command_log"},
			{"/api/ddd/ddd_commands", "Commands"},
			{"/api/ddd/ddd_queries", "Queries"},
			{"/api/ddd/ddd_events", "Events"},
			{"/api/ddd/ddd_stats", "Statistics"},
			{"/api/ddd/ddd_jobs", "Jobs"},
			{"/api/ddd/ddd_traces", "Traces"},
			{"/api/ddd/ddd_health", ""},
		} {
			code, body := getBody(t, server, tc.path)
			if code != http.StatusOK {
				t.Errorf("GET %s: expected 200, got %d, body: %s", tc.path, code, body[:min(len(body), 200)])
			}
			if tc.expect != "" && !strings.Contains(body, tc.expect) {
				t.Errorf("GET %s: expected %q in body", tc.path, tc.expect)
			}
		}
	})
}

func TestE2E_DDDViewerHealthJSON(t *testing.T) {
	runE2EForBothStores(t, func(t *testing.T, server *httptest.Server, _ *infrastructure.AppContext) {
		resp, err := http.Get(server.URL + "/api/ddd/ddd_health")
		if err != nil {
			t.Fatalf("GET health: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
			t.Errorf("expected json content-type, got %s", ct)
		}
		b, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(b), `"status"`) {
			t.Errorf("expected status in health json, got: %s", string(b))
		}
	})
}

func TestE2E_PlaceOrderThenCheckDDDViewer(t *testing.T) {
	runE2EForBothStores(t, func(t *testing.T, server *httptest.Server, app *infrastructure.AppContext) {
		ctx := context.Background()
		result, err := command.Dispatch[*ordercommand.PlaceOrderCommand, *ordercommand.PlaceOrderResult](ctx, app.CmdBus, &ordercommand.PlaceOrderCommand{
			UserID: orderdomain.NewUserID("user-e2e"),
			Items:  []ordercommand.ItemInput{{ProductID: orderdomain.NewProductID("widget"), ProductName: "Widget", Price: 10, Quantity: 2}},
		})
		if err != nil {
			t.Fatalf("place order: %v", err)
		}

		code, body := getBody(t, server, "/api/ddd/ddd_commands")
		if code != http.StatusOK {
			t.Errorf("commands page: expected 200, got %d", code)
		}
		if !strings.Contains(body, "PlaceOrder") {
			t.Error("expected PlaceOrder in commands page after dispatching command")
		}

		code, body = getBody(t, server, "/api/ddd/ddd_events")
		if code != http.StatusOK {
			t.Errorf("events page: expected 200, got %d", code)
		}
		if !strings.Contains(body, "OrderPlaced") {
			t.Error("expected OrderPlaced in events page after dispatching command")
		}

		code, body = getBody(t, server, "/api/ddd/ddd_stats")
		if code != http.StatusOK {
			t.Errorf("stats page: expected 200, got %d", code)
		}
		if !strings.Contains(body, "PlaceOrder") {
			t.Error("expected PlaceOrder in stats page")
		}

		code, body = getBody(t, server, "/api/ddd/ddd_traces")
		if code != http.StatusOK {
			t.Errorf("traces page: expected 200, got %d", code)
		}

		code, body = getBody(t, server, "/api/ddd/ddd_overview")
		if code != http.StatusOK {
			t.Errorf("overview page: expected 200, got %d", code)
		}
		if !strings.Contains(body, string(result.OrderID)) {
			t.Logf("overview does not contain order ID (may be expected, overview shows tables)")
		}
	})
}

func TestE2E_PlaceOrderViaHTTPThenCheckDDDViewer(t *testing.T) {
	runE2EForBothStores(t, func(t *testing.T, server *httptest.Server, app *infrastructure.AppContext) {
		resp, err := http.PostForm(server.URL+"/orders", url.Values{
			"user_id":    {"user-http-e2e"},
			"qty_laptop": {"1"},
		})
		if err != nil {
			t.Fatalf("post order: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusSeeOther {
			t.Errorf("place order: expected 200/303, got %d", resp.StatusCode)
		}

		code, body := getBody(t, server, "/api/ddd/ddd_commands")
		if code != http.StatusOK {
			t.Errorf("commands page: expected 200, got %d", code)
		}
		if !strings.Contains(body, "PlaceOrder") {
			t.Error("expected PlaceOrder in commands page after HTTP place order")
		}

		code, body = getBody(t, server, "/api/ddd/ddd_events")
		if code != http.StatusOK {
			t.Errorf("events page: expected 200, got %d", code)
		}
		if !strings.Contains(body, "OrderPlaced") {
			t.Error("expected OrderPlaced in events page after HTTP place order")
		}
	})
}

func TestE2E_DDDViewerSchemaDetailPage(t *testing.T) {
	runE2EForBothStores(t, func(t *testing.T, server *httptest.Server, _ *infrastructure.AppContext) {
		for _, table := range []string{
			"ddd_command_log",
			"ddd_query_log",
			"ddd_event_log",
			"ddd_event_handler_log",
			"ddd_domain_events",
		} {
			code, body := getBody(t, server, "/api/ddd/ddd_schema/"+table)
			if code != http.StatusOK {
				t.Errorf("schema detail %s: expected 200, got %d", table, code)
			}
			if !strings.Contains(body, table) {
				t.Errorf("schema detail %s: expected table name in body", table)
			}
		}
	})
}

func TestE2E_DDDViewerSchemaNonDDDTable(t *testing.T) {
	runE2EForBothStores(t, func(t *testing.T, server *httptest.Server, _ *infrastructure.AppContext) {
		code, _ := getBody(t, server, "/api/ddd/ddd_schema/orders")
		if code != http.StatusNotFound {
			t.Errorf("non-ddd table: expected 404, got %d", code)
		}
	})
}

func TestE2E_DDDViewerNavLinkCrossPage(t *testing.T) {
	runE2EForBothStores(t, func(t *testing.T, server *httptest.Server, _ *infrastructure.AppContext) {
		_, body := getBody(t, server, "/api/ddd/ddd_overview")
		navLinks := []string{
			"ddd_overview",
			"ddd_schema/",
			"ddd_commands",
			"ddd_queries",
			"ddd_events",
			"ddd_stats",
			"ddd_jobs",
			"ddd_traces",
		}
		for _, link := range navLinks {
			if !strings.Contains(body, link) {
				t.Errorf("overview page missing nav link: %s", link)
			}
		}

		_, body = getBody(t, server, "/api/ddd/ddd_schema/ddd_command_log")
		if !strings.Contains(body, "ddd_schema/") {
			t.Error("schema detail page missing back link to ddd_schema/")
		}
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

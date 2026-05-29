package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/ddd-qce/exampleapp/infrastructure"
)

func setupHTTPTest(t *testing.T, storeType string) (*httptest.Server, *infrastructure.AppContext) {
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
	handler := NewHandler(app, nil)
	mux := http.NewServeMux()
	mux.HandleFunc("/", handler.Dashboard)
	mux.HandleFunc("GET /orders", handler.ListOrders)
	mux.HandleFunc("GET /orders/new", handler.NewOrderForm)
	mux.HandleFunc("GET /inventory", handler.Inventory)
	mux.HandleFunc("GET /jobs", handler.ListJobs)
	mux.HandleFunc("GET /traces", handler.ListTraces)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)
	return server, app
}

func runHTTPTestForBothStores(t *testing.T, fn func(t *testing.T, server *httptest.Server, app *infrastructure.AppContext)) {
	t.Helper()
	t.Run("Memory", func(t *testing.T) {
		server, app := setupHTTPTest(t, infrastructure.StoreTypeMemory)
		fn(t, server, app)
	})
	t.Run("PostgreSQL", func(t *testing.T) {
		if testing.Short() {
			t.Skip("skipping PostgreSQL test in short mode")
		}
		server, app := setupHTTPTest(t, infrastructure.StoreTypePostgreSQL)
		fn(t, server, app)
	})
}

func TestHTTP_Dashboard(t *testing.T) {
	runHTTPTestForBothStores(t, func(t *testing.T, server *httptest.Server, _ *infrastructure.AppContext) {
		resp, err := http.Get(server.URL + "/")
		if err != nil {
			t.Fatalf("get dashboard failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestHTTP_OrdersPage(t *testing.T) {
	runHTTPTestForBothStores(t, func(t *testing.T, server *httptest.Server, _ *infrastructure.AppContext) {
		resp, err := http.Get(server.URL + "/orders")
		if err != nil {
			t.Fatalf("get orders failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestHTTP_NewOrderForm(t *testing.T) {
	runHTTPTestForBothStores(t, func(t *testing.T, server *httptest.Server, _ *infrastructure.AppContext) {
		resp, err := http.Get(server.URL + "/orders/new")
		if err != nil {
			t.Fatalf("get new order form failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestHTTP_InventoryPage(t *testing.T) {
	runHTTPTestForBothStores(t, func(t *testing.T, server *httptest.Server, _ *infrastructure.AppContext) {
		resp, err := http.Get(server.URL + "/inventory")
		if err != nil {
			t.Fatalf("get inventory failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestHTTP_JobsPage(t *testing.T) {
	runHTTPTestForBothStores(t, func(t *testing.T, server *httptest.Server, _ *infrastructure.AppContext) {
		resp, err := http.Get(server.URL + "/jobs")
		if err != nil {
			t.Fatalf("get jobs failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestHTTP_TracesPage(t *testing.T) {
	runHTTPTestForBothStores(t, func(t *testing.T, server *httptest.Server, _ *infrastructure.AppContext) {
		resp, err := http.Get(server.URL + "/traces")
		if err != nil {
			t.Fatalf("get traces failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected 200, got %d", resp.StatusCode)
		}
	})
}

func TestHTTP_PlaceOrder(t *testing.T) {
	runHTTPTestForBothStores(t, func(t *testing.T, server *httptest.Server, _ *infrastructure.AppContext) {
		resp, err := http.PostForm(server.URL+"/orders", map[string][]string{
			"user_id":    {"user-test"},
			"qty_laptop": {"1"},
		})
		if err != nil {
			t.Fatalf("post order failed: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusSeeOther {
			t.Errorf("expected 200 or 303, got %d", resp.StatusCode)
		}
	})
}

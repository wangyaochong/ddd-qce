package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ddd-qce/exampleapp/infrastructure"
)

func setupHTTPTest(t *testing.T) (*httptest.Server, *infrastructure.AppContext) {
	app := infrastructure.WireApp()
	handler := NewHandler(app)
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

func TestHTTP_Dashboard(t *testing.T) {
	server, _ := setupHTTPTest(t)
	resp, err := http.Get(server.URL + "/")
	if err != nil {
		t.Fatalf("get dashboard failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHTTP_OrdersPage(t *testing.T) {
	server, _ := setupHTTPTest(t)
	resp, err := http.Get(server.URL + "/orders")
	if err != nil {
		t.Fatalf("get orders failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHTTP_NewOrderForm(t *testing.T) {
	server, _ := setupHTTPTest(t)
	resp, err := http.Get(server.URL + "/orders/new")
	if err != nil {
		t.Fatalf("get new order form failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHTTP_InventoryPage(t *testing.T) {
	server, _ := setupHTTPTest(t)
	resp, err := http.Get(server.URL + "/inventory")
	if err != nil {
		t.Fatalf("get inventory failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHTTP_JobsPage(t *testing.T) {
	server, _ := setupHTTPTest(t)
	resp, err := http.Get(server.URL + "/jobs")
	if err != nil {
		t.Fatalf("get jobs failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHTTP_TracesPage(t *testing.T) {
	server, _ := setupHTTPTest(t)
	resp, err := http.Get(server.URL + "/traces")
	if err != nil {
		t.Fatalf("get traces failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestHTTP_PlaceOrder(t *testing.T) {
	server, app := setupHTTPTest(t)
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
	_ = app
}

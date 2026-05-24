package http

import (
	"net/http"

	"github.com/ddd-qce/exampleapp/infrastructure"
)

func NewServer(app *infrastructure.AppContext) *http.Server {
	mux := http.NewServeMux()
	h := NewHandler(app)

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

	return &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}
}

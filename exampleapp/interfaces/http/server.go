package http

import (
	"log"
	"net/http"

	"github.com/ddd-qce/exampleapp/infrastructure"
)

func NewServer(app *infrastructure.AppContext) *http.Server {
	mux := http.NewServeMux()

	lr, err := NewLiveReloadServer(TemplateDir())
	if err != nil {
		log.Printf("[LiveReload] Warning: could not start livereload: %v", err)
	}

	h := NewHandler(app, lr)

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

	mux.HandleFunc("POST /test/query", h.TestQuery)
	mux.HandleFunc("POST /test/command", h.TestCommand)
	mux.HandleFunc("POST /test/event", h.TestEvent)
	mux.HandleFunc("POST /test/qce", h.TestQCE)

	if lr != nil {
		mux.Handle("/livereload", lr)
	}

	app.DDDViewer.RegisterRoutes(mux)

	srv := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	if lr != nil {
		srv.RegisterOnShutdown(func() { lr.Close() })
	}

	return srv
}

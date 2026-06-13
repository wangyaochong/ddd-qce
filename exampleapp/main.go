package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/ddd-qce/core/app"
	"github.com/ddd-qce/exampleapp/infrastructure"
	httpinterface "github.com/ddd-qce/exampleapp/interfaces/http"
)

func main() {
	appCtx := infrastructure.WireApp()
	defer appCtx.Close(context.Background())

	server := httpinterface.NewServer(appCtx)
	appCtx.RegisterLifecycle(app.LifecycleFunc(func(ctx context.Context) error {
		return server.Shutdown(ctx)
	}))

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	log.Println("DDD-QCE E-Commerce starting on http://localhost:8555")
	appCtx.WaitForSignal(30 * time.Second)
}

package main

import (
	"log"

	"github.com/ddd-qce/exampleapp/infrastructure"
	httpinterface "github.com/ddd-qce/exampleapp/interfaces/http"
)

func main() {
	app := infrastructure.WireApp()
	defer app.Close()
	server := httpinterface.NewServer(app)
	log.Println("DDD-QCE E-Commerce starting on http://localhost:8080")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}

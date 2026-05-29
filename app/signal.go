package app

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func (a *App) WaitForSignal(timeout time.Duration) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	sig := <-sigCh
	log.Printf("received %v, shutting down...", sig)
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := a.Close(ctx); err != nil {
		log.Printf("shutdown error: %v", err)
	}
}

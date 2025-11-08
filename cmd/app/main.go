package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	app, err := initializeApp()
	if err != nil {
		log.Fatalf("failed to wire application: %v", err)
	}

	if err := app.Run(ctx); err != nil {
		log.Fatalf("application stopped with error: %v", err)
	}
}

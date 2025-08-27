package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	appapi "github.com/latoulicious/RoutePilot/internal/app/api"
	"github.com/latoulicious/RoutePilot/internal/config"
)

func main() {
	fmt.Println("Starting RoutePilot API Server...")

	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}

	// Build server
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server, cleanup, err := appapi.BuildServer(ctx, cfg)
	if err != nil {
		log.Fatalf("bootstrap error: %v", err)
	}
	defer cleanup()

	// Start server
	go func() {
		if err := server.Start(); err != nil {
			log.Printf("server error: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("Shutting down server...")
	if err := server.Shutdown(context.Background()); err != nil {
		log.Printf("server forced to shutdown: %v", err)
	}
	fmt.Println("Server exited")
}

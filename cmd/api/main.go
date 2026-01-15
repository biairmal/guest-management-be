package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"
)

func main() {
	// Initialize HTTP server
	mux := http.NewServeMux()

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	var wg sync.WaitGroup

	// Start server in a goroutine
	wg.Go(func() {
		log.Println("Starting server on port 8080")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	})

	// Wait for server to start
	select {
	case err := <-serverErr:
		log.Fatalf("Failed to start server: %v", err)
	case <-time.After(100 * time.Millisecond):
		log.Println("Server started successfully")
	}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	select {
	case <-ctx.Done():
		log.Println("Shutdown signal received, initiating graceful shutdown...")
	case err := <-serverErr:
		log.Printf("Server error occurred: %v, initiating shutdown...", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
		if errors.Is(err, context.DeadlineExceeded) {
			log.Println("Shutdown timeout exceeded, forcing server close...")
			server.Close()
		}
	}

	wg.Wait()
	log.Println("Server shutdown completed")
}

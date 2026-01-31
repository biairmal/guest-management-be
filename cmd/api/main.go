package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/biairmal/go-sdk/errorz"
	"github.com/biairmal/go-sdk/httpkit"
	"github.com/biairmal/go-sdk/httpkit/handler"
	"github.com/biairmal/go-sdk/httpkit/middleware"
	"github.com/biairmal/go-sdk/httpkit/response"
	"github.com/biairmal/go-sdk/logger"
	"github.com/go-chi/chi/v5"
)

func main() {
	log := logger.NewZerolog(&logger.Options{
		Level:  logger.LevelInfo,
		Output: logger.OutputStdout,
		Format: logger.FormatText,
	})

	r := chi.NewRouter()
	r.Use(middleware.Recover(), middleware.RequestID(), middleware.Logging(log, nil))

	r.Get("/health", httpkit.Health())
	r.Get("/ready", httpkit.Readiness(func(_ context.Context) error { return nil }))
	r.Get("/api/v1/ping", handler.Handle(pingHandler))

	r.Route("/api/v1/items", func(r chi.Router) {
		r.Get("/", handler.Handle(listItems))
		r.Get("/{id}", handler.Handle(getItem))
		r.Post("/", handler.Handle(createItem))
		r.Put("/{id}", handler.Handle(updateItem))
		r.Delete("/{id}", handler.Handle(deleteItem))
	})

	server := &http.Server{
		Addr:              ":8080",
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Info("Starting server on port 8080")
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	select {
	case err := <-serverErr:
		log.Fatalf("Failed to start server: %v", err)
	case <-time.After(100 * time.Millisecond):
		log.Info("Server started successfully")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	select {
	case <-ctx.Done():
		log.Info("Shutdown signal received, initiating graceful shutdown...")
	case err := <-serverErr:
		log.Infof("Server error occurred: %v, initiating shutdown...", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Infof("Error during server shutdown: %v", err)
		if errors.Is(err, context.DeadlineExceeded) {
			log.Info("Shutdown timeout exceeded, forcing server close...")
			_ = server.Close()
		}
	}

	wg.Wait()
	log.Info("Server shutdown completed")
}

func pingHandler(_ *http.Request) (any, error) {
	return response.OK(map[string]string{"pong": "ok"}), nil
}

// Dummy CRUD: in-memory store for demo.
var itemsStore = []item{
	{ID: "1", Name: "Item One"},
	{ID: "2", Name: "Item Two"},
}

type item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func listItems(r *http.Request) (any, error) {
	limit := r.URL.Query().Get("limit")
	_ = limit
	return response.OK(itemsStore), nil
}

func getItem(r *http.Request) (any, error) {
	id := chi.URLParam(r, "id")
	for _, it := range itemsStore {
		if it.ID == id {
			return response.OK(it), nil
		}
	}
	return nil, errorz.NotFound()
}

func createItem(r *http.Request) (any, error) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest()
	}
	created := item{ID: "3", Name: body.Name}
	itemsStore = append(itemsStore, created)
	return response.Created(created), nil
}

func updateItem(r *http.Request) (any, error) {
	id := chi.URLParam(r, "id")
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return nil, errorz.BadRequest()
	}
	for i := range itemsStore {
		if itemsStore[i].ID == id {
			itemsStore[i].Name = body.Name
			return response.OK(itemsStore[i]), nil
		}
	}
	return nil, errorz.NotFound()
}

func deleteItem(r *http.Request) (any, error) {
	id := chi.URLParam(r, "id")
	for i := range itemsStore {
		if itemsStore[i].ID == id {
			itemsStore = append(itemsStore[:i], itemsStore[i+1:]...)
			return response.NoContent(), nil
		}
	}
	return nil, errorz.NotFound()
}

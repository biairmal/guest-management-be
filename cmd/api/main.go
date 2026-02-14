package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"time"

	"github.com/biairmal/go-sdk/config"
	"github.com/biairmal/go-sdk/httpkit"
	"github.com/biairmal/go-sdk/httpkit/middleware"
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/sqlkit"
	_ "github.com/biairmal/guest-management-be/api/swagger"
	"github.com/biairmal/guest-management-be/internal/app"
	appconfig "github.com/biairmal/guest-management-be/internal/config"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
	httpSwagger "github.com/swaggo/http-swagger"
)

// @title           Guest Management API
// @version         1.0
// @description     This is a guest management server.
// @host            localhost:8080
// @BasePath        /

// @externalDocs.description  OpenAPI
// @externalDocs.url          https://swagger.io/resources/open-api/
func main() {
	// Load configurations
	var cfg appconfig.Config
	err := config.Load(&cfg,
		config.EnvFile(".env"),
		config.Files("configs/config.yaml"),
	)
	if err != nil {
		panic("Failed to load configurations: " + err.Error())
	}

	ctx := context.Background()

	// Initialize logger
	log := logger.NewZerolog(&cfg.Logger)

	// Initialize database
	db, err := sqlkit.New(ctx, &cfg.Database)
	if err != nil {
		log.Panicf("Database config failed: %v", err)
	}
	if db != nil {
		defer func() { _ = db.Close() }()
	}

	// Initialize router
	r := chi.NewRouter()
	r.Use(middleware.Recover(), middleware.RequestID(), middleware.Logging(log, nil))

	r.Get("/health", httpkit.Health())
	r.Get("/ready", httpkit.Readiness(func(_ context.Context) error { return nil }))

	// Setup Swagger
	setupSwagger(&cfg, r)

	// Initialize application
	application := app.NewApp(app.Options{}, log, db, r)
	application.Initialize()

	server := &http.Server{
		Addr:              "127.0.0.1:8080",
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serverErr := make(chan error, 1)
	var wg sync.WaitGroup

	wg.Add(1)

	startServer(&wg, log, server, serverErr)
	gracefulShutdown(log, serverErr, server)

	wg.Wait()

	log.Info("Server shutdown completed")
}

func setupSwagger(cfg *appconfig.Config, r *chi.Mux) {
	if cfg.Swagger.Enabled {
		r.Route("/swagger", func(r chi.Router) {
			r.Use(chiMiddleware.BasicAuth("swagger", map[string]string{
				cfg.Swagger.Username: cfg.Swagger.Password,
			}))
			r.Get("/", func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/swagger/index.html", http.StatusFound)
			})
			r.Get("/*", httpSwagger.WrapHandler)
		})
	}
}

func startServer(wg *sync.WaitGroup, log logger.Logger, server *http.Server, serverErr chan error) {
	go func() {
		defer wg.Done()
		log.Info("Starting server on port 127.0.0.1:8080")
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
}

func gracefulShutdown(log logger.Logger, serverErr chan error, server *http.Server) {
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
}

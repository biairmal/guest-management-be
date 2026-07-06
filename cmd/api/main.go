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
	"github.com/biairmal/go-sdk/ctxkit"
	"github.com/biairmal/go-sdk/errorz"
	"github.com/biairmal/go-sdk/httpkit"
	"github.com/biairmal/go-sdk/httpkit/middleware"
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/redis"
	"github.com/biairmal/go-sdk/sqlkit"
	"github.com/biairmal/go-sdk/tracer"
	_ "github.com/biairmal/guest-management-be/api/swagger"
	"github.com/biairmal/guest-management-be/internal/app"
	appconfig "github.com/biairmal/guest-management-be/internal/config"
	"github.com/biairmal/guest-management-be/internal/core/validation"
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
	if err := cfg.Server.Validate(); err != nil {
		panic("Invalid server configuration: " + err.Error())
	}
	if err := cfg.App.Validate(); err != nil {
		panic("Invalid app configuration: " + err.Error())
	}
	if err := cfg.Tracing.Validate(); err != nil {
		panic("Invalid tracing configuration: " + err.Error())
	}

	ctx := context.Background()

	// Initialize logger. ContextExtractor surfaces request_id/correlation_id/
	// trace_id/user_id from context on every *WithContext log call.
	cfg.Logger.ContextExtractor = ctxkit.LoggerExtractor()
	log := logger.NewZerolog(&cfg.Logger)

	// Initialize tracer
	tr, err := newTracer(&cfg, log)
	if err != nil {
		log.Panicf("Tracer config failed: %v", err)
	}
	defer func() { _ = tr.Shutdown(context.Background()) }()

	// Initialize database
	db, err := sqlkit.New(ctx, &cfg.Database)
	if err != nil {
		log.Panicf("Database config failed: %v", err)
	}
	if db != nil {
		defer func() { _ = db.Close() }()
	}

	// Initialize Redis
	redisClient, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		log.Panicf("Redis config failed: %v", err)
	}
	defer func() { _ = redisClient.Close() }()

	// Initialize router
	r := chi.NewRouter()
	r.Use(middleware.Recover(), middleware.RequestID(), middleware.Tracing(tr), middleware.Logging(log, nil))

	r.Get("/health", httpkit.Health())
	r.Get("/ready", httpkit.Readiness(readinessCheck(db, redisClient)))

	// Setup Swagger
	setupSwagger(&cfg, r)

	// Initialize boundary validator
	val := validation.New(cfg.Validator)

	// Initialize application. cfg.App carries every registered feature's own
	// config (app.<feature>.* in config.yaml); internal/app resolves each
	// feature's section itself when it wires that feature's repositories.
	application := app.NewApp(log, db, r, val, redisClient, cfg.App)
	if err := application.Initialize(); err != nil {
		panic("Failed to initialize application: " + err.Error())
	}

	server := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           r,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		ReadTimeout:       cfg.Server.ReadTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
	}

	serverErr := make(chan error, 1)
	var wg sync.WaitGroup

	wg.Add(1)

	startServer(&wg, log, &cfg, server, serverErr)
	gracefulShutdown(log, &cfg, serverErr, server)

	wg.Wait()

	log.Info("Server shutdown completed")
}

// newTracer returns a NoOp tracer when tracing is disabled, otherwise a real
// OTel tracer shipping spans to cfg.Tracing.Tracer.Endpoint.
func newTracer(cfg *appconfig.Config, log logger.Logger) (tracer.Tracer, error) {
	if !cfg.Tracing.Enabled {
		return tracer.NewNoOp(), nil
	}
	return tracer.NewOTel(cfg.Tracing.Tracer, tracer.WithLogger(log))
}

// readinessCheck pings the leader database and Redis so /ready reflects real
// dependency health instead of always returning 200.
func readinessCheck(db *sqlkit.DB, redisClient redis.Client) func(context.Context) error {
	return func(ctx context.Context) error {
		if err := db.Leader().PingContext(ctx); err != nil {
			return errorz.Wrap(err).WithCode(errorz.CodeServiceUnavailable).WithMessage("database not ready")
		}
		if err := redisClient.Ping(ctx); err != nil {
			return errorz.Wrap(err).WithCode(errorz.CodeServiceUnavailable).WithMessage("redis not ready")
		}
		return nil
	}
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

func startServer(
	wg *sync.WaitGroup, log logger.Logger, cfg *appconfig.Config, server *http.Server, serverErr chan error,
) {
	go func() {
		defer wg.Done()
		log.Infof("Starting server on %s", cfg.Server.Addr())
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

func gracefulShutdown(log logger.Logger, cfg *appconfig.Config, serverErr chan error, server *http.Server) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer stop()

	select {
	case <-ctx.Done():
		log.Info("Shutdown signal received, initiating graceful shutdown...")
	case err := <-serverErr:
		log.Infof("Server error occurred: %v, initiating shutdown...", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Infof("Error during server shutdown: %v", err)
		if errors.Is(err, context.DeadlineExceeded) {
			log.Info("Shutdown timeout exceeded, forcing server close...")
			_ = server.Close()
		}
	}
}

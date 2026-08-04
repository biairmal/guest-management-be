package main

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"

	"github.com/biairmal/go-sdk/lib/config"
	"github.com/biairmal/go-sdk/lib/ctxkit"
	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/httpkit"
	"github.com/biairmal/go-sdk/lib/httpkit/middleware"
	"github.com/biairmal/go-sdk/lib/lifecycle"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/go-sdk/lib/metrics"
	"github.com/biairmal/go-sdk/lib/ratelimit"
	"github.com/biairmal/go-sdk/lib/redis"
	"github.com/biairmal/go-sdk/lib/sqlkit"
	"github.com/biairmal/go-sdk/lib/tracer"
	_ "github.com/biairmal/guest-management-be/api/swagger"
	"github.com/biairmal/guest-management-be/internal/app"
	appconfig "github.com/biairmal/guest-management-be/internal/config"
	"github.com/biairmal/guest-management-be/internal/core/validation"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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
	cfg := loadConfig()

	ctx := context.Background()

	// Initialize logger. ContextExtractor surfaces request_id/correlation_id/
	// trace_id/user_id from context on every *WithContext log call.
	cfg.Logger.ContextExtractor = ctxkit.LoggerExtractor()
	log := logger.NewZerolog(&cfg.Logger)

	deps := buildDependencies(ctx, &cfg, log)

	// readiness starts true; lifecycle.Run flips it to false as soon as a
	// shutdown signal is received, so /ready starts returning 503 before the
	// server stops accepting connections.
	var ready atomic.Bool
	ready.Store(true)

	r := buildRouter(&cfg, log, &ready, deps)

	// Initialize boundary validator
	val := validation.New(cfg.Validator)

	// Initialize application. cfg.App carries every registered feature's own
	// config (app.<feature>.* in config.yaml); internal/app resolves each
	// feature's section itself when it wires that feature's repositories.
	application := app.NewApp(log, deps.db, r, val, deps.redisClient, cfg.App)
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

	go func() {
		log.Infof("Starting server on %s", cfg.Server.Addr())
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Errorf("Server error: %v", err)
		}
	}()

	runLifecycle(ctx, log, server, &cfg, &ready, deps)
}

// loadConfig loads Config from configs/config.yaml + .env and validates
// every section, panicking with a descriptive message on the first failure.
func loadConfig() appconfig.Config {
	var cfg appconfig.Config
	if err := config.Load(&cfg, config.EnvFile(".env"), config.Files("configs/config.yaml")); err != nil {
		panic("Failed to load configurations: " + err.Error())
	}

	validators := []struct {
		name string
		fn   func() error
	}{
		{"server", cfg.Server.Validate},
		{"app", cfg.App.Validate},
		{"tracing", cfg.Tracing.Validate},
		{"metrics", cfg.Metrics.Validate},
		{"rate limit", cfg.RateLimit.Validate},
		{"lifecycle", cfg.Lifecycle.Validate},
	}
	for _, v := range validators {
		if err := v.fn(); err != nil {
			panic("Invalid " + v.name + " configuration: " + err.Error())
		}
	}
	return cfg
}

// dependencies bundles the infrastructure clients main wires once and
// threads through router construction and lifecycle shutdown.
type dependencies struct {
	tracer      tracer.Tracer
	db          *sqlkit.DB
	redisClient redis.Client
	recorder    metrics.Recorder
	limiter     ratelimit.Limiter
}

// buildDependencies constructs every infrastructure client the app needs,
// panicking (via the logger) on the first failure.
func buildDependencies(ctx context.Context, cfg *appconfig.Config, log logger.Logger) dependencies {
	tr, err := newTracer(cfg, log)
	if err != nil {
		log.Panicf("Tracer config failed: %v", err)
	}

	db, err := sqlkit.New(ctx, &cfg.Database)
	if err != nil {
		log.Panicf("Database config failed: %v", err)
	}

	redisClient, err := redis.NewClient(&cfg.Redis)
	if err != nil {
		log.Panicf("Redis config failed: %v", err)
	}

	rec, err := newMetricsRecorder(cfg)
	if err != nil {
		log.Panicf("Metrics config failed: %v", err)
	}

	limiter, err := newRateLimiter(cfg, redisClient)
	if err != nil {
		log.Panicf("Rate limit config failed: %v", err)
	}

	return dependencies{tracer: tr, db: db, redisClient: redisClient, recorder: rec, limiter: limiter}
}

// buildRouter wires the middleware chain, health/ready/metrics endpoints,
// and Swagger UI onto a fresh chi.Mux. Middleware order: Metrics outermost
// (counts every request incl. panics) -> Recover -> RequestID -> Correlation
// -> Tracing -> Logging -> RateLimit (keyed by IP; no authenticated user yet).
func buildRouter(cfg *appconfig.Config, log logger.Logger, ready *atomic.Bool, deps dependencies) *chi.Mux {
	r := chi.NewRouter()
	r.Use(
		middleware.Metrics(deps.recorder, nil),
		middleware.Recover(),
		middleware.RequestID(),
		middleware.Correlation(),
		middleware.Tracing(deps.tracer),
		middleware.Logging(log, nil),
		middleware.RateLimit(deps.limiter, middleware.KeyByIP),
	)

	r.Get("/health", httpkit.Health())
	r.Get("/ready", httpkit.Readiness(readinessCheck(ready, deps.db, deps.redisClient)))
	if cfg.Metrics.Enabled {
		r.Handle("/metrics", promhttp.Handler())
	}

	setupSwagger(cfg, r)
	return r
}

// runLifecycle blocks until a shutdown signal arrives, then drains in-flight
// requests and closes tracer/redis/db in that order (least-recoverable
// first) under their own deadlines.
func runLifecycle(
	ctx context.Context, log logger.Logger, server *http.Server, cfg *appconfig.Config,
	ready *atomic.Bool, deps dependencies,
) {
	err := lifecycle.Run(ctx, server, cfg.Lifecycle,
		lifecycle.WithReadiness(ready),
		lifecycle.WithLogger(log),
		lifecycle.WithCloser("tracer", lifecycle.CloserFromTracer(deps.tracer)),
		lifecycle.WithCloser("redis", lifecycle.CloserFromRedis(deps.redisClient)),
		lifecycle.WithCloser("db", lifecycle.CloserFromDB(deps.db)),
	)
	if err != nil {
		log.Errorf("Shutdown completed with errors: %v", err)
		return
	}
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

// newMetricsRecorder returns a NoOp Recorder when metrics are disabled,
// otherwise a Prometheus recorder scraped at /metrics.
func newMetricsRecorder(cfg *appconfig.Config) (metrics.Recorder, error) {
	if !cfg.Metrics.Enabled {
		return metrics.NewNoOp(), nil
	}
	return metrics.NewPrometheus(cfg.Metrics.Metrics)
}

// newRateLimiter returns a nil Limiter when rate limiting is disabled (the
// RateLimit middleware treats a nil Limiter as pass-through), otherwise a
// Limiter built from the configured backend (memory or redis).
func newRateLimiter(cfg *appconfig.Config, redisClient redis.Client) (ratelimit.Limiter, error) {
	if !cfg.RateLimit.Enabled {
		return nil, nil //nolint:nilnil // nil Limiter is the documented pass-through sentinel for RateLimit middleware
	}
	return ratelimit.FromConfig(cfg.RateLimit.RateLimit, redisClient)
}

// readinessCheck pings the leader database and Redis so /ready reflects real
// dependency health instead of always returning 200, and also honors the
// lifecycle readiness flag flipped during graceful shutdown.
func readinessCheck(ready *atomic.Bool, db *sqlkit.DB, redisClient redis.Client) func(context.Context) error {
	return func(ctx context.Context) error {
		if !ready.Load() {
			return errorz.ServiceUnavailable().WithMessage("shutting down")
		}
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

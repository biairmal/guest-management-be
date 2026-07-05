package app

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/redis"
	"github.com/biairmal/go-sdk/sqlkit"
	appconfig "github.com/biairmal/guest-management-be/internal/config"
	"github.com/biairmal/guest-management-be/internal/core/validation"
	"github.com/go-chi/chi/v5"
)

// App is the composition root: it wires repositories, services, handlers, and routes.
type App struct {
	logger        logger.Logger
	db            *sqlkit.DB
	router        *chi.Mux
	validator     validation.Validator
	redisClient   redis.Client
	featureConfig appconfig.FeatureConfig
	repositories  *repositories
	service       *service
	handler       *handler
}

// NewApp returns an App ready to be Initialize()d. featureConfig is the
// app.<feature>.* config tree; redisClient feeds any feature repository that
// opts into caching. Each feature's own config lives under featureConfig
// (e.g. featureConfig.Events) — registering a new feature means reading its
// section here, not changing this constructor's signature.
func NewApp(
	logger logger.Logger, db *sqlkit.DB, router *chi.Mux, validator validation.Validator,
	redisClient redis.Client, featureConfig appconfig.FeatureConfig,
) *App {
	return &App{
		logger: logger, db: db, router: router, validator: validator,
		redisClient: redisClient, featureConfig: featureConfig,
	}
}

// Initialize wires repositories, services, handlers, and routes for every feature.
func (a *App) Initialize() error {
	repositories, err := a.initializeRepository(a.logger, a.db, a.redisClient, a.featureConfig)
	if err != nil {
		return err
	}
	a.repositories = repositories
	a.service = a.initializeService(a.logger, a.repositories)
	a.handler = a.initializeHandler(a.logger, a.validator, a.service)
	a.initializeRoutes(a.logger, a.router, a.handler)
	return nil
}

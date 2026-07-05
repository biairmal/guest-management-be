package app

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/sqlkit"
	"github.com/biairmal/guest-management-be/internal/core/validation"
	"github.com/go-chi/chi/v5"
)

// App is the composition root: it wires repositories, services, handlers, and routes.
type App struct {
	logger       logger.Logger
	db           *sqlkit.DB
	router       *chi.Mux
	validator    validation.Validator
	repositories *repositories
	service      *service
	handler      *handler
}

// NewApp returns an App ready to be Initialize()d.
func NewApp(logger logger.Logger, db *sqlkit.DB, router *chi.Mux, validator validation.Validator) *App {
	return &App{logger: logger, db: db, router: router, validator: validator}
}

// Initialize wires repositories, services, handlers, and routes for every feature.
func (a *App) Initialize() {
	a.repositories = a.initializeRepository(a.logger, a.db)
	a.service = a.initializeService(a.logger, a.repositories)
	a.handler = a.initializeHandler(a.logger, a.validator, a.service)
	a.initializeRoutes(a.logger, a.router, a.handler)
}

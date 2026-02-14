package app

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/sqlkit"
	"github.com/go-chi/chi/v5"
)

type App struct {
	options    Options
	logger     logger.Logger
	db         *sqlkit.DB
	router     *chi.Mux
	repository *repository
	service    *service
	handler    *handler
}

type Options struct {
	Repository RepositoryOptions
	Service    ServiceOptions
	Handler    HandlerOptions
	Router     RouterOptions
}

func NewApp(options Options, logger logger.Logger, db *sqlkit.DB, router *chi.Mux) *App {
	return &App{options: options, logger: logger, db: db, router: router}
}

func (a *App) Initialize() {
	a.repository = a.initializeRepository(a.logger, a.options.Repository, a.db)
	a.service = a.initializeService(a.logger, a.options.Service, a.repository)
	a.handler = a.initializeHandler(a.logger, a.options.Handler, a.service)
	a.initializeRoutes(a.logger, a.options.Router, a.router, a.handler)
}

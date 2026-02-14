package app

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/sqlkit"
	"github.com/go-chi/chi/v5"
)

type App struct {
	options Options
	logger  logger.Logger
	db      *sqlkit.DB
	mux     *chi.Mux
}

type Options struct{}

func NewApp(options Options, logger logger.Logger, db *sqlkit.DB, mux *chi.Mux) *App {
	return &App{options: options, logger: logger, db: db, mux: mux}
}

func (a *App) Initialize() {
	domain := a.initializeDomain(a.logger, a.db)
	service := a.initializeService(a.logger, domain)
	handler := a.initializeHandler(a.logger, service)
	a.initializeRoutes(a.logger, a.mux, handler)
}

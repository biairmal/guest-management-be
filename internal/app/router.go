package app

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/go-chi/chi/v5"
)

func (a *App) initializeRoutes(_ logger.Logger, mux *chi.Mux, handler *handler) {
	events.InitCategoryRoutes(mux, handler.categoryHandler)
}

package app

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/go-chi/chi/v5"
)

type RouterOptions struct {
	Category events.CategoryRouterOptions
}

func (a *App) initializeRoutes(_ logger.Logger, options RouterOptions, mux *chi.Mux, handler *handler) {
	events.InitCategoryRoutes(options.Category, mux, handler.categoryHandler)
}

package app

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/guest-management-be/internal/features/events"
)

type handler struct {
	categoryHandler *events.CategoryHandler
}

type HandlerOptions struct {
	Category events.CategoryHandlerOptions
}

func (a *App) initializeHandler(_ logger.Logger, options HandlerOptions, service *service) *handler {
	return &handler{
		categoryHandler: events.NewCategoryHandler(options.Category, service.categoryService),
	}
}

package app

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/guest-management-be/internal/core/validation"
	"github.com/biairmal/guest-management-be/internal/features/events"
)

type handler struct {
	categoryHandler *events.CategoryHandler
}

func (a *App) initializeHandler(_ logger.Logger, validator validation.Validator, service *service) *handler {
	return &handler{
		categoryHandler: events.NewCategoryHandler(service.categoryService, validator),
	}
}

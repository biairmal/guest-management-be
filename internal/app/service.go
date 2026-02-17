package app

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/guest-management-be/internal/features/events"
)

type service struct {
	categoryService events.CategoryService
}

type ServiceOptions struct {
	Category events.CategoryServiceOptions
}

func (a *App) initializeService(logger logger.Logger, options ServiceOptions, repository *repository) *service {
	return &service{
		categoryService: events.NewCategoryService(options.Category, logger, repository.categoryRepository),
	}
}

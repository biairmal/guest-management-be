package app

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/guest-management-be/internal/features/events"
)

type service struct {
	categoryService events.CategoryService
}

func (a *App) initializeService(logger logger.Logger, domain *domain) *service {
	return &service{
		categoryService: *events.NewCategoryService(logger, domain.categoryRepository),
	}
}

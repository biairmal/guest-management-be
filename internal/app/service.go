package app

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/guest-management-be/internal/features/events"
)

type service struct {
	categoryService events.CategoryService
}

func (a *App) initializeService(logger logger.Logger, repositories *repositories) *service {
	return &service{
		categoryService: events.NewCategoryService(logger, repositories.categoryRepository),
	}
}

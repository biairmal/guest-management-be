package app

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/sqlkit"
	"github.com/biairmal/guest-management-be/internal/features/events"
)

type repository struct {
	categoryRepository events.CategoryRepository
}

type RepositoryOptions struct {
	Category events.CategoryRepositoryOptions
}

func (a *App) initializeRepository(_ logger.Logger, options RepositoryOptions, db *sqlkit.DB) *repository {
	return &repository{
		categoryRepository: events.NewCategoryRepository(options.Category, db),
	}
}

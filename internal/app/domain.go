package app

import (
	"github.com/biairmal/go-sdk/logger"
	"github.com/biairmal/go-sdk/sqlkit"
	"github.com/biairmal/guest-management-be/internal/features/events"
)

type domain struct {
	categoryRepository events.CategoryRepository
}

func (a *App) initializeDomain(_ logger.Logger, db *sqlkit.DB) *domain {
	return &domain{
		categoryRepository: events.NewCategoryRepository(db),
	}
}

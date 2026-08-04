package app

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
)

type service struct {
	categoryService events.CategoryService
	tenantService   tenants.TenantService
}

func (a *App) initializeService(logger logger.Logger, repositories *repositories) *service {
	return &service{
		categoryService: events.NewCategoryService(logger, repositories.categoryRepository),
		tenantService:   tenants.NewTenantService(logger, repositories.tenantRepository),
	}
}

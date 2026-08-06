package app

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/guest-management-be/internal/core/validation"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/users"
)

type handler struct {
	categoryHandler *events.CategoryHandler
	tenantHandler   *tenants.TenantHandler
	userHandler     *users.UserHandler
}

func (a *App) initializeHandler(_ logger.Logger, validator validation.Validator, service *service) *handler {
	return &handler{
		categoryHandler: events.NewCategoryHandler(service.categoryService, validator),
		tenantHandler:   tenants.NewTenantHandler(service.tenantService, validator),
		userHandler:     users.NewUserHandler(service.userService, validator),
	}
}

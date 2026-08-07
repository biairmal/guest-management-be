package app

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/guest-management-be/internal/core/validation"
	appauth "github.com/biairmal/guest-management-be/internal/features/auth"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/users"
)

type handler struct {
	categoryHandler     *events.CategoryHandler
	eventHandler        *events.EventHandler
	workflowStepHandler *events.WorkflowStepHandler
	tenantHandler       *tenants.TenantHandler
	userHandler         *users.UserHandler
	authHandler         *appauth.Handler
}

func (a *App) initializeHandler(_ logger.Logger, validator validation.Validator, service *service) *handler {
	return &handler{
		categoryHandler:     events.NewCategoryHandler(service.categoryService, validator),
		eventHandler:        events.NewEventHandler(service.eventService, validator),
		workflowStepHandler: events.NewWorkflowStepHandler(service.workflowStepService, validator),
		tenantHandler:       tenants.NewTenantHandler(service.tenantService, validator),
		userHandler:         users.NewUserHandler(service.userService, validator),
		authHandler:         appauth.NewHandler(service.authService, validator),
	}
}

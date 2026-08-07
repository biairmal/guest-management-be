package app

import (
	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/logger"
	appconfig "github.com/biairmal/guest-management-be/internal/config"
	appauth "github.com/biairmal/guest-management-be/internal/features/auth"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/users"
)

type service struct {
	categoryService     events.CategoryService
	eventService        events.EventService
	workflowStepService events.WorkflowStepService
	tenantService       tenants.TenantService
	userService         users.UserService
	authService         appauth.Service
}

func (a *App) initializeService(
	logger logger.Logger, repositories *repositories,
	authIssuer sdkauth.Issuer, authValidator sdkauth.Validator, authConfig *appconfig.AuthConfig,
) *service {
	return &service{
		categoryService: events.NewCategoryService(logger, repositories.categoryRepository),
		eventService: events.NewEventService(
			logger, repositories.eventRepository,
			repositories.workflowStepTemplateRepository, repositories.workflowStepRepository,
		),
		workflowStepService: events.NewWorkflowStepService(logger, repositories.workflowStepRepository),
		tenantService:       tenants.NewTenantService(logger, repositories.tenantRepository),
		userService:         users.NewUserService(logger, repositories.userRepository),
		authService: appauth.NewService(
			logger, repositories.userRepository, authIssuer, authValidator,
			authConfig.Token.Issuer.DefaultTTL, authConfig.RefreshTTL,
		),
	}
}

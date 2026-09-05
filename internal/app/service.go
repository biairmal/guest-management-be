package app

import (
	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/logger"
	appconfig "github.com/biairmal/guest-management-be/internal/config"
	coreauthz "github.com/biairmal/guest-management-be/internal/core/authz"
	appauth "github.com/biairmal/guest-management-be/internal/features/auth"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/roles"
	"github.com/biairmal/guest-management-be/internal/features/staffing"
	"github.com/biairmal/guest-management-be/internal/features/templates"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/users"
)

type service struct {
	categoryService             events.CategoryService
	eventService                events.EventService
	workflowStepService         events.WorkflowStepService
	workflowStepTemplateService events.WorkflowStepTemplateService
	tenantService               tenants.TenantService
	userService                 users.UserService
	authService                 appauth.Service
	messageTemplateService      templates.MessageTemplateService
	staffAssignmentService      staffing.StaffAssignmentService
	authzChecker                *coreauthz.Checker
}

func (a *App) initializeService(
	logger logger.Logger, repositories *repositories,
	authIssuer sdkauth.Issuer, authValidator sdkauth.Validator, authConfig *appconfig.AuthConfig,
) *service {
	permissionResolver := roles.NewPermissionResolver(repositories.rolePermissionRepository)
	authzChecker := coreauthz.NewChecker(logger, permissionResolver)

	return &service{
		categoryService: events.NewCategoryService(logger, repositories.categoryRepository),
		eventService: events.NewEventService(
			logger, repositories.eventRepository,
			repositories.workflowStepTemplateRepository, repositories.workflowStepRepository,
		),
		workflowStepService: events.NewWorkflowStepService(logger, repositories.workflowStepRepository),
		workflowStepTemplateService: events.NewWorkflowStepTemplateService(
			logger, repositories.workflowStepTemplateRepository,
		),
		tenantService: tenants.NewTenantService(logger, repositories.tenantRepository),
		userService:   users.NewUserService(logger, repositories.userRepository, repositories.roleRepository),
		authService: appauth.NewService(
			logger, repositories.userRepository, authIssuer, authValidator,
			authConfig.Token.Issuer.DefaultTTL, authConfig.RefreshTTL,
		),
		messageTemplateService: templates.NewMessageTemplateService(logger, repositories.messageTemplateRepository),
		staffAssignmentService: staffing.NewStaffAssignmentService(
			logger, repositories.staffAssignmentRepository, repositories.eventRepository,
			repositories.userRepository, repositories.roleRepository,
		),
		authzChecker: authzChecker,
	}
}

package app

import (
	sdkauth "github.com/biairmal/go-sdk/lib/auth"
	"github.com/biairmal/go-sdk/lib/logger"
	appconfig "github.com/biairmal/guest-management-be/internal/config"
	coreauthz "github.com/biairmal/guest-management-be/internal/core/authz"
	appauth "github.com/biairmal/guest-management-be/internal/features/auth"
	"github.com/biairmal/guest-management-be/internal/features/events/category"
	"github.com/biairmal/guest-management-be/internal/features/events/event"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowstep"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowsteptemplate"
	"github.com/biairmal/guest-management-be/internal/features/guests"
	"github.com/biairmal/guest-management-be/internal/features/roles"
	"github.com/biairmal/guest-management-be/internal/features/scans"
	"github.com/biairmal/guest-management-be/internal/features/staffing"
	"github.com/biairmal/guest-management-be/internal/features/templates"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/tickets"
	"github.com/biairmal/guest-management-be/internal/features/users"
)

type service struct {
	categoryService             category.Service
	eventService                event.Service
	workflowStepService         workflowstep.Service
	workflowStepTemplateService workflowsteptemplate.Service
	tenantService               tenants.TenantService
	userService                 users.UserService
	authService                 appauth.Service
	messageTemplateService      templates.MessageTemplateService
	staffAssignmentService      staffing.StaffAssignmentService
	ticketTypeService           tickets.TicketTypeService
	guestService                guests.GuestService
	scanLogService              scans.ScanLogService
	authzChecker                *coreauthz.Checker
}

func (a *App) initializeService(
	logger logger.Logger, repositories *repositories,
	authIssuer sdkauth.Issuer, authValidator sdkauth.Validator, authConfig *appconfig.AuthConfig,
) *service {
	permissionResolver := roles.NewPermissionResolver(repositories.rolePermissionRepository)
	authzChecker := coreauthz.NewChecker(logger, permissionResolver)

	return &service{
		categoryService: category.NewService(logger, repositories.categoryRepository),
		eventService: event.NewService(
			logger, repositories.eventRepository,
			repositories.workflowStepTemplateRepository, repositories.workflowStepRepository,
		),
		workflowStepService: workflowstep.NewService(logger, repositories.workflowStepRepository),
		workflowStepTemplateService: workflowsteptemplate.NewService(
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
		ticketTypeService: tickets.NewTicketTypeService(
			logger, repositories.ticketTypeRepository,
			repositories.ticketTypeWorkflowStepRepository, repositories.workflowStepRepository,
		),
		guestService: guests.NewGuestService(
			logger, repositories.guestRepository, repositories.ticketRepository,
			repositories.eventRepository, repositories.ticketTypeRepository,
			repositories.guestPIIEncryptor, newQueueInvitationPublisher(a.queuePublisher),
		),
		scanLogService: scans.NewScanLogService(
			logger, repositories.scanLogRepository, repositories.ticketRepository,
			repositories.workflowStepRepository, repositories.ticketTypeWorkflowStepRepository,
		),
		authzChecker: authzChecker,
	}
}

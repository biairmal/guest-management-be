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
	"github.com/biairmal/guest-management-be/internal/features/guests"
	"github.com/biairmal/guest-management-be/internal/features/roles"
	"github.com/biairmal/guest-management-be/internal/features/scans"
	"github.com/biairmal/guest-management-be/internal/features/staffing"
	"github.com/biairmal/guest-management-be/internal/features/templates"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/tickets/tickettype"
	"github.com/biairmal/guest-management-be/internal/features/tickets/tickettypetemplate"
	"github.com/biairmal/guest-management-be/internal/features/users"
)

type service struct {
	categoryService        category.Service
	eventService           event.Service
	workflowStepService    workflowstep.Service
	tenantService          tenants.TenantService
	userService            users.UserService
	authService            appauth.Service
	messageTemplateService templates.MessageTemplateService
	staffAssignmentService staffing.StaffAssignmentService
	ticketTypeService      tickettype.TicketTypeService
	guestService           guests.GuestService
	scanLogService         scans.ScanLogService
	authzChecker           *coreauthz.Checker
}

func (a *App) initializeService(
	logger logger.Logger, repositories *repositories,
	authIssuer sdkauth.Issuer, authValidator sdkauth.Validator, authConfig *appconfig.AuthConfig,
) *service {
	permissionResolver := roles.NewPermissionResolver(repositories.rolePermissionRepository)
	eventRoleResolver := staffing.NewEventRoleResolver(repositories.staffAssignmentRepository)
	authzChecker := coreauthz.NewChecker(logger, permissionResolver, eventRoleResolver)

	// ticketTypeService is constructed before eventService: eventService.Create
	// needs it as its TicketTypeSeeder (B12) — see event.NewService below.
	ticketTypeService := tickettype.NewTicketTypeService(
		logger, repositories.ticketTypeRepository, repositories.ticketTypeWorkflowStepRepository,
		repositories.ticketTypeTemplateRepository, repositories.ticketTypeTemplateStepRepository,
	)
	// The events side of an event's setup save (B16): the tickets-side store is
	// event's TicketTypeStore and the guests-side aggregate reader is its
	// GuestStats, both wired here so events never imports tickets or guests.
	// workflowStepService is a same-slice collaborator (its Sync reconciles the
	// setup's steps), so it is constructed before eventService too.
	workflowStepService := workflowstep.NewService(logger, repositories.workflowStepRepository)

	return &service{
		// The tickets-side Store is the category's TicketTypeTemplateStore
		// (B15) — wired here so events never imports tickets.
		categoryService: category.NewService(
			logger, repositories.categoryRepository, repositories.categoryVersionRepository,
			repositories.workflowStepTemplateRepository,
			tickettypetemplate.NewStore(
				logger, repositories.ticketTypeTemplateRepository, repositories.ticketTypeTemplateStepRepository,
			),
			a.db,
		),
		eventService: event.NewService(
			logger, repositories.eventRepository, repositories.eventTenantRepository,
			repositories.workflowStepTemplateRepository, repositories.workflowStepRepository,
			workflowStepService, ticketTypeService,
			tickettype.NewSetupStore(
				logger, repositories.ticketTypeRepository, repositories.ticketTypeWorkflowStepRepository,
			),
			guests.NewGuestStats(logger, a.db),
			repositories.categoryVersionRepository, a.db,
		),
		workflowStepService: workflowStepService,
		tenantService:       tenants.NewTenantService(logger, repositories.tenantRepository),
		userService: users.NewUserService(
			logger, repositories.userRepository, repositories.roleRepository, a.db,
		),
		authService: appauth.NewService(
			logger, repositories.userRepository, authIssuer, authValidator,
			authConfig.Token.Issuer.DefaultTTL, authConfig.RefreshTTL,
		),
		messageTemplateService: templates.NewMessageTemplateService(logger, repositories.messageTemplateRepository),
		staffAssignmentService: staffing.NewStaffAssignmentService(
			logger, repositories.staffAssignmentRepository, repositories.eventRepository,
			repositories.userRepository, repositories.roleRepository,
		),
		ticketTypeService: ticketTypeService,
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

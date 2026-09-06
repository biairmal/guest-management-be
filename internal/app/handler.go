package app

import (
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/biairmal/guest-management-be/internal/core/validation"
	appauth "github.com/biairmal/guest-management-be/internal/features/auth"
	"github.com/biairmal/guest-management-be/internal/features/events/category"
	"github.com/biairmal/guest-management-be/internal/features/events/event"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowstep"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowsteptemplate"
	"github.com/biairmal/guest-management-be/internal/features/guests"
	"github.com/biairmal/guest-management-be/internal/features/staffing"
	"github.com/biairmal/guest-management-be/internal/features/templates"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/tickets"
	"github.com/biairmal/guest-management-be/internal/features/users"
)

type handler struct {
	categoryHandler             *category.Handler
	eventHandler                *event.Handler
	workflowStepHandler         *workflowstep.Handler
	workflowStepTemplateHandler *workflowsteptemplate.Handler
	tenantHandler               *tenants.TenantHandler
	userHandler                 *users.UserHandler
	authHandler                 *appauth.Handler
	messageTemplateHandler      *templates.MessageTemplateHandler
	staffAssignmentHandler      *staffing.StaffAssignmentHandler
	ticketTypeHandler           *tickets.TicketTypeHandler
	guestHandler                *guests.GuestHandler
}

func (a *App) initializeHandler(_ logger.Logger, validator validation.Validator, service *service) *handler {
	return &handler{
		categoryHandler:     category.NewHandler(service.categoryService, validator),
		eventHandler:        event.NewHandler(service.eventService, validator),
		workflowStepHandler: workflowstep.NewHandler(service.workflowStepService, validator),
		workflowStepTemplateHandler: workflowsteptemplate.NewHandler(
			service.workflowStepTemplateService, validator,
		),
		tenantHandler:          tenants.NewTenantHandler(service.tenantService, validator),
		userHandler:            users.NewUserHandler(service.userService, validator),
		authHandler:            appauth.NewHandler(service.authService, validator),
		messageTemplateHandler: templates.NewMessageTemplateHandler(service.messageTemplateService, validator),
		staffAssignmentHandler: staffing.NewStaffAssignmentHandler(service.staffAssignmentService, validator),
		ticketTypeHandler:      tickets.NewTicketTypeHandler(service.ticketTypeService, validator),
		guestHandler:           guests.NewGuestHandler(service.guestService, validator),
	}
}

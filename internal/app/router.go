package app

import (
	"github.com/biairmal/go-sdk/lib/logger"
	coreauthz "github.com/biairmal/guest-management-be/internal/core/authz"
	appauth "github.com/biairmal/guest-management-be/internal/features/auth"
	"github.com/biairmal/guest-management-be/internal/features/events/category"
	"github.com/biairmal/guest-management-be/internal/features/events/event"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowstep"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowsteptemplate"
	"github.com/biairmal/guest-management-be/internal/features/guests"
	"github.com/biairmal/guest-management-be/internal/features/scans"
	"github.com/biairmal/guest-management-be/internal/features/staffing"
	"github.com/biairmal/guest-management-be/internal/features/templates"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/tickets"
	"github.com/biairmal/guest-management-be/internal/features/users"
	"github.com/go-chi/chi/v5"
)

// initializeRoutes registers every feature's HTTP routes on mux. checker is
// the shared authz.Checker built in initializeService — staffing (and later
// phases reusing internal/core/authz) needs it to gate routes on a
// permission code, a cross-cutting collaborator beyond the usual
// InitXRoutes(mux, handler) shape.
func (a *App) initializeRoutes(_ logger.Logger, mux *chi.Mux, handler *handler, checker *coreauthz.Checker) {
	category.InitCategoryRoutes(mux, handler.categoryHandler)
	event.InitEventRoutes(mux, handler.eventHandler)
	workflowstep.InitWorkflowStepRoutes(mux, handler.workflowStepHandler)
	workflowsteptemplate.InitWorkflowStepTemplateRoutes(mux, handler.workflowStepTemplateHandler)
	tenants.InitTenantRoutes(mux, handler.tenantHandler)
	users.InitUserRoutes(mux, handler.userHandler, checker)
	appauth.InitAuthRoutes(mux, handler.authHandler)
	templates.InitMessageTemplateRoutes(mux, handler.messageTemplateHandler)
	staffing.InitStaffAssignmentRoutes(mux, handler.staffAssignmentHandler, checker)
	tickets.InitTicketTypeRoutes(mux, handler.ticketTypeHandler, checker)
	guests.InitGuestRoutes(mux, handler.guestHandler, checker)
	guests.InitGuestRSVPRoutes(mux, handler.guestHandler)
	scans.InitScanLogRoutes(mux, handler.scanLogHandler, checker)
}

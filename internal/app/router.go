package app

import (
	"github.com/biairmal/go-sdk/lib/logger"
	coreauthz "github.com/biairmal/guest-management-be/internal/core/authz"
	appauth "github.com/biairmal/guest-management-be/internal/features/auth"
	"github.com/biairmal/guest-management-be/internal/features/events/category"
	"github.com/biairmal/guest-management-be/internal/features/events/event"
	"github.com/biairmal/guest-management-be/internal/features/events/workflowstep"
	"github.com/biairmal/guest-management-be/internal/features/guests"
	"github.com/biairmal/guest-management-be/internal/features/scans"
	"github.com/biairmal/guest-management-be/internal/features/staffing"
	"github.com/biairmal/guest-management-be/internal/features/templates"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/tickets/tickettype"
	"github.com/biairmal/guest-management-be/internal/features/users"
	"github.com/go-chi/chi/v5"
)

// initializeRoutes registers every feature's HTTP routes on mux. checker is
// the shared authz.Checker built in initializeService — staffing (and later
// phases reusing internal/core/authz) needs it to gate routes on a
// permission code, a cross-cutting collaborator beyond the usual
// InitXRoutes(mux, handler) shape.
//
// inTenant is the second such collaborator: one authz.RequireEventInTenant
// middleware, built here over events/event's tenant repository (so core never
// imports the feature) and shared by every {event_id}-scoped route group. It
// makes "another tenant's event is 404, not 403 and not data" one rule in one
// place instead of a check each feature could forget (B16 R4 / API-001).
func (a *App) initializeRoutes(
	log logger.Logger, mux *chi.Mux, handler *handler, repositories *repositories, checker *coreauthz.Checker,
) {
	inTenant := coreauthz.RequireEventInTenant(log, repositories.eventTenantRepository)

	category.InitCategoryRoutes(mux, handler.categoryHandler, checker)
	event.InitEventRoutes(mux, handler.eventHandler, checker, inTenant)
	workflowstep.InitWorkflowStepRoutes(mux, handler.workflowStepHandler, inTenant)
	tenants.InitTenantRoutes(mux, handler.tenantHandler)
	users.InitUserRoutes(mux, handler.userHandler, checker)
	appauth.InitAuthRoutes(mux, handler.authHandler)
	templates.InitMessageTemplateRoutes(mux, handler.messageTemplateHandler)
	staffing.InitStaffAssignmentRoutes(mux, handler.staffAssignmentHandler, checker, inTenant)
	tickettype.InitTicketTypeRoutes(mux, handler.ticketTypeHandler, inTenant)
	guests.InitGuestRoutes(mux, handler.guestHandler, checker, inTenant)
	guests.InitGuestRSVPRoutes(mux, handler.guestHandler)
	scans.InitScanLogRoutes(mux, handler.scanLogHandler, checker, inTenant)
}

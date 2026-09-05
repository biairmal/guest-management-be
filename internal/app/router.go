package app

import (
	"github.com/biairmal/go-sdk/lib/logger"
	coreauthz "github.com/biairmal/guest-management-be/internal/core/authz"
	appauth "github.com/biairmal/guest-management-be/internal/features/auth"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/staffing"
	"github.com/biairmal/guest-management-be/internal/features/templates"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/users"
	"github.com/go-chi/chi/v5"
)

// initializeRoutes registers every feature's HTTP routes on mux. checker is
// the shared authz.Checker built in initializeService — staffing (and later
// phases reusing internal/core/authz) needs it to gate routes on a
// permission code, a cross-cutting collaborator beyond the usual
// InitXRoutes(mux, handler) shape.
func (a *App) initializeRoutes(_ logger.Logger, mux *chi.Mux, handler *handler, checker *coreauthz.Checker) {
	events.InitCategoryRoutes(mux, handler.categoryHandler)
	events.InitEventRoutes(mux, handler.eventHandler)
	events.InitWorkflowStepRoutes(mux, handler.workflowStepHandler)
	events.InitWorkflowStepTemplateRoutes(mux, handler.workflowStepTemplateHandler)
	tenants.InitTenantRoutes(mux, handler.tenantHandler)
	users.InitUserRoutes(mux, handler.userHandler)
	appauth.InitAuthRoutes(mux, handler.authHandler)
	templates.InitMessageTemplateRoutes(mux, handler.messageTemplateHandler)
	staffing.InitStaffAssignmentRoutes(mux, handler.staffAssignmentHandler, checker)
}

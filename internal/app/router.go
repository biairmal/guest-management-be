package app

import (
	"github.com/biairmal/go-sdk/lib/logger"
	appauth "github.com/biairmal/guest-management-be/internal/features/auth"
	"github.com/biairmal/guest-management-be/internal/features/events"
	"github.com/biairmal/guest-management-be/internal/features/tenants"
	"github.com/biairmal/guest-management-be/internal/features/users"
	"github.com/go-chi/chi/v5"
)

func (a *App) initializeRoutes(_ logger.Logger, mux *chi.Mux, handler *handler) {
	events.InitCategoryRoutes(mux, handler.categoryHandler)
	events.InitEventRoutes(mux, handler.eventHandler)
	events.InitWorkflowStepRoutes(mux, handler.workflowStepHandler)
	tenants.InitTenantRoutes(mux, handler.tenantHandler)
	users.InitUserRoutes(mux, handler.userHandler)
	appauth.InitAuthRoutes(mux, handler.authHandler)
}

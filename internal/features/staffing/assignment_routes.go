package staffing

import (
	"net/http"

	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"

	"github.com/biairmal/guest-management-be/internal/core/authz"
)

// InitStaffAssignmentRoutes registers event-scoped staff assignment routes
// on the given router. Unlike other InitXRoutes functions in this codebase,
// this one takes a *authz.Checker: every route requires the manage_staff
// permission (see docs/STAFFING_RBAC.md ss6), the first feature in this
// service needing a cross-cutting authorization collaborator at the routing
// layer.
//
// inTenant is authz.RequireEventInTenant, built in internal/app and r.Use'd
// first, so another tenant's event is 404 before the permission check can
// answer 403. This service's own loadTenantScopedEvent still checks the tenant
// too (it also returns the event); the duplication is deliberate — the service
// stays correct for any transport, the middleware makes the rule uniform across
// every {event_id} route.
func InitStaffAssignmentRoutes(
	r *chi.Mux, h *StaffAssignmentHandler, checker *authz.Checker, inTenant func(http.Handler) http.Handler,
) {
	r.Route("/api/v1/events/{event_id}/staff", func(r chi.Router) {
		r.Use(inTenant)
		r.Use(authz.RequirePermission(checker, PermissionManageStaff))
		r.Get("/", handler.Handle(h.List))
		r.Post("/", handler.Handle(h.Create))
		r.Get("/{id}", handler.Handle(h.GetByID))
		r.Put("/{id}", handler.Handle(h.Update))
		r.Delete("/{id}", handler.Handle(h.Delete))
	})
}

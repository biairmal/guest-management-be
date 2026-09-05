package staffing

import (
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
func InitStaffAssignmentRoutes(r *chi.Mux, h *StaffAssignmentHandler, checker *authz.Checker) {
	r.Route("/api/v1/events/{event_id}/staff", func(r chi.Router) {
		r.Use(authz.RequirePermission(checker, PermissionManageStaff))
		r.Get("/", handler.Handle(h.List))
		r.Post("/", handler.Handle(h.Create))
		r.Get("/{id}", handler.Handle(h.GetByID))
		r.Put("/{id}", handler.Handle(h.Update))
		r.Delete("/{id}", handler.Handle(h.Delete))
	})
}

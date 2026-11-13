package event

import (
	"net/http"

	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"

	"github.com/biairmal/guest-management-be/internal/core/authz"
)

// InitEventRoutes registers event routes on the given router. Reads are open to
// any authenticated caller (the service applies the tenant scope from the JWT);
// writes require the manage_events permission.
//
// inTenant is authz.RequireEventInTenant, built once in internal/app and shared
// by every {event_id}-scoped route group: it 404s an event outside the caller's
// tenant before the permission check runs, so another tenant's event is absent
// rather than forbidden.
func InitEventRoutes(
	r *chi.Mux, eventH *Handler, checker *authz.Checker, inTenant func(http.Handler) http.Handler,
) {
	r.Route("/api/v1/events", func(r chi.Router) {
		r.Get("/", handler.Handle(eventH.List))
		r.Get("/{id}", handler.Handle(eventH.GetByID))
		r.Group(func(r chi.Router) {
			r.Use(authz.RequirePermission(checker, PermissionManageEvents))
			r.Post("/", handler.Handle(eventH.Create))
			r.Put("/{id}", handler.Handle(eventH.Update))
			r.Delete("/{id}", handler.Handle(eventH.Delete))
		})
	})

	r.Route("/api/v1/events/{event_id}/setup", func(r chi.Router) {
		r.Use(inTenant)
		r.Get("/", handler.Handle(eventH.GetSetup))
		r.Group(func(r chi.Router) {
			r.Use(authz.RequirePermission(checker, PermissionManageEvents))
			r.Put("/", handler.Handle(eventH.ReplaceSetup))
		})
	})
}

package tickets

import (
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"

	"github.com/biairmal/guest-management-be/internal/core/authz"
)

// InitTicketTypeRoutes registers event-scoped ticket type routes on the
// given router. Every route requires the manage_events permission (see
// tickets_constants.go) — the first phase of ticket-type management, unlike
// events/workflow-steps, gates on a permission code from creation.
func InitTicketTypeRoutes(r *chi.Mux, h *TicketTypeHandler, checker *authz.Checker) {
	r.Route("/api/v1/events/{event_id}/ticket-types", func(r chi.Router) {
		r.Use(authz.RequirePermission(checker, PermissionManageEvents))
		r.Get("/", handler.Handle(h.List))
		r.Post("/", handler.Handle(h.Create))
		r.Get("/{id}", handler.Handle(h.GetByID))
		r.Put("/{id}", handler.Handle(h.Update))
		r.Delete("/{id}", handler.Handle(h.Delete))
		r.Put("/{id}/workflow-steps", handler.Handle(h.ReplaceWorkflowSteps))
	})
}

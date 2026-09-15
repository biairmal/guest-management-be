package tickettypetemplate

import (
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"

	"github.com/biairmal/guest-management-be/internal/core/authz"
	"github.com/biairmal/guest-management-be/internal/features/tickets"
)

// InitTicketTypeTemplateRoutes registers category-scoped ticket type
// template routes on the given router. Every route requires the
// manage_events permission (tickets.PermissionManageEvents, declared at the
// tickets feature root) — matched to the gate on the ticket_types the
// template seeds (B7), not copied verbatim from the sibling
// workflow-step-templates feature (see docs/DEVELOPMENT_PLAN.md B12).
func InitTicketTypeTemplateRoutes(r *chi.Mux, h *Handler, checker *authz.Checker) {
	r.Route("/api/v1/event-categories/{category_id}/ticket-type-templates", func(r chi.Router) {
		r.Use(authz.RequirePermission(checker, tickets.PermissionManageEvents))
		r.Get("/", handler.Handle(h.List))
		r.Post("/", handler.Handle(h.Create))
		r.Get("/{id}", handler.Handle(h.GetByID))
		r.Put("/{id}", handler.Handle(h.Update))
		r.Delete("/{id}", handler.Handle(h.Delete))
	})
}

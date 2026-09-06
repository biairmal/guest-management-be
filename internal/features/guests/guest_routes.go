package guests

import (
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"

	"github.com/biairmal/guest-management-be/internal/core/authz"
)

// InitGuestRoutes registers event-scoped guest routes on the given router.
// Every route requires the manage_guests permission (see
// guests_constants.go).
func InitGuestRoutes(r *chi.Mux, h *GuestHandler, checker *authz.Checker) {
	r.Route("/api/v1/events/{event_id}/guests", func(r chi.Router) {
		r.Use(authz.RequirePermission(checker, PermissionManageGuests))
		r.Get("/", handler.Handle(h.List))
		r.Post("/", handler.Handle(h.Create))
		r.Get("/{id}", handler.Handle(h.GetByID))
		r.Put("/{id}", handler.Handle(h.Update))
		r.Delete("/{id}", handler.Handle(h.Delete))
		r.Post("/{id}/invitation", handler.Handle(h.SendInvitation))
	})
}

// InitGuestRSVPRoutes registers the public, unauthenticated guest RSVP
// route. Deliberately separate from InitGuestRoutes: it takes no
// *authz.Checker (nothing to authorize — the invitation token is the only
// gate) and must be marked public in the route policy (configs/config.yaml
// auth.token.rules), the same mechanism auth's /login and /refresh use.
func InitGuestRSVPRoutes(r *chi.Mux, h *GuestHandler) {
	r.Route("/api/v1/guests/rsvp", func(r chi.Router) {
		r.Post("/{token}", handler.Handle(h.RSVP))
	})
}

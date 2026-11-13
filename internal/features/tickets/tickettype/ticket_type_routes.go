package tickettype

import (
	"net/http"

	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"
)

// InitTicketTypeRoutes registers event-scoped ticket type routes on the given
// router. Reads only: since B16 the event's setup save
// (PUT /api/v1/events/{event_id}/setup) is the single way to change an event's
// ticket types, so two write paths can't drift. These reads stay because the
// guest ticket-type picker needs the list, and they no longer require
// manage_events — a caller who may register guests must be able to see the
// ticket types to pick from.
//
// inTenant is authz.RequireEventInTenant, built in internal/app: a ticket type
// of another tenant's event reads as 404, not as data.
func InitTicketTypeRoutes(r *chi.Mux, h *TicketTypeHandler, inTenant func(http.Handler) http.Handler) {
	r.Route("/api/v1/events/{event_id}/ticket-types", func(r chi.Router) {
		r.Use(inTenant)
		r.Get("/", handler.Handle(h.List))
		r.Get("/{id}", handler.Handle(h.GetByID))
	})
}

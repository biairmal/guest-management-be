package workflowstep

import (
	"net/http"

	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"
)

// InitWorkflowStepRoutes registers event-scoped workflow step routes on the
// given router. Reads only: since B16 the event's setup save
// (PUT /api/v1/events/{event_id}/setup) is the single way to change an event's
// steps, so two write paths can't drift. These reads stay because the guest
// ticket-type picker and the scan step picker need the step list, and ushers
// need it without manage_events.
//
// inTenant is authz.RequireEventInTenant, built in internal/app: a step of
// another tenant's event reads as 404, not as data.
func InitWorkflowStepRoutes(r *chi.Mux, workflowStepH *Handler, inTenant func(http.Handler) http.Handler) {
	r.Route("/api/v1/events/{event_id}/workflow-steps", func(r chi.Router) {
		r.Use(inTenant)
		r.Get("/", handler.Handle(workflowStepH.List))
		r.Get("/{id}", handler.Handle(workflowStepH.GetByID))
	})
}

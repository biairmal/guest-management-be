package events

import (
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"
)

// InitWorkflowStepRoutes registers event-scoped workflow step routes on the given router.
func InitWorkflowStepRoutes(r *chi.Mux, workflowStepH *WorkflowStepHandler) {
	r.Route("/api/v1/events/{event_id}/workflow-steps", func(r chi.Router) {
		r.Get("/", handler.Handle(workflowStepH.List))
		r.Put("/", handler.Handle(workflowStepH.Sync))
		r.Get("/{id}", handler.Handle(workflowStepH.GetByID))
		r.Post("/", handler.Handle(workflowStepH.Create))
		r.Put("/{id}", handler.Handle(workflowStepH.Update))
		r.Delete("/{id}", handler.Handle(workflowStepH.Delete))
	})
}

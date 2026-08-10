package events

import (
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"
)

// InitWorkflowStepTemplateRoutes registers category-scoped workflow step template routes on the given router.
func InitWorkflowStepTemplateRoutes(r *chi.Mux, workflowStepTemplateH *WorkflowStepTemplateHandler) {
	r.Route("/api/v1/event-categories/{category_id}/workflow-step-templates", func(r chi.Router) {
		r.Get("/", handler.Handle(workflowStepTemplateH.List))
		r.Get("/{id}", handler.Handle(workflowStepTemplateH.GetByID))
		r.Post("/", handler.Handle(workflowStepTemplateH.Create))
		r.Put("/{id}", handler.Handle(workflowStepTemplateH.Update))
		r.Delete("/{id}", handler.Handle(workflowStepTemplateH.Delete))
	})
}

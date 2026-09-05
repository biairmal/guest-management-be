package event

import (
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"
)

// InitEventRoutes registers event routes on the given router.
func InitEventRoutes(r *chi.Mux, eventH *Handler) {
	r.Route("/api/v1/events", func(r chi.Router) {
		r.Get("/", handler.Handle(eventH.List))
		r.Get("/{id}", handler.Handle(eventH.GetByID))
		r.Post("/", handler.Handle(eventH.Create))
		r.Put("/{id}", handler.Handle(eventH.Update))
		r.Delete("/{id}", handler.Handle(eventH.Delete))
	})
}

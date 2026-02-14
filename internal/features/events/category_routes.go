package events

import (
	"github.com/biairmal/go-sdk/httpkit/handler"
	"github.com/go-chi/chi/v5"
)

type CategoryRouterOptions struct{}

func InitCategoryRoutes(_ CategoryRouterOptions, r *chi.Mux, categoryH *CategoryHandler) {
	r.Route("/api/v1/event-categories", func(r chi.Router) {
		r.Get("/", handler.Handle(categoryH.List))
		r.Get("/{id}", handler.Handle(categoryH.GetByID))
		r.Post("/", handler.Handle(categoryH.Create))
		r.Put("/{id}", handler.Handle(categoryH.Update))
		r.Delete("/{id}", handler.Handle(categoryH.Delete))
	})
}

package users

import (
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"
)

// InitUserRoutes registers user routes on the given router.
func InitUserRoutes(r *chi.Mux, userH *UserHandler) {
	r.Route("/api/v1/users", func(r chi.Router) {
		r.Get("/", handler.Handle(userH.List))
		r.Get("/{id}", handler.Handle(userH.GetByID))
		r.Post("/", handler.Handle(userH.Create))
		r.Put("/{id}", handler.Handle(userH.Update))
		r.Delete("/{id}", handler.Handle(userH.Delete))
	})
}

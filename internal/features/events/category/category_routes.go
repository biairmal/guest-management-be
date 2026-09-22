package category

import (
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"

	"github.com/biairmal/guest-management-be/internal/core/authz"
)

// InitCategoryRoutes registers event category routes on the given router.
// Reads are open to any authenticated caller (the service applies tenant
// scope); writes require the manage_events permission.
func InitCategoryRoutes(r *chi.Mux, categoryH *Handler, checker *authz.Checker) {
	r.Route("/api/v1/event-categories", func(r chi.Router) {
		r.Get("/", handler.Handle(categoryH.List))
		r.Get("/{id}", handler.Handle(categoryH.GetByID))
		r.Group(func(r chi.Router) {
			r.Use(authz.RequirePermission(checker, PermissionManageEvents))
			r.Post("/", handler.Handle(categoryH.Create))
			r.Put("/{id}", handler.Handle(categoryH.Replace))
			r.Delete("/{id}", handler.Handle(categoryH.Delete))
		})
	})
}

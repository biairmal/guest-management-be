package tenants

import (
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"
)

// InitTenantRoutes registers tenant routes on the given router.
func InitTenantRoutes(r *chi.Mux, tenantH *TenantHandler) {
	r.Route("/api/v1/tenants", func(r chi.Router) {
		r.Get("/", handler.Handle(tenantH.List))
		r.Get("/{id}", handler.Handle(tenantH.GetByID))
		r.Post("/", handler.Handle(tenantH.Create))
		r.Put("/{id}", handler.Handle(tenantH.Update))
		r.Delete("/{id}", handler.Handle(tenantH.Delete))
	})
}

package users

import (
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"

	"github.com/biairmal/guest-management-be/internal/core/authz"
)

// InitUserRoutes registers user routes on the given router. checker gates
// list/get/create/update/delete/set-password/transfer-master on
// PermissionManageUsers (AC1). POST /me/password is registered outside that
// group — it is self-service-only (see UserService.SetOwnPassword) and needs
// only the valid-token requirement every /api/v1/users route already gets
// from the mux's auth middleware. The static "me" segment and the "{id}"
// param at the same path depth do not collide — chi tries static children
// before param children regardless of registration order (see
// docs/PATTERNS.md's "/me" sub-resource pattern).
func InitUserRoutes(r *chi.Mux, userH *UserHandler, checker *authz.Checker) {
	r.Route("/api/v1/users", func(r chi.Router) {
		r.Post("/me/password", handler.Handle(userH.SetOwnPassword))

		r.Group(func(r chi.Router) {
			r.Use(authz.RequirePermission(checker, PermissionManageUsers))
			r.Get("/", handler.Handle(userH.List))
			r.Get("/{id}", handler.Handle(userH.GetByID))
			r.Post("/", handler.Handle(userH.Create))
			r.Put("/{id}", handler.Handle(userH.Update))
			r.Delete("/{id}", handler.Handle(userH.Delete))
			r.Post("/{id}/password", handler.Handle(userH.SetPassword))
			r.Post("/{id}/transfer-master", handler.Handle(userH.TransferMaster))
		})
	})
}

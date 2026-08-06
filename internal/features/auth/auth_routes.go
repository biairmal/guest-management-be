package auth

import (
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"
)

// InitAuthRoutes registers auth routes on the given router. Both routes must
// be marked public in the auth route policy (configs/config.yaml
// auth.token.rules) — otherwise the global Auth middleware would require a
// valid access token to reach the very endpoints that issue one.
func InitAuthRoutes(r *chi.Mux, authH *Handler) {
	r.Route("/api/v1/auth", func(r chi.Router) {
		r.Post("/login", handler.Handle(authH.Login))
		r.Post("/refresh", handler.Handle(authH.Refresh))
	})
}

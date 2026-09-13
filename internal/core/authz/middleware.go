package authz

import (
	"net/http"

	"github.com/biairmal/go-sdk/lib/httpkit"
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RequirePermission returns a middleware requiring the caller — already
// authenticated by httpkit/middleware.Auth earlier in the chain — to hold
// code. When the route carries a valid {event_id} URL param it runs
// Checker.RequireForEvent (system role, falling back to an event-scoped
// assignment on that event); otherwise it runs the unscoped Checker.Require
// unchanged, so every gated route that has no event_id in its path keeps its
// original behavior with zero call-site changes. On failure it writes the
// standard error envelope with the status httpkit.StatusCodeFromError maps
// the errorz code to (401 no role claim, 403 missing permission, 500
// resolver failure) — the same WriteErrorResponse/StatusCodeFromError
// pattern go-sdk's own Auth middleware uses.
func RequirePermission(checker *Checker, code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var err error
			if eventID, ok := eventIDURLParam(r); ok {
				err = checker.RequireForEvent(r.Context(), eventID, code)
			} else {
				err = checker.Require(r.Context(), code)
			}
			if err != nil {
				handler.WriteErrorResponse(w, httpkit.StatusCodeFromError(err), err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// eventIDURLParam returns the route's {event_id} path param parsed as a
// UUID, and whether one was present and valid.
func eventIDURLParam(r *http.Request) (uuid.UUID, bool) {
	raw := chi.URLParam(r, "event_id")
	if raw == "" {
		return uuid.Nil, false
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		return uuid.Nil, false
	}
	return id, true
}

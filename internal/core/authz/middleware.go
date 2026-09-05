package authz

import (
	"net/http"

	"github.com/biairmal/go-sdk/lib/httpkit"
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
)

// RequirePermission returns a middleware requiring the caller — already
// authenticated by httpkit/middleware.Auth earlier in the chain — to hold
// code, per Checker.Require. On failure it writes the standard error
// envelope with the status httpkit.StatusCodeFromError maps the errorz code
// to (401 no role claim, 403 missing permission, 500 resolver failure) —
// the same WriteErrorResponse/StatusCodeFromError pattern go-sdk's own Auth
// middleware uses.
func RequirePermission(checker *Checker, code string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := checker.Require(r.Context(), code); err != nil {
				handler.WriteErrorResponse(w, httpkit.StatusCodeFromError(err), err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

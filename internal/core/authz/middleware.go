package authz

import (
	"net/http"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/httpkit"
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/biairmal/go-sdk/lib/logger"
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

// RequireEventInTenant returns a middleware that rejects any request whose
// {event_id} names an event outside the caller's tenant, with 404 — never 403
// and never data, so another tenant's event IDs cannot be probed (API-001,
// docs/STAFFING_RBAC.md ss6). Before this existed, authz.Checker never looked
// at tenant: a system role such as Tenant Admin passed RequirePermission for
// *any* event_id, and only staffing checked the event's tenant itself, so a
// tenant admin could read and change another tenant's guests, ticket types,
// steps and scans by knowing an event ID.
//
// Use it on every {event_id}-scoped route group, r.Use'd *before*
// RequirePermission: another tenant's event must read as absent (404), not as
// forbidden (403). Costs one primary-key read per request.
//
// A missing or unparsable {event_id} is 400 (the same message the handlers
// produce), a missing tenant claim 401, and a resolver failure 500 — written
// with the standard error envelope via handler.WriteErrorResponse, exactly as
// RequirePermission does.
func RequireEventInTenant(log logger.Logger, resolver EventTenantResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := checkEventInTenant(r, log, resolver); err != nil {
				handler.WriteErrorResponse(w, httpkit.StatusCodeFromError(err), err)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// checkEventInTenant is RequireEventInTenant's decision, split out so the
// middleware body stays a single branch.
func checkEventInTenant(r *http.Request, log logger.Logger, resolver EventTenantResolver) error {
	eventID, ok := eventIDURLParam(r)
	if !ok {
		return errorz.BadRequest().WithMessage("invalid event id")
	}
	callerTenant, ok := TenantIDFromContext(r.Context())
	if !ok {
		return errorz.Unauthorized().WithMessage("no tenant claim present")
	}
	owner, found, err := resolver.EventTenantID(r.Context(), eventID)
	if err != nil {
		log.ErrorWithContext(r.Context(), "authz: resolve event tenant failed",
			logger.F("event_id", eventID), logger.F("error", err))
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to resolve event tenant")
	}
	if !found || owner != callerTenant {
		return errorz.NotFound().WithMessage("event not found")
	}
	return nil
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

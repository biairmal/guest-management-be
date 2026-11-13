package event

import (
	"net/http"
	"testing"

	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/biairmal/guest-management-be/internal/core/authz"
)

// TestInitEventRoutes checks the routes register without a chi pattern clash
// (registering /api/v1/events and /api/v1/events/{event_id}/setup on the same
// mux) and that every path the story promises resolves — a mistake here would
// otherwise only show up as a panic or a 404 at runtime.
//
// (*chi.Mux).Match answers per pattern, not per method — an unregistered method
// on a registered pattern still matches, as chi's own 405 handler — so this
// checks the patterns exist, not which verbs they allow.
func TestInitEventRoutes(t *testing.T) {
	mux := chi.NewMux()
	InitEventRoutes(mux, NewHandler(nil, nil), authz.NewChecker(logger.NewNoOp(), nil, nil),
		func(next http.Handler) http.Handler { return next })

	id := uuid.NewString()
	routes := []struct {
		method string
		path   string
	}{
		{http.MethodGet, "/api/v1/events"},
		{http.MethodPost, "/api/v1/events"},
		{http.MethodGet, "/api/v1/events/" + id},
		{http.MethodPut, "/api/v1/events/" + id},
		{http.MethodDelete, "/api/v1/events/" + id},
		{http.MethodGet, "/api/v1/events/" + id + "/setup"},
		{http.MethodPut, "/api/v1/events/" + id + "/setup"},
	}
	for _, r := range routes {
		if !mux.Match(chi.NewRouteContext(), r.method, r.path) {
			t.Errorf("%s %s does not resolve", r.method, r.path)
		}
	}

	// Sanity check that Match discriminates at all, so the assertions above
	// aren't vacuously true.
	if mux.Match(chi.NewRouteContext(), http.MethodGet, "/api/v1/events/"+id+"/nope") {
		t.Error("an unregistered path resolves; Match is not discriminating")
	}
}

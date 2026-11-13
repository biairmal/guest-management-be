package scans

import (
	"net/http"

	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"

	"github.com/biairmal/guest-management-be/internal/core/authz"
)

// InitScanLogRoutes registers event-scoped scan routes on the given router.
// Every route requires the check_in permission (see scans_constants.go) —
// the same permission gates both recording a scan and reading scan history,
// per the Technical Design (no acceptance criterion asks for a narrower
// read-only tier).
//
// inTenant is authz.RequireEventInTenant, built in internal/app and r.Use'd
// first, so another tenant's event is 404 before the permission check runs.
//
// ponytail: that costs one primary-key read per scan. Acceptable for now;
// cache the event→tenant mapping only if scan latency (PRD-001 G3) shows it
// matters.
func InitScanLogRoutes(
	r *chi.Mux, h *ScanLogHandler, checker *authz.Checker, inTenant func(http.Handler) http.Handler,
) {
	r.Route("/api/v1/events/{event_id}/scans", func(r chi.Router) {
		r.Use(inTenant)
		r.Use(authz.RequirePermission(checker, PermissionCheckIn))
		r.Post("/", handler.Handle(h.RecordScan))
		r.Get("/", handler.Handle(h.List))
	})
}

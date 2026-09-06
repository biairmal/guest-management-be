package scans

import (
	"github.com/biairmal/go-sdk/lib/httpkit/handler"
	"github.com/go-chi/chi/v5"

	"github.com/biairmal/guest-management-be/internal/core/authz"
)

// InitScanLogRoutes registers event-scoped scan routes on the given router.
// Every route requires the check_in permission (see scans_constants.go) —
// the same permission gates both recording a scan and reading scan history,
// per the Technical Design (no acceptance criterion asks for a narrower
// read-only tier).
func InitScanLogRoutes(r *chi.Mux, h *ScanLogHandler, checker *authz.Checker) {
	r.Route("/api/v1/events/{event_id}/scans", func(r chi.Router) {
		r.Use(authz.RequirePermission(checker, PermissionCheckIn))
		r.Post("/", handler.Handle(h.RecordScan))
		r.Get("/", handler.Handle(h.List))
	})
}

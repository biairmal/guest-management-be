package scans

// PermissionCheckIn is the permission code (seeded in B6's catalog, migration
// 000014 — "scan tickets / record guest check-ins", docs/STAFFING_RBAC.md §3)
// required for both scan endpoints: recording a scan and reading scan
// history. Gated in scan_log_routes.go via authz.RequirePermission.
//
// B9's Technical Design deliberately reuses this existing code rather than
// minting a new record_scans permission: a new code would have shipped
// ungranted to every role the seed data already gives check_in to (Usher,
// Photobooth Staff, Tenant Staff, Tenant Admin, Super Admin) — see
// docs/DEVELOPMENT_PLAN.md §B9.
const PermissionCheckIn = "check_in"

package tickets

// PermissionManageEvents is the permission code (see docs/STAFFING_RBAC.md
// and migration 000014) required to create/update/delete an event's ticket
// types and replace their workflow-step applicability. Same value as the
// seeded manage_events permission, declared locally since permission codes
// are feature-owned (AGENTS.md) — not imported from another feature. Gated
// on every route in ticket_type_routes.go via authz.RequirePermission.
const PermissionManageEvents = "manage_events"

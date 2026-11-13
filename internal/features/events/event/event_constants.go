package event

// PermissionManageEvents is the permission code (seeded by migration 000014)
// required to create, update or delete an event and to replace its setup.
// Reads stay open to any authenticated caller — the service and the
// RequireEventInTenant middleware apply tenant scope. Declared here since
// permission codes are feature-owned (AGENTS.md); events/category declares
// its own copy of the same code for the category routes.
const PermissionManageEvents = "manage_events"

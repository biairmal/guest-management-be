package staffing

// PermissionManageStaff is the permission code (see docs/STAFFING_RBAC.md
// ss3) required to assign/remove/update event staff. Gated on every route
// in assignment_routes.go via authz.RequirePermission.
const PermissionManageStaff = "manage_staff"

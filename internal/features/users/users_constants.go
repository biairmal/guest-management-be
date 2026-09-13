package users

// PermissionManageUsers is the permission code (see docs/STAFFING_RBAC.md §3)
// required to list/create/update/delete users within a tenant, to set
// another user's password, and to transfer tenant-master ownership. Gated on
// every /api/v1/users route except POST /me/password, which is
// self-service-only and needs just a valid token (see user_routes.go and
// UserService.SetOwnPassword).
const PermissionManageUsers = "manage_users"

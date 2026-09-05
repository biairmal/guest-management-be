package roles

import (
	"context"

	"github.com/google/uuid"
)

// NewPermissionResolver returns an authz.PermissionResolver-shaped adapter
// (see internal/core/authz) backed by repo. It exists so internal/core/authz
// never imports this feature slice — authz depends on the small interface
// it declares, and this package supplies an implementation, the same
// direction as auth.NewService taking a users repository rather than
// users.UserService. The returned value satisfies authz.PermissionResolver
// structurally (Go interface satisfaction), without this package importing
// internal/core/authz.
func NewPermissionResolver(repo RolePermissionRepository) *PermissionResolverAdapter {
	return &PermissionResolverAdapter{repo: repo}
}

// PermissionResolverAdapter adapts a RolePermissionRepository to the shape
// internal/core/authz.PermissionResolver expects (ResolvePermissions(ctx,
// roleID) ([]string, error)).
type PermissionResolverAdapter struct {
	repo RolePermissionRepository
}

// ResolvePermissions returns the permission codes granted to roleID.
func (a *PermissionResolverAdapter) ResolvePermissions(ctx context.Context, roleID uuid.UUID) ([]string, error) {
	return a.repo.PermissionCodesByRoleID(ctx, roleID)
}

package authz

import (
	"context"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/google/uuid"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../mocks/authz/mock_permission_resolver.go -package=mockauthz github.com/biairmal/guest-management-be/internal/core/authz PermissionResolver

// PermissionResolver resolves the permission codes granted to a role.
// Declared here (in core, not a feature slice) so this package never
// imports a feature package; internal/features/roles supplies the
// implementation (roles.NewPermissionResolver) and internal/app wires it
// in — the same dependency direction as auth.Service depending on a users
// repository rather than users.UserService.
type PermissionResolver interface {
	// ResolvePermissions returns the permission codes granted to roleID.
	ResolvePermissions(ctx context.Context, roleID uuid.UUID) ([]string, error)
}

// Checker is a small, pure permission oracle: given the caller's role_id
// claim, it answers whether that role holds a given permission code.
type Checker struct {
	logger   logger.Logger
	resolver PermissionResolver
}

// NewChecker returns a Checker backed by resolver.
func NewChecker(logger logger.Logger, resolver PermissionResolver) *Checker {
	return &Checker{logger: logger, resolver: resolver}
}

// Has reports whether the caller (identified by the "role_id" claim in ctx)
// holds code. Returns an error only when the role's permission set could
// not be resolved — the role simply lacking the permission is a false
// result, not an error.
func (c *Checker) Has(ctx context.Context, code string) (bool, error) {
	roleID, ok := RoleIDFromContext(ctx)
	if !ok {
		return false, nil
	}
	codes, err := c.resolver.ResolvePermissions(ctx, roleID)
	if err != nil {
		c.logger.ErrorWithContext(ctx, "authz: resolve permissions failed",
			logger.F("role_id", roleID), logger.F("error", err))
		return false, errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to resolve permissions")
	}
	return containsCode(codes, code), nil
}

// Require returns nil when the caller holds code, and an errorz error
// otherwise: errorz.Unauthorized when no role_id claim is present in ctx
// (authz can't tell who the caller is), errorz.Forbidden when the role
// resolves but lacks code, and a wrapped errorz.Internal when the resolver
// itself fails.
func (c *Checker) Require(ctx context.Context, code string) error {
	roleID, ok := RoleIDFromContext(ctx)
	if !ok {
		return errorz.Unauthorized().WithMessage("no role claim present")
	}
	codes, err := c.resolver.ResolvePermissions(ctx, roleID)
	if err != nil {
		c.logger.ErrorWithContext(ctx, "authz: resolve permissions failed",
			logger.F("role_id", roleID), logger.F("error", err))
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to resolve permissions")
	}
	if !containsCode(codes, code) {
		return errorz.Forbidden().WithMessage("missing required permission: " + code)
	}
	return nil
}

// containsCode reports whether codes contains code.
func containsCode(codes []string, code string) bool {
	for _, have := range codes {
		if have == code {
			return true
		}
	}
	return false
}

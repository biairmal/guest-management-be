package authz

import (
	"context"
	"errors"

	"github.com/biairmal/go-sdk/lib/errorz"
	"github.com/biairmal/go-sdk/lib/logger"
	"github.com/google/uuid"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../mocks/authz/mock_permission_resolver.go -package=mockauthz github.com/biairmal/guest-management-be/internal/core/authz PermissionResolver
//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../mocks/authz/mock_event_role_resolver.go -package=mockauthz github.com/biairmal/guest-management-be/internal/core/authz EventRoleResolver

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

// EventRoleResolver resolves the event-scoped role a user holds on a
// specific event, if any. Declared here for the same reason as
// PermissionResolver: internal/features/staffing (the slice that owns
// EventStaffAssignment) supplies the implementation
// (staffing.NewEventRoleResolver) and internal/app wires it in, without
// this package importing that feature slice.
type EventRoleResolver interface {
	// ResolveEventRoleID returns the role ID of userID's active
	// event_staff_assignments row on eventID, and whether one was found.
	ResolveEventRoleID(ctx context.Context, eventID, userID uuid.UUID) (roleID uuid.UUID, found bool, err error)
}

// Checker is a small, pure permission oracle: given the caller's role_id
// claim, it answers whether that role holds a given permission code. It also
// answers the event-scoped variant (RequireForEvent) by falling back to an
// event_staff_assignments row when the system-level check fails.
type Checker struct {
	logger            logger.Logger
	resolver          PermissionResolver
	eventRoleResolver EventRoleResolver
}

// NewChecker returns a Checker backed by resolver (system-level role_id
// claim checks) and eventRoleResolver (event-scoped assignment fallback used
// by RequireForEvent).
func NewChecker(logger logger.Logger, resolver PermissionResolver, eventRoleResolver EventRoleResolver) *Checker {
	return &Checker{logger: logger, resolver: resolver, eventRoleResolver: eventRoleResolver}
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

// RequireForEvent returns nil when the caller holds code either via their
// system-level role (per Require) or, when that fails with an unauthorized
// or forbidden result, via an active event_staff_assignments row on eventID
// whose role holds code. Require's result is returned immediately on
// success or on any other error (e.g. a wrapped internal resolver failure —
// never masked as a 403); the event-scoped fallback only runs on a 401/403
// from the system check. If no assignment is found on eventID, or one is
// found but its role lacks code, the original system-check error is
// returned unchanged.
func (c *Checker) RequireForEvent(ctx context.Context, eventID uuid.UUID, code string) error {
	systemErr := c.Require(ctx, code)
	if systemErr == nil {
		return nil
	}

	var authzErr *errorz.Error
	if !errors.As(systemErr, &authzErr) ||
		(authzErr.Code != errorz.CodeUnauthorized && authzErr.Code != errorz.CodeForbidden) {
		return systemErr
	}

	userID, ok := UserIDFromContext(ctx)
	if !ok {
		return systemErr
	}

	roleID, found, err := c.eventRoleResolver.ResolveEventRoleID(ctx, eventID, userID)
	if err != nil {
		c.logger.ErrorWithContext(ctx, "authz: resolve event role failed",
			logger.F("event_id", eventID), logger.F("user_id", userID), logger.F("error", err))
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to resolve event role")
	}
	if !found {
		return systemErr
	}

	codes, err := c.resolver.ResolvePermissions(ctx, roleID)
	if err != nil {
		c.logger.ErrorWithContext(ctx, "authz: resolve permissions failed",
			logger.F("role_id", roleID), logger.F("error", err))
		return errorz.Wrap(err).WithCode(errorz.CodeInternal).WithMessage("failed to resolve permissions")
	}
	if !containsCode(codes, code) {
		return systemErr
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

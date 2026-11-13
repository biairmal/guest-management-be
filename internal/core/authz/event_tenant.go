package authz

import (
	"context"

	"github.com/google/uuid"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../../../mocks/authz/mock_event_tenant_resolver.go -package=mockauthz github.com/biairmal/guest-management-be/internal/core/authz EventTenantResolver

// EventTenantResolver resolves which tenant owns an event. Declared here (in
// core, not a feature slice) for the same reason as PermissionResolver and
// EventRoleResolver: internal/features/events/event supplies the
// implementation (its EventTenantRepository) and internal/app wires it in,
// without this package importing that feature slice.
type EventTenantResolver interface {
	// EventTenantID returns the tenant owning eventID, and whether a live
	// (not soft-deleted) event with that ID exists at all.
	EventTenantID(ctx context.Context, eventID uuid.UUID) (tenantID uuid.UUID, found bool, err error)
}

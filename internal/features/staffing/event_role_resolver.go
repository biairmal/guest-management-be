package staffing

import (
	"context"

	"github.com/biairmal/go-sdk/lib/repository"
	"github.com/google/uuid"
)

// NewEventRoleResolver returns an authz.EventRoleResolver-shaped adapter
// (see internal/core/authz) backed by repo. It exists so internal/core/authz
// never imports this feature slice — authz depends on the small interface
// it declares, and this package supplies an implementation, mirroring
// roles.NewPermissionResolver's pattern. The returned value satisfies
// authz.EventRoleResolver structurally (Go interface satisfaction), without
// this package importing internal/core/authz.
func NewEventRoleResolver(
	repo repository.Repository[EventStaffAssignment, uuid.UUID],
) *EventRoleResolverAdapter {
	return &EventRoleResolverAdapter{repo: repo}
}

// EventRoleResolverAdapter adapts a staff assignment repository to the shape
// internal/core/authz.EventRoleResolver expects (ResolveEventRoleID(ctx,
// eventID, userID) (roleID uuid.UUID, found bool, err error)).
type EventRoleResolverAdapter struct {
	repo repository.Repository[EventStaffAssignment, uuid.UUID]
}

// ResolveEventRoleID returns the role ID of userID's active
// event_staff_assignments row on eventID, and whether one was found. Reuses
// the same event_id+user_id, Limit:1 lookup shape as
// staffAssignmentServiceImpl.hasActiveAssignment — the audit repository
// decorator already excludes soft-deleted rows, so a match here means an
// active assignment.
func (a *EventRoleResolverAdapter) ResolveEventRoleID(
	ctx context.Context, eventID, userID uuid.UUID,
) (uuid.UUID, bool, error) {
	opts := &repository.ListOptions{
		Filter: repository.Filter{Conditions: []repository.FilterCondition{
			{Field: "event_id", Operator: repository.FilterOperatorEq, Value: eventID},
			{Field: "user_id", Operator: repository.FilterOperatorEq, Value: userID},
		}},
		Pagination: repository.Pagination{Limit: 1},
		SkipCount:  true,
	}
	items, _, err := a.repo.List(ctx, opts)
	if err != nil {
		return uuid.Nil, false, err
	}
	if len(items) == 0 {
		return uuid.Nil, false, nil
	}
	return items[0].RoleID, true, nil
}

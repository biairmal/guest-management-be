package staffing

import "github.com/google/uuid"

// CreateAssignmentInput is the input for assigning a user to an event with
// an event-scoped role. event_id is taken from the URL and tenant_id from
// the caller's JWT claim; neither is part of this input (see
// docs/FEATURES.md#staffing).
//
// swagger:model CreateAssignmentInput
type CreateAssignmentInput struct {
	UserID uuid.UUID `json:"user_id" validate:"required"`
	RoleID uuid.UUID `json:"role_id" validate:"required"`
}

// UpdateAssignmentInput is the input for changing an existing assignment's
// role. event_id and user_id are immutable after creation — re-assigning a
// different user is a remove (Delete) plus a new assignment (Create), not
// an update (see docs/STAFFING_RBAC.md ss5).
//
// swagger:model UpdateAssignmentInput
type UpdateAssignmentInput struct {
	RoleID *uuid.UUID `json:"role_id,omitempty"`
}

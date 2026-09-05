package staffing

import (
	"time"

	"github.com/google/uuid"
)

// EventStaffAssignment represents a row in the event_staff_assignments
// table — a user assigned to one event with an event-scoped role (see
// docs/STAFFING_RBAC.md ss5). A user can be assigned to a given event at
// most once while active; removing and later re-assigning the same user to
// the same event is a new row (migration 000015's partial unique index on
// (event_id, user_id) WHERE deleted_at IS NULL), preserving the full
// staffing history. Supports soft delete via deleted_at. Uses db tags for
// reflection-based scanning.
//
// swagger:model EventStaffAssignment
type EventStaffAssignment struct {
	ID        uuid.UUID  `json:"id"                   db:"id"`
	EventID   uuid.UUID  `json:"event_id"             db:"event_id"`
	UserID    uuid.UUID  `json:"user_id"              db:"user_id"`
	RoleID    uuid.UUID  `json:"role_id"              db:"role_id"`
	CreatedAt time.Time  `json:"created_at"           db:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"           db:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// TableName returns the database table name.
func (EventStaffAssignment) TableName() string {
	return "event_staff_assignments"
}

package roles

import (
	"time"

	"github.com/google/uuid"
)

// Scope values for Role.Scope. A "system" role is assignable to a user's
// tenant-wide role (users.role_id); an "event" role is assignable to a
// staff member's role on one specific event (staffing.EventStaffAssignment.
// RoleID). A role is one or the other, never both — enforced by a DB CHECK
// constraint (migration 000013) and re-validated by callers that assign a
// role into one of those two contexts.
const (
	ScopeSystem = "system"
	ScopeEvent  = "event"
)

// Role represents a row in the roles table — a named, reusable set of
// permissions (via role_permissions) with a scope that determines where it
// may be assigned. Unlike most tables in this schema, roles has NO
// deleted_at column (it is system/reference data — see docs/DATABASE.md
// ss5); do not add one here, and never wire this model through
// corerepository.NewRepository (use NewRepositoryNoAudit instead — see
// role_repository.go). Uses db tags for reflection-based scanning.
//
// swagger:model Role
type Role struct {
	ID          uuid.UUID `json:"id"          db:"id"`
	Name        string    `json:"name"        db:"name"`
	Description *string   `json:"description,omitempty" db:"description"`
	Scope       string    `json:"scope"       db:"scope"`
	CreatedAt   time.Time `json:"created_at"  db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"  db:"updated_at"`
}

// TableName returns the database table name.
func (Role) TableName() string {
	return "roles"
}

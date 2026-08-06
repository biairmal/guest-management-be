package users

import (
	"time"

	"github.com/google/uuid"
)

// User represents a row in the users table — a member of a tenant with a
// role and login credentials. PasswordHash is never serialized to JSON;
// only the service package computes and reads it. Supports soft delete via
// deleted_at. Uses db tags for reflection-based scanning.
//
// swagger:model User
type User struct {
	ID             uuid.UUID  `json:"id"                  db:"id"`
	TenantID       uuid.UUID  `json:"tenant_id"           db:"tenant_id"`
	Email          string     `json:"email"                db:"email"`
	PasswordHash   string     `json:"-"                    db:"password_hash"`
	RoleID         uuid.UUID  `json:"role_id"             db:"role_id"`
	IsTenantMaster bool       `json:"is_tenant_master"     db:"is_tenant_master"`
	CreatedAt      time.Time  `json:"created_at"           db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"           db:"updated_at"`
	DeletedAt      *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

// TableName returns the database table name.
func (User) TableName() string {
	return "users"
}

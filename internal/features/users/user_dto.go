package users

import "github.com/google/uuid"

// CreateInput is the input for creating a user. tenant_id is set once at
// creation and is not part of UpdateInput — users do not move tenants.
//
// swagger:model UserCreateInput
type CreateInput struct {
	TenantID       uuid.UUID `json:"tenant_id"                 validate:"required"`
	Email          string    `json:"email"                     validate:"required,email"`
	Password       string    `json:"password"                  validate:"required,min=8"`
	RoleID         uuid.UUID `json:"role_id"                   validate:"required"`
	IsTenantMaster bool      `json:"is_tenant_master,omitempty"`
}

// UpdateInput is the input for updating a user. Only non-nil fields are
// applied; a non-nil Password is re-hashed before storage.
//
// swagger:model UserUpdateInput
type UpdateInput struct {
	Email          *string    `json:"email,omitempty"    validate:"omitempty,email"`
	Password       *string    `json:"password,omitempty" validate:"omitempty,min=8"`
	RoleID         *uuid.UUID `json:"role_id,omitempty"`
	IsTenantMaster *bool      `json:"is_tenant_master,omitempty"`
}

package users

import "github.com/google/uuid"

// CreateInput is the input for creating a user. tenant_id is not part of the
// request: the service resolves it from the caller's "tenant_id" JWT claim
// (authz.TenantIDFromContext), mirroring staffing's tenant-scoping-from-JWT
// precedent (see docs/FEATURES.md#staffing). must_change_password is not a
// request field either — the service always sets it true on create.
//
// swagger:model UserCreateInput
type CreateInput struct {
	Email          string    `json:"email"                     validate:"required,email"`
	Password       string    `json:"password"                  validate:"required,min=8"`
	RoleID         uuid.UUID `json:"role_id"                   validate:"required"`
	IsTenantMaster bool      `json:"is_tenant_master,omitempty"`
}

// UpdateInput is the input for updating a user. Only non-nil fields are
// applied. Password is not part of UpdateInput — password changes go through
// the dedicated POST /api/v1/users/{id}/password endpoint (SetPasswordInput)
// instead, so there is exactly one way to set the field.
//
// swagger:model UserUpdateInput
type UpdateInput struct {
	Email          *string    `json:"email,omitempty"    validate:"omitempty,email"`
	RoleID         *uuid.UUID `json:"role_id,omitempty"`
	IsTenantMaster *bool      `json:"is_tenant_master,omitempty"`
}

// SetPasswordInput is the input for POST /api/v1/users/{id}/password. On
// success the service hashes Password and clears MustChangePassword.
//
// swagger:model UserSetPasswordInput
type SetPasswordInput struct {
	Password string `json:"password" validate:"required,min=8"`
}

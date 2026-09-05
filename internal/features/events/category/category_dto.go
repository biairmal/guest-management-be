package category

import "github.com/google/uuid"

// CreateInput is the input for creating an event category.
//
// swagger:model CreateInput
type CreateInput struct {
	Source   string     `json:"source"              validate:"required,oneof=app tenant"`
	TenantID *uuid.UUID `json:"tenant_id,omitempty"`
	Name     string     `json:"name"                validate:"required"`
}

// UpdateInput is the input for updating an event category.
//
// swagger:model UpdateInput
type UpdateInput struct {
	Source   *string    `json:"source,omitempty"    validate:"omitempty,oneof=app tenant"`
	TenantID *uuid.UUID `json:"tenant_id,omitempty"`
	Name     *string    `json:"name,omitempty"      validate:"omitempty,min=1"`
}

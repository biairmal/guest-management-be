package tenants

import "encoding/json"

// CreateInput is the input for creating a tenant.
//
// swagger:model TenantCreateInput
type CreateInput struct {
	Name     string          `json:"name"               validate:"required"`
	Type     *string         `json:"type,omitempty"`
	Settings json.RawMessage `json:"settings,omitempty"`
	Branding json.RawMessage `json:"branding,omitempty"`
}

// UpdateInput is the input for updating a tenant.
//
// swagger:model TenantUpdateInput
type UpdateInput struct {
	Name     *string         `json:"name,omitempty"     validate:"omitempty,min=1"`
	Type     *string         `json:"type,omitempty"`
	Settings json.RawMessage `json:"settings,omitempty"`
	Branding json.RawMessage `json:"branding,omitempty"`
}

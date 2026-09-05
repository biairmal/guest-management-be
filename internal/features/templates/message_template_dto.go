package templates

import (
	"encoding/json"

	"github.com/google/uuid"
)

// CreateInput is the input for creating a message template.
//
// swagger:model MessageTemplateCreateInput
type CreateInput struct {
	Source    string          `json:"source"              validate:"required,oneof=app tenant event"`
	TenantID  *uuid.UUID      `json:"tenant_id,omitempty"`
	EventID   *uuid.UUID      `json:"event_id,omitempty"`
	Name      string          `json:"name"                validate:"required"`
	Channel   string          `json:"channel"             validate:"required,oneof=email whatsapp"`
	Subject   *string         `json:"subject,omitempty"`
	Body      string          `json:"body"                validate:"required"`
	Variables json.RawMessage `json:"variables,omitempty"`
}

// UpdateInput is the input for updating a message template.
//
// swagger:model MessageTemplateUpdateInput
type UpdateInput struct {
	Source    *string         `json:"source,omitempty"    validate:"omitempty,oneof=app tenant event"`
	TenantID  *uuid.UUID      `json:"tenant_id,omitempty"`
	EventID   *uuid.UUID      `json:"event_id,omitempty"`
	Name      *string         `json:"name,omitempty"      validate:"omitempty,min=1"`
	Channel   *string         `json:"channel,omitempty"   validate:"omitempty,oneof=email whatsapp"`
	Subject   *string         `json:"subject,omitempty"`
	Body      *string         `json:"body,omitempty"      validate:"omitempty,min=1"`
	Variables json.RawMessage `json:"variables,omitempty"`
}

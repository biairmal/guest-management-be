package category

import (
	"encoding/json"

	"github.com/google/uuid"
)

// WorkflowStepInput is one workflow step of a category's template set. Its
// position in the workflow_steps array is its step order.
//
// swagger:model WorkflowStepInput
type WorkflowStepInput struct {
	Name           string `json:"name"            validate:"required"`
	AllowsMultiple bool   `json:"allows_multiple"`
}

// TicketTypeInput is one ticket type of a category's template set. Steps are
// indexes into the same payload's workflow_steps array; rules is an opaque
// JSON document, passed through unvalidated (defaults to {}).
//
// swagger:model TicketTypeInput
type TicketTypeInput struct {
	Name  string          `json:"name"            validate:"required"`
	Rules json.RawMessage `json:"rules,omitempty" swaggertype:"object"`
	Steps []int           `json:"steps"`
}

// CreateInput is the input for creating an event category together with its
// whole template set. tenant_id is honored only for platform-tenant callers
// creating a tenant category; everyone else gets their own tenant from the
// JWT. The max=100 array limits match query.DefaultMaxSize, the list limit
// event creation seeds with, so a template set is never silently truncated.
//
// swagger:model CreateInput
type CreateInput struct {
	Source        string              `json:"source"              validate:"required,oneof=app tenant"`
	TenantID      *uuid.UUID          `json:"tenant_id,omitempty"`
	Name          string              `json:"name"                validate:"required"`
	WorkflowSteps []WorkflowStepInput `json:"workflow_steps"      validate:"max=100,dive"`
	TicketTypes   []TicketTypeInput   `json:"ticket_types"        validate:"max=100,dive"`
}

// ReplaceInput is the input for replacing an event category's name and whole
// template set. TemplateVersion is the version the edit was based on; a
// mismatch with the current version is a 409. source/tenant_id are immutable.
//
// swagger:model ReplaceInput
type ReplaceInput struct {
	Name            string              `json:"name"             validate:"required"`
	TemplateVersion int                 `json:"template_version" validate:"required,min=1"`
	WorkflowSteps   []WorkflowStepInput `json:"workflow_steps"   validate:"max=100,dive"`
	TicketTypes     []TicketTypeInput   `json:"ticket_types"     validate:"max=100,dive"`
}

// Detail is an event category with its current template set — the
// response of GET/POST/PUT on a single category.
//
// swagger:model Detail
type Detail struct {
	EventCategory
	WorkflowSteps []WorkflowStepView `json:"workflow_steps"`
	TicketTypes   []TicketTypeView   `json:"ticket_types"`
}

// WorkflowStepView is one workflow step template; array order is step order.
type WorkflowStepView struct {
	ID             uuid.UUID `json:"id"`
	Name           string    `json:"name"`
	AllowsMultiple bool      `json:"allows_multiple"`
}

// TicketTypeView is one ticket type template; Steps are indexes into
// Detail.WorkflowSteps, ascending.
type TicketTypeView struct {
	ID    uuid.UUID       `json:"id"`
	Name  string          `json:"name"`
	Rules json.RawMessage `json:"rules" swaggertype:"object"`
	Steps []int           `json:"steps"`
}

// TicketTypeTemplateView is a stored ticket type template as
// TicketTypeTemplateStore reports it.
type TicketTypeTemplateView struct {
	ID                      uuid.UUID
	Name                    string
	Rules                   json.RawMessage
	WorkflowStepTemplateIDs []uuid.UUID
}

// TicketTypeTemplateDraft is a ticket type template to write through
// TicketTypeTemplateStore, with step indexes already resolved to the new
// workflow step template IDs.
type TicketTypeTemplateDraft struct {
	Name                    string
	Rules                   json.RawMessage
	WorkflowStepTemplateIDs []uuid.UUID
}

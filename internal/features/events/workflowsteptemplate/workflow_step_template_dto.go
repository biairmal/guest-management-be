package workflowsteptemplate

import "encoding/json"

// CreateWorkflowStepTemplateInput is the input for creating a workflow step template.
//
// swagger:model CreateWorkflowStepTemplateInput
type CreateWorkflowStepTemplateInput struct {
	Name                    string           `json:"name"                                 validate:"required"`
	OrderIndex              int              `json:"order_index"                          validate:"gte=0"`
	AllowsMultiple          bool             `json:"allows_multiple,omitempty"`
	TicketTypeApplicability *json.RawMessage `json:"ticket_type_applicability,omitempty"`
}

// UpdateWorkflowStepTemplateInput is the input for updating a workflow step
// template. category_id is immutable and not part of this input.
//
// swagger:model UpdateWorkflowStepTemplateInput
type UpdateWorkflowStepTemplateInput struct {
	Name                    *string          `json:"name,omitempty"                       validate:"omitempty,min=1"`
	OrderIndex              *int             `json:"order_index,omitempty"                validate:"omitempty,gte=0"`
	AllowsMultiple          *bool            `json:"allows_multiple,omitempty"`
	TicketTypeApplicability *json.RawMessage `json:"ticket_type_applicability,omitempty"`
}

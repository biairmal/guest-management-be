package workflowstep

import "github.com/google/uuid"

// CreateWorkflowStepInput is the input for creating a workflow step.
//
// swagger:model CreateWorkflowStepInput
type CreateWorkflowStepInput struct {
	Name           string `json:"name"                       validate:"required"`
	OrderIndex     int    `json:"order_index"                validate:"gte=0"`
	AllowsMultiple bool   `json:"allows_multiple,omitempty"`
}

// UpdateWorkflowStepInput is the input for updating a workflow step.
// event_id is immutable and not part of this input.
//
// swagger:model UpdateWorkflowStepInput
type UpdateWorkflowStepInput struct {
	Name           *string `json:"name,omitempty"            validate:"omitempty,min=1"`
	OrderIndex     *int    `json:"order_index,omitempty"      validate:"omitempty,gte=0"`
	AllowsMultiple *bool   `json:"allows_multiple,omitempty"`
}

// SyncWorkflowStepInput is one entry in a WorkflowStepService.Sync request:
// an existing step to update when ID is set, or a new step to create when
// ID is omitted.
//
// swagger:model SyncWorkflowStepInput
type SyncWorkflowStepInput struct {
	ID             *uuid.UUID `json:"id,omitempty"`
	Name           string     `json:"name"                       validate:"required"`
	OrderIndex     int        `json:"order_index"                validate:"gte=0"`
	AllowsMultiple bool       `json:"allows_multiple,omitempty"`
}
